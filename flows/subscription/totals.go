package subscription

import (
	"strings"

	"github.com/lazybark/cents/flows/settings"
)

// TotalsInBaseCents sums what subscriptions cost in the base currency: per
// month (those paid monthly or weekly, a week being 52/12 of a month) and
// per year (every period: weeks, months, quarters and years). Ones without
// a conversion rate are skipped.
func TotalsInBaseCents(subs []Subscription, stts settings.AppSettings) (monthlyTotal int64, yearlyProjection int64) {
	for _, sub := range subs {
		converted, ok := stts.ConvertToBaseCents(sub.Currency, sub.AmountCents)
		if !ok {
			continue
		}

		normalized := sub
		normalized.Period = strings.ToLower(strings.TrimSpace(sub.Period))
		perMonth, perYear := normalized.PerMonthAndYear(converted)
		monthlyTotal += perMonth
		yearlyProjection += perYear
	}

	return monthlyTotal, yearlyProjection
}
