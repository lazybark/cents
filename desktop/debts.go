package desktop

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/debt"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/money"
)

var (
	errDebtNotFound = errors.New("debt not found")
	errLogNotFound  = errors.New("log entry not found")
)

// DebtRow has amounts in the debt's currency and, as Base*, in the base
// currency at the rate recorded with it (HasRate is false when it has none).
type DebtRow struct {
	ID              uint    `json:"id"`
	IsOwedToUser    bool    `json:"isOwedToUser"`
	Peer            string  `json:"peer"`
	Currency        string  `json:"currency"`
	IsBase          bool    `json:"isBase"`
	RateToBase      float64 `json:"rateToBase"`
	HasRate         bool    `json:"hasRate"`
	AmountCents     int64   `json:"amountCents"`
	PaidCents       int64   `json:"paidCents"`
	LeftCents       int64   `json:"leftCents"`
	BaseAmountCents int64   `json:"baseAmountCents"`
	BasePaidCents   int64   `json:"basePaidCents"`
	BaseLeftCents   int64   `json:"baseLeftCents"`
	CreatedAt       string  `json:"createdAt"`
	DueDate         string  `json:"dueDate"`
	Overdue         bool    `json:"overdue"`
	Comment         string  `json:"comment"`
}

type DebtsView struct {
	BaseCurrency string         `json:"baseCurrency"`
	Mode         string         `json:"mode"`
	Debts        []DebtRow      `json:"debts"`
	Progress     Progress       `json:"progress"`
	Counts       map[string]int `json:"counts"`
	Currencies   []CurrencyRate `json:"currencies"`
	// Cashflow is what a payment added to incomes and expenses can use.
	Cashflow CashflowOptions `json:"cashflow"`
}

// NewDebtInput is a new debt. Rate is the currency's rate to the base
// currency; empty takes the one in settings now.
type NewDebtInput struct {
	IsOwedToUser bool   `json:"isOwedToUser"`
	Peer         string `json:"peer"`
	Currency     string `json:"currency"`
	Rate         string `json:"rate"`
	Amount       string `json:"amount"`
	AmountPaid   string `json:"amountPaid"`
	CreatedAt    string `json:"createdAt"`
	DueDate      string `json:"dueDate"`
	Comment      string `json:"comment"`
}

// DebtUpdateInput changes a debt. Rate replaces the recorded rate of a debt
// in another currency; empty keeps it.
type DebtUpdateInput struct {
	ID         uint   `json:"id"`
	Rate       string `json:"rate"`
	Amount     string `json:"amount"`
	AmountPaid string `json:"amountPaid"`
	CreatedAt  string `json:"createdAt"`
	DueDate    string `json:"dueDate"`
	Comment    string `json:"comment"`
}

// Debts lists one of the TUI's debt lists: "outgoing" (unpaid, I owe),
// "incoming" (unpaid, owed to me) or "paid" (both directions). Progress is in
// the base currency; Counts has every list's size for the tabs.
func (a *API) Debts(mode string) (DebtsView, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return DebtsView{}, err
	}

	items, err := storage.LoadDebts()
	if err != nil {
		return DebtsView{}, fmt.Errorf("failed to load debts: %w", err)
	}

	listMode := debt.ListMode(mode)
	if listMode != debt.ListIncoming && listMode != debt.ListPaid {
		listMode = debt.ListOutgoing
	}

	accounts, err := storage.LoadAccounts()
	if err != nil {
		return DebtsView{}, fmt.Errorf("failed to load accounts: %w", err)
	}

	listed := debt.Filter(items, listMode)
	paid, total := debt.ProgressInBaseCents(listed)
	now := time.Now()

	view := DebtsView{
		BaseCurrency: stts.BaseCurrencyLabel(),
		Mode:         string(listMode),
		Debts:        make([]DebtRow, 0, len(listed)),
		Progress:     Progress{PaidCents: paid, TotalCents: total},
		Counts:       map[string]int{},
		Currencies:   currencyRates(stts),
		Cashflow:     cashflowOptions(stts, accounts),
	}

	for _, m := range []debt.ListMode{debt.ListOutgoing, debt.ListIncoming, debt.ListPaid} {
		view.Counts[string(m)] = len(debt.Filter(items, m))
	}

	for _, item := range listed {
		view.Debts = append(view.Debts, debtRow(item, stts, now))
	}

	return view, nil
}

func (a *API) CreateDebt(input NewDebtInput) (CreatedIn, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return CreatedIn{}, err
	}

	currency, ok := matchOption(stts.CurrencyOptions(), strings.TrimSpace(input.Currency))

	rate := 0.0
	if ok {
		var err error
		if rate, err = stts.EntryRate(currency, input.Rate); err != nil {
			return CreatedIn{}, err
		}
	}

	entry, err := debt.New(input.IsOwedToUser, input.Peer, input.Currency, rate, input.Amount, input.AmountPaid, input.CreatedAt, input.DueDate, input.Comment, dates.ISO, time.Now())
	if err != nil {
		return CreatedIn{}, err
	}

	if !ok {
		return CreatedIn{}, fmt.Errorf("unknown currency %q: add it in settings first", input.Currency)
	}

	entry.Currency = currency

	if err := storage.CreateDebt(&entry); err != nil {
		return CreatedIn{}, fmt.Errorf("save failed: %w", err)
	}

	// Like the TUI, show the list the new debt belongs to.
	mode := debt.ListOutgoing
	if entry.IsOwedToUser {
		mode = debt.ListIncoming
	}

	if entry.IsPaid() {
		mode = debt.ListPaid
	}

	return CreatedIn{Mode: string(mode)}, nil
}

// UpdateDebt changes amount, amount paid, dates and comment, plus the rate
// of a debt in another currency; peer, currency and direction can't change,
// as in the TUI.
func (a *API) UpdateDebt(input DebtUpdateInput) error {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return err
	}

	current, err := findDebt(storage, input.ID)
	if err != nil {
		return err
	}

	now := time.Now()
	updated, err := current.Edit(input.Amount, input.AmountPaid, input.CreatedAt, input.DueDate, input.Comment, dates.ISO, now)
	if err != nil {
		return err
	}

	if !stts.IsBase(current.Currency) && strings.TrimSpace(input.Rate) != "" {
		rate, err := money.ParseRate(input.Rate)
		if err != nil {
			return err
		}

		if updated, err = updated.WithRate(rate, now); err != nil {
			return err
		}
	}

	if err := storage.SaveDebt(&updated); err != nil {
		return fmt.Errorf("save failed: %w", err)
	}

	return nil
}

// DebtLogs returns a debt's latest payments, newest first.
func (a *API) DebtLogs(id uint) ([]PaymentLog, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return nil, err
	}

	logs, err := storage.LoadDebtLogs(id)
	if err != nil {
		return nil, err
	}

	result := make([]PaymentLog, 0, len(logs))
	for _, entry := range logs {
		result = append(result, PaymentLog{ID: entry.ID, When: entry.CreatedAt.Local().Format("2006-01-02 15:04"), DeltaCents: entry.DeltaPaidCents, Note: entry.Note, CashflowID: entry.CashflowEntryID})
	}

	return result, nil
}

// AddDebtPayment logs a payment and moves the amount paid by it, returning
// the debt as it is now. With Cashflow.Add the payment is also added as an
// expense (an income, for a debt owed to the user), all together.
func (a *API) AddDebtPayment(input PaymentInput) (DebtRow, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return DebtRow{}, err
	}

	current, err := findDebt(storage, input.ID)
	if err != nil {
		return DebtRow{}, err
	}

	now := time.Now()
	updated, entry, err := current.ApplyPayment(input.Delta, input.Date, input.Note, dates.ISO, now)
	if err != nil {
		return DebtRow{}, err
	}

	accounts, err := storage.LoadAccounts()
	if err != nil {
		return DebtRow{}, fmt.Errorf("failed to load accounts: %w", err)
	}

	what := "Payment to " + current.Peer
	if current.IsOwedToUser {
		what = "Repayment from " + current.Peer
	}

	cash, err := paymentEntry(input.Cashflow, stts, accounts, current.IsOwedToUser, current.Currency, entry.DeltaPaidCents, input.Date, paymentComment(what, input.Note), now)
	if err != nil {
		return DebtRow{}, err
	}

	if err := storage.AddDebtPayment(&updated, &entry, cash); err != nil {
		return DebtRow{}, err
	}

	return debtRow(updated, stts, now), nil
}

// DeleteDebtPayment removes one logged payment and undoes it, so the amount
// paid moves back by the payment's amount. Only the payments DebtLogs lists
// can be removed. Returns the debt as it is now.
func (a *API) DeleteDebtPayment(debtID uint, logID uint) (DebtRow, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return DebtRow{}, err
	}

	current, err := findDebt(storage, debtID)
	if err != nil {
		return DebtRow{}, err
	}

	logs, err := storage.LoadDebtLogs(debtID)
	if err != nil {
		return DebtRow{}, err
	}

	var payment *debt.DebtLog
	for i := range logs {
		if logs[i].ID == logID {
			payment = &logs[i]
		}
	}

	if payment == nil {
		return DebtRow{}, errLogNotFound
	}

	now := time.Now()
	updated, err := current.RemovePayment(*payment, now)
	if err != nil {
		return DebtRow{}, err
	}

	if err := storage.DeleteDebtLog(&updated, logID); err != nil {
		return DebtRow{}, err
	}

	return debtRow(updated, stts, now), nil
}

func (a *API) DeleteDebt(id uint) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	if _, err := findDebt(storage, id); err != nil {
		return err
	}

	if err := storage.DeleteDebt(id); err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	return nil
}

func findDebt(storage StorageWorker, id uint) (debt.Debt, error) {
	items, err := storage.LoadDebts()
	if err != nil {
		return debt.Debt{}, fmt.Errorf("failed to load debts: %w", err)
	}

	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}

	return debt.Debt{}, errDebtNotFound
}

func debtRow(item debt.Debt, stts settings.AppSettings, now time.Time) DebtRow {
	baseLeft, hasRate := item.LeftBaseCents()

	return DebtRow{
		ID:              item.ID,
		IsOwedToUser:    item.IsOwedToUser,
		Peer:            item.Peer,
		Currency:        item.Currency,
		IsBase:          stts.IsBase(item.Currency),
		RateToBase:      item.RateToBase,
		HasRate:         hasRate,
		AmountCents:     item.AmountCents,
		PaidCents:       item.AmountPaidCents,
		LeftCents:       item.LeftCents(),
		BaseAmountCents: item.AmountBaseCents,
		BasePaidCents:   item.AmountPaidBaseCents,
		BaseLeftCents:   baseLeft,
		CreatedAt:       formatDay(item.DebtCreatedAt),
		DueDate:         formatOptionalDay(item.DueDate),
		Overdue:         overdue(item.DueDate, item.IsPaid(), now),
		Comment:         item.Comment,
	}
}
