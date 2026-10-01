package desktop

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/money"
)

// monthLayout is what the frontend uses to name a month, like
// <input type="month">.
const monthLayout = "2006-01"

var errCashflowNotFound = errors.New("entry not found")

type CashflowRow struct {
	ID          uint   `json:"id"`
	IsIncome    bool   `json:"isIncome"`
	Date        string `json:"date"`
	Currency    string `json:"currency"`
	AmountCents int64  `json:"amountCents"`
	BaseCents   int64  `json:"baseCents"`
	HasRate     bool   `json:"hasRate"`
	Category    string `json:"category"`
	Account     string `json:"account"`
	Comment     string `json:"comment"`
}

// CashflowOptions are the choices the income/expense form offers.
type CashflowOptions struct {
	Currencies        []string `json:"currencies"`
	IncomeCategories  []string `json:"incomeCategories"`
	ExpenseCategories []string `json:"expenseCategories"`
	Accounts          []string `json:"accounts"`
}

type CashflowMonth struct {
	BaseCurrency string          `json:"baseCurrency"`
	Month        string          `json:"month"`
	Entries      []CashflowRow   `json:"entries"`
	IncomeCents  int64           `json:"incomeCents"`
	ExpenseCents int64           `json:"expenseCents"`
	NetCents     int64           `json:"netCents"`
	MissingRates int             `json:"missingRates"`
	Options      CashflowOptions `json:"options"`
}

type CashflowOverviewRow struct {
	Month        string `json:"month"`
	IncomeCents  int64  `json:"incomeCents"`
	ExpenseCents int64  `json:"expenseCents"`
	NetCents     int64  `json:"netCents"`
	DeltaCents   int64  `json:"deltaCents"`
	HasPrev      bool   `json:"hasPrev"`
}

type CashflowOverview struct {
	BaseCurrency string                `json:"baseCurrency"`
	Rows         []CashflowOverviewRow `json:"rows"`
	MissingRates int                   `json:"missingRates"`
}

type NewCashflowInput struct {
	IsIncome bool   `json:"isIncome"`
	Currency string `json:"currency"`
	Amount   string `json:"amount"`
	Date     string `json:"date"`
	Category string `json:"category"`
	Account  string `json:"account"`
	Comment  string `json:"comment"`
}

type NewCashflowResult struct {
	// Month is the entry's month, so the frontend can show it.
	Month string `json:"month"`
}

// CashflowMonth lists one month's incomes and expenses with their totals.
// month is "YYYY-MM"; empty means the current month.
func (a *API) CashflowMonth(month string) (CashflowMonth, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return CashflowMonth{}, err
	}

	start := cashflow.MonthStart(time.Now())
	if strings.TrimSpace(month) != "" {
		parsed, err := time.ParseInLocation(monthLayout, month, time.Local)
		if err != nil {
			return CashflowMonth{}, errors.New("month must use YYYY-MM format")
		}

		start = parsed
	}

	stts, err := storage.LoadAppSettings()
	if err != nil {
		return CashflowMonth{}, fmt.Errorf("failed to load settings: %w", err)
	}

	entries, err := storage.LoadCashflows()
	if err != nil {
		return CashflowMonth{}, fmt.Errorf("failed to load cashflows: %w", err)
	}

	accounts, err := storage.LoadAccounts()
	if err != nil {
		return CashflowMonth{}, fmt.Errorf("failed to load accounts: %w", err)
	}

	items := cashflow.ForMonth(entries, start)
	income, expense, missing := cashflow.Totals(items, stts)

	result := CashflowMonth{
		BaseCurrency: stts.BaseCurrencyLabel(),
		Month:        start.Format(monthLayout),
		Entries:      make([]CashflowRow, 0, len(items)),
		IncomeCents:  income,
		ExpenseCents: expense,
		NetCents:     income - expense,
		MissingRates: missing,
		Options: CashflowOptions{
			Currencies:        stts.CurrencyOptions(),
			IncomeCategories:  stts.IncomeCategoryOptions(),
			ExpenseCategories: stts.ExpenseCategoryOptions(),
			Accounts:          accountNames(accounts),
		},
	}

	for _, entry := range items {
		baseCents, ok := stts.ConvertToBaseCents(entry.Currency, entry.AmountCents)
		result.Entries = append(result.Entries, CashflowRow{
			ID:          entry.ID,
			IsIncome:    entry.IsIncome,
			Date:        entry.EntryDate.Local().Format(logDateLayout),
			Currency:    entry.Currency,
			AmountCents: entry.AmountCents,
			BaseCents:   baseCents,
			HasRate:     ok,
			Category:    entry.Category,
			Account:     entry.AccountName,
			Comment:     entry.Comment,
		})
	}

	return result, nil
}

// CashflowOverview totals every month, newest first.
func (a *API) CashflowOverview() (CashflowOverview, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return CashflowOverview{}, err
	}

	stts, err := storage.LoadAppSettings()
	if err != nil {
		return CashflowOverview{}, fmt.Errorf("failed to load settings: %w", err)
	}

	entries, err := storage.LoadCashflows()
	if err != nil {
		return CashflowOverview{}, fmt.Errorf("failed to load cashflows: %w", err)
	}

	rows, missing := cashflow.MonthlyOverview(entries, stts)
	result := CashflowOverview{
		BaseCurrency: stts.BaseCurrencyLabel(),
		Rows:         make([]CashflowOverviewRow, 0, len(rows)),
		MissingRates: missing,
	}

	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		result.Rows = append(result.Rows, CashflowOverviewRow{
			Month:        row.Month.Format(monthLayout),
			IncomeCents:  row.IncomeBase,
			ExpenseCents: row.ExpenseBase,
			NetCents:     row.NetBase,
			DeltaCents:   row.DeltaFromPrev,
			HasPrev:      row.HasPrev,
		})
	}

	return result, nil
}

func (a *API) CreateCashflow(input NewCashflowInput) (NewCashflowResult, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return NewCashflowResult{}, err
	}

	stts, err := storage.LoadAppSettings()
	if err != nil {
		return NewCashflowResult{}, fmt.Errorf("failed to load settings: %w", err)
	}

	accounts, err := storage.LoadAccounts()
	if err != nil {
		return NewCashflowResult{}, fmt.Errorf("failed to load accounts: %w", err)
	}

	amount, err := money.ParseAmountCents(input.Amount)
	if err != nil {
		return NewCashflowResult{}, fmt.Errorf("amount error: %w", err)
	}

	// Parsed as UTC midnight, exactly like the TUI's DD.MM.YYYY input, so
	// entries from both interfaces land on the same day.
	entryDate, err := time.Parse(logDateLayout, strings.TrimSpace(input.Date))
	if err != nil {
		return NewCashflowResult{}, errors.New("date must use YYYY-MM-DD format")
	}

	currency, ok := matchOption(stts.CurrencyOptions(), strings.TrimSpace(input.Currency))
	if !ok {
		return NewCashflowResult{}, fmt.Errorf("unknown currency %q: add it in settings first", input.Currency)
	}

	accountName := strings.TrimSpace(input.Account)
	if accountName != "" {
		if accountName, ok = matchOption(accountNames(accounts), accountName); !ok {
			return NewCashflowResult{}, fmt.Errorf("unknown account %q", input.Account)
		}
	}

	categories := stts.ExpenseCategoryOptions()
	if input.IsIncome {
		categories = stts.IncomeCategoryOptions()
	}

	entry, err := cashflow.New(input.IsIncome, currency, amount, entryDate, input.Category, categories, accountName, input.Comment, time.Now())
	if err != nil {
		return NewCashflowResult{}, err
	}

	if err := storage.CreateCashflow(&entry); err != nil {
		return NewCashflowResult{}, fmt.Errorf("save failed: %w", err)
	}

	return NewCashflowResult{Month: cashflow.MonthStart(entry.EntryDate).Format(monthLayout)}, nil
}

func (a *API) DeleteCashflow(id uint) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	entries, err := storage.LoadCashflows()
	if err != nil {
		return fmt.Errorf("failed to load cashflows: %w", err)
	}

	found := false
	for _, entry := range entries {
		if entry.ID == id {
			found = true

			break
		}
	}

	if !found {
		return errCashflowNotFound
	}

	if err := storage.DeleteCashflow(id); err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	return nil
}
