package desktop

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lazybark/cents/rates"
	storage "github.com/lazybark/cents/storage/sqlite"
)

// newEmptyAPI is a brand new database, as on first launch.
func newEmptyAPI(t *testing.T, fetcher *fakeRates) *API {
	t.Helper()

	db, _, err := storage.OpenDatabase(filepath.Join(t.TempDir(), "cents.db"))
	if err != nil {
		t.Fatal(err)
	}

	api := newAPI(Options{Storage: storage.NewSQLiteStorage(db)})
	api.rates = fetcher

	return api
}

func euroQuote() rates.Quote {
	return rates.Quote{Source: "test", Date: "2026-10-02", PerBase: map[string]float64{"USD": 1.25, "GEL": 3.2, "GBP": 0.8}}
}

func TestSetupCurrencies(t *testing.T) {
	fetcher := &fakeRates{quote: euroQuote()}
	api := newEmptyAPI(t, fetcher)

	if !api.Status().NeedsCurrencies {
		t.Fatal("a new database needs currencies")
	}

	for want, input := range map[string]CurrencySetupInput{
		"pick at least one currency":                    {Base: "EUR"},
		`unknown currency "shells"`:                     {Codes: []string{"EUR", "shells"}, Base: "EUR"},
		"pick which of your currencies is the base one": {Codes: []string{"EUR", "USD"}, Base: "GEL"},
	} {
		if _, err := api.SetupCurrencies(input); err == nil || err.Error() != want {
			t.Errorf("want %q, got %v", want, err)
		}
	}

	status, err := api.SetupCurrencies(CurrencySetupInput{Codes: []string{"eur", "USD", "gel", "usd"}, Base: "eur"})
	if err != nil || !status.Auto || status.BaseCode != "EUR" || status.Linked != 2 || status.UpdatedAt == "" || status.Source != "test" {
		t.Fatalf("setup: %+v %v", status, err)
	}

	if api.Status().NeedsCurrencies {
		t.Fatal("set up now")
	}

	view := mustSettings(t, api)
	if view.BaseCurrency != "EUR" || view.BaseCodeName != "Euro" || len(view.Currencies) != 2 {
		t.Fatalf("unexpected settings %+v", view)
	}

	byName := map[string]CurrencySetting{}
	for _, c := range view.Currencies {
		byName[c.Name] = c
	}

	if usd := byName["USD"]; usd.RateToBase != 0.8 || !usd.Linked || usd.Code != "USD" || usd.CodeName != "US Dollar" {
		t.Fatalf("unexpected USD %+v", usd)
	}

	if gel := byName["GEL"]; gel.RateToBase != 0.3125 {
		t.Fatalf("unexpected GEL %+v", gel)
	}

	// New records offer exactly the picked currencies.
	month, _ := api.CashflowMonth("")
	if strings.Join(month.Options.Currencies, ",") != "EUR,GEL,USD" {
		t.Fatalf("unexpected options %v", month.Options.Currencies)
	}
}

func TestSetupCurrenciesOffline(t *testing.T) {
	api := newEmptyAPI(t, &fakeRates{err: errors.New("no network")})

	status, err := api.SetupCurrencies(CurrencySetupInput{Codes: []string{"EUR", "USD"}, Base: "EUR"})
	if err != nil || status.UpdatedAt != "" || !strings.Contains(status.LastError, "no network") {
		t.Fatalf("setup should work offline and say why rates are missing: %+v %v", status, err)
	}

	if usd := mustSettings(t, api).Currencies[0]; usd.RateToBase != 0 || !usd.Linked {
		t.Fatalf("USD should wait for a rate: %+v", usd)
	}
}

func TestSkipCurrencySetup(t *testing.T) {
	api := newEmptyAPI(t, &fakeRates{})
	if err := api.SkipCurrencySetup(); err != nil || api.Status().NeedsCurrencies {
		t.Fatalf("skip: %v", err)
	}
}

func TestRefreshRatesAndLinkedCurrencies(t *testing.T) {
	fetcher := &fakeRates{quote: euroQuote()}
	api := newEmptyAPI(t, fetcher)
	if _, err := api.SetupCurrencies(CurrencySetupInput{Codes: []string{"EUR", "USD"}, Base: "EUR"}); err != nil {
		t.Fatal(err)
	}

	// A linked currency's rate is fetched when left empty; a typed one wins.
	if err := api.SaveCurrency(CurrencyInput{Code: "GBP"}); err != nil {
		t.Fatal(err)
	}

	if err := api.SaveCurrency(CurrencyInput{Code: "GEL", Name: "₾", Rate: "0.3"}); err != nil {
		t.Fatal(err)
	}

	if err := api.SaveCurrency(CurrencyInput{Name: "Shells", Rate: "2"}); err != nil {
		t.Fatal(err)
	}

	for want, input := range map[string]CurrencyInput{
		"EUR is the base currency":                       {Code: "EUR"},
		`USD is already in the list as "USD"`:            {Code: "USD", Name: "US$"},
		`unknown currency "XYZ": pick one from the list`: {Code: "XYZ"},
	} {
		if err := api.SaveCurrency(input); err == nil || err.Error() != want {
			t.Errorf("want %q, got %v", want, err)
		}
	}

	rates := func() map[string]float64 {
		out := map[string]float64{}
		for _, c := range mustSettings(t, api).Currencies {
			out[c.Name] = c.RateToBase
		}
		return out
	}

	if got := rates(); got["GBP"] != 1.25 || got["₾"] != 0.3 || got["Shells"] != 2 {
		t.Fatalf("unexpected rates %v", got)
	}

	fetcher.quote.PerBase = map[string]float64{"USD": 1, "GEL": 2, "GBP": 0.5}
	status, err := api.RefreshRates()
	if err != nil || len(status.Unlinked) != 1 || status.Unlinked[0] != "Shells" || status.Linked != 3 {
		t.Fatalf("refresh: %+v %v", status, err)
	}

	if got := rates(); got["USD"] != 1 || got["₾"] != 0.5 || got["GBP"] != 2 || got["Shells"] != 2 {
		t.Fatalf("refresh should update linked currencies only: %v", got)
	}

	// Just refreshed: nothing is due, so the hourly check doesn't fetch.
	calls := fetcher.calls
	api.refreshIfDue(context.Background())
	if fetcher.calls != calls {
		t.Fatal("rates fetched again within a day")
	}

	if status, _ := api.SetRatesAuto(false); status.Auto {
		t.Fatal("expected auto off")
	}

	fetcher.err = errors.New("down")
	if _, err := api.RefreshRates(); err == nil || !strings.Contains(err.Error(), "down") {
		t.Fatalf("expected the fetch error, got %v", err)
	}

	if status, _ := api.RatesStatus(); !strings.Contains(status.LastError, "down") {
		t.Fatalf("status should keep the error: %+v", status)
	}

	if err := api.SaveCurrency(CurrencyInput{Code: "JPY"}); err == nil || !strings.Contains(err.Error(), "couldn't fetch the rate, so type one") {
		t.Fatalf("expected a fetch error asking for a rate, got %v", err)
	}
}

func TestRefreshNeedsALinkedBase(t *testing.T) {
	api := newEmptyAPI(t, &fakeRates{quote: euroQuote()})
	if _, err := api.RefreshRates(); err == nil || !strings.Contains(err.Error(), "isn't linked") {
		t.Fatalf("the default $ base isn't linked: %v", err)
	}
}
