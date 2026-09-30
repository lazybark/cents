package settings

import "testing"

func TestArchivedCategoriesAreLeftOutOfOptions(t *testing.T) {
	stts := AppSettings{
		IncomeCategories:  []SettingIncomeCategory{{CategoryName: "Salary"}, {CategoryName: "Side gig", Archived: true}},
		ExpenseCategories: []SettingExpenseCategory{{CategoryName: "Car", Archived: true}, {CategoryName: "Rent"}},
	}

	if got := stts.IncomeCategoryOptions(); len(got) != 1 || got[0] != "Salary" {
		t.Fatalf("unexpected income options %v", got)
	}

	if got := stts.ExpenseCategoryOptions(); len(got) != 1 || got[0] != "Rent" {
		t.Fatalf("unexpected expense options %v", got)
	}

	if !stts.IsArchivedCategory(true, " side GIG ") || stts.IsArchivedCategory(true, "Salary") || stts.IsArchivedCategory(false, "Side gig") || !stts.IsArchivedCategory(false, "car") {
		t.Fatal("unexpected archived lookups")
	}
}
