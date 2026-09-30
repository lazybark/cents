package desktop

import (
	"context"
	"errors"
	"fmt"
	"github.com/lazybark/cents/flows/budget"
	"github.com/lazybark/cents/flows/note"
	"github.com/lazybark/cents/flows/tag"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/analytics"
	"github.com/lazybark/cents/flows/asset"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/credit"
	"github.com/lazybark/cents/flows/debt"
	"github.com/lazybark/cents/flows/goal"
	"github.com/lazybark/cents/flows/invoice"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/subscription"
	"github.com/lazybark/cents/flows/tax"
	"github.com/lazybark/cents/rates"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type StorageWorker interface {
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
	Close() error
	Encrypted() bool
	ChangePassword(current, next string) error
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

// Options configure the desktop app. Storage is nil when no database has
// been chosen yet; the frontend then shows the setup screen.
type Options struct {
	Storage    StorageWorker
	DBPath     string
	SetupNote  string
	CreatePath string
	OpenPath   string
	// Choose opens (or creates) the database picked in a file dialog and
	// remembers it, returning its storage and resolved path; ErrLocked for an
	// encrypted one, which needs Unlock.
	Choose func(path string, create bool) (StorageWorker, string, error)

	// LockedPath is an encrypted database to open once its password is
	// given (with Unlock, which also remembers it).
	LockedPath string
	Unlock     func(path, password string) (StorageWorker, error)
	// Encrypt and Decrypt turn the open database (the file at path) into an
	// encrypted one or back, returning the storage to use from then on.
	Encrypt func(s StorageWorker, path, password string) (StorageWorker, error)
	Decrypt func(s StorageWorker, password string) (StorageWorker, error)
}

// ErrLocked says a database picked is encrypted: it opens with Unlock.
var ErrLocked = errors.New("the database is encrypted")

// API is bound to the frontend: every exported method becomes callable from
// JS as window.go.desktop.API.<Method>() and returns a Promise.
type API struct {
	ctx  context.Context
	opts Options

	mu      sync.Mutex
	storage StorageWorker
	dbPath  string
	// locked is an encrypted database waiting for its password.
	locked string

	// rates fetches exchange rates; refreshing lets one refresh run at a
	// time, and ratesError is the last one's error.
	rates      rates.Fetcher
	refreshing sync.Mutex
	ratesError string

	// pick asks where to save backups and exports; nil uses the system's
	// dialogs.
	pick pickers
}

type Status struct {
	Ready  bool   `json:"ready"`
	DBPath string `json:"dbPath"`
	Note   string `json:"note"`
	// NeedsCurrencies is set for a new database whose currencies haven't
	// been picked yet.
	NeedsCurrencies bool `json:"needsCurrencies"`
	// Locked is set while an encrypted database (LockedPath) waits for its
	// password; Encrypted says the open one is encrypted.
	Locked     bool   `json:"locked"`
	LockedPath string `json:"lockedPath"`
	Encrypted  bool   `json:"encrypted"`
}

var errNoDatabase = errors.New("no database chosen yet")

func newAPI(opts Options) *API {
	return &API{opts: opts, storage: opts.Storage, dbPath: opts.DBPath, locked: opts.LockedPath, rates: rates.NewHTTP()}
}

func (a *API) startup(ctx context.Context) {
	a.ctx = ctx
	go a.keepRatesFresh(ctx)
}

func (a *API) Status() Status {
	a.mu.Lock()
	status := Status{Ready: a.storage != nil, DBPath: a.dbPath, Note: a.opts.SetupNote, Locked: a.locked != "", LockedPath: a.locked}
	storage := a.storage
	a.mu.Unlock()

	if storage != nil {
		status.Encrypted = storage.Encrypted()
		if stts, err := storage.LoadAppSettings(); err == nil {
			status.NeedsCurrencies = !stts.CurrenciesSetUp
		}
	}

	return status
}

// CreateDatabase asks where to create a new database. It reports false when
// the user cancelled the dialog.
func (a *API) CreateDatabase() (bool, error) {
	dir, name := filepath.Split(a.opts.CreatePath)
	// From Settings, start next to the database in use.
	if current := a.Status().DBPath; current != "" {
		dir, name = filepath.Dir(current), "cents.db"
	}

	if dir != "" {
		// The dialog falls back to an arbitrary folder if this one is missing.
		_ = os.MkdirAll(dir, 0o700)
	}

	path, err := a.pickers().SaveFile("Create a new cents database", dir, name)
	if err != nil {
		return false, fmt.Errorf("file dialog failed: %w", err)
	}

	return a.choose(path, true)
}

// OpenDatabase asks for an existing database. It reports false when the user
// cancelled the dialog.
func (a *API) OpenDatabase() (bool, error) {
	dir, name := "", ""
	if a.opts.OpenPath != "" {
		dir, name = filepath.Split(a.opts.OpenPath)
	}

	// From Settings, start in the folder of the database in use.
	if current := a.Status().DBPath; current != "" {
		dir, name = filepath.Dir(current), ""
	}

	path, err := a.pickers().OpenFile("Open a cents database", dir, name)
	if err != nil {
		return false, fmt.Errorf("file dialog failed: %w", err)
	}

	return a.choose(path, false)
}

func (a *API) choose(path string, create bool) (bool, error) {
	if path == "" {
		return false, nil
	}

	storage, dbPath, err := a.opts.Choose(path, create)
	if errors.Is(err, ErrLocked) {
		// Opened once its password is given; the one open stays till then.
		a.mu.Lock()
		a.locked = path
		a.mu.Unlock()
		return true, nil
	}

	if err != nil {
		return false, err
	}

	a.use(storage, dbPath)
	return true, nil
}

// use switches to storage, the database at dbPath, closing the one before
// (which saves it, when encrypted).
func (a *API) use(storage StorageWorker, dbPath string) {
	a.mu.Lock()
	previous := a.storage
	a.storage = storage
	a.dbPath = dbPath
	a.locked = ""
	a.mu.Unlock()

	if previous != nil && previous != storage {
		_ = previous.Close()
	}
}

// currentStorage returns the open storage, or an error before setup is done.
func (a *API) currentStorage() (StorageWorker, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.storage == nil {
		return nil, errNoDatabase
	}

	return a.storage, nil
}

func databaseFilters() []runtime.FileFilter {
	return []runtime.FileFilter{{DisplayName: "cents database (*.db)", Pattern: "*.db"}}
}
