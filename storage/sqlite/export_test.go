package sqlite

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/debt"
	"github.com/lazybark/cents/flows/goal"
	"github.com/lazybark/cents/flows/invoice"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/subscription"
	"github.com/lazybark/cents/flows/tax"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestExportDataWritesSingleDatasetJSON(t *testing.T) {
	db := newExportTestDB(t)
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	if err := db.Create(&account.Account{Name: "Main", Currency: "$", BalanceCents: 12345, LastUpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "accounts.json")
	result, err := ExportData(db, ExportRequest{Dataset: ExportDatasetAccounts, Format: ExportFormatJSON, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != path {
		t.Fatalf("expected path %q, got %q", path, result.Path)
	}
	if result.FileCount != 1 {
		t.Fatalf("expected one exported file, got %d", result.FileCount)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var rows []account.Account
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Name != "Main" || rows[0].BalanceCents != 12345 {
		t.Fatalf("unexpected account export: %#v", rows)
	}
}

func TestExportDataWritesAllCSVFiles(t *testing.T) {
	db := newExportTestDB(t)
	if err := db.Create(&settings.SettingRecord{SettingID: "base_currency", SettingValue: "$"}).Error; err != nil {
		t.Fatal(err)
	}

	result, err := ExportData(db, ExportRequest{Dataset: ExportDatasetAll, Format: ExportFormatCSV, Path: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if result.FileCount != len(exportTables()) {
		t.Fatalf("expected %d exported files, got %d", len(exportTables()), result.FileCount)
	}

	file, err := os.Open(filepath.Join(result.Path, "setting_records.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected header and one row, got %#v", rows)
	}
	if rows[0][0] != "SettingID" || rows[0][1] != "SettingValue" {
		t.Fatalf("unexpected header: %#v", rows[0])
	}
	if rows[1][0] != "base_currency" || rows[1][1] != "$" {
		t.Fatalf("unexpected row: %#v", rows[1])
	}
}

func newExportTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	if err := db.AutoMigrate(
		&account.Account{},
		&account.AccountValueLog{},
		&subscription.Subscription{},
		&debt.Debt{},
		&debt.DebtLog{},
		&goal.Goal{},
		&goal.GoalLog{},
		&tax.Tax{},
		&tax.TaxLog{},
		&invoice.Invoice{},
		&cashflow.CashflowEntry{},
		&settings.SettingRecord{},
		&settings.SettingCurrency{},
		&settings.SettingPaymentMethod{},
		&settings.SettingTaxType{},
		&settings.SettingIncomeCategory{},
		&settings.SettingExpenseCategory{},
	); err != nil {
		t.Fatal(err)
	}

	return db
}
