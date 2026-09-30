package analytics

import (
	"sort"
	"strings"
	"time"

	"github.com/lazybark/cents/flows/cashflow"
)

// usualMonths is how many months before make up "usual".
const usualMonths = 12

// CategoryChange is a category's spending in a month against its usual: the
// average over the months before (UsualCents; HasUsual is false when there
// are none). Unusual flags a change worth a look: at least 30% either way
// (or a category new to the usual months) and at least 2% of the usual
// monthly spending.
type CategoryChange struct {
	Category    string
	Cents       int64
	UsualCents  int64
	DiffCents   int64
	DiffPercent float64
	HasUsual    bool
	Unusual     bool
}

// Comparison is a month's spending by category against the usual.
type Comparison struct {
	Month           time.Time
	UsualMonths     int
	TotalCents      int64
	UsualTotalCents int64
	Items           []CategoryChange
}

// VersusUsual compares month's expenses by category with the average of up
// to twelve months before it, since the first expense. Categories match
// regardless of case; the biggest differences come first.
func VersusUsual(entries []cashflow.CashflowEntry, month time.Time) Comparison {
	month = cashflow.MonthStart(month)
	result := Comparison{Month: month, Items: []CategoryChange{}}

	var first time.Time
	for _, e := range entries {
		if _, ok := e.BaseCents(); ok && !e.IsIncome {
			if m := cashflow.MonthStart(e.EntryDate); first.IsZero() || m.Before(first) {
				first = m
			}
		}
	}

	if first.IsZero() {
		return result
	}

	from := month.AddDate(0, -usualMonths, 0)
	if first.After(from) {
		from = first
	}

	for m := from; m.Before(month); m = m.AddDate(0, 1, 0) {
		result.UsualMonths++
	}

	now := map[string]int64{}
	before := map[string]int64{}
	names := map[string]string{}
	for _, e := range entries {
		base, ok := e.BaseCents()
		if e.IsIncome || !ok {
			continue
		}

		m := cashflow.MonthStart(e.EntryDate)
		name := strings.TrimSpace(e.Category)
		key := strings.ToLower(name)
		switch {
		case m.Equal(month):
			now[key] += base
		case !m.Before(from) && m.Before(month):
			before[key] += base
		default:
			continue
		}

		if _, ok := names[key]; !ok {
			names[key] = name
		}
	}

	for key := range names {
		c := CategoryChange{Category: names[key], Cents: now[key], HasUsual: result.UsualMonths > 0}
		if c.HasUsual {
			c.UsualCents = before[key] / int64(result.UsualMonths)
		}

		c.DiffCents = c.Cents - c.UsualCents
		if c.UsualCents > 0 {
			c.DiffPercent = float64(c.DiffCents) / float64(c.UsualCents) * 100
		}

		result.TotalCents += c.Cents
		result.UsualTotalCents += c.UsualCents
		result.Items = append(result.Items, c)
	}

	floor := result.UsualTotalCents / 50
	for i := range result.Items {
		c := &result.Items[i]
		big := abs(c.DiffCents) >= floor && c.DiffCents != 0
		c.Unusual = c.HasUsual && big && (c.UsualCents == 0 || c.DiffPercent >= 30 || c.DiffPercent <= -30)
	}

	sort.SliceStable(result.Items, func(i, j int) bool {
		a, b := abs(result.Items[i].DiffCents), abs(result.Items[j].DiffCents)
		if a != b {
			return a > b
		}

		return result.Items[i].Category < result.Items[j].Category
	})

	return result
}

// SavingsRate is the part of income kept, in percent; false without income.
func SavingsRate(income, expense int64) (float64, bool) {
	if income <= 0 {
		return 0, false
	}

	return float64(income-expense) / float64(income) * 100, true
}
