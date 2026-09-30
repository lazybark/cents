package sqlite

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/budget"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/subscription"
	"github.com/lazybark/cents/flows/tax"
)

func openTest(t *testing.T) (*SQLiteStorage, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cents.db")
	db, _, err := OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}

	return NewSQLiteStorage(db), path
}

func TestRenamesNeedNoRecordChanges(t *testing.T) {
	s, _ := openTest(t)

	food := settings.SettingExpenseCategory{CategoryName: "Food"}
	if err := s.SaveSettingExpenseCategory(&food); err != nil || food.UID == "" {
		t.Fatalf("expected a UID: %+v %v", food, err)
	}

	bank := account.Account{Name: "Bank", Currency: "$"}
	if err := s.CreateAccount(&bank); err != nil || bank.UID == "" {
		t.Fatalf("expected a UID: %+v %v", bank, err)
	}

	entry := cashflow.CashflowEntry{Currency: "$", AmountCents: 100, Category: "food", AccountName: " bank ", EntryDate: time.Now()}
	if err := s.CreateCashflow(&entry); err != nil {
		t.Fatal(err)
	}

	if entry.CategoryUID != food.UID || entry.AccountUID != bank.UID {
		t.Fatalf("expected links: %+v", entry)
	}

	// Rename both: a saved row built without its UID keeps it.
	renamed := settings.SettingExpenseCategory{ID: food.ID, CategoryName: "Groceries"}
	if err := s.SaveSettingExpenseCategory(&renamed); err != nil {
		t.Fatal(err)
	}
	if err := s.db.Model(&account.Account{}).Where("id = ?", bank.ID).Update("name", "Main bank").Error; err != nil {
		t.Fatal(err)
	}

	entries, _ := s.LoadCashflows()
	if entries[0].Category != "Groceries" || entries[0].AccountName != "Main bank" {
		t.Fatalf("expected the new names: %+v", entries[0])
	}

	// Editing an entry links what it's set to now.
	entries[0].AccountName = ""
	if err := s.SaveCashflows(entries); err != nil {
		t.Fatal(err)
	}
	if entries, _ = s.LoadCashflows(); entries[0].AccountUID != "" || entries[0].CategoryUID != food.UID {
		t.Fatalf("expected the account unlinked: %+v", entries[0])
	}

	for _, bad := range []cashflow.CashflowEntry{
		{Category: "Nope", EntryDate: time.Now()},
		{Category: "Groceries", AccountName: "Nope", EntryDate: time.Now()},
		{IsIncome: true, Category: "Groceries", EntryDate: time.Now()}, // an expense category
	} {
		if err := s.CreateCashflow(&bad); err == nil {
			t.Errorf("expected %+v to fail", bad)
		}
	}
}

func TestOtherRecordsLink(t *testing.T) {
	s, _ := openTest(t)

	// A payment method typed in (the TUI's custom one) is added.
	sub := subscription.Subscription{Name: "Music", Currency: "$", AmountCents: 5, Period: "month", PaymentMethod: "Revolut"}
	if err := s.CreateSubscription(&sub); err != nil || sub.PaymentMethodUID == "" {
		t.Fatalf("expected a link: %+v %v", sub, err)
	}
	stts, _ := s.LoadAppSettings()
	if !containsMethod(stts.PaymentMethods, "Revolut") {
		t.Fatal("expected Revolut added to payment methods")
	}

	vat := settings.SettingTaxType{Country: "NL", TaxTypeName: "VAT"}
	if err := s.SaveSettingTaxType(&vat); err != nil {
		t.Fatal(err)
	}
	x := tax.Tax{TaxTypeID: vat.ID, Currency: "$", AmountDueCents: 100}
	if err := s.CreateTax(&x); err != nil || x.TaxTypeUID != vat.UID {
		t.Fatalf("expected a link: %+v %v", x, err)
	}
	if taxes, _ := s.LoadTaxes(); taxes[0].TaxTypeID != vat.ID || taxes[0].TaxTypeName != "VAT" || taxes[0].TaxCountry != "NL" {
		t.Fatalf("expected the type filled in: %+v", taxes[0])
	}

	rent := settings.SettingExpenseCategory{CategoryName: "Rent"}
	if err := s.SaveSettingExpenseCategory(&rent); err != nil {
		t.Fatal(err)
	}
	b := budget.Budget{Category: "rent", LimitCents: 100}
	if err := s.CreateBudget(&b); err != nil || b.CategoryUID != rent.UID {
		t.Fatalf("expected a link: %+v %v", b, err)
	}
	total := budget.Budget{LimitCents: 100}
	if err := s.CreateBudget(&total); err != nil || total.CategoryUID != "" {
		t.Fatalf("all spending links nothing: %+v %v", total, err)
	}
	if budgets, _ := s.LoadBudgets(); budgets[0].Category != "Rent" || budgets[1].Category != "" {
		t.Fatalf("unexpected budgets %+v", budgets)
	}

	// Two active accounts with one name can't be told apart.
	for range 2 {
		if err := s.CreateAccount(&account.Account{Name: "Cash", Currency: "$"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.CreateCashflow(&cashflow.CashflowEntry{Category: "Rent", AccountName: "cash", EntryDate: time.Now()}); err == nil {
		t.Fatal("expected an ambiguous account to fail")
	}
}

func containsMethod(methods []settings.SettingPaymentMethod, name string) bool {
	for _, m := range methods {
		if m.PaymentMethodName == name {
			return true
		}
	}

	return false
}

func TestDeletingWhatsUsedAndMerging(t *testing.T) {
	s, _ := openTest(t)

	food := settings.SettingExpenseCategory{CategoryName: "Food"}
	groceries := settings.SettingExpenseCategory{CategoryName: "Groceries"}
	spare := settings.SettingExpenseCategory{CategoryName: "Spare"}
	for _, c := range []*settings.SettingExpenseCategory{&food, &groceries, &spare} {
		if err := s.SaveSettingExpenseCategory(c); err != nil {
			t.Fatal(err)
		}
	}

	for _, name := range []string{"Food", "Food", "Groceries"} {
		if err := s.CreateCashflow(&cashflow.CashflowEntry{Category: name, EntryDate: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	for _, b := range []budget.Budget{{Category: "Food", LimitCents: 1}, {Category: "Groceries", LimitCents: 2}} {
		if err := s.CreateBudget(&b); err != nil {
			t.Fatal(err)
		}
	}

	var inUse InUseError
	if err := s.DeleteSetting("expense_category", food.ID); !errors.As(err, &inUse) || inUse.Count != 3 {
		t.Fatalf("expected Food in use by 2 entries and a budget: %v", err)
	}

	if err := s.DeleteSetting("expense_category", spare.ID); err != nil {
		t.Fatalf("an unused one goes: %v", err)
	}

	if _, err := s.Merge("expense_category", food.ID, food.ID); err == nil {
		t.Fatal("expected merging into itself to fail")
	}

	moved, err := s.Merge("expense_category", food.ID, groceries.ID)
	if err != nil || moved != 2 {
		t.Fatalf("expected 2 entries moved: %d %v", moved, err)
	}

	entries, _ := s.LoadCashflows()
	for _, e := range entries {
		if e.Category != "Groceries" {
			t.Fatalf("expected all in Groceries: %+v", e)
		}
	}

	// Groceries had a budget: Food's went with Food.
	if budgets, _ := s.LoadBudgets(); len(budgets) != 1 || budgets[0].LimitCents != 2 {
		t.Fatalf("unexpected budgets %+v", budgets)
	}

	stts, _ := s.LoadAppSettings()
	for _, c := range stts.ExpenseCategories {
		if c.CategoryName == "Food" {
			t.Fatal("expected Food gone")
		}
	}

	// Accounts: blocked while entries use them, mergeable.
	old := account.Account{Name: "Old", Currency: "$"}
	main := account.Account{Name: "Main", Currency: "$"}
	for _, a := range []*account.Account{&old, &main} {
		if err := s.CreateAccount(a); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.UpsertAccountValueLog(old.ID, time.Now(), 5); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateCashflow(&cashflow.CashflowEntry{Category: "Groceries", AccountName: "Old", EntryDate: time.Now()}); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteAccount(old.ID); !errors.As(err, &inUse) {
		t.Fatalf("expected the account in use: %v", err)
	}

	if moved, err := s.Merge("account", old.ID, main.ID); err != nil || moved != 1 {
		t.Fatalf("unexpected %d %v", moved, err)
	}

	if logs, _ := s.LoadAccountValueLogs(old.ID); len(logs) != 0 {
		t.Fatal("expected the merged account's history gone with it")
	}

	if entries, _ = s.LoadCashflows(); entries[0].AccountName != "Main" {
		t.Fatalf("expected the entry on Main: %+v", entries[0])
	}
}

// An old database names what records use. Opening it links them, after a
// backup, keeping every name.
func TestOldDatabasesAreLinked(t *testing.T) {
	s, path := openTest(t)
	db := s.db

	asOldDatabase(t, db)
	for _, stmt := range []string{
		"ALTER TABLE cashflow_entries ADD COLUMN category text",
		"ALTER TABLE cashflow_entries ADD COLUMN account_name text",
		"ALTER TABLE budgets ADD COLUMN category text NOT NULL DEFAULT ''",
		"ALTER TABLE subscriptions ADD COLUMN payment_method text",
		"ALTER TABLE taxes ADD COLUMN tax_type_id integer",
		"ALTER TABLE taxes ADD COLUMN tax_country text",
		"ALTER TABLE taxes ADD COLUMN tax_type_name text",
		"INSERT INTO setting_expense_categories (category_name, uid, archived) VALUES ('Food', '', 0)",
		"INSERT INTO setting_income_categories (category_name, uid, archived) VALUES ('Salary', '', 0)",
		"INSERT INTO accounts (name, currency, uid, archived) VALUES ('Bank', '$', '', 0)",
		"INSERT INTO setting_tax_types (country, tax_type_name, uid) VALUES ('NL', 'VAT', '')",
		"INSERT INTO cashflow_entries (is_income, currency, amount_cents, category, account_name) VALUES (0, '$', 1, ' food ', 'BANK'), (1, '$', 2, 'Salary', ''), (0, '$', 3, 'Gone', 'Closed'), (1, '€', 4, 'Gone', 'Closed')",
		"INSERT INTO budgets (category, limit_cents) VALUES ('Food', 10), ('', 20)",
		"INSERT INTO subscriptions (name, currency, amount_cents, period, payment_method, is_active) VALUES ('Music', '$', 5, 'month', 'Revolut', 1)",
		"INSERT INTO taxes (tax_type_id, tax_country, tax_type_name, currency, amount_due_cents) VALUES ((SELECT id FROM setting_tax_types WHERE tax_type_name = 'VAT'), 'NL', 'VAT', '$', 100), (999, 'DE', 'Income', '$', 200)",
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	db, _, err := OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	s = NewSQLiteStorage(db)

	backups, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "cents-before-links-*.db"))
	if len(backups) != 1 {
		t.Fatalf("expected a backup next to the database: %v", backups)
	}
	if info, err := os.Stat(backups[0]); err != nil || info.Size() == 0 {
		t.Fatalf("expected the backup written: %v", err)
	}

	entries, _ := s.LoadCashflows()
	got := map[int64][2]string{}
	for _, e := range entries {
		got[e.AmountCents] = [2]string{e.Category, e.AccountName}
	}
	if got[1] != [2]string{"Food", "Bank"} || got[2] != [2]string{"Salary", ""} || got[3] != [2]string{"Gone", "Closed"} || got[4] != [2]string{"Gone", "Closed"} {
		t.Fatalf("unexpected names %v", got)
	}

	// Gone and Closed matched nothing: archived stand-ins, one for each
	// kind of category, and an account kept out of totals.
	stts, _ := s.LoadAppSettings()
	if !hasArchivedExpense(stts, "Gone") || !hasArchivedIncome(stts, "Gone") {
		t.Fatalf("expected archived Gone categories: %+v %+v", stts.ExpenseCategories, stts.IncomeCategories)
	}

	accounts, _ := s.LoadAccounts()
	for _, a := range accounts {
		if a.Name == "Closed" && (!a.Archived || !a.IgnoreInSummaries || a.Currency != "$") {
			t.Fatalf("unexpected stand-in %+v", a)
		}
	}

	if subs, _ := s.LoadSubscriptions(); subs[0].PaymentMethod != "Revolut" {
		t.Fatalf("unexpected subscription %+v", subs[0])
	}

	taxes, _ := s.LoadTaxes()
	names := map[int64]string{}
	for _, x := range taxes {
		names[x.AmountDueCents] = x.TaxCountry + "/" + x.TaxTypeName
	}
	if names[100] != "NL/VAT" || names[200] != "DE/Income" {
		t.Fatalf("unexpected taxes %v", names)
	}

	if budgets, _ := s.LoadBudgets(); budgets[0].Category != "Food" || budgets[1].Category != "" {
		t.Fatalf("unexpected budgets %+v", budgets)
	}

	if needsLinking(db) {
		t.Fatal("expected the name columns gone")
	}
}

func hasArchivedExpense(stts settings.AppSettings, name string) bool {
	for _, c := range stts.ExpenseCategories {
		if c.CategoryName == name && c.Archived && c.UID != "" {
			return true
		}
	}

	return false
}

func hasArchivedIncome(stts settings.AppSettings, name string) bool {
	for _, c := range stts.IncomeCategories {
		if c.CategoryName == name && c.Archived && c.UID != "" {
			return true
		}
	}

	return false
}
