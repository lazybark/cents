package debt

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/money"
)

// ListMode picks which debts a list shows, like the TUI's debt menu.
type ListMode string

const (
	ListOutgoing ListMode = "outgoing"
	ListIncoming ListMode = "incoming"
	ListPaid     ListMode = "paid"
)

const defaultPaymentNote = "manual paid adjustment"

// New validates a new debt and builds it. Every interface creates debts
// through here, so the rules and messages match; dates are typed in format
// and rate is the currency's rate to keep with it (see settings.EntryRate).
func New(isOwedToUser bool, peer, currency string, rate float64, amount, amountPaid, createdAt, dueDate, comment string, format dates.Format, now time.Time) (Debt, error) {
	peer = strings.TrimSpace(peer)
	if peer == "" {
		return Debt{}, errors.New("peer is required")
	}

	d := Debt{Peer: peer, Currency: strings.TrimSpace(currency), IsOwedToUser: isOwedToUser, RateToBase: max(rate, 0)}

	return d.Edit(amount, amountPaid, createdAt, dueDate, comment, format, now)
}

// Edit applies the fields that can change after creation. Peer, currency,
// direction and rate stay as they are.
func (d Debt) Edit(amount, amountPaid, createdAt, dueDate, comment string, format dates.Format, now time.Time) (Debt, error) {
	amountCents, err := money.ParseAmountCents(amount)
	if err != nil {
		return Debt{}, fmt.Errorf("amount error: %w", err)
	}

	paidCents, err := money.ParseAmountCents(amountPaid)
	if err != nil {
		return Debt{}, fmt.Errorf("amount paid error: %w", err)
	}

	if paidCents > amountCents {
		return Debt{}, errors.New("amount paid cannot be more than amount")
	}

	created, err := format.Required(createdAt, "created")
	if err != nil {
		return Debt{}, err
	}

	due, err := format.Optional(dueDate, "due")
	if err != nil {
		return Debt{}, err
	}

	d.AmountCents = amountCents
	d.AmountPaidCents = paidCents
	d.DebtCreatedAt = created
	d.DueDate = due
	d.Comment = strings.TrimSpace(comment)
	d.LastUpdatedAt = now

	return d.withBaseAmounts(), nil
}

// WithRate replaces the recorded rate to the base currency, to fill in or
// correct it, and recomputes the base amounts with it.
func (d Debt) WithRate(rate float64, now time.Time) (Debt, error) {
	if rate <= 0 {
		return Debt{}, errors.New("rate must be greater than zero")
	}

	d.RateToBase = rate
	d.LastUpdatedAt = now

	return d.withBaseAmounts(), nil
}

func (d Debt) withBaseAmounts() Debt {
	d.AmountBaseCents, _ = settings.BaseCents(d.AmountCents, d.RateToBase)
	d.AmountPaidBaseCents, _ = settings.BaseCents(d.AmountPaidCents, d.RateToBase)

	return d
}

// LeftBaseCents is what is still to be paid in the base currency at the
// recorded rate, reporting false when the debt has no rate.
func (d Debt) LeftBaseCents() (int64, bool) {
	if d.RateToBase <= 0 {
		return 0, false
	}

	return max(d.AmountBaseCents-d.AmountPaidBaseCents, 0), true
}

// ApplyPayment adds delta (signed, like "-10") to the amount paid and
// returns the updated debt with the log entry to store alongside it. The
// amount paid has to stay between zero and the amount.
func (d Debt) ApplyPayment(delta, date, note string, format dates.Format, now time.Time) (Debt, DebtLog, error) {
	deltaCents, err := money.ParseSignedAmountCents(delta)
	if err != nil {
		return Debt{}, DebtLog{}, fmt.Errorf("log delta error: %w", err)
	}

	if deltaCents == 0 {
		return Debt{}, DebtLog{}, errors.New("delta cannot be zero")
	}

	nextPaid := d.AmountPaidCents + deltaCents
	if nextPaid < 0 || nextPaid > d.AmountCents {
		return Debt{}, DebtLog{}, errors.New("delta makes amount paid out of range")
	}

	when, err := format.LogTime(date, now)
	if err != nil {
		return Debt{}, DebtLog{}, err
	}

	note = strings.TrimSpace(note)
	if note == "" {
		note = defaultPaymentNote
	}

	d.AmountPaidCents = nextPaid
	d.LastUpdatedAt = now

	return d.withBaseAmounts(), DebtLog{DebtID: d.ID, DeltaPaidCents: deltaCents, Note: note, CreatedAt: when}, nil
}

func (d Debt) IsPaid() bool {
	return d.AmountPaidCents >= d.AmountCents
}

// LeftCents is what is still to be paid, never below zero.
func (d Debt) LeftCents() int64 {
	return max(d.AmountCents-d.AmountPaidCents, 0)
}

// Filter keeps the debts a list mode shows: unpaid debts by direction, or
// every paid debt.
func Filter(items []Debt, mode ListMode) []Debt {
	filtered := make([]Debt, 0, len(items))
	for _, item := range items {
		switch mode {
		case ListOutgoing:
			if !item.IsOwedToUser && !item.IsPaid() {
				filtered = append(filtered, item)
			}
		case ListIncoming:
			if item.IsOwedToUser && !item.IsPaid() {
				filtered = append(filtered, item)
			}
		case ListPaid:
			if item.IsPaid() {
				filtered = append(filtered, item)
			}
		}
	}

	return filtered
}

// ProgressInBaseCents totals paid and total amounts in the base currency at
// each debt's recorded rate, capping each debt's paid amount at its total.
// Debts without a rate count with their raw amounts.
func ProgressInBaseCents(items []Debt) (paid int64, total int64) {
	for _, item := range items {
		if item.RateToBase > 0 {
			itemTotal := max(item.AmountBaseCents, 0)
			total += itemTotal
			paid += min(max(item.AmountPaidBaseCents, 0), itemTotal)

			continue
		}

		itemTotal := max(item.AmountCents, 0)
		total += itemTotal
		paid += min(max(item.AmountPaidCents, 0), itemTotal)
	}

	total = max(total, 0)
	paid = min(max(paid, 0), total)

	return paid, total
}

// RemovePayment undoes a logged payment: the amount paid moves back by its
// delta. The result has to stay between zero and the amount, which it may
// not if the amount paid was edited by hand since.
func (d Debt) RemovePayment(entry DebtLog, now time.Time) (Debt, error) {
	if entry.DebtID != d.ID {
		return Debt{}, errors.New("payment belongs to another debt")
	}

	nextPaid := d.AmountPaidCents - entry.DeltaPaidCents
	if nextPaid < 0 || nextPaid > d.AmountCents {
		return Debt{}, errors.New("removing this payment makes amount paid out of range; edit amount paid instead")
	}

	d.AmountPaidCents = nextPaid
	d.LastUpdatedAt = now

	return d.withBaseAmounts(), nil
}
