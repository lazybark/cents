package cashflow

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
)

type CashflowEntry struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	IsIncome      bool
	Currency      string
	AmountCents   int64
	// RateToBase is the currency's rate to the base currency when the entry
	// was made (0 when none was known) and AmountBaseCents the amount at it.
	RateToBase      float64
	AmountBaseCents int64
	EntryDate       time.Time
	Category        string
	AccountName     string
	Comment         string
}

const (
	CashflowFieldCurrency = iota
	CashflowFieldAmount
	CashflowFieldDate
	CashflowFieldCategory
	CashflowFieldAccount
	CashflowFieldComment
	CashflowFieldCount
)

type AddCashflowForm struct {
	Inputs          []textinput.Model
	Active          int
	CurrencyOptions []string
	CurrencyIndex   int
	IsIncome        bool
	CategoryOptions []string
	CategoryIndex   int
	AccountOptions  []string
	AccountIndex    int
}

type CashflowMonthlyOverviewRow struct {
	Month         time.Time
	IncomeBase    int64
	ExpenseBase   int64
	NetBase       int64
	DeltaFromPrev int64
	HasPrev       bool
}
