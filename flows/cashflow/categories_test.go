package cashflow

import (
	"testing"
	"time"
)

func TestByCategory(t *testing.T) {
	entries := []CashflowEntry{
		CashflowEntry{Category: "Rent", AmountCents: 1000, EntryDate: day(2026, 1, 3)}.withRate(1),
		CashflowEntry{Category: "Food", AmountCents: 200, EntryDate: day(2026, 1, 9)}.withRate(1),
		CashflowEntry{Category: " food ", AmountCents: 100, EntryDate: day(2026, 1, 20)}.withRate(2),
		CashflowEntry{Category: "Rent", AmountCents: 1000, EntryDate: day(2026, 3, 3)}.withRate(1),
		CashflowEntry{Category: "Gift", AmountCents: 5, EntryDate: day(2026, 4, 1)}.withRate(0),
		CashflowEntry{Category: "Salary", AmountCents: 9999, EntryDate: day(2025, 1, 1), IsIncome: true}.withRate(1),
	}

	months, series, missing := ByCategory(entries, false)
	if len(months) != 3 || months[0] != MonthStart(day(2026, 1, 1)) || months[2].Month() != time.March || missing != 1 {
		t.Fatalf("expected January..March and one missing rate, got %v %d", months, missing)
	}

	if len(series) != 2 || series[0].Category != "Rent" || series[1].Category != "Food" {
		t.Fatalf("expected Rent then Food, got %+v", series)
	}

	food := series[1]
	if food.Values[0] != 400 || food.Entries[0] != 2 || food.Values[1] != 0 || food.Values[2] != 0 {
		t.Fatalf("food should merge case and spaces, at each entry's rate: %+v", food)
	}

	if rent := series[0]; rent.Values[1] != 0 || rent.Values[2] != 1000 || rent.Entries[2] != 1 {
		t.Fatalf("unexpected rent %+v", rent)
	}

	months, series, _ = ByCategory(entries, true)
	if len(months) != 1 || len(series) != 1 || series[0].Values[0] != 9999 {
		t.Fatalf("unexpected incomes %v %+v", months, series)
	}

	if months, series, _ := ByCategory(nil, true); months != nil || series != nil {
		t.Fatal("expected nothing for no entries")
	}
}
