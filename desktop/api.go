package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/lazybark/cents/service"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// StorageWorker is a database the app can open: what the service works on,
// plus what the app does with the file.
type StorageWorker interface {
	service.Store
	Close() error
	Encrypted() bool
	ChangePassword(current, next string) error
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

	// Server is a cents server to connect to (with Reconnect) instead of a
	// database file. SaveServer remembers the one connected to, so the next
	// launch connects to it too; picking a database file forgets it.
	Server     *Server
	SaveServer func(Server) error
}

// ErrLocked says a database picked is encrypted: it opens with Unlock.
var ErrLocked = errors.New("the database is encrypted")

// API is bound to the frontend: every exported method becomes callable from
// JS as window.go.desktop.API.<Method>() and returns a Promise. The data
// calls come from the embedded Backend: sw, switched to the service on the
// database open or to a client of the server connected to. The methods here
// are the app's own: picking, unlocking, connecting and saving files.
type API struct {
	service.Backend

	// sw is the Backend, and files the Files, of the database or server in
	// use; svc is the service on the database, which runs the background
	// rate updates too (a server runs its own).
	sw    *service.Switch
	files service.Files
	svc   *service.Service

	ctx  context.Context
	opts Options

	mu      sync.Mutex
	storage StorageWorker
	dbPath  string
	// locked is an encrypted database waiting for its password.
	locked string
	// server is the server in use, or (with connected false) the one to
	// connect to; serverError is why connecting to it failed.
	server      *Server
	connected   bool
	serverError string

	// pick asks where to save backups and exports; nil uses the system's
	// dialogs.
	pick          pickers
	importMu      sync.Mutex
	pendingImport *pendingDatabaseImport
}

// ratesEvent tells the frontend that rates changed in the background.
const ratesEvent = "rates-updated"

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
	// Server is the server in use, or (while not Ready) the one to connect
	// to with Reconnect; ServerError is why connecting failed.
	Server      string `json:"server"`
	ServerError string `json:"serverError"`
}

func newAPI(opts Options) *API {
	a := &API{opts: opts, storage: opts.Storage, dbPath: opts.DBPath, locked: opts.LockedPath}
	a.svc = service.New(func() (service.Store, error) {
		storage, err := a.currentStorage()
		if err != nil {
			return nil, err
		}

		return storage, nil
	})
	a.sw = service.NewSwitch(a.svc, a.svc)
	a.Backend, a.files = a.sw, a.sw
	a.server = opts.Server

	return a
}

func (a *API) startup(ctx context.Context) {
	a.ctx = ctx
	go a.svc.KeepRatesFresh(ctx, func() { runtime.EventsEmit(ctx, ratesEvent) })
}

func (a *API) Status() Status {
	a.mu.Lock()
	status := Status{Ready: a.storage != nil || a.connected, DBPath: a.dbPath, Note: a.opts.SetupNote, Locked: a.locked != "", LockedPath: a.locked, ServerError: a.serverError}
	if a.server != nil {
		status.Server = a.server.URL
	}

	storage := a.storage
	a.mu.Unlock()

	if storage != nil {
		status.Encrypted = storage.Encrypted()
	}

	if status.Ready {
		if info, err := a.Info(); err == nil {
			status.NeedsCurrencies = info.NeedsCurrencies
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
	a.server, a.connected, a.serverError = nil, false, ""
	a.sw.To(a.svc, a.svc)
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
		return nil, service.ErrNoDatabase
	}

	return a.storage, nil
}

func databaseFilters() []runtime.FileFilter {
	return []runtime.FileFilter{{DisplayName: "cents database (*.db)", Pattern: "*.db"}}
}
