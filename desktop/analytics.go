package desktop

import (
	"fmt"
	"time"

	"github.com/lazybark/cents/flows/analytics"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/summary"
)

const (
	dayLayout = "2006-01-02"

	// The headline figures go by the last year of full months, whatever
	// range is picked for the charts.
	headlineMonths = 12
	forecastDays   = 90
)

// Analytics is the Analytics screen: headline figures, net worth over time,
// where the money goes over the picked range, and what's coming due. Money
// is in cents of the base currency.
type Analytics struct {
	BaseCurrency string `json:"baseCurrency"`
	// Months is the picked range, 0 for all time.
	Months int `json:"months"`

	// The averages over the last full months (AveragedMonths of them, up to
	// a year).
	AveragedMonths int     `json:"averagedMonths"`
	AverageIncome  int64   `json:"averageIncomeCents"`
	AverageExpense int64   `json:"averageExpenseCents"`
	SavingsRate    float64 `json:"savingsRate"`
	HasSavingsRate bool    `json:"hasSavingsRate"`

	// Runway is how many months the accounts would cover the average
	// expense.
	Accounts     int64   `json:"accountsCents"`
	RunwayMonths float64 `json:"runwayMonths"`
	HasRunway    bool    `json:"hasRunway"`

	// Committed is what subscriptions and obligations cost a month; FixedShare
	// is that as a part of the average expense, in percent.
	Committed     int64   `json:"committedCents"`
	FixedShare    float64 `json:"fixedShare"`
	HasFixedShare bool    `json:"hasFixedShare"`

	// NetWorth now, and how it changed since the end of last year (or since
	// the first month there's a value for, ChangeSince). When that start is
	// an estimate (ChangeEstimated), the change is in what's owned only, as
	// the estimate has nothing else.
	NetWorth        int64  `json:"netWorthCents"`
	Change          int64  `json:"changeCents"`
	HasChange       bool   `json:"hasChange"`
	ChangeSince     string `json:"changeSince"`
	ChangeEstimated bool   `json:"changeEstimated"`

	NetWorthHistory []NetWorthPoint `json:"netWorthHistory"`

	// Over the picked range, the running month included.
	Spending []CategorySpending `json:"spending"`
	Monthly  []MonthTotals      `json:"monthly"`

	Forecast []ForecastItem `json:"forecast"`
	// What the forecast adds up to out and in, and what usually comes in
	// over as many days going by the average income.
	ForecastDays    int   `json:"forecastDays"`
	ForecastOut     int64 `json:"forecastOutCents"`
	ForecastIn      int64 `json:"forecastInCents"`
	TypicalIncome   int64 `json:"typicalIncomeCents"`
	ForecastMissing int   `json:"forecastMissing"`
}

type NetWorthPoint struct {
	Month     string `json:"month"`
	Cents     int64  `json:"cents"`
	Estimated bool   `json:"estimated"`
}

type CategorySpending struct {
	Category string  `json:"category"`
	Cents    int64   `json:"cents"`
	Share    float64 `json:"share"`
}

type MonthTotals struct {
	Month        string `json:"month"`
	IncomeCents  int64  `json:"incomeCents"`
	ExpenseCents int64  `json:"expenseCents"`
}

type ForecastItem struct {
	Date      string `json:"date"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Incoming  bool   `json:"incoming"`
	Currency  string `json:"currency"`
	IsBase    bool   `json:"isBase"`
	Cents     int64  `json:"cents"`
	BaseCents int64  `json:"baseCents"`
	HasRate   bool   `json:"hasRate"`
	Overdue   bool   `json:"overdue"`
}

// Analytics builds the Analytics screen for the last months months (0 for
// all time) and keeps this month's net worth for its history.
func (a *API) Analytics(months int) (Analytics, error) {
	if months < 0 {
		return Analytics{}, fmt.Errorf("the range can't be negative")
	}

	storage, err := a.currentStorage()
	if err != nil {
		return Analytics{}, err
	}

	data, err := loadSummaryData(storage)
	if err != nil {
		return Analytics{}, err
	}

	now := time.Now()
	sum := summary.Compute(data, now)
	if err := keepSnapshot(storage, sum, now); err != nil {
		return Analytics{}, err
	}

	history, err := netWorthHistory(storage, data, now)
	if err != nil {
		return Analytics{}, err
	}

	// Like Statistics, real numbers: archived categories count.
	rows, _ := cashflow.MonthlyOverview(data.Cashflows)
	avg := analytics.Average(analytics.Window(rows, now, headlineMonths))
	stts := data.Settings

	result := Analytics{
		BaseCurrency:    stts.BaseCurrencyLabel(),
		Months:          months,
		AveragedMonths:  avg.Months,
		AverageIncome:   avg.IncomeCents,
		AverageExpense:  avg.ExpenseCents,
		SavingsRate:     avg.SavingsRate,
		HasSavingsRate:  avg.HasRate,
		Accounts:        sum.AccountsCents,
		Committed:       analytics.CommittedPerMonth(data.Subscriptions, stts),
		NetWorth:        sum.NetWorthCents(),
		NetWorthHistory: make([]NetWorthPoint, 0, len(history)),
		Spending:        make([]CategorySpending, 0),
		Monthly:         monthTotals(rows, now, months),
		Forecast:        make([]ForecastItem, 0),
		ForecastDays:    forecastDays,
		TypicalIncome:   avg.IncomeCents * forecastDays / 30,
	}

	if avg.ExpenseCents > 0 {
		result.RunwayMonths = float64(sum.AccountsCents) / float64(avg.ExpenseCents)
		result.HasRunway = true
		result.FixedShare = float64(result.Committed) / float64(avg.ExpenseCents) * 100
		result.HasFixedShare = true
	}

	for _, point := range history {
		result.NetWorthHistory = append(result.NetWorthHistory, NetWorthPoint{Month: point.Month.Format(monthLayout), Cents: point.Cents, Estimated: point.Estimated})
	}

	if start, ok := yearStart(history, now); ok {
		result.Change = result.NetWorth - start.Cents
		if start.Estimated {
			result.Change = analytics.Snapshot(sum, now).OwnedCents - start.Cents
		}

		result.HasChange = true
		result.ChangeSince = start.Month.Format(monthLayout)
		result.ChangeEstimated = start.Estimated
	}

	for _, share := range analytics.SpendingByCategory(data.Cashflows, now, months) {
		result.Spending = append(result.Spending, CategorySpending(share))
	}

	for _, due := range analytics.Forecast(data, now, forecastDays) {
		result.Forecast = append(result.Forecast, ForecastItem{
			Date:      due.Date.Format(dayLayout),
			Kind:      due.Kind,
			Name:      due.Name,
			Incoming:  due.Incoming,
			Currency:  due.Currency,
			IsBase:    due.Currency == "" || stts.IsBase(due.Currency),
			Cents:     due.Cents,
			BaseCents: due.BaseCents,
			HasRate:   due.HasRate,
			Overdue:   due.Overdue,
		})

		switch {
		case !due.HasRate:
			result.ForecastMissing++
		case due.Incoming:
			result.ForecastIn += due.BaseCents
		default:
			result.ForecastOut += due.BaseCents
		}
	}

	return result, nil
}

// keepSnapshot keeps net worth now as this month's, so the history has it
// once the month is over.
func keepSnapshot(storage StorageWorker, sum summary.Summary, now time.Time) error {
	snapshot := analytics.Snapshot(sum, now)
	if err := storage.SaveNetWorthSnapshot(&snapshot); err != nil {
		return err
	}

	return nil
}

func netWorthHistory(storage StorageWorker, data summary.Data, now time.Time) ([]analytics.MonthValue, error) {
	h := analytics.History{Accounts: data.Accounts, Assets: data.Assets, Settings: data.Settings}

	var err error
	if h.Snapshots, err = storage.LoadNetWorthSnapshots(); err != nil {
		return nil, err
	}

	if h.AccountLogs, err = storage.LoadAllAccountValueLogs(); err != nil {
		return nil, err
	}

	if h.AssetLogs, err = storage.LoadAllAssetValueLogs(); err != nil {
		return nil, err
	}

	return analytics.NetWorthHistory(h, now), nil
}

// yearStart is net worth as the year began: last December's, else the first
// month this year there's a value for, unless that's this month.
func yearStart(history []analytics.MonthValue, now time.Time) (analytics.MonthValue, bool) {
	december := analytics.MonthStart(now)
	december = december.AddDate(0, -int(december.Month()), 0)

	for _, point := range history {
		if point.Month.Equal(december) {
			return point, true
		}

		if point.Month.After(december) {
			return point, !point.Month.Equal(analytics.MonthStart(now))
		}
	}

	return analytics.MonthValue{}, false
}

// monthTotals is income and expense for each of the last months months (all
// since the first entry when 0), the running one included, oldest first.
func monthTotals(rows []cashflow.CashflowMonthlyOverviewRow, now time.Time, months int) []MonthTotals {
	byMonth := map[string]cashflow.CashflowMonthlyOverviewRow{}
	for _, row := range rows {
		byMonth[row.Month.Format(monthLayout)] = row
	}

	last := cashflow.MonthStart(now)
	first := last
	if months > 0 {
		first = last.AddDate(0, -(months - 1), 0)
	} else if len(rows) > 0 && rows[0].Month.Before(last) {
		first = rows[0].Month
	}

	totals := make([]MonthTotals, 0)
	for month := first; !month.After(last); month = month.AddDate(0, 1, 0) {
		row := byMonth[month.Format(monthLayout)]
		totals = append(totals, MonthTotals{Month: month.Format(monthLayout), IncomeCents: row.IncomeBase, ExpenseCents: row.ExpenseBase})
	}

	return totals
}
