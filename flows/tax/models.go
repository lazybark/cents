package tax

import "time"

type Tax struct {
	ID              uint `gorm:"primaryKey"`
	CreatedAt       time.Time
	LastUpdatedAt   time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	TaxTypeID       uint
	TaxCountry      string
	TaxTypeName     string
	AmountDueCents  int64
	AmountPaidCents int64
	Period          string
	DueDate         *time.Time
	Comment         string
}

type TaxLog struct {
	ID             uint `gorm:"primaryKey"`
	CreatedAt      time.Time
	TaxID          uint `gorm:"index;not null"`
	DeltaPaidCents int64
	Note           string
}
