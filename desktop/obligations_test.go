package desktop

import "testing"

func TestObligationsAreListedApart(t *testing.T) {
	api := newTestAPI(t)

	for _, input := range []SubscriptionInput{
		{Name: "Netflix", Type: "Multimedia", Currency: "$", Amount: "10", Period: "month", PaymentMethod: "Other", NextPayment: inDays(2), IsActive: true},
		{Name: "Rent", Type: "Rent", Currency: "$", Amount: "900", Period: "month", PaymentMethod: "Bank", NextPayment: inDays(4), IsActive: true, IsObligation: true},
		{Name: "Insurance", Type: "Insurance", Currency: "$", Amount: "1200", Period: "year", PaymentMethod: "Bank", IsActive: true, IsObligation: true},
	} {
		if err := api.CreateSubscription(input); err != nil {
			t.Fatal(err)
		}
	}

	subs, _ := api.Subscriptions("active", "subscription")
	obligations, err := api.Subscriptions("active", "obligation")
	if err != nil || len(subs.Subscriptions) != 1 || subs.Subscriptions[0].Name != "Netflix" || len(obligations.Subscriptions) != 2 || obligations.Kind != "obligation" || !obligations.Subscriptions[0].IsObligation {
		t.Fatalf("unexpected lists %+v %+v %v", subs.Subscriptions, obligations.Subscriptions, err)
	}

	if subs.Active.MonthlyCents != 1000 || obligations.Active.MonthlyCents != 90000 || obligations.Active.YearlyCents != 90000*12+120000 {
		t.Fatalf("unexpected totals %+v %+v", subs.Active, obligations.Active)
	}

	if !containsFold(obligations.Options.Types, "Rent") || containsFold(subs.Options.Types, "Rent") {
		t.Fatalf("each kind suggests its own types: %v %v", subs.Options.Types, obligations.Options.Types)
	}

	overview, _ := api.Overview()
	if len(overview.Unscheduled) != 1 || overview.Unscheduled[0].Name != "Insurance" {
		t.Fatalf("the insurance has no date and should be named: %v", overview.Unscheduled)
	}

	if overview.MonthlySubscriptions != 1000 || overview.MonthlyObligations != 90000 || overview.YearlyObligations != 90000*12+120000 || len(overview.Upcoming) != 2 {
		t.Fatalf("unexpected overview %+v", overview)
	}

	// Moving one over is an edit.
	netflix := subs.Subscriptions[0]
	if err := api.UpdateSubscription(SubscriptionInput{ID: netflix.ID, Name: "Netflix", Type: "Multimedia", Currency: "$", Amount: "10", Period: "month", PaymentMethod: "Other", IsActive: true, IsObligation: true}); err != nil {
		t.Fatal(err)
	}

	if subs, _ := api.Subscriptions("all", "subscription"); len(subs.Subscriptions) != 0 {
		t.Fatalf("expected it moved: %+v", subs.Subscriptions)
	}
}
