package account

import "time"

type Account struct {
	ID                uint `gorm:"primaryKey"`
	CreatedAt         time.Time
	LastUpdatedAt     time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	Name              string
	Description       string
	Currency          string
	BalanceCents      int64
	LeftoverCents     int64
	IgnoreInSummaries bool
}

type AccountValueLog struct {
	ID         uint `gorm:"primaryKey"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	AccountID  uint      `gorm:"not null;index;uniqueIndex:idx_account_log_day"`
	LogDate    time.Time `gorm:"not null;uniqueIndex:idx_account_log_day"`
	ValueCents int64
}
