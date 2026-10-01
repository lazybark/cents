package subscription

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lazybark/cents/money"
)

// PaymentDateLayout is how PaymentDateYearly is stored.
const PaymentDateLayout = "02.01.2006"

func PeriodOptions() []string {
	return []string{"month", "year"}
}

func TypeOptions() []string {
	return []string{"Software", "Domain", "Service", "Multimedia", "Other"}
}

// New validates the fields of a new subscription and builds it. Every
// interface creates subscriptions through here so they enforce the same
// rules. paymentDateYearly uses PaymentDateLayout; both day fields are
// optional.
func New(name, currency, amount, period, paymentMethod, subType string, isActive bool, paymentDateYearly, paymentDayMonthly string, now time.Time) (Subscription, error) {
	name = strings.TrimSpace(name)
	currency = strings.TrimSpace(currency)
	paymentMethod = strings.TrimSpace(paymentMethod)

	if name == "" {
		return Subscription{}, errors.New("subscription name is required")
	}

	if currency == "" {
		return Subscription{}, errors.New("currency is required")
	}

	if paymentMethod == "" {
		return Subscription{}, errors.New("payment method is required")
	}

	amountCents, err := money.ParseAmountCents(amount)
	if err != nil {
		return Subscription{}, fmt.Errorf("amount error: %w", err)
	}

	dateYearly, err := parseOptionalDate(paymentDateYearly)
	if err != nil {
		return Subscription{}, err
	}

	dayMonthly, err := parseOptionalDay(paymentDayMonthly, 1, 31, "monthly")
	if err != nil {
		return Subscription{}, err
	}

	if !contains(PeriodOptions(), period) {
		return Subscription{}, fmt.Errorf("period must be one of %s", strings.Join(PeriodOptions(), ", "))
	}

	if !contains(TypeOptions(), subType) {
		return Subscription{}, fmt.Errorf("type must be one of %s", strings.Join(TypeOptions(), ", "))
	}

	return Subscription{
		Name:              name,
		Currency:          currency,
		AmountCents:       amountCents,
		Period:            period,
		PaymentMethod:     paymentMethod,
		Type:              subType,
		IsActive:          isActive,
		PaymentDateYearly: dateYearly,
		PaymentDayMonthly: dayMonthly,
		LastUpdatedAt:     now,
	}, nil
}

// Edit applies the fields that can change after creation: amount, payment
// method and whether the subscription is active.
func (s Subscription) Edit(amount, paymentMethod string, isActive bool, now time.Time) (Subscription, error) {
	paymentMethod = strings.TrimSpace(paymentMethod)
	if paymentMethod == "" {
		return Subscription{}, errors.New("payment method is required")
	}

	amountCents, err := money.ParseAmountCents(amount)
	if err != nil {
		return Subscription{}, fmt.Errorf("amount error: %w", err)
	}

	s.AmountCents = amountCents
	s.PaymentMethod = paymentMethod
	s.IsActive = isActive
	s.LastUpdatedAt = now

	return s, nil
}

// SortByAmount sorts by raw amount, largest first, then newest first.
func SortByAmount(values []Subscription) []Subscription {
	if len(values) < 2 {
		return values
	}

	cloned := make([]Subscription, len(values))
	copy(cloned, values)
	sort.SliceStable(cloned, func(i, j int) bool {
		if cloned[i].AmountCents == cloned[j].AmountCents {
			return cloned[i].CreatedAt.After(cloned[j].CreatedAt)
		}

		return cloned[i].AmountCents > cloned[j].AmountCents
	})

	return cloned
}

func SplitByActivity(subs []Subscription) (active []Subscription, inactive []Subscription) {
	active = make([]Subscription, 0, len(subs))
	inactive = make([]Subscription, 0, len(subs))

	for _, sub := range subs {
		if sub.IsActive {
			active = append(active, sub)

			continue
		}

		inactive = append(inactive, sub)
	}

	return active, inactive
}

func parseOptionalDate(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	if _, err := time.Parse(PaymentDateLayout, trimmed); err != nil {
		return "", errors.New("yearly date must use DD.MM.YYYY format")
	}

	return trimmed, nil
}

func parseOptionalDay(raw string, minValue int, maxValue int, label string) (*int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return nil, errors.New(label + " day must be an integer")
	}

	if value < minValue || value > maxValue {
		return nil, fmt.Errorf("%s day must be between %d and %d", label, minValue, maxValue)
	}

	return &value, nil
}

func contains(options []string, value string) bool {
	for _, option := range options {
		if option == value {
			return true
		}
	}

	return false
}
