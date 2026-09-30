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
