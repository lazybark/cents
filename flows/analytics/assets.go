package analytics

import (
	"time"

	"github.com/lazybark/cents/flows/asset"
	"github.com/lazybark/cents/flows/settings"
)

// AssetMonth is property or investments at a month's end, in the base
// currency at today's rates (so rate changes don't show up as gains).
// Gain is value minus cost of those with a cost given; HasCost is false
// when none has one.
type AssetMonth struct {
	Month      time.Time
	ValueCents int64
	CostCents  int64
	GainCents  int64
	HasCost    bool
}

// AssetGrowth is one kind's value month by month from first to now: the
// latest value logged by each month's end (the value now for the running
// month). Assets left out of net worth, without a value yet or without a
// rate aren't counted.
func AssetGrowth(assets []asset.Asset, logs []asset.AssetValueLog, stts settings.AppSettings, kind asset.Kind, first, now time.Time) []AssetMonth {
	byAsset := map[uint][]asset.AssetValueLog{}
	for _, l := range logs {
		byAsset[l.AssetID] = append(byAsset[l.AssetID], l)
	}

	current := MonthStart(now)
	months := []AssetMonth{}
	for month := MonthStart(first); !month.After(current); month = month.AddDate(0, 1, 0) {
		m := AssetMonth{Month: month}
		for _, item := range assets {
			if item.Kind != string(kind) || item.IgnoreInNetWorth {
				continue
			}

			value, ok := item.ValueCents, true
			if !month.Equal(current) {
				value, ok = latestBefore(byAsset[item.ID], month.AddDate(0, 1, 0), func(l asset.AssetValueLog) (time.Time, int64) { return l.LogDate, l.ValueCents })
			}

			if !ok {
				continue
			}

			base, ok := stts.ConvertToBaseCents(item.Currency, value)
			if !ok {
				continue
			}

			m.ValueCents += base
			if item.CostCents > 0 {
				cost, _ := stts.ConvertToBaseCents(item.Currency, item.CostCents)
				m.CostCents += cost
				m.GainCents += base - cost
				m.HasCost = true
			}
		}

		months = append(months, m)
	}

	return months
}
