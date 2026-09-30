package desktop

import (
	"testing"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/settings"
)

type fakeStorage struct {
	accounts []account.Account
	settings settings.AppSettings
}

func (f fakeStorage) LoadAccounts() ([]account.Account, error) {
	return f.accounts, nil
}

func (f fakeStorage) LoadAppSettings() (settings.AppSettings, error) {
	return f.settings, nil
}

func TestBalanceSumsConvertibleAccountsInBaseCurrency(t *testing.T) {
	api := newAPI(Options{Storage: fakeStorage{
		settings: settings.AppSettings{
			BaseCurrency: "$",
			Currencies:   []settings.SettingCurrency{{CurrencyName: "EUR", RateToBase: 1.08}},
		},
		accounts: []account.Account{
			{Name: "Checking", Currency: "$", BalanceCents: 452310},
			{Name: "Savings", Currency: "eur", BalanceCents: 1200000},
			{Name: "Crypto", Currency: "BTC", BalanceCents: 50000},
			{Name: "Card", Currency: "$", BalanceCents: -83250},
			{Name: "Shared", Currency: "$", BalanceCents: 30000, IgnoreInSummaries: true},
		},
	}})

	balance, err := api.Balance()
	if err != nil {
		t.Fatal(err)
	}

	if balance.BaseCurrency != "$" {
		t.Fatalf("expected base currency $, got %q", balance.BaseCurrency)
	}
	if balance.TotalCents != 1665060 {
		t.Fatalf("expected total 1665060, got %d", balance.TotalCents)
	}
	if balance.IgnoredCount != 1 {
		t.Fatalf("expected 1 ignored account, got %d", balance.IgnoredCount)
	}
	if balance.MissingRates != 1 {
		t.Fatalf("expected 1 account missing a rate, got %d", balance.MissingRates)
	}
	if len(balance.Accounts) != 5 {
		t.Fatalf("expected 5 accounts, got %d", len(balance.Accounts))
	}
	if got := balance.Accounts[1]; !got.HasRate || got.BaseCents != 1296000 {
		t.Fatalf("expected EUR savings converted to 1296000, got %+v", got)
	}
	if got := balance.Accounts[2]; got.HasRate {
		t.Fatalf("expected BTC account to have no rate, got %+v", got)
	}
}

func TestBalanceWithoutDatabaseFails(t *testing.T) {
	api := newAPI(Options{})

	if api.Status().Ready {
		t.Fatal("expected status not ready without storage")
	}
	if _, err := api.Balance(); err == nil {
		t.Fatal("expected error without storage")
	}
}
