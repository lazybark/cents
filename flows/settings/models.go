package settings

import "time"

type AppSettings struct {
	BaseCurrency string
	// BaseCurrencyCode is the known currency (ISO code) the base stands
	// for, or "" when it isn't linked to one; rates can't be fetched then.
	BaseCurrencyCode string
	// BaseCurrencyUID and BaseCurrencyID are the base currency's row.
	BaseCurrencyUID string
	BaseCurrencyID  uint
	Rates           RatesState
	// CurrenciesSetUp is false for a new database until its currencies are
	// picked (or the pick skipped).
	CurrenciesSetUp bool
	// Backup is the last backup made of the database.
	Backup            BackupState
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

// BackupState is when the last backup was made and where it went; At is
// zero before the first one.
type BackupState struct {
	At   time.Time
	Path string
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
	// UID is what records refer to it by, so renaming it changes nothing
	// else. Set when it's first saved.
	UID string `gorm:"not null;default:'';index"`
	// IsBase marks the base currency's row: it's in the table so records can
	// link it like any other, but settings keep it apart (BaseCurrency) and
	// leave it out of Currencies.
	IsBase bool `gorm:"not null;default:false"`
}

type SettingPaymentMethod struct {
	ID                uint `gorm:"primaryKey"`
	CreatedAt         time.Time
	LastUpdatedAt     time.Time
	PaymentMethodName string `gorm:"not null;uniqueIndex"`
	PaymentMethodType string `gorm:"not null"`
	// UID is what records refer to it by, so renaming it changes nothing
	// else. Set when it's first saved.
	UID string `gorm:"not null;default:'';index"`
	// Archived methods aren't offered for new payments.
	Archived  bool `gorm:"not null;default:false"`
	IsDefault bool `gorm:"not null;default:false"`
	// Currency is what the method pays in (a card's, say), "" for any; it
	// starts the currency of payments made with it. CurrencyUID links it.
	CurrencyUID string `gorm:"not null;default:''"`
	Currency    string `gorm:"-"`
}

type SettingTaxType struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time
	Country       string `gorm:"not null;uniqueIndex:idx_tax_type_country_name"`
	TaxTypeName   string `gorm:"not null;uniqueIndex:idx_tax_type_country_name"`
	// UID is what records refer to it by, so renaming it changes nothing
	// else. Set when it's first saved.
	UID         string `gorm:"not null;default:'';index"`
	Description string
	URL         string
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
	// UID is what records refer to it by, so renaming it changes nothing
	// else. Set when it's first saved.
	UID      string `gorm:"not null;default:'';index"`
	Archived bool   `gorm:"not null;default:false"`
}

type SettingExpenseCategory struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time
	CategoryName  string `gorm:"not null;uniqueIndex"`
	// UID is what records refer to it by, so renaming it changes nothing
	// else. Set when it's first saved.
	UID      string `gorm:"not null;default:'';index"`
	Archived bool   `gorm:"not null;default:false"`
}

// RateRecord is a currency's rate to the base currency (Base) on a day, kept
// whenever rates are saved, so analytics can tell how much rate changes
// moved what's held in other currencies. A day keeps its last rate.
type RateRecord struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Day       time.Time `gorm:"not null;uniqueIndex:idx_rate_record_links"`
	// The currency and the base it's against, linked; their names are
	// filled in on load and turned back into links on save.
	CurrencyUID string  `gorm:"not null;default:'';uniqueIndex:idx_rate_record_links"`
	BaseUID     string  `gorm:"not null;default:'';uniqueIndex:idx_rate_record_links"`
	Currency    string  `gorm:"-"`
	Base        string  `gorm:"-"`
	RateToBase  float64 `gorm:"not null"`
}
