// Package asset holds what the user owns besides money on accounts:
// property (cars, apartments, land…) and investments (stocks, bonds…).
// Both work the same way and count towards net worth at today's rates,
// like accounts.
package asset

import "time"

type Asset struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	Kind          string    `gorm:"not null;index"`
	Name          string
	Type          string
	// CurrencyUID links the currency; its name (Currency) is filled in on
	// load and turned back into the link on save.
	CurrencyUID string `gorm:"not null;default:''"`
	Currency    string `gorm:"-"`
	// ValueCents is what the asset is worth now; CostCents what was paid
	// for it (0 when not given).
	ValueCents       int64
	CostCents        int64
	AcquiredAt       *time.Time
	Description      string
	IgnoreInNetWorth bool
}

// AssetValueLog is the asset's value on a day, one entry per day.
type AssetValueLog struct {
	ID         uint `gorm:"primaryKey"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	AssetID    uint      `gorm:"not null;index;uniqueIndex:idx_asset_log_day"`
	LogDate    time.Time `gorm:"not null;uniqueIndex:idx_asset_log_day"`
	ValueCents int64
}
