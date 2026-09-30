// Package report turns records into tables for spreadsheets: one table per
// CSV file, with plain dates (YYYY-MM-DD), amounts as decimals with a dot
// (no thousands separators or currency signs), names instead of ids, and
// amounts both in their own currency and in the base currency.
package report

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
)

// Table is one CSV file: its name (without extension), header and rows.
type Table struct {
	Name   string
	Header []string
	Rows   [][]string
}

// Period limits the tables of things that happen over time (entries, logs,
// history) to the days from From to To, both included; a zero bound is
// open. Tables of current records (accounts, invoices, debts and so on)
// list them all.
type Period struct {
	From time.Time
	To   time.Time
}

// Contains reports whether day (read as the day it was saved) is in p.
func (p Period) Contains(day time.Time) bool {
	d := dates.Day(day)
	if !p.From.IsZero() && d.Before(dates.Day(p.From)) {
		return false
	}

	return p.To.IsZero() || !d.After(dates.Day(p.To))
}

// ContainsMonth reports whether any day of month is in p.
func (p Period) ContainsMonth(month time.Time) bool {
	first := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1)

	return (p.To.IsZero() || !first.After(dates.Day(p.To))) && (p.From.IsZero() || !last.Before(dates.Day(p.From)))
}

// amount is cents as a decimal: "-1234.50".
func amount(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}

	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}

// baseAmount is an amount in the base currency, empty when it couldn't be
// converted.
func baseAmount(cents int64, ok bool) string {
	if !ok {
		return ""
	}

	return amount(cents)
}

// rate is a rate to the base currency, empty when none is known.
func rate(value float64) string {
	if value <= 0 {
		return ""
	}

	return strconv.FormatFloat(value, 'f', -1, 64)
}

func day(value time.Time) string {
	if value.IsZero() {
		return ""
	}

	return dates.Text(value)
}

func optionalDay(value *time.Time) string {
	if value == nil {
		return ""
	}

	return day(*value)
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}

	return "no"
}

// inBase labels a column as being in the base currency: "Amount (€)".
func inBase(label, base string) string {
	return fmt.Sprintf("%s (%s)", label, strings.TrimSpace(base))
}
