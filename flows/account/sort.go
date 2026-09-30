package account

import (
	"sort"
	"strings"

	"github.com/lazybark/cents/flows/settings"
)

type SortField int

const (
	SortBaseAmount SortField = iota
	SortName
	SortCurrency
	SortUpdated
)

func SortOptions() []string {
	return []string{
		"Balance in base currency",
		"Name",
		"Currency",
		"Last updated",
	}
}

func SortLabel(field SortField) string {
	options := SortOptions()
	index := int(field)
	if index < 0 || index >= len(options) {
		return options[0]
	}

	return options[index]
}

// Sort returns a sorted copy. Ties fall back to newest created first.
func Sort(accounts []Account, stts settings.AppSettings, field SortField) []Account {
	if len(accounts) < 2 {
		return accounts
	}

	cloned := make([]Account, len(accounts))
	copy(cloned, accounts)
	sort.SliceStable(cloned, func(i, j int) bool {
		left := cloned[i]
		right := cloned[j]

		switch field {
		case SortName:
			leftName := strings.ToLower(strings.TrimSpace(left.Name))
			rightName := strings.ToLower(strings.TrimSpace(right.Name))
			if leftName != rightName {
				return leftName < rightName
			}
		case SortCurrency:
			leftCurrency := strings.ToLower(strings.TrimSpace(left.Currency))
			rightCurrency := strings.ToLower(strings.TrimSpace(right.Currency))
			if leftCurrency != rightCurrency {
				return leftCurrency < rightCurrency
			}
		case SortUpdated:
			if !left.LastUpdatedAt.Equal(right.LastUpdatedAt) {
				return left.LastUpdatedAt.After(right.LastUpdatedAt)
			}
		default:
			leftComparable := comparableBaseCents(left, stts)
			rightComparable := comparableBaseCents(right, stts)
			if leftComparable != rightComparable {
				return leftComparable > rightComparable
			}
		}

		if left.CreatedAt.Equal(right.CreatedAt) {
			return left.ID > right.ID
		}

		return left.CreatedAt.After(right.CreatedAt)
	})

	return cloned
}

// comparableBaseCents converts to the base currency for sorting, falling back
// to the raw balance when there is no conversion rate.
func comparableBaseCents(acct Account, stts settings.AppSettings) int64 {
	if converted, ok := stts.ConvertToBaseCents(acct.Currency, acct.BalanceCents); ok {
		return converted
	}

	return acct.BalanceCents
}
