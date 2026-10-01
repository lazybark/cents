package desktop

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/tax"
	"github.com/lazybark/cents/money"
)

var errTaxNotFound = errors.New("tax not found")

// TaxRow has amounts in the tax's currency and, as Base*, in the base
// currency at the rate recorded with the tax.
type TaxRow struct {
	ID            uint    `json:"id"`
	Country       string  `json:"country"`
	TypeName      string  `json:"typeName"`
	Currency      string  `json:"currency"`
	IsBase        bool    `json:"isBase"`
	RateToBase    float64 `json:"rateToBase"`
	DueCents      int64   `json:"dueCents"`
	PaidCents     int64   `json:"paidCents"`
	LeftCents     int64   `json:"leftCents"`
	BaseDueCents  int64   `json:"baseDueCents"`
	BasePaidCents int64   `json:"basePaidCents"`
	BaseLeftCents int64   `json:"baseLeftCents"`
	PaidPercent   float64 `json:"paidPercent"`
	Period        string  `json:"period"`
	DueDate       string  `json:"dueDate"`
	Overdue       bool    `json:"overdue"`
	Comment       string  `json:"comment"`
}

type TaxTypeOption struct {
	ID    uint   `json:"id"`
	Label string `json:"label"`
}

// TaxCurrency is a currency a new tax can use, with its rate to the base
// currency in settings now, to start the tax's own rate with.
type TaxCurrency struct {
	Name string  `json:"name"`
	Rate float64 `json:"rate"`
}

type TaxesView struct {
	BaseCurrency string          `json:"baseCurrency"`
	Mode         string          `json:"mode"`
	Taxes        []TaxRow        `json:"taxes"`
	Progress     Progress        `json:"progress"`
	Counts       map[string]int  `json:"counts"`
	TaxTypes     []TaxTypeOption `json:"taxTypes"`
	Currencies   []TaxCurrency   `json:"currencies"`
}

// NewTaxInput is a new tax. Rate is the currency's rate to the base
// currency, ignored for the base currency itself.
type NewTaxInput struct {
	TaxTypeID  uint   `json:"taxTypeId"`
	Currency   string `json:"currency"`
	Rate       string `json:"rate"`
	AmountDue  string `json:"amountDue"`
	AmountPaid string `json:"amountPaid"`
	Period     string `json:"period"`
	DueDate    string `json:"dueDate"`
	Comment    string `json:"comment"`
}

// TaxUpdateInput changes a tax. Rate replaces the recorded rate of a tax in
// another currency; empty keeps it.
type TaxUpdateInput struct {
	ID         uint   `json:"id"`
	Rate       string `json:"rate"`
	AmountDue  string `json:"amountDue"`
	AmountPaid string `json:"amountPaid"`
	Period     string `json:"period"`
	DueDate    string `json:"dueDate"`
	Comment    string `json:"comment"`
}

// Taxes lists "unpaid" or "paid" taxes. Progress is in the base currency at
// each tax's recorded rate; Counts has both lists' sizes for the tabs.
func (a *API) Taxes(mode string) (TaxesView, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return TaxesView{}, err
	}

	items, err := storage.LoadTaxes()
	if err != nil {
		return TaxesView{}, fmt.Errorf("failed to load taxes: %w", err)
	}

	listMode := tax.ListMode(mode)
	if listMode != tax.ListPaid {
		listMode = tax.ListUnpaid
	}

	listed := tax.Filter(items, listMode)
	paid, total := tax.ProgressTotals(listed)
	now := time.Now()

	view := TaxesView{
		BaseCurrency: stts.BaseCurrencyLabel(),
		Mode:         string(listMode),
		Taxes:        make([]TaxRow, 0, len(listed)),
		Progress:     Progress{PaidCents: paid, TotalCents: total},
		Counts: map[string]int{
			string(tax.ListUnpaid): len(tax.Filter(items, tax.ListUnpaid)),
			string(tax.ListPaid):   len(tax.Filter(items, tax.ListPaid)),
		},
		TaxTypes:   make([]TaxTypeOption, 0, len(stts.TaxTypes)),
		Currencies: taxCurrencies(stts),
	}

	for _, t := range stts.TaxTypes {
		view.TaxTypes = append(view.TaxTypes, TaxTypeOption{ID: t.ID, Label: taxTypeLabel(t)})
	}

	for _, item := range listed {
		view.Taxes = append(view.Taxes, taxRow(item, stts, now))
	}

	return view, nil
}

func (a *API) CreateTax(input NewTaxInput) (CreatedIn, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return CreatedIn{}, err
	}

	if len(stts.TaxTypes) == 0 {
		return CreatedIn{}, errors.New("no tax types configured; add one in settings")
	}

	var taxType settings.SettingTaxType
	for _, t := range stts.TaxTypes {
		if t.ID == input.TaxTypeID {
			taxType = t
		}
	}

	if taxType.ID == 0 {
		return CreatedIn{}, errors.New("unknown tax type")
	}

	currency, rate, err := taxCurrencyAndRate(stts, input.Currency, input.Rate)
	if err != nil {
		return CreatedIn{}, err
	}

	entry, err := tax.New(taxType, currency, rate, input.AmountDue, input.AmountPaid, input.Period, input.DueDate, input.Comment, dates.ISO, time.Now())
	if err != nil {
		return CreatedIn{}, err
	}

	if err := storage.CreateTax(&entry); err != nil {
		return CreatedIn{}, fmt.Errorf("save failed: %w", err)
	}

	mode := tax.ListUnpaid
	if entry.IsPaid() {
		mode = tax.ListPaid
	}

	return CreatedIn{Mode: string(mode)}, nil
}

// UpdateTax changes amounts, period, due date and comment, plus the rate of
// a tax in another currency; the tax type and currency can't change.
func (a *API) UpdateTax(input TaxUpdateInput) error {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return err
	}

	current, err := findTax(storage, input.ID)
	if err != nil {
		return err
	}

	now := time.Now()
	updated, err := current.Edit(input.AmountDue, input.AmountPaid, input.Period, input.DueDate, input.Comment, dates.ISO, now)
	if err != nil {
		return err
	}

	if !isBaseCurrency(stts, current.Currency) && strings.TrimSpace(input.Rate) != "" {
		rate, err := money.ParseRate(input.Rate)
		if err != nil {
			return err
		}

		if updated, err = updated.WithRate(rate, now); err != nil {
			return err
		}
	}

	if err := storage.SaveTax(&updated); err != nil {
		return fmt.Errorf("save failed: %w", err)
	}

	return nil
}

// TaxLogs returns a tax's latest payments, newest first.
func (a *API) TaxLogs(id uint) ([]PaymentLog, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return nil, err
	}

	logs, err := storage.LoadTaxLogs(id)
	if err != nil {
		return nil, err
	}

	result := make([]PaymentLog, 0, len(logs))
	for _, entry := range logs {
		result = append(result, PaymentLog{ID: entry.ID, When: entry.CreatedAt.Local().Format("2006-01-02 15:04"), DeltaCents: entry.DeltaPaidCents, Note: entry.Note})
	}

	return result, nil
}

// AddTaxPayment logs a payment and moves the amount paid by it, returning
// the tax as it is now.
func (a *API) AddTaxPayment(input PaymentInput) (TaxRow, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return TaxRow{}, err
	}

	current, err := findTax(storage, input.ID)
	if err != nil {
		return TaxRow{}, err
	}

	now := time.Now()
	updated, entry, err := current.ApplyPayment(input.Delta, input.Date, input.Note, dates.ISO, now)
	if err != nil {
		return TaxRow{}, err
	}

	if err := storage.SaveTax(&updated); err != nil {
		return TaxRow{}, fmt.Errorf("tax update failed: %w", err)
	}

	if err := storage.CreateTaxLog(&entry); err != nil {
		return TaxRow{}, fmt.Errorf("log save failed: %w", err)
	}

	return taxRow(updated, stts, now), nil
}

func (a *API) DeleteTax(id uint) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	if _, err := findTax(storage, id); err != nil {
		return err
	}

	if err := storage.DeleteTax(id); err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	return nil
}

func findTax(storage StorageWorker, id uint) (tax.Tax, error) {
	items, err := storage.LoadTaxes()
	if err != nil {
		return tax.Tax{}, fmt.Errorf("failed to load taxes: %w", err)
	}

	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}

	return tax.Tax{}, errTaxNotFound
}

// taxCurrencies lists the base currency (rate 1) and every configured
// currency with its current rate.
func taxCurrencies(stts settings.AppSettings) []TaxCurrency {
	options := stts.CurrencyOptions()
	result := make([]TaxCurrency, 0, len(options))
	for i, name := range options {
		rate := 1.0
		if i > 0 {
			rate, _ = stts.RateToBase(name)
		}

		result = append(result, TaxCurrency{Name: name, Rate: rate})
	}

	return result
}

func isBaseCurrency(stts settings.AppSettings, currency string) bool {
	return strings.EqualFold(strings.TrimSpace(currency), stts.BaseCurrencyLabel())
}

// taxCurrencyAndRate checks a new tax's currency is a configured one (empty
// means the base currency) and picks its rate: 1 for the base currency,
// otherwise the rate typed (or, left empty, the one in settings now).
func taxCurrencyAndRate(stts settings.AppSettings, rawCurrency, rawRate string) (string, float64, error) {
	if strings.TrimSpace(rawCurrency) == "" {
		return stts.BaseCurrencyLabel(), 1, nil
	}

	currency, ok := matchOption(stts.CurrencyOptions(), strings.TrimSpace(rawCurrency))
	if !ok {
		return "", 0, fmt.Errorf("unknown currency %q: add it in settings first", rawCurrency)
	}

	if isBaseCurrency(stts, currency) {
		return currency, 1, nil
	}

	if strings.TrimSpace(rawRate) == "" {
		rate, ok := stts.RateToBase(currency)
		if !ok {
			return "", 0, fmt.Errorf("%s has no rate to %s: type one", currency, stts.BaseCurrencyLabel())
		}

		return currency, rate, nil
	}

	rate, err := money.ParseRate(rawRate)
	if err != nil {
		return "", 0, err
	}

	return currency, rate, nil
}

func taxRow(item tax.Tax, stts settings.AppSettings, now time.Time) TaxRow {
	return TaxRow{
		ID:            item.ID,
		Country:       item.TaxCountry,
		TypeName:      item.TaxTypeName,
		Currency:      item.Currency,
		IsBase:        isBaseCurrency(stts, item.Currency),
		RateToBase:    item.RateToBase,
		DueCents:      item.AmountDueCents,
		PaidCents:     item.AmountPaidCents,
		LeftCents:     item.LeftCents(),
		BaseDueCents:  item.AmountDueBaseCents,
		BasePaidCents: item.AmountPaidBaseCents,
		BaseLeftCents: item.LeftBaseCents(),
		PaidPercent:   item.PaidPercent(),
		Period:        item.Period,
		DueDate:       formatOptionalDay(item.DueDate),
		Overdue:       overdue(item.DueDate, item.IsPaid(), now),
		Comment:       item.Comment,
	}
}
