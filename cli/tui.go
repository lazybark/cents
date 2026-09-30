package cli

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lazybark/cents/app"
	storage "github.com/lazybark/cents/storage/sqlite"
	"gorm.io/gorm"
)

func runTUI(l launcher) error {
	dbPath, note, err := l.resolve()
	if err != nil {
		return err
	}

	var (
		db      *gorm.DB
		created bool
	)

	if dbPath == "" {
		createPath, openPath := setupDefaults()
		chosen, err := app.RunSetup(app.SetupOptions{
			Note:       note,
			CreatePath: createPath,
			OpenPath:   openPath,
			Choose: func(path string, create bool) error {
				var err error
				db, dbPath, created, err = l.choose(path, create)

				return err
			},
		})
		if err != nil {
			return err
		}

		if !chosen {
			return nil
		}
	} else {
		db, created, err = l.open(dbPath)
		if errors.Is(err, storage.ErrEncrypted) {
			return fmt.Errorf("%s is encrypted with a password: open it in the desktop app (cents desktop), the terminal app can't ask for it", dbPath)
		}
		if err != nil {
			return fmt.Errorf("database init failed: %w", err)
		}
	}

	sqlite := storage.NewSQLiteStorage(db)

	accounts, err := storage.LoadAccounts(db)
	if err != nil {
		return fmt.Errorf("database read failed: %w", err)
	}

	subscriptions, err := storage.LoadSubscriptions(db)
	if err != nil {
		return fmt.Errorf("subscriptions read failed: %w", err)
	}

	debts, err := storage.LoadDebts(db)
	if err != nil {
		return fmt.Errorf("debts read failed: %w", err)
	}

	goals, err := storage.LoadGoals(db)
	if err != nil {
		return fmt.Errorf("goals read failed: %w", err)
	}

	taxes, err := storage.LoadTaxes(db)
	if err != nil {
		return fmt.Errorf("taxes read failed: %w", err)
	}

	invoices, err := storage.LoadInvoices(db)
	if err != nil {
		return fmt.Errorf("invoices read failed: %w", err)
	}

	cashflows, err := storage.LoadCashflows(db)
	if err != nil {
		return fmt.Errorf("cashflows read failed: %w", err)
	}

	settings, err := storage.LoadAppSettings(db)
	if err != nil {
		return fmt.Errorf("settings read failed: %w", err)
	}

	m := app.NewApp(sqlite, dbPath, created, accounts, subscriptions, debts, goals, taxes, invoices, cashflows, settings)

	program := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("program failed: %w", err)
	}

	return nil
}
