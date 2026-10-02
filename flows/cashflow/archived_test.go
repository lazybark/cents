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
