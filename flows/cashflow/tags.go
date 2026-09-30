package cashflow

import (
	"sort"
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
)

// TagTotals adds up the entries with a tag, in the base currency: how many,
// income and expenses, from when to when, and by category.
type TagTotals struct {
	Tag          string
	Count        int
	IncomeCents  int64
	ExpenseCents int64
	// MissingRates counts entries without a rate, left out of the sums.
	MissingRates int
	First, Last  time.Time
	Categories   []CategoryTotal
}

// CategoryTotal is what a category adds up to under a tag.
type CategoryTotal struct {
	Category string
	IsIncome bool
	Cents    int64
}

// ByTag adds up entries for each tag they have, most recently used first.
func ByTag(entries []CashflowEntry) []TagTotals {
	byKey := map[string]*TagTotals{}
	cats := map[string]map[string]*CategoryTotal{}
	for _, e := range entries {
		for _, name := range e.Tags {
			key := strings.ToLower(strings.TrimSpace(name))
			t, ok := byKey[key]
			if !ok {
				t = &TagTotals{Tag: name}
				byKey[key] = t
				cats[key] = map[string]*CategoryTotal{}
			}

			day := dates.Day(e.EntryDate)
			t.Count++
			if t.First.IsZero() || day.Before(t.First) {
				t.First = day
			}
			if day.After(t.Last) {
				t.Last = day
			}

			base, ok := e.BaseCents()
			if !ok {
				t.MissingRates++
				continue
			}

			if e.IsIncome {
				t.IncomeCents += base
			} else {
				t.ExpenseCents += base
			}

			ckey := strings.ToLower(strings.TrimSpace(e.Category))
			if e.IsIncome {
				ckey = "+" + ckey
			}

			c, ok := cats[key][ckey]
			if !ok {
				c = &CategoryTotal{Category: e.Category, IsIncome: e.IsIncome}
				cats[key][ckey] = c
			}

			c.Cents += base
		}
	}

	result := make([]TagTotals, 0, len(byKey))
	for key, t := range byKey {
		for _, c := range cats[key] {
			t.Categories = append(t.Categories, *c)
		}

		sort.Slice(t.Categories, func(i, j int) bool {
			if t.Categories[i].Cents != t.Categories[j].Cents {
				return t.Categories[i].Cents > t.Categories[j].Cents
			}

			return t.Categories[i].Category < t.Categories[j].Category
		})

		result = append(result, *t)
	}

	sort.Slice(result, func(i, j int) bool {
		if !result[i].Last.Equal(result[j].Last) {
			return result[i].Last.After(result[j].Last)
		}

		return strings.ToLower(result[i].Tag) < strings.ToLower(result[j].Tag)
	})

	return result
}
