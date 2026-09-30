package desktop

import (
	"testing"
	"time"
)

// monthsAgo is the 5th of the month n months back, as the frontend sends
// dates.
func monthsAgo(n int) string {
	now := time.Now()
	return time.Date(now.Year(), now.Month()-time.Month(n), 5, 0, 0, 0, 0, time.Local).Format(dayLayout)
}

func TestAnalytics(t *testing.T) {
	api := newTestAPI(t)

	if err := api.CreateAccount(NewAccountInput{Name: "Bank", Description: "Main", Currency: "$", Amount: "3000"}); err != nil {
		t.Fatal(err)
	}

	accounts, _ := api.Accounts(0)
	if err := api.SaveAccountValueLog(ValueLogInput{AccountID: accounts.Accounts[0].ID, Date: monthsAgo(14), Value: "500"}); err != nil {
		t.Fatal(err)
	}

	for _, input := range []NewCashflowInput{
		{IsIncome: true, Currency: "$", Amount: "2000", Date: monthsAgo(2), Category: "Salary"},
		{Currency: "$", Amount: "1500", Date: monthsAgo(2), Category: "Rent"},
		{IsIncome: true, Currency: "$", Amount: "2000", Date: monthsAgo(1), Category: "Salary"},
		{Currency: "$", Amount: "500", Date: monthsAgo(1), Category: "Rent", Comment: "food"},
		// The running month counts toward the range, not the averages.
		{Currency: "$", Amount: "100", Date: monthsAgo(0), Category: "Rent", Comment: "food"},
	} {
		if _, err := api.CreateCashflow(input); err != nil {
			t.Fatal(err)
		}
	}

	if err := api.CreateSubscription(SubscriptionInput{Name: "Rent", Type: "Rent", Currency: "$", Amount: "600", Period: "month", PaymentMethod: "Bank", NextPayment: inDays(3), IsActive: true, IsObligation: true}); err != nil {
		t.Fatal(err)
	}

	if _, err := api.CreateDebt(NewDebtInput{IsOwedToUser: true, Peer: "Ann", Currency: "$", Amount: "50", AmountPaid: "0", CreatedAt: monthsAgo(0), DueDate: inDays(10)}); err != nil {
		t.Fatal(err)
	}

	got, err := api.Analytics(3)
	if err != nil {
		t.Fatal(err)
	}

	if got.AveragedMonths != 2 || got.AverageIncome != 200000 || got.AverageExpense != 100000 || !got.HasSavingsRate || got.SavingsRate != 50 {
		t.Fatalf("unexpected averages %+v", got)
	}

	if !got.HasRunway || got.RunwayMonths != 3 || got.Committed != 60000 || got.FixedShare != 60 {
		t.Fatalf("unexpected runway or fixed costs %+v", got)
	}

	if got.NetWorth != 300000+5000 {
		t.Fatalf("unexpected net worth %d", got.NetWorth)
	}

	// From the log 14 months back to now: estimates, then this month's
	// snapshot.
	history := got.NetWorthHistory
	if len(history) != 15 || !history[0].Estimated || history[0].Cents != 50000 || history[14].Estimated || history[14].Cents != got.NetWorth {
		t.Fatalf("unexpected history %+v", history)
	}

	// Started from an estimate, the change is in what's owned: the account
	// went from 500 (or 3000, when last December came after the log) to 3000.
	if !got.HasChange || !got.ChangeEstimated || got.Change != 300000-50000 && got.Change != 0 {
		t.Fatalf("unexpected change %+v", got)
	}

	if len(got.Monthly) != 3 || got.Monthly[0].ExpenseCents != 150000 || got.Monthly[2].ExpenseCents != 10000 {
		t.Fatalf("unexpected months %+v", got.Monthly)
	}

	if len(got.Spending) != 1 || got.Spending[0].Category != "Rent" || got.Spending[0].Cents != 210000 || got.Spending[0].Share != 100 {
		t.Fatalf("unexpected spending %+v", got.Spending)
	}

	if len(got.Forecast) < 3 || got.Forecast[0].Name != "Rent" || got.ForecastIn != 5000 || got.ForecastOut < 180000 || got.TypicalIncome != 600000 {
		t.Fatalf("unexpected forecast %+v", got)
	}

	all, err := api.Analytics(0)
	// All time starts at the oldest entry or value logged: the log 14
	// months back.
	if err != nil || len(all.Monthly) != 15 || all.RangeStart != monthsAgo(14)[:7] || len(all.Spending) != 1 {
		t.Fatalf("all time covers every month since the first log: %+v %v", all.Monthly, err)
	}

	if _, err := api.Analytics(-1); err == nil {
		t.Fatal("expected a negative range to fail")
	}
}

func TestAnalyticsWithoutData(t *testing.T) {
	api := newTestAPI(t)

	got, err := api.Analytics(12)
	if err != nil {
		t.Fatal(err)
	}

	if got.HasSavingsRate || got.HasRunway || got.HasFixedShare || got.HasChange || len(got.NetWorthHistory) != 1 || len(got.Monthly) != 12 || len(got.Forecast) != 0 {
		t.Fatalf("unexpected empty analytics %+v", got)
	}
}

func TestEnteringNetWorth(t *testing.T) {
	api := newTestAPI(t)
	thisMonth := time.Now().Format(monthLayout)
	backfill := monthsAgo(3)[:7]

	if err := api.SetNetWorth(NetWorthInput{Month: backfill, Amount: "-250.50"}); err != nil {
		t.Fatal(err)
	}

	got, err := api.Analytics(12)
	if err != nil {
		t.Fatal(err)
	}

	sources := map[string]string{}
	for _, p := range got.NetWorthHistory {
		sources[p.Month] = p.Source
	}
	// No values were logged, so the months between have no estimate.
	if len(got.NetWorthHistory) != 2 || got.NetWorthHistory[0].Cents != -25050 || sources[backfill] != SourceEntered || sources[thisMonth] != SourceSaved {
		t.Fatalf("expected the backfill and this month: %+v", got.NetWorthHistory)
	}

	// Entering this month sticks, though the app keeps it on every load.
	if err := api.SetNetWorth(NetWorthInput{Month: thisMonth, Amount: "1000"}); err != nil {
		t.Fatal(err)
	}
	got, _ = api.Analytics(12)
	if last := got.NetWorthHistory[len(got.NetWorthHistory)-1]; last.Source != SourceEntered || last.Cents != 100000 {
		t.Fatalf("expected the entered value to stay: %+v", last)
	}

	// Deleting goes back to the app's value, and the backfill to nothing.
	for _, month := range []string{thisMonth, backfill} {
		if err := api.DeleteNetWorth(month); err != nil {
			t.Fatal(err)
		}
	}
	got, _ = api.Analytics(12)
	if len(got.NetWorthHistory) != 1 || got.NetWorthHistory[0].Source != SourceSaved || got.NetWorthHistory[0].Cents != 0 {
		t.Fatalf("expected only this month, kept by the app: %+v", got.NetWorthHistory)
	}

	future := time.Now().AddDate(0, 2, 0).Format(monthLayout)
	if err := api.SetNetWorth(NetWorthInput{Month: future, Amount: "1"}); err == nil {
		t.Fatal("expected a future month to fail")
	}
	if err := api.DeleteNetWorth(backfill); err == nil {
		t.Fatal("expected nothing left to delete")
	}
}
