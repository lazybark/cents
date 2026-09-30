package desktop

import (
	"time"

	"github.com/lazybark/cents/flows/analytics"
	"github.com/lazybark/cents/flows/asset"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/summary"
)

// Commitments is what's committed each month against the average income.
type Commitments struct {
	ObligationsCents    int64   `json:"obligationsCents"`
	CreditPaymentsCents int64   `json:"creditPaymentsCents"`
	TotalCents          int64   `json:"totalCents"`
	Share               float64 `json:"share"`
	HasShare            bool    `json:"hasShare"`
}

// UsualComparison is a month's spending by category against the usual.
// For the running month, DaysIn of DaysInMonth have gone by.
type UsualComparison struct {
	Month           string           `json:"month"`
	UsualMonths     int              `json:"usualMonths"`
	TotalCents      int64            `json:"totalCents"`
	UsualTotalCents int64            `json:"usualTotalCents"`
	DaysIn          int              `json:"daysIn"`
	DaysInMonth     int              `json:"daysInMonth"`
	Items           []CategoryChange `json:"items"`
}

type CategoryChange struct {
	Category    string  `json:"category"`
	Cents       int64   `json:"cents"`
	UsualCents  int64   `json:"usualCents"`
	DiffCents   int64   `json:"diffCents"`
	DiffPercent float64 `json:"diffPercent"`
	HasUsual    bool    `json:"hasUsual"`
	Unusual     bool    `json:"unusual"`
}

// ExposureRow is what's held in one currency, in it and in the base.
type ExposureRow struct {
	Currency       string  `json:"currency"`
	IsBase         bool    `json:"isBase"`
	OwnedCents     int64   `json:"ownedCents"`
	OwedToMeCents  int64   `json:"owedToMeCents"`
	OwedByMeCents  int64   `json:"owedByMeCents"`
	NetCents       int64   `json:"netCents"`
	OwnedBaseCents int64   `json:"ownedBaseCents"`
	NetBaseCents   int64   `json:"netBaseCents"`
	HasRate        bool    `json:"hasRate"`
	Share          float64 `json:"share"`
}

// FXView is the effect of exchange rates over the picked range.
type FXView struct {
	Months     []MonthCents    `json:"months"`
	TotalCents int64           `json:"totalCents"`
	ByCurrency []CurrencyCents `json:"byCurrency"`
	Missing    []string        `json:"missing"`
}

type MonthCents struct {
	Month string `json:"month"`
	Cents int64  `json:"cents"`
}

type CurrencyCents struct {
	Currency string `json:"currency"`
	Cents    int64  `json:"cents"`
}

// AssetGrowthView is property or investments over the picked range, with
// the latest month's figures.
type AssetGrowthView struct {
	Months     []AssetMonth `json:"months"`
	ValueCents int64        `json:"valueCents"`
	CostCents  int64        `json:"costCents"`
	GainCents  int64        `json:"gainCents"`
	HasCost    bool         `json:"hasCost"`
	Count      int          `json:"count"`
}

type AssetMonth struct {
	Month      string `json:"month"`
	ValueCents int64  `json:"valueCents"`
	CostCents  int64  `json:"costCents"`
	GainCents  int64  `json:"gainCents"`
	HasCost    bool   `json:"hasCost"`
}

// GoalPaceRow is how an unfinished goal is going at its recent pace.
type GoalPaceRow struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	Currency       string `json:"currency"`
	LeftCents      int64  `json:"leftCents"`
	TargetDate     string `json:"targetDate"`
	PerMonthCents  int64  `json:"perMonthCents"`
	Projected      string `json:"projected"`
	MonthsLate     int    `json:"monthsLate"`
	NeededPerMonth int64  `json:"neededPerMonthCents"`
	Status         string `json:"status"`
}

// rangeStart is the first month of the picked range: months back from now
// (the running month included), or for all time the first month with an
// entry or a value logged.
func rangeStart(months int, now time.Time, rows []cashflow.CashflowMonthlyOverviewRow, h analytics.History) time.Time {
	current := cashflow.MonthStart(now)
	if months > 0 {
		return current.AddDate(0, -(months - 1), 0)
	}

	first := current
	if len(rows) > 0 && rows[0].Month.Before(first) {
		first = rows[0].Month
	}

	for _, l := range h.AccountLogs {
		if m := cashflow.MonthStart(l.LogDate); m.Before(first) {
			first = m
		}
	}

	for _, l := range h.AssetLogs {
		if m := cashflow.MonthStart(l.LogDate); m.Before(first) {
			first = m
		}
	}

	return first
}

// comparison compares month with the usual. While a month is running
// (running), spending less than usual is expected, so only spending more
// is flagged.
func comparison(entries []cashflow.CashflowEntry, month time.Time, running bool) UsualComparison {
	c := analytics.VersusUsual(entries, month)
	result := UsualComparison{Month: c.Month.Format(monthLayout), UsualMonths: c.UsualMonths, TotalCents: c.TotalCents, UsualTotalCents: c.UsualTotalCents, Items: make([]CategoryChange, 0, len(c.Items))}
	for _, item := range c.Items {
		row := CategoryChange(item)
		row.Unusual = row.Unusual && (!running || row.DiffCents > 0)
		result.Items = append(result.Items, row)
	}

	return result
}

func assetGrowth(data summary.Data, h analytics.History, kind asset.Kind, first, now time.Time) AssetGrowthView {
	view := AssetGrowthView{Months: []AssetMonth{}}
	for _, item := range data.Assets {
		if item.Kind == string(kind) && !item.IgnoreInNetWorth {
			view.Count++
		}
	}

	for _, m := range analytics.AssetGrowth(data.Assets, h.AssetLogs, data.Settings, kind, first, now) {
		view.Months = append(view.Months, AssetMonth{Month: m.Month.Format(monthLayout), ValueCents: m.ValueCents, CostCents: m.CostCents, GainCents: m.GainCents, HasCost: m.HasCost})
	}

	if n := len(view.Months); n > 0 {
		last := view.Months[n-1]
		view.ValueCents, view.CostCents, view.GainCents, view.HasCost = last.ValueCents, last.CostCents, last.GainCents, last.HasCost
	}

	return view
}

// addMore fills in the parts of Analytics beyond the first version.
func addMore(result *Analytics, storage StorageWorker, data summary.Data, h analytics.History, rows []cashflow.CashflowMonthlyOverviewRow, first, now time.Time) error {
	stts := data.Settings

	creditLogs, err := storage.LoadAllCreditLogs()
	if err != nil {
		return err
	}

	c := analytics.CommitmentsPerMonth(data.Subscriptions, data.Credits, creditLogs, stts, now, result.AverageIncome)
	result.Commitments = Commitments(c)

	for i, m := range result.Monthly {
		rate, ok := analytics.SavingsRate(m.IncomeCents, m.ExpenseCents)
		result.Monthly[i].SavingsRate, result.Monthly[i].HasSavingsRate = rate, ok
		result.Monthly[i].FixedCents = min(result.Committed, m.ExpenseCents)
		result.Monthly[i].FlexibleCents = m.ExpenseCents - result.Monthly[i].FixedCents
	}

	result.ThisMonth = comparison(data.Cashflows, now, true)
	local := now.Local()
	result.ThisMonth.DaysIn = local.Day()
	result.ThisMonth.DaysInMonth = time.Date(local.Year(), local.Month()+1, 0, 0, 0, 0, 0, time.Local).Day()
	result.LastMonth = comparison(data.Cashflows, cashflow.MonthStart(now).AddDate(0, -1, 0), false)

	result.Exposure = make([]ExposureRow, 0)
	for _, e := range analytics.CurrencyExposure(data) {
		result.Exposure = append(result.Exposure, ExposureRow{
			Currency: e.Currency, IsBase: stts.IsBase(e.Currency),
			OwnedCents: e.OwnedCents, OwedToMeCents: e.OwedToMeCents, OwedByMeCents: e.OwedByMeCents, NetCents: e.NetCents(),
			OwnedBaseCents: e.OwnedBaseCents, NetBaseCents: e.NetBaseCents, HasRate: e.HasRate, Share: e.Share,
		})
	}

	records, err := storage.LoadRateRecords()
	if err != nil {
		return err
	}

	rates := analytics.RateHistory(analytics.RateSources{Records: records, Cashflows: data.Cashflows, Debts: data.Debts, Credits: data.Credits, Invoices: data.Invoices, Settings: stts}, now)
	fx := analytics.ExchangeEffect(h, rates, first, now)
	result.FX = FXView{Months: make([]MonthCents, 0, len(fx.Months)), TotalCents: fx.TotalCents, ByCurrency: make([]CurrencyCents, 0, len(fx.ByCurrency)), Missing: fx.Missing}
	for _, m := range fx.Months {
		result.FX.Months = append(result.FX.Months, MonthCents{Month: m.Month.Format(monthLayout), Cents: m.Cents})
	}

	for _, c := range fx.ByCurrency {
		result.FX.ByCurrency = append(result.FX.ByCurrency, CurrencyCents(c))
	}

	result.Property = assetGrowth(data, h, asset.KindProperty, first, now)
	result.Investments = assetGrowth(data, h, asset.KindInvestment, first, now)

	goalLogs, err := storage.LoadAllGoalLogs()
	if err != nil {
		return err
	}

	result.Goals = make([]GoalPaceRow, 0)
	for _, p := range analytics.Pace(data.Goals, goalLogs, now) {
		row := GoalPaceRow{ID: p.ID, Name: p.Name, Currency: p.Currency, LeftCents: p.LeftCents, PerMonthCents: p.PerMonthCents, MonthsLate: p.MonthsLate, NeededPerMonth: p.NeededPerMonth, Status: p.Status, TargetDate: formatOptionalDay(p.TargetDate)}
		if p.Projected != nil {
			row.Projected = formatDay(*p.Projected)
		}

		result.Goals = append(result.Goals, row)
	}

	return nil
}
