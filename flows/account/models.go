package account

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
)

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

type AddAccountForm struct {
	Fields            []textinput.Model
	Labels            []string
	CurrencyOptions   []string
	CurrencyIndex     int
	IgnoreInSummaries bool
	Active            int
}
