package desktop

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lazybark/cents/flows/currency"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/rates"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ratesEvent tells the frontend that rates changed in the background.
const ratesEvent = "rates-updated"

// ratesCheckEvery is how often the app checks whether rates are due while
// it runs; they're fetched when the last fetch is a day old.
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
func (a *API) CurrencyCatalog() []CatalogCurrency {
	all := currency.All()
	result := make([]CatalogCurrency, 0, len(all))
	for _, c := range all {
		result = append(result, CatalogCurrency{Code: c.Code, Name: c.Name, Symbol: c.Symbol})
	}

	return result
}

func (a *API) RatesStatus() (RatesStatus, error) {
	_, stts, err := a.storageAndSettings()
	if err != nil {
		return RatesStatus{}, err
	}

	return a.ratesStatus(stts), nil
}

// RefreshRates fetches rates now, whether or not they're due.
func (a *API) RefreshRates() (RatesStatus, error) {
	stts, err := a.refreshRates(a.context())
	if err != nil {
		return a.ratesStatus(stts), err
	}

	return a.ratesStatus(stts), nil
}

// SetRatesAuto turns daily rate updates on or off; turning them on fetches
// rates if they're due.
func (a *API) SetRatesAuto(on bool) (RatesStatus, error) {
	storage, err := a.currentStorage()
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
		a.refreshIfDue(a.context())
	}

	return a.RatesStatus()
}

// SetupCurrencies sets up a new database's currencies: the picked ones,
// with base as the base currency, named by their codes, and rates fetched
// now (if that works) and daily from then on.
func (a *API) SetupCurrencies(input CurrencySetupInput) (RatesStatus, error) {
	storage, stts, err := a.storageAndSettings()
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
	if quote, err := a.fetch(a.context(), base); err == nil {
		updated, rateRecords, _ := rates.Apply(stts, quote, now)
		currencies = updated
		records = append(records, rateRecords...)
		for _, c := range stts.Currencies {
			if !containsCurrency(updated, c.CurrencyName) {
				currencies = append(currencies, c)
			}
		}
	} else {
		a.setRatesError(err)
	}

	if err := storage.SaveRates(currencies, records); err != nil {
		return RatesStatus{}, err
	}

	return a.RatesStatus()
}

// SkipCurrencySetup leaves a new database's currencies as they are (the
// default "$"); they can be set up in settings later.
func (a *API) SkipCurrencySetup() error {
	storage, err := a.currentStorage()
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

func (a *API) ratesStatus(stts settings.AppSettings) RatesStatus {
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

	a.mu.Lock()
	status.LastError = a.ratesError
	a.mu.Unlock()

	return status
}

func (a *API) setRatesError(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.ratesError = ""
	if err != nil {
		a.ratesError = err.Error()
	}
}

func (a *API) fetch(ctx context.Context, base string) (rates.Quote, error) {
	return a.rates.Fetch(ctx, base)
}

// refreshRates fetches rates for the base currency and saves them on every
// linked currency. Only one refresh runs at a time.
func (a *API) refreshRates(ctx context.Context) (settings.AppSettings, error) {
	a.refreshing.Lock()
	defer a.refreshing.Unlock()

	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return settings.AppSettings{}, err
	}

	if stts.BaseCurrencyCode == "" {
		err := fmt.Errorf("the base currency %q isn't linked to a known currency: link it in settings", stts.BaseCurrencyLabel())
		a.setRatesError(err)

		return stts, err
	}

	quote, err := a.fetch(ctx, stts.BaseCurrencyCode)
	if err != nil {
		a.setRatesError(err)

		return stts, err
	}

	updated, records, _ := rates.Apply(stts, quote, time.Now())
	if err := storage.SaveRates(updated, records); err != nil {
		a.setRatesError(err)

		return stts, err
	}

	a.setRatesError(nil)

	_, stts, err = a.storageAndSettings()

	return stts, err
}

// refreshIfDue fetches rates when automatic updates are on and the last
// fetch is a day old, telling the frontend when they changed.
func (a *API) refreshIfDue(ctx context.Context) {
	_, stts, err := a.storageAndSettings()
	if err != nil || !stts.RatesDue(time.Now()) {
		return
	}

	if _, err := a.refreshRates(ctx); err == nil && a.ctx != nil {
		runtime.EventsEmit(a.ctx, ratesEvent)
	}
}

// keepRatesFresh checks on launch, then every hour while the app runs.
func (a *API) keepRatesFresh(ctx context.Context) {
	a.refreshIfDue(ctx)

	ticker := time.NewTicker(ratesCheckEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.refreshIfDue(ctx)
		}
	}
}

func (a *API) context() context.Context {
	if a.ctx != nil {
		return a.ctx
	}

	return context.Background()
}
