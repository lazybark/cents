package sqlite

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lazybark/cents/flows/debt"
	"github.com/lazybark/cents/flows/tax"
)

// Backup writes a full copy of the database to path, replacing a file
// there. SQLite writes the copy (VACUUM INTO) as one consistent snapshot,
// so it's safe while the app has the database open; it goes to a file next
// to path first and takes its place once complete, so a failed backup
// leaves what was there.
func (s *SQLiteStorage) Backup(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("choose where to save the backup")
	}

	target, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve the backup path: %w", err)
	}

	if s.isOpenDatabase(target) || s.vault != nil && sameFile(s.vault.Path(), target) {
		return errors.New("that's the database itself; choose another file for the backup")
	}

	if info, err := os.Stat(target); err == nil && info.IsDir() {
		return fmt.Errorf("%s is a folder; choose a file name for the backup", target)
	}

	// An encrypted database's backup is encrypted with the same password.
	if s.vault != nil {
		if err := s.vault.SaveTo(target); err != nil {
			return fmt.Errorf("failed to write the backup: %w", err)
		}

		return nil
	}

	temp := target + ".partial"
	_ = os.Remove(temp)
	if err := s.db.Exec("VACUUM INTO ?", temp).Error; err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("failed to write the backup: %w", err)
	}

	if err := os.Rename(temp, target); err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("failed to save the backup: %w", err)
	}

	return nil
}

// isOpenDatabase reports whether path is the database file open now.
func (s *SQLiteStorage) isOpenDatabase(path string) bool {
	var rows []struct {
		Name string
		File string
	}

	if err := s.db.Raw("PRAGMA database_list").Scan(&rows).Error; err != nil {
		return false
	}

	for _, row := range rows {
		if row.File == "" {
			continue
		}

		open, err := filepath.EvalSymlinks(row.File)
		if err != nil {
			open = row.File
		}

		target, err := filepath.EvalSymlinks(path)
		if err != nil {
			target = path
		}

		if open == target {
			return true
		}
	}

	return false
}

// LoadAllDebtLogs returns every debt's payment log, oldest first.
func (s *SQLiteStorage) LoadAllDebtLogs() ([]debt.DebtLog, error) {
	var logs []debt.DebtLog

	if err := s.db.Order("created_at asc, id asc").Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to load debt logs: %w", err)
	}

	return logs, nil
}

// LoadAllTaxLogs returns every tax's payment log, oldest first.
func (s *SQLiteStorage) LoadAllTaxLogs() ([]tax.TaxLog, error) {
	var logs []tax.TaxLog

	if err := s.db.Order("created_at asc, id asc").Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to load tax logs: %w", err)
	}

	return logs, nil
}

// Close closes the database, saving an encrypted one first; the storage
// can't be used after.
func (s *SQLiteStorage) Close() error {
	if s.vault != nil {
		if err := s.vault.Close(); err != nil {
			return err
		}
	}

	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}

func sameFile(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}

	return ra == rb
}
