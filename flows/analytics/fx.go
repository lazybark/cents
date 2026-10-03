package analytics

import (
	"sort"
	"time"

	"github.com/lazybark/cents/money"
)

// FXMonth is how much rate changes moved what was held in other currencies
// over a month, in the base currency.
type FXMonth struct {
	Month time.Time
	Cents int64
}

// CurrencyEffect is one currency's part of the effect over the months.
type CurrencyEffect struct {
	Currency string
	Cents    int64
}

// FXEffect is the effect of exchange rates month by month. Missing names
// currencies held in some month without a rate known at its start or end;
// those months leave them out.
type FXEffect struct {
	Months     []FXMonth
	TotalCents int64
	ByCurrency []CurrencyEffect
	Missing    []string
}

// ExchangeEffect is, for each month from first to now, what was held at the
// start of the month (going by value logs) priced at the rate at the month's
// end (today, for the running month) minus at the rate at its start: the
// change in net worth from rates alone, holdings left as they were.
func ExchangeEffect(h History, rates Rates, first, now time.Time) FXEffect {
	result := FXEffect{Months: []FXMonth{}, ByCurrency: []CurrencyEffect{}, Missing: []string{}}
	byCurrency := map[string]int64{}
	names := map[string]string{}
	missing := map[string]string{}
	current := MonthStart(now)

	for month := MonthStart(first); !month.After(current); month = month.AddDate(0, 1, 0) {
		held := holdingsBefore(h, month, false)
		startDay := month.AddDate(0, 0, -1)
		endDay := month.AddDate(0, 1, -1)
		if month.Equal(current) {
			endDay = now
		}

		var cents int64
		for key, amount := range held.cents {
			if key == rates.base || amount == 0 {
				continue
			}

			from, okFrom := rates.At(key, startDay)
			to, okTo := rates.At(key, endDay)
			if !okFrom || !okTo {
				missing[key] = held.names[key]
				continue
			}

			effect := money.ToBaseCents(amount, to) - money.ToBaseCents(amount, from)
			cents += effect
			byCurrency[key] += effect
			names[key] = held.names[key]
		}

		result.Months = append(result.Months, FXMonth{Month: month, Cents: cents})
		result.TotalCents += cents
	}

	for key, cents := range byCurrency {
		result.ByCurrency = append(result.ByCurrency, CurrencyEffect{Currency: names[key], Cents: cents})
	}

	sort.Slice(result.ByCurrency, func(i, j int) bool {
		a, b := result.ByCurrency[i].Cents, result.ByCurrency[j].Cents
		if abs(a) != abs(b) {
			return abs(a) > abs(b)
		}

		return result.ByCurrency[i].Currency < result.ByCurrency[j].Currency
	})

	for _, name := range missing {
		result.Missing = append(result.Missing, name)
	}

	sort.Strings(result.Missing)

	return result
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}

	return v
}
