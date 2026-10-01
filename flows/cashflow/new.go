package cashflow

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lazybark/cents/flows/settings"
)

// New validates and builds an income or expense entry. categories are the
// configured categories for that kind; the entry must use one of them.
// Amount and date are parsed by the caller, since each interface takes
// dates in its own format; rate is the currency's rate to the base currency
// to keep with it (see settings.EntryRate).
func New(isIncome bool, currency string, amountCents int64, rate float64, entryDate time.Time, category string, categories []string, accountName, comment string, now time.Time) (CashflowEntry, error) {
	if len(categories) == 0 {
		if isIncome {
			return CashflowEntry{}, errors.New("no income categories configured; add one in settings")
		}

		return CashflowEntry{}, errors.New("no expense categories configured; add one in settings")
	}

	category = strings.TrimSpace(category)
	if category == "" {
		return CashflowEntry{}, errors.New("category is required")
	}

	known := false
	for _, option := range categories {
		if option == category {
			known = true

			break
		}
	}

	if !known {
		return CashflowEntry{}, fmt.Errorf("unknown category %q", category)
	}

	currency = strings.TrimSpace(currency)
	if currency == "" {
		return CashflowEntry{}, errors.New("currency is required")
	}

	entry := CashflowEntry{
		IsIncome:      isIncome,
		Currency:      currency,
		AmountCents:   amountCents,
		EntryDate:     entryDate,
		Category:      category,
		AccountName:   strings.TrimSpace(accountName),
		Comment:       strings.TrimSpace(comment),
		LastUpdatedAt: now,
	}

	return entry.withRate(rate), nil
}

// WithRate replaces the entry's rate to the base currency, to fill in or
// correct the rate it was actually made at.
func (e CashflowEntry) WithRate(rate float64, now time.Time) (CashflowEntry, error) {
	if rate <= 0 {
		return CashflowEntry{}, errors.New("rate must be greater than zero")
	}

	e.LastUpdatedAt = now

	return e.withRate(rate), nil
}

func (e CashflowEntry) withRate(rate float64) CashflowEntry {
	e.RateToBase = max(rate, 0)
	e.AmountBaseCents, _ = settings.BaseCents(e.AmountCents, e.RateToBase)

	return e
}

// BaseCents is the amount in the base currency at the entry's rate,
// reporting false when it has none.
func (e CashflowEntry) BaseCents() (int64, bool) {
	if e.RateToBase <= 0 {
		return 0, false
	}

	return e.AmountBaseCents, true
}
