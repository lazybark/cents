// Package credit holds loans the user owes a bank or another issuer:
// mortgages, car loans, credit cards and the like. Like debts, a credit
// keeps the rate to the base currency it was recorded with.
package credit

import "time"

type Credit struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	Name          string
	Purpose       string
	Issuer        string
	Currency      string
	// TotalCents is the whole credit (what was taken plus anything added
	// since, like drawdowns, interest or fees); PaidCents what is paid back.
	TotalCents int64
	PaidCents  int64
	// RateToBase is the currency's rate to the base currency when the credit
	// was recorded (0 when none was known); the base amounts are kept at it.
	RateToBase      float64
	TotalBaseCents  int64
	PaidBaseCents   int64
	InterestPercent float64
	StartDate       time.Time
	DueDate         *time.Time
	Comment         string
}

// CreditLog is a payment (DeltaPaidCents) or an addition to the credit
// (DeltaTotalCents); one of the two is set.
type CreditLog struct {
	ID              uint `gorm:"primaryKey"`
	CreatedAt       time.Time
	CreditID        uint `gorm:"index;not null"`
	DeltaPaidCents  int64
	DeltaTotalCents int64
	Note            string
	// CashflowEntryID is the expense added with this payment, 0 for none;
	// deleting the payment deletes it too.
	CashflowEntryID uint `gorm:"not null;default:0"`
}
