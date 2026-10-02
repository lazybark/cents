package settings

import "time"

type AppSettings struct {
	BaseCurrency string
	// BaseCurrencyCode is the known currency (ISO code) the base stands
	// for, or "" when it isn't linked to one; rates can't be fetched then.
	BaseCurrencyCode string
	Rates            RatesState
	// CurrenciesSetUp is false for a new database until its currencies are
	// picked (or the pick skipped).
	CurrenciesSetUp   bool
	Currencies        []SettingCurrency
	PaymentMethods    []SettingPaymentMethod
	TaxTypes          []SettingTaxType
	IncomeCategories  []SettingIncomeCategory
	ExpenseCategories []SettingExpenseCategory
}

type SettingRecord struct {
	SettingID    string `gorm:"primaryKey"`
	SettingValue string
}

// RatesState is how rates are kept up to date: Auto fetches them once a
// day; UpdatedAt, Source and Date describe the last fetch.
type RatesState struct {
	Auto      bool
	UpdatedAt time.Time
	Source    string
	Date      string
}

// SettingCurrency is a currency records can use. CurrencyName is what
// records store; Code links it to a known currency (ISO code) so its rate
// can be fetched. An empty Code means "work it out from the name" (see
// LinkedCode).
type SettingCurrency struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time
	CurrencyName  string  `gorm:"not null;uniqueIndex"`
	RateToBase    float64 `gorm:"not null"`
	Code          string  `gorm:"not null;default:''"`
}

type SettingPaymentMethod struct {
	ID                uint `gorm:"primaryKey"`
	CreatedAt         time.Time
	LastUpdatedAt     time.Time
	PaymentMethodName string `gorm:"not null;uniqueIndex"`
	PaymentMethodType string `gorm:"not null"`
	IsDefault         bool   `gorm:"not null;default:false"`
}

type SettingTaxType struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time
	Country       string `gorm:"not null;uniqueIndex:idx_tax_type_country_name"`
	TaxTypeName   string `gorm:"not null;uniqueIndex:idx_tax_type_country_name"`
	Description   string
	URL           string
}

// Archived categories (for something temporary, like a side gig or buying
// a car) can't be picked for new entries and are hidden in the categories
// chart unless asked for; entries made with them stay as they are and still
// count in the totals.
type SettingIncomeCategory struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time
	CategoryName  string `gorm:"not null;uniqueIndex"`
	Archived      bool   `gorm:"not null;default:false"`
}

type SettingExpenseCategory struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time
	CategoryName  string `gorm:"not null;uniqueIndex"`
	Archived      bool   `gorm:"not null;default:false"`
}
