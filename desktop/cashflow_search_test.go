package desktop

import (
	"strings"
	"testing"
)

func TestCashflowSearchAndBulkChange(t *testing.T) {
	api := newTestAPI(t) // categories Salary (income), Rent (expense); EUR at 1.08
	if err := api.SaveCategory(CategoryInput{Name: "Food"}); err != nil {
		t.Fatal(err)
	}
	if err := api.CreateAccount(NewAccountInput{Name: "Card", Description: "x", Currency: "$", Amount: "1"}); err != nil {
		t.Fatal(err)
	}

	for _, input := range []NewCashflowInput{
		{Currency: "$", Amount: "12.50", Date: "2026-08-03", Category: "Rent", Comment: "Coffee in Dubai"},
		{Currency: "EUR", Amount: "100", Date: "2026-09-10", Category: "Rent", Comment: "flat"},
		{Currency: "$", Amount: "40", Date: "2026-09-12", Category: "Rent", Comment: "groceries", Account: "Card"},
		{IsIncome: true, Currency: "$", Amount: "1000", Date: "2026-09-28", Category: "Salary"},
	} {
		if _, err := api.CreateCashflow(input); err != nil {
			t.Fatal(err)
		}
	}

	all, err := api.CashflowSearch(CashflowSearchInput{})
	if err != nil || all.Total != 4 || all.Entries[0].Category != "Salary" || all.IncomeCents != 100000 || all.ExpenseCents != 1250+10800+4000 {
		t.Fatalf("unexpected %+v %v", all, err)
	}
	if len(all.Options.ExpenseCategories) != 2 || len(all.Options.Accounts) != 1 {
		t.Fatalf("unexpected options %+v", all.Options)
	}

	dubai, _ := api.CashflowSearch(CashflowSearchInput{Text: "DUBAI"})
	if dubai.Total != 1 || dubai.Entries[0].Comment != "Coffee in Dubai" {
		t.Fatalf("unexpected %+v", dubai.Entries)
	}

	ranged, err := api.CashflowSearch(CashflowSearchInput{Kind: "expense", From: "2026-09-01", To: "2026-09-30", MinBase: "50"})
	if err != nil || ranged.Total != 1 || ranged.Entries[0].Comment != "flat" {
		t.Fatalf("unexpected %+v %v", ranged.Entries, err)
	}

	for _, bad := range []CashflowSearchInput{{From: "1.9.2026"}, {MinBase: "lots"}} {
		if _, err := api.CashflowSearch(bad); err == nil {
			t.Errorf("expected %+v to fail", bad)
		}
	}

	// Move the coffee and the groceries to Food, on the card.
	ids := []uint{}
	for _, e := range all.Entries {
		if e.Comment == "Coffee in Dubai" || e.Comment == "groceries" {
			ids = append(ids, e.ID)
		}
	}

	n, err := api.ChangeCashflows(CashflowBulkInput{IDs: ids, SetCategory: true, Category: "food", SetAccount: true, Account: "Card"})
	if err != nil || n != 2 {
		t.Fatalf("unexpected %d %v", n, err)
	}

	food, _ := api.CashflowSearch(CashflowSearchInput{Category: "Food"})
	if food.Total != 2 || food.Entries[0].Account != "Card" || food.Entries[1].Account != "Card" {
		t.Fatalf("expected both in Food on the card: %+v", food.Entries)
	}

	// Taking the account off.
	if _, err := api.ChangeCashflows(CashflowBulkInput{IDs: ids, SetAccount: true, Account: ""}); err != nil {
		t.Fatal(err)
	}
	if food, _ = api.CashflowSearch(CashflowSearchInput{Account: "Card"}); food.Total != 0 {
		t.Fatalf("expected no entries on the card: %+v", food.Entries)
	}

	salary := all.Entries[0].ID
	for _, c := range []struct {
		input CashflowBulkInput
		want  string
	}{
		{CashflowBulkInput{IDs: append([]uint{salary}, ids...), SetCategory: true, Category: "Food"}, "different categories"},
		{CashflowBulkInput{IDs: []uint{salary}, SetCategory: true, Category: "Food"}, "unknown income category"},
		{CashflowBulkInput{IDs: ids, SetAccount: true, Account: "Nope"}, "unknown account"},
		{CashflowBulkInput{IDs: ids}, "pick what to change"},
		{CashflowBulkInput{SetAccount: true}, "pick the entries"},
		{CashflowBulkInput{IDs: []uint{999}, SetAccount: true}, "not found"},
	} {
		if _, err := api.ChangeCashflows(c.input); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: expected %q, got %v", c.input, c.want, err)
		}
	}
}
