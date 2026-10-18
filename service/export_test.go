package service

import (
	"bytes"
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveBackup(t *testing.T) {
	api := newTestAPI(t)
	dir := t.TempDir()

	if _, err := SaveBackup(api, " "); err == nil {
		t.Fatal("expected a path to be asked for")
	}

	folder := filepath.Join(dir, "folder.db")
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := SaveBackup(api, folder); err == nil || !strings.Contains(err.Error(), "is a folder") {
		t.Fatalf("expected a folder to be refused, got %v", err)
	}

	got, err := SaveBackup(api, filepath.Join(dir, "my-backup")) // no extension: .db is added
	if err != nil || got.Path != filepath.Join(dir, "my-backup.db") || got.At == "" {
		t.Fatalf("unexpected %+v %v", got, err)
	}

	if info, err := os.Stat(got.Path); err != nil || info.Size() == 0 {
		t.Fatalf("expected the backup file: %v", err)
	}

	if _, err := os.Stat(got.Path + ".partial"); !os.IsNotExist(err) {
		t.Fatalf("expected no partial file left: %v", err)
	}

	view, err := api.ExportOptions()
	if err != nil || view.LastBackupPath != got.Path || view.LastBackupAt != got.At || len(view.Datasets) != 16 {
		t.Fatalf("expected the backup remembered: %+v %v", view, err)
	}

	// A backup that's only sent isn't remembered.
	var sent bytes.Buffer
	if _, err := api.WriteBackup(&sent, ""); err != nil || sent.Len() == 0 {
		t.Fatalf("expected the backup sent: %v", err)
	}

	if again, _ := api.ExportOptions(); again.LastBackupPath != got.Path {
		t.Fatalf("expected the last saved backup kept: %+v", again)
	}
}

func readCSV(t *testing.T, path string) [][]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.HasPrefix(data, []byte("\uFEFF")) {
		t.Fatalf("%s: expected the UTF-8 mark", path)
	}

	rows, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\uFEFF")))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	return rows
}

func TestExportCSV(t *testing.T) {
	api := newTestAPI(t) // base $, EUR at 1.08
	for _, input := range []NewCashflowInput{
		{IsIncome: true, Currency: "$", Amount: "1000", Date: "2026-03-01", Category: "Salary"},
		{Currency: "EUR", Amount: "12.5", Date: "2026-03-15", Category: "Rent", Comment: "flat, March"},
		{Currency: "$", Amount: "9", Date: "2026-04-02", Category: "Rent"},
	} {
		if _, err := api.CreateCashflow(input); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := api.CreateDebt(NewDebtInput{Peer: "Ann", Currency: "$", Amount: "10", AmountPaid: "0", CreatedAt: "2026-03-01"}); err != nil {
		t.Fatal(err)
	}

	parent := t.TempDir()

	for _, bad := range []ExportInput{
		{From: "2026-04-01", To: "2026-03-01", Datasets: []string{"debts"}},
		{From: "01.03.2026", Datasets: []string{"debts"}},
		{Datasets: nil},
		{Datasets: []string{"nope"}},
	} {
		if _, err := ExportCSV(api, parent, bad); err == nil {
			t.Errorf("expected %+v to fail", bad)
		}
	}

	got, err := ExportCSV(api, parent, ExportInput{From: "2026-03-01", To: "2026-03-31", Datasets: []string{"incomes_expenses", "debts", "monthly_summary"}})
	if err != nil {
		t.Fatal(err)
	}

	today := time.Now().Format("2006-01-02")
	if got.Folder != filepath.Join(parent, "cents-export-"+today) || len(got.Files) != 3 || got.Files[0].Name != "incomes_expenses.csv" || got.Files[0].Rows != 2 {
		t.Fatalf("unexpected %+v", got)
	}

	rows := readCSV(t, filepath.Join(got.Folder, "incomes_expenses.csv"))
	if len(rows) != 3 || strings.Join(rows[2], "|") != "2026-03-15|Expense|Rent||EUR|12.50|1.08|13.50|flat, March|" || rows[0][7] != "Amount ($)" {
		t.Fatalf("unexpected rows %q", rows)
	}

	if debts := readCSV(t, filepath.Join(got.Folder, "debts.csv")); len(debts) != 2 || debts[1][1] != "Ann" {
		t.Fatalf("unexpected debts %q", debts)
	}

	// Exporting again the same day makes a new folder.
	again, err := ExportCSV(api, parent, ExportInput{Datasets: []string{"debts"}})
	if err != nil || again.Folder != got.Folder+"-2" {
		t.Fatalf("expected a second folder: %+v %v", again, err)
	}

	if _, err := ExportCSV(api, filepath.Join(parent, "missing"), ExportInput{Datasets: []string{"debts"}}); err == nil {
		t.Fatal("expected a missing folder to be refused")
	}

	var one bytes.Buffer
	if rows, err := api.WriteCSV(&one, "debts", "", ""); err != nil || rows != 1 || !strings.Contains(one.String(), "Ann") {
		t.Fatalf("expected one debt: %d %v %q", rows, err, one.String())
	}

	if _, err := api.WriteCSV(&one, "nope", "", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected an unknown dataset not found, got %v", err)
	}

	view, _ := api.ExportOptions()
	if view.FirstDate != "2026-03-01" {
		t.Fatalf("expected the first entry's day: %+v", view)
	}
}
