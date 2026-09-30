package desktop

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/settings"
	storage "github.com/lazybark/cents/storage/sqlite"
)

func newTestAPI(t *testing.T) *API {
	t.Helper()

	db, _, err := storage.OpenDatabase(filepath.Join(t.TempDir(), "cents.db"))
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Create(&settings.SettingCurrency{CurrencyName: "EUR", RateToBase: 1.08}).Error; err != nil {
		t.Fatal(err)
	}

	return newAPI(Options{Storage: storage.NewSQLiteStorage(db)})
}

func mustCreate(t *testing.T, api *API, input NewAccountInput) AccountRow {
	t.Helper()

	if err := api.CreateAccount(input); err != nil {
		t.Fatal(err)
	}

	overview, err := api.Accounts(int(account.SortName))
	if err != nil {
		t.Fatal(err)
	}

	for _, row := range overview.Accounts {
		if row.Name == strings.TrimSpace(input.Name) {
			return row
		}
	}

	t.Fatalf("created account %q not listed", input.Name)

	return AccountRow{}
}

func TestAccountsSumsConvertibleAccountsInBaseCurrency(t *testing.T) {
	api := newTestAPI(t)
	mustCreate(t, api, NewAccountInput{Name: "Checking", Description: "main", Currency: "$", Amount: "4523.10"})
	mustCreate(t, api, NewAccountInput{Name: "Savings", Description: "eu", Currency: "eur", Amount: "12000"})
	mustCreate(t, api, NewAccountInput{Name: "Shared", Description: "meta", Currency: "$", Amount: "300", IgnoreInSummaries: true})

	overview, err := api.Accounts(int(account.SortBaseAmount))
	if err != nil {
		t.Fatal(err)
	}

	if overview.TotalCents != 452310+1296000 {
		t.Fatalf("expected total 1748310, got %d", overview.TotalCents)
	}
	if overview.IgnoredCount != 1 {
		t.Fatalf("expected 1 ignored account, got %d", overview.IgnoredCount)
	}
	if got := overview.Accounts[0]; got.Name != "Savings" || got.Currency != "EUR" || got.BaseCents != 1296000 {
		t.Fatalf("expected EUR savings first with canonical currency, got %+v", got)
	}
	if len(overview.CurrencyTotals) != 1 || overview.CurrencyTotals[0] != (CurrencyTotal{Currency: "EUR", Cents: 1200000, BaseCents: 1296000, HasRate: true}) {
		t.Fatalf("expected one EUR subtotal, got %+v", overview.CurrencyTotals)
	}
	if strings.Join(overview.Currencies, ",") != "$,EUR" {
		t.Fatalf("unexpected currency options %v", overview.Currencies)
	}
}

func TestCreateAccountValidatesLikeTUI(t *testing.T) {
	api := newTestAPI(t)

	cases := map[string]NewAccountInput{
		"name is required":        {Description: "d", Currency: "$", Amount: "1"},
		"description is required": {Name: "n", Currency: "$", Amount: "1"},
		"amount cannot be":        {Name: "n", Description: "d", Currency: "$", Amount: "-5"},
		"amount must be a number": {Name: "n", Description: "d", Currency: "$", Amount: "abc"},
		"unknown currency":        {Name: "n", Description: "d", Currency: "BTC", Amount: "1"},
	}

	for want, input := range cases {
		if err := api.CreateAccount(input); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("expected error containing %q, got %v", want, err)
		}
	}
}

func TestUpdateAmountAndValueLogs(t *testing.T) {
	api := newTestAPI(t)
	row := mustCreate(t, api, NewAccountInput{Name: "Checking", Description: "main", Currency: "$", Amount: "10"})

	result, err := api.UpdateAccountAmount(AmountUpdateInput{ID: row.ID, Amount: "25.50", IgnoreInSummaries: true, UpdateLog: true})
	if err != nil || result.Warning != "" {
		t.Fatalf("update failed: %v %q", err, result.Warning)
	}

	overview, err := api.Accounts(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := overview.Accounts[0]; got.BalanceCents != 2550 || !got.IgnoreInSummaries {
		t.Fatalf("expected updated amount and ignore flag, got %+v", got)
	}

	if err := api.SaveAccountValueLog(ValueLogInput{AccountID: row.ID, Date: "2026-01-15", Value: "7"}); err != nil {
		t.Fatal(err)
	}
	if err := api.SaveAccountValueLog(ValueLogInput{AccountID: row.ID, Date: "2026-01-15", Value: "8"}); err != nil {
		t.Fatal(err)
	}

	logs, err := api.AccountValueLogs(row.ID)
	if err != nil {
		t.Fatal(err)
	}

	today := time.Now().Format(logDateLayout)
	if len(logs) != 2 || logs[0].Date != today || logs[0].ValueCents != 2550 || logs[1].Date != "2026-01-15" || logs[1].ValueCents != 800 {
		t.Fatalf("expected today's log and one replaced 2026-01-15 log, got %+v", logs)
	}

	if err := api.SaveAccountValueLog(ValueLogInput{AccountID: row.ID, Date: "15.01.2026", Value: "1"}); err == nil {
		t.Fatal("expected error for wrong date format")
	}
}

func TestDeleteAccountRemovesItAndItsLogs(t *testing.T) {
	api := newTestAPI(t)
	row := mustCreate(t, api, NewAccountInput{Name: "Old", Description: "gone", Currency: "$", Amount: "1"})
	if err := api.SaveAccountValueLog(ValueLogInput{AccountID: row.ID, Date: "2026-01-15", Value: "1"}); err != nil {
		t.Fatal(err)
	}

	if err := api.DeleteAccount(row.ID); err != nil {
		t.Fatal(err)
	}

	overview, err := api.Accounts(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(overview.Accounts) != 0 {
		t.Fatalf("expected no accounts, got %+v", overview.Accounts)
	}

	logs, err := api.AccountValueLogs(row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 0 {
		t.Fatalf("expected logs deleted, got %+v", logs)
	}

	if err := api.DeleteAccount(row.ID); err != errAccountNotFound {
		t.Fatalf("expected not found on second delete, got %v", err)
	}
}

func TestAccountsWithoutDatabaseFails(t *testing.T) {
	api := newAPI(Options{})

	if api.Status().Ready {
		t.Fatal("expected status not ready without storage")
	}
	if _, err := api.Accounts(0); err == nil {
		t.Fatal("expected error without storage")
	}
}

func TestOverviewAndSettings(t *testing.T) {
	api := newTestAPI(t)
	api.dbPath = "/tmp/cents.db"
	mustCreate(t, api, NewAccountInput{Name: "Checking", Description: "main", Currency: "$", Amount: "100"})
	mustCreate(t, api, NewAccountInput{Name: "Savings", Description: "eu", Currency: "EUR", Amount: "50"})

	overview, err := api.Overview()
	if err != nil {
		t.Fatal(err)
	}
	if overview.Accounts != 10000+5400 || overview.NetWorth != overview.Accounts || overview.BaseCurrency != "$" {
		t.Fatalf("unexpected overview %+v", overview)
	}
	if now := time.Now(); overview.Month.Year() != now.Year() || overview.Month.Month() != now.Month() || overview.Month.Day() != 1 {
		t.Fatalf("expected current month, got %v", overview.Month)
	}

	view, err := api.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if view.DBPath != "/tmp/cents.db" || view.BaseCurrency != "$" || len(view.Currencies) != 1 || view.Currencies[0] != (CurrencyRate{Name: "EUR", RateToBase: 1.08}) {
		t.Fatalf("unexpected settings %+v", view)
	}
}
