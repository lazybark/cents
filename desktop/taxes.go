package desktop

import (
	"errors"
	"fmt"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/tax"
)

var errTaxNotFound = errors.New("tax not found")

type TaxRow struct {
	ID          uint    `json:"id"`
	Country     string  `json:"country"`
	TypeName    string  `json:"typeName"`
	DueCents    int64   `json:"dueCents"`
	PaidCents   int64   `json:"paidCents"`
	LeftCents   int64   `json:"leftCents"`
	PaidPercent float64 `json:"paidPercent"`
	Period      string  `json:"period"`
	DueDate     string  `json:"dueDate"`
	Overdue     bool    `json:"overdue"`
	Comment     string  `json:"comment"`
}

type TaxTypeOption struct {
	ID    uint   `json:"id"`
	Label string `json:"label"`
}

type TaxesView struct {
	BaseCurrency string          `json:"baseCurrency"`
	Mode         string          `json:"mode"`
	Taxes        []TaxRow        `json:"taxes"`
	Progress     Progress        `json:"progress"`
	Counts       map[string]int  `json:"counts"`
	TaxTypes     []TaxTypeOption `json:"taxTypes"`
}

type NewTaxInput struct {
	TaxTypeID  uint   `json:"taxTypeId"`
	AmountDue  string `json:"amountDue"`
	AmountPaid string `json:"amountPaid"`
	Period     string `json:"period"`
	DueDate    string `json:"dueDate"`
	Comment    string `json:"comment"`
}

type TaxUpdateInput struct {
	ID         uint   `json:"id"`
	AmountDue  string `json:"amountDue"`
	AmountPaid string `json:"amountPaid"`
	Period     string `json:"period"`
	DueDate    string `json:"dueDate"`
	Comment    string `json:"comment"`
}

// Taxes lists "unpaid" or "paid" taxes. Tax amounts are always in the base
// currency; Counts has both lists' sizes for the tabs.
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
		TaxTypes: make([]TaxTypeOption, 0, len(stts.TaxTypes)),
	}

	for _, t := range stts.TaxTypes {
		view.TaxTypes = append(view.TaxTypes, TaxTypeOption{ID: t.ID, Label: taxTypeLabel(t)})
	}

	for _, item := range listed {
		view.Taxes = append(view.Taxes, taxRow(item, now))
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

	entry, err := tax.New(taxType, input.AmountDue, input.AmountPaid, input.Period, input.DueDate, input.Comment, dates.ISO, time.Now())
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

// UpdateTax changes amounts, period, due date and comment; the tax type
// can't change, as in the TUI.
func (a *API) UpdateTax(input TaxUpdateInput) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	current, err := findTax(storage, input.ID)
	if err != nil {
		return err
	}

	updated, err := current.Edit(input.AmountDue, input.AmountPaid, input.Period, input.DueDate, input.Comment, dates.ISO, time.Now())
	if err != nil {
		return err
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
		result = append(result, PaymentLog{When: entry.CreatedAt.Local().Format("2006-01-02 15:04"), DeltaCents: entry.DeltaPaidCents, Note: entry.Note})
	}

	return result, nil
}

// AddTaxPayment logs a payment and moves the amount paid by it, returning
// the tax as it is now.
func (a *API) AddTaxPayment(input PaymentInput) (TaxRow, error) {
	storage, err := a.currentStorage()
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

	return taxRow(updated, now), nil
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

func taxRow(item tax.Tax, now time.Time) TaxRow {
	return TaxRow{
		ID:          item.ID,
		Country:     item.TaxCountry,
		TypeName:    item.TaxTypeName,
		DueCents:    item.AmountDueCents,
		PaidCents:   item.AmountPaidCents,
		LeftCents:   item.LeftCents(),
		PaidPercent: item.PaidPercent(),
		Period:      item.Period,
		DueDate:     formatOptionalDay(item.DueDate),
		Overdue:     overdue(item.DueDate, item.IsPaid(), now),
		Comment:     item.Comment,
	}
}
