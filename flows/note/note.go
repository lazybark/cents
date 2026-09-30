// Package note keeps notes on months: a few words on why a month looks the
// way it does ("trip to Dubai"), shown wherever the month is.
package note

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// MaxLength is the most a note can say, in characters.
const MaxLength = 500

// MonthLayout is how a month is written: YYYY-MM.
const MonthLayout = "2006-01"

// MonthNote is a note on a month, kept at its first day (UTC midnight).
type MonthNote struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Month     time.Time `gorm:"not null;uniqueIndex"`
	Text      string    `gorm:"not null"`
}

// ParseMonth reads a YYYY-MM month as notes keep it.
func ParseMonth(raw string) (time.Time, error) {
	month, err := time.Parse(MonthLayout, strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, errors.New("month must use YYYY-MM format")
	}

	return month, nil
}

// Clean tidies a typed note: trimmed, at most MaxLength characters. An
// empty one means no note.
func Clean(text string) (string, error) {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if n := utf8.RuneCountInString(text); n > MaxLength {
		return "", fmt.Errorf("a note can be at most %d characters; this one has %d", MaxLength, n)
	}

	return text, nil
}
