// Package dates parses the dates people type into a form. Each interface
// has its own format (the TUI takes DD.MM.YYYY, the desktop's date inputs
// send YYYY-MM-DD), so parsing takes the format and errors name it.
package dates

import (
	"errors"
	"strings"
	"time"
)

type Format struct {
	// Layout is the Go time layout.
	Layout string
	// Label is how the format reads in error messages.
	Label string
}

var (
	TUI = Format{Layout: "02.01.2006", Label: "DD.MM.YYYY"}
	ISO = Format{Layout: "2006-01-02", Label: "YYYY-MM-DD"}
)

// Required parses a mandatory date. what names the field in errors, like
// "created" in "created date is required".
func (f Format) Required(raw, what string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, errors.New(what + " date is required")
	}

	value, err := time.Parse(f.Layout, trimmed)
	if err != nil {
		return time.Time{}, errors.New(what + " date must use " + f.Label + " format")
	}

	return value, nil
}

// Optional parses a date that may be left empty, returning nil then.
func (f Format) Optional(raw, what string) (*time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	value, err := time.Parse(f.Layout, trimmed)
	if err != nil {
		return nil, errors.New(what + " date must use " + f.Label + " format")
	}

	return &value, nil
}

// LogTime is when a transaction log entry happened: now when raw is empty,
// otherwise that day at the current time of day.
func (f Format) LogTime(raw string, now time.Time) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return now, nil
	}

	value, err := time.Parse(f.Layout, trimmed)
	if err != nil {
		return time.Time{}, errors.New("log date must use " + f.Label + " format")
	}

	return time.Date(value.Year(), value.Month(), value.Day(), now.Hour(), now.Minute(), now.Second(), 0, now.Location()), nil
}
