package desktop

import (
	"strings"
	"testing"
)

func categoryID(t *testing.T, api *API, name string) uint {
	t.Helper()
	view, _ := api.Settings()
	for _, c := range view.ExpenseCategories {
		if c.Name == name {
			return c.ID
		}
	}

	t.Fatalf("no expense category %q", name)
	return 0
}

func TestRenamingAndMergingSettings(t *testing.T) {
	api := newTestAPI(t) // expense category Rent

	if err := api.SaveCategory(CategoryInput{Name: "Housing"}); err != nil {
		t.Fatal(err)
	}

	for _, category := range []string{"Rent", "Rent", "Housing"} {
		if _, err := api.CreateCashflow(NewCashflowInput{Currency: "$", Amount: "10", Date: monthsAgo(0), Category: category}); err != nil {
			t.Fatal(err)
		}
	}
	if err := api.SaveBudget(BudgetInput{Category: "Rent", Limit: "100"}); err != nil {
		t.Fatal(err)
	}

	// Renaming shows on every entry and the budget, with nothing else saved.
	rent := categoryID(t, api, "Rent")
	if err := api.SaveCategory(CategoryInput{ID: rent, Name: "Flat"}); err != nil {
		t.Fatal(err)
	}

	month, _ := api.CashflowMonth("")
	names := map[string]int{}
	for _, e := range month.Entries {
		names[e.Category]++
	}
	if names["Flat"] != 2 || names["Housing"] != 1 || month.Budgets[0].Name != "Flat" {
		t.Fatalf("expected the new name everywhere: %v %+v", names, month.Budgets)
	}

	// Used, so it can't be deleted: merge it instead.
	err := api.DeleteSetting(settingExpenseCategory, rent)
	if err == nil || !strings.Contains(err.Error(), "used by 3 records") {
		t.Fatalf("expected the delete refused: %v", err)
	}

	result, err := api.Merge(MergeInput{Kind: settingExpenseCategory, FromID: rent, IntoID: categoryID(t, api, "Housing")})
	if err != nil || result.Moved != 3 {
		t.Fatalf("expected 2 entries and the budget moved: %+v %v", result, err)
	}

	month, _ = api.CashflowMonth("")
	for _, e := range month.Entries {
		if e.Category != "Housing" {
			t.Fatalf("expected everything in Housing: %+v", e)
		}
	}
	if month.Budgets[0].Name != "Housing" || month.Budgets[0].SpentCents != 3000 {
		t.Fatalf("expected the budget on Housing: %+v", month.Budgets)
	}

	if _, err := api.Merge(MergeInput{Kind: "budget", FromID: 1, IntoID: 2}); err == nil {
		t.Fatal("budgets aren't merged")
	}
}

func TestPaymentMethodsAndAccountsLink(t *testing.T) {
	api := newTestAPI(t)

	if err := api.SavePaymentMethod(PaymentMethodInput{Name: "Card", Type: "Card"}); err != nil {
		t.Fatal(err)
	}
	if err := api.CreateSubscription(SubscriptionInput{Name: "Music", Type: "Multimedia", Currency: "$", Amount: "5", Period: "month", PaymentMethod: "Card", IsActive: true}); err != nil {
		t.Fatal(err)
	}

	view, _ := api.Settings()
	var card PaymentMethodSetting
	for _, p := range view.PaymentMethods {
		if p.Name == "Card" {
			card = p
		}
	}

	// Archived: off the pickers, still on the subscription; renamed too.
	if err := api.SavePaymentMethod(PaymentMethodInput{ID: card.ID, Name: "Old card", Type: "Card", Archived: true}); err != nil {
		t.Fatal(err)
	}
	subs, _ := api.Subscriptions("all", "subscription")
	if subs.Subscriptions[0].PaymentMethod != "Old card" {
		t.Fatalf("expected the renamed method: %+v", subs.Subscriptions[0])
	}
	for _, option := range subs.Options.PaymentMethods {
		if option == "Old card" {
			t.Fatal("an archived method isn't offered")
		}
	}
	if view, _ = api.Settings(); !anyArchived(view.PaymentMethods) {
		t.Fatalf("expected it archived: %+v", view.PaymentMethods)
	}

	// An account with entries can't be deleted, only merged.
	if err := api.CreateAccount(NewAccountInput{Name: "Old", Description: "x", Currency: "$", Amount: "1"}); err != nil {
		t.Fatal(err)
	}
	if err := api.CreateAccount(NewAccountInput{Name: "New", Description: "x", Currency: "$", Amount: "1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := api.CreateCashflow(NewCashflowInput{Currency: "$", Amount: "3", Date: monthsAgo(0), Category: "Rent", Account: "Old"}); err != nil {
		t.Fatal(err)
	}

	accounts, _ := api.Accounts(0)
	ids := map[string]AccountRow{}
	for _, a := range accounts.Accounts {
		ids[a.Name] = a
	}
	if ids["Old"].UsedBy != 1 || ids["New"].UsedBy != 0 {
		t.Fatalf("unexpected usage %+v", accounts.Accounts)
	}

	if err := api.DeleteAccount(ids["Old"].ID); err == nil || !strings.Contains(err.Error(), "used by 1 record") {
		t.Fatalf("expected the delete refused: %v", err)
	}
	if _, err := api.Merge(MergeInput{Kind: "account", FromID: ids["Old"].ID, IntoID: ids["New"].ID}); err != nil {
		t.Fatal(err)
	}
	if month, _ := api.CashflowMonth(""); month.Entries[0].Account != "New" {
		t.Fatalf("expected the entry on New: %+v", month.Entries[0])
	}
}

func anyArchived(methods []PaymentMethodSetting) bool {
	for _, m := range methods {
		if m.Archived {
			return true
		}
	}

	return false
}
