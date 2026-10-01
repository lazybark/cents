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

// New validates a new tax of taxType and builds it. Amounts are in the base
// currency; dates are typed in format.
func New(taxType settings.SettingTaxType, amountDue, amountPaid, period, dueDate, comment string, format dates.Format, now time.Time) (Tax, error) {
	period = strings.TrimSpace(period)
	if period == "" {
		return Tax{}, errors.New("period is required")
	}

	t := Tax{
		TaxTypeID:   taxType.ID,
		TaxCountry:  strings.TrimSpace(taxType.Country),
		TaxTypeName: strings.TrimSpace(taxType.TaxTypeName),
	}

	return t.Edit(amountDue, amountPaid, period, dueDate, comment, format, now)
}

// Edit applies the fields that can change after creation; the tax type
// stays as it is.
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

	return t, nil
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

	return t, TaxLog{TaxID: t.ID, DeltaPaidCents: deltaCents, Note: note, CreatedAt: when}, nil
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

// ProgressTotals sums paid and due amounts, capping each tax's paid amount
// at what is due. Tax amounts are always in the base currency.
func ProgressTotals(items []Tax) (paid int64, total int64) {
	for _, item := range items {
		itemTotal := max(item.AmountDueCents, 0)
		total += itemTotal
		paid += min(max(item.AmountPaidCents, 0), itemTotal)
	}

	return min(max(paid, 0), total), max(total, 0)
}
