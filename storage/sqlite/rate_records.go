package sqlite

import (
	"fmt"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/settings"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LoadRateRecords returns the rates kept day by day, oldest first.
func (s *SQLiteStorage) LoadRateRecords() ([]settings.RateRecord, error) {
	var items []settings.RateRecord

	if err := s.db.Order("day asc, id asc").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to load rate history: %w", err)
	}

	return items, nil
}

// recordRates keeps today's rate of each currency in currencies (all of
// them when nil) in the rate history, against the base currency as it is
// now in db.
func recordRates(db *gorm.DB, currencies []settings.SettingCurrency, now time.Time) error {
	stts, err := LoadAppSettings(db)
	if err != nil {
		return err
	}

	if currencies == nil {
		currencies = stts.Currencies
	}

	day := dates.Day(now)
	base := stts.BaseCurrencyLabel()
	records := make([]settings.RateRecord, 0, len(currencies))
	for _, c := range currencies {
		if c.RateToBase <= 0 || stts.IsBase(c.CurrencyName) {
			continue
		}

		records = append(records, settings.RateRecord{Day: day, Currency: c.CurrencyName, Base: base, RateToBase: c.RateToBase})
	}

	if len(records) == 0 {
		return nil
	}

	err = db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "day"}, {Name: "currency_uid"}, {Name: "base_uid"}},
		DoUpdates: clause.AssignmentColumns([]string{"updated_at", "rate_to_base"}),
	}).Create(&records).Error
	if err != nil {
		return fmt.Errorf("failed to keep rate history: %w", err)
	}

	return nil
}
