package goal

import "github.com/lazybark/cents/flows/settings"

// ProgressInBaseCents totals goals in the base currency, capping each goal's
// accumulated amount at its target. Goals without a conversion rate count
// with their raw amounts.
func ProgressInBaseCents(items []Goal, stts settings.AppSettings) (accumulated int64, target int64) {
	for _, item := range items {
		itemTarget := item.TargetAmountCents
		itemAccumulated := item.AmountAccumulatedCents
		if itemTarget < 0 {
			itemTarget = 0
		}

		if itemAccumulated < 0 {
			itemAccumulated = 0
		}

		if itemAccumulated > itemTarget {
			itemAccumulated = itemTarget
		}

		targetBase, targetOK := stts.ConvertToBaseCents(item.Currency, itemTarget)
		accumulatedBase, accumulatedOK := stts.ConvertToBaseCents(item.Currency, itemAccumulated)
		if targetOK && accumulatedOK {
			target += targetBase
			accumulated += accumulatedBase

			continue
		}

		target += itemTarget
		accumulated += itemAccumulated
	}

	if accumulated > target {
		accumulated = target
	}

	if accumulated < 0 {
		accumulated = 0
	}

	if target < 0 {
		target = 0
	}

	return accumulated, target
}
