package account

import "github.com/lazybark/cents/flows/settings"

// SumInBaseCents totals account balances in the base currency, skipping
// accounts ignored in summaries and those without a conversion rate.
func SumInBaseCents(accounts []Account, stts settings.AppSettings) int64 {
	var total int64

	for _, acct := range accounts {
		if acct.IgnoreInSummaries {
			continue
		}

		converted, ok := stts.ConvertToBaseCents(acct.Currency, acct.BalanceCents)
		if !ok {
			continue
		}

		total += converted
	}

	return total
}
