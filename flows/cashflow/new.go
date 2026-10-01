package cashflow

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// New validates and builds an income or expense entry. categories are the
// configured categories for that kind; the entry must use one of them.
// Amount and date are parsed by the caller, since each interface takes
// dates in its own format.
func New(isIncome bool, currency string, amountCents int64, entryDate time.Time, category string, categories []string, accountName, comment string, now time.Time) (CashflowEntry, error) {
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

	return CashflowEntry{
		IsIncome:      isIncome,
		Currency:      currency,
		AmountCents:   amountCents,
		EntryDate:     entryDate,
		Category:      category,
		AccountName:   strings.TrimSpace(accountName),
		Comment:       strings.TrimSpace(comment),
		LastUpdatedAt: now,
	}, nil
}
