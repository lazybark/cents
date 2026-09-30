package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lazybark/cents/config"
	storage "github.com/lazybark/cents/storage/sqlite"
)

func newTestLauncher(t *testing.T) (launcher, string) {
	t.Helper()
	dir := t.TempDir()

	return launcher{configPath: filepath.Join(dir, "config", "config.yml")}, dir
}

func TestResolveAsksForSetupWithoutConfig(t *testing.T) {
	l, _ := newTestLauncher(t)

	dbPath, note, err := l.resolve()
	if err != nil {
		t.Fatal(err)
	}
	if dbPath != "" || note != "" {
		t.Fatalf("expected setup without note, got path %q note %q", dbPath, note)
	}
}

func TestChooseCreateSavesConfigAndNextResolveSkipsSetup(t *testing.T) {
	l, dir := newTestLauncher(t)
	want := filepath.Join(dir, "data", "cents.db")

	_, got, created, err := l.choose(want, true)
	if err != nil {
		t.Fatal(err)
	}
	if got != want || !created {
		t.Fatalf("expected new database at %q, got %q (created %v)", want, got, created)
	}

	cfg, err := config.Load(l.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DBPath != want {
		t.Fatalf("expected config to store %q, got %q", want, cfg.DBPath)
	}

	dbPath, note, err := l.resolve()
	if err != nil {
		t.Fatal(err)
	}
	if dbPath != want || note != "" {
		t.Fatalf("expected resolve to return %q without note, got %q note %q", want, dbPath, note)
	}
}

func TestResolveAsksForSetupWhenConfiguredDatabaseIsMissing(t *testing.T) {
	l, dir := newTestLauncher(t)
	if err := config.Save(l.configPath, config.Config{DBPath: filepath.Join(dir, "gone.db")}); err != nil {
		t.Fatal(err)
	}

	dbPath, note, err := l.resolve()
	if err != nil {
		t.Fatal(err)
	}
	if dbPath != "" || note == "" {
		t.Fatalf("expected setup with a note, got path %q note %q", dbPath, note)
	}
	if _, err := os.Stat(filepath.Join(dir, "gone.db")); !os.IsNotExist(err) {
		t.Fatal("expected resolve not to create the missing database")
	}
}

func TestChooseCreateRefusesExistingFile(t *testing.T) {
	l, dir := newTestLauncher(t)
	path := filepath.Join(dir, "cents.db")
	if _, _, _, err := l.choose(path, true); err != nil {
		t.Fatal(err)
	}

	if _, _, _, err := l.choose(path, true); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected already-exists error, got %v", err)
	}
}

func TestChooseOpenRejectsNonCentsFile(t *testing.T) {
	l, dir := newTestLauncher(t)
	path := filepath.Join(dir, "notes.db")
	if err := os.WriteFile(path, []byte("not a database"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, _, _, err := l.choose(path, false); err == nil {
		t.Fatal("expected error for a non-database file")
	}
	if _, err := config.Load(l.configPath); err != config.ErrNotFound {
		t.Fatalf("expected config not to be written, got %v", err)
	}
}

func TestDBOverrideIgnoresConfig(t *testing.T) {
	l, dir := newTestLauncher(t)
	l.dbOverride = filepath.Join(dir, "scratch.db")

	dbPath, _, err := l.resolve()
	if err != nil {
		t.Fatal(err)
	}
	if dbPath != l.dbOverride {
		t.Fatalf("expected override path %q, got %q", l.dbOverride, dbPath)
	}
	if _, err := config.Load(l.configPath); err != config.ErrNotFound {
		t.Fatalf("expected config untouched, got %v", err)
	}
}

// An encrypted database resolves (it opens with its password), but can't be
// opened without it; unlock opens it and remembers it.
func TestEncryptedDatabases(t *testing.T) {
	l, dir := newTestLauncher(t)
	db, dbPath, _, err := l.choose(filepath.Join(dir, "cents.db"), true)
	if err != nil {
		t.Fatal(err)
	}

	encrypted, err := storage.NewSQLiteStorage(db).Encrypt(dbPath, "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if err := encrypted.Close(); err != nil {
		t.Fatal(err)
	}

	resolved, note, err := l.resolve()
	if err != nil || resolved != dbPath || note != "" {
		t.Fatalf("expected the encrypted database resolved: %q %q %v", resolved, note, err)
	}

	if _, _, err := l.open(dbPath); !errors.Is(err, storage.ErrEncrypted) {
		t.Fatalf("expected it refused without a password: %v", err)
	}

	if _, _, _, err := l.choose(dbPath, false); !errors.Is(err, storage.ErrEncrypted) {
		t.Fatalf("expected choose to say encrypted: %v", err)
	}

	if _, _, err := l.unlock(dbPath, "wrong horse"); err == nil {
		t.Fatal("expected a wrong password to fail")
	}

	db, vault, err := l.unlock(dbPath, "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	_ = storage.NewEncryptedStorage(db, vault).Close()
}
