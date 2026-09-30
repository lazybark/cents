package desktop

import "testing"

func TestSubscriptionPaymentLog(t *testing.T) {
	api := newTestAPI(t)

	// Rent paid by hand, due 10 days ago and not marked paid: overdue.
	if err := api.CreateSubscription(SubscriptionInput{Name: "Rent", Type: "Rent", Currency: "$", Amount: "900", Period: "month", PaymentMethod: "Bank", NextPayment: inDays(-10), IsActive: true, IsObligation: true, PaidManually: true}); err != nil {
		t.Fatal(err)
	}

	view, _ := api.Subscriptions("active", "obligation")
	rent := view.Subscriptions[0]
	if rent.DueInDays != -10 || !rent.DueSoon || !rent.PaidManually || rent.NextPayment != inDays(-10) {
		t.Fatalf("expected 10 days overdue: %+v", rent)
	}

	overview, _ := api.Overview()
	if len(overview.Upcoming) != 1 || overview.Upcoming[0].DueInDays != -10 {
		t.Fatalf("an overdue payment comes up: %+v", overview.Upcoming)
	}

	// Two clicks: the overdue one, then (by accident) next month's.
	first, err := api.MarkSubscriptionPaid(rent.ID)
	if err != nil || first.DueInDays < 15 {
		t.Fatalf("first mark: %+v %v", first, err)
	}

	second, _ := api.MarkSubscriptionPaid(rent.ID)
	if second.DueInDays <= first.DueInDays {
		t.Fatalf("second mark should move further: %+v", second)
	}

	payments, err := api.SubscriptionPayments(rent.ID)
	if err != nil || len(payments) != 2 || payments[0].PaidFor != first.NextPayment || payments[1].PaidFor != inDays(-10) {
		t.Fatalf("expected two records, latest first: %+v %v", payments, err)
	}

	// Deleting the accidental one rolls back.
	row, err := api.DeleteSubscriptionPayment(rent.ID, payments[0].ID)
	if err != nil || row.NextPayment != first.NextPayment {
		t.Fatalf("rolled back to the first mark's next: %+v %v", row, err)
	}

	// Deleting them all brings back the overdue one.
	row, _ = api.DeleteSubscriptionPayment(rent.ID, payments[1].ID)
	if row.NextPayment != inDays(-10) || row.DueInDays != -10 {
		t.Fatalf("expected overdue again: %+v", row)
	}

	if _, err := api.DeleteSubscriptionPayment(rent.ID, payments[1].ID); err == nil || err.Error() != "log entry not found" {
		t.Fatalf("expected not found, got %v", err)
	}

	// Charged automatically, the same passed date rolls on instead.
	if err := api.UpdateSubscription(SubscriptionInput{ID: rent.ID, Name: "Rent", Type: "Rent", Currency: "$", Amount: "900", Period: "month", PaymentMethod: "Bank", NextPayment: inDays(-10), IsActive: true, IsObligation: true}); err != nil {
		t.Fatal(err)
	}

	view, _ = api.Subscriptions("active", "obligation")
	if view.Subscriptions[0].DueInDays < 0 {
		t.Fatalf("an automatic payment shouldn't be overdue: %+v", view.Subscriptions[0])
	}

	if _, err := api.MarkSubscriptionPaid(rent.ID); err != nil {
		t.Fatal(err)
	}

	if err := api.DeleteSubscription(rent.ID); err != nil {
		t.Fatal(err)
	}

	if payments, _ := api.SubscriptionPayments(rent.ID); len(payments) != 0 {
		t.Fatalf("deleting the subscription deletes its records: %+v", payments)
	}
}
