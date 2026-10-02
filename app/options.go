package app

import (
	"os"
	"strings"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/settings"
)

type accountSortField = account.SortField

const (
	accountSortBaseAmount = account.SortBaseAmount
	accountSortName       = account.SortName
	accountSortCurrency   = account.SortCurrency
	accountSortUpdated    = account.SortUpdated
)

func incomeCategorySelectionOptions(settings settings.AppSettings) []string {
	return settings.IncomeCategoryOptions()
}

func expenseCategorySelectionOptions(settings settings.AppSettings) []string {
	return settings.ExpenseCategoryOptions()
}

// accountSelectionOptions are "" (no account) and the accounts new records
// can pick; archived ones are left out.
func accountSelectionOptions(accounts []account.Account) []string {
	return append([]string{""}, account.PickerNames(accounts)...)
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

func paymentMethodTypeOptions() []string {
	return settings.PaymentMethodTypeOptions()
}
