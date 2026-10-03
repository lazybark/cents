package desktop

import (
	"github.com/lazybark/cents/dates"
	"time"

	"github.com/lazybark/cents/flows/settings"
)

// Debts and taxes both track an amount paid that moves through logged
// payments; these types are shared by their API methods.

type Progress struct {
	PaidCents  int64 `json:"paidCents"`
	TotalCents int64 `json:"totalCents"`
}

type PaymentLog struct {
	ID         uint   `json:"id"`
	When       string `json:"when"`
	DeltaCents int64  `json:"deltaCents"`
	Note       string `json:"note"`
	// CashflowID is the expense or income added with the payment, 0 for none.
	CashflowID uint `json:"cashflowId"`
}

// PaymentInput adds Delta (signed, like "-10") to an amount paid on Date
// (YYYY-MM-DD, empty for now).
type PaymentInput struct {
	ID    uint   `json:"id"`
	Delta string `json:"delta"`
	Date  string `json:"date"`
	Note  string `json:"note"`
	// Cashflow adds the payment to incomes and expenses too (debts only).
	Cashflow PaymentCashflow `json:"cashflow"`
}

// CurrencyRate is a currency a new record can use, with its rate to the
// base currency in settings now (1 for the base currency itself), to start
// the record's own rate with.
type CurrencyRate struct {
	Name string  `json:"name"`
	Rate float64 `json:"rate"`
}

// currencyRates lists the base currency and every configured currency
// with its current rate.
func currencyRates(stts settings.AppSettings) []CurrencyRate {
	options := stts.CurrencyOptions()
	result := make([]CurrencyRate, 0, len(options))
	for _, name := range options {
		rate, _ := stts.EntryRate(name, "")
		result = append(result, CurrencyRate{Name: name, Rate: rate})
	}

	return result
}

// CreatedIn names the list a new debt or tax shows up in.
type CreatedIn struct {
	Mode string `json:"mode"`
}

// formatDay shows a calendar date as the day it was saved (see dates.Day).
func formatDay(value time.Time) string {
	return dates.Text(value)
}

func formatOptionalDay(value *time.Time) string {
	if value == nil {
		return ""
	}

	return formatDay(*value)
}

// overdue reports whether an unpaid item's due date is before today.
func overdue(due *time.Time, paid bool, now time.Time) bool {
	if due == nil || paid {
		return false
	}

	return formatDay(*due) < now.Format(logDateLayout)
}
