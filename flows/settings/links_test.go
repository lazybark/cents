package settings

import (
	"testing"
	"time"
)

func TestLinksAndDue(t *testing.T) {
	if got := (SettingCurrency{CurrencyName: "₾"}).LinkedCode(); got != "GEL" {
		t.Fatalf("symbol should link, got %q", got)
	}

	if got := (SettingCurrency{CurrencyName: "$"}).LinkedCode(); got != "" {
		t.Fatalf("$ is ambiguous, got %q", got)
	}

	if got := (SettingCurrency{CurrencyName: "$", Code: "usd"}).LinkedCode(); got != "USD" {
		t.Fatalf("stored code should win, got %q", got)
	}

	if got := BaseCodeFor("$", "USD", "$"); got != "USD" {
		t.Fatalf("stored base link, got %q", got)
	}

	if got := BaseCodeFor("€", "USD", "$"); got != "EUR" {
		t.Fatalf("a link made for another label shouldn't count, got %q", got)
	}

	now := time.Now()
	stts := AppSettings{BaseCurrencyCode: "EUR", Rates: RatesState{Auto: true, UpdatedAt: now.Add(-25 * time.Hour)}}
	if !stts.RatesDue(now) {
		t.Fatal("a day-old fetch is due")
	}

	stts.Rates.UpdatedAt = now.Add(-time.Hour)
	if stts.RatesDue(now) {
		t.Fatal("an hour-old fetch isn't due")
	}

	stts.Rates = RatesState{Auto: true}
	if !stts.RatesDue(now) {
		t.Fatal("never fetched is due")
	}

	stts.BaseCurrencyCode = ""
	if stts.RatesDue(now) {
		t.Fatal("can't fetch without a linked base")
	}

	renamed, err := (SettingCurrency{ID: 1, CurrencyName: "$", Code: "USD"}).Apply("CAD", "0.7", now)
	if err != nil || renamed.Code != "" || renamed.LinkedCode() != "CAD" {
		t.Fatalf("renaming should drop the old link: %+v %v", renamed, err)
	}

	kept, _ := (SettingCurrency{ID: 1, CurrencyName: "$", Code: "USD"}).Apply("$", "0.9", now)
	if kept.Code != "USD" {
		t.Fatalf("a new rate shouldn't drop the link: %+v", kept)
	}
}
