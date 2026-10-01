package settings

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

// BaseCurrencySettingID is the SettingRecord key of the base currency.
const BaseCurrencySettingID = "base_currency"

// The functions below validate edits to settings with the same rules and
// messages in every interface. Each takes the record being edited (a zero
// value for a new one) and returns it updated.

func PaymentMethodTypeOptions() []string {
	return []string{"Card", "Crypto", "E-Wallet", "Other"}
}

func BaseCurrencyRecord(value string) (SettingRecord, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return SettingRecord{}, errors.New("base currency cannot be empty")
	}

	return SettingRecord{SettingID: BaseCurrencySettingID, SettingValue: value}, nil
}

func (c SettingCurrency) Apply(name, rate string, now time.Time) (SettingCurrency, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return SettingCurrency{}, errors.New("currency name is required")
	}

	value, err := strconv.ParseFloat(strings.TrimSpace(rate), 64)
	if err != nil {
		return SettingCurrency{}, errors.New("rate must be a number")
	}

	if value <= 0 {
		return SettingCurrency{}, errors.New("rate must be greater than zero")
	}

	c.CurrencyName = name
	c.RateToBase = value
	stamp(&c.CreatedAt, &c.LastUpdatedAt, c.ID, now)

	return c, nil
}

// Apply for a payment method: the first one ever added becomes the default.
func (p SettingPaymentMethod) Apply(name, methodType string, isDefault bool, isFirst bool, now time.Time) (SettingPaymentMethod, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return SettingPaymentMethod{}, errors.New("payment method name is required")
	}

	known := false
	for _, option := range PaymentMethodTypeOptions() {
		if option == methodType {
			known = true

			break
		}
	}

	if !known {
		return SettingPaymentMethod{}, errors.New("payment method type must be one of " + strings.Join(PaymentMethodTypeOptions(), ", "))
	}

	if isFirst && p.ID == 0 {
		isDefault = true
	}

	p.PaymentMethodName = name
	p.PaymentMethodType = methodType
	p.IsDefault = isDefault
	stamp(&p.CreatedAt, &p.LastUpdatedAt, p.ID, now)

	return p, nil
}

func (t SettingTaxType) Apply(country, name, description, url string, now time.Time) (SettingTaxType, error) {
	country = strings.TrimSpace(country)
	name = strings.TrimSpace(name)

	if country == "" {
		return SettingTaxType{}, errors.New("country is required")
	}

	if name == "" {
		return SettingTaxType{}, errors.New("tax type name is required")
	}

	t.Country = country
	t.TaxTypeName = name
	t.Description = strings.TrimSpace(description)
	t.URL = strings.TrimSpace(url)
	stamp(&t.CreatedAt, &t.LastUpdatedAt, t.ID, now)

	return t, nil
}

func (c SettingIncomeCategory) Apply(name string, now time.Time) (SettingIncomeCategory, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return SettingIncomeCategory{}, errors.New("income category name is required")
	}

	c.CategoryName = name
	stamp(&c.CreatedAt, &c.LastUpdatedAt, c.ID, now)

	return c, nil
}

func (c SettingExpenseCategory) Apply(name string, now time.Time) (SettingExpenseCategory, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return SettingExpenseCategory{}, errors.New("expense category name is required")
	}

	c.CategoryName = name
	stamp(&c.CreatedAt, &c.LastUpdatedAt, c.ID, now)

	return c, nil
}

// stamp sets LastUpdatedAt, and CreatedAt for records not saved yet.
func stamp(createdAt *time.Time, updatedAt *time.Time, id uint, now time.Time) {
	*updatedAt = now
	if id == 0 {
		*createdAt = now
	}
}
