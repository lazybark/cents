package cashflow

import (
	"github.com/lazybark/cents/dates"
	"sort"
	"time"
)

// MonthStart returns the first moment of value's month in local time,
// value read as the day it was saved (see dates.MonthOf).
func MonthStart(value time.Time) time.Time {
	return dates.MonthOf(value)
}

// ForMonth returns the entries dated in month, newest first.
func ForMonth(entries []CashflowEntry, month time.Time) []CashflowEntry {
	month = MonthStart(month)
	filtered := make([]CashflowEntry, 0)

	for _, entry := range entries {
		if MonthStart(entry.EntryDate).Equal(month) {
			filtered = append(filtered, entry)
		}
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].EntryDate.Equal(filtered[j].EntryDate) {
			return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
		}

		return filtered[i].EntryDate.After(filtered[j].EntryDate)
	})

	return filtered
}

// Totals sums income and expense in the base currency, at each entry's
// recorded rate. Entries without a rate are skipped and counted in
// missingRates.
func Totals(entries []CashflowEntry) (income int64, expense int64, missingRates int) {
	for _, entry := range entries {
		amountBase, ok := entry.BaseCents()
		if !ok {
			missingRates++

			continue
		}

		if entry.IsIncome {
			income += amountBase
		} else {
			expense += amountBase
		}
	}

	return income, expense, missingRates
}

// MonthlyOverview totals every month from the oldest to the newest entry,
// oldest first, including empty months in between. Entries without a
// recorded rate are skipped and counted in missingRates.
func MonthlyOverview(entries []CashflowEntry) ([]CashflowMonthlyOverviewRow, int) {
	totalsByMonth := make(map[time.Time]CashflowMonthlyOverviewRow)
	missingRates := 0

	for _, entry := range entries {
		amountBase, ok := entry.BaseCents()
		if !ok {
			missingRates++

			continue
		}

		month := MonthStart(entry.EntryDate)
		row := totalsByMonth[month]
		row.Month = month

		if entry.IsIncome {
			row.IncomeBase += amountBase
		} else {
			row.ExpenseBase += amountBase
		}

		totalsByMonth[month] = row
	}

	if len(totalsByMonth) == 0 {
		return nil, missingRates
	}

	var minMonth, maxMonth time.Time
	for month := range totalsByMonth {
		if minMonth.IsZero() || month.Before(minMonth) {
			minMonth = month
		}

		if month.After(maxMonth) {
			maxMonth = month
		}
	}

	rows := make([]CashflowMonthlyOverviewRow, 0)

	var prevNet int64

	hasPrev := false

	for month := minMonth; !month.After(maxMonth); month = MonthStart(month.AddDate(0, 1, 0)) {
		row := totalsByMonth[month]
		row.Month = month
		row.NetBase = row.IncomeBase - row.ExpenseBase
		if hasPrev {
			row.HasPrev = true
			row.DeltaFromPrev = row.NetBase - prevNet
		}

		rows = append(rows, row)
		prevNet = row.NetBase
		hasPrev = true
	}

	return rows, missingRates
}
