package desktop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/analytics"
	"github.com/lazybark/cents/flows/report"
	"github.com/lazybark/cents/flows/settings"
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

// ExportView is what the Backup & export screen shows.
type ExportView struct {
	DBPath string `json:"dbPath"`
	// The last backup, if any: when (local time) and where.
	LastBackupAt   string          `json:"lastBackupAt"`
	LastBackupPath string          `json:"lastBackupPath"`
	Datasets       []ExportDataset `json:"datasets"`
	// FirstDate is the earliest day there's a record for, for "all time".
	FirstDate string `json:"firstDate"`
}

type ExportDataset struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	// Timed ones keep to the picked period; the others list everything.
	Timed bool `json:"timed"`
}

// BackupResult is where a backup went; Path is empty when the dialog was
// cancelled.
type BackupResult struct {
	Path string `json:"path"`
	At   string `json:"at"`
}

// ExportInput picks the period (YYYY-MM-DD, either may be empty for no
// limit) and the datasets (keys) to export.
type ExportInput struct {
	From     string   `json:"from"`
	To       string   `json:"to"`
	Datasets []string `json:"datasets"`
}

// ExportResult is the folder the files went to (empty when the dialog was
// cancelled) and each file with its number of rows.
type ExportResult struct {
	Folder string       `json:"folder"`
	Files  []ExportFile `json:"files"`
}

type ExportFile struct {
	Name string `json:"name"`
	Rows int    `json:"rows"`
}

const backupTimeLayout = "2006-01-02 15:04"

func (a *API) ExportOptions() (ExportView, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return ExportView{}, err
	}

	view := ExportView{DBPath: a.Status().DBPath, LastBackupPath: stts.Backup.Path, Datasets: make([]ExportDataset, 0)}
	if !stts.Backup.At.IsZero() {
		view.LastBackupAt = stts.Backup.At.Local().Format(backupTimeLayout)
	}

	for _, ds := range report.Datasets() {
		view.Datasets = append(view.Datasets, ExportDataset{Key: ds.Key, Label: ds.Label, Timed: ds.Timed})
	}

	entries, err := storage.LoadCashflows()
	if err != nil {
		return ExportView{}, err
	}

	var first time.Time
	for _, e := range entries {
		if day := dates.Day(e.EntryDate); first.IsZero() || day.Before(first) {
			first = day
		}
	}

	if !first.IsZero() {
		view.FirstDate = dates.Text(first)
	}

	return view, nil
}

// BackupDatabase asks where to save a backup and writes it there.
func (a *API) BackupDatabase() (BackupResult, error) {
	_, stts, err := a.storageAndSettings()
	if err != nil {
		return BackupResult{}, err
	}

	dir := filepath.Dir(a.Status().DBPath)
	if stts.Backup.Path != "" {
		dir = filepath.Dir(stts.Backup.Path)
	}

	name := "cents-backup-" + time.Now().Format("2006-01-02") + ".db"
	path, err := a.pickers().SaveFile("Save a backup of the database", dir, name)
	if err != nil {
		return BackupResult{}, fmt.Errorf("file dialog failed: %w", err)
	}

	if path == "" {
		return BackupResult{}, nil
	}

	return a.backup(path, time.Now())
}

func (a *API) backup(path string, now time.Time) (BackupResult, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return BackupResult{}, err
	}

	if filepath.Ext(path) == "" {
		path += ".db"
	}

	if err := storage.Backup(path); err != nil {
		return BackupResult{}, err
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	records := []settings.SettingRecord{
		{SettingID: settings.LastBackupAtID, SettingValue: now.UTC().Format(time.RFC3339)},
		{SettingID: settings.LastBackupPathID, SettingValue: abs},
	}
	if err := storage.SaveSettingRecords(records); err != nil {
		return BackupResult{}, fmt.Errorf("the backup is saved, but remembering it failed: %w", err)
	}

	return BackupResult{Path: abs, At: now.Local().Format(backupTimeLayout)}, nil
}

// ExportCSV asks for a folder and writes the picked datasets as CSV files
// into a new folder in it.
func (a *API) ExportCSV(input ExportInput) (ExportResult, error) {
	// Checked before the dialog, so a mistake doesn't cost a pick.
	if _, err := exportPeriod(input); err != nil {
		return ExportResult{}, err
	}

	if len(input.Datasets) == 0 {
		return ExportResult{}, errors.New("pick at least one thing to export")
	}

	_, stts, err := a.storageAndSettings()
	if err != nil {
		return ExportResult{}, err
	}

	dir := filepath.Dir(a.Status().DBPath)
	if stts.Backup.Path != "" {
		dir = filepath.Dir(stts.Backup.Path)
	}

	folder, err := a.pickers().Folder("Choose a folder for the CSV files", dir)
	if err != nil {
		return ExportResult{}, fmt.Errorf("folder dialog failed: %w", err)
	}

	if folder == "" {
		return ExportResult{}, nil
	}

	return a.exportTo(folder, input, time.Now())
}

func exportPeriod(input ExportInput) (report.Period, error) {
	from, err := dates.ISO.Optional(input.From, "from")
	if err != nil {
		return report.Period{}, err
	}

	to, err := dates.ISO.Optional(input.To, "to")
	if err != nil {
		return report.Period{}, err
	}

	var p report.Period
	if from != nil {
		p.From = *from
	}

	if to != nil {
		p.To = *to
	}

	if !p.From.IsZero() && !p.To.IsZero() && p.To.Before(p.From) {
		return report.Period{}, errors.New("the period ends before it starts")
	}

	return p, nil
}

// exportTo writes the datasets into a new folder in parent, named for the
// day ("cents-export-2026-10-04", then "-2" and so on).
func (a *API) exportTo(parent string, input ExportInput, now time.Time) (ExportResult, error) {
	period, err := exportPeriod(input)
	if err != nil {
		return ExportResult{}, err
	}

	wanted := map[string]bool{}
	for _, key := range input.Datasets {
		wanted[key] = true
	}

	var datasets []report.Dataset
	for _, ds := range report.Datasets() {
		if wanted[ds.Key] {
			datasets = append(datasets, ds)
			delete(wanted, ds.Key)
		}
	}

	if len(wanted) > 0 || len(datasets) == 0 {
		return ExportResult{}, errors.New("pick at least one thing to export")
	}

	storage, err := a.currentStorage()
	if err != nil {
		return ExportResult{}, err
	}

	in, err := reportInput(storage, now)
	if err != nil {
		return ExportResult{}, err
	}

	in.Period = period
	folder, err := newExportFolder(parent, "cents-export-"+now.Format("2006-01-02"))
	if err != nil {
		return ExportResult{}, err
	}

	result := ExportResult{Folder: folder, Files: make([]ExportFile, 0, len(datasets))}
	for _, ds := range datasets {
		table := ds.Build(in)
		data, err := report.CSV(table)
		if err != nil {
			return ExportResult{}, fmt.Errorf("failed to make %s: %w", table.Name, err)
		}

		name := table.Name + ".csv"
		if err := os.WriteFile(filepath.Join(folder, name), data, 0o644); err != nil {
			return ExportResult{}, fmt.Errorf("failed to write %s: %w", name, err)
		}

		result.Files = append(result.Files, ExportFile{Name: name, Rows: len(table.Rows)})
	}

	return result, nil
}

func newExportFolder(parent, name string) (string, error) {
	parent = strings.TrimSpace(parent)
	if info, err := os.Stat(parent); err != nil || !info.IsDir() {
		return "", fmt.Errorf("%s isn't a folder", parent)
	}

	for n := 1; n < 1000; n++ {
		candidate := filepath.Join(parent, name)
		if n > 1 {
			candidate = filepath.Join(parent, fmt.Sprintf("%s-%d", name, n))
		}

		err := os.Mkdir(candidate, 0o755)
		if err == nil {
			return candidate, nil
		}

		if !os.IsExist(err) {
			return "", fmt.Errorf("failed to create the export folder: %w", err)
		}
	}

	return "", errors.New("too many exports in that folder today")
}

// reportInput loads everything the CSV tables are made from.
func reportInput(storage StorageWorker, now time.Time) (report.Input, error) {
	data, err := loadSummaryData(storage)
	if err != nil {
		return report.Input{}, err
	}

	in := report.Input{Data: data, Now: now}
	h, err := loadHistory(storage, data)
	if err != nil {
		return report.Input{}, err
	}

	in.AccountLogs, in.AssetLogs = h.AccountLogs, h.AssetLogs
	in.NetWorth = analytics.NetWorthHistory(h, now)

	if in.DebtLogs, err = storage.LoadAllDebtLogs(); err != nil {
		return report.Input{}, err
	}

	if in.CreditLogs, err = storage.LoadAllCreditLogs(); err != nil {
		return report.Input{}, err
	}

	if in.TaxLogs, err = storage.LoadAllTaxLogs(); err != nil {
		return report.Input{}, err
	}

	if in.GoalLogs, err = storage.LoadAllGoalLogs(); err != nil {
		return report.Input{}, err
	}

	if in.Notes, err = storage.LoadMonthNotes(); err != nil {
		return report.Input{}, err
	}

	return in, nil
}
