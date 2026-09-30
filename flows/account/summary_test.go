package account

import (
	"testing"

	"github.com/lazybark/cents/flows/settings"
)

func TestTotalsByCurrency(t *testing.T) {
	stts := settings.AppSettings{
		BaseCurrency: "€",
		Currencies:   []settings.SettingCurrency{{CurrencyName: "USD", RateToBase: 0.9}},
	}
	accounts := []Account{
		{Currency: "€", BalanceCents: 100000},
		{Currency: "USD", BalanceCents: 50000},
		{Currency: "usd", BalanceCents: 25000},
		{Currency: "BTC", BalanceCents: 300},
		{Currency: "USD", BalanceCents: 99999, IgnoreInSummaries: true},
	}

	got := TotalsByCurrency(accounts, stts)
	want := []CurrencyTotal{
		{Currency: "BTC", Cents: 300},
		{Currency: "USD", Cents: 75000, BaseCents: 67500, HasRate: true},
	}

	if len(got) != len(want) {
		t.Fatalf("expected %d totals, got %+v", len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("total %d: expected %+v, got %+v", i, want[i], got[i])
		}
	}
}
