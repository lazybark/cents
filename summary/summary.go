// Package summary computes the home-screen figures (financial summary and
// obligations) shared by the TUI and the desktop app. All amounts are in the
// base currency; entries without a conversion rate are skipped.
package summary

import (
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

type Data struct {
	Settings      settings.AppSettings
	Accounts      []account.Account
	Subscriptions []subscription.Subscription
	Debts         []debt.Debt
	Goals         []goal.Goal
	Taxes         []tax.Tax
	Invoices      []invoice.Invoice
	Cashflows     []cashflow.CashflowEntry
	// Assets are property and investments; only the desktop app has them.
	Assets []asset.Asset
}

type Summary struct {
	// Financial summary.
	MonthlyNetCents           int64
	MonthlySubscriptionsCents int64
	YearlySubscriptionsCents  int64
	AccountsCents             int64
	PropertyCents             int64
	InvestmentsCents          int64

	// Obligations. "ToMe" is owed to the user, "ByMe" is owed by the user.
	DebtsToMeCents        int64
	DebtsByMeCents        int64
	UnpaidTaxesCents      int64
	InvoicesToMeCents     int64
	InvoicesByMeCents     int64
	GoalsAccumulatedCents int64
	GoalsTargetCents      int64
}

// NetWorthCents is what the user has plus what they are owed, minus what
// they owe: accounts, property and investments + debts and invoices owed to
// them - debts, taxes and invoices they owe.
func (s Summary) NetWorthCents() int64 {
	return s.AccountsCents + s.PropertyCents + s.InvestmentsCents + s.DebtsToMeCents + s.InvoicesToMeCents - s.DebtsByMeCents - s.UnpaidTaxesCents - s.InvoicesByMeCents
}

// Compute builds the summary. month selects which month's cashflows make up
// the monthly net.
func Compute(data Data, month time.Time) Summary {
	stts := data.Settings
	var result Summary

	income, expense, _ := cashflow.Totals(cashflow.ForMonth(data.Cashflows, month))
	result.MonthlyNetCents = income - expense

	active := make([]subscription.Subscription, 0, len(data.Subscriptions))
	for _, sub := range data.Subscriptions {
		if sub.IsActive {
			active = append(active, sub)
		}
	}

	result.MonthlySubscriptionsCents, result.YearlySubscriptionsCents = subscription.TotalsInBaseCents(active, stts)
	result.AccountsCents = account.SumInBaseCents(data.Accounts, stts)
	result.PropertyCents = asset.Total(asset.Filter(data.Assets, asset.KindProperty), stts).ValueCents
	result.InvestmentsCents = asset.Total(asset.Filter(data.Assets, asset.KindInvestment), stts).ValueCents

	// Debts, invoices and incomes/expenses count at the rate recorded with
	// each; ones without a rate are left out.
	for _, d := range data.Debts {
		if d.IsPaid() {
			continue
		}

		converted, ok := d.LeftBaseCents()
		if !ok {
			continue
		}

		if d.IsOwedToUser {
			result.DebtsToMeCents += converted
		} else {
			result.DebtsByMeCents += converted
		}
	}

	// Taxes keep their base amounts at the rate they were recorded with.
	for _, t := range data.Taxes {
		if !t.IsPaid() {
			result.UnpaidTaxesCents += t.LeftBaseCents()
		}
	}

	result.InvoicesToMeCents, result.InvoicesByMeCents = invoice.UnpaidInBaseCents(data.Invoices)

	result.GoalsAccumulatedCents, result.GoalsTargetCents = goal.ProgressInBaseCents(data.Goals, stts)

	return result
}
