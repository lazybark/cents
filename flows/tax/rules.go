package tax

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/money"
)

// ListMode picks which taxes a list shows, like the TUI's tax menu.
type ListMode string

const (
	ListUnpaid ListMode = "unpaid"
	ListPaid   ListMode = "paid"
)

const defaultPaymentNote = "manual tax paid adjustment"

// New validates a new tax of taxType and builds it. Amounts are in
// currency, which converts to the base currency at rate (1 for the base
// currency itself); dates are typed in format.
func New(taxType settings.SettingTaxType, currency string, rate float64, amountDue, amountPaid, period, dueDate, comment string, format dates.Format, now time.Time) (Tax, error) {
	period = strings.TrimSpace(period)
	if period == "" {
		return Tax{}, errors.New("period is required")
	}

	currency = strings.TrimSpace(currency)
	if currency == "" {
		return Tax{}, errors.New("currency is required")
	}

	if rate <= 0 {
		return Tax{}, errors.New("rate must be greater than zero")
	}

	t := Tax{
		TaxTypeID:   taxType.ID,
		TaxCountry:  strings.TrimSpace(taxType.Country),
		TaxTypeName: strings.TrimSpace(taxType.TaxTypeName),
		Currency:    currency,
		RateToBase:  rate,
	}

	return t.Edit(amountDue, amountPaid, period, dueDate, comment, format, now)
}

// WithRate replaces the recorded rate to the base currency, for when it was
// entered wrong, and recomputes the base amounts with it.
func (t Tax) WithRate(rate float64, now time.Time) (Tax, error) {
	if rate <= 0 {
		return Tax{}, errors.New("rate must be greater than zero")
	}

	t.RateToBase = rate
	t.LastUpdatedAt = now

	return t.withBaseAmounts(), nil
}

// withBaseAmounts recomputes the base amounts at the recorded rate.
func (t Tax) withBaseAmounts() Tax {
	t.AmountDueBaseCents = money.ToBaseCents(t.AmountDueCents, t.RateToBase)
	t.AmountPaidBaseCents = money.ToBaseCents(t.AmountPaidCents, t.RateToBase)

	return t
}

// Edit applies the fields that can change after creation; the tax type,
// currency and rate stay as they are.
func (t Tax) Edit(amountDue, amountPaid, period, dueDate, comment string, format dates.Format, now time.Time) (Tax, error) {
	dueCents, err := money.ParseAmountCents(amountDue)
	if err != nil {
		return Tax{}, fmt.Errorf("amount due error: %w", err)
	}

	if dueCents <= 0 {
		return Tax{}, errors.New("amount due must be greater than zero")
	}

	paidCents, err := money.ParseAmountCents(amountPaid)
	if err != nil {
		return Tax{}, fmt.Errorf("amount paid error: %w", err)
	}

	period = strings.TrimSpace(period)
	if period == "" {
		return Tax{}, errors.New("period is required")
	}

	due, err := format.Optional(dueDate, "due")
	if err != nil {
		return Tax{}, err
	}

	t.AmountDueCents = dueCents
	t.AmountPaidCents = paidCents
	t.Period = period
	t.DueDate = due
	t.Comment = strings.TrimSpace(comment)
	t.LastUpdatedAt = now

	return t.withBaseAmounts(), nil
}

// ApplyPayment adds delta (signed, like "-10") to the amount paid and
// returns the updated tax with the log entry to store alongside it. Paying
// more than is due is allowed; going below zero is not.
func (t Tax) ApplyPayment(delta, date, note string, format dates.Format, now time.Time) (Tax, TaxLog, error) {
	deltaCents, err := money.ParseSignedAmountCents(delta)
	if err != nil {
		return Tax{}, TaxLog{}, fmt.Errorf("log delta error: %w", err)
	}

	if deltaCents == 0 {
		return Tax{}, TaxLog{}, errors.New("delta cannot be zero")
	}

	nextPaid := t.AmountPaidCents + deltaCents
	if nextPaid < 0 {
		return Tax{}, TaxLog{}, errors.New("delta makes amount paid negative")
	}

	when, err := format.LogTime(date, now)
	if err != nil {
		return Tax{}, TaxLog{}, err
	}

	note = strings.TrimSpace(note)
	if note == "" {
		note = defaultPaymentNote
	}

	t.AmountPaidCents = nextPaid
	t.LastUpdatedAt = now

	return t.withBaseAmounts(), TaxLog{TaxID: t.ID, DeltaPaidCents: deltaCents, Note: note, CreatedAt: when}, nil
}

func (t Tax) IsPaid() bool {
	return t.AmountPaidCents >= t.AmountDueCents
}

// LeftCents is what is still to be paid, never below zero.
func (t Tax) LeftCents() int64 {
	return max(t.AmountDueCents-t.AmountPaidCents, 0)
}

// PaidPercent is how much of the amount due is paid, from 0 to 100.
func (t Tax) PaidPercent() float64 {
	if t.AmountDueCents <= 0 {
		return 0
	}

	return min(max(float64(t.AmountPaidCents)/float64(t.AmountDueCents)*100, 0), 100)
}

func Filter(items []Tax, mode ListMode) []Tax {
	filtered := make([]Tax, 0, len(items))
	for _, item := range items {
		if (mode == ListPaid) == item.IsPaid() {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

// LeftBaseCents is what is still to be paid in the base currency, at the
// recorded rate, never below zero.
func (t Tax) LeftBaseCents() int64 {
	return max(t.AmountDueBaseCents-t.AmountPaidBaseCents, 0)
}

// ProgressTotals sums paid and due amounts in the base currency, at each
// tax's recorded rate, capping each tax's paid amount at what is due.
func ProgressTotals(items []Tax) (paid int64, total int64) {
	for _, item := range items {
		itemTotal := max(item.AmountDueBaseCents, 0)
		total += itemTotal
		paid += min(max(item.AmountPaidBaseCents, 0), itemTotal)
	}

	return min(max(paid, 0), total), max(total, 0)
}
