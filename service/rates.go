package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lazybark/cents/flows/currency"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/rates"
)

// ratesCheckEvery is how often KeepRatesFresh checks whether rates are
// due; they're fetched when the last fetch is a day old.
const ratesCheckEvery = time.Hour

type CatalogCurrency struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Symbol string `json:"symbol"`
}

// RatesStatus says how rates are kept up to date. Unlinked lists the
// currencies whose rates aren't fetched (their names aren't linked to a
// known currency); LastError is the last fetch's error since the app
// started, if it failed.
type RatesStatus struct {
	Auto      bool     `json:"auto"`
	BaseCode  string   `json:"baseCode"`
	UpdatedAt string   `json:"updatedAt"`
	Source    string   `json:"source"`
	Date      string   `json:"date"`
	Linked    int      `json:"linked"`
	Unlinked  []string `json:"unlinked"`
	LastError string   `json:"lastError"`
}

type CurrencySetupInput struct {
	Codes []string `json:"codes"`
	Base  string   `json:"base"`
}

// CurrencyCatalog lists every currency that can be picked, by code.
func (s *Service) CurrencyCatalog() ([]CatalogCurrency, error) {
	all := currency.All()
	result := make([]CatalogCurrency, 0, len(all))
	for _, c := range all {
		result = append(result, CatalogCurrency{Code: c.Code, Name: c.Name, Symbol: c.Symbol})
	}

	return result, nil
}

func (s *Service) RatesStatus() (RatesStatus, error) {
	_, stts, err := s.storageAndSettings()
	if err != nil {
		return RatesStatus{}, err
	}

	return s.ratesStatus(stts), nil
}

// RefreshRates fetches rates now, whether or not they're due.
func (s *Service) RefreshRates() (RatesStatus, error) {
	stts, err := s.refreshRates(context.Background())
	if err != nil {
		return s.ratesStatus(stts), err
	}

	return s.ratesStatus(stts), nil
}

// SetRatesAuto turns daily rate updates on or off; turning them on fetches
// rates if they're due.
func (s *Service) SetRatesAuto(on bool) (RatesStatus, error) {
	storage, err := s.currentStorage()
	if err != nil {
		return RatesStatus{}, err
	}

	value := "0"
	if on {
		value = "1"
	}

	if err := storage.SaveSettingRecords([]settings.SettingRecord{{SettingID: settings.RatesAutoID, SettingValue: value}}); err != nil {
		return RatesStatus{}, err
	}

	if on {
		s.refreshIfDue(context.Background())
	}

	return s.RatesStatus()
}

// SetupCurrencies sets up a new database's currencies: the picked ones,
// with base as the base currency, named by their codes, and rates fetched
// now (if that works) and daily from then on.
func (s *Service) SetupCurrencies(input CurrencySetupInput) (RatesStatus, error) {
	storage, stts, err := s.storageAndSettings()
	if err != nil {
		return RatesStatus{}, err
	}

	codes := make([]string, 0, len(input.Codes))
	seen := map[string]bool{}
	for _, raw := range input.Codes {
		c, ok := currency.Find(raw)
		if !ok {
			return RatesStatus{}, fmt.Errorf("unknown currency %q", raw)
		}

		if !seen[c.Code] {
			seen[c.Code] = true
			codes = append(codes, c.Code)
		}
	}

	base := strings.ToUpper(strings.TrimSpace(input.Base))
	if len(codes) == 0 {
		return RatesStatus{}, errors.New("pick at least one currency")
	}

	if !seen[base] {
		return RatesStatus{}, errors.New("pick which of your currencies is the base one")
	}

	existing := map[string]bool{}
	for _, c := range stts.Currencies {
		existing[strings.ToUpper(strings.TrimSpace(c.CurrencyName))] = true
		existing[c.LinkedCode()] = true
	}

	now := time.Now()
	stts.BaseCurrency = base
	stts.BaseCurrencyCode = base
	stts.Currencies = nil
	for _, code := range codes {
		if code != base && !existing[code] {
			stts.Currencies = append(stts.Currencies, settings.SettingCurrency{CreatedAt: now, LastUpdatedAt: now, CurrencyName: code, Code: code})
		}
	}

	records := []settings.SettingRecord{
		{SettingID: settings.BaseCurrencySettingID, SettingValue: base},
		{SettingID: settings.BaseCurrencyCodeID, SettingValue: base},
		{SettingID: settings.BaseCurrencyCodeForID, SettingValue: base},
		{SettingID: settings.RatesAutoID, SettingValue: "1"},
		{SettingID: settings.CurrenciesSetUpID, SettingValue: "1"},
	}

	// Rates are fetched now if that works; otherwise the daily check tries
	// again, and the currencies show "no rate" until then.
	currencies := stts.Currencies
	if quote, err := s.fetch(context.Background(), base); err == nil {
		updated, rateRecords, _ := rates.Apply(stts, quote, now)
		currencies = updated
		records = append(records, rateRecords...)
		for _, c := range stts.Currencies {
			if !containsCurrency(updated, c.CurrencyName) {
				currencies = append(currencies, c)
			}
		}
	} else {
		s.setRatesError(err)
	}

	if err := storage.SaveRates(currencies, records); err != nil {
		return RatesStatus{}, err
	}

	return s.RatesStatus()
}

// SkipCurrencySetup leaves a new database's currencies as they are (the
// default "$"); they can be set up in settings later.
func (s *Service) SkipCurrencySetup() error {
	storage, err := s.currentStorage()
	if err != nil {
		return err
	}

	return storage.SaveSettingRecords([]settings.SettingRecord{{SettingID: settings.CurrenciesSetUpID, SettingValue: "1"}})
}

func containsCurrency(items []settings.SettingCurrency, name string) bool {
	for _, item := range items {
		if item.CurrencyName == name {
			return true
		}
	}

	return false
}

func (s *Service) ratesStatus(stts settings.AppSettings) RatesStatus {
	status := RatesStatus{
		Auto:     stts.Rates.Auto,
		BaseCode: stts.BaseCurrencyCode,
		Source:   stts.Rates.Source,
		Date:     stts.Rates.Date,
		Unlinked: []string{},
	}

	if !stts.Rates.UpdatedAt.IsZero() {
		status.UpdatedAt = stts.Rates.UpdatedAt.Format(time.RFC3339)
	}

	for _, c := range stts.Currencies {
		if stts.Linkable(c) {
			status.Linked++
		} else {
			status.Unlinked = append(status.Unlinked, c.CurrencyName)
		}
	}

	s.mu.Lock()
	status.LastError = s.ratesError
	s.mu.Unlock()

	return status
}

func (s *Service) setRatesError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ratesError = ""
	if err != nil {
		s.ratesError = err.Error()
	}
}

func (s *Service) fetch(ctx context.Context, base string) (rates.Quote, error) {
	quote, err := s.rates.Fetch(ctx, base)
	if err != nil {
		return rates.Quote{}, upstreamError{err}
	}

	return quote, nil
}

// refreshRates fetches rates for the base currency and saves them on every
// linked currency. Only one refresh runs at a time.
func (s *Service) refreshRates(ctx context.Context) (settings.AppSettings, error) {
	s.refreshing.Lock()
	defer s.refreshing.Unlock()

	storage, stts, err := s.storageAndSettings()
	if err != nil {
		return settings.AppSettings{}, err
	}

	if stts.BaseCurrencyCode == "" {
		err := fmt.Errorf("the base currency %q isn't linked to a known currency: link it in settings", stts.BaseCurrencyLabel())
		s.setRatesError(err)

		return stts, err
	}

	quote, err := s.fetch(ctx, stts.BaseCurrencyCode)
	if err != nil {
		s.setRatesError(err)

		return stts, err
	}

	// The fetch takes a while: the rates go onto the settings as they are
	// now, so a change made meanwhile isn't undone.
	if _, stts, err = s.storageAndSettings(); err != nil {
		return stts, err
	}

	if !strings.EqualFold(quote.Base, stts.BaseCurrencyCode) {
		err := errors.New("the base currency changed while rates were fetched; refresh them again")
		s.setRatesError(err)

		return stts, err
	}

	updated, records, _ := rates.Apply(stts, quote, time.Now())
	if err := storage.SaveRates(updated, records); err != nil {
		s.setRatesError(err)

		return stts, err
	}

	s.setRatesError(nil)

	_, stts, err = s.storageAndSettings()

	return stts, err
}

// refreshIfDue fetches rates when automatic updates are on and the last
// fetch is a day old. It reports whether they changed.
func (s *Service) refreshIfDue(ctx context.Context) bool {
	_, stts, err := s.storageAndSettings()
	if err != nil || !stts.RatesDue(time.Now()) {
		return false
	}

	_, err = s.refreshRates(ctx)

	return err == nil
}

// KeepRatesFresh fetches rates when they're due: now, then every hour
// until ctx is done. changed (if not nil) is called after each update.
func (s *Service) KeepRatesFresh(ctx context.Context, changed func()) {
	check := func() {
		if s.refreshIfDue(ctx) && changed != nil {
			changed()
		}
	}

	check()

	ticker := time.NewTicker(ratesCheckEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check()
		}
	}
}
