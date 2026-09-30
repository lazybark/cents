package desktop

import (
	"errors"
	"fmt"
	"github.com/lazybark/cents/flows/tag"
	"strings"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/money"
)

// searchShown is how many matches are listed; totals count them all.
const searchShown = 1000

// CashflowSearchInput filters entries across all months; empty fields
// don't limit. Dates are YYYY-MM-DD, amounts in the base currency.
type CashflowSearchInput struct {
	Text     string `json:"text"`
	Kind     string `json:"kind"`
	Category string `json:"category"`
	Account  string `json:"account"`
	Currency string `json:"currency"`
	Tag      string `json:"tag"`
	From     string `json:"from"`
	To       string `json:"to"`
	MinBase  string `json:"minBase"`
	MaxBase  string `json:"maxBase"`
}

// CashflowSearchResult is what matched, newest first (the first
// searchShown of Total), with totals over every match in the base currency.
type CashflowSearchResult struct {
	BaseCurrency string        `json:"baseCurrency"`
	Entries      []CashflowRow `json:"entries"`
	Total        int           `json:"total"`
	IncomeCents  int64         `json:"incomeCents"`
	ExpenseCents int64         `json:"expenseCents"`
	NetCents     int64         `json:"netCents"`
	MissingRates int           `json:"missingRates"`
	Options      SearchOptions `json:"options"`
}

// SearchOptions are what the filters and the bulk change offer: every
// category (archived too, as old entries use them) and account.
type SearchOptions struct {
	IncomeCategories  []string `json:"incomeCategories"`
	ExpenseCategories []string `json:"expenseCategories"`
	Accounts          []string `json:"accounts"`
	Currencies        []string `json:"currencies"`
	Tags              []string `json:"tags"`
}

func (a *API) CashflowSearch(input CashflowSearchInput) (CashflowSearchResult, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return CashflowSearchResult{}, err
	}

	q := cashflow.Query{Text: input.Text, Kind: input.Kind, Category: input.Category, Account: input.Account, Currency: input.Currency, Tag: input.Tag}
	if from, err := dates.ISO.Optional(input.From, "from"); err != nil {
		return CashflowSearchResult{}, err
	} else if from != nil {
		q.From = *from
	}

	if to, err := dates.ISO.Optional(input.To, "to"); err != nil {
		return CashflowSearchResult{}, err
	} else if to != nil {
		q.To = *to
	}

	for _, bound := range []struct {
		raw  string
		dest **int64
		name string
	}{{input.MinBase, &q.MinBase, "from"}, {input.MaxBase, &q.MaxBase, "to"}} {
		if strings.TrimSpace(bound.raw) == "" {
			continue
		}

		cents, err := money.ParseAmountCents(bound.raw)
		if err != nil {
			return CashflowSearchResult{}, fmt.Errorf("amount %s must be a number", bound.name)
		}

		*bound.dest = &cents
	}

	entries, err := storage.LoadCashflows()
	if err != nil {
		return CashflowSearchResult{}, fmt.Errorf("failed to load cashflows: %w", err)
	}

	accounts, err := storage.LoadAccounts()
	if err != nil {
		return CashflowSearchResult{}, fmt.Errorf("failed to load accounts: %w", err)
	}

	matched := cashflow.Search(entries, q)
	income, expense, missing := cashflow.Totals(matched)
	result := CashflowSearchResult{
		BaseCurrency: stts.BaseCurrencyLabel(),
		Entries:      make([]CashflowRow, 0, min(len(matched), searchShown)),
		Total:        len(matched),
		IncomeCents:  income,
		ExpenseCents: expense,
		NetCents:     income - expense,
		MissingRates: missing,
		Options:      SearchOptions{IncomeCategories: []string{}, ExpenseCategories: []string{}, Accounts: []string{}, Currencies: stts.CurrencyOptions(), Tags: []string{}},
	}

	for i, e := range matched {
		if i == searchShown {
			break
		}

		result.Entries = append(result.Entries, cashflowRow(e, stts))
	}

	for _, c := range stts.IncomeCategories {
		result.Options.IncomeCategories = append(result.Options.IncomeCategories, c.CategoryName)
	}

	for _, c := range stts.ExpenseCategories {
		result.Options.ExpenseCategories = append(result.Options.ExpenseCategories, c.CategoryName)
	}

	for _, acct := range accounts {
		result.Options.Accounts = append(result.Options.Accounts, acct.Name)
	}

	tags, err := storage.LoadTags()
	if err != nil {
		return CashflowSearchResult{}, err
	}

	for _, t := range tags {
		result.Options.Tags = append(result.Options.Tags, t.Name)
	}

	return result, nil
}

// CashflowBulkInput changes several entries at once: their category when
// SetCategory, their account ("" for none) when SetAccount; AddTag and
// RemoveTag put a tag on them or take it off.
type CashflowBulkInput struct {
	IDs         []uint `json:"ids"`
	SetCategory bool   `json:"setCategory"`
	Category    string `json:"category"`
	SetAccount  bool   `json:"setAccount"`
	Account     string `json:"account"`
	AddTag      string `json:"addTag"`
	RemoveTag   string `json:"removeTag"`
}

func (a *API) ChangeCashflows(input CashflowBulkInput) (int, error) {
	if len(input.IDs) == 0 {
		return 0, errors.New("pick the entries to change")
	}

	addTag, removeTag := strings.TrimSpace(input.AddTag), strings.TrimSpace(input.RemoveTag)
	if !input.SetCategory && !input.SetAccount && addTag == "" && removeTag == "" {
		return 0, errors.New("pick what to change")
	}

	if addTag != "" {
		var err error
		if addTag, err = tag.CleanName(addTag); err != nil {
			return 0, err
		}
	}

	if input.SetCategory && strings.TrimSpace(input.Category) == "" {
		return 0, errors.New("pick a category")
	}

	storage, err := a.currentStorage()
	if err != nil {
		return 0, err
	}

	entries, err := storage.LoadCashflows()
	if err != nil {
		return 0, fmt.Errorf("failed to load cashflows: %w", err)
	}

	wanted := map[uint]bool{}
	for _, id := range input.IDs {
		wanted[id] = true
	}

	changed := make([]cashflow.CashflowEntry, 0, len(input.IDs))
	kinds := map[bool]bool{}
	for _, e := range entries {
		if !wanted[e.ID] {
			continue
		}

		kinds[e.IsIncome] = true
		if input.SetCategory {
			e.Category = input.Category
		}

		if input.SetAccount {
			e.AccountName = input.Account
		}

		if addTag != "" && !tag.Has(e.Tags, addTag) {
			e.Tags = append(e.Tags, addTag)
		}

		if removeTag != "" {
			kept := make([]string, 0, len(e.Tags))
			for _, t := range e.Tags {
				if !strings.EqualFold(t, removeTag) {
					kept = append(kept, t)
				}
			}

			e.Tags = kept
		}

		changed = append(changed, e)
	}

	if len(changed) != len(wanted) {
		return 0, errCashflowNotFound
	}

	// Incomes and expenses have their own categories.
	if input.SetCategory && len(kinds) > 1 {
		return 0, errors.New("incomes and expenses have different categories: pick only one kind to change their category")
	}

	if err := storage.SaveCashflows(changed); err != nil {
		return 0, fmt.Errorf("change failed: %w", err)
	}

	return len(changed), nil
}
