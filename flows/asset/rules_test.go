package asset

import (
	"testing"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/settings"
)

func TestNewAndEditValidate(t *testing.T) {
	now := time.Now()
	cases := map[string]Fields{
		"name is required":                         {Type: "Car", Value: "1"},
		"type is required":                         {Name: "Golf", Value: "1"},
		"value error: amount must be a number":     {Name: "Golf", Type: "Car", Value: ""},
		"cost error: amount cannot be negative":    {Name: "Golf", Type: "Car", Value: "1", Cost: "-1"},
		"acquired date must use YYYY-MM-DD format": {Name: "Golf", Type: "Car", Value: "1", AcquiredAt: "1.1.2020"},
	}

	for want, f := range cases {
		if _, err := New(KindProperty, "$", f, dates.ISO, now); err == nil || err.Error() != want {
			t.Errorf("want %q, got %v", want, err)
		}
	}

	if _, err := New(KindProperty, " ", Fields{Name: "a", Type: "b", Value: "1"}, dates.ISO, now); err == nil || err.Error() != "currency is required" {
		t.Fatalf("expected currency error, got %v", err)
	}

	car, err := New(KindProperty, " EUR ", Fields{Name: " Golf ", Type: " Car ", Value: "8000", Cost: "", AcquiredAt: "2020-05-01", Description: " old "}, dates.ISO, now)
	if err != nil || car.Kind != "property" || car.Currency != "EUR" || car.Name != "Golf" || car.Type != "Car" || car.ValueCents != 800000 || car.CostCents != 0 || car.AcquiredAt == nil || car.Description != "old" {
		t.Fatalf("unexpected asset %+v %v", car, err)
	}

	if _, ok := car.GainCents(); ok {
		t.Fatal("no cost means no gain")
	}

	car.ID = 3
	edited, err := car.Edit(Fields{Name: "Golf", Type: "Car", Value: "7000", Cost: "12000"}, dates.ISO, now)
	if gain, ok := edited.GainCents(); err != nil || edited.ID != 3 || edited.Currency != "EUR" || !ok || gain != -500000 {
		t.Fatalf("unexpected edit %+v %v", edited, err)
	}

	if _, err := ParseKind("boat"); err == nil {
		t.Fatal("expected unknown kind")
	}
}

func TestTotalAndFilter(t *testing.T) {
	stts := settings.AppSettings{BaseCurrency: "$", Currencies: []settings.SettingCurrency{{CurrencyName: "EUR", RateToBase: 2}}}
	items := []Asset{
		{Kind: "investment", Name: "b fund", Type: "ETFs", Currency: "$", ValueCents: 1000, CostCents: 800},
		{Kind: "investment", Name: "A shares", Type: "Stocks", Currency: "EUR", ValueCents: 500},
		{Kind: "investment", Name: "c etf", Type: "etfs", Currency: "EUR", ValueCents: 100, CostCents: 150},
		{Kind: "investment", Name: "coins", Type: "Crypto", Currency: "BTC", ValueCents: 1},
		{Kind: "investment", Name: "pension", Type: "Pension fund", Currency: "$", ValueCents: 5, IgnoreInNetWorth: true},
		{Kind: "property", Name: "flat", Type: "Apartment", Currency: "$", ValueCents: 99},
	}

	investments := Filter(items, KindInvestment)
	if len(investments) != 5 || investments[0].Name != "A shares" || investments[1].Name != "b fund" {
		t.Fatalf("unexpected filter %+v", investments)
	}

	totals := Total(investments, stts)
	if totals.ValueCents != 1000+1000+200 || totals.CostCents != 800+300 || totals.WithCostCents != 1200 || totals.MissingRates != 1 || totals.Ignored != 1 {
		t.Fatalf("unexpected totals %+v", totals)
	}

	if len(totals.ByType) != 2 || totals.ByType[0].Type != "ETFs" || totals.ByType[0].Count != 2 || totals.ByType[0].BaseCents != 1200 {
		t.Fatalf("types should merge ignoring case, biggest first: %+v", totals.ByType)
	}
}
