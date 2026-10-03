package analytics

import (
	"testing"
	"time"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/asset"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/credit"
	"github.com/lazybark/cents/flows/debt"
	"github.com/lazybark/cents/flows/invoice"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/subscription"
	"github.com/lazybark/cents/flows/tax"
	"github.com/lazybark/cents/summary"
)

func d(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, time.UTC) }
func ptr(t time.Time) *time.Time               { return &t }

var stts = settings.AppSettings{BaseCurrency: "€", Currencies: []settings.SettingCurrency{{CurrencyName: "$", RateToBase: 0.5}}}

func TestNetWorthHistory(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.Local)
	h := History{
		Settings: stts,
		Snapshots: []NetWorthSnapshot{
			{Month: d(2026, 3, 1), NetWorthCents: 5000},
			{Month: d(2026, 4, 1), NetWorthCents: 6000},
		},
		Accounts: []account.Account{
			{ID: 1, Currency: "€"},
			{ID: 2, Currency: "$"},
			{ID: 3, Currency: "€", IgnoreInSummaries: true},
		},
		AccountLogs: []account.AccountValueLog{
			{AccountID: 1, LogDate: d(2026, 1, 10), ValueCents: 1000},
			{AccountID: 1, LogDate: d(2026, 1, 31), ValueCents: 1200},
			{AccountID: 2, LogDate: d(2026, 2, 5), ValueCents: 400},
			{AccountID: 3, LogDate: d(2025, 12, 1), ValueCents: 99999},
		},
		Assets:    []asset.Asset{{ID: 7, Currency: "€"}},
		AssetLogs: []asset.AssetValueLog{{AssetID: 7, LogDate: d(2026, 2, 20), ValueCents: 300}},
	}

	got := NetWorthHistory(h, now)
	want := []MonthValue{
		{Month: d(2026, 1, 1), Cents: 1200, Estimated: true},
		{Month: d(2026, 2, 1), Cents: 1200 + 200 + 300, Estimated: true},
		{Month: d(2026, 3, 1), Cents: 5000},
		{Month: d(2026, 4, 1), Cents: 6000},
	}

	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("month %d: want %+v, got %+v", i, want[i], got[i])
		}
	}

	// A value logged on the first of a month at local midnight belongs to
	// that month, whatever the time zone.
	boundary := History{
		Settings:    stts,
		Accounts:    []account.Account{{ID: 1, Currency: "€"}},
		AccountLogs: []account.AccountValueLog{{AccountID: 1, LogDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local), ValueCents: 700}, {AccountID: 1, LogDate: time.Date(2026, 2, 10, 0, 0, 0, 0, time.Local), ValueCents: 100}},
	}
	if got := NetWorthHistory(boundary, now); got[0].Cents != 100 || got[1].Cents != 700 {
		t.Fatalf("expected February at 100 and March at 700: %+v", got)
	}

	if none := NetWorthHistory(History{Settings: stts}, now); len(none) != 0 {
		t.Fatalf("without data, nothing: %+v", none)
	}
}

func TestAveragesAndSpending(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.Local)
	entry := func(income bool, cents int64, category string, y int, m time.Month) cashflow.CashflowEntry {
		return cashflow.CashflowEntry{IsIncome: income, AmountCents: cents, RateToBase: 1, AmountBaseCents: cents, Category: category, EntryDate: time.Date(y, m, 5, 0, 0, 0, 0, time.Local)}
	}

	entries := []cashflow.CashflowEntry{
		entry(true, 1000, "Salary", 2026, 2),
		entry(false, 600, "Rent", 2026, 2),
		entry(true, 1000, "Salary", 2026, 3),
		entry(false, 300, "rent", 2026, 3),
		entry(false, 100, "Food", 2026, 3),
		entry(false, 9999, "Food", 2026, 4), // the running month
		entry(false, 50, "Old", 2025, 1),
	}

	rows, _ := cashflow.MonthlyOverview(entries)
	window := Window(rows, now, 2)
	if len(window) != 2 || window[0].Month.Month() != time.February || window[1].Month.Month() != time.March {
		t.Fatalf("expected the last two full months: %+v", window)
	}

	avg := Average(window)
	if avg.Months != 2 || avg.IncomeCents != 1000 || avg.ExpenseCents != 500 || !avg.HasRate || avg.SavingsRate != 50 {
		t.Fatalf("unexpected averages %+v", avg)
	}

	if Average(nil).HasRate {
		t.Fatal("no months, no rate")
	}

	shares := SpendingByCategory(entries, now, 3)
	if len(shares) != 2 || shares[0].Category != "Food" || shares[0].Cents != 10099 || shares[1].Cents != 900 {
		t.Fatalf("expected Food then Rent (merged) over three months: %+v", shares)
	}

	if all := SpendingByCategory(entries, now, 0); len(all) != 3 {
		t.Fatalf("all months: %+v", all)
	}
}

func TestCommittedAndForecast(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.Local)
	subs := []subscription.Subscription{
		{Name: "Music", Currency: "€", AmountCents: 1000, Period: "month", IsActive: true, NextPaymentDate: ptr(d(2026, 4, 20))},
		{Name: "Rent", Currency: "$", AmountCents: 100000, Period: "month", IsActive: true, IsObligation: true, PaidManually: true, NextPaymentDate: ptr(d(2026, 4, 1))},
		{Name: "Old", Currency: "€", AmountCents: 500, Period: "month", IsActive: false, NextPaymentDate: ptr(d(2026, 4, 16))},
	}

	if got := CommittedPerMonth(subs, stts); got != (1000*12+50000*12)/12 {
		t.Fatalf("unexpected committed %d", got)
	}

	data := summary.Data{
		Settings:      stts,
		Subscriptions: subs,
		Debts: []debt.Debt{
			{Peer: "Bank", Currency: "€", AmountCents: 1000, AmountPaidCents: 400, RateToBase: 1, AmountBaseCents: 1000, AmountPaidBaseCents: 400, DueDate: ptr(d(2026, 5, 1))},
			{Peer: "Friend", Currency: "€", AmountCents: 200, RateToBase: 1, AmountBaseCents: 200, IsOwedToUser: true, DueDate: ptr(d(2026, 4, 30))},
			{Peer: "Far", Currency: "€", AmountCents: 200, DueDate: ptr(d(2026, 12, 1))},
		},
		Credits:  []credit.Credit{{Name: "Car", Currency: "€", TotalCents: 500, RateToBase: 1, TotalBaseCents: 500, DueDate: ptr(d(2026, 6, 1))}},
		Taxes:    []tax.Tax{{TaxCountry: "NL", TaxTypeName: "VAT", Period: "Q1", Currency: "€", AmountDueCents: 300, AmountDueBaseCents: 300, RateToBase: 1, DueDate: ptr(d(2026, 4, 10))}},
		Invoices: []invoice.Invoice{{Title: "Design", Currency: "€", AmountCents: 700, RateToBase: 1, AmountBaseCents: 700, DueDate: ptr(d(2026, 5, 10))}},
	}

	items := Forecast(data, now, 90)
	kinds := map[string]int{}
	for _, item := range items {
		kinds[item.Kind]++
	}

	// Rent: overdue April, then May, June, July. Music: April 20 to June 20
	// (the window ends July 14).
	if kinds[DueObligation] != 4 || kinds[DueSubscription] != 3 || kinds[DueDebt] != 2 || kinds[DueCredit] != 1 || kinds[DueTax] != 1 || kinds[DueInvoice] != 1 {
		t.Fatalf("unexpected kinds %v in %+v", kinds, items)
	}

	if first := items[0]; first.Kind != DueObligation || !first.Overdue || first.BaseCents != 50000 || !first.Date.Equal(d(2026, 4, 1)) {
		t.Fatalf("expected the overdue rent first: %+v", first)
	}

	for _, item := range items {
		if item.Name == "Friend" && !item.Incoming || item.Name == "Design" && !item.Incoming || item.Name == "Bank" && (item.Incoming || item.BaseCents != 600) {
			t.Fatalf("unexpected %+v", item)
		}

		if item.Name == "NL / VAT Q1" && !item.Overdue {
			t.Fatalf("the tax is past due: %+v", item)
		}
	}
}
