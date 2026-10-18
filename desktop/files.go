package desktop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lazybark/cents/service"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// pickers asks which database to open and where to save things. The Wails one shows the system's
// dialogs; tests stand in for it. An empty path means the user cancelled.
type pickers interface {
	SaveFile(title, dir, name string) (string, error)
	OpenFile(title, dir, name string) (string, error)
	Folder(title, dir string) (string, error)
}

type wailsPickers struct{ api *API }

func (p wailsPickers) SaveFile(title, dir, name string) (string, error) {
	return runtime.SaveFileDialog(p.api.ctx, runtime.SaveDialogOptions{
		Title:                title,
		DefaultDirectory:     dir,
		DefaultFilename:      name,
		Filters:              databaseFilters(),
		CanCreateDirectories: true,
	})
}

func (p wailsPickers) OpenFile(title, dir, name string) (string, error) {
	return runtime.OpenFileDialog(p.api.ctx, runtime.OpenDialogOptions{
		Title:            title,
		DefaultDirectory: dir,
		DefaultFilename:  name,
		Filters:          databaseFilters(),
	})
}

func (p wailsPickers) Folder(title, dir string) (string, error) {
	return runtime.OpenDirectoryDialog(p.api.ctx, runtime.OpenDialogOptions{
		Title:                title,
		DefaultDirectory:     dir,
		CanCreateDirectories: true,
	})
}

func (a *API) pickers() pickers {
	if a.pick != nil {
		return a.pick
	}

	return wailsPickers{api: a}
}

// Settings adds the database file to the service's settings.
func (a *API) Settings() (service.SettingsView, error) {
	view, err := a.Backend.Settings()
	view.DBPath = a.Status().DBPath

	return view, err
}

// ExportOptions adds the database file to the service's export options.
func (a *API) ExportOptions() (service.ExportView, error) {
	view, err := a.Backend.ExportOptions()
	view.DBPath = a.Status().DBPath

	return view, err
}

// saveDir is where saving starts: the folder of the last backup, or else
// the database's.
func (a *API) saveDir() (string, error) {
	view, err := a.Backend.ExportOptions()
	if err != nil {
		return "", err
	}

	// The last backup may have been made on another computer, from a
	// server.
	if dir := filepath.Dir(view.LastBackupPath); view.LastBackupPath != "" && isDir(dir) {
		return dir, nil
	}

	if path := a.Status().DBPath; path != "" {
		return filepath.Dir(path), nil
	}

	return os.UserHomeDir()
}

// BackupDatabase asks where to save a backup and writes it there. Path is
// empty when the dialog was cancelled.
func (a *API) BackupDatabase() (service.BackupResult, error) {
	dir, err := a.saveDir()
	if err != nil {
		return service.BackupResult{}, err
	}

	name := "cents-backup-" + time.Now().Format("2006-01-02") + ".db"
	path, err := a.pickers().SaveFile("Save a backup of the database", dir, name)
	if err != nil {
		return service.BackupResult{}, fmt.Errorf("file dialog failed: %w", err)
	}

	if path == "" {
		return service.BackupResult{}, nil
	}

	// The backup takes the place of the file picked, which mustn't be the
	// database the app has open.
	if filepath.Ext(path) == "" {
		path += ".db"
	}

	if sameFile(path, a.Status().DBPath) {
		return service.BackupResult{}, errors.New("that's the database itself; choose another file for the backup")
	}

	return service.SaveBackup(a.files, path)
}

// ExportCSV asks for a folder and writes the picked datasets as CSV files
// into a new folder in it. Folder is empty when the dialog was cancelled.
func (a *API) ExportCSV(input service.ExportInput) (service.ExportResult, error) {
	// Checked before the dialog, so a mistake doesn't cost a pick.
	if err := input.Check(); err != nil {
		return service.ExportResult{}, err
	}

	dir, err := a.saveDir()
	if err != nil {
		return service.ExportResult{}, err
	}

	folder, err := a.pickers().Folder("Choose a folder for the CSV files", dir)
	if err != nil {
		return service.ExportResult{}, fmt.Errorf("folder dialog failed: %w", err)
	}

	if folder == "" {
		return service.ExportResult{}, nil
	}

	return service.ExportCSV(a.files, folder, input)
}

// OpenInvoiceURL opens an invoice's link in the system browser.
func (a *API) OpenInvoiceURL(id uint) error {
	link, err := a.InvoiceLink(id)
	if err != nil {
		return err
	}

	runtime.BrowserOpenURL(a.ctx, link)

	return nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// sameFile reports whether a and b are one file (b existing).
func sameFile(a, b string) bool {
	if a == "" || b == "" {
		return false
	}

	left, err := os.Stat(a)
	if err != nil {
		return false
	}

	right, err := os.Stat(b)

	return err == nil && os.SameFile(left, right)
}
