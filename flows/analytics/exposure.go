package analytics

import (
	"sort"

	"github.com/lazybark/cents/summary"
)

// Exposure is what's held in one currency now: owned (accounts counted in
// summaries, property and investments counted in net worth) and what's
// left owed to and by the user in it (debts, invoices, credits, taxes), in
// that currency and in the base currency at today's rate.
type Exposure struct {
	Currency      string
	OwnedCents    int64
	OwedToMeCents int64
	OwedByMeCents int64
	// The base amounts; HasRate is false when the currency has no rate.
	OwnedBaseCents int64
	NetBaseCents   int64
	HasRate        bool
	// Share is the part of everything owned, in percent.
	Share float64
}

func (e Exposure) NetCents() int64 {
	return e.OwnedCents + e.OwedToMeCents - e.OwedByMeCents
}

// CurrencyExposure splits what's held now by currency, largest owned first.
func CurrencyExposure(data summary.Data) []Exposure {
	stts := data.Settings
	byKey := map[string]*Exposure{}
	order := []string{}
	get := func(currency string) *Exposure {
		key := currencyKey(currency)
		if key == "" {
			return nil
		}

		if _, ok := byKey[key]; !ok {
			byKey[key] = &Exposure{Currency: currency}
			order = append(order, key)
		}

		return byKey[key]
	}

	for _, acct := range data.Accounts {
		if e := get(acct.Currency); e != nil && !acct.IgnoreInSummaries {
			e.OwnedCents += acct.BalanceCents
		}
	}

	for _, item := range data.Assets {
		if e := get(item.Currency); e != nil && !item.IgnoreInNetWorth {
			e.OwnedCents += item.ValueCents
		}
	}

	for _, d := range data.Debts {
		if e := get(d.Currency); e != nil && !d.IsPaid() {
			if d.IsOwedToUser {
				e.OwedToMeCents += d.LeftCents()
			} else {
				e.OwedByMeCents += d.LeftCents()
			}
		}
	}

	for _, inv := range data.Invoices {
		if e := get(inv.Currency); e != nil && !inv.Paid {
			if inv.IsIncoming {
				e.OwedByMeCents += inv.AmountCents
			} else {
				e.OwedToMeCents += inv.AmountCents
			}
		}
	}

	for _, c := range data.Credits {
		if e := get(c.Currency); e != nil && !c.IsPaid() {
			e.OwedByMeCents += c.LeftCents()
		}
	}

	for _, t := range data.Taxes {
		if e := get(t.Currency); e != nil && !t.IsPaid() {
			e.OwedByMeCents += t.LeftCents()
		}
	}

	result := make([]Exposure, 0, len(order))
	var owned int64
	for _, key := range order {
		e := *byKey[key]
		if e.OwnedCents == 0 && e.OwedToMeCents == 0 && e.OwedByMeCents == 0 {
			continue
		}

		if base, ok := stts.ConvertToBaseCents(e.Currency, e.OwnedCents); ok {
			e.OwnedBaseCents = base
			e.NetBaseCents, _ = stts.ConvertToBaseCents(e.Currency, e.NetCents())
			e.HasRate = true
			if base > 0 {
				owned += base
			}
		}

		result = append(result, e)
	}

	for i := range result {
		if owned > 0 && result[i].HasRate && result[i].OwnedBaseCents > 0 {
			result[i].Share = float64(result[i].OwnedBaseCents) / float64(owned) * 100
		}
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].OwnedBaseCents != result[j].OwnedBaseCents {
			return result[i].OwnedBaseCents > result[j].OwnedBaseCents
		}

		return result[i].NetBaseCents > result[j].NetBaseCents
	})

	return result
}
