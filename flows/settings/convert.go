package settings

import (
	"math"
	"strings"
)

func (s AppSettings) BaseCurrencyLabel() string {
	base := strings.TrimSpace(s.BaseCurrency)
	if base == "" {
		return "$"
	}

	return base
}

func (s AppSettings) RateToBase(currency string) (float64, bool) {
	target := strings.TrimSpace(currency)
	if target == "" {
		return 0, false
	}

	for _, entry := range s.Currencies {
		if strings.EqualFold(strings.TrimSpace(entry.CurrencyName), target) && entry.RateToBase > 0 {
			return entry.RateToBase, true
		}
	}

	return 0, false
}

func (s AppSettings) ConvertToBaseCents(currency string, cents int64) (int64, bool) {
	if strings.EqualFold(strings.TrimSpace(currency), s.BaseCurrencyLabel()) {
		return cents, true
	}

	rate, ok := s.RateToBase(currency)
	if !ok {
		return 0, false
	}

	return int64(math.Round(float64(cents) * rate)), true
}
