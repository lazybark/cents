package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lazybark/cents/config"
	storage "github.com/lazybark/cents/storage/sqlite"
	"gorm.io/gorm"
)

// launcher decides which database a run uses. Both interfaces share it: they
// only differ in how they ask the user when setup is needed.
type launcher struct {
	configPath string
	dbOverride string
}

// resolve returns the database path to open. An empty path means the user has
// to pick one first; note then says why, when there is more to it than a
// missing config file.
func (l launcher) resolve() (dbPath string, note string, err error) {
	if l.dbOverride != "" {
		path, err := absPath(l.dbOverride)
		return path, "", err
	}

	cfg, err := config.Load(l.configPath)
	if errors.Is(err, config.ErrNotFound) {
		return "", "", nil
	}

	if err != nil {
		return "", "", err
	}

	if strings.TrimSpace(cfg.DBPath) == "" {
		return "", "Config file has no database path set.", nil
	}

	// An encrypted one is fine; it's opened with its password.
	if err := storage.CheckDatabase(cfg.DBPath); err != nil && !errors.Is(err, storage.ErrEncrypted) {
		return "", fmt.Sprintf("Configured database can't be used: %v", err), nil
	}

	return cfg.DBPath, "", nil
}

// unlock opens the encrypted database at path with its password and saves
// it to the config file, like choose.
func (l launcher) unlock(path, password string) (*gorm.DB, *storage.Vault, error) {
	db, vault, err := storage.OpenEncrypted(path, password)
	if err != nil {
		return nil, nil, err
	}

	if err := config.Save(l.configPath, config.Config{DBPath: path}); err != nil {
		_ = storage.NewEncryptedStorage(db, vault).Close()
		return nil, nil, err
	}

	return db, vault, nil
}

// open opens a resolved database. With --db, a missing file is created, but an
// existing file must already be a cents database.
func (l launcher) open(dbPath string) (*gorm.DB, bool, error) {
	if _, err := os.Stat(dbPath); err == nil {
		if err := storage.CheckDatabase(dbPath); err != nil {
			return nil, false, err
		}
	}

	return storage.OpenDatabase(dbPath)
}

// choose opens (or creates) the database picked during setup and saves it to
// the config file, so the next launch skips setup.
func (l launcher) choose(rawPath string, create bool) (*gorm.DB, string, bool, error) {
	dbPath, err := absPath(rawPath)
	if err != nil {
		return nil, "", false, err
	}

	if create {
		if err := prepareNewDatabase(dbPath); err != nil {
			return nil, "", false, err
		}
	} else if err := storage.CheckDatabase(dbPath); err != nil {
		return nil, "", false, err
	}

	db, created, err := storage.OpenDatabase(dbPath)
	if err != nil {
		return nil, "", false, err
	}

	if err := config.Save(l.configPath, config.Config{DBPath: dbPath}); err != nil {
		return nil, "", false, err
	}

	return db, dbPath, created, nil
}

// setupDefaults suggests paths for the setup screens: a new database next to
// the config file, and an existing cents.db in the working directory, which is
// where databases lived before the config file existed.
func setupDefaults() (createPath string, openPath string) {
	createPath, err := config.DefaultDBPath()
	if err != nil {
		createPath = "cents.db"
	}

	if workingDir, err := os.Getwd(); err == nil {
		candidate := filepath.Join(workingDir, "cents.db")
		if storage.CheckDatabase(candidate) == nil {
			openPath = candidate
		}
	}

	return createPath, openPath
}

func prepareNewDatabase(dbPath string) error {
	_, err := os.Stat(dbPath)
	if err == nil {
		return fmt.Errorf("%s already exists: choose \"open existing\" to use it", dbPath)
	}

	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to check database path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	return nil
}

// absPath expands a leading ~ and makes the path absolute, so the config file
// never stores a path relative to wherever the app happened to start.
func absPath(raw string) (string, error) {
	path := strings.TrimSpace(raw)
	if path == "" {
		return "", errors.New("database path is empty")
	}

	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to locate home directory: %w", err)
		}

		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve database path: %w", err)
	}

	return abs, nil
}
