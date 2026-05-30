package debt

import "time"

type Debt struct {
	ID              uint `gorm:"primaryKey"`
	CreatedAt       time.Time
	LastUpdatedAt   time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	Peer            string
	Currency        string
	AmountCents     int64
	AmountPaidCents int64
	IsOwedToUser    bool
	DebtCreatedAt   time.Time
	DueDate         *time.Time
	Comment         string
}

type DebtLog struct {
	ID             uint `gorm:"primaryKey"`
	CreatedAt      time.Time
	DebtID         uint `gorm:"index;not null"`
	DeltaPaidCents int64
	Note           string
}
