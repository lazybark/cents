package budget

import "time"

// Budget is a monthly spending limit for an expense category, or for all
// spending when Category is empty. The limit is in the base currency, like
// the totals spending is compared in.
type Budget struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	// CategoryUID links the expense category, "" for all spending; its
	// name (Category) is filled in on load.
	CategoryUID string `gorm:"not null;default:''"`
	Category    string `gorm:"-"`
	LimitCents  int64  `gorm:"not null"`
}

// IsTotal reports whether b limits all spending rather than a category.
func (b Budget) IsTotal() bool {
	return b.Category == ""
}
