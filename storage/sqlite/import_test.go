package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/debt"
	"github.com/lazybark/cents/flows/invoice"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/tag"
)

func importStore(t *testing.T) (*SQLiteStorage, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cents.db")
	db, _, err := OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	s := NewSQLiteStorage(db)
	t.Cleanup(func() { _ = s.Close() })
	return s, path
}

func TestImportDatabaseLinksPreviewAndRepeat(t *testing.T) {
	source, path := importStore(t)
	target, _ := importStore(t)
	create := func(s *SQLiteStorage, item any) {
		t.Helper()
		if err := s.db.Create(item).Error; err != nil {
			t.Fatal(err)
		}
	}
	c := settings.SettingExpenseCategory{CategoryName: "Food"}
	create(source, &c)
	create(target, &settings.SettingExpenseCategory{CategoryName: "Food"})
	stts, _ := source.LoadAppSettings()
	bank := account.Account{Name: "Imported bank", CurrencyUID: stts.BaseCurrencyUID, BalanceCents: 10000, CreatedAt: time.Now().UTC()}
	create(source, &bank)
	create(target, &account.Account{Name: "Keep bank", Currency: "$", BalanceCents: 50000})
	entry := cashflow.CashflowEntry{IsIncome: false, CurrencyUID: stts.BaseCurrencyUID, AccountUID: bank.UID, CategoryUID: c.UID, AmountCents: 1200, RateToBase: 1, AmountBaseCents: 1200, EntryDate: time.Now().UTC(), CreatedAt: time.Now().UTC(), Comment: "Imported lunch"}
	create(source, &entry)
	create(target, &cashflow.CashflowEntry{Currency: "$", AmountCents: 900, Comment: "Keep entry"})
	label := tag.Tag{Name: "Trip"}
	create(source, &label)
	create(source, &tag.EntryTag{EntryID: entry.ID, TagUID: label.UID})
	work := tag.Tag{Name: "Work"}
	create(source, &work)
	create(source, &tag.EntryTag{EntryID: entry.ID, TagUID: work.UID})
	existingTag := tag.Tag{Name: "Trip"}
	create(target, &existingTag)
	create(target, &tag.EntryTag{EntryID: 1, TagUID: existingTag.UID})
	create(source, &invoice.Invoice{Title: "Imported invoice", CurrencyUID: stts.BaseCurrencyUID, CashflowEntryID: entry.ID, Paid: true})
	d := debt.Debt{Peer: "Friend", CurrencyUID: stts.BaseCurrencyUID, AmountCents: 2500, AmountPaidCents: 1200}
	create(source, &d)
	create(source, &debt.DebtLog{DebtID: d.ID, CashflowEntryID: entry.ID, DeltaPaidCents: 1200})
	create(source, &account.AccountValueLog{AccountID: bank.ID, LogDate: time.Now().UTC(), ValueCents: 10000})
	preview, err := target.ImportDatabase(context.Background(), path, true)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Preview || preview.Added < 8 {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	var count int64
	if err := target.db.Table("cashflow_entries").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("preview wrote entries: %d %v", count, err)
	}
	if target.db.Migrator().HasTable(&ImportRecord{}) {
		t.Fatal("preview persisted import identities")
	}
	result, err := target.ImportDatabase(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != preview.Added {
		t.Fatalf("preview/commit mismatch: %+v %+v", preview, result)
	}
	entries, err := target.LoadCashflows()
	if err != nil || len(entries) != 2 {
		t.Fatalf("entries: %+v %v", entries, err)
	}
	var imported cashflow.CashflowEntry
	for _, e := range entries {
		if e.Comment == "Imported lunch" {
			imported = e
		}
	}
	if imported.ID == entry.ID || imported.AccountName != bank.Name || imported.Category != "Food" || len(imported.Tags) != 2 || imported.Tags[0] != "Trip" || imported.Tags[1] != "Work" {
		t.Fatalf("broken imported links: %+v", imported)
	}
	invoices, _ := target.LoadInvoices()
	if len(invoices) != 1 || invoices[0].CashflowEntryID != imported.ID {
		t.Fatalf("invoice link: %+v", invoices)
	}
	var log debt.DebtLog
	if err := target.db.First(&log).Error; err != nil || log.CashflowEntryID != imported.ID {
		t.Fatalf("debt link: %+v %v", log, err)
	}
	var bankLog account.AccountValueLog
	if err := target.db.First(&bankLog).Error; err != nil || bankLog.AccountID == bank.ID {
		t.Fatalf("account log: %+v %v", bankLog, err)
	}
	// Edits made after import win over a repeated source snapshot.
	if err := target.db.Table("cashflow_entries").Where("id = ?", imported.ID).Update("comment", "Edited locally").Error; err != nil {
		t.Fatal(err)
	}
	again, err := target.ImportDatabase(context.Background(), path, false)
	if err != nil || again.Added != 0 || again.Conflicts == 0 {
		t.Fatalf("repeat: %+v %v", again, err)
	}
	var keep account.Account
	if err := target.db.Where("name = ?", "Keep bank").First(&keep).Error; err != nil || keep.BalanceCents != 50000 {
		t.Fatalf("overwrote current account: %+v %v", keep, err)
	}
}

func TestImportFailureRollsBackAndRejectsWrongFiles(t *testing.T) {
	source, path := importStore(t)
	target, _ := importStore(t)
	if err := source.db.Create(&settings.SettingExpenseCategory{CategoryName: "Must roll back"}).Error; err != nil {
		t.Fatal(err)
	}
	// Bad data late in dependency order must roll back earlier inserts too.
	if err := source.db.Create(&tag.EntryTag{EntryID: 999, TagUID: "missing"}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := target.ImportDatabase(context.Background(), path, false); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("bad reference: %v", err)
	}
	var count int64
	if err := target.db.Table("setting_expense_categories").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("partial import: %d %v", count, err)
	}
	if target.db.Migrator().HasTable(&ImportRecord{}) {
		t.Fatal("failed import persisted identities")
	}
	bad := filepath.Join(t.TempDir(), "fake.db")
	if err := os.WriteFile(bad, []byte("not a database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := target.ImportDatabase(context.Background(), bad, true); err == nil {
		t.Fatal("accepted malformed upload")
	}
	if err := source.db.Exec("CREATE VIEW custom AS SELECT 1").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := target.ImportDatabase(context.Background(), path, true); err == nil || !strings.Contains(err.Error(), "views") {
		t.Fatalf("accepted view: %v", err)
	}
}

func TestImportBaseCurrency(t *testing.T) {
	source, path := importStore(t)
	if err := source.db.Model(&settings.SettingCurrency{}).Where("is_base = ?", true).Updates(map[string]any{"currency_name": "EUR", "code": "EUR"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := source.SaveSettingRecord(&settings.SettingRecord{SettingID: settings.BaseCurrencySettingID, SettingValue: "EUR"}); err != nil {
		t.Fatal(err)
	}
	fresh, _ := importStore(t)
	if _, err := fresh.ImportDatabase(context.Background(), path, true); err != nil {
		t.Fatal(err)
	}
	if stts, _ := fresh.LoadAppSettings(); stts.BaseCurrency != "$" {
		t.Fatal("preview changed base currency")
	}
	if _, err := fresh.ImportDatabase(context.Background(), path, false); err != nil {
		t.Fatal(err)
	}
	if stts, _ := fresh.LoadAppSettings(); stts.BaseCurrency != "EUR" || !stts.CurrenciesSetUp {
		t.Fatalf("did not adopt base: %+v", stts)
	}
	used, _ := importStore(t)
	if err := used.CreateAccount(&account.Account{Name: "Keep", Currency: "$"}); err != nil {
		t.Fatal(err)
	}
	if _, err := used.ImportDatabase(context.Background(), path, false); err == nil || !strings.Contains(err.Error(), "base currency") {
		t.Fatalf("different bases: %v", err)
	}
}

func TestImportPreservesIdenticalTransactions(t *testing.T) {
	source, path := importStore(t)
	target, _ := importStore(t)
	stts, _ := source.LoadAppSettings()
	for range 2 {
		if err := source.db.Create(&cashflow.CashflowEntry{CurrencyUID: stts.BaseCurrencyUID, AmountCents: 100, Comment: "Same purchase", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := range 2 {
		result, err := target.ImportDatabase(context.Background(), path, false)
		if err != nil {
			t.Fatal(err)
		}
		var count int64
		if err := target.db.Table("cashflow_entries").Count(&count).Error; err != nil || count != 2 {
			t.Fatalf("transaction multiplicity: %d %v", count, err)
		}
		if i == 1 && result.Added != 0 {
			t.Fatalf("repeat duplicated transactions: %+v", result)
		}
	}
}

func TestImportRefusesEncryptedDestination(t *testing.T) {
	source, path := importStore(t)
	if err := source.CreateAccount(&account.Account{Name: "Imported securely", Currency: "$"}); err != nil {
		t.Fatal(err)
	}
	plain, targetPath := importStore(t)
	encrypted, err := plain.Encrypt(targetPath, "test-password")
	if err != nil {
		t.Fatal(err)
	}
	defer encrypted.Close()
	if _, err := encrypted.ImportDatabase(context.Background(), path, true); err == nil || !strings.Contains(err.Error(), "encrypted destinations") {
		t.Fatalf("encrypted target: %v", err)
	}
	accounts, err := encrypted.LoadAccounts()
	if err != nil || len(accounts) != 0 {
		t.Fatalf("failed import changed encrypted target: %+v %v", accounts, err)
	}
}
