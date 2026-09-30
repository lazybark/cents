package subscription

import (
	"strings"

	"github.com/lazybark/cents/flows/settings"
)

// TotalsInBaseCents sums monthly subscriptions and projects a year of cost
// (yearly subscriptions plus twelve months of monthly ones), in the base
// currency. Subscriptions without a conversion rate are skipped.
func TotalsInBaseCents(subs []Subscription, stts settings.AppSettings) (monthlyTotal int64, yearlyProjection int64) {
	var yearlyOnly int64

	for _, sub := range subs {
		converted, ok := stts.ConvertToBaseCents(sub.Currency, sub.AmountCents)
		if !ok {
			continue
		}

		if strings.EqualFold(strings.TrimSpace(sub.Period), "month") {
			monthlyTotal += converted

			continue
		}

		if strings.EqualFold(strings.TrimSpace(sub.Period), "year") {
			yearlyOnly += converted
		}
	}

	yearlyProjection = yearlyOnly + monthlyTotal*12

	return monthlyTotal, yearlyProjection
}
