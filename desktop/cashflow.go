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

// CashflowRow is an entry; BaseCents is its amount at the rate recorded
// with it (HasRate is false when it has none).
type CashflowRow struct {
	ID          uint    `json:"id"`
	IsIncome    bool    `json:"isIncome"`
	Date        string  `json:"date"`
	Currency    string  `json:"currency"`
	IsBase      bool    `json:"isBase"`
	RateToBase  float64 `json:"rateToBase"`
	AmountCents int64   `json:"amountCents"`
	BaseCents   int64   `json:"baseCents"`
	HasRate     bool    `json:"hasRate"`
	Category    string  `json:"category"`
	Account     string  `json:"account"`
	Comment     string  `json:"comment"`
}

// CashflowOptions are the choices the income/expense form offers.
type CashflowOptions struct {
	Currencies        []string       `json:"currencies"`
	Rates             []CurrencyRate `json:"rates"`
	IncomeCategories  []string       `json:"incomeCategories"`
	ExpenseCategories []string       `json:"expenseCategories"`
	Accounts          []string       `json:"accounts"`
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

// NewCashflowInput is a new entry. Rate is the currency's rate to the base
// currency; empty takes the one in settings now.
type NewCashflowInput struct {
	IsIncome bool   `json:"isIncome"`
	Currency string `json:"currency"`
	Rate     string `json:"rate"`
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
	income, expense, missing := cashflow.Totals(items)

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
			Rates:             currencyRates(stts),
			IncomeCategories:  stts.IncomeCategoryOptions(),
			ExpenseCategories: stts.ExpenseCategoryOptions(),
			Accounts:          accountNames(accounts),
		},
	}

	for _, entry := range items {
		baseCents, ok := entry.BaseCents()
		result.Entries = append(result.Entries, CashflowRow{
			ID:          entry.ID,
			IsIncome:    entry.IsIncome,
			Date:        entry.EntryDate.Local().Format(logDateLayout),
			Currency:    entry.Currency,
			IsBase:      stts.IsBase(entry.Currency),
			RateToBase:  entry.RateToBase,
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
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return CashflowOverview{}, err
	}

	entries, err := storage.LoadCashflows()
	if err != nil {
		return CashflowOverview{}, fmt.Errorf("failed to load cashflows: %w", err)
	}

	rows, missing := cashflow.MonthlyOverview(entries)
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

	rate, err := stts.EntryRate(currency, input.Rate)
	if err != nil {
		return NewCashflowResult{}, err
	}

	entry, err := cashflow.New(input.IsIncome, currency, amount, rate, entryDate, input.Category, categories, accountName, input.Comment, time.Now())
	if err != nil {
		return NewCashflowResult{}, err
	}

	if err := storage.CreateCashflow(&entry); err != nil {
		return NewCashflowResult{}, fmt.Errorf("save failed: %w", err)
	}

	return NewCashflowResult{Month: cashflow.MonthStart(entry.EntryDate).Format(monthLayout)}, nil
}

// CashflowRateInput sets the rate an entry was made at. With WholeMonth it
// goes to every entry in the same currency and month too, to fill in a
// month's rate in one go.
type CashflowRateInput struct {
	ID         uint   `json:"id"`
	Rate       string `json:"rate"`
	WholeMonth bool   `json:"wholeMonth"`
}

// SetCashflowRate sets an entry's recorded rate (and with WholeMonth, the
// rate of the other entries in its currency and month), returning how many
// entries changed. Entries in the base currency always keep rate 1.
func (a *API) SetCashflowRate(input CashflowRateInput) (int, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return 0, err
	}

	rate, err := money.ParseRate(input.Rate)
	if err != nil {
		return 0, err
	}

	entries, err := storage.LoadCashflows()
	if err != nil {
		return 0, fmt.Errorf("failed to load cashflows: %w", err)
	}

	var target *cashflow.CashflowEntry
	for i := range entries {
		if entries[i].ID == input.ID {
			target = &entries[i]
		}
	}

	if target == nil {
		return 0, errCashflowNotFound
	}

	if stts.IsBase(target.Currency) {
		return 0, errors.New("entries in the base currency always use rate 1")
	}

	month := cashflow.MonthStart(target.EntryDate)
	now := time.Now()
	changed := make([]cashflow.CashflowEntry, 0, 1)

	for _, entry := range entries {
		sameMonth := strings.EqualFold(strings.TrimSpace(entry.Currency), strings.TrimSpace(target.Currency)) && cashflow.MonthStart(entry.EntryDate).Equal(month)
		if entry.ID != target.ID && !(input.WholeMonth && sameMonth) {
			continue
		}

		updated, err := entry.WithRate(rate, now)
		if err != nil {
			return 0, err
		}

		changed = append(changed, updated)
	}

	if err := storage.SaveCashflows(changed); err != nil {
		return 0, err
	}

	return len(changed), nil
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

// CategoryStat is one category's amounts month by month (aligned with
// CashflowCategories.Months), in the base currency, with entry counts.
type CategoryStat struct {
	Name    string  `json:"name"`
	Values  []int64 `json:"values"`
	Entries []int   `json:"entries"`
}

type CashflowCategories struct {
	BaseCurrency string         `json:"baseCurrency"`
	IsIncome     bool           `json:"isIncome"`
	Months       []string       `json:"months"`
	Categories   []CategoryStat `json:"categories"`
	MissingRates int            `json:"missingRates"`
}

// CashflowCategories splits expenses ("expense") or incomes ("income") by
// category, month by month from the oldest month with an entry to the
// newest, largest category first.
func (a *API) CashflowCategories(kind string) (CashflowCategories, error) {
	var isIncome bool
	switch kind {
	case "income":
		isIncome = true
	case "expense":
	default:
		return CashflowCategories{}, fmt.Errorf("unknown kind %q: use income or expense", kind)
	}

	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return CashflowCategories{}, err
	}

	entries, err := storage.LoadCashflows()
	if err != nil {
		return CashflowCategories{}, fmt.Errorf("failed to load cashflows: %w", err)
	}

	months, series, missing := cashflow.ByCategory(entries, isIncome)
	result := CashflowCategories{
		BaseCurrency: stts.BaseCurrencyLabel(),
		IsIncome:     isIncome,
		Months:       make([]string, 0, len(months)),
		Categories:   make([]CategoryStat, 0, len(series)),
		MissingRates: missing,
	}

	for _, month := range months {
		result.Months = append(result.Months, month.Format(monthLayout))
	}

	for _, s := range series {
		result.Categories = append(result.Categories, CategoryStat{Name: s.Category, Values: s.Values, Entries: s.Entries})
	}

	return result, nil
}
