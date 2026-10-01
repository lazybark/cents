package invoice

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/money"
)

// ListMode picks which invoices a list shows, like the TUI's invoice menu.
// Incoming invoices are the ones I have to pay; outgoing ones are paid to me.
type ListMode string

const (
	ListOutgoing ListMode = "outgoing"
	ListIncoming ListMode = "incoming"
	ListPaid     ListMode = "paid"
)

// Fields are an invoice's values as typed into a form. Every field can be
// changed after creation. Rate is the rate to the base currency typed, if
// any; see settings.EditRate for what an empty one means.
type Fields struct {
	Title         string
	IsIncoming    bool
	Currency      string
	Rate          string
	Amount        string
	Paid          bool
	Peer          string
	InvoiceDate   string
	DueDate       string
	TargetAccount string
	URL           string
	Description   string
}

// New validates a new invoice and builds it. Every interface creates
// invoices through here, so the rules and messages match.
func New(f Fields, stts settings.AppSettings, format dates.Format, now time.Time) (Invoice, error) {
	return Invoice{}.Edit(f, stts, format, now)
}

// Edit replaces the invoice's values with f. Only the title is required;
// an empty amount means zero. An invoice without a currency has no rate.
func (i Invoice) Edit(f Fields, stts settings.AppSettings, format dates.Format, now time.Time) (Invoice, error) {
	title := strings.TrimSpace(f.Title)
	if title == "" {
		return Invoice{}, errors.New("invoice title is required")
	}

	var amount int64
	if raw := strings.TrimSpace(f.Amount); raw != "" {
		cents, err := money.ParseAmountCents(raw)
		if err != nil {
			return Invoice{}, fmt.Errorf("amount error: %w", err)
		}

		amount = cents
	}

	invoiceDate, err := format.Optional(f.InvoiceDate, "invoice")
	if err != nil {
		return Invoice{}, err
	}

	dueDate, err := format.Optional(f.DueDate, "due")
	if err != nil {
		return Invoice{}, err
	}

	currency := strings.TrimSpace(f.Currency)
	rate := 0.0
	if currency != "" {
		rate, err = stts.EditRate(i.Currency, i.RateToBase, currency, f.Rate)
		if err != nil {
			return Invoice{}, err
		}
	}

	i.Title = title
	i.IsIncoming = f.IsIncoming
	i.Currency = currency
	i.AmountCents = amount
	i.RateToBase = rate
	i.AmountBaseCents, _ = settings.BaseCents(amount, rate)
	i.Paid = f.Paid
	i.Peer = strings.TrimSpace(f.Peer)
	i.InvoiceDate = invoiceDate
	i.DueDate = dueDate
	i.TargetAccount = strings.TrimSpace(f.TargetAccount)
	i.URL = strings.TrimSpace(f.URL)
	i.Description = strings.TrimSpace(f.Description)
	i.LastUpdatedAt = now

	return i, nil
}

// Mode is the list the invoice shows up in.
func (i Invoice) Mode() ListMode {
	switch {
	case i.Paid:
		return ListPaid
	case i.IsIncoming:
		return ListIncoming
	default:
		return ListOutgoing
	}
}

// Filter keeps the invoices a list mode shows, latest due date first;
// invoices without one come last, newest first.
func Filter(items []Invoice, mode ListMode) []Invoice {
	filtered := make([]Invoice, 0, len(items))
	for _, item := range items {
		if item.Mode() == mode {
			filtered = append(filtered, item)
		}
	}

	sort.SliceStable(filtered, func(a int, b int) bool {
		left, right := filtered[a], filtered[b]

		if left.DueDate != nil && right.DueDate != nil && !left.DueDate.Equal(*right.DueDate) {
			return left.DueDate.After(*right.DueDate)
		}

		if (left.DueDate == nil) != (right.DueDate == nil) {
			return left.DueDate != nil
		}

		if left.CreatedAt.Equal(right.CreatedAt) {
			return left.ID > right.ID
		}

		return left.CreatedAt.After(right.CreatedAt)
	})

	return filtered
}

// BaseCents is the amount in the base currency at the invoice's rate,
// reporting false when it has none.
func (i Invoice) BaseCents() (int64, bool) {
	if i.RateToBase <= 0 {
		return 0, false
	}

	return i.AmountBaseCents, true
}

// UnpaidInBaseCents totals unpaid invoices in the base currency at their
// recorded rates: what is owed to me (outgoing) and what I owe (incoming).
// Invoices without a rate are left out.
func UnpaidInBaseCents(items []Invoice) (toMe int64, byMe int64) {
	for _, item := range items {
		if item.Paid {
			continue
		}

		converted, ok := item.BaseCents()
		if !ok {
			continue
		}

		if item.IsIncoming {
			byMe += converted
		} else {
			toMe += converted
		}
	}

	return toMe, byMe
}
