package service

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/analytics"
	"github.com/lazybark/cents/flows/report"
	"github.com/lazybark/cents/flows/settings"
)

// ExportView is what the Backup & export screen shows.
type ExportView struct {
	// DBPath is the database file, for the desktop app (which fills it in).
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

func (s *Service) ExportOptions() (ExportView, error) {
	storage, stts, err := s.storageAndSettings()
	if err != nil {
		return ExportView{}, err
	}

	view := ExportView{LastBackupPath: stts.Backup.Path, Datasets: make([]ExportDataset, 0)}
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

// Files are the calls that move whole files, which don't fit JSON: the
// server streams them. SaveBackup and ExportCSV save them as files.
type Files interface {
	// WriteBackup writes a copy of the whole database to w (encrypted with
	// the same password, for an encrypted one) and remembers savedTo, where
	// the copy is kept, as the last backup.
	WriteBackup(w io.Writer, savedTo string) (BackupResult, error)
	// WriteCSV writes one dataset (a key from ExportOptions) of the period
	// (YYYY-MM-DD, either may be empty) as CSV. It reports the rows.
	WriteCSV(w io.Writer, dataset, from, to string) (int, error)
}

func (s *Service) WriteBackup(w io.Writer, savedTo string) (BackupResult, error) {
	storage, err := s.currentStorage()
	if err != nil {
		return BackupResult{}, err
	}

	dir, err := os.MkdirTemp("", "cents-backup-")
	if err != nil {
		return BackupResult{}, fmt.Errorf("failed to make room for the backup: %w", err)
	}
	defer os.RemoveAll(dir)

	copied := filepath.Join(dir, "cents.db")
	if err := storage.Backup(copied); err != nil {
		return BackupResult{}, err
	}

	file, err := os.Open(copied)
	if err != nil {
		return BackupResult{}, fmt.Errorf("failed to read the backup: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(w, file); err != nil {
		return BackupResult{}, fmt.Errorf("failed to send the backup: %w", err)
	}

	now := time.Now()
	result := BackupResult{Path: strings.TrimSpace(savedTo), At: now.Local().Format(backupTimeLayout)}
	if result.Path == "" {
		return result, nil
	}

	records := []settings.SettingRecord{
		{SettingID: settings.LastBackupAtID, SettingValue: now.UTC().Format(time.RFC3339)},
		{SettingID: settings.LastBackupPathID, SettingValue: result.Path},
	}
	if err := storage.SaveSettingRecords(records); err != nil {
		return BackupResult{}, fmt.Errorf("the backup is saved, but remembering it failed: %w", err)
	}

	return result, nil
}

func (s *Service) WriteCSV(w io.Writer, dataset, from, to string) (int, error) {
	period, err := exportPeriod(ExportInput{From: from, To: to})
	if err != nil {
		return 0, err
	}

	var found *report.Dataset
	for _, ds := range report.Datasets() {
		if ds.Key == dataset {
			found = &ds
			break
		}
	}

	if found == nil {
		return 0, notFoundError(fmt.Sprintf("dataset %q", dataset))
	}

	storage, err := s.currentStorage()
	if err != nil {
		return 0, err
	}

	in, err := reportInput(storage, time.Now())
	if err != nil {
		return 0, err
	}

	in.Period = period
	table := found.Build(in)
	data, err := report.CSV(table)
	if err != nil {
		return 0, fmt.Errorf("failed to make %s: %w", table.Name, err)
	}

	if _, err := w.Write(data); err != nil {
		return 0, fmt.Errorf("failed to send %s: %w", table.Name, err)
	}

	return len(table.Rows), nil
}

// SaveBackup saves a backup from files at path (".db" is added without an
// extension). It goes to a file next to path first and takes its place once
// complete, so a failed backup leaves what was there. The caller makes sure
// path isn't the database itself.
func SaveBackup(files Files, path string) (BackupResult, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return BackupResult{}, errors.New("choose where to save the backup")
	}

	if filepath.Ext(path) == "" {
		path += ".db"
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return BackupResult{}, fmt.Errorf("failed to resolve the backup path: %w", err)
	}

	if info, err := os.Stat(abs); err == nil && info.IsDir() {
		return BackupResult{}, fmt.Errorf("%s is a folder; choose a file name for the backup", abs)
	}

	var result BackupResult
	err = writeFile(abs, func(w io.Writer) error {
		var err error
		result, err = files.WriteBackup(w, abs)
		return err
	})

	return result, err
}

// ExportCSV writes the datasets from files as CSV files into a new folder
// in parent, named for the day ("cents-export-2026-10-04", then "-2" and so
// on).
func ExportCSV(files Files, parent string, input ExportInput) (ExportResult, error) {
	keys, err := input.keys()
	if err != nil {
		return ExportResult{}, err
	}

	folder, err := newExportFolder(parent, "cents-export-"+time.Now().Format("2006-01-02"))
	if err != nil {
		return ExportResult{}, err
	}

	result := ExportResult{Folder: folder, Files: make([]ExportFile, 0, len(keys))}
	for _, key := range keys {
		name := key + ".csv"
		rows := 0
		err := writeFile(filepath.Join(folder, name), func(w io.Writer) error {
			var err error
			rows, err = files.WriteCSV(w, key, input.From, input.To)
			return err
		})
		if err != nil {
			return ExportResult{}, fmt.Errorf("failed to write %s: %w", name, err)
		}

		result.Files = append(result.Files, ExportFile{Name: name, Rows: rows})
	}

	return result, nil
}

// Check says what's wrong with input, if anything.
func (input ExportInput) Check() error {
	_, err := input.keys()
	return err
}

// keys checks input and lists its datasets in the order of
// report.Datasets.
func (input ExportInput) keys() ([]string, error) {
	if _, err := exportPeriod(input); err != nil {
		return nil, err
	}

	wanted := map[string]bool{}
	for _, key := range input.Datasets {
		wanted[key] = true
	}

	var keys []string
	for _, ds := range report.Datasets() {
		if wanted[ds.Key] {
			keys = append(keys, ds.Key)
			delete(wanted, ds.Key)
		}
	}

	if len(wanted) > 0 || len(keys) == 0 {
		return nil, errors.New("pick at least one thing to export")
	}

	return keys, nil
}

// writeFile writes path through a file next to it, which takes its place
// once write succeeds.
func writeFile(path string, write func(io.Writer) error) error {
	temp := path + ".partial"
	file, err := os.Create(temp)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", filepath.Base(path), err)
	}

	err = write(file)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}

	if err == nil {
		err = os.Rename(temp, path)
	}

	if err != nil {
		_ = os.Remove(temp)
	}

	return err
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
func reportInput(storage Store, now time.Time) (report.Input, error) {
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
