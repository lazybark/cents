// Package money holds amount parsing shared by every interface, so the TUI
// and the desktop app accept exactly the same input.
package money

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

// ParseAmountCents parses a non-negative decimal amount like "1234.56".
func ParseAmountCents(raw string) (int64, error) {
	amount, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, errors.New("amount must be a number")
	}

	if amount < 0 {
		return 0, errors.New("amount cannot be negative")
	}

	return int64(math.Round(amount * 100)), nil
}

// ParseSignedAmountCents parses an amount that may start with + or -, like
// "-25.50", for adjustments.
func ParseSignedAmountCents(raw string) (int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, errors.New("delta is required")
	}

	sign := int64(1)
	if strings.HasPrefix(trimmed, "+") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "+"))
	}

	if strings.HasPrefix(trimmed, "-") {
		sign = -1
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
	}

	amount, err := ParseAmountCents(trimmed)
	if err != nil {
		return 0, err
	}

	return sign * amount, nil
}

// ParseRate parses a conversion rate to the base currency, like "1.08".
func ParseRate(raw string) (float64, error) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, errors.New("rate must be a number")
	}

	if value <= 0 {
		return 0, errors.New("rate must be greater than zero")
	}

	return value, nil
}

// ToBaseCents converts cents at rate, rounding to the nearest cent.
func ToBaseCents(cents int64, rate float64) int64 {
	return int64(math.Round(float64(cents) * rate))
}
