package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lazybark/cents/flows/export"
	"github.com/lazybark/cents/flows/settings"
)

const (
	editAmountFieldCurrent = iota
	editAmountFieldUpdateLog
	editAmountFieldIgnore
	editAmountFieldLogDate
	editAmountFieldLogValue
	editAmountFieldCount
)

func renderHeader(width int) string {
	title := appTitleStyle.Render("CENTS")
	badge := badgeStyle.Render("Personal Finance TUI")
	subtitle := hintStyle.Render("incomes, expenses, accounts, subscriptions, invoices, debts, goals, taxes and settings")
	line := lipgloss.JoinVertical(lipgloss.Left, lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", badge), subtitle)

	return panelStyle.Width(width).Render(line)
}

func renderFooter(width int, status string) string {
	content := lipgloss.JoinHorizontal(lipgloss.Center, statusLabelStyle.Render("STATUS"), " ", statusStyle.Render(status))

	return panelStyle.Width(width).Render(content)
}

func menuColumnLayout(groups []menuGroup) ([]int, []int) {
	leftColumn := make([]int, 0, len(groups)/2+1)
	rightColumn := make([]int, 0, len(groups)/2+1)

	for index := range groups {
		if index%2 == 0 {
			leftColumn = append(leftColumn, index)
		} else {
			rightColumn = append(rightColumn, index)
		}
	}

	return leftColumn, rightColumn
}

func menuColumnPosition(groupIndex int) (column int, row int) {
	if groupIndex%2 == 0 {
		return 0, groupIndex / 2
	}

	return 1, groupIndex / 2
}

func menuMoveWithinColumn(currentGroup int, direction int) (int, bool) {
	leftColumn, rightColumn := menuColumnLayout(appMenuGroups())
	column, row := menuColumnPosition(currentGroup)
	currentColumn := leftColumn
	if column == 1 {
		currentColumn = rightColumn
	}

	targetRow := row + direction
	if targetRow < 0 || targetRow >= len(currentColumn) {
		return 0, false
	}

	return currentColumn[targetRow], true
}

func menuMoveAcrossColumns(currentGroup int, direction int) (int, bool) {
	leftColumn, rightColumn := menuColumnLayout(appMenuGroups())
	column, row := menuColumnPosition(currentGroup)

	if direction < 0 {
		if column == 1 && row < len(leftColumn) {
			return leftColumn[row], true
		}

		return 0, false
	}

	if column == 0 && row < len(rightColumn) {
		return rightColumn[row], true
	}

	return 0, false
}

func selectedCurrencyOption(options []string, index int) string {
	if len(options) == 0 {
		return "$"
	}

	if index < 0 || index >= len(options) {
		return options[0]
	}

	return options[index]
}

func selectedPaymentMethodOption(options []string, index int) string {
	if len(options) == 0 {
		return "Other"
	}

	if index < 0 || index >= len(options) {
		return options[0]
	}

	return options[index]
}

func renderMoneyWithCurrency(currency string, cents int64) string {
	if currency == "" {
		return formatAmount(cents)
	}

	sign := ""
	absoluteCents := cents

	if cents < 0 {
		sign = "-"
		absoluteCents = -cents
	}

	return sign + currency + " " + formatAmount(absoluteCents)
}

func renderSignedMoneyWithCurrency(currency string, cents int64) string {
	formatted := renderMoneyWithCurrency(currency, cents)
	if cents > 0 {
		return positiveStyle.Render(formatted)
	}

	if cents < 0 {
		return obligationStyle.Render(formatted)
	}

	return formatted
}

func settingsExpenseCategoryStartCursor(settings settings.AppSettings) int {
	return settingsIncomeCategoryAddCursor(settings) + 1
}

func settingsExpenseCategoryAddCursor(settings settings.AppSettings) int {
	return settingsExpenseCategoryStartCursor(settings) + len(settings.ExpenseCategories)
}

func settingsCursorByName(currencies []settings.SettingCurrency, name string) int {
	for index := range currencies {
		if strings.EqualFold(currencies[index].CurrencyName, name) {
			return index + 1
		}
	}
	return 0
}

func settingsPaymentMethodStartCursor(settings settings.AppSettings) int {
	return len(settings.Currencies) + 2
}

func settingsPaymentMethodAddCursor(settings settings.AppSettings) int {
	return settingsPaymentMethodStartCursor(settings) + len(settings.PaymentMethods)
}

func settingsCursorByPaymentMethodName(settings settings.AppSettings, name string) int {
	start := settingsPaymentMethodStartCursor(settings)
	for index := range settings.PaymentMethods {
		if strings.EqualFold(strings.TrimSpace(settings.PaymentMethods[index].PaymentMethodName), strings.TrimSpace(name)) {
			return start + index
		}
	}
	return settingsPaymentMethodAddCursor(settings)
}

func settingsTaxTypeStartCursor(settings settings.AppSettings) int {
	return settingsPaymentMethodAddCursor(settings) + 1
}

func settingsTaxTypeAddCursor(settings settings.AppSettings) int {
	return settingsTaxTypeStartCursor(settings) + len(settings.TaxTypes)
}

func settingsCursorByTaxTypeID(settings settings.AppSettings, id uint, country string, name string) int {
	start := settingsTaxTypeStartCursor(settings)
	for index := range settings.TaxTypes {
		item := settings.TaxTypes[index]
		if id > 0 && item.ID == id {
			return start + index
		}
		if strings.EqualFold(strings.TrimSpace(item.Country), strings.TrimSpace(country)) && strings.EqualFold(strings.TrimSpace(item.TaxTypeName), strings.TrimSpace(name)) {
			return start + index
		}
	}
	return settingsTaxTypeAddCursor(settings)
}

func settingsIncomeCategoryStartCursor(settings settings.AppSettings) int {
	return settingsTaxTypeAddCursor(settings) + 1
}

func settingsIncomeCategoryAddCursor(settings settings.AppSettings) int {
	return settingsIncomeCategoryStartCursor(settings) + len(settings.IncomeCategories)
}

func settingsCursorByIncomeCategoryName(settings settings.AppSettings, name string) int {
	start := settingsIncomeCategoryStartCursor(settings)
	for index := range settings.IncomeCategories {
		if strings.EqualFold(strings.TrimSpace(settings.IncomeCategories[index].CategoryName), strings.TrimSpace(name)) {
			return start + index
		}
	}
	return settingsIncomeCategoryAddCursor(settings)
}

func settingsCursorByExpenseCategoryName(settings settings.AppSettings, name string) int {
	start := settingsExpenseCategoryStartCursor(settings)
	for index := range settings.ExpenseCategories {
		if strings.EqualFold(strings.TrimSpace(settings.ExpenseCategories[index].CategoryName), strings.TrimSpace(name)) {
			return start + index
		}
	}

	return settingsExpenseCategoryAddCursor(settings)
}

func paymentMethodTypeIndex(options []string, value string) int {
	for i := range options {
		if strings.EqualFold(strings.TrimSpace(options[i]), strings.TrimSpace(value)) {
			return i
		}
	}

	return 0
}

func selectedPaymentMethodType(options []string, index int) string {
	if len(options) == 0 {
		return "Other"
	}

	if index < 0 || index >= len(options) {
		return options[0]
	}

	return options[index]
}

func selectedStringOption(options []string, index int) string {
	if len(options) == 0 {
		return ""
	}

	if index < 0 || index >= len(options) {
		return options[0]
	}

	return options[index]
}

func currencySelectionOptions(settings settings.AppSettings) []string {
	base := strings.TrimSpace(settings.BaseCurrency)
	if base == "" {
		base = "$"
	}

	options := []string{base}
	seen := map[string]struct{}{strings.ToLower(base): {}}

	for _, currency := range settings.Currencies {
		name := strings.TrimSpace(currency.CurrencyName)
		if name == "" {
			continue
		}

		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			continue
		}

		seen[key] = struct{}{}
		options = append(options, name)
	}

	return options
}

func paymentMethodSelectionOptions(settings settings.AppSettings) []string {
	if len(settings.PaymentMethods) == 0 {
		return []string{"Other"}
	}

	options := make([]string, 0, len(settings.PaymentMethods))
	for _, method := range settings.PaymentMethods {
		name := strings.TrimSpace(method.PaymentMethodName)
		if name == "" {
			continue
		}

		if method.IsDefault {
			options = append([]string{name}, options...)

			continue
		}

		options = append(options, name)
	}

	if len(options) == 0 {
		return []string{"Other"}
	}

	return options
}

func databaseStatus(created bool, count int, dbPath string) string {
	if created {
		return "created " + dbPath + " and loaded " + strconv.Itoa(count) + " accounts"
	}

	return "opened " + dbPath + " with " + strconv.Itoa(count) + " accounts"
}

type menuGroup struct {
	title string
	items []string
}

func (m TheApplication) renderDataExport(width int) string {
	datasetPrefix := " "
	formatPrefix := " "
	pathPrefix := " "
	runStyle := buttonStyle

	switch m.exportForm.Active {
	case export.ExportFieldDataset:
		datasetPrefix = ">"
	case export.ExportFieldFormat:
		formatPrefix = ">"
	case export.ExportFieldPath:
		pathPrefix = ">"
	case export.ExportFieldRun:
		runStyle = buttonActiveStyle
	}

	formatOptions := make([]string, 0, len(m.exportForm.FormatOptions))

	for i, option := range m.exportForm.FormatOptions {
		style := buttonStyle
		if i == m.exportForm.FormatIndex {
			style = buttonActiveStyle
		}
		formatOptions = append(formatOptions, style.Render(option.Label()))
	}

	rows := []string{
		sectionTitleStyle.Render("Export"),
		hintStyle.Render("Use up/down to change fields. Left/right changes options. Enter exports from the Export button. Esc returns to menu."),
		"",
		fmt.Sprintf("%s %-8s %s", datasetPrefix, "Data", buttonActiveStyle.Render(selectedExportDataset(m.exportForm).Label())),
		fmt.Sprintf("%s %-8s %s", formatPrefix, "Format", strings.Join(formatOptions, " ")),
		fmt.Sprintf("%s %-8s %s", pathPrefix, "Path", m.exportForm.PathInput.View()),
		"",
		runStyle.Render("Export"),
		"",
		mutedStyle.Render("Path may be a folder or a filename. CSV exports all data as a folder of table files."),
	}

	return panelStyle.Width(width).Render(strings.Join(rows, "\n"))
}
