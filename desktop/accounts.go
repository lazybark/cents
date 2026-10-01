package desktop

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/money"
)

// logDateLayout is what <input type="date"> sends and expects.
const logDateLayout = "2006-01-02"

var errAccountNotFound = errors.New("account not found")

type AccountRow struct {
	ID                uint      `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	Currency          string    `json:"currency"`
	BalanceCents      int64     `json:"balanceCents"`
	BaseCents         int64     `json:"baseCents"`
	HasRate           bool      `json:"hasRate"`
	IgnoreInSummaries bool      `json:"ignoreInSummaries"`
	LastUpdatedAt     time.Time `json:"lastUpdatedAt"`
}

type CurrencyTotal struct {
	Currency  string `json:"currency"`
	Cents     int64  `json:"cents"`
	BaseCents int64  `json:"baseCents"`
	HasRate   bool   `json:"hasRate"`
}

type AccountsOverview struct {
	BaseCurrency   string          `json:"baseCurrency"`
	TotalCents     int64           `json:"totalCents"`
	CurrencyTotals []CurrencyTotal `json:"currencyTotals"`
	IgnoredCount   int             `json:"ignoredCount"`
	MissingRates   int             `json:"missingRates"`
	Accounts       []AccountRow    `json:"accounts"`
	Currencies     []string        `json:"currencies"`
	SortOptions    []string        `json:"sortOptions"`
	Sort           int             `json:"sort"`
	LoadedAt       time.Time       `json:"loadedAt"`
}

type NewAccountInput struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	Currency          string `json:"currency"`
	Amount            string `json:"amount"`
	IgnoreInSummaries bool   `json:"ignoreInSummaries"`
}

type AmountUpdateInput struct {
	ID                uint   `json:"id"`
	Amount            string `json:"amount"`
	IgnoreInSummaries bool   `json:"ignoreInSummaries"`
	UpdateLog         bool   `json:"updateLog"`
}

type AmountUpdateResult struct {
	// Warning is set when the amount was saved but the value log was not.
	Warning string `json:"warning"`
}

type ValueLog struct {
	Date       string `json:"date"`
	ValueCents int64  `json:"valueCents"`
}

type ValueLogInput struct {
	AccountID uint   `json:"accountId"`
	Date      string `json:"date"`
	Value     string `json:"value"`
}

// Accounts reads accounts fresh from storage on every call, so changes made
// in the TUI (or elsewhere) show up after a refresh. sort is an index into
// SortOptions; anything out of range sorts by base balance.
func (a *API) Accounts(sort int) (AccountsOverview, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return AccountsOverview{}, err
	}

	stts, err := storage.LoadAppSettings()
	if err != nil {
		return AccountsOverview{}, fmt.Errorf("failed to load settings: %w", err)
	}

	accounts, err := storage.LoadAccounts()
	if err != nil {
		return AccountsOverview{}, fmt.Errorf("failed to load accounts: %w", err)
	}

	sortOptions := account.SortOptions()
	if sort < 0 || sort >= len(sortOptions) {
		sort = int(account.SortBaseAmount)
	}

	accounts = account.Sort(accounts, stts, account.SortField(sort))

	result := AccountsOverview{
		BaseCurrency: stts.BaseCurrencyLabel(),
		TotalCents:   account.SumInBaseCents(accounts, stts),
		Accounts:     make([]AccountRow, 0, len(accounts)),
		Currencies:   stts.CurrencyOptions(),
		SortOptions:  sortOptions,
		Sort:         sort,
		LoadedAt:     time.Now(),
	}

	byCurrency := account.TotalsByCurrency(accounts, stts)
	result.CurrencyTotals = make([]CurrencyTotal, 0, len(byCurrency))
	for _, total := range byCurrency {
		result.CurrencyTotals = append(result.CurrencyTotals, CurrencyTotal(total))
	}

	for _, acct := range accounts {
		baseCents, ok := stts.ConvertToBaseCents(acct.Currency, acct.BalanceCents)

		if acct.IgnoreInSummaries {
			result.IgnoredCount++
		} else if !ok {
			result.MissingRates++
		}

		result.Accounts = append(result.Accounts, AccountRow{
			ID:                acct.ID,
			Name:              acct.Name,
			Description:       acct.Description,
			Currency:          acct.Currency,
			BalanceCents:      acct.BalanceCents,
			BaseCents:         baseCents,
			HasRate:           ok,
			IgnoreInSummaries: acct.IgnoreInSummaries,
			LastUpdatedAt:     acct.LastUpdatedAt,
		})
	}

	return result, nil
}

func (a *API) CreateAccount(input NewAccountInput) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	stts, err := storage.LoadAppSettings()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	entry, err := account.New(input.Name, input.Description, input.Currency, input.Amount, input.IgnoreInSummaries, time.Now())
	if err != nil {
		return err
	}

	currency, ok := matchOption(stts.CurrencyOptions(), entry.Currency)
	if !ok {
		return fmt.Errorf("unknown currency %q: add it in settings first", entry.Currency)
	}

	entry.Currency = currency

	if err := storage.CreateAccount(&entry); err != nil {
		return fmt.Errorf("save failed: %w", err)
	}

	return nil
}

func (a *API) UpdateAccountAmount(input AmountUpdateInput) (AmountUpdateResult, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return AmountUpdateResult{}, err
	}

	amount, err := money.ParseAmountCents(input.Amount)
	if err != nil {
		return AmountUpdateResult{}, fmt.Errorf("amount error: %w", err)
	}

	if err := requireAccount(storage, input.ID); err != nil {
		return AmountUpdateResult{}, err
	}

	now := time.Now()
	if err := storage.UpdateAccountAmount(input.ID, amount, input.IgnoreInSummaries, now); err != nil {
		return AmountUpdateResult{}, fmt.Errorf("update failed: %w", err)
	}

	var result AmountUpdateResult
	if input.UpdateLog {
		if err := storage.UpsertAccountValueLog(input.ID, now, amount); err != nil {
			result.Warning = "log update failed: " + err.Error()
		}
	}

	return result, nil
}

func (a *API) DeleteAccount(id uint) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	if err := requireAccount(storage, id); err != nil {
		return err
	}

	if err := storage.DeleteAccount(id); err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	return nil
}

// AccountValueLogs returns an account's value history, newest first.
func (a *API) AccountValueLogs(accountID uint) ([]ValueLog, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return nil, err
	}

	logs, err := storage.LoadAccountValueLogs(accountID)
	if err != nil {
		return nil, err
	}

	result := make([]ValueLog, 0, len(logs))
	for _, entry := range logs {
		result = append(result, ValueLog{Date: entry.LogDate.Local().Format(logDateLayout), ValueCents: entry.ValueCents})
	}

	return result, nil
}

// SaveAccountValueLog adds the value for a day, or replaces it if that day
// already has one.
func (a *API) SaveAccountValueLog(input ValueLogInput) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	rawDay := strings.TrimSpace(input.Date)
	if rawDay == "" {
		return errors.New("log date is required")
	}

	day, err := time.ParseInLocation(logDateLayout, rawDay, time.Local)
	if err != nil {
		return errors.New("log date must use YYYY-MM-DD format")
	}

	value, err := money.ParseAmountCents(input.Value)
	if err != nil {
		return fmt.Errorf("log value error: %w", err)
	}

	if err := requireAccount(storage, input.AccountID); err != nil {
		return err
	}

	if err := storage.UpsertAccountValueLog(input.AccountID, day, value); err != nil {
		return fmt.Errorf("log save failed: %w", err)
	}

	return nil
}

func requireAccount(storage StorageWorker, id uint) error {
	accounts, err := storage.LoadAccounts()
	if err != nil {
		return fmt.Errorf("failed to load accounts: %w", err)
	}

	for _, acct := range accounts {
		if acct.ID == id {
			return nil
		}
	}

	return errAccountNotFound
}

// matchOption finds value among options ignoring case, returning the option's
// own spelling so stored currencies stay consistent.
func matchOption(options []string, value string) (string, bool) {
	for _, option := range options {
		if strings.EqualFold(strings.TrimSpace(option), value) {
			return option, true
		}
	}

	return "", false
}

// accountNames lists account names for pickers, skipping blank ones.
func accountNames(accounts []account.Account) []string {
	names := make([]string, 0, len(accounts))
	for _, acct := range accounts {
		if name := strings.TrimSpace(acct.Name); name != "" {
			names = append(names, name)
		}
	}

	return names
}
