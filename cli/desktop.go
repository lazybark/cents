package cli

import (
	"errors"
	"fmt"

	"github.com/lazybark/cents/config"
	"github.com/lazybark/cents/desktop"
	storage "github.com/lazybark/cents/storage/sqlite"
)

func runDesktop(l launcher) error {
	server, err := l.server()
	if err != nil {
		return err
	}

	createPath, openPath := setupDefaults()
	opts := desktop.Options{
		CreatePath: createPath,
		OpenPath:   openPath,
		SaveServer: func(s desktop.Server) error {
			return l.saveServer(config.Server{URL: s.URL, Token: s.Token, Certificate: s.Certificate})
		},
		Choose: func(path string, create bool) (desktop.StorageWorker, string, error) {
			db, dbPath, _, err := l.choose(path, create)
			if errors.Is(err, storage.ErrEncrypted) {
				return nil, "", desktop.ErrLocked
			}

			if err != nil {
				return nil, "", err
			}

			return storage.NewSQLiteStorage(db), dbPath, nil
		},
		Unlock: func(path, password string) (desktop.StorageWorker, error) {
			db, vault, err := l.unlock(path, password)
			if err != nil {
				return nil, err
			}

			return storage.NewEncryptedStorage(db, vault), nil
		},
		Encrypt: func(s desktop.StorageWorker, path, password string) (desktop.StorageWorker, error) {
			return s.(*storage.SQLiteStorage).Encrypt(path, password)
		},
		Decrypt: func(s desktop.StorageWorker, password string) (desktop.StorageWorker, error) {
			return s.(*storage.SQLiteStorage).Decrypt(password)
		},
	}

	// A server is connected to once the window is up, so it can say how
	// that went.
	if server != nil {
		opts.Server = &desktop.Server{URL: server.URL, Token: server.Token, Certificate: server.Certificate}
		return desktop.Run(opts)
	}

	dbPath, note, err := l.resolve()
	if err != nil {
		return err
	}

	opts.SetupNote = note

	// An encrypted database opens once its password is given.
	if dbPath != "" {
		if encrypted, err := storage.IsEncrypted(dbPath); err == nil && encrypted {
			opts.LockedPath = dbPath
			return desktop.Run(opts)
		}
	}

	if dbPath != "" {
		db, _, err := l.open(dbPath)
		if err != nil {
			return fmt.Errorf("database init failed: %w", err)
		}

		opts.Storage = storage.NewSQLiteStorage(db)
		opts.DBPath = dbPath
	}

	return desktop.Run(opts)
}
