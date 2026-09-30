// Package summary computes the home-screen figures (financial summary and
// obligations) shared by the TUI and the desktop app. All amounts are in the
// base currency; entries without a conversion rate are skipped.
package summary

import (
	"time"

	"github.com/lazybark/cents/flows/account"
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
}

type Summary struct {
	// Financial summary.
	MonthlyNetCents           int64
	MonthlySubscriptionsCents int64
	YearlySubscriptionsCents  int64
	AccountsCents             int64

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
// they owe: accounts + debts and invoices owed to them - debts, taxes and
// invoices they owe.
func (s Summary) NetWorthCents() int64 {
	return s.AccountsCents + s.DebtsToMeCents + s.InvoicesToMeCents - s.DebtsByMeCents - s.UnpaidTaxesCents - s.InvoicesByMeCents
}

// Compute builds the summary. month selects which month's cashflows make up
// the monthly net.
func Compute(data Data, month time.Time) Summary {
	stts := data.Settings
	var result Summary

	income, expense := monthlyCashflow(data.Cashflows, stts, month)
	result.MonthlyNetCents = income - expense

	active := make([]subscription.Subscription, 0, len(data.Subscriptions))
	for _, sub := range data.Subscriptions {
		if sub.IsActive {
			active = append(active, sub)
		}
	}

	result.MonthlySubscriptionsCents, result.YearlySubscriptionsCents = subscription.TotalsInBaseCents(active, stts)
	result.AccountsCents = account.SumInBaseCents(data.Accounts, stts)

	for _, d := range data.Debts {
		if d.AmountPaidCents >= d.AmountCents {
			continue
		}

		converted, ok := stts.ConvertToBaseCents(d.Currency, d.AmountCents-d.AmountPaidCents)
		if !ok {
			continue
		}

		if d.IsOwedToUser {
			result.DebtsToMeCents += converted
		} else {
			result.DebtsByMeCents += converted
		}
	}

	// Tax amounts are always stored in the base currency.
	for _, t := range data.Taxes {
		if t.AmountPaidCents < t.AmountDueCents {
			result.UnpaidTaxesCents += t.AmountDueCents - t.AmountPaidCents
		}
	}

	for _, inv := range data.Invoices {
		if inv.Paid {
			continue
		}

		converted, ok := stts.ConvertToBaseCents(inv.Currency, inv.AmountCents)
		if !ok {
			continue
		}

		if inv.IsIncoming {
			result.InvoicesToMeCents += converted
		} else {
			result.InvoicesByMeCents += converted
		}
	}

	result.GoalsAccumulatedCents, result.GoalsTargetCents = goal.ProgressInBaseCents(data.Goals, stts)

	return result
}

func monthlyCashflow(entries []cashflow.CashflowEntry, stts settings.AppSettings, month time.Time) (income int64, expense int64) {
	month = month.Local()

	for _, entry := range entries {
		day := entry.EntryDate.Local()
		if day.Year() != month.Year() || day.Month() != month.Month() {
			continue
		}

		amountBase, ok := stts.ConvertToBaseCents(entry.Currency, entry.AmountCents)
		if !ok {
			continue
		}

		if entry.IsIncome {
			income += amountBase
		} else {
			expense += amountBase
		}
	}

	return income, expense
}
