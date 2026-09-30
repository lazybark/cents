package sqlite

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/lazybark/cents/flows/settings"
)

func TestCurrencySetupAndLinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cents.db")
	db, _, err := OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}

	stts, _ := LoadAppSettings(db)
	if stts.CurrenciesSetUp || stts.BaseCurrencyCode != "" || stts.Rates.Auto {
		t.Fatalf("a new database isn't set up: %+v", stts)
	}

	store := NewSQLiteStorage(db)
	if err := store.SaveSettingRecords([]settings.SettingRecord{
		{SettingID: settings.BaseCurrencyCodeID, SettingValue: "USD"},
		{SettingID: settings.BaseCurrencyCodeForID, SettingValue: "$"},
		{SettingID: settings.RatesAutoID, SettingValue: "1"},
		{SettingID: settings.RatesUpdatedAtID, SettingValue: "2026-10-02T09:00:00Z"},
	}); err != nil {
		t.Fatal(err)
	}

	stts, _ = LoadAppSettings(db)
	if stts.BaseCurrencyCode != "USD" || !stts.Rates.Auto || !stts.Rates.UpdatedAt.Equal(time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("links not loaded: %+v", stts)
	}

	// An older database with records counts as set up on open.
	other := filepath.Join(t.TempDir(), "old.db")
	old, _, _ := OpenDatabase(other)
	if err := old.Create(&settings.SettingCurrency{CurrencyName: "₾", RateToBase: 0.3}).Error; err != nil {
		t.Fatal(err)
	}

	if old, _, err = OpenDatabase(other); err != nil {
		t.Fatal(err)
	}

	stts, _ = LoadAppSettings(old)
	if !stts.CurrenciesSetUp || stts.Currencies[0].LinkedCode() != "GEL" {
		t.Fatalf("an existing database should count as set up: %+v", stts)
	}
}
