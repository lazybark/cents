package debt

import "time"

type Debt struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	Peer          string
	// CurrencyUID links the currency; its name (Currency) is filled in on
	// load and turned back into the link on save.
	CurrencyUID     string `gorm:"not null;default:''"`
	Currency        string `gorm:"-"`
	AmountCents     int64
	AmountPaidCents int64
	// RateToBase is the currency's rate to the base currency when the debt
	// was recorded (0 when none was known); the base amounts are kept at it.
	RateToBase          float64
	AmountBaseCents     int64
	AmountPaidBaseCents int64
	IsOwedToUser        bool
	DebtCreatedAt       time.Time
	DueDate             *time.Time
	Comment             string
}

type DebtLog struct {
	ID             uint `gorm:"primaryKey"`
	CreatedAt      time.Time
	DebtID         uint `gorm:"index;not null"`
	DeltaPaidCents int64
	Note           string
	// CashflowEntryID is the expense (or income) added with this payment,
	// 0 for none; deleting the payment deletes it too.
	CashflowEntryID uint `gorm:"not null;default:0"`
}
