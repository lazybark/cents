package app

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lazybark/cents/flows/export"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/subscription"
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

func (m TheApplication) renderDashboard(width int) string {
	base := m.baseCurrencyLabel()

	// Monthly net: income - expenses for current month.
	var monthlyIncome int64
	var monthlyExpense int64

	for _, item := range m.filteredCashflowsForMonth() {
		amountBase, ok := m.convertToBaseCents(item.Currency, item.AmountCents)
		if !ok {
			continue
		}

		if item.IsIncome {
			monthlyIncome += amountBase
		} else {
			monthlyExpense += amountBase
		}
	}

	monthlyNet := monthlyIncome - monthlyExpense

	// Subscription totals.
	activeSubscriptions := make([]subscription.Subscription, 0)
	for _, sub := range m.subscriptions {
		if sub.IsActive {
			activeSubscriptions = append(activeSubscriptions, sub)
		}
	}

	monthlySubCost, yearlySubProjection := m.subscriptionTotalsInBaseCents(activeSubscriptions)

	// Total accounts.
	totalAccounts := m.sumAccountsInBaseCents()

	// Unpaid debts (separate by direction).
	var unPaidDebtToMe int64

	var unPaidDebtByMe int64

	for _, d := range m.debts {
		if d.AmountPaidCents >= d.AmountCents {
			continue
		}

		converted, ok := m.convertToBaseCents(d.Currency, d.AmountCents-d.AmountPaidCents)
		if !ok {
			continue
		}

		if d.IsOwedToUser {
			unPaidDebtToMe += converted
		} else {
			unPaidDebtByMe += converted
		}
	}

	// Unpaid taxes.
	var unpaidTaxes int64

	for _, tax := range m.taxes {
		if tax.AmountPaidCents < tax.AmountDueCents {
			converted, ok := m.convertToBaseCents(base, tax.AmountDueCents-tax.AmountPaidCents)

			if !ok {
				continue
			}

			unpaidTaxes += converted
		}
	}

	// Unpaid invoices (separate by direction).
	var unpaidInvoiceToMe int64

	var unpaidInvoiceByMe int64

	for _, inv := range m.invoices {
		if inv.Paid {
			continue
		}

		converted, ok := m.convertToBaseCents(inv.Currency, inv.AmountCents)
		if !ok {
			continue
		}

		if inv.IsIncoming {
			unpaidInvoiceToMe += converted
		} else {
			unpaidInvoiceByMe += converted
		}
	}

	// Goals progress.
	accumulatedBase, targetBase := m.goalProgressTotalsBase(m.goals)

	var goalsProgressStr string

	if targetBase > 0 {
		percentage := (float64(accumulatedBase) / float64(targetBase)) * 100
		goalsProgressStr = fmt.Sprintf("%s / %s (%.1f%%)", renderMoneyWithCurrency(base, accumulatedBase), renderMoneyWithCurrency(base, targetBase), percentage)
	} else {
		goalsProgressStr = fmt.Sprintf("%s / %s", renderMoneyWithCurrency(base, accumulatedBase), renderMoneyWithCurrency(base, targetBase))
	}

	// Build left column.
	leftLines := []string{sectionTitleStyle.Render("Financial Summary")}
	leftLines = append(leftLines,
		fmt.Sprintf("Monthly net:        %s", renderMoneyWithCurrency(base, monthlyNet)),
		fmt.Sprintf("Monthly subscr:     %s", renderMoneyWithCurrency(base, monthlySubCost)),
		fmt.Sprintf("Yearly subscr:      %s", renderMoneyWithCurrency(base, yearlySubProjection)),
		fmt.Sprintf("Total accounts:     %s", renderMoneyWithCurrency(base, totalAccounts)),
	)
	leftColumn := strings.Join(leftLines, "\n")

	// Build right column.
	rightLines := []string{sectionTitleStyle.Render("Obligations")}
	rightLines = append(rightLines,
		fmt.Sprintf("Debts (to):         %s", renderMoneyWithCurrency(base, unPaidDebtToMe)),
		fmt.Sprintf("Debts (by):         %s", m.renderMoneyConditionalRed(base, unPaidDebtByMe)),
		fmt.Sprintf("Unpaid taxes:       %s", m.renderMoneyConditionalRed(base, unpaidTaxes)),
		fmt.Sprintf("Invoices (to):      %s", renderMoneyWithCurrency(base, unpaidInvoiceToMe)),
		fmt.Sprintf("Invoices (by):      %s", m.renderMoneyConditionalRed(base, unpaidInvoiceByMe)),
		fmt.Sprintf("Goals progress:     %s", goalsProgressStr),
	)
	rightColumn := strings.Join(rightLines, "\n")

	// Combine columns.
	columnWidth := (width - 8) / 2
	leftView := lipgloss.NewStyle().Width(columnWidth).Render(leftColumn)
	rightView := lipgloss.NewStyle().Width(columnWidth).Render(rightColumn)
	twoColumnDashboard := lipgloss.JoinHorizontal(lipgloss.Top, leftView, "  ", rightView)

	return panelStyle.Width(width).Render(twoColumnDashboard)
}

func (m TheApplication) renderBody(width int) string {
	switch m.screen {
	case screenMenu:
		return m.renderMenu(width)
	case screenAddAccount:
		return m.renderAddAccount(width)
	case screenAccountTable:
		return m.renderAccountTable(width)
	case screenEditAmount:
		return m.renderEditAmount(width)
	case screenSubscriptionNew:
		return m.renderSubscriptionNew(width)
	case screenSubscriptionEdit:
		return m.renderSubscriptionEdit(width)
	case screenSubscriptionList:
		return m.renderSubscriptionList(width)
	case screenSettings:
		return m.renderSettings(width)
	case screenDebtNew:
		return m.renderDebtNew(width)
	case screenDebtList:
		return m.renderDebtList(width)
	case screenDebtEdit:
		return m.renderDebtEdit(width)
	case screenGoalNew:
		return m.renderGoalNew(width)
	case screenGoalList:
		return m.renderGoalList(width)
	case screenGoalEdit:
		return m.renderGoalEdit(width)
	case screenTaxNew:
		return m.renderTaxNew(width)
	case screenTaxList:
		return m.renderTaxList(width)
	case screenTaxEdit:
		return m.renderTaxEdit(width)
	case screenInvoiceNew:
		return m.renderInvoiceNew(width)
	case screenInvoiceList:
		return m.renderInvoiceList(width)
	case screenInvoiceEdit:
		return m.renderInvoiceEdit(width)
	case screenCashflowNew:
		return m.renderCashflowNew(width)
	case screenCashflowHistory:
		return m.renderCashflowHistory(width)
	case screenCashflowOverview:
		return m.renderCashflowOverview(width)
	case screenDataExport:
		return m.renderDataExport(width)
	default:
		return m.renderMenu(width)
	}
}

func (m TheApplication) renderMenu(width int) string {
	dashboard := m.renderDashboard(width)
	groups := appMenuGroups()
	leftColumn := make([]string, 0, len(groups)/2+1)
	rightColumn := make([]string, 0, len(groups)/2+1)

	for gi, group := range groups {
		block := m.renderMenuGroupBlock(gi, group)
		if gi%2 == 0 {
			leftColumn = append(leftColumn, block)
		} else {
			rightColumn = append(rightColumn, block)
		}
	}

	columnWidth := (width - 5) / 2
	leftView := lipgloss.NewStyle().Width(columnWidth).Render(strings.Join(leftColumn, "\n\n"))
	rightView := lipgloss.NewStyle().Width(columnWidth).Render(strings.Join(rightColumn, "\n\n"))
	twoColumnMenu := lipgloss.JoinHorizontal(lipgloss.Top, leftView, "  ", rightView)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		dashboard,
		"",
		headlineStyle.Render("Menu"),
		"",
		twoColumnMenu,
		"",
		mutedStyle.Render("Use up/down to change groups, left/right (or Tab/Shift+Tab) to change buttons, Enter to open, ? for full help."),
		"",
		mutedStyle.Render("Press q to quit."),
		"",
		m.help.View(m.keys),
	)

	return panelStyle.Width(width).Render(content)
}

func (m TheApplication) renderMenuGroupBlock(groupIndex int, group menuGroup) string {
	buttons := make([]string, 0, len(group.items))

	for itemIndex, label := range group.items {
		style := buttonStyle
		if groupIndex == m.menuGroup && itemIndex == m.menuItem {
			style = buttonActiveStyle
		}

		buttons = append(buttons, style.Render(label))
	}

	return fieldLabelStyle.Render(group.title) + "\n" + strings.Join(buttons, " ")
}

func (m TheApplication) renderProgressBar(paid int64, total int64, width int) string {
	if width < 8 {
		width = 8
	}

	if total <= 0 {
		bar := progressRestStyle.Render(strings.Repeat("░", width))
		return bar + " 0.0%"
	}

	if paid < 0 {
		paid = 0
	}

	if paid > total {
		paid = total
	}

	ratio := float64(paid) / float64(total)

	filled := int(math.Round(ratio * float64(width)))
	if filled < 0 {
		filled = 0
	}

	if filled > width {
		filled = width
	}

	bar := progressFillStyle.Render(strings.Repeat("█", filled)) + progressRestStyle.Render(strings.Repeat("░", width-filled))

	return fmt.Sprintf("%s %5.1f%%", bar, ratio*100)
}
