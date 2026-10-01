package summary

import (
	"testing"
	"time"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/asset"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/debt"
	"github.com/lazybark/cents/flows/goal"
	"github.com/lazybark/cents/flows/invoice"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/subscription"
	"github.com/lazybark/cents/flows/tax"
)

func TestCompute(t *testing.T) {
	month := time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local)
	inMonth := time.Date(2026, 9, 12, 10, 0, 0, 0, time.Local)
	otherMonth := time.Date(2026, 8, 30, 10, 0, 0, 0, time.Local)

	data := Data{
		Settings: settings.AppSettings{
			BaseCurrency: "€",
			Currencies:   []settings.SettingCurrency{{CurrencyName: "USD", RateToBase: 0.5}},
		},
		Accounts: []account.Account{
			{Currency: "€", BalanceCents: 100000},
			{Currency: "USD", BalanceCents: 20000},
			{Currency: "€", BalanceCents: 99999, IgnoreInSummaries: true},
		},
		Subscriptions: []subscription.Subscription{
			{Currency: "€", AmountCents: 1000, Period: "month", IsActive: true},
			{Currency: "USD", AmountCents: 12000, Period: "year", IsActive: true},
			{Currency: "€", AmountCents: 5000, Period: "month", IsActive: false},
		},
		// Debts, invoices and incomes/expenses count at the rate recorded
		// with each (USD ones at 0.25, not today's 0.5); BTC has none.
		Debts: []debt.Debt{
			{Currency: "€", AmountCents: 3000, AmountPaidCents: 1000, IsOwedToUser: true, RateToBase: 1, AmountBaseCents: 3000, AmountPaidBaseCents: 1000},
			{Currency: "USD", AmountCents: 4000, IsOwedToUser: false, RateToBase: 0.25, AmountBaseCents: 1000},
			{Currency: "€", AmountCents: 500, AmountPaidCents: 500, IsOwedToUser: false, RateToBase: 1, AmountBaseCents: 500, AmountPaidBaseCents: 500},
			{Currency: "BTC", AmountCents: 1, IsOwedToUser: false},
		},
		// Taxes count with the base amounts recorded with them, whatever
		// the currency's rate in settings is now (EUR is 1.08 here).
		Taxes: []tax.Tax{
			{Currency: "$", AmountDueCents: 7000, AmountPaidCents: 2000, AmountDueBaseCents: 7000, AmountPaidBaseCents: 2000},
			{Currency: "$", AmountDueCents: 100, AmountPaidCents: 100, AmountDueBaseCents: 100, AmountPaidBaseCents: 100},
			{Currency: "EUR", AmountDueCents: 1000, AmountPaidCents: 500, AmountDueBaseCents: 1500, AmountPaidBaseCents: 750},
		},
		// Incoming invoices are the ones the user has to pay.
		Invoices: []invoice.Invoice{
			{Currency: "€", AmountCents: 800, IsIncoming: false, RateToBase: 1, AmountBaseCents: 800},
			{Currency: "USD", AmountCents: 600, IsIncoming: true, RateToBase: 0.25, AmountBaseCents: 150},
			{Currency: "€", AmountCents: 999, IsIncoming: true, Paid: true, RateToBase: 1, AmountBaseCents: 999},
		},
		Goals: []goal.Goal{
			{Currency: "€", TargetAmountCents: 10000, AmountAccumulatedCents: 2500},
			{Currency: "€", TargetAmountCents: 1000, AmountAccumulatedCents: 5000},
		},
		// Assets count at today's rates, like accounts.
		Assets: []asset.Asset{
			{Kind: "property", Currency: "€", ValueCents: 300000},
			{Kind: "property", Currency: "€", ValueCents: 999, IgnoreInNetWorth: true},
			{Kind: "investment", Currency: "USD", ValueCents: 8000},
			{Kind: "investment", Currency: "BTC", ValueCents: 5},
		},
		Cashflows: []cashflow.CashflowEntry{
			{Currency: "€", AmountCents: 50000, IsIncome: true, EntryDate: inMonth, RateToBase: 1, AmountBaseCents: 50000},
			{Currency: "USD", AmountCents: 10000, IsIncome: false, EntryDate: inMonth, RateToBase: 0.25, AmountBaseCents: 2500},
			{Currency: "€", AmountCents: 77777, IsIncome: true, EntryDate: otherMonth, RateToBase: 1, AmountBaseCents: 77777},
		},
	}

	got := Compute(data, month)
	want := Summary{
		MonthlyNetCents:           50000 - 2500,
		MonthlySubscriptionsCents: 1000,
		YearlySubscriptionsCents:  6000 + 12*1000,
		AccountsCents:             100000 + 10000,
		PropertyCents:             300000,
		InvestmentsCents:          4000,
		DebtsToMeCents:            2000,
		DebtsByMeCents:            1000,
		UnpaidTaxesCents:          5000 + 750,
		InvoicesToMeCents:         800,
		InvoicesByMeCents:         150,
		GoalsAccumulatedCents:     2500 + 1000,
		GoalsTargetCents:          11000,
	}

	if got != want {
		t.Fatalf("summary mismatch\n got: %+v\nwant: %+v", got, want)
	}

	if net := got.NetWorthCents(); net != 110000+300000+4000+2000+800-1000-5750-150 {
		t.Fatalf("unexpected net worth %d", net)
	}
}
