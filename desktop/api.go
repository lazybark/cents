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
	"github.com/lazybark/cents/flows/settings"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type StorageWorker interface {
	LoadAccounts() ([]account.Account, error)
	LoadAppSettings() (settings.AppSettings, error)
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

type AccountBalance struct {
	Name              string `json:"name"`
	Currency          string `json:"currency"`
	BalanceCents      int64  `json:"balanceCents"`
	BaseCents         int64  `json:"baseCents"`
	HasRate           bool   `json:"hasRate"`
	IgnoreInSummaries bool   `json:"ignoreInSummaries"`
}

type Balance struct {
	BaseCurrency string           `json:"baseCurrency"`
	TotalCents   int64            `json:"totalCents"`
	IgnoredCount int              `json:"ignoredCount"`
	MissingRates int              `json:"missingRates"`
	Accounts     []AccountBalance `json:"accounts"`
	LoadedAt     time.Time        `json:"loadedAt"`
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

// Balance reads accounts fresh from storage on every call, so changes made
// in the TUI (or elsewhere) show up after a refresh.
func (a *API) Balance() (Balance, error) {
	a.mu.Lock()
	storage := a.storage
	a.mu.Unlock()

	if storage == nil {
		return Balance{}, errNoDatabase
	}

	stts, err := storage.LoadAppSettings()
	if err != nil {
		return Balance{}, fmt.Errorf("failed to load settings: %w", err)
	}

	accounts, err := storage.LoadAccounts()
	if err != nil {
		return Balance{}, fmt.Errorf("failed to load accounts: %w", err)
	}

	result := Balance{
		BaseCurrency: stts.BaseCurrencyLabel(),
		TotalCents:   account.SumInBaseCents(accounts, stts),
		Accounts:     make([]AccountBalance, 0, len(accounts)),
		LoadedAt:     time.Now(),
	}

	for _, acct := range accounts {
		baseCents, ok := stts.ConvertToBaseCents(acct.Currency, acct.BalanceCents)

		if acct.IgnoreInSummaries {
			result.IgnoredCount++
		} else if !ok {
			result.MissingRates++
		}

		result.Accounts = append(result.Accounts, AccountBalance{
			Name:              acct.Name,
			Currency:          acct.Currency,
			BalanceCents:      acct.BalanceCents,
			BaseCents:         baseCents,
			HasRate:           ok,
			IgnoreInSummaries: acct.IgnoreInSummaries,
		})
	}

	return result, nil
}

func databaseFilters() []runtime.FileFilter {
	return []runtime.FileFilter{{DisplayName: "cents database (*.db)", Pattern: "*.db"}}
}
