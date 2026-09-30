package budget

import (
	"testing"
	"time"

	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/settings"
)

var stts = settings.AppSettings{
	BaseCurrency:      "€",
	ExpenseCategories: []settings.SettingExpenseCategory{{CategoryName: "Food"}, {CategoryName: "Rent"}, {CategoryName: "Car", Archived: true}},
}

func TestNewAndEdit(t *testing.T) {
	now := time.Now()

	food, err := New(Fields{Category: " food ", Limit: "300"}, stts, nil, now)
	if err != nil || food.Category != "Food" || food.LimitCents != 30000 || food.IsTotal() {
		t.Fatalf("unexpected %+v %v", food, err)
	}
	food.ID = 1

	total, err := New(Fields{Limit: "2000.50"}, stts, []Budget{food}, now)
	if err != nil || !total.IsTotal() || total.LimitCents != 200050 {
		t.Fatalf("unexpected total %+v %v", total, err)
	}
	total.ID = 2
	existing := []Budget{food, total}

	for _, bad := range []Fields{
		{Category: "FOOD", Limit: "10"},
		{Category: "", Limit: "10"},
		{Category: "Travel", Limit: "10"},
		{Category: "Rent", Limit: ""},
		{Category: "Rent", Limit: "0"},
		{Category: "Rent", Limit: "-5"},
		{Category: "Rent", Limit: "abc"},
	} {
		if _, err := New(bad, stts, existing, now); err == nil {
			t.Errorf("expected %+v to fail", bad)
		}
	}

	// An archived category can still get a budget; it's still a category.
	if _, err := New(Fields{Category: "Car", Limit: "10"}, stts, existing, now); err != nil {
		t.Fatal(err)
	}

	// Editing keeps its own category, and may keep one that left settings.
	if edited, err := Edit(food, Fields{Category: "Food", Limit: "350"}, stts, existing, now); err != nil || edited.LimitCents != 35000 || edited.ID != 1 {
		t.Fatalf("unexpected %+v %v", edited, err)
	}

	gone := Budget{ID: 3, Category: "Old"}
	if _, err := Edit(gone, Fields{Category: "old", Limit: "5"}, stts, append(existing, gone), now); err != nil {
		t.Fatal(err)
	}
}

func TestMonth(t *testing.T) {
	entry := func(income bool, cents int64, rate float64, category string, m time.Month) cashflow.CashflowEntry {
		return cashflow.CashflowEntry{IsIncome: income, AmountCents: cents, RateToBase: rate, AmountBaseCents: int64(float64(cents) * rate), Category: category, EntryDate: time.Date(2026, m, 10, 0, 0, 0, 0, time.Local)}
	}

	entries := []cashflow.CashflowEntry{
		entry(false, 20000, 1, "Food", 3),
		entry(false, 10000, 0.5, "food", 3), // 50.00 in base
		entry(false, 90000, 1, "Rent", 3),
		entry(false, 99999, 1, "Food", 2),   // another month
		entry(true, 500000, 1, "Salary", 3), // income
		entry(false, 700, 0, "Taxi", 3),     // no rate
	}

	budgets := []Budget{
		{ID: 1, Category: "Rent", LimitCents: 80000},
		{ID: 2, Category: "Food", LimitCents: 30000},
		{ID: 3, LimitCents: 200000},
		{ID: 4, Category: "Fun", LimitCents: 10000},
	}

	got, missing := Month(budgets, entries, time.Date(2026, 3, 31, 0, 0, 0, 0, time.Local))
	if missing != 1 || len(got) != 4 {
		t.Fatalf("unexpected %+v %d", got, missing)
	}

	want := []struct {
		id    uint
		spent int64
		left  int64
		state string
	}{
		{3, 115000, 85000, StateOK},
		{2, 25000, 5000, StateClose},
		{4, 0, 10000, StateOK},
		{1, 90000, -10000, StateOver},
	}

	for i, w := range want {
		p := got[i]
		if p.Budget.ID != w.id || p.SpentCents != w.spent || p.LeftCents != w.left || p.State != w.state {
			t.Errorf("row %d: want %+v, got %+v", i, w, p)
		}
	}

	if got[1].Percent < 83 || got[1].Percent > 84 {
		t.Fatalf("unexpected percent %v", got[1].Percent)
	}
}
