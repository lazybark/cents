package desktop

import (
	"strings"
	"testing"
)

func mustSettings(t *testing.T, api *API) SettingsView {
	t.Helper()

	view, err := api.Settings()
	if err != nil {
		t.Fatal(err)
	}

	return view
}

func TestSettingsCurrencies(t *testing.T) {
	api := newTestAPI(t)

	if err := api.SaveCurrency(CurrencyInput{Name: "GBP", Rate: "1.27"}); err != nil {
		t.Fatal(err)
	}

	for want, input := range map[string]CurrencyInput{
		"rate must be greater than zero":        {Name: "CHF", Rate: "0"},
		`a currency named "eur" already exists`: {Name: "eur", Rate: "1"},
		`"$" is the base currency`:              {Name: "$", Rate: "1"},
		"setting not found":                     {ID: 999, Name: "X", Rate: "1"},
	} {
		if err := api.SaveCurrency(input); err == nil || err.Error() != want {
			t.Errorf("expected %q, got %v", want, err)
		}
	}

	view := mustSettings(t, api)
	gbp := view.Currencies[1]
	if len(view.Currencies) != 2 || gbp.Name != "GBP" || gbp.RateToBase != 1.27 {
		t.Fatalf("unexpected currencies %+v", view.Currencies)
	}

	if err := api.SaveCurrency(CurrencyInput{ID: gbp.ID, Name: "GBP", Rate: "1.3"}); err != nil {
		t.Fatal(err)
	}

	mustCreate(t, api, NewAccountInput{Name: "UK", Description: "isa", Currency: "GBP", Amount: "100"})
	view = mustSettings(t, api)
	if view.Currencies[1].RateToBase != 1.3 || view.Currencies[1].UsedBy != 1 {
		t.Fatalf("expected edited rate and one user, got %+v", view.Currencies[1])
	}

	overview, err := api.Accounts(0)
	if err != nil || overview.TotalCents != 13000 {
		t.Fatalf("expected new rate in totals, got %d %v", overview.TotalCents, err)
	}

	if err := api.DeleteSetting("currency", gbp.ID); err != nil {
		t.Fatal(err)
	}

	if err := api.DeleteSetting("currency", gbp.ID); err != errSettingNotFound {
		t.Fatalf("expected not found, got %v", err)
	}

	overview, _ = api.Accounts(0)
	if overview.TotalCents != 0 || overview.Accounts[0].HasRate {
		t.Fatalf("expected account without rate after delete, got %+v", overview)
	}
}

func TestSettingsBaseCurrency(t *testing.T) {
	api := newTestAPI(t)

	if err := api.SaveBaseCurrency(" "); err == nil || err.Error() != "base currency cannot be empty" {
		t.Fatalf("unexpected %v", err)
	}

	if err := api.SaveBaseCurrency("eur"); err == nil || !strings.Contains(err.Error(), "currency list") {
		t.Fatalf("expected refusal for listed currency, got %v", err)
	}

	if err := api.SaveBaseCurrency("€"); err != nil {
		t.Fatal(err)
	}

	if view := mustSettings(t, api); view.BaseCurrency != "€" {
		t.Fatalf("expected € base, got %q", view.BaseCurrency)
	}
}

func TestSettingsPaymentMethodsKeepOneDefault(t *testing.T) {
	api := newTestAPI(t)

	view := mustSettings(t, api)
	if len(view.PaymentMethods) != 1 || view.PaymentMethods[0].Name != "Other" || !view.PaymentMethods[0].IsDefault {
		t.Fatalf("expected the built-in default, got %+v", view.PaymentMethods)
	}

	if err := api.SavePaymentMethod(PaymentMethodInput{Name: "Visa", Type: "Card", IsDefault: true}); err != nil {
		t.Fatal(err)
	}

	if err := api.SavePaymentMethod(PaymentMethodInput{Name: "visa", Type: "Card"}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate error, got %v", err)
	}

	if err := api.SavePaymentMethod(PaymentMethodInput{Name: "Cash", Type: "Paper"}); err == nil || !strings.HasPrefix(err.Error(), "payment method type must be one of") {
		t.Fatalf("expected type error, got %v", err)
	}

	view = mustSettings(t, api)
	if view.PaymentMethods[0].Name != "Visa" || !view.PaymentMethods[0].IsDefault || view.PaymentMethods[1].IsDefault {
		t.Fatalf("expected Visa to be the only default, got %+v", view.PaymentMethods)
	}

	if err := api.DeleteSetting("payment_method", view.PaymentMethods[0].ID); err != nil {
		t.Fatal(err)
	}

	view = mustSettings(t, api)
	if len(view.PaymentMethods) != 1 || !view.PaymentMethods[0].IsDefault {
		t.Fatalf("expected remaining method to become default, got %+v", view.PaymentMethods)
	}
}

func TestSettingsTaxTypesAndCategories(t *testing.T) {
	api := newTestAPI(t)

	if err := api.SaveTaxType(TaxTypeInput{Country: "Netherlands", Name: "Income Tax", URL: "https://example.org"}); err != nil {
		t.Fatal(err)
	}

	if err := api.SaveTaxType(TaxTypeInput{Country: "netherlands", Name: "income tax"}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate error, got %v", err)
	}

	if err := api.SaveTaxType(TaxTypeInput{Country: "Netherlands"}); err == nil || err.Error() != "tax type name is required" {
		t.Fatalf("unexpected %v", err)
	}

	if err := api.SaveCategory(CategoryInput{IsIncome: true, Name: "Freelance"}); err != nil {
		t.Fatal(err)
	}

	// Income and expense categories are separate lists, so a name can be in both.
	if err := api.SaveCategory(CategoryInput{Name: "Salary"}); err != nil {
		t.Fatal(err)
	}

	if err := api.SaveCategory(CategoryInput{IsIncome: true, Name: "salary"}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate error, got %v", err)
	}

	if _, err := api.CreateCashflow(NewCashflowInput{IsIncome: true, Currency: "$", Amount: "1", Date: "2026-09-01", Category: "Freelance"}); err != nil {
		t.Fatal(err)
	}

	view := mustSettings(t, api)
	if len(view.TaxTypes) != 1 || view.TaxTypes[0].URL != "https://example.org" {
		t.Fatalf("unexpected tax types %+v", view.TaxTypes)
	}

	income := map[string]CategorySetting{}
	for _, c := range view.IncomeCategories {
		income[c.Name] = c
	}

	if income["Freelance"].UsedBy != 1 || len(view.ExpenseCategories) != 2 {
		t.Fatalf("unexpected categories %+v / %+v", view.IncomeCategories, view.ExpenseCategories)
	}

	if err := api.SaveCategory(CategoryInput{IsIncome: true, ID: income["Freelance"].ID, Name: "Contracting"}); err != nil {
		t.Fatal(err)
	}

	if err := api.DeleteSetting("income_category", income["Salary"].ID); err != nil {
		t.Fatal(err)
	}

	if err := api.DeleteSetting("bogus", 1); err != errSettingNotFound {
		t.Fatalf("expected not found for unknown kind, got %v", err)
	}

	view = mustSettings(t, api)
	if len(view.IncomeCategories) != 1 || view.IncomeCategories[0].Name != "Contracting" {
		t.Fatalf("unexpected income categories %+v", view.IncomeCategories)
	}
}
