package analytics

import (
	"errors"
	"strings"
	"time"

	"github.com/lazybark/cents/money"
)

// MonthLayout is how a month is written: YYYY-MM.
const MonthLayout = "2006-01"

// ParseMonth reads a YYYY-MM month for the net worth history: one that has
// begun by now.
func ParseMonth(raw string, now time.Time) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, errors.New("month is required")
	}

	month, err := time.Parse(MonthLayout, trimmed)
	if err != nil {
		return time.Time{}, errors.New("month must use YYYY-MM format")
	}

	if month.After(MonthStart(now)) {
		return time.Time{}, errors.New("month can't be in the future")
	}

	return month, nil
}

// Manual is a net worth the user entered for a month, like a backfill. It
// keeps no breakdown; the amount may be negative.
func Manual(rawMonth, rawAmount string, now time.Time) (NetWorthSnapshot, error) {
	month, err := ParseMonth(rawMonth, now)
	if err != nil {
		return NetWorthSnapshot{}, err
	}

	if strings.TrimSpace(rawAmount) == "" {
		return NetWorthSnapshot{}, errors.New("net worth is required")
	}

	cents, err := money.ParseSignedAmountCents(rawAmount)
	if err != nil {
		return NetWorthSnapshot{}, errors.New("net worth must be a number, like 1234.56 or -500")
	}

	return NetWorthSnapshot{Month: month, NetWorthCents: cents, Manual: true}, nil
}
