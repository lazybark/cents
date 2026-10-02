package cashflow

import (
	"testing"

	"github.com/lazybark/cents/flows/settings"
)

func TestWithoutArchived(t *testing.T) {
	stts := settings.AppSettings{ExpenseCategories: []settings.SettingExpenseCategory{{CategoryName: "Car", Archived: true}}}
	entries := []CashflowEntry{
		{ID: 1, Category: "car"},
		{ID: 2, Category: "Rent"},
		{ID: 3, Category: "Car", IsIncome: true},
	}

	kept, left := WithoutArchived(entries, stts)
	if left != 1 || len(kept) != 2 || kept[0].ID != 2 || kept[1].ID != 3 {
		t.Fatalf("only the expense in the archived category should go: %+v %d", kept, left)
	}
}

func TestNewRefusesArchivedCategories(t *testing.T) {
	stts := settings.AppSettings{IncomeCategories: []settings.SettingIncomeCategory{{CategoryName: "Salary"}, {CategoryName: "Side gig", Archived: true}}}

	if _, err := New(true, "$", 1, 1, day(2026, 1, 1), "Side gig", stts.IncomeCategoryOptions(), "", "", day(2026, 1, 1)); err == nil || err.Error() != `unknown category "Side gig"` {
		t.Fatalf("expected archived category to be refused, got %v", err)
	}
}

func TestEditKeepsItsOwnCategory(t *testing.T) {
	now := day(2026, 3, 1)
	entry := CashflowEntry{ID: 7, CreatedAt: day(2026, 1, 1), Category: "Car", Currency: "$", AmountCents: 100}

	edited, err := entry.Edit(false, "$", 250, 1, day(2026, 3, 2), " car ", []string{"Rent"}, "", "fixed", now)
	if err != nil || edited.ID != 7 || !edited.CreatedAt.Equal(entry.CreatedAt) || edited.Category != "Car" || edited.AmountCents != 250 || edited.Comment != "fixed" {
		t.Fatalf("keeping an archived category should work: %+v %v", edited, err)
	}

	if _, err := entry.Edit(false, "$", 250, 1, now, "Boat", []string{"Rent"}, "", "", now); err == nil || err.Error() != `unknown category "Boat"` {
		t.Fatalf("a new category must be a current one, got %v", err)
	}

	// Switching to income needs an income category.
	if _, err := entry.Edit(true, "$", 250, 1, now, "Car", []string{"Salary"}, "", "", now); err == nil {
		t.Fatal("an expense category isn't an income one")
	}

	moved, err := entry.Edit(true, "$", 250, 1, now, "Salary", []string{"Salary"}, "", "", now)
	if err != nil || !moved.IsIncome || moved.Category != "Salary" {
		t.Fatalf("switching to income: %+v %v", moved, err)
	}
}
