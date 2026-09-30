package desktop

import "testing"

func TestUpdateCashflow(t *testing.T) {
	api := newTestAPI(t)
	old := mustCreate(t, api, NewAccountInput{Name: "Old card", Description: "closed", Currency: "$", Amount: "0"})
	mustCreate(t, api, NewAccountInput{Name: "Checking", Description: "main", Currency: "$", Amount: "0"})

	if err := api.SaveCategory(CategoryInput{Name: "Car"}); err != nil {
		t.Fatal(err)
	}

	if _, err := api.CreateCashflow(NewCashflowInput{Currency: "EUR", Rate: "1.5", Amount: "100", Date: "2026-03-05", Category: "Car", Account: "Old card", Comment: "typo"}); err != nil {
		t.Fatal(err)
	}

	entry := func(month string) CashflowRow {
		t.Helper()
		data, err := api.CashflowMonth(month)
		if err != nil || len(data.Entries) != 1 {
			t.Fatalf("expected one entry in %s: %+v %v", month, data.Entries, err)
		}
		return data.Entries[0]
	}

	car := entry("2026-03")

	// Archive the category and the account the entry uses.
	view, _ := api.Settings()
	for _, c := range view.ExpenseCategories {
		if c.Name == "Car" {
			if err := api.SaveCategory(CategoryInput{ID: c.ID, Name: "Car", Archived: true}); err != nil {
				t.Fatal(err)
			}
		}
	}

	if _, err := api.UpdateAccountAmount(AmountUpdateInput{ID: old.ID, Amount: "0", Archived: true}); err != nil {
		t.Fatal(err)
	}

	// Keeping them (and the currency) is fine; the rate stays as recorded.
	moved, err := api.UpdateCashflow(CashflowUpdateInput{ID: car.ID, NewCashflowInput: NewCashflowInput{Currency: "eur", Amount: "120", Date: "2026-04-01", Category: "car", Account: "old card", Comment: " fixed "}})
	if err != nil || moved.Month != "2026-04" {
		t.Fatalf("update: %+v %v", moved, err)
	}

	got := entry("2026-04")
	if got.ID != car.ID || got.AmountCents != 12000 || got.RateToBase != 1.5 || got.BaseCents != 18000 || got.Category != "Car" || got.Account != "Old card" || got.Comment != "fixed" || !got.CategoryArchived {
		t.Fatalf("unexpected entry %+v", got)
	}

	for want, input := range map[string]NewCashflowInput{
		`unknown category "Boat"`:                          {Currency: "EUR", Amount: "1", Date: "2026-04-01", Category: "Boat"},
		`account "Old card" is archived`:                   {Currency: "EUR", Amount: "1", Date: "2026-04-01", Category: "Rent", Account: "Old card"},
		`unknown currency "GBP": add it in settings first`: {Currency: "GBP", Amount: "1", Date: "2026-04-01", Category: "Car"},
		"amount error: amount must be a number":            {Currency: "EUR", Amount: "x", Date: "2026-04-01", Category: "Car"},
	} {
		if want == `account "Old card" is archived` {
			// The account was dropped first, so picking it again is refused.
			if _, err := api.UpdateCashflow(CashflowUpdateInput{ID: car.ID, NewCashflowInput: NewCashflowInput{Currency: "EUR", Amount: "1", Date: "2026-04-01", Category: "Car"}}); err != nil {
				t.Fatal(err)
			}
		}

		if _, err := api.UpdateCashflow(CashflowUpdateInput{ID: car.ID, NewCashflowInput: input}); err == nil || err.Error() != want {
			t.Errorf("want %q, got %v", want, err)
		}
	}

	// A new currency takes its rate from settings; a typed rate wins.
	if _, err := api.UpdateCashflow(CashflowUpdateInput{ID: car.ID, NewCashflowInput: NewCashflowInput{Currency: "$", Amount: "50", Date: "2026-04-01", Category: "Car", Account: "Checking"}}); err != nil {
		t.Fatal(err)
	}

	if got := entry("2026-04"); got.RateToBase != 1 || got.BaseCents != 5000 || got.Account != "Checking" {
		t.Fatalf("base currency should use rate 1: %+v", got)
	}

	if _, err := api.UpdateCashflow(CashflowUpdateInput{ID: car.ID, NewCashflowInput: NewCashflowInput{Currency: "EUR", Amount: "10", Date: "2026-04-01", Category: "Car"}}); err != nil {
		t.Fatal(err)
	}

	if got := entry("2026-04"); got.RateToBase != 1.08 {
		t.Fatalf("a new currency should take the settings rate: %+v", got)
	}

	// Switching an expense to an income needs an income category.
	if _, err := api.UpdateCashflow(CashflowUpdateInput{ID: car.ID, NewCashflowInput: NewCashflowInput{IsIncome: true, Currency: "EUR", Amount: "10", Date: "2026-04-01", Category: "Car"}}); err == nil {
		t.Fatal("expected an expense category to be refused for an income")
	}

	if _, err := api.UpdateCashflow(CashflowUpdateInput{ID: car.ID, NewCashflowInput: NewCashflowInput{IsIncome: true, Currency: "EUR", Amount: "10", Date: "2026-04-01", Category: "Salary"}}); err != nil {
		t.Fatal(err)
	}

	if got := entry("2026-04"); !got.IsIncome || got.Category != "Salary" {
		t.Fatalf("expected an income now: %+v", got)
	}

	if _, err := api.UpdateCashflow(CashflowUpdateInput{ID: 999}); err != errCashflowNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}
