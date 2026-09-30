package desktop

import (
	"testing"
	"time"
)

// inZone runs the rest of the test as if the computer were in zone.
func inZone(t *testing.T, zone string) {
	t.Helper()

	loc, err := time.LoadLocation(zone)
	if err != nil {
		t.Skip(err)
	}

	saved := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = saved })
}

// Dates are the day typed, west of UTC too and after moving to another
// time zone: a value logged east of UTC and an entry on the first of the
// month, read in Los Angeles.
func TestDatesStayTheDayTyped(t *testing.T) {
	inZone(t, "Europe/Moscow")
	api := newTestAPI(t)

	if err := api.CreateAccount(NewAccountInput{Name: "Bank", Description: "Main", Currency: "$", Amount: "10"}); err != nil {
		t.Fatal(err)
	}

	accounts, _ := api.Accounts(0)
	id := accounts.Accounts[0].ID
	if err := api.SaveAccountValueLog(ValueLogInput{AccountID: id, Date: "2026-03-01", Value: "500"}); err != nil {
		t.Fatal(err)
	}

	inZone(t, "America/Los_Angeles")

	if _, err := api.CreateCashflow(NewCashflowInput{Currency: "$", Amount: "5", Date: "2026-03-01", Category: "Rent"}); err != nil {
		t.Fatal(err)
	}

	month, err := api.CashflowMonth("2026-03")
	if err != nil || len(month.Entries) != 1 || month.Entries[0].Date != "2026-03-01" {
		t.Fatalf("expected the entry on March 1: %+v %v", month.Entries, err)
	}

	if february, _ := api.CashflowMonth("2026-02"); len(february.Entries) != 0 {
		t.Fatalf("nothing in February: %+v", february.Entries)
	}

	logs, err := api.AccountValueLogs(id)
	found := false
	for _, l := range logs {
		found = found || l.Date == "2026-03-01" && l.ValueCents == 50000
	}
	if err != nil || !found {
		t.Fatalf("expected the value logged on March 1: %+v %v", logs, err)
	}

	values, err := api.AccountMonthlyValues(id)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range values {
		if v.ValueCents == 50000 && v.Month != "2026-03" {
			t.Fatalf("the March value is under %s", v.Month)
		}
	}

	if _, err := api.CreateDebt(NewDebtInput{Peer: "Bank", Currency: "$", Amount: "10", AmountPaid: "0", CreatedAt: "2026-03-01", DueDate: "2099-01-01"}); err != nil {
		t.Fatal(err)
	}

	debts, _ := api.Debts("all")
	if len(debts.Debts) != 1 || debts.Debts[0].CreatedAt != "2026-03-01" || debts.Debts[0].DueDate != "2099-01-01" || debts.Debts[0].Overdue {
		t.Fatalf("unexpected debt %+v", debts.Debts)
	}
}
