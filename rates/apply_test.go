package rates

import (
	"testing"
	"time"

	"github.com/lazybark/cents/flows/settings"
)

func TestApply(t *testing.T) {
	stts := settings.AppSettings{
		BaseCurrency:     "€",
		BaseCurrencyCode: "EUR",
		Currencies: []settings.SettingCurrency{
			{ID: 1, CurrencyName: "$", RateToBase: 0.88},
			{ID: 2, CurrencyName: "$US", Code: "USD", RateToBase: 0.5},
			{ID: 3, CurrencyName: "₾", RateToBase: 0.3},
			{ID: 4, CurrencyName: "KPW", RateToBase: 0.001},
		},
	}
	quote := Quote{Base: "EUR", Source: "test", Date: "2026-10-02", PerBase: map[string]float64{"USD": 1.25, "GEL": 3.2}}
	now := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)

	updated, records, missing := Apply(stts, quote, now)
	if len(updated) != 2 || updated[0].ID != 2 || updated[0].RateToBase != 0.8 || updated[1].ID != 3 || updated[1].RateToBase != 0.3125 {
		t.Fatalf("unexpected updates %+v", updated)
	}

	if len(missing) != 1 || missing[0] != "KPW" {
		t.Fatalf("unexpected missing %v", missing)
	}

	if len(records) != 3 || records[0].SettingValue != "2026-10-02T09:00:00Z" || records[1].SettingValue != "test" {
		t.Fatalf("unexpected records %+v", records)
	}
}
