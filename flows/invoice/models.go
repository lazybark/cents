package invoice

import "time"

type Invoice struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	Title         string
	IsIncoming    bool
	Currency      string
	AmountCents   int64
	Paid          bool
	Peer          string
	InvoiceDate   *time.Time
	DueDate       *time.Time
	TargetAccount string
	URL           string
	Description   string
}
