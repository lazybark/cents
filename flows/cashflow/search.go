package cashflow

import (
	"sort"
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/money"
)

// Query picks entries across every month. Empty fields don't limit.
type Query struct {
	// Text is looked for in the comment, category, account and currency,
	// ignoring case; a number also finds entries of that amount.
	Text string
	// Kind is "income", "expense" or "" for both.
	Kind     string
	Category string
	Account  string
	Currency string
	// Tag is a tag the entries must have.
	Tag string
	// From and To are days, both included.
	From, To time.Time
	// MinBase and MaxBase bound the amount in the base currency; entries
	// without a rate don't match a bound.
	MinBase, MaxBase *int64
}

func hasTag(tags []string, name string) bool {
	for _, t := range tags {
		if strings.EqualFold(strings.TrimSpace(t), strings.TrimSpace(name)) {
			return true
		}
	}

	return false
}

// Search returns the entries matching q, newest first.
func Search(entries []CashflowEntry, q Query) []CashflowEntry {
	text := strings.ToLower(strings.TrimSpace(q.Text))
	amount, isAmount := int64(0), false
	if text != "" {
		if cents, err := money.ParseAmountCents(strings.TrimPrefix(text, "-")); err == nil {
			amount, isAmount = cents, true
		}
	}

	same := func(a, b string) bool { return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b)) }
	result := make([]CashflowEntry, 0)
	for _, e := range entries {
		switch {
		case q.Kind == "income" && !e.IsIncome, q.Kind == "expense" && e.IsIncome:
			continue
		case q.Category != "" && !same(e.Category, q.Category):
			continue
		case q.Account != "" && !same(e.AccountName, q.Account):
			continue
		case q.Currency != "" && !same(e.Currency, q.Currency):
			continue
		case q.Tag != "" && !hasTag(e.Tags, q.Tag):
			continue
		case !q.From.IsZero() && dates.Day(e.EntryDate).Before(dates.Day(q.From)):
			continue
		case !q.To.IsZero() && dates.Day(e.EntryDate).After(dates.Day(q.To)):
			continue
		}

		if q.MinBase != nil || q.MaxBase != nil {
			base, ok := e.BaseCents()
			if !ok || q.MinBase != nil && base < *q.MinBase || q.MaxBase != nil && base > *q.MaxBase {
				continue
			}
		}

		if text != "" {
			found := isAmount && (e.AmountCents == amount || e.AmountBaseCents == amount)
			for _, field := range append([]string{e.Comment, e.Category, e.AccountName, e.Currency}, e.Tags...) {
				found = found || strings.Contains(strings.ToLower(field), text)
			}

			if !found {
				continue
			}
		}

		result = append(result, e)
	}

	sort.SliceStable(result, func(i, j int) bool {
		if !result[i].EntryDate.Equal(result[j].EntryDate) {
			return result[i].EntryDate.After(result[j].EntryDate)
		}

		return result[i].ID > result[j].ID
	})

	return result
}
