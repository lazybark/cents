package desktop

import (
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakePickers answers dialogs with fixed paths ("" is cancelling).
type fakePickers struct {
	file, open, folder string
	dirs               []string
}

func (f *fakePickers) SaveFile(_, dir, _ string) (string, error) {
	f.dirs = append(f.dirs, dir)
	return f.file, nil
}

func (f *fakePickers) OpenFile(_, dir, _ string) (string, error) {
	f.dirs = append(f.dirs, dir)
	return f.open, nil
}

func (f *fakePickers) Folder(_, dir string) (string, error) {
	f.dirs = append(f.dirs, dir)
	return f.folder, nil
}

func TestBackupDatabase(t *testing.T) {
	api := newTestAPI(t)
	dir := t.TempDir()
	pick := &fakePickers{}
	api.pick = pick

	if got, err := api.BackupDatabase(); err != nil || got.Path != "" {
		t.Fatalf("cancelling does nothing: %+v %v", got, err)
	}

	pick.file = filepath.Join(dir, "my-backup") // no extension: .db is added
	got, err := api.BackupDatabase()
	if err != nil || got.Path != filepath.Join(dir, "my-backup.db") || got.At == "" {
		t.Fatalf("unexpected %+v %v", got, err)
	}

	if info, err := os.Stat(got.Path); err != nil || info.Size() == 0 {
		t.Fatalf("expected the backup file: %v", err)
	}

	view, err := api.ExportOptions()
	if err != nil || view.LastBackupPath != got.Path || view.LastBackupAt != got.At || len(view.Datasets) != 16 {
		t.Fatalf("expected the backup remembered: %+v %v", view, err)
	}

	// The next backup starts in the folder of the last one.
	if _, err := api.BackupDatabase(); err != nil || pick.dirs[len(pick.dirs)-1] != dir {
		t.Fatalf("expected the last folder: %v %v", pick.dirs, err)
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
	pick := &fakePickers{folder: parent}
	api.pick = pick

	for _, bad := range []ExportInput{
		{From: "2026-04-01", To: "2026-03-01", Datasets: []string{"debts"}},
		{From: "01.03.2026", Datasets: []string{"debts"}},
		{Datasets: nil},
		{Datasets: []string{"nope"}},
	} {
		if _, err := api.ExportCSV(bad); err == nil {
			t.Errorf("expected %+v to fail", bad)
		}
	}

	got, err := api.ExportCSV(ExportInput{From: "2026-03-01", To: "2026-03-31", Datasets: []string{"incomes_expenses", "debts", "monthly_summary"}})
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
	again, err := api.ExportCSV(ExportInput{Datasets: []string{"debts"}})
	if err != nil || again.Folder != got.Folder+"-2" {
		t.Fatalf("expected a second folder: %+v %v", again, err)
	}

	pick.folder = ""
	if cancelled, err := api.ExportCSV(ExportInput{Datasets: []string{"debts"}}); err != nil || cancelled.Folder != "" {
		t.Fatalf("cancelling does nothing: %+v %v", cancelled, err)
	}

	view, _ := api.ExportOptions()
	if view.FirstDate != "2026-03-01" {
		t.Fatalf("expected the first entry's day: %+v", view)
	}
}
