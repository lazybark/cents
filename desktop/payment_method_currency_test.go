package desktop

import "testing"

func TestPaymentMethodCurrency(t *testing.T) {
	api := newTestAPI(t)

	if err := api.SavePaymentMethod(PaymentMethodInput{Name: "Visa", Type: "Card", Currency: "GBP"}); err == nil || err.Error() != `unknown currency "GBP": add it in settings first` {
		t.Fatalf("expected unknown currency, got %v", err)
	}

	if err := api.SavePaymentMethod(PaymentMethodInput{Name: "Visa", Type: "Card", Currency: "eur"}); err != nil {
		t.Fatal(err)
	}

	view, _ := api.Settings()
	var visa PaymentMethodSetting
	for _, p := range view.PaymentMethods {
		if p.Name == "Visa" {
			visa = p
		}
	}

	if visa.Currency != "EUR" {
		t.Fatalf("expected EUR, got %+v", visa)
	}

	subs, _ := api.Subscriptions("active", "subscription")
	if subs.Options.PaymentMethodCurrencies["Visa"] != "EUR" || subs.Options.PaymentMethodCurrencies["Other"] != "" {
		t.Fatalf("subscriptions should know each method's currency: %+v", subs.Options.PaymentMethodCurrencies)
	}

	// Saving without a currency clears it.
	if err := api.SavePaymentMethod(PaymentMethodInput{ID: visa.ID, Name: "Visa", Type: "Card"}); err != nil {
		t.Fatal(err)
	}

	if subs, _ := api.Subscriptions("active", "subscription"); len(subs.Options.PaymentMethodCurrencies) != 0 {
		t.Fatalf("expected no currencies: %+v", subs.Options.PaymentMethodCurrencies)
	}
}
