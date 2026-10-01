package cashflow

import (
	"sort"
	"strings"
	"time"
)

// CategorySeries is one category's amounts month by month, in the base
// currency at each entry's recorded rate, with how many entries made them.
type CategorySeries struct {
	Category string
	Values   []int64
	Entries  []int
}

// ByCategory splits incomes (or expenses) by category, month by month from
// the oldest to the newest month with an entry, empty months included.
// Categories come largest first. Entries without a rate are skipped and
// counted in missingRates.
func ByCategory(entries []CashflowEntry, isIncome bool) (months []time.Time, series []CategorySeries, missingRates int) {
	type key struct {
		category string
		month    time.Time
	}

	amounts := map[key]int64{}
	counts := map[key]int{}
	names := map[string]string{}
	var first, last time.Time

	for _, entry := range entries {
		if entry.IsIncome != isIncome {
			continue
		}

		base, ok := entry.BaseCents()
		if !ok {
			missingRates++

			continue
		}

		category := strings.TrimSpace(entry.Category)
		id := strings.ToLower(category)
		if _, seen := names[id]; !seen {
			names[id] = category
		}

		month := MonthStart(entry.EntryDate)
		amounts[key{id, month}] += base
		counts[key{id, month}]++

		if first.IsZero() || month.Before(first) {
			first = month
		}

		if month.After(last) {
			last = month
		}
	}

	if first.IsZero() {
		return nil, nil, missingRates
	}

	for month := first; !month.After(last); month = MonthStart(month.AddDate(0, 1, 0)) {
		months = append(months, month)
	}

	totals := map[string]int64{}
	for id, name := range names {
		s := CategorySeries{Category: name, Values: make([]int64, len(months)), Entries: make([]int, len(months))}
		for i, month := range months {
			s.Values[i] = amounts[key{id, month}]
			s.Entries[i] = counts[key{id, month}]
			totals[name] += s.Values[i]
		}

		series = append(series, s)
	}

	sort.Slice(series, func(i, j int) bool {
		if totals[series[i].Category] != totals[series[j].Category] {
			return totals[series[i].Category] > totals[series[j].Category]
		}

		return series[i].Category < series[j].Category
	})

	return months, series, missingRates
}
