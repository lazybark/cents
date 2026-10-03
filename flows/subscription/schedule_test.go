package subscription

import (
	"testing"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/settings"
)

func d(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, time.UTC) }

func ptr(t time.Time) *time.Time { return &t }

func TestNextPaymentRollsForward(t *testing.T) {
	today := d(2026, 10, 2)
	cases := []struct {
		name string
		sub  Subscription
		want time.Time
	}{
		{"future date stays", Subscription{Period: PeriodMonth, NextPaymentDate: ptr(d(2026, 12, 15))}, d(2026, 12, 15)},
		{"today is today", Subscription{Period: PeriodMonth, NextPaymentDate: ptr(d(2026, 10, 2))}, d(2026, 10, 2)},
		{"passed monthly rolls on", Subscription{Period: PeriodMonth, NextPaymentDate: ptr(d(2026, 8, 18))}, d(2026, 10, 18)},
		{"31st in a short month", Subscription{Period: PeriodMonth, NextPaymentDate: ptr(d(2026, 1, 31))}, d(2026, 10, 31)},
		{"weekly", Subscription{Period: PeriodWeek, NextPaymentDate: ptr(d(2026, 9, 1))}, d(2026, 10, 6)},
		{"quarterly", Subscription{Period: PeriodQuarter, NextPaymentDate: ptr(d(2026, 1, 10))}, d(2026, 10, 10)},
		{"yearly", Subscription{Period: PeriodYear, NextPaymentDate: ptr(d(2024, 2, 29))}, d(2027, 2, 28)},
		{"TUI yearly date", Subscription{Period: PeriodYear, PaymentDateYearly: "16.02.2027"}, d(2027, 2, 16)},
		{"TUI monthly day, passed", Subscription{Period: PeriodMonth, PaymentDayMonthly: intPtr(1)}, d(2026, 11, 1)},
		{"TUI monthly day, ahead", Subscription{Period: PeriodMonth, PaymentDayMonthly: intPtr(18)}, d(2026, 10, 18)},
	}

	for _, c := range cases {
		got, ok := c.sub.NextPayment(today)
		if !ok || !got.Equal(c.want) {
			t.Errorf("%s: want %s, got %s %v", c.name, c.want.Format("2006-01-02"), got.Format("2006-01-02"), ok)
		}
	}

	// February then back to the 31st: the anchor day is kept.
	feb := Subscription{Period: PeriodMonth, NextPaymentDate: ptr(d(2027, 1, 31))}
	if got, _ := feb.NextPayment(d(2027, 2, 2)); !got.Equal(d(2027, 2, 28)) {
		t.Fatalf("expected 28 Feb, got %s", got)
	}

	if got, _ := feb.NextPayment(d(2027, 3, 1)); !got.Equal(d(2027, 3, 31)) {
		t.Fatalf("expected back on the 31st, got %s", got)
	}

	if _, ok := (Subscription{Period: PeriodMonth}).NextPayment(today); ok {
		t.Fatal("no schedule without a date")
	}
}

func intPtr(v int) *int { return &v }

func TestMarkPaidAndSoon(t *testing.T) {
	today := d(2026, 10, 2)
	sub := Subscription{IsActive: true, Period: PeriodMonth, NextPaymentDate: ptr(d(2026, 8, 31))}

	next, days, soon, ok := sub.DueIn(today)
	if !ok || !next.Equal(d(2026, 10, 31)) || days != 29 || soon {
		t.Fatalf("unexpected %s %d %v", next, days, soon)
	}

	if _, days, soon, _ := sub.DueIn(d(2026, 10, 27)); days != 4 || !soon {
		t.Fatalf("4 days before a monthly payment is soon: %d %v", days, soon)
	}

	paid, paidFor, err := sub.MarkPaid(d(2026, 10, 27), today)
	if err != nil || !paidFor.Equal(d(2026, 10, 31)) {
		t.Fatal(err, paidFor)
	}

	if next, _ := paid.NextPayment(d(2026, 10, 27)); !next.Equal(d(2026, 11, 30)) {
		t.Fatalf("paid early: next should be November's, got %s", next)
	}

	if next, _ := paid.NextPayment(d(2026, 12, 1)); !next.Equal(d(2026, 12, 31)) {
		t.Fatalf("the 31st comes back after November: %s", next)
	}

	yearly := Subscription{IsActive: true, Period: PeriodYear, NextPaymentDate: ptr(d(2026, 10, 30))}
	inactive := Subscription{IsActive: false, Period: PeriodYear, NextPaymentDate: ptr(d(2026, 10, 3))}
	later := Subscription{IsActive: true, Period: PeriodYear, NextPaymentDate: ptr(d(2026, 11, 5))}
	weekly := Subscription{IsActive: true, Period: PeriodWeek, NextPaymentDate: ptr(d(2026, 10, 3))}
	upcoming := Upcoming([]Subscription{yearly, inactive, later, weekly, sub}, today)
	if len(upcoming) != 2 || !upcoming[0].NextPaymentDate.Equal(d(2026, 10, 3)) || upcoming[1].Period != PeriodYear {
		t.Fatalf("expected the weekly then the yearly: %+v", upcoming)
	}

	if _, _, err := (Subscription{Period: PeriodMonth}).MarkPaid(today, today); err == nil {
		t.Fatal("expected an error without a schedule")
	}
}

func TestUpdate(t *testing.T) {
	now := time.Now()
	ok := Fields{Name: " Rent ", Type: "Rent", Currency: "€", Amount: "950", Period: "Month", PaymentMethod: "Bank", NextPayment: "2026-10-31", IsActive: true}

	// Payments marked paid are kept: a new date continues after them.
	sub, err := Subscription{ID: 4, LastPaidDate: ptr(d(2026, 9, 30))}.Update(ok, dates.ISO, now)
	if err != nil || sub.ID != 4 || sub.Name != "Rent" || sub.Period != "month" || sub.AmountCents != 95000 || !sub.NextPaymentDate.Equal(d(2026, 10, 31)) || !sub.LastPaidDate.Equal(d(2026, 9, 30)) {
		t.Fatalf("unexpected %+v %v", sub, err)
	}

	if sub.PaymentDayMonthly == nil || *sub.PaymentDayMonthly != 31 || sub.PaymentDateYearly != "" {
		t.Fatalf("monthly day should follow for the TUI: %+v", sub)
	}

	yearly := ok
	yearly.Period = "year"
	sub, _ = sub.Update(yearly, dates.ISO, now)
	if sub.PaymentDateYearly != "31.10.2026" || sub.PaymentDayMonthly != nil {
		t.Fatalf("yearly date should follow for the TUI: %+v", sub)
	}

	cases := map[string]func(*Fields){
		"subscription name is required":                    func(f *Fields) { f.Name = "" },
		"type is required":                                 func(f *Fields) { f.Type = " " },
		"period must be one of week, month, quarter, year": func(f *Fields) { f.Period = "daily" },
		"next payment date must use YYYY-MM-DD format":     func(f *Fields) { f.NextPayment = "31.10.2026" },
		"payment method is required":                       func(f *Fields) { f.PaymentMethod = "" },
	}
	for want, change := range cases {
		f := ok
		change(&f)
		if _, err := (Subscription{}).Update(f, dates.ISO, now); err == nil || err.Error() != want {
			t.Errorf("want %q, got %v", want, err)
		}
	}
}

func TestTotalsCountEveryPeriod(t *testing.T) {
	stts := settings.AppSettings{BaseCurrency: "$"}
	subs := []Subscription{
		{Currency: "$", AmountCents: 1200, Period: "week"},
		{Currency: "$", AmountCents: 1000, Period: "month"},
		{Currency: "$", AmountCents: 3000, Period: "quarter"},
		{Currency: "$", AmountCents: 10000, Period: "year"},
	}

	monthly, yearly := TotalsInBaseCents(subs, stts)
	if monthly != 1200*52/12+1000 || yearly != 1200*52+1000*12+3000*4+10000 {
		t.Fatalf("unexpected totals %d %d", monthly, yearly)
	}
}

func TestPaidManuallyStaysDue(t *testing.T) {
	today := d(2026, 10, 10)
	rent := Subscription{IsActive: true, PaidManually: true, Period: PeriodMonth, NextPaymentDate: ptr(d(2026, 9, 4))}

	next, days, soon, _ := rent.DueIn(today)
	if !next.Equal(d(2026, 9, 4)) || days != -36 || !soon {
		t.Fatalf("an unpaid manual payment stays due: %s %d %v", next, days, soon)
	}

	paid, paidFor, _ := rent.MarkPaid(today, today)
	if !paidFor.Equal(d(2026, 9, 4)) {
		t.Fatalf("marks the oldest unpaid one: %s", paidFor)
	}

	if next, days, _, _ := paid.DueIn(today); !next.Equal(d(2026, 10, 4)) || days != -6 {
		t.Fatalf("then October's is overdue: %s %d", next, days)
	}

	paid, _, _ = paid.MarkPaid(today, today)
	if next, days, soon, _ := paid.DueIn(today); !next.Equal(d(2026, 11, 4)) || days != 25 || soon {
		t.Fatalf("paid up: next is November's: %s %d %v", next, days, soon)
	}

	// The same schedule charged automatically never goes overdue.
	auto := rent
	auto.PaidManually = false
	if next, _ := auto.NextPayment(today); !next.Equal(d(2026, 11, 4)) {
		t.Fatalf("an automatic payment rolls on: %s", next)
	}

	// Rolling back: without the latest record, the one before is next.
	paid.LastPaidDate = ptr(d(2026, 9, 4))
	if next, _ := paid.NextPayment(today); !next.Equal(d(2026, 10, 4)) {
		t.Fatalf("rolled back: %s", next)
	}
}

func TestPaidUpTo(t *testing.T) {
	sub := Subscription{Period: PeriodMonth, NextPaymentDate: ptr(d(2026, 10, 4)), LastPaidDate: ptr(d(2026, 11, 4))}
	got := sub.PaidUpTo(d(2026, 10, 3), 12)
	if len(got) != 2 || !got[0].Equal(d(2026, 10, 4)) || !got[1].Equal(d(2026, 11, 4)) {
		t.Fatalf("expected October and November: %v", got)
	}

	weekly := Subscription{Period: PeriodWeek, NextPaymentDate: ptr(d(2025, 1, 1)), LastPaidDate: ptr(d(2026, 10, 1))}
	if got := weekly.PaidUpTo(d(2026, 10, 3), 12); len(got) != 12 || !got[11].Equal(d(2026, 9, 30)) {
		t.Fatalf("expected the last 12: %v", got)
	}

	if (Subscription{Period: PeriodMonth}).PaidUpTo(d(2026, 10, 3), 12) != nil {
		t.Fatal("nothing paid, nothing to list")
	}
}
