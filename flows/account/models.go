package account

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
)

type Account struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	Name          string
	// UID is what records refer to it by, so renaming it changes nothing
	// else. Set when it's first saved.
	UID         string `gorm:"not null;default:'';index"`
	Description string
	// CurrencyUID links the currency; its name (Currency) is filled in on
	// load and turned back into the link on save.
	CurrencyUID       string `gorm:"not null;default:''"`
	Currency          string `gorm:"-"`
	BalanceCents      int64
	LeftoverCents     int64
	IgnoreInSummaries bool
	// Archived accounts (closed, or kept only for their history) aren't
	// offered when picking an account for new records.
	Archived bool `gorm:"not null;default:false"`
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
