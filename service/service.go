// Package service is what cents does, whatever the client: the desktop app
// calls it in-process, the server exposes it over REST, and client.Client
// calls that server. Service and client.Client both implement Backend, so
// either can stand in for the other.
package service

import (
	"errors"
	"sync"
	"time"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/analytics"
	"github.com/lazybark/cents/flows/asset"
	"github.com/lazybark/cents/flows/budget"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/credit"
	"github.com/lazybark/cents/flows/debt"
	"github.com/lazybark/cents/flows/goal"
	"github.com/lazybark/cents/flows/invoice"
	"github.com/lazybark/cents/flows/note"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/subscription"
	"github.com/lazybark/cents/flows/tag"
	"github.com/lazybark/cents/flows/tax"
	"github.com/lazybark/cents/rates"
)

// Store is the database the service works on.
type Store interface {
	LoadAppSettings() (settings.AppSettings, error)
	LoadAccounts() ([]account.Account, error)
	LoadSubscriptions() ([]subscription.Subscription, error)
	LoadDebts() ([]debt.Debt, error)
	LoadGoals() ([]goal.Goal, error)
	LoadTaxes() ([]tax.Tax, error)
	LoadInvoices() ([]invoice.Invoice, error)
	LoadCashflows() ([]cashflow.CashflowEntry, error)
	CreateCashflow(entry *cashflow.CashflowEntry) error
	SaveCashflows(entries []cashflow.CashflowEntry) error
	DeleteCashflow(id uint) error
	CreateSubscription(entry *subscription.Subscription) error
	SaveSubscription(entry *subscription.Subscription) error
	DeleteSubscription(id uint) error
	LoadSubscriptionPayments(subscriptionID uint) ([]subscription.SubscriptionPayment, error)
	AddSubscriptionPayment(entry *subscription.Subscription, payment *subscription.SubscriptionPayment) error
	DeleteSubscriptionPayment(entry *subscription.Subscription, paymentID uint) error
	CreateAccount(entry *account.Account) error
	UpdateAccountAmount(id uint, amountCents int64, ignoreInSummaries bool, updatedAt time.Time) error
	DeleteAccount(id uint) error
	SetAccountArchived(id uint, archived bool) error
	LoadAccountValueLogs(accountID uint) ([]account.AccountValueLog, error)
	UpsertAccountValueLog(accountID uint, day time.Time, valueCents int64) error
	DeleteAccountValueLog(accountID uint, logID uint) error
	SaveSettingRecord(entry *settings.SettingRecord) error
	SaveSettingCurrency(entry *settings.SettingCurrency) error
	SaveSettingPaymentMethod(entry *settings.SettingPaymentMethod) error
	SaveSettingTaxType(entry *settings.SettingTaxType) error
	SaveSettingIncomeCategory(entry *settings.SettingIncomeCategory) error
	SaveSettingExpenseCategory(entry *settings.SettingExpenseCategory) error
	DeleteSetting(targetType string, id uint) error
	SaveSettingRecords(records []settings.SettingRecord) error
	SaveRates(currencies []settings.SettingCurrency, records []settings.SettingRecord) error
	CreateDebt(entry *debt.Debt) error
	SaveDebt(entry *debt.Debt) error
	CreateDebtLog(entry *debt.DebtLog) error
	LoadDebtLogs(debtID uint) ([]debt.DebtLog, error)
	DeleteDebtLog(entry *debt.Debt, logID uint) error
	AddDebtPayment(entry *debt.Debt, log *debt.DebtLog, cash *cashflow.CashflowEntry) error
	DeleteDebt(id uint) error
	CreateTax(entry *tax.Tax) error
	SaveTax(entry *tax.Tax) error
	CreateTaxLog(entry *tax.TaxLog) error
	LoadTaxLogs(taxID uint) ([]tax.TaxLog, error)
	DeleteTax(id uint) error
	CreateGoal(entry *goal.Goal) error
	SaveGoal(entry *goal.Goal) error
	CreateGoalLog(entry *goal.GoalLog) error
	LoadGoalLogs(goalID uint) ([]goal.GoalLog, error)
	DeleteGoalLog(entry *goal.Goal, logID uint) error
	DeleteGoal(id uint) error
	CreateInvoice(entry *invoice.Invoice) error
	SaveInvoice(entry *invoice.Invoice) error
	DeleteInvoice(id uint) error
	SaveInvoicePaid(entry *invoice.Invoice, cash *cashflow.CashflowEntry, unlink uint) error
	LoadAssets() ([]asset.Asset, error)
	CreateAsset(entry *asset.Asset) error
	SaveAsset(entry *asset.Asset) error
	DeleteAsset(id uint) error
	LoadAssetValueLogs(assetID uint) ([]asset.AssetValueLog, error)
	UpsertAssetValueLog(assetID uint, day time.Time, valueCents int64) error
	DeleteAssetValueLog(assetID uint, logID uint) error
	LoadCredits() ([]credit.Credit, error)
	CreateCredit(entry *credit.Credit) error
	SaveCredit(entry *credit.Credit) error
	DeleteCredit(id uint) error
	LoadCreditLogs(creditID uint) ([]credit.CreditLog, error)
	AddCreditLog(entry *credit.Credit, log *credit.CreditLog, cash *cashflow.CashflowEntry) error
	DeleteCreditLog(entry *credit.Credit, logID uint) error
	LoadNetWorthSnapshots() ([]analytics.NetWorthSnapshot, error)
	SaveNetWorthSnapshot(entry *analytics.NetWorthSnapshot) error
	SetNetWorthSnapshot(entry *analytics.NetWorthSnapshot) error
	DeleteNetWorthSnapshot(month time.Time) error
	LoadAllAccountValueLogs() ([]account.AccountValueLog, error)
	LoadAllAssetValueLogs() ([]asset.AssetValueLog, error)
	LoadAllGoalLogs() ([]goal.GoalLog, error)
	LoadAllCreditLogs() ([]credit.CreditLog, error)
	LoadRateRecords() ([]settings.RateRecord, error)
	LoadAllDebtLogs() ([]debt.DebtLog, error)
	LoadAllTaxLogs() ([]tax.TaxLog, error)
	Backup(path string) error
	LoadTags() ([]tag.Tag, error)
	TagUsage() (map[string]int, error)
	SaveTag(entry *tag.Tag) error
	DeleteTag(id uint) error
	LoadMonthNotes() ([]note.MonthNote, error)
	SaveMonthNote(month time.Time, text string) error
	Merge(kind string, from, into uint) (int64, error)
	LoadBudgets() ([]budget.Budget, error)
	CreateBudget(entry *budget.Budget) error
	SaveBudget(entry *budget.Budget) error
	DeleteBudget(id uint) error
}

// Service implements Backend (and Files) on a Store.
type Service struct {
	// store returns the database to use; the desktop app can switch
	// databases while running, so it's asked for on every call.
	store func() (Store, error)

	mu sync.Mutex
	// rates fetches exchange rates; refreshing lets one refresh run at a
	// time, and ratesError is the last one's error.
	rates      rates.Fetcher
	refreshing sync.Mutex
	ratesError string
}

var (
	_ Backend = (*Service)(nil)
	_ Files   = (*Service)(nil)
)

// New makes a service on whatever database store returns at the time of
// each call.
func New(store func() (Store, error)) *Service {
	return &Service{store: store, rates: rates.NewHTTP()}
}

// On makes a service on one database.
func On(store Store) *Service {
	return New(func() (Store, error) { return store, nil })
}

// WithRates makes the service fetch exchange rates from f (tests keep them
// offline this way).
func (s *Service) WithRates(f rates.Fetcher) *Service {
	s.rates = f
	return s
}

// ErrNoDatabase is what a call fails with while there's no database to
// work on.
var ErrNoDatabase = errors.New("no database chosen yet")

func (s *Service) currentStorage() (Store, error) {
	storage, err := s.store()
	if err == nil && storage == nil {
		err = ErrNoDatabase
	}

	return storage, err
}

// Info is what a client needs to know first.
type Info struct {
	BaseCurrency string `json:"baseCurrency"`
	// NeedsCurrencies is set for a new database whose currencies haven't
	// been picked yet (see SetupCurrencies).
	NeedsCurrencies bool `json:"needsCurrencies"`
}

func (s *Service) Info() (Info, error) {
	_, stts, err := s.storageAndSettings()
	if err != nil {
		return Info{}, err
	}

	return Info{BaseCurrency: stts.BaseCurrencyLabel(), NeedsCurrencies: !stts.CurrenciesSetUp}, nil
}
