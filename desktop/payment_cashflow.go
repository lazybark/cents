package desktop

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/settings"
)

// PaymentCashflow asks for a debt or credit payment to be added to incomes
// and expenses too, on the payment's day: as an expense, or as an income
// for a debt owed to the user. Category is required then; Account is
// optional.
type PaymentCashflow struct {
	Add      bool   `json:"add"`
	Category string `json:"category"`
	Account  string `json:"account"`
}

// cashflowOptions are the categories and accounts such an entry can use.
func cashflowOptions(stts settings.AppSettings, accounts []account.Account) CashflowOptions {
	return CashflowOptions{
		Currencies:        stts.CurrencyOptions(),
		Rates:             currencyRates(stts),
		IncomeCategories:  stts.IncomeCategoryOptions(),
		ExpenseCategories: stts.ExpenseCategoryOptions(),
		Accounts:          accountNames(accounts),
	}
}

// paymentEntry builds the income or expense for a payment of cents in
// currency, made on date (YYYY-MM-DD; empty for today), or nil when it
// wasn't asked for. Its rate is the currency's in settings now, as for any
// new entry.
func paymentEntry(req PaymentCashflow, stts settings.AppSettings, accounts []account.Account, isIncome bool, currency string, cents int64, date, comment string, now time.Time) (*cashflow.CashflowEntry, error) {
	if !req.Add {
		return nil, nil
	}

	kind := "an expense"
	if isIncome {
		kind = "an income"
	}

	if cents <= 0 {
		return nil, fmt.Errorf("only a payment can be added as %s, not one taken back", kind)
	}

	day := strings.TrimSpace(date)
	if day == "" {
		day = now.Format(logDateLayout)
	}

	// Stored as UTC midnight, like every other entry's date.
	entryDate, err := time.Parse(logDateLayout, day)
	if err != nil {
		return nil, errors.New("date must use YYYY-MM-DD format")
	}

	accountName := strings.TrimSpace(req.Account)
	if accountName != "" {
		var ok bool
		if accountName, ok = matchOption(accountNames(accounts), accountName); !ok {
			return nil, fmt.Errorf("unknown account %q", req.Account)
		}
	}

	categories := stts.ExpenseCategoryOptions()
	if isIncome {
		categories = stts.IncomeCategoryOptions()
	}

	rate, _ := stts.EntryRate(currency, "")
	entry, err := cashflow.New(isIncome, currency, cents, rate, entryDate, req.Category, categories, accountName, comment, now)
	if err != nil {
		return nil, fmt.Errorf("can't add it as %s: %w", kind, err)
	}

	return &entry, nil
}

// paymentComment reads like "Payment to Bank: first one".
func paymentComment(what, note string) string {
	if note = strings.TrimSpace(note); note != "" {
		return what + ": " + note
	}

	return what
}
