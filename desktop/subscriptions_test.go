package desktop

import (
	"strings"
	"testing"
)

func TestSubscriptionLifecycle(t *testing.T) {
	api := newTestAPI(t)

	inputs := []NewSubscriptionInput{
		{Name: "Music", Currency: "$", Amount: "10", Period: "month", PaymentMethod: "Other", Type: "Multimedia", IsActive: true, PaymentDay: "5"},
		{Name: "Domain", Currency: "eur", Amount: "20", Period: "year", PaymentMethod: "Visa", Type: "Domain", IsActive: true, PaymentDate: "2026-12-24"},
		{Name: "Old", Currency: "$", Amount: "3", Period: "month", PaymentMethod: "Other", Type: "Other", IsActive: false},
	}
	for _, input := range inputs {
		if err := api.CreateSubscription(input); err != nil {
			t.Fatal(err)
		}
	}

	view, err := api.Subscriptions("active")
	if err != nil {
		t.Fatal(err)
	}

	if view.Mode != "active" || len(view.Subscriptions) != 2 || view.Subscriptions[0].Name != "Domain" {
		t.Fatalf("expected two active subscriptions, largest first, got %+v", view.Subscriptions)
	}

	if view.Active.MonthlyCents != 1000 || view.Active.YearlyCents != 2160+12000 || view.Inactive.MonthlyCents != 300 {
		t.Fatalf("unexpected totals %+v %+v", view.Active, view.Inactive)
	}

	domain := view.Subscriptions[0]
	if domain.Currency != "EUR" || domain.PaymentDate != "2026-12-24" || domain.BaseCents != 2160 || view.Subscriptions[1].PaymentDay != "5" {
		t.Fatalf("unexpected rows %+v", view.Subscriptions)
	}

	if strings.Join(view.Options.Periods, ",") != "month,year" || len(view.Options.Types) != 5 || view.Options.PaymentMethods[0] != "Other" {
		t.Fatalf("unexpected options %+v", view.Options)
	}

	if err := api.UpdateSubscription(SubscriptionUpdateInput{ID: domain.ID, Amount: "25", PaymentMethod: "Mastercard", IsActive: false}); err != nil {
		t.Fatal(err)
	}

	all, err := api.Subscriptions("all")
	if err != nil {
		t.Fatal(err)
	}

	if len(all.Subscriptions) != 3 || all.Subscriptions[0].Name != "Domain" || all.Subscriptions[0].IsActive || all.Subscriptions[0].PaymentMethod != "Mastercard" || all.Subscriptions[0].Period != "year" {
		t.Fatalf("expected edited Domain kept its period and went inactive, got %+v", all.Subscriptions)
	}

	if err := api.UpdateSubscription(SubscriptionUpdateInput{ID: domain.ID, Amount: "25", PaymentMethod: " "}); err == nil || err.Error() != "payment method is required" {
		t.Fatalf("expected payment method error, got %v", err)
	}

	if err := api.DeleteSubscription(domain.ID); err != nil {
		t.Fatal(err)
	}

	if err := api.DeleteSubscription(domain.ID); err != errSubscriptionNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestCreateSubscriptionValidates(t *testing.T) {
	api := newTestAPI(t)

	cases := map[string]NewSubscriptionInput{
		"subscription name is required": {Currency: "$", Amount: "1", Period: "month", PaymentMethod: "Other", Type: "Other"},
		"currency is required":          {Name: "x", Amount: "1", Period: "month", PaymentMethod: "Other", Type: "Other"},
		"unknown currency":              {Name: "x", Currency: "BTC", Amount: "1", Period: "month", PaymentMethod: "Other", Type: "Other"},
		"monthly day must be between":   {Name: "x", Currency: "$", Amount: "1", Period: "month", PaymentMethod: "Other", Type: "Other", PaymentDay: "40"},
		"yearly date must use":          {Name: "x", Currency: "$", Amount: "1", Period: "year", PaymentMethod: "Other", Type: "Other", PaymentDate: "24.12.2026"},
	}

	for want, input := range cases {
		if err := api.CreateSubscription(input); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("expected error containing %q, got %v", want, err)
		}
	}
}
