package cashflow

import "time"

type CashflowEntry struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	IsIncome      bool
	Currency      string
	AmountCents   int64
	EntryDate     time.Time
	Category      string
	AccountName   string
	Comment       string
}
