package desktop

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lazybark/cents/flows/analytics"
	"github.com/lazybark/cents/flows/budget"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/settings"
)

// BudgetRow is a budget against a month's spending, in the base currency.
// Name is the category, or "All spending" for the total; Archived says the
// category is archived (or gone from settings).
type BudgetRow struct {
	ID         uint    `json:"id"`
	Category   string  `json:"category"`
	Name       string  `json:"name"`
	IsTotal    bool    `json:"isTotal"`
	Archived   bool    `json:"archived"`
	LimitCents int64   `json:"limitCents"`
	SpentCents int64   `json:"spentCents"`
	LeftCents  int64   `json:"leftCents"`
	Percent    float64 `json:"percent"`
	State      string  `json:"state"`
	// UsualCents is the category's average a month over the year before.
	UsualCents int64 `json:"usualCents"`
}

// BudgetsView is the Budgets screen for a month. For the running month,
// DaysIn of DaysInMonth have gone by.
type BudgetsView struct {
	BaseCurrency string        `json:"baseCurrency"`
	Month        string        `json:"month"`
	IsCurrent    bool          `json:"isCurrent"`
	DaysIn       int           `json:"daysIn"`
	DaysInMonth  int           `json:"daysInMonth"`
	Rows         []BudgetRow   `json:"rows"`
	MissingRates int           `json:"missingRates"`
	Options      BudgetOptions `json:"options"`
}

// BudgetOptions are what a new budget can be for: expense categories in
// use (not archived) without one yet, and whether all spending has one;
// Usual suggests a limit for each (by category, "" for all spending).
type BudgetOptions struct {
	Categories []string         `json:"categories"`
	HasTotal   bool             `json:"hasTotal"`
	Usual      map[string]int64 `json:"usual"`
}

type BudgetInput struct {
	ID       uint   `json:"id"`
	Category string `json:"category"`
	Limit    string `json:"limit"`
}

const allSpending = "All spending"

func parseMonth(month string, now time.Time) (time.Time, error) {
	if strings.TrimSpace(month) == "" {
		return cashflow.MonthStart(now), nil
	}

	parsed, err := time.ParseInLocation(monthLayout, strings.TrimSpace(month), time.Local)
	if err != nil {
		return time.Time{}, errors.New("month must use YYYY-MM format")
	}

	return parsed, nil
}

// Budgets shows each budget against month's spending ("" for this month).
func (a *API) Budgets(month string) (BudgetsView, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return BudgetsView{}, err
	}

	now := time.Now()
	start, err := parseMonth(month, now)
	if err != nil {
		return BudgetsView{}, err
	}

	data, err := loadSummaryData(storage)
	if err != nil {
		return BudgetsView{}, err
	}

	stts := data.Settings
	view := BudgetsView{
		BaseCurrency: stts.BaseCurrencyLabel(),
		Month:        start.Format(monthLayout),
		IsCurrent:    start.Equal(cashflow.MonthStart(now)),
		DaysInMonth:  time.Date(start.Year(), start.Month()+1, 0, 0, 0, 0, 0, time.Local).Day(),
		Rows:         make([]BudgetRow, 0, len(data.Budgets)),
		Options:      BudgetOptions{Categories: make([]string, 0), Usual: map[string]int64{}},
	}

	if view.IsCurrent {
		view.DaysIn = now.Local().Day()
	}

	usual := analytics.VersusUsual(data.Cashflows, start)
	usualBy := map[string]int64{"": usual.UsualTotalCents}
	for _, item := range usual.Items {
		usualBy[usageKey(item.Category)] = item.UsualCents
	}

	rows, missing := budgetRows(data.Budgets, data.Cashflows, start, stts)
	view.MissingRates = missing
	budgeted := map[string]bool{}
	for _, row := range rows {
		budgeted[usageKey(row.Category)] = true
		row.UsualCents = usualBy[usageKey(row.Category)]
		view.Options.HasTotal = view.Options.HasTotal || row.IsTotal
		view.Rows = append(view.Rows, row)
	}

	view.Options.Usual[""] = usualBy[""]
	for _, c := range stts.ExpenseCategories {
		view.Options.Usual[c.CategoryName] = usualBy[usageKey(c.CategoryName)]
		if !c.Archived && !budgeted[usageKey(c.CategoryName)] {
			view.Options.Categories = append(view.Options.Categories, c.CategoryName)
		}
	}

	return view, nil
}

// activeCategory reports whether category is an expense category in
// settings that isn't archived.
func activeCategory(stts settings.AppSettings, category string) bool {
	for _, c := range stts.ExpenseCategories {
		if usageKey(c.CategoryName) == usageKey(category) {
			return !c.Archived
		}
	}

	return false
}

// SaveBudget adds a budget (ID 0) or changes one.
func (a *API) SaveBudget(input BudgetInput) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	stts, err := storage.LoadAppSettings()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	existing, err := storage.LoadBudgets()
	if err != nil {
		return err
	}

	fields := budget.Fields{Category: input.Category, Limit: input.Limit}
	if input.ID == 0 {
		b, err := budget.New(fields, stts, existing, time.Now())
		if err != nil {
			return err
		}

		return storage.CreateBudget(&b)
	}

	for _, b := range existing {
		if b.ID == input.ID {
			edited, err := budget.Edit(b, fields, stts, existing, time.Now())
			if err != nil {
				return err
			}

			return storage.SaveBudget(&edited)
		}
	}

	return fmt.Errorf("budget %d not found", input.ID)
}

func (a *API) DeleteBudget(id uint) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	return storage.DeleteBudget(id)
}

// budgetRows is each budget against month's spending, the total first;
// missing counts expenses without a rate, which can't be counted.
func budgetRows(budgets []budget.Budget, entries []cashflow.CashflowEntry, month time.Time, stts settings.AppSettings) ([]BudgetRow, int) {
	progress, missing := budget.Month(budgets, entries, month)
	rows := make([]BudgetRow, 0, len(progress))
	for _, p := range progress {
		b := p.Budget
		row := BudgetRow{
			ID: b.ID, Category: b.Category, Name: b.Category, IsTotal: b.IsTotal(),
			LimitCents: b.LimitCents, SpentCents: p.SpentCents, LeftCents: p.LeftCents, Percent: p.Percent, State: p.State,
		}

		if b.IsTotal() {
			row.Name = allSpending
		} else {
			row.Archived = !activeCategory(stts, b.Category)
		}

		rows = append(rows, row)
	}

	return rows, missing
}
