package main

import (
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lazybark/cents/app"
	storage "github.com/lazybark/cents/storage/sqlite"
)

func main() {
	db, dbPath, created, err := storage.OpenDatabase()
	if err != nil {
		log.Println("database init failed:", err)
		os.Exit(1)
	}

	sqlite, err := storage.NewSQLiteStorage()
	if err != nil {
		log.Println("storage worker init failed:", err)
		os.Exit(1)
	}

	accounts, err := storage.LoadAccounts(db)
	if err != nil {
		log.Println("database read failed:", err)
		os.Exit(1)
	}

	subscriptions, err := storage.LoadSubscriptions(db)
	if err != nil {
		log.Println("subscriptions read failed:", err)
		os.Exit(1)
	}

	debts, err := storage.LoadDebts(db)
	if err != nil {
		log.Println("debts read failed:", err)
		os.Exit(1)
	}

	goals, err := storage.LoadGoals(db)
	if err != nil {
		log.Println("goals read failed:", err)
		os.Exit(1)
	}

	taxes, err := storage.LoadTaxes(db)
	if err != nil {
		log.Println("taxes read failed:", err)
		os.Exit(1)
	}

	invoices, err := storage.LoadInvoices(db)
	if err != nil {
		log.Println("invoices read failed:", err)
		os.Exit(1)
	}

	cashflows, err := storage.LoadCashflows(db)
	if err != nil {
		log.Println("cashflows read failed:", err)
		os.Exit(1)
	}

	settings, err := storage.LoadAppSettings(db)
	if err != nil {
		log.Println("settings read failed:", err)
		os.Exit(1)
	}

	m := app.NewApp(sqlite, dbPath, created, accounts, subscriptions, debts, goals, taxes, invoices, cashflows, settings)

	program := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		log.Println("program failed:", err)
		os.Exit(1)
	}
}
