package desktop

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/credit"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/money"
)

var errCreditNotFound = errors.New("credit not found")

// CreditRow has amounts in the credit's currency and, as Base*, in the base
// currency at the rate recorded with it (HasRate is false when it has none).
type CreditRow struct {
	ID              uint    `json:"id"`
	Name            string  `json:"name"`
	Purpose         string  `json:"purpose"`
	Issuer          string  `json:"issuer"`
	Currency        string  `json:"currency"`
	IsBase          bool    `json:"isBase"`
	RateToBase      float64 `json:"rateToBase"`
	HasRate         bool    `json:"hasRate"`
	TotalCents      int64   `json:"totalCents"`
	PaidCents       int64   `json:"paidCents"`
	LeftCents       int64   `json:"leftCents"`
	BaseTotalCents  int64   `json:"baseTotalCents"`
	BasePaidCents   int64   `json:"basePaidCents"`
	BaseLeftCents   int64   `json:"baseLeftCents"`
	InterestPercent float64 `json:"interestPercent"`
	StartDate       string  `json:"startDate"`
	DueDate         string  `json:"dueDate"`
	Overdue         bool    `json:"overdue"`
	Comment         string  `json:"comment"`
}

// CreditsView is one credit list. Progress is in the base currency at
// recorded rates; LeftCents is what is left on every credit, whichever list
// is shown, as counted in net worth.
type CreditsView struct {
	BaseCurrency string         `json:"baseCurrency"`
	Mode         string         `json:"mode"`
	Credits      []CreditRow    `json:"credits"`
	Progress     Progress       `json:"progress"`
	LeftCents    int64          `json:"leftCents"`
	Counts       map[string]int `json:"counts"`
	Currencies   []CurrencyRate `json:"currencies"`
	Purposes     []string       `json:"purposes"`
}

// CreditInput is a credit as typed into the form; ID is zero for a new one.
// Currency is only used when creating it. Rate is the currency's rate to
// the base currency: empty takes the one in settings now for a new credit
// and keeps the recorded one for an existing credit.
type CreditInput struct {
	ID              uint   `json:"id"`
	Currency        string `json:"currency"`
	Rate            string `json:"rate"`
	Name            string `json:"name"`
	Purpose         string `json:"purpose"`
	Issuer          string `json:"issuer"`
	Total           string `json:"total"`
	Paid            string `json:"paid"`
	InterestPercent string `json:"interestPercent"`
	StartDate       string `json:"startDate"`
	DueDate         string `json:"dueDate"`
	Comment         string `json:"comment"`
}

// CreditLogRow is a payment or an addition to a credit; Delta is positive.
type CreditLogRow struct {
	ID         uint   `json:"id"`
	When       string `json:"when"`
	Kind       string `json:"kind"`
	DeltaCents int64  `json:"deltaCents"`
	Note       string `json:"note"`
}

// CreditLogInput logs a "payment" or an "addition" of Amount on Date
// (YYYY-MM-DD, empty for now).
type CreditLogInput struct {
	ID     uint   `json:"id"`
	Kind   string `json:"kind"`
	Amount string `json:"amount"`
	Date   string `json:"date"`
	Note   string `json:"note"`
}

// Credits lists "active" credits (still being paid) or "paid" ones.
func (a *API) Credits(mode string) (CreditsView, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return CreditsView{}, err
	}

	items, err := storage.LoadCredits()
	if err != nil {
		return CreditsView{}, err
	}

	listMode := credit.ListMode(mode)
	if listMode != credit.ListPaid {
		listMode = credit.ListActive
	}

	listed := credit.Filter(items, listMode)
	paid, total := credit.ProgressInBaseCents(listed)
	now := time.Now()

	view := CreditsView{
		BaseCurrency: stts.BaseCurrencyLabel(),
		Mode:         string(listMode),
		Credits:      make([]CreditRow, 0, len(listed)),
		Progress:     Progress{PaidCents: paid, TotalCents: total},
		LeftCents:    credit.UnpaidInBaseCents(items),
		Counts: map[string]int{
			string(credit.ListActive): len(credit.Filter(items, credit.ListActive)),
			string(credit.ListPaid):   len(credit.Filter(items, credit.ListPaid)),
		},
		Currencies: currencyRates(stts),
		Purposes:   credit.PurposeOptions(),
	}

	for _, item := range listed {
		view.Credits = append(view.Credits, creditRow(item, stts, now))
	}

	return view, nil
}

func (a *API) CreateCredit(input CreditInput) (CreatedIn, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return CreatedIn{}, err
	}

	currency, ok := matchOption(stts.CurrencyOptions(), strings.TrimSpace(input.Currency))
	if !ok {
		return CreatedIn{}, fmt.Errorf("unknown currency %q: add it in settings first", input.Currency)
	}

	rate, err := stts.EntryRate(currency, input.Rate)
	if err != nil {
		return CreatedIn{}, err
	}

	entry, err := credit.New(currency, rate, creditFields(input), dates.ISO, time.Now())
	if err != nil {
		return CreatedIn{}, err
	}

	if err := storage.CreateCredit(&entry); err != nil {
		return CreatedIn{}, fmt.Errorf("save failed: %w", err)
	}

	return CreatedIn{Mode: string(creditMode(entry))}, nil
}

// UpdateCredit changes a credit and, for one in another currency, its
// recorded rate; the currency stays. It returns the list it is in now.
func (a *API) UpdateCredit(input CreditInput) (CreatedIn, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return CreatedIn{}, err
	}

	current, err := findCredit(storage, input.ID)
	if err != nil {
		return CreatedIn{}, err
	}

	now := time.Now()
	updated, err := current.Edit(creditFields(input), dates.ISO, now)
	if err != nil {
		return CreatedIn{}, err
	}

	if !stts.IsBase(current.Currency) && strings.TrimSpace(input.Rate) != "" {
		rate, err := money.ParseRate(input.Rate)
		if err != nil {
			return CreatedIn{}, err
		}

		if updated, err = updated.WithRate(rate, now); err != nil {
			return CreatedIn{}, err
		}
	}

	if err := storage.SaveCredit(&updated); err != nil {
		return CreatedIn{}, err
	}

	return CreatedIn{Mode: string(creditMode(updated))}, nil
}

func (a *API) DeleteCredit(id uint) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	if _, err := findCredit(storage, id); err != nil {
		return err
	}

	return storage.DeleteCredit(id)
}

// CreditLogs returns a credit's payments and additions, newest first.
func (a *API) CreditLogs(id uint) ([]CreditLogRow, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return nil, err
	}

	logs, err := storage.LoadCreditLogs(id)
	if err != nil {
		return nil, err
	}

	result := make([]CreditLogRow, 0, len(logs))
	for _, entry := range logs {
		delta := entry.DeltaPaidCents
		if entry.Kind() == credit.LogAddition {
			delta = entry.DeltaTotalCents
		}

		result = append(result, CreditLogRow{ID: entry.ID, When: entry.CreatedAt.Local().Format("2006-01-02 15:04"), Kind: string(entry.Kind()), DeltaCents: delta, Note: entry.Note})
	}

	return result, nil
}

// AddCreditLog logs a payment or an addition, returning the credit as it is
// now.
func (a *API) AddCreditLog(input CreditLogInput) (CreditRow, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return CreditRow{}, err
	}

	current, err := findCredit(storage, input.ID)
	if err != nil {
		return CreditRow{}, err
	}

	now := time.Now()
	updated, entry, err := current.ApplyLog(credit.LogKind(input.Kind), input.Amount, input.Date, input.Note, dates.ISO, now)
	if err != nil {
		return CreditRow{}, err
	}

	if err := storage.AddCreditLog(&updated, &entry); err != nil {
		return CreditRow{}, err
	}

	return creditRow(updated, stts, now), nil
}

// DeleteCreditLog removes one log entry and undoes it, returning the credit
// as it is now.
func (a *API) DeleteCreditLog(creditID uint, logID uint) (CreditRow, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return CreditRow{}, err
	}

	current, err := findCredit(storage, creditID)
	if err != nil {
		return CreditRow{}, err
	}

	logs, err := storage.LoadCreditLogs(creditID)
	if err != nil {
		return CreditRow{}, err
	}

	var found *credit.CreditLog
	for i := range logs {
		if logs[i].ID == logID {
			found = &logs[i]
		}
	}

	if found == nil {
		return CreditRow{}, errLogNotFound
	}

	now := time.Now()
	updated, err := current.RemoveLog(*found, now)
	if err != nil {
		return CreditRow{}, err
	}

	if err := storage.DeleteCreditLog(&updated, logID); err != nil {
		return CreditRow{}, err
	}

	return creditRow(updated, stts, now), nil
}

func creditFields(input CreditInput) credit.Fields {
	return credit.Fields{
		Name:            input.Name,
		Purpose:         input.Purpose,
		Issuer:          input.Issuer,
		Total:           input.Total,
		Paid:            input.Paid,
		InterestPercent: input.InterestPercent,
		StartDate:       input.StartDate,
		DueDate:         input.DueDate,
		Comment:         input.Comment,
	}
}

func creditMode(item credit.Credit) credit.ListMode {
	if item.IsPaid() {
		return credit.ListPaid
	}

	return credit.ListActive
}

func findCredit(storage StorageWorker, id uint) (credit.Credit, error) {
	items, err := storage.LoadCredits()
	if err != nil {
		return credit.Credit{}, err
	}

	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}

	return credit.Credit{}, errCreditNotFound
}

func creditRow(item credit.Credit, stts settings.AppSettings, now time.Time) CreditRow {
	baseLeft, hasRate := item.LeftBaseCents()

	return CreditRow{
		ID:              item.ID,
		Name:            item.Name,
		Purpose:         item.Purpose,
		Issuer:          item.Issuer,
		Currency:        item.Currency,
		IsBase:          stts.IsBase(item.Currency),
		RateToBase:      item.RateToBase,
		HasRate:         hasRate,
		TotalCents:      item.TotalCents,
		PaidCents:       item.PaidCents,
		LeftCents:       item.LeftCents(),
		BaseTotalCents:  item.TotalBaseCents,
		BasePaidCents:   item.PaidBaseCents,
		BaseLeftCents:   baseLeft,
		InterestPercent: item.InterestPercent,
		StartDate:       formatDay(item.StartDate),
		DueDate:         formatOptionalDay(item.DueDate),
		Overdue:         overdue(item.DueDate, item.IsPaid(), now),
		Comment:         item.Comment,
	}
}
