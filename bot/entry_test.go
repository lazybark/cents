package bot

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

var testChoices = choices{
	Base:       "€",
	Currencies: map[string]string{"€": "€", "eur": "€", "usd": "USD", "$": "USD", "gel": "GEL"},
	Income:     []string{"Salary", "Side job"},
	Expense:    []string{"Food", "Eating out", "Rent", "Eating in"},
	Accounts:   []string{"TBC Business", "Wallet"},
}

func TestParseEntry(t *testing.T) {
	today := time.Date(2026, 10, 8, 15, 0, 0, 0, time.Local)

	for text, want := range map[string]entry{
		"12.50 food lunch":           {Amount: "12.50", Currency: "€", Category: "Food", Comment: "lunch", Date: "2026-10-08"},
		"12,5 Food":                  {Amount: "12.5", Currency: "€", Category: "Food", Date: "2026-10-08"},
		"+3000 salary october":       {IsIncome: true, Amount: "3000", Currency: "€", Category: "Salary", Comment: "october", Date: "2026-10-08"},
		"15 usd rent #home #fixed":   {Amount: "15", Currency: "USD", Category: "Rent", Tags: []string{"home", "fixed"}, Date: "2026-10-08"},
		"$15 rent":                   {Amount: "15", Currency: "USD", Category: "Rent", Date: "2026-10-08"},
		"40€ eating out dinner @tbc": {Amount: "40", Currency: "€", Category: "Eating out", Comment: "dinner", Account: "TBC Business", Date: "2026-10-08"},
		"40 eating in yesterday":     {Amount: "40", Currency: "€", Category: "Eating in", Date: "2026-10-07"},
		"9 food 2026-09-30":          {Amount: "9", Currency: "€", Category: "Food", Date: "2026-09-30"},
		"9 food 5.10":                {Amount: "9", Currency: "€", Category: "Food", Date: "2026-10-05"},
		"9 food 24.12":               {Amount: "9", Currency: "€", Category: "Food", Date: "2025-12-24"},
		"9 fo bread":                 {Amount: "9", Currency: "€", Category: "Food", Comment: "bread", Date: "2026-10-08"},
		"9 coffee with Ann":          {Amount: "9", Currency: "€", Comment: "coffee with Ann", Date: "2026-10-08"},
		"9 eat":                      {Amount: "9", Currency: "€", Comment: "eat", Date: "2026-10-08"},
		"-7 gel food":                {Amount: "7", Currency: "GEL", Category: "Food", Date: "2026-10-08"},
		"+50 side":                   {IsIncome: true, Amount: "50", Currency: "€", Category: "Side job", Date: "2026-10-08"},
		"+50 food":                   {IsIncome: true, Amount: "50", Currency: "€", Comment: "food", Date: "2026-10-08"},
	} {
		got, err := parseEntry(text, testChoices, today)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %+v %v, want %+v", text, got, err, want)
		}
	}

	for text, want := range map[string]string{
		"hello":         "not an entry",
		"":              "not an entry",
		"food 12":       "not an entry",
		"0 food":        "above zero",
		"12btc food":    `unknown currency "btc"`,
		"12 food @nope": `no account starts with "nope"`,
	} {
		_, err := parseEntry(text, testChoices, today)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: expected %q, got %v", text, want, err)
		}
	}

	if _, err := parseEntry("12 rent @t", choices{Accounts: []string{"TBC", "Travel card"}}, today); err == nil || !strings.Contains(err.Error(), "TBC or Travel card") {
		t.Errorf("expected the ambiguous account named: %v", err)
	}

	if _, err := parseEntry("hi there", testChoices, today); !errors.Is(err, errNotEntry) {
		t.Errorf("expected not an entry: %v", err)
	}
}
