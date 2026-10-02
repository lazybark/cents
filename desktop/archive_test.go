package desktop

import "testing"

func TestArchivedCategory(t *testing.T) {
	api := newTestAPI(t)

	if err := api.SaveCategory(CategoryInput{Name: "Car"}); err != nil {
		t.Fatal(err)
	}

	for _, input := range []NewCashflowInput{
		{Currency: "$", Amount: "5000", Date: "2026-03-05", Category: "Car"},
		{Currency: "$", Amount: "100", Date: "2026-03-06", Category: "Rent"},
		{IsIncome: true, Currency: "$", Amount: "300", Date: "2026-03-01", Category: "Salary"},
	} {
		if _, err := api.CreateCashflow(input); err != nil {
			t.Fatal(err)
		}
	}

	view, _ := api.Settings()
	var car CategorySetting
	for _, c := range view.ExpenseCategories {
		if c.Name == "Car" {
			car = c
		}
	}

	if err := api.SaveCategory(CategoryInput{ID: car.ID, Name: "Car", Archived: true}); err != nil {
		t.Fatal(err)
	}

	view, _ = api.Settings()
	for _, c := range view.ExpenseCategories {
		if c.Name == "Car" && (!c.Archived || c.UsedBy != 1) {
			t.Fatalf("expected Car archived and still used: %+v", c)
		}
	}

	month, err := api.CashflowMonth("2026-03")
	if err != nil || len(month.Entries) != 3 || month.ExpenseCents != 510000 {
		t.Fatalf("the month should still have the car and count it: %+v %v", month, err)
	}

	for _, option := range month.Options.ExpenseCategories {
		if option == "Car" {
			t.Fatal("archived category offered for new entries")
		}
	}

	if _, err := api.CreateCashflow(NewCashflowInput{Currency: "$", Amount: "1", Date: "2026-03-07", Category: "Car"}); err == nil || err.Error() != `unknown category "Car"` {
		t.Fatalf("expected archived category to be refused, got %v", err)
	}

	stats, err := api.CashflowStats(false)
	if err != nil || len(stats.Rows) != 1 || stats.Rows[0].ExpenseCents != 10000 || stats.ArchivedLeftOut != 1 {
		t.Fatalf("statistics should leave the car out: %+v %v", stats, err)
	}

	with, _ := api.CashflowStats(true)
	all, _ := api.CashflowOverview()
	if with.Rows[0].ExpenseCents != 510000 || with.ArchivedLeftOut != 0 || all.Rows[0].ExpenseCents != 510000 {
		t.Fatalf("including archived (and all months) should count the car: %+v %+v", with, all)
	}

	categories, _ := api.CashflowCategories("expense", false)
	if len(categories.Categories) != 1 || categories.Categories[0].Name != "Rent" || categories.ArchivedLeftOut != 1 {
		t.Fatalf("categories should leave the car out: %+v", categories)
	}

	categories, _ = api.CashflowCategories("expense", true)
	if len(categories.Categories) != 2 || categories.Categories[0].Name != "Car" || !categories.Categories[0].Archived || categories.Categories[1].Archived {
		t.Fatalf("including archived should mark the car: %+v", categories)
	}

	if incomes, _ := api.CashflowCategories("income", false); incomes.ArchivedLeftOut != 0 {
		t.Fatalf("expense archives shouldn't count for incomes: %+v", incomes)
	}

	// Unarchiving brings it back.
	if err := api.SaveCategory(CategoryInput{ID: car.ID, Name: "Car"}); err != nil {
		t.Fatal(err)
	}

	if stats, _ := api.CashflowStats(false); stats.Rows[0].ExpenseCents != 510000 {
		t.Fatalf("unarchived category should count again: %+v", stats)
	}
}

func TestArchivedAccount(t *testing.T) {
	api := newTestAPI(t)
	old := mustCreate(t, api, NewAccountInput{Name: "Old card", Description: "closed", Currency: "$", Amount: "0"})
	mustCreate(t, api, NewAccountInput{Name: "Checking", Description: "main", Currency: "$", Amount: "100"})

	if _, err := api.UpdateAccountAmount(AmountUpdateInput{ID: old.ID, Amount: "0", Archived: true}); err != nil {
		t.Fatal(err)
	}

	overview, err := api.Accounts(0)
	if err != nil || len(overview.Accounts) != 2 || overview.Accounts[1].Name != "Old card" || !overview.Accounts[1].Archived || overview.Accounts[0].Archived {
		t.Fatalf("archived account should stay listed, last: %+v %v", overview.Accounts, err)
	}

	month, _ := api.CashflowMonth("")
	invoices, _ := api.Invoices("outgoing")
	if len(month.Options.Accounts) != 1 || month.Options.Accounts[0] != "Checking" || len(invoices.Accounts) != 1 {
		t.Fatalf("archived account offered in pickers: %+v %+v", month.Options.Accounts, invoices.Accounts)
	}

	if _, err := api.CreateCashflow(NewCashflowInput{Currency: "$", Amount: "1", Date: "2026-03-01", Category: "Rent", Account: "old card"}); err == nil || err.Error() != `account "Old card" is archived` {
		t.Fatalf("expected archived account error, got %v", err)
	}

	if _, err := api.UpdateAccountAmount(AmountUpdateInput{ID: old.ID, Amount: "0"}); err != nil {
		t.Fatal(err)
	}

	if month, _ := api.CashflowMonth(""); len(month.Options.Accounts) != 2 {
		t.Fatalf("unarchived account should be offered again: %+v", month.Options.Accounts)
	}
}
