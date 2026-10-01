package cashflow

import (
	"testing"
	"time"

	"github.com/lazybark/cents/flows/settings"
)

func day(year int, month time.Month, d int) time.Time {
	return time.Date(year, month, d, 12, 0, 0, 0, time.Local)
}

func TestForMonthTotalsAndOverview(t *testing.T) {
	stts := settings.AppSettings{BaseCurrency: "$", Currencies: []settings.SettingCurrency{{CurrencyName: "EUR", RateToBase: 2}}}
	entries := []CashflowEntry{
		{ID: 1, IsIncome: true, Currency: "$", AmountCents: 1000, EntryDate: day(2026, 7, 3)},
		{ID: 2, IsIncome: false, Currency: "EUR", AmountCents: 100, EntryDate: day(2026, 9, 1)},
		{ID: 3, IsIncome: true, Currency: "$", AmountCents: 5000, EntryDate: day(2026, 9, 20)},
		{ID: 4, IsIncome: false, Currency: "BTC", AmountCents: 7, EntryDate: day(2026, 9, 5)},
	}

	september := ForMonth(entries, day(2026, 9, 15))
	if len(september) != 3 || september[0].ID != 3 || september[1].ID != 4 || september[2].ID != 2 {
		t.Fatalf("expected September entries newest first, got %+v", september)
	}

	income, expense, missing := Totals(september, stts)
	if income != 5000 || expense != 200 || missing != 1 {
		t.Fatalf("unexpected totals %d %d %d", income, expense, missing)
	}

	rows, missing := MonthlyOverview(entries, stts)
	if missing != 1 || len(rows) != 3 {
		t.Fatalf("expected July..September with one missing rate, got %d rows, %d missing", len(rows), missing)
	}

	if rows[1].Month.Month() != time.August || rows[1].NetBase != 0 || rows[1].DeltaFromPrev != -1000 {
		t.Fatalf("expected empty August after July, got %+v", rows[1])
	}

	if rows[2].NetBase != 4800 || rows[2].DeltaFromPrev != 4800 || rows[0].HasPrev {
		t.Fatalf("unexpected rows %+v", rows)
	}
}

func TestNewValidates(t *testing.T) {
	now := time.Now()
	categories := []string{"Salary"}

	if _, err := New(true, "$", 1, now, "Salary", nil, "", "", now); err == nil || err.Error() != "no income categories configured; add one in settings" {
		t.Fatalf("unexpected error %v", err)
	}

	if _, err := New(false, "$", 1, now, "Rent", categories, "", "", now); err == nil {
		t.Fatal("expected unknown category error")
	}

	entry, err := New(true, "$", 100, now, "Salary", categories, " Main ", " note ", now)
	if err != nil || entry.AccountName != "Main" || entry.Comment != "note" || !entry.IsIncome {
		t.Fatalf("unexpected entry %+v, %v", entry, err)
	}
}
