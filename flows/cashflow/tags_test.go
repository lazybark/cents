package cashflow

import (
	"testing"
	"time"
)

func TestByTag(t *testing.T) {
	day := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC) }
	entries := []CashflowEntry{
		{Category: "Flights", AmountCents: 80000, RateToBase: 1, AmountBaseCents: 80000, Tags: []string{"Trip to Japan"}, EntryDate: day(3, 1)},
		{Category: "Food", AmountCents: 5000, RateToBase: 0.5, AmountBaseCents: 2500, Tags: []string{"trip to japan", "Food abroad"}, EntryDate: day(4, 10)},
		{Category: "Food", AmountCents: 3000, RateToBase: 1, AmountBaseCents: 3000, Tags: []string{"Trip to Japan"}, EntryDate: day(4, 12)},
		{Category: "Refund", IsIncome: true, AmountCents: 10000, RateToBase: 1, AmountBaseCents: 10000, Tags: []string{"Trip to Japan"}, EntryDate: day(5, 2)},
		{Category: "Food", AmountCents: 999, Tags: []string{"Food abroad"}, EntryDate: day(1, 5)},   // no rate
		{Category: "Rent", AmountCents: 1, RateToBase: 1, AmountBaseCents: 1, EntryDate: day(6, 1)}, // untagged
	}

	got := ByTag(entries)
	if len(got) != 2 || got[0].Tag != "Trip to Japan" {
		t.Fatalf("expected Japan first (used last): %+v", got)
	}

	japan := got[0]
	if japan.Count != 4 || japan.ExpenseCents != 85500 || japan.IncomeCents != 10000 || !japan.First.Equal(day(3, 1)) || !japan.Last.Equal(day(5, 2)) {
		t.Fatalf("unexpected totals %+v", japan)
	}

	if len(japan.Categories) != 3 || japan.Categories[0].Category != "Flights" || japan.Categories[1].Category != "Refund" || !japan.Categories[1].IsIncome || japan.Categories[2].Cents != 5500 {
		t.Fatalf("unexpected categories %+v", japan.Categories)
	}

	if food := got[1]; food.Count != 2 || food.MissingRates != 1 || food.ExpenseCents != 2500 {
		t.Fatalf("unexpected %+v", food)
	}
}
