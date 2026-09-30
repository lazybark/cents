package desktop

import (
	"testing"
	"time"
)

func TestBudgets(t *testing.T) {
	api := newTestAPI(t) // base $, EUR at 1.08; expense category Rent
	thisMonth := time.Now().Format(monthLayout)
	today := time.Now().Format(dayLayout)

	for _, input := range []NewCashflowInput{
		{Currency: "$", Amount: "100", Date: monthsAgo(1), Category: "Rent"},
		{Currency: "$", Amount: "90", Date: today, Category: "Rent"},
		{Currency: "EUR", Amount: "20", Date: today, Category: "Rent"}, // 21.60
		{IsIncome: true, Currency: "$", Amount: "999", Date: today, Category: "Salary"},
	} {
		if _, err := api.CreateCashflow(input); err != nil {
			t.Fatal(err)
		}
	}

	view, err := api.Budgets("")
	if err != nil || len(view.Rows) != 0 || len(view.Options.Categories) != 1 || view.Options.Categories[0] != "Rent" || view.Options.HasTotal || !view.IsCurrent || view.DaysIn == 0 {
		t.Fatalf("unexpected empty view %+v %v", view, err)
	}

	if view.Options.Usual["Rent"] != 10000 || view.Options.Usual[""] != 10000 {
		t.Fatalf("expected usual spending as suggestions: %+v", view.Options.Usual)
	}

	if err := api.SaveBudget(BudgetInput{Category: "rent", Limit: "120"}); err != nil {
		t.Fatal(err)
	}
	if err := api.SaveBudget(BudgetInput{Limit: "500"}); err != nil {
		t.Fatal(err)
	}
	if err := api.SaveBudget(BudgetInput{Category: "Rent", Limit: "1"}); err == nil {
		t.Fatal("expected a second Rent budget to fail")
	}

	view, _ = api.Budgets("")
	if len(view.Rows) != 2 || !view.Rows[0].IsTotal || view.Rows[0].Name != "All spending" || len(view.Options.Categories) != 0 || !view.Options.HasTotal {
		t.Fatalf("unexpected view %+v", view)
	}

	rent := view.Rows[1]
	if rent.Category != "Rent" || rent.SpentCents != 11160 || rent.LeftCents != 840 || rent.State != "close" || rent.UsualCents != 10000 {
		t.Fatalf("unexpected rent %+v", rent)
	}

	overview, _ := api.Overview()
	if len(overview.Budgets) != 2 || overview.Budgets[1].Name != "Rent" || overview.Budgets[1].State != "close" {
		t.Fatalf("expected both budgets on the overview: %+v", overview.Budgets)
	}

	month, _ := api.CashflowMonth("")
	if len(month.Budgets) != 2 || month.Budgets[1].SpentCents != 11160 {
		t.Fatalf("expected the budgets with the month: %+v", month.Budgets)
	}

	if earlier, _ := api.CashflowMonth(monthsAgo(1)[:7]); earlier.Budgets[1].SpentCents != 10000 {
		t.Fatalf("expected last month's spending against the budget: %+v", earlier.Budgets)
	}

	// Over it after lowering the limit.
	if err := api.SaveBudget(BudgetInput{ID: rent.ID, Category: "Rent", Limit: "100"}); err != nil {
		t.Fatal(err)
	}
	if view, _ = api.Budgets(thisMonth); view.Rows[1].State != "over" || view.Rows[1].LeftCents != -1160 {
		t.Fatalf("expected over: %+v", view.Rows[1])
	}

	// Last month: within limits, not the running month.
	last, err := api.Budgets(monthsAgo(1)[:7])
	if err != nil || last.IsCurrent || last.DaysIn != 0 || last.Rows[1].SpentCents != 10000 || last.Rows[1].State != "close" {
		t.Fatalf("unexpected last month %+v %v", last, err)
	}

	// Settings count the budget as a use of the category.
	settingsView, _ := api.Settings()
	for _, c := range settingsView.ExpenseCategories {
		if c.Name == "Rent" && c.UsedBy != 4 {
			t.Fatalf("expected 3 entries and the budget: %+v", c)
		}
	}

	if err := api.DeleteBudget(rent.ID); err != nil {
		t.Fatal(err)
	}
	if err := api.DeleteBudget(rent.ID); err == nil {
		t.Fatal("expected deleting twice to fail")
	}
	if _, err := api.Budgets("2026-13"); err == nil {
		t.Fatal("expected a bad month to fail")
	}
}
