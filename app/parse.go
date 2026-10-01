package app

import (
	"errors"
	"strings"
	"time"

	"github.com/lazybark/cents/flows/tax"
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
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, errors.New("created date is required")
	}

	value, err := time.Parse("02.01.2006", trimmed)
	if err != nil {
		return time.Time{}, errors.New("created date must use DD.MM.YYYY format")
	}

	return value, nil
}

func parseOptionalDatePointer(raw string) (*time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	value, err := time.Parse("02.01.2006", trimmed)
	if err != nil {
		return nil, errors.New("due date must use DD.MM.YYYY format")
	}

	return &value, nil
}

func parseSignedAmountCents(raw string) (int64, error) {
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

	amount, err := parseAmountCents(trimmed)
	if err != nil {
		return 0, err
	}

	return sign * amount, nil
}

func parseLogDateOrToday(raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Now(), nil
	}

	value, err := time.Parse("02.01.2006", trimmed)
	if err != nil {
		return time.Time{}, errors.New("log date must use DD.MM.YYYY format")
	}

	now := time.Now()

	return time.Date(value.Year(), value.Month(), value.Day(), now.Hour(), now.Minute(), now.Second(), 0, now.Location()), nil
}

func taxProgressTotals(items []tax.Tax) (paid int64, total int64) {
	for _, item := range items {
		itemTotal := item.AmountDueCents
		itemPaid := item.AmountPaidCents

		if itemTotal < 0 {
			itemTotal = 0
		}

		if itemPaid < 0 {
			itemPaid = 0
		}

		if itemPaid > itemTotal {
			itemPaid = itemTotal
		}

		total += itemTotal
		paid += itemPaid
	}

	if paid > total {
		paid = total
	}

	if paid < 0 {
		paid = 0
	}

	if total < 0 {
		total = 0
	}

	return paid, total
}
