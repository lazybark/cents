package app

import (
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/money"
)

func parseAmountCents(raw string) (int64, error) {
	return money.ParseAmountCents(raw)
}

func parseOptionalAmountCents(raw string) (int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}

	return parseAmountCents(trimmed)
}

func parseRequiredDate(raw string) (time.Time, error) {
	return dates.TUI.Required(raw, "created")
}

func parseOptionalDatePointer(raw string) (*time.Time, error) {
	return dates.TUI.Optional(raw, "due")
}

func parseSignedAmountCents(raw string) (int64, error) {
	return money.ParseSignedAmountCents(raw)
}

func parseLogDateOrToday(raw string) (time.Time, error) {
	return dates.TUI.LogTime(raw, time.Now())
}
