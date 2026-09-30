package rates

import (
	"time"

	"github.com/lazybark/cents/flows/settings"
)

// Apply puts a quote's rates on every linked currency (other than the base
// itself) and returns them, with the records that say when and where the
// rates came from, to save together; missing lists the currencies the
// quote had no rate for. Currencies that aren't linked keep their rates.
func Apply(stts settings.AppSettings, quote Quote, now time.Time) (updated []settings.SettingCurrency, records []settings.SettingRecord, missing []string) {
	for _, c := range stts.Currencies {
		code := c.LinkedCode()
		if code == "" || code == stts.BaseCurrencyCode {
			continue
		}

		rate, ok := quote.RateToBase(code)
		if !ok {
			missing = append(missing, c.CurrencyName)

			continue
		}

		c.RateToBase = rate
		c.LastUpdatedAt = now
		updated = append(updated, c)
	}

	records = []settings.SettingRecord{
		{SettingID: settings.RatesUpdatedAtID, SettingValue: now.UTC().Format(time.RFC3339)},
		{SettingID: settings.RatesSourceID, SettingValue: quote.Source},
		{SettingID: settings.RatesDateID, SettingValue: quote.Date},
	}

	return updated, records, missing
}
