package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/debt"
	"github.com/lazybark/cents/flows/goal"
	"github.com/lazybark/cents/flows/invoice"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/subscription"
	"github.com/lazybark/cents/flows/tax"
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
	DeleteCashflow(id uint) error
	CreateSubscription(entry *subscription.Subscription) error
	SaveSubscription(entry *subscription.Subscription) error
	DeleteSubscription(id uint) error
	CreateAccount(entry *account.Account) error
	UpdateAccountAmount(id uint, amountCents int64, ignoreInSummaries bool, updatedAt time.Time) error
	DeleteAccount(id uint) error
	LoadAccountValueLogs(accountID uint) ([]account.AccountValueLog, error)
	UpsertAccountValueLog(accountID uint, day time.Time, valueCents int64) error
	SaveSettingRecord(entry *settings.SettingRecord) error
	SaveSettingCurrency(entry *settings.SettingCurrency) error
	SaveSettingPaymentMethod(entry *settings.SettingPaymentMethod) error
	SaveSettingTaxType(entry *settings.SettingTaxType) error
	SaveSettingIncomeCategory(entry *settings.SettingIncomeCategory) error
	SaveSettingExpenseCategory(entry *settings.SettingExpenseCategory) error
	DeleteSetting(targetType string, id uint) error
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
	// remembers it, returning its storage and resolved path.
	Choose func(path string, create bool) (StorageWorker, string, error)
}

// API is bound to the frontend: every exported method becomes callable from
// JS as window.go.desktop.API.<Method>() and returns a Promise.
type API struct {
	ctx  context.Context
	opts Options

	mu      sync.Mutex
	storage StorageWorker
	dbPath  string
}

type Status struct {
	Ready  bool   `json:"ready"`
	DBPath string `json:"dbPath"`
	Note   string `json:"note"`
}

var errNoDatabase = errors.New("no database chosen yet")

func newAPI(opts Options) *API {
	return &API{opts: opts, storage: opts.Storage, dbPath: opts.DBPath}
}

func (a *API) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *API) Status() Status {
	a.mu.Lock()
	defer a.mu.Unlock()

	return Status{Ready: a.storage != nil, DBPath: a.dbPath, Note: a.opts.SetupNote}
}

// CreateDatabase asks where to create a new database. It reports false when
// the user cancelled the dialog.
func (a *API) CreateDatabase() (bool, error) {
	dir, name := filepath.Split(a.opts.CreatePath)
	if dir != "" {
		// The dialog falls back to an arbitrary folder if this one is missing.
		_ = os.MkdirAll(dir, 0o700)
	}

	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:                "Create a new cents database",
		DefaultDirectory:     dir,
		DefaultFilename:      name,
		Filters:              databaseFilters(),
		CanCreateDirectories: true,
	})
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

	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Open a cents database",
		DefaultDirectory: dir,
		DefaultFilename:  name,
		Filters:          databaseFilters(),
	})
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
	if err != nil {
		return false, err
	}

	a.mu.Lock()
	a.storage = storage
	a.dbPath = dbPath
	a.mu.Unlock()

	return true, nil
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
