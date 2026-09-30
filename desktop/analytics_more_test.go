package desktop

import (
	"testing"
	"time"
)

func TestAnalyticsSections(t *testing.T) {
	api := newTestAPI(t) // base $, EUR at 1.08

	for _, input := range []NewCashflowInput{
		{IsIncome: true, Currency: "$", Amount: "1000", Date: monthsAgo(2), Category: "Salary"},
		{Currency: "$", Amount: "200", Date: monthsAgo(2), Category: "Rent"},
		{IsIncome: true, Currency: "$", Amount: "1000", Date: monthsAgo(1), Category: "Salary"},
		{Currency: "$", Amount: "200", Date: monthsAgo(1), Category: "Rent"},
		{Currency: "$", Amount: "500", Date: monthsAgo(0), Category: "Rent"},
	} {
		if _, err := api.CreateCashflow(input); err != nil {
			t.Fatal(err)
		}
	}

	if err := api.CreateAccount(NewAccountInput{Name: "Euro", Description: "Savings", Currency: "EUR", Amount: "100"}); err != nil {
		t.Fatal(err)
	}

	if err := api.CreateSubscription(SubscriptionInput{Name: "Rent", Type: "Rent", Currency: "$", Amount: "150", Period: "month", PaymentMethod: "Bank", NextPayment: inDays(3), IsActive: true, IsObligation: true}); err != nil {
		t.Fatal(err)
	}

	got, err := api.Analytics(3)
	if err != nil {
		t.Fatal(err)
	}

	// Obligations of 150 a month against an income of 1000.
	if c := got.Commitments; c.ObligationsCents != 15000 || c.TotalCents != 15000 || !c.HasShare || c.Share != 15 {
		t.Fatalf("unexpected commitments %+v", c)
	}

	m := got.Monthly
	if len(m) != 3 || !m[0].HasSavingsRate || m[0].SavingsRate != 80 || m[2].HasSavingsRate || m[0].FixedCents != 15000 || m[0].FlexibleCents != 5000 {
		t.Fatalf("unexpected months %+v", m)
	}

	if tm := got.ThisMonth; tm.UsualMonths != 2 || len(tm.Items) != 1 || tm.Items[0].UsualCents != 20000 || !tm.Items[0].Unusual || tm.DaysIn != time.Now().Day() || tm.DaysInMonth < 28 {
		t.Fatalf("unexpected this month %+v", tm)
	}

	// This month's rent is above usual; nothing is flagged for being
	// lower while the month runs.
	for _, item := range got.ThisMonth.Items {
		if item.Unusual && item.DiffCents < 0 {
			t.Fatalf("lower spending flagged in a running month: %+v", item)
		}
	}

	if lm := got.LastMonth; lm.UsualMonths != 1 || len(lm.Items) != 1 || lm.Items[0].Unusual {
		t.Fatalf("unexpected last month %+v", lm)
	}

	if len(got.Exposure) != 1 || got.Exposure[0].Currency != "EUR" || got.Exposure[0].OwnedBaseCents != 10800 || got.Exposure[0].Share != 100 || got.Exposure[0].IsBase {
		t.Fatalf("unexpected exposure %+v", got.Exposure)
	}

	if len(got.FX.Months) != 3 || got.FX.Months[0].Month != got.RangeStart || len(got.Property.Months) != 3 || len(got.Investments.Months) != 3 {
		t.Fatalf("unexpected ranges %+v %+v", got.FX, got.Property)
	}

	if got.Goals == nil || got.FX.ByCurrency == nil || got.FX.Missing == nil {
		t.Fatal("lists are empty, not null, for the frontend")
	}
}
