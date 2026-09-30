package credit

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/money"
)

// ListMode picks which credits a list shows.
type ListMode string

const (
	ListActive ListMode = "active"
	ListPaid   ListMode = "paid"
)

// LogKind is what a log entry does to a credit.
type LogKind string

const (
	LogPayment  LogKind = "payment"
	LogAddition LogKind = "addition"
)

const (
	defaultPaymentNote  = "payment"
	defaultAdditionNote = "added to credit"
)

// PurposeOptions are common purposes, offered as suggestions; any other can
// be typed in.
func PurposeOptions() []string {
	return []string{"Mortgage", "Car loan", "Consumer loan", "Personal loan", "Student loan", "Credit card", "Business loan", "Refinancing", "Other"}
}

// Fields are a credit's values as typed into a form; dates in format.
type Fields struct {
	Name            string
	Purpose         string
	Issuer          string
	Total           string
	Paid            string
	InterestPercent string
	StartDate       string
	DueDate         string
	Comment         string
}

// New validates a new credit in currency and builds it; rate is the
// currency's rate to keep with it (see settings.EntryRate).
func New(currency string, rate float64, f Fields, format dates.Format, now time.Time) (Credit, error) {
	currency = strings.TrimSpace(currency)
	if currency == "" {
		return Credit{}, errors.New("currency is required")
	}

	return Credit{Currency: currency, RateToBase: max(rate, 0)}.Edit(f, format, now)
}

// Edit applies f; currency and rate stay as they are.
func (c Credit) Edit(f Fields, format dates.Format, now time.Time) (Credit, error) {
	name := strings.TrimSpace(f.Name)
	if name == "" {
		return Credit{}, errors.New("name is required")
	}

	total, err := money.ParseAmountCents(f.Total)
	if err != nil {
		return Credit{}, fmt.Errorf("amount error: %w", err)
	}

	if total <= 0 {
		return Credit{}, errors.New("amount must be greater than zero")
	}

	var paid int64
	if raw := strings.TrimSpace(f.Paid); raw != "" {
		if paid, err = money.ParseAmountCents(raw); err != nil {
			return Credit{}, fmt.Errorf("amount paid error: %w", err)
		}
	}

	if paid > total {
		return Credit{}, errors.New("amount paid cannot be more than amount")
	}

	interest, err := parsePercent(f.InterestPercent)
	if err != nil {
		return Credit{}, err
	}

	start, err := format.Required(f.StartDate, "start")
	if err != nil {
		return Credit{}, err
	}

	due, err := format.Optional(f.DueDate, "due")
	if err != nil {
		return Credit{}, err
	}

	if due != nil && due.Before(start) {
		return Credit{}, errors.New("due date cannot be before the start date")
	}

	c.Name = name
	c.Purpose = strings.TrimSpace(f.Purpose)
	c.Issuer = strings.TrimSpace(f.Issuer)
	c.TotalCents = total
	c.PaidCents = paid
	c.InterestPercent = interest
	c.StartDate = start
	c.DueDate = due
	c.Comment = strings.TrimSpace(f.Comment)
	c.LastUpdatedAt = now

	return c.withBaseAmounts(), nil
}

// parsePercent parses a yearly interest rate like "5.5" (or "5,5%"); empty
// means none.
func parsePercent(raw string) (float64, error) {
	trimmed := strings.TrimSuffix(strings.TrimSpace(raw), "%")
	trimmed = strings.ReplaceAll(strings.TrimSpace(trimmed), ",", ".")
	if trimmed == "" {
		return 0, nil
	}

	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, errors.New("interest must be a number")
	}

	if value < 0 {
		return 0, errors.New("interest cannot be negative")
	}

	return value, nil
}

// WithRate replaces the recorded rate to the base currency, to fill in or
// correct it, and recomputes the base amounts with it.
func (c Credit) WithRate(rate float64, now time.Time) (Credit, error) {
	if rate <= 0 {
		return Credit{}, errors.New("rate must be greater than zero")
	}

	c.RateToBase = rate
	c.LastUpdatedAt = now

	return c.withBaseAmounts(), nil
}

func (c Credit) withBaseAmounts() Credit {
	c.TotalBaseCents, _ = settings.BaseCents(c.TotalCents, c.RateToBase)
	c.PaidBaseCents, _ = settings.BaseCents(c.PaidCents, c.RateToBase)

	return c
}

// ApplyLog records a payment or an addition of amount (positive, like
// "250") on date, returning the updated credit with the log entry to store
// alongside it. A payment can't take the amount paid above the total.
func (c Credit) ApplyLog(kind LogKind, amount, date, note string, format dates.Format, now time.Time) (Credit, CreditLog, error) {
	cents, err := money.ParseAmountCents(amount)
	if err != nil {
		return Credit{}, CreditLog{}, fmt.Errorf("amount error: %w", err)
	}

	if cents == 0 {
		return Credit{}, CreditLog{}, errors.New("amount must be greater than zero")
	}

	entry := CreditLog{CreditID: c.ID, Note: strings.TrimSpace(note)}

	switch kind {
	case LogPayment:
		if c.PaidCents+cents > c.TotalCents {
			return Credit{}, CreditLog{}, errors.New("payment is more than what is left; add to the credit first")
		}

		c.PaidCents += cents
		entry.DeltaPaidCents = cents
		if entry.Note == "" {
			entry.Note = defaultPaymentNote
		}
	case LogAddition:
		c.TotalCents += cents
		entry.DeltaTotalCents = cents
		if entry.Note == "" {
			entry.Note = defaultAdditionNote
		}
	default:
		return Credit{}, CreditLog{}, fmt.Errorf("unknown kind of entry %q", kind)
	}

	when, err := format.LogTime(date, now)
	if err != nil {
		return Credit{}, CreditLog{}, err
	}

	entry.CreatedAt = when
	c.LastUpdatedAt = now

	return c.withBaseAmounts(), entry, nil
}

// RemoveLog undoes a log entry: a payment comes off the amount paid, an
// addition off the total. The amount paid has to stay between zero and the
// total, which it may not if the amounts were edited by hand since.
func (c Credit) RemoveLog(entry CreditLog, now time.Time) (Credit, error) {
	if entry.CreditID != c.ID {
		return Credit{}, errors.New("log entry belongs to another credit")
	}

	paid := c.PaidCents - entry.DeltaPaidCents
	total := c.TotalCents - entry.DeltaTotalCents
	if paid < 0 || total <= 0 || paid > total {
		return Credit{}, errors.New("removing this entry makes the amounts out of range; edit them instead")
	}

	c.PaidCents = paid
	c.TotalCents = total
	c.LastUpdatedAt = now

	return c.withBaseAmounts(), nil
}

// Kind is what the entry did to its credit.
func (l CreditLog) Kind() LogKind {
	if l.DeltaTotalCents != 0 {
		return LogAddition
	}

	return LogPayment
}

func (c Credit) IsPaid() bool {
	return c.PaidCents >= c.TotalCents
}

// LeftCents is what is still to be paid, never below zero.
func (c Credit) LeftCents() int64 {
	return max(c.TotalCents-c.PaidCents, 0)
}

// LeftBaseCents is what is still to be paid in the base currency at the
// recorded rate, reporting false when the credit has no rate.
func (c Credit) LeftBaseCents() (int64, bool) {
	if c.RateToBase <= 0 {
		return 0, false
	}

	return max(c.TotalBaseCents-c.PaidBaseCents, 0), true
}

// Filter keeps the credits a list shows: ones still being paid, or paid off.
func Filter(items []Credit, mode ListMode) []Credit {
	filtered := make([]Credit, 0, len(items))
	for _, item := range items {
		if item.IsPaid() == (mode == ListPaid) {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

// ProgressInBaseCents totals paid and total amounts in the base currency at
// each credit's recorded rate. Credits without a rate count with their raw
// amounts.
func ProgressInBaseCents(items []Credit) (paid int64, total int64) {
	for _, item := range items {
		if item.RateToBase > 0 {
			total += item.TotalBaseCents
			paid += min(item.PaidBaseCents, item.TotalBaseCents)

			continue
		}

		total += item.TotalCents
		paid += min(item.PaidCents, item.TotalCents)
	}

	return paid, total
}

// UnpaidInBaseCents is what is left to pay on every credit, in the base
// currency at recorded rates; credits without a rate are left out.
func UnpaidInBaseCents(items []Credit) int64 {
	var left int64
	for _, item := range items {
		if cents, ok := item.LeftBaseCents(); ok {
			left += cents
		}
	}

	return left
}
