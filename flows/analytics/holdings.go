package analytics

import (
	"time"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/asset"
)

// held is what's owned at a month's end in each currency (by key), from
// value logs, with the name to show for each.
type held struct {
	cents map[string]int64
	names map[string]string
}

func (h *held) add(currency string, cents int64) {
	key := currencyKey(currency)
	if key == "" {
		return
	}

	if _, ok := h.names[key]; !ok {
		h.names[key] = currency
	}

	h.cents[key] += cents
}

// holdingsBefore is what accounts counted in summaries and assets counted in
// net worth were worth before end (a UTC midnight), going by their value
// logs; with current set (the running month), by their amounts now.
func holdingsBefore(h History, end time.Time, current bool) held {
	result := held{cents: map[string]int64{}, names: map[string]string{}}
	accountLogs := map[uint][]account.AccountValueLog{}
	for _, l := range h.AccountLogs {
		accountLogs[l.AccountID] = append(accountLogs[l.AccountID], l)
	}

	assetLogs := map[uint][]asset.AssetValueLog{}
	for _, l := range h.AssetLogs {
		assetLogs[l.AssetID] = append(assetLogs[l.AssetID], l)
	}

	for _, acct := range h.Accounts {
		if acct.IgnoreInSummaries {
			continue
		}

		if current {
			result.add(acct.Currency, acct.BalanceCents)
		} else if cents, ok := latestBefore(accountLogs[acct.ID], end, func(l account.AccountValueLog) (time.Time, int64) { return l.LogDate, l.ValueCents }); ok {
			result.add(acct.Currency, cents)
		}
	}

	for _, item := range h.Assets {
		if item.IgnoreInNetWorth {
			continue
		}

		if current {
			result.add(item.Currency, item.ValueCents)
		} else if cents, ok := latestBefore(assetLogs[item.ID], end, func(l asset.AssetValueLog) (time.Time, int64) { return l.LogDate, l.ValueCents }); ok {
			result.add(item.Currency, cents)
		}
	}

	return result
}
