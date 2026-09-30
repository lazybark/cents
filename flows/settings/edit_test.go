package settings

import (
	"testing"
	"time"
)

func TestApplyValidates(t *testing.T) {
	now := time.Now()
	errs := map[string]error{}

	_, errs["base currency cannot be empty"] = BaseCurrencyRecord("  ")
	_, errs["currency name is required"] = SettingCurrency{}.Apply(" ", "1", now)
	_, errs["rate must be a number"] = SettingCurrency{}.Apply("EUR", "x", now)
	_, errs["rate must be greater than zero"] = SettingCurrency{}.Apply("EUR", "0", now)
	_, errs["payment method name is required"] = SettingPaymentMethod{}.Apply("", "Card", false, false, now)
	_, errs["payment method type must be one of Card, Crypto, E-Wallet, Other"] = SettingPaymentMethod{}.Apply("Visa", "Cash", false, false, now)
	_, errs["country is required"] = SettingTaxType{}.Apply("", "VAT", "", "", now)
	_, errs["tax type name is required"] = SettingTaxType{}.Apply("NL", "", "", "", now)
	_, errs["income category name is required"] = SettingIncomeCategory{}.Apply("", now)
	_, errs["expense category name is required"] = SettingExpenseCategory{}.Apply("", now)

	for want, err := range errs {
		if err == nil || err.Error() != want {
			t.Errorf("expected %q, got %v", want, err)
		}
	}
}

func TestApplyBuildsRecords(t *testing.T) {
	now := time.Now()
	created := now.Add(-time.Hour)

	currency, err := SettingCurrency{ID: 3, CreatedAt: created}.Apply(" EUR ", " 1.08 ", now)
	if err != nil || currency.CurrencyName != "EUR" || currency.RateToBase != 1.08 || !currency.CreatedAt.Equal(created) || !currency.LastUpdatedAt.Equal(now) {
		t.Fatalf("edit should keep CreatedAt: %+v %v", currency, err)
	}

	method, err := SettingPaymentMethod{}.Apply("Visa", "Card", false, true, now)
	if err != nil || !method.IsDefault || !method.CreatedAt.Equal(now) {
		t.Fatalf("first payment method should become default: %+v %v", method, err)
	}

	method, err = SettingPaymentMethod{ID: 1}.Apply("Visa", "Card", false, true, now)
	if err != nil || method.IsDefault {
		t.Fatalf("editing never forces default: %+v %v", method, err)
	}

	taxType, err := SettingTaxType{}.Apply(" NL ", " VAT ", " sales ", " https://x ", now)
	if err != nil || taxType.Country != "NL" || taxType.TaxTypeName != "VAT" || taxType.Description != "sales" || taxType.URL != "https://x" {
		t.Fatalf("unexpected tax type %+v %v", taxType, err)
	}
}
