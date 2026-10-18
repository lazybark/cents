package invoice

import "time"

type Invoice struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	Title         string
	IsIncoming    bool
	// CurrencyUID links the currency; its name (Currency) is filled in on
	// load and turned back into the link on save.
	CurrencyUID string `gorm:"not null;default:''"`
	Currency    string `gorm:"-"`
	AmountCents int64
	// RateToBase is the currency's rate to the base currency when the
	// invoice was recorded (0 when none was known, or no currency), and
	// AmountBaseCents the amount at it.
	RateToBase      float64
	AmountBaseCents int64
	Paid            bool
	Peer            string
	InvoiceDate     *time.Time
	DueDate         *time.Time
	TargetAccount   string
	URL             string
	Description     string
	// CashflowEntryID is the expense (or income) added when the invoice was
	// marked paid, 0 for none; unmarking or deleting it deletes that too.
	CashflowEntryID uint `gorm:"not null;default:0"`
}
