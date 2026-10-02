package settings

import (
	"strings"
	"time"

	"github.com/lazybark/cents/flows/currency"
)

// SettingRecord keys for currency links and rates.
const (
	BaseCurrencyCodeID    = "base_currency_code"
	BaseCurrencyCodeForID = "base_currency_code_for"
	RatesAutoID           = "rates_auto"
	RatesUpdatedAtID      = "rates_updated_at"
	RatesSourceID         = "rates_source"
	RatesDateID           = "rates_date"
	CurrenciesSetUpID     = "currencies_set_up"
)

// LinkedCode is the known currency this one stands for: its stored code,
// or one worked out from its name ("EUR", "€"); "" when there's none.
func (c SettingCurrency) LinkedCode() string {
	if code := strings.TrimSpace(c.Code); code != "" {
		return strings.ToUpper(code)
	}

	return currency.CodeFor(c.CurrencyName)
}

// BaseCodeFor is the code the base currency label stands for, given the
// link stored for it: the link counts only while it was made for this
// label, since another interface may rename the base.
func BaseCodeFor(label, linkedCode, linkedFor string) string {
	if linkedCode != "" && strings.EqualFold(strings.TrimSpace(linkedFor), strings.TrimSpace(label)) {
		return strings.ToUpper(linkedCode)
	}

	return currency.CodeFor(label)
}

// RatesDue reports whether rates should be fetched now: automatic updates
// are on, the base currency is linked, and the last fetch is a day old.
func (s AppSettings) RatesDue(now time.Time) bool {
	return s.Rates.Auto && s.BaseCurrencyCode != "" && (s.Rates.UpdatedAt.IsZero() || now.Sub(s.Rates.UpdatedAt) >= 24*time.Hour)
}

// Linkable reports whether a currency's rate can be fetched: both it and
// the base currency are linked to known currencies.
func (s AppSettings) Linkable(c SettingCurrency) bool {
	return s.BaseCurrencyCode != "" && c.LinkedCode() != ""
}
