package asset

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/money"
)

type Kind string

const (
	KindProperty   Kind = "property"
	KindInvestment Kind = "investment"
)

// ParseKind checks raw names a kind of asset.
func ParseKind(raw string) (Kind, error) {
	switch kind := Kind(strings.TrimSpace(raw)); kind {
	case KindProperty, KindInvestment:
		return kind, nil
	default:
		return "", fmt.Errorf("unknown kind of asset %q", raw)
	}
}

// TypeOptions are common types of a kind of asset, offered as suggestions;
// any other type can be typed in.
func TypeOptions(kind Kind) []string {
	if kind == KindInvestment {
		return []string{"Stocks", "Bonds", "ETFs", "Mutual funds", "Crypto", "Deposits", "Precious metals", "REITs", "Options & derivatives", "Pension fund", "P2P lending", "Other"}
	}

	return []string{"Apartment", "House", "Land", "Commercial property", "Garage / parking", "Car", "Motorcycle", "Boat", "Jewelry", "Art & collectibles", "Equipment", "Other"}
}

// Fields are an asset's values as typed into a form; dates in format.
type Fields struct {
	Name             string
	Type             string
	Value            string
	Cost             string
	AcquiredAt       string
	Description      string
	IgnoreInNetWorth bool
}

// New validates a new asset of kind in currency and builds it.
func New(kind Kind, currency string, f Fields, format dates.Format, now time.Time) (Asset, error) {
	currency = strings.TrimSpace(currency)
	if currency == "" {
		return Asset{}, errors.New("currency is required")
	}

	return Asset{Kind: string(kind), Currency: currency}.Edit(f, format, now)
}

// Edit applies f; kind and currency stay as they are.
func (a Asset) Edit(f Fields, format dates.Format, now time.Time) (Asset, error) {
	name := strings.TrimSpace(f.Name)
	if name == "" {
		return Asset{}, errors.New("name is required")
	}

	assetType := strings.TrimSpace(f.Type)
	if assetType == "" {
		return Asset{}, errors.New("type is required")
	}

	value, err := money.ParseAmountCents(f.Value)
	if err != nil {
		return Asset{}, fmt.Errorf("value error: %w", err)
	}

	var cost int64
	if raw := strings.TrimSpace(f.Cost); raw != "" {
		if cost, err = money.ParseAmountCents(raw); err != nil {
			return Asset{}, fmt.Errorf("cost error: %w", err)
		}
	}

	acquired, err := format.Optional(f.AcquiredAt, "acquired")
	if err != nil {
		return Asset{}, err
	}

	a.Name = name
	a.Type = assetType
	a.ValueCents = value
	a.CostCents = cost
	a.AcquiredAt = acquired
	a.Description = strings.TrimSpace(f.Description)
	a.IgnoreInNetWorth = f.IgnoreInNetWorth
	a.LastUpdatedAt = now

	return a, nil
}

// GainCents is the value minus the cost, reporting false when no cost was
// given.
func (a Asset) GainCents() (int64, bool) {
	if a.CostCents <= 0 {
		return 0, false
	}

	return a.ValueCents - a.CostCents, true
}

// Filter keeps the assets of kind, by name.
func Filter(items []Asset, kind Kind) []Asset {
	filtered := make([]Asset, 0, len(items))
	for _, item := range items {
		if Kind(item.Kind) == kind {
			filtered = append(filtered, item)
		}
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		return strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name)
	})

	return filtered
}

// TypeTotal is the value of one type of asset in the base currency.
type TypeTotal struct {
	Type      string
	Count     int
	BaseCents int64
}

// Totals are assets' figures in the base currency. WithCostCents is the
// value of those with a cost, to compare with CostCents for the gain.
type Totals struct {
	ValueCents    int64
	CostCents     int64
	WithCostCents int64
	MissingRates  int
	Ignored       int
	ByType        []TypeTotal
}

// Total sums the assets counted in net worth in the base currency at
// today's rates. Assets without a rate are skipped and counted in
// MissingRates; ones left out of net worth are counted in Ignored.
func Total(items []Asset, stts settings.AppSettings) Totals {
	var totals Totals
	index := map[string]int{}

	for _, item := range items {
		if item.IgnoreInNetWorth {
			totals.Ignored++

			continue
		}

		value, ok := stts.ConvertToBaseCents(item.Currency, item.ValueCents)
		if !ok {
			totals.MissingRates++

			continue
		}

		totals.ValueCents += value

		if item.CostCents > 0 {
			cost, _ := stts.ConvertToBaseCents(item.Currency, item.CostCents)
			totals.CostCents += cost
			totals.WithCostCents += value
		}

		key := strings.ToLower(item.Type)
		i, seen := index[key]
		if !seen {
			i = len(totals.ByType)
			index[key] = i
			totals.ByType = append(totals.ByType, TypeTotal{Type: item.Type})
		}

		totals.ByType[i].Count++
		totals.ByType[i].BaseCents += value
	}

	sort.SliceStable(totals.ByType, func(i, j int) bool {
		return totals.ByType[i].BaseCents > totals.ByType[j].BaseCents
	})

	return totals
}
