package tax

import "time"

type Tax struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	// TaxTypeUID links the tax type; its id, country and name are filled
	// in on load.
	TaxTypeUID      string `gorm:"not null;default:''"`
	TaxTypeID       uint   `gorm:"-"`
	TaxCountry      string `gorm:"-"`
	TaxTypeName     string `gorm:"-"`
	AmountDueCents  int64
	AmountPaidCents int64
	// Currency is what the tax is paid in. RateToBase is its rate to the
	// base currency when the tax was recorded; the base amounts are kept at
	// that rate, so later rate changes in settings don't move them.
	// CurrencyUID links the currency; its name (Currency) is filled in on
	// load and turned back into the link on save.
	CurrencyUID         string `gorm:"not null;default:''"`
	Currency            string `gorm:"-"`
	RateToBase          float64
	AmountDueBaseCents  int64
	AmountPaidBaseCents int64
	Period              string
	DueDate             *time.Time
	Comment             string
}

type TaxLog struct {
	ID             uint `gorm:"primaryKey"`
	CreatedAt      time.Time
	TaxID          uint `gorm:"index;not null"`
	DeltaPaidCents int64
	Note           string
}
