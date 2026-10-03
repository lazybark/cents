package analytics

import (
	"sort"
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/credit"
	"github.com/lazybark/cents/flows/debt"
	"github.com/lazybark/cents/flows/invoice"
	"github.com/lazybark/cents/flows/settings"
)

// RatePoint is a currency's rate to the base currency on a day.
type RatePoint struct {
	Day  time.Time
	Rate float64
}

// Rates is what's known of each currency's rate over time, by currency
// (lower-cased), oldest first.
type Rates struct {
	base   string
	points map[string][]RatePoint
}

// RateSources are where past rates come from: the rate history kept since
// it started, and the rates recorded with dated records before that.
type RateSources struct {
	Records   []settings.RateRecord
	Cashflows []cashflow.CashflowEntry
	Debts     []debt.Debt
	Credits   []credit.Credit
	Invoices  []invoice.Invoice
	Settings  settings.AppSettings
}

func currencyKey(currency string) string {
	return strings.ToLower(strings.TrimSpace(currency))
}

// RateHistory gathers the known rates. On a day with several, the rate
// history wins over records, and today's rate in settings over both.
func RateHistory(src RateSources, now time.Time) Rates {
	stts := src.Settings
	r := Rates{base: currencyKey(stts.BaseCurrencyLabel()), points: map[string][]RatePoint{}}
	add := func(currency string, day time.Time, rate float64) {
		key := currencyKey(currency)
		if rate <= 0 || key == "" || key == r.base {
			return
		}

		r.points[key] = append(r.points[key], RatePoint{Day: dates.Day(day), Rate: rate})
	}

	for _, e := range src.Cashflows {
		add(e.Currency, e.EntryDate, e.RateToBase)
	}

	for _, d := range src.Debts {
		add(d.Currency, d.DebtCreatedAt, d.RateToBase)
	}

	for _, c := range src.Credits {
		add(c.Currency, c.StartDate, c.RateToBase)
	}

	for _, inv := range src.Invoices {
		if inv.InvoiceDate != nil {
			add(inv.Currency, *inv.InvoiceDate, inv.RateToBase)
		}
	}

	for _, rec := range src.Records {
		if currencyKey(rec.Base) == r.base {
			add(rec.Currency, rec.Day, rec.RateToBase)
		}
	}

	for _, c := range stts.Currencies {
		add(c.CurrencyName, now, c.RateToBase)
	}

	for key := range r.points {
		sort.SliceStable(r.points[key], func(i, j int) bool { return r.points[key][i].Day.Before(r.points[key][j].Day) })
	}

	return r
}

// At is currency's rate on day: the latest known by then; 1 for the base
// currency. ok is false when no rate was known yet.
func (r Rates) At(currency string, day time.Time) (float64, bool) {
	key := currencyKey(currency)
	if key == r.base {
		return 1, true
	}

	day = dates.Day(day)
	var rate float64
	found := false
	for _, p := range r.points[key] {
		if p.Day.After(day) {
			break
		}

		rate, found = p.Rate, true
	}

	return rate, found
}
