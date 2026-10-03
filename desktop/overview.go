package desktop

import (
	"fmt"
	"time"

	"github.com/lazybark/cents/flows/subscription"
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
	MonthlyObligations   int64 `json:"monthlyObligationsCents"`
	YearlyObligations    int64 `json:"yearlyObligationsCents"`
	Accounts             int64 `json:"accountsCents"`
	Property             int64 `json:"propertyCents"`
	Investments          int64 `json:"investmentsCents"`

	DebtsToMe        int64 `json:"debtsToMeCents"`
	DebtsByMe        int64 `json:"debtsByMeCents"`
	UnpaidTaxes      int64 `json:"unpaidTaxesCents"`
	Credits          int64 `json:"creditsCents"`
	InvoicesToMe     int64 `json:"invoicesToMeCents"`
	InvoicesByMe     int64 `json:"invoicesByMeCents"`
	GoalsAccumulated int64 `json:"goalsAccumulatedCents"`
	GoalsTarget      int64 `json:"goalsTargetCents"`

	// Upcoming are the active subscriptions paid soon, soonest first.
	Upcoming []SubscriptionRow `json:"upcoming"`

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

	upcoming := make([]SubscriptionRow, 0)
	for _, sub := range subscription.Upcoming(data.Subscriptions, now) {
		upcoming = append(upcoming, subscriptionRow(sub, data.Settings, now))
	}

	return Overview{
		Upcoming:             upcoming,
		BaseCurrency:         data.Settings.BaseCurrencyLabel(),
		Month:                month,
		NetWorth:             sum.NetWorthCents(),
		MonthlyNet:           sum.MonthlyNetCents,
		MonthlySubscriptions: sum.MonthlySubscriptionsCents,
		YearlySubscriptions:  sum.YearlySubscriptionsCents,
		MonthlyObligations:   sum.MonthlyObligationsCents,
		YearlyObligations:    sum.YearlyObligationsCents,
		Accounts:             sum.AccountsCents,
		Property:             sum.PropertyCents,
		Investments:          sum.InvestmentsCents,
		DebtsToMe:            sum.DebtsToMeCents,
		DebtsByMe:            sum.DebtsByMeCents,
		UnpaidTaxes:          sum.UnpaidTaxesCents,
		Credits:              sum.CreditsCents,
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

	if data.Credits, err = storage.LoadCredits(); err != nil {
		return data, err
	}

	return data, nil
}
