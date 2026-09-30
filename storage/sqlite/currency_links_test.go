package sqlite

import (
	"errors"
	"testing"
	"time"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/settings"
)

func saveBase(t *testing.T, s *SQLiteStorage, label, code string) {
	t.Helper()
	if err := s.SaveSettingRecords([]settings.SettingRecord{
		{SettingID: settings.BaseCurrencySettingID, SettingValue: label},
		{SettingID: settings.BaseCurrencyCodeID, SettingValue: code},
		{SettingID: settings.BaseCurrencyCodeForID, SettingValue: label},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCurrenciesAreLinked(t *testing.T) {
	s, _ := openTest(t)

	stts, _ := s.LoadAppSettings()
	if stts.BaseCurrencyUID == "" || len(stts.Currencies) != 0 || stts.BaseCurrency != "$" {
		t.Fatalf("expected a base row kept apart: %+v", stts)
	}

	eur := settings.SettingCurrency{CurrencyName: "EUR", RateToBase: 1.1}
	if err := s.SaveSettingCurrency(&eur); err != nil {
		t.Fatal(err)
	}

	for _, a := range []account.Account{{Name: "Dollars", Currency: " $ "}, {Name: "Euros", Currency: "eur"}} {
		if err := s.CreateAccount(&a); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.CreateAccount(&account.Account{Name: "Nope", Currency: "XYZ"}); err == nil {
		t.Fatal("expected an unknown currency to fail")
	}

	// Rename EUR: records show the new name, nothing else saved.
	renamed := settings.SettingCurrency{ID: eur.ID, CurrencyName: "€uro", RateToBase: 1.1}
	if err := s.SaveSettingCurrency(&renamed); err != nil {
		t.Fatal(err)
	}

	accounts, _ := s.LoadAccounts()
	got := map[string]string{}
	for _, a := range accounts {
		got[a.Name] = a.Currency
	}
	if got["Dollars"] != "$" || got["Euros"] != "€uro" {
		t.Fatalf("unexpected currencies %v", got)
	}

	// Relabel the base: the same currency, so its records follow.
	saveBase(t, s, "USD", "USD")
	accounts, _ = s.LoadAccounts()
	for _, a := range accounts {
		if a.Name == "Dollars" && a.Currency != "USD" {
			t.Fatalf("expected the relabelled base: %+v", a)
		}
	}
	if stts, _ = s.LoadAppSettings(); stts.BaseCurrency != "USD" || len(stts.Currencies) != 1 {
		t.Fatalf("unexpected settings %+v", stts)
	}

	// Switch to another currency: the dollar account stays in dollars,
	// now an ordinary currency without a rate.
	saveBase(t, s, "GBP", "GBP")
	stts, _ = s.LoadAppSettings()
	if stts.BaseCurrency != "GBP" || len(stts.Currencies) != 2 {
		t.Fatalf("expected USD kept as a currency: %+v", stts.Currencies)
	}
	for _, c := range stts.Currencies {
		if c.CurrencyName == "USD" && (c.IsBase || c.RateToBase != 0) {
			t.Fatalf("expected USD without a rate: %+v", c)
		}
	}
	accounts, _ = s.LoadAccounts()
	for _, a := range accounts {
		if a.Name == "Dollars" && a.Currency != "USD" {
			t.Fatalf("a switch keeps records in their currency: %+v", a)
		}
	}

	// Used currencies can't be deleted; the base not at all.
	var inUse InUseError
	if err := s.DeleteSetting("currency", eur.ID); !errors.As(err, &inUse) {
		t.Fatalf("expected EUR in use: %v", err)
	}
	if err := s.DeleteSetting("currency", stts.BaseCurrencyID); !errors.Is(err, errBaseCurrency) {
		t.Fatalf("expected the base refused: %v", err)
	}
	if _, err := s.Merge("currency", stts.BaseCurrencyID, eur.ID); !errors.Is(err, errBaseCurrency) {
		t.Fatalf("expected the base refused: %v", err)
	}

	// Merging moves records; rates of the merged one go with it.
	if err := recordRates(s.db, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	usd := currencyByName(stts, "USD")
	if moved, err := s.Merge("currency", usd.ID, eur.ID); err != nil || moved != 1 {
		t.Fatalf("unexpected merge %d %v", moved, err)
	}
	records, _ := s.LoadRateRecords()
	for _, r := range records {
		if r.Currency == "USD" || r.Currency == "" || r.Base != "GBP" {
			t.Fatalf("unexpected rate record %+v", r)
		}
	}
}

func currencyByName(stts settings.AppSettings, name string) settings.SettingCurrency {
	for _, c := range stts.Currencies {
		if c.CurrencyName == name {
			return c
		}
	}

	return settings.SettingCurrency{}
}

// A database from before currency links: records name their currency, the
// rate history too, and one names a currency settings don't have.
func TestOldCurrenciesAreLinked(t *testing.T) {
	s, path := openTest(t)
	eur := settings.SettingCurrency{CurrencyName: "EUR", RateToBase: 1.1}
	if err := s.SaveSettingCurrency(&eur); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateCashflow(&cashflow.CashflowEntry{Currency: "EUR", AmountCents: 1, EntryDate: time.Now()}); err != nil {
		t.Fatal(err)
	}

	db := s.db
	asOldDatabase(t, db)
	for _, stmt := range []string{
		"DROP TABLE rate_records",
		"CREATE TABLE rate_records (id integer PRIMARY KEY AUTOINCREMENT, created_at datetime, updated_at datetime, day datetime NOT NULL, currency text NOT NULL, base text NOT NULL, rate_to_base real NOT NULL)",
		"CREATE UNIQUE INDEX idx_rate_record ON rate_records(day, currency, base)",
		"INSERT INTO rate_records (day, currency, base, rate_to_base) VALUES ('2026-01-01 00:00:00+00:00', 'EUR', '$', 1.05), ('2026-01-01 00:00:00+00:00', 'GONE', '$', 2), ('2026-01-02 00:00:00+00:00', 'EUR', '$', 1.07)",
		"INSERT INTO cashflow_entries (is_income, currency, amount_cents, category_uid, account_uid) VALUES (0, ' eur ', 2, '', ''), (0, 'KZT', 3, '', ''), (0, '$', 4, '', '')",
		"DELETE FROM setting_currencies WHERE is_base = 1",
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	db, _, err := OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	s = NewSQLiteStorage(db)

	entries, _ := s.LoadCashflows()
	got := map[int64]string{}
	for _, e := range entries {
		got[e.AmountCents] = e.Currency
	}
	if got[1] != "EUR" || got[2] != "EUR" || got[3] != "KZT" || got[4] != "$" {
		t.Fatalf("unexpected currencies %v", got)
	}

	stts, _ := s.LoadAppSettings()
	if kzt := currencyByName(stts, "KZT"); kzt.UID == "" || kzt.RateToBase != 0 {
		t.Fatalf("expected KZT added without a rate: %+v", stts.Currencies)
	}
	if stts.BaseCurrencyUID == "" {
		t.Fatal("expected a base row")
	}

	records, _ := s.LoadRateRecords()
	kept := 0
	for _, r := range records {
		if r.Currency == "EUR" && r.Base == "$" && !r.Day.After(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)) {
			kept++
		}
		if r.Currency == "GONE" {
			t.Fatal("rates of an unknown currency are left out")
		}
	}
	if kept != 2 {
		t.Fatalf("expected both EUR rates kept: %+v", records)
	}

	if needsLinking(db) {
		t.Fatal("expected the currency columns gone")
	}
}
