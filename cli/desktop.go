package cli

import (
	"fmt"

	"github.com/lazybark/cents/desktop"
	storage "github.com/lazybark/cents/storage/sqlite"
)

func runDesktop(l launcher) error {
	dbPath, note, err := l.resolve()
	if err != nil {
		return err
	}

	createPath, openPath := setupDefaults()
	opts := desktop.Options{
		SetupNote:  note,
		CreatePath: createPath,
		OpenPath:   openPath,
		Choose: func(path string, create bool) (desktop.StorageWorker, string, error) {
			db, dbPath, _, err := l.choose(path, create)
			if err != nil {
				return nil, "", err
			}

			return storage.NewSQLiteStorage(db), dbPath, nil
		},
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
