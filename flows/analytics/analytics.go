// Package analytics answers the bigger questions about the data: is net
// worth growing, how much of income is kept, how long savings would last,
// how much spending is locked in, and what is coming due.
package analytics

import (
	"github.com/lazybark/cents/dates"
	"sort"
	"strings"
	"time"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/asset"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/subscription"
	"github.com/lazybark/cents/summary"
)

// NetWorthSnapshot is net worth as it was in a month, kept so its history
// can be shown. The current month's is updated until the month is over,
// unless the user entered it (Manual): those are never replaced by the app.
type NetWorthSnapshot struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Month         time.Time `gorm:"not null;uniqueIndex"`
	NetWorthCents int64
	// What it's made of: what's owned (accounts, property, investments)
	// and owed to or by the user.
	OwnedCents    int64
	OwedToMeCents int64
	OwedByMeCents int64
	// Manual is set on values the user entered, which have no breakdown.
	Manual bool `gorm:"not null;default:false"`
}

// MonthStart is the first day of value's month (read as the day it was
// saved, see dates.Day), as a UTC midnight like snapshots store it.
func MonthStart(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// Snapshot is net worth now, for month.
func Snapshot(sum summary.Summary, month time.Time) NetWorthSnapshot {
	return NetWorthSnapshot{
		Month:         MonthStart(month),
		NetWorthCents: sum.NetWorthCents(),
		OwnedCents:    sum.AccountsCents + sum.PropertyCents + sum.InvestmentsCents,
		OwedToMeCents: sum.DebtsToMeCents + sum.InvoicesToMeCents,
		OwedByMeCents: sum.DebtsByMeCents + sum.UnpaidTaxesCents + sum.InvoicesByMeCents + sum.CreditsCents,
	}
}

// MonthValue is one month of the net worth history. Estimated months have
// no snapshot: they count only what's owned (accounts, property and
// investments, from their value history), as debts and the like have no
// history to go by.
type MonthValue struct {
	Month     time.Time
	Cents     int64
	Estimated bool
	// Manual is a value the user entered.
	Manual bool
}

// History is the input for NetWorthHistory.
type History struct {
	Snapshots   []NetWorthSnapshot
	Accounts    []account.Account
	AccountLogs []account.AccountValueLog
	Assets      []asset.Asset
	AssetLogs   []asset.AssetValueLog
	Settings    settings.AppSettings
}

// NetWorthHistory is net worth month by month, from the oldest snapshot or
// value logged to this month: the snapshot where there is one, otherwise
// an estimate. Months with neither are left out.
func NetWorthHistory(h History, now time.Time) []MonthValue {
	snapshots := map[time.Time]NetWorthSnapshot{}
	first := MonthStart(now)
	for _, s := range h.Snapshots {
		month := MonthStart(s.Month)
		snapshots[month] = s
		if month.Before(first) {
			first = month
		}
	}

	// Only values that count can start the history: logs of accounts
	// ignored in summaries would add months of nothing.
	counted := map[uint]bool{}
	for _, acct := range h.Accounts {
		counted[acct.ID] = !acct.IgnoreInSummaries
	}

	for _, l := range h.AccountLogs {
		if m := MonthStart(l.LogDate); counted[l.AccountID] && m.Before(first) {
			first = m
		}
	}

	countedAssets := map[uint]bool{}
	for _, item := range h.Assets {
		countedAssets[item.ID] = !item.IgnoreInNetWorth
	}

	for _, l := range h.AssetLogs {
		if m := MonthStart(l.LogDate); countedAssets[l.AssetID] && m.Before(first) {
			first = m
		}
	}

	accountLogs := map[uint][]account.AccountValueLog{}
	for _, l := range h.AccountLogs {
		accountLogs[l.AccountID] = append(accountLogs[l.AccountID], l)
	}

	assetLogs := map[uint][]asset.AssetValueLog{}
	for _, l := range h.AssetLogs {
		assetLogs[l.AssetID] = append(assetLogs[l.AssetID], l)
	}

	values := []MonthValue{}
	for month := first; !month.After(MonthStart(now)); month = month.AddDate(0, 1, 0) {
		if s, ok := snapshots[month]; ok {
			values = append(values, MonthValue{Month: month, Cents: s.NetWorthCents, Manual: s.Manual})
			continue
		}

		end := month.AddDate(0, 1, 0)
		var owned int64
		// Whether any value was logged by then; without one there's nothing
		// to estimate from, and the month is left out rather than shown as 0.
		known := false
		for _, acct := range h.Accounts {
			if acct.IgnoreInSummaries {
				continue
			}

			if cents, ok := latestBefore(accountLogs[acct.ID], end, func(l account.AccountValueLog) (time.Time, int64) { return l.LogDate, l.ValueCents }); ok {
				if base, ok := h.Settings.ConvertToBaseCents(acct.Currency, cents); ok {
					owned += base
					known = true
				}
			}
		}

		for _, item := range h.Assets {
			if item.IgnoreInNetWorth {
				continue
			}

			if cents, ok := latestBefore(assetLogs[item.ID], end, func(l asset.AssetValueLog) (time.Time, int64) { return l.LogDate, l.ValueCents }); ok {
				if base, ok := h.Settings.ConvertToBaseCents(item.Currency, cents); ok {
					owned += base
					known = true
				}
			}
		}

		if known {
			values = append(values, MonthValue{Month: month, Cents: owned, Estimated: true})
		}
	}

	return values
}

// latestBefore is the value of the latest log dated before end, a UTC
// midnight.
func latestBefore[T any](logs []T, end time.Time, read func(T) (time.Time, int64)) (int64, bool) {
	var (
		best  time.Time
		value int64
		found bool
	)

	for _, l := range logs {
		day, cents := read(l)
		if dates.Day(day).Before(end) && (!found || day.After(best)) {
			best, value, found = day, cents, true
		}
	}

	return value, found
}

// Window is the last full months before now (the current month is still
// running), at most n of them, oldest first; fewer when entries started
// later.
func Window(rows []cashflow.CashflowMonthlyOverviewRow, now time.Time, n int) []cashflow.CashflowMonthlyOverviewRow {
	current := cashflow.MonthStart(now)
	full := []cashflow.CashflowMonthlyOverviewRow{}
	for _, row := range rows {
		if row.Month.Before(current) {
			full = append(full, row)
		}
	}

	if len(full) > n {
		full = full[len(full)-n:]
	}

	return full
}

// Averages are per month over a window of months.
type Averages struct {
	Months       int
	IncomeCents  int64
	ExpenseCents int64
	// SavingsRate is the share of income kept, in percent; HasRate is false
	// without income.
	SavingsRate float64
	HasRate     bool
}

func Average(window []cashflow.CashflowMonthlyOverviewRow) Averages {
	var income, expense int64
	for _, row := range window {
		income += row.IncomeBase
		expense += row.ExpenseBase
	}

	a := Averages{Months: len(window)}
	if a.Months == 0 {
		return a
	}

	a.IncomeCents = income / int64(a.Months)
	a.ExpenseCents = expense / int64(a.Months)
	a.SavingsRate, a.HasRate = SavingsRate(income, expense)

	return a
}

// CategoryShare is a category's part of spending over some months.
type CategoryShare struct {
	Category string
	Cents    int64
	Share    float64
}

// SpendingByCategory totals expenses by category over the last months
// months (0 for all), largest first, with each one's share.
func SpendingByCategory(entries []cashflow.CashflowEntry, now time.Time, months int) []CategoryShare {
	from := time.Time{}
	if months > 0 {
		from = cashflow.MonthStart(now).AddDate(0, -(months - 1), 0)
	}

	totals := map[string]int64{}
	names := map[string]string{}
	var all int64
	for _, entry := range entries {
		base, ok := entry.BaseCents()
		if entry.IsIncome || !ok || cashflow.MonthStart(entry.EntryDate).Before(from) {
			continue
		}

		name := strings.TrimSpace(entry.Category)
		key := strings.ToLower(name)
		if _, seen := names[key]; !seen {
			names[key] = name
		}

		totals[key] += base
		all += base
	}

	shares := make([]CategoryShare, 0, len(totals))
	for key, cents := range totals {
		share := 0.0
		if all > 0 {
			share = float64(cents) / float64(all) * 100
		}

		shares = append(shares, CategoryShare{Category: names[key], Cents: cents, Share: share})
	}

	sort.Slice(shares, func(i, j int) bool {
		if shares[i].Cents != shares[j].Cents {
			return shares[i].Cents > shares[j].Cents
		}

		return shares[i].Category < shares[j].Category
	})

	return shares
}

// CommittedPerMonth is what active subscriptions and obligations cost a
// month on average (a year's worth over twelve), in the base currency.
func CommittedPerMonth(subs []subscription.Subscription, stts settings.AppSettings) int64 {
	active, _ := subscription.SplitByActivity(subs)
	_, yearly := subscription.TotalsInBaseCents(active, stts)

	return yearly / 12
}
