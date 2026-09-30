package sqlite

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lazybark/cents/flows/settings"
)

func TestBackup(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cents.db")
	db, _, err := OpenDatabase(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	s := NewSQLiteStorage(db)

	if err := db.Create(&settings.SettingIncomeCategory{CategoryName: "Salary"}).Error; err != nil {
		t.Fatal(err)
	}

	backup := filepath.Join(dir, "backups", "copy.db")
	if err := os.MkdirAll(filepath.Dir(backup), 0o755); err != nil {
		t.Fatal(err)
	}

	// Twice: the second replaces the first.
	for range 2 {
		if err := s.Backup(backup); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := os.Stat(backup + ".partial"); !os.IsNotExist(err) {
		t.Fatal("the partial file should be gone")
	}

	copyDB, _, err := OpenDatabase(backup)
	if err != nil {
		t.Fatal(err)
	}

	var count int64
	copyDB.Model(&settings.SettingIncomeCategory{}).Where("category_name = ?", "Salary").Count(&count)
	if count != 1 {
		t.Fatalf("expected the category in the backup, got %d", count)
	}

	if err := s.Backup(dbPath); err == nil {
		t.Fatal("expected backing up onto the database itself to fail")
	}

	if err := s.Backup(dir); err == nil {
		t.Fatal("expected a folder to fail")
	}

	if err := s.Backup(" "); err == nil {
		t.Fatal("expected an empty path to fail")
	}
}
