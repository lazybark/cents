package app

import (
	"fmt"
	"math"
	"strings"
	"time"
)

func formatAmount(cents int64) string {
	return fmt.Sprintf("%.2f", float64(cents)/100)
}

func formatUpdatedAt(value time.Time) string {
	if value.IsZero() || value.Year() < 1971 {
		return "1970-01-01"
	}

	return value.Local().Format("2006-01-02 15:04")
}

func truncateText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return ""
	}

	if len(value) <= limit {
		return value
	}

	if limit <= 1 {
		return value[:limit]
	}

	return value[:limit-1] + "…"
}

func clamp(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}

	if value > maximum {
		return maximum
	}

	return value
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}

	return right
}

func formatRate(value float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", value), "0"), ".")
}

func beginningOfMonth(value time.Time) time.Time {
	local := value.Local()

	return time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, local.Location())
}

func monthShift(month time.Time, delta int) time.Time {
	base := beginningOfMonth(month)

	return base.AddDate(0, delta, 0)
}

func debtDirectionLabel(isOwedToUser bool) string {
	if isOwedToUser {
		return "incoming (someone owes me)"
	}

	return "outgoing (i owe someone)"
}

func (m TheApplication) baseCurrencyLabel() string {
	base := strings.TrimSpace(m.settings.BaseCurrency)
	if base == "" {
		return "$"
	}

	return base
}

func (m TheApplication) convertToBaseCents(currency string, cents int64) (int64, bool) {
	base := m.baseCurrencyLabel()
	if strings.EqualFold(strings.TrimSpace(currency), base) {
		return cents, true
	}

	rate, ok := m.rateToBase(currency)
	if !ok {
		return 0, false
	}

	return int64(math.Round(float64(cents) * rate)), true
}

func (m TheApplication) rateToBase(currency string) (float64, bool) {
	target := strings.TrimSpace(currency)
	if target == "" {
		return 0, false
	}

	for _, entry := range m.settings.Currencies {
		if strings.EqualFold(strings.TrimSpace(entry.CurrencyName), target) && entry.RateToBase > 0 {
			return entry.RateToBase, true
		}
	}

	return 0, false
}

func (m TheApplication) convertedAmountForBase(currency string, cents int64) string {
	base := m.baseCurrencyLabel()

	if strings.EqualFold(strings.TrimSpace(currency), base) {
		return ""
	}

	convertedCents, ok := m.convertToBaseCents(currency, cents)
	if !ok {
		return ""
	}

	return renderMoneyWithCurrency(base, convertedCents)
}
