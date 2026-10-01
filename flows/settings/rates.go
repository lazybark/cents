package settings

import (
	"strings"

	"github.com/lazybark/cents/money"
)

// Dated records (incomes and expenses, debts, invoices, taxes) keep the rate
// to the base currency they were entered with, so their base amounts don't
// move when a rate in settings changes later. A rate of 0 means none is
// known; such records are left out of base totals until one is set.

// IsBase reports whether currency is the base currency.
func (s AppSettings) IsBase(currency string) bool {
	return strings.EqualFold(strings.TrimSpace(currency), s.BaseCurrencyLabel())
}

// EntryRate is the rate to save with a new record in currency: 1 for the
// base currency, the typed rate if there is one, otherwise the currency's
// rate in settings now (0 when it has none).
func (s AppSettings) EntryRate(currency, typed string) (float64, error) {
	if s.IsBase(currency) {
		return 1, nil
	}

	if strings.TrimSpace(typed) == "" {
		rate, _ := s.RateToBase(currency)

		return rate, nil
	}

	return money.ParseRate(typed)
}

// EditRate is the rate to save when a record changes from oldCurrency at
// oldRate to currency: the typed rate if there is one, otherwise the old
// rate while the currency stays the same, or the rate a new record would
// get in the new currency.
func (s AppSettings) EditRate(oldCurrency string, oldRate float64, currency, typed string) (float64, error) {
	if s.IsBase(currency) || strings.TrimSpace(typed) != "" || !strings.EqualFold(strings.TrimSpace(oldCurrency), strings.TrimSpace(currency)) {
		return s.EntryRate(currency, typed)
	}

	return oldRate, nil
}

// BaseCents converts cents at a recorded rate, reporting false when the
// record has no rate.
func BaseCents(cents int64, rate float64) (int64, bool) {
	if rate <= 0 {
		return 0, false
	}

	return money.ToBaseCents(cents, rate), true
}
