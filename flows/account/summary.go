package account

import (
	"sort"
	"strings"

	"github.com/lazybark/cents/flows/settings"
)

// CurrencyTotal is the sum of one non-base currency's accounts, in that
// currency and (when a rate exists) converted to the base currency.
type CurrencyTotal struct {
	Currency  string
	Cents     int64
	BaseCents int64
	HasRate   bool
}

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

// TotalsByCurrency sums accounts per non-base currency, skipping accounts
// ignored in summaries. Currencies match case-insensitively and are sorted
// by name. Currencies without a rate are included, since the base total
// leaves them out. BaseCents converts account by account, like
// SumInBaseCents, so the figures add up to the base total.
func TotalsByCurrency(accounts []Account, stts settings.AppSettings) []CurrencyTotal {
	base := strings.ToLower(stts.BaseCurrencyLabel())
	index := map[string]int{}
	totals := []CurrencyTotal{}

	for _, acct := range accounts {
		currency := strings.TrimSpace(acct.Currency)
		key := strings.ToLower(currency)
		if acct.IgnoreInSummaries || key == base {
			continue
		}

		i, seen := index[key]
		if !seen {
			i = len(totals)
			index[key] = i
			totals = append(totals, CurrencyTotal{Currency: currency})
		}

		converted, ok := stts.ConvertToBaseCents(currency, acct.BalanceCents)
		totals[i].Cents += acct.BalanceCents
		totals[i].BaseCents += converted
		totals[i].HasRate = ok
	}

	sort.Slice(totals, func(i, j int) bool {
		return strings.ToLower(totals[i].Currency) < strings.ToLower(totals[j].Currency)
	})

	return totals
}
