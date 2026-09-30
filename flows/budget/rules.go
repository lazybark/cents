package budget

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/money"
)

// States of a budget in a month.
const (
	StateOK    = "ok"
	StateClose = "close"
	StateOver  = "over"
)

// closeAt is the part of the limit, in percent, from which a budget is
// close to it.
const closeAt = 80

// Fields is a budget as typed: the expense category ("" for all spending)
// and the monthly limit.
type Fields struct {
	Category string
	Limit    string
}

func key(category string) string {
	return strings.ToLower(strings.TrimSpace(category))
}

// New makes a budget from f. The category must be an expense category in
// settings, without a budget yet among existing (all spending counts as
// one too).
func New(f Fields, stts settings.AppSettings, existing []Budget, now time.Time) (Budget, error) {
	b := Budget{CreatedAt: now}
	return apply(b, f, stts, existing, now)
}

// Edit changes b to f; the same checks as New apply, b itself aside.
func Edit(b Budget, f Fields, stts settings.AppSettings, existing []Budget, now time.Time) (Budget, error) {
	return apply(b, f, stts, existing, now)
}

func apply(b Budget, f Fields, stts settings.AppSettings, existing []Budget, now time.Time) (Budget, error) {
	category := strings.TrimSpace(f.Category)
	if category != "" {
		found := false
		for _, c := range stts.ExpenseCategories {
			if key(c.CategoryName) == key(category) {
				category, found = c.CategoryName, true
				break
			}
		}

		// A budget kept on a category that left settings can still be edited.
		if !found && key(b.Category) != key(category) {
			return Budget{}, fmt.Errorf("unknown expense category %q", category)
		}
	}

	for _, other := range existing {
		if other.ID != b.ID && key(other.Category) == key(category) {
			if category == "" {
				return Budget{}, errors.New("there's a budget for all spending already")
			}

			return Budget{}, fmt.Errorf("%s has a budget already", category)
		}
	}

	if strings.TrimSpace(f.Limit) == "" {
		return Budget{}, errors.New("limit is required")
	}

	limit, err := money.ParseAmountCents(f.Limit)
	if err != nil || limit <= 0 {
		return Budget{}, errors.New("limit must be a number above 0, like 300 or 249.90")
	}

	b.Category = category
	b.LimitCents = limit
	b.LastUpdatedAt = now

	return b, nil
}

// Progress is a budget against what was spent in a month, in the base
// currency.
type Progress struct {
	Budget     Budget
	SpentCents int64
	// LeftCents is what's left of the limit, negative when over.
	LeftCents int64
	Percent   float64
	State     string
}

// Month is each budget's progress in month: what its category (all
// spending, for the total) took in expenses then. Expenses without a rate
// to the base currency can't be counted; Missing says how many there were.
// The total comes first, then categories by name.
func Month(budgets []Budget, entries []cashflow.CashflowEntry, month time.Time) (result []Progress, missing int) {
	month = cashflow.MonthStart(month)
	spent := map[string]int64{}
	var total int64
	for _, e := range entries {
		if e.IsIncome || !cashflow.MonthStart(e.EntryDate).Equal(month) {
			continue
		}

		base, ok := e.BaseCents()
		if !ok {
			missing++
			continue
		}

		spent[key(e.Category)] += base
		total += base
	}

	result = make([]Progress, 0, len(budgets))
	for _, b := range budgets {
		p := Progress{Budget: b, SpentCents: spent[key(b.Category)]}
		if b.IsTotal() {
			p.SpentCents = total
		}

		p.LeftCents = b.LimitCents - p.SpentCents
		if b.LimitCents > 0 {
			p.Percent = float64(p.SpentCents) / float64(b.LimitCents) * 100
		}

		switch {
		case p.SpentCents > b.LimitCents:
			p.State = StateOver
		case p.Percent >= closeAt:
			p.State = StateClose
		default:
			p.State = StateOK
		}

		result = append(result, p)
	}

	sort.SliceStable(result, func(i, j int) bool {
		a, b := result[i].Budget, result[j].Budget
		if a.IsTotal() != b.IsTotal() {
			return a.IsTotal()
		}

		return key(a.Category) < key(b.Category)
	})

	return result, missing
}
