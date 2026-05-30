package settings

import "time"

type AppSettings struct {
	BaseCurrency      string
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

type SettingCurrency struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time
	CurrencyName  string  `gorm:"not null;uniqueIndex"`
	RateToBase    float64 `gorm:"not null"`
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

type SettingIncomeCategory struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time
	CategoryName  string `gorm:"not null;uniqueIndex"`
}

type SettingExpenseCategory struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time
	CategoryName  string `gorm:"not null;uniqueIndex"`
}
