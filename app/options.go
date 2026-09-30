package app

import (
	"os"
	"strings"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/settings"
)

type accountSortField int

const (
	accountSortBaseAmount accountSortField = iota
	accountSortName
	accountSortCurrency
	accountSortUpdated
)

func incomeCategorySelectionOptions(settings settings.AppSettings) []string {
	items := make([]string, 0, len(settings.IncomeCategories))

	for _, category := range settings.IncomeCategories {
		name := strings.TrimSpace(category.CategoryName)
		if name != "" {
			items = append(items, name)
		}
	}

	return items
}

func expenseCategorySelectionOptions(settings settings.AppSettings) []string {
	items := make([]string, 0, len(settings.ExpenseCategories))

	for _, category := range settings.ExpenseCategories {
		name := strings.TrimSpace(category.CategoryName)
		if name != "" {
			items = append(items, name)
		}
	}

	return items
}

func accountSelectionOptions(accounts []account.Account) []string {
	items := make([]string, 0, len(accounts)+1)
	items = append(items, "")

	for _, acct := range accounts {
		name := strings.TrimSpace(acct.Name)
		if name != "" {
			items = append(items, name)
		}
	}

	return items
}

func cashflowAccountDisplayOptions(accountOptions []string) []string {
	items := make([]string, len(accountOptions))

	for i, option := range accountOptions {
		if strings.TrimSpace(option) == "" {
			items[i] = "(empty)"
		} else {
			items[i] = option
		}
	}

	return items
}

func defaultExportPath() string {
	workingDir, err := os.Getwd()
	if err != nil {
		return "."
	}

	return workingDir
}

func appMenuGroups() []menuGroup {
	return []menuGroup{
		{title: "Incomes and Expences", items: []string{"New Expence", "New Income", "History", "Overview"}},
		{title: "Accounts", items: []string{"add account", "list accounts"}},
		{title: "Subscriptions", items: []string{"new", "active", "all"}},
		{title: "Invoices", items: []string{"new", "outgoing", "incoming", "history"}},
		{title: "Debts", items: []string{"new", "outgoing", "incoming", "history"}},
		{title: "Goals", items: []string{"new", "all", "history"}},
		{title: "Taxes", items: []string{"new", "unpaid", "history"}},
		{title: "Settings", items: []string{"edit", "Export"}},
	}
}
