package settings

import "testing"

func TestEntryAndEditRates(t *testing.T) {
	stts := AppSettings{BaseCurrency: "USD", Currencies: []SettingCurrency{{CurrencyName: "EUR", RateToBase: 1.08}}}

	cases := []struct {
		name            string
		currency, typed string
		want            float64
	}{
		{"base always 1", " usd ", "7", 1},
		{"settings rate", "eur", "", 1.08},
		{"typed rate", "EUR", " 1.1 ", 1.1},
		{"no rate known", "BTC", "", 0},
	}

	for _, c := range cases {
		if got, err := stts.EntryRate(c.currency, c.typed); err != nil || got != c.want {
			t.Errorf("%s: got %v %v", c.name, got, err)
		}
	}

	if _, err := stts.EntryRate("EUR", "abc"); err == nil || err.Error() != "rate must be a number" {
		t.Fatalf("expected rate error, got %v", err)
	}

	if got, _ := stts.EditRate("EUR", 1.5, "eur", ""); got != 1.5 {
		t.Fatalf("same currency should keep its rate, got %v", got)
	}

	if got, _ := stts.EditRate("BTC", 9, "EUR", ""); got != 1.08 {
		t.Fatalf("new currency should take the settings rate, got %v", got)
	}

	if got, _ := stts.EditRate("EUR", 1.5, "USD", ""); got != 1 {
		t.Fatalf("base currency should use 1, got %v", got)
	}

	if base, ok := BaseCents(1001, 1.08); !ok || base != 1081 {
		t.Fatalf("unexpected conversion %d %v", base, ok)
	}

	if _, ok := BaseCents(100, 0); ok {
		t.Fatal("no rate means no base amount")
	}
}
