package desktop

import (
	"fmt"
	"time"

	"github.com/lazybark/cents/summary"
)

// Overview mirrors the TUI home screen: financial summary and obligations,
// all in the base currency, plus net worth.
type Overview struct {
	BaseCurrency string    `json:"baseCurrency"`
	Month        time.Time `json:"month"`
	NetWorth     int64     `json:"netWorthCents"`

	MonthlyNet           int64 `json:"monthlyNetCents"`
	MonthlySubscriptions int64 `json:"monthlySubscriptionsCents"`
	YearlySubscriptions  int64 `json:"yearlySubscriptionsCents"`
	Accounts             int64 `json:"accountsCents"`
	Property             int64 `json:"propertyCents"`
	Investments          int64 `json:"investmentsCents"`

	DebtsToMe        int64 `json:"debtsToMeCents"`
	DebtsByMe        int64 `json:"debtsByMeCents"`
	UnpaidTaxes      int64 `json:"unpaidTaxesCents"`
	InvoicesToMe     int64 `json:"invoicesToMeCents"`
	InvoicesByMe     int64 `json:"invoicesByMeCents"`
	GoalsAccumulated int64 `json:"goalsAccumulatedCents"`
	GoalsTarget      int64 `json:"goalsTargetCents"`

	LoadedAt time.Time `json:"loadedAt"`
}

// Overview computes the home screen for the current month.
func (a *API) Overview() (Overview, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return Overview{}, err
	}

	data, err := loadSummaryData(storage)
	if err != nil {
		return Overview{}, err
	}

	now := time.Now()
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	sum := summary.Compute(data, month)

	return Overview{
		BaseCurrency:         data.Settings.BaseCurrencyLabel(),
		Month:                month,
		NetWorth:             sum.NetWorthCents(),
		MonthlyNet:           sum.MonthlyNetCents,
		MonthlySubscriptions: sum.MonthlySubscriptionsCents,
		YearlySubscriptions:  sum.YearlySubscriptionsCents,
		Accounts:             sum.AccountsCents,
		Property:             sum.PropertyCents,
		Investments:          sum.InvestmentsCents,
		DebtsToMe:            sum.DebtsToMeCents,
		DebtsByMe:            sum.DebtsByMeCents,
		UnpaidTaxes:          sum.UnpaidTaxesCents,
		InvoicesToMe:         sum.InvoicesToMeCents,
		InvoicesByMe:         sum.InvoicesByMeCents,
		GoalsAccumulated:     sum.GoalsAccumulatedCents,
		GoalsTarget:          sum.GoalsTargetCents,
		LoadedAt:             now,
	}, nil
}

func loadSummaryData(storage StorageWorker) (summary.Data, error) {
	var (
		data summary.Data
		err  error
	)

	if data.Settings, err = storage.LoadAppSettings(); err != nil {
		return data, fmt.Errorf("failed to load settings: %w", err)
	}

	if data.Accounts, err = storage.LoadAccounts(); err != nil {
		return data, fmt.Errorf("failed to load accounts: %w", err)
	}

	if data.Subscriptions, err = storage.LoadSubscriptions(); err != nil {
		return data, fmt.Errorf("failed to load subscriptions: %w", err)
	}

	if data.Debts, err = storage.LoadDebts(); err != nil {
		return data, fmt.Errorf("failed to load debts: %w", err)
	}

	if data.Goals, err = storage.LoadGoals(); err != nil {
		return data, fmt.Errorf("failed to load goals: %w", err)
	}

	if data.Taxes, err = storage.LoadTaxes(); err != nil {
		return data, fmt.Errorf("failed to load taxes: %w", err)
	}

	if data.Invoices, err = storage.LoadInvoices(); err != nil {
		return data, fmt.Errorf("failed to load invoices: %w", err)
	}

	if data.Cashflows, err = storage.LoadCashflows(); err != nil {
		return data, fmt.Errorf("failed to load cashflows: %w", err)
	}

	if data.Assets, err = storage.LoadAssets(); err != nil {
		return data, err
	}

	return data, nil
}
