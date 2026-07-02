package app

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/debt"
	"github.com/lazybark/cents/flows/export"
	"github.com/lazybark/cents/flows/goal"
	"github.com/lazybark/cents/flows/invoice"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/subscription"
	"github.com/lazybark/cents/flows/tax"
	storage "github.com/lazybark/cents/storage/sqlite"
	"gorm.io/gorm"
)

type TheApplication struct {
	db                               *gorm.DB
	dbPath                           string
	created                          bool
	screen                           screen
	accounts                         []account.Account
	subscriptions                    []subscription.Subscription
	debts                            []debt.Debt
	goals                            []goal.Goal
	taxes                            []tax.Tax
	invoices                         []invoice.Invoice
	cashflows                        []cashflow.CashflowEntry
	subscriptionMode                 subscriptionListMode
	subscriptionCursor               int
	debtMode                         debtListMode
	debtCursor                       int
	debtLogs                         []debt.DebtLog
	editingDebtID                    uint
	goalMode                         goalListMode
	goalCursor                       int
	goalLogs                         []goal.GoalLog
	editingGoalID                    uint
	taxMode                          taxListMode
	taxCursor                        int
	taxLogs                          []tax.TaxLog
	editingTaxID                     uint
	invoiceMode                      invoiceListMode
	invoiceCursor                    int
	invoicePage                      int
	editingInvoiceID                 uint
	cashflowHistoryMonth             time.Time
	cashflowCursor                   int
	cashflowOverviewPage             int
	settings                         settings.AppSettings
	settingsCursor                   int
	settingsEditMode                 settingsEditMode
	settingsEditInput                textinput.Model
	settingsCurrencyNameInput        textinput.Model
	settingsCurrencyRateInput        textinput.Model
	settingsCurrencyField            int
	settingsCurrencyEditingID        uint
	settingsPaymentMethodNameInput   textinput.Model
	settingsPaymentMethodField       int
	settingsPaymentMethodEditingID   uint
	settingsPaymentMethodTypeOptions []string
	settingsPaymentMethodTypeIndex   int
	settingsPaymentMethodIsDefault   bool
	settingsTaxTypeCountryInput      textinput.Model
	settingsTaxTypeNameInput         textinput.Model
	settingsTaxTypeDescriptionInput  textinput.Model
	settingsTaxTypeURLInput          textinput.Model
	settingsTaxTypeField             int
	settingsTaxTypeEditingID         uint
	settingsIncomeCategoryNameInput  textinput.Model
	settingsIncomeCategoryEditingID  uint
	settingsExpenseCategoryNameInput textinput.Model
	settingsExpenseCategoryEditingID uint
	settingsDeleteConfirm            bool
	settingsDeleteTargetType         string
	settingsDeleteTargetID           uint
	settingsDeleteTargetName         string
	settingsDeleteChoice             int
	deleteConfirmActive              bool
	deleteConfirmType                string
	deleteConfirmID                  uint
	deleteConfirmName                string
	status                           string
	width                            int
	height                           int
	quitting                         bool
	menuGroup                        int
	menuItem                         int
	cursor                           int
	accountSortField                 accountSortField
	accountSortMenu                  bool
	accountSortCursor                int
	addForm                          account.AddAccountForm
	addSubscriptionForm              subscription.AddSubscriptionForm
	addDebtForm                      debt.AddDebtForm
	addGoalForm                      goal.AddGoalForm
	addTaxForm                       tax.AddTaxForm
	addInvoiceForm                   invoice.AddInvoiceForm
	addCashflowForm                  cashflow.AddCashflowForm
	exportForm                       export.ExportForm
	editSubscriptionForm             subscription.EditSubscriptionForm
	editDebtForm                     debt.EditDebtForm
	editGoalForm                     goal.EditGoalForm
	editTaxForm                      tax.EditTaxForm
	editInvoiceForm                  invoice.EditInvoiceForm
	editingSubscriptionID            uint
	editingSubscriptionMode          subscriptionListMode
	editInput                        textinput.Model
	editAmountLogDateInput           textinput.Model
	editAmountLogValueInput          textinput.Model
	editAmountActiveField            int
	editAmountUpdateLog              bool
	editAmountIgnoreInSummaries      bool
	accountValueLogs                 []account.AccountValueLog
	help                             help.Model
	keys                             keyMap
}

func NewApp(db *gorm.DB, dbPath string, created bool, accounts []account.Account, subscriptions []subscription.Subscription, debts []debt.Debt, goals []goal.Goal, taxes []tax.Tax, invoices []invoice.Invoice, cashflows []cashflow.CashflowEntry, settings settings.AppSettings) TheApplication {
	accounts = sortAccounts(accounts, settings, accountSortBaseAmount)
	currencyOptions := currencySelectionOptions(settings)
	accountOptions := accountSelectionOptions(accounts)
	paymentMethodOptions := paymentMethodSelectionOptions(settings)
	addForm := account.NewAddAccountForm(currencyOptions)
	addSubForm := subscription.NewAddSubscriptionForm(currencyOptions, paymentMethodOptions)
	addDebtForm := debt.NewAddDebtForm(currencyOptions)
	addGoalForm := goal.NewAddGoalForm(currencyOptions)
	addTaxForm := tax.NewAddTaxForm(settings.TaxTypes)
	addInvoiceForm := invoice.NewAddInvoiceForm(currencyOptions, accountOptions)
	addCashflowForm := cashflow.NewAddCashflowForm(currencyOptions, incomeCategorySelectionOptions(settings), accountOptions, true)
	exportForm := export.NewExportForm(defaultExportPath())
	editInput := textinput.New()
	editInput.Placeholder = "1234.56"
	editInput.CharLimit = 24
	editInput.Width = 20
	settingsInput := textinput.New()
	settingsInput.Placeholder = "$"
	settingsInput.CharLimit = 24
	settingsInput.Width = 20
	settingsCurrencyNameInput := textinput.New()
	settingsCurrencyNameInput.Placeholder = "EUR"
	settingsCurrencyNameInput.CharLimit = 24
	settingsCurrencyNameInput.Width = 20
	settingsCurrencyRateInput := textinput.New()
	settingsCurrencyRateInput.Placeholder = "1.23"
	settingsCurrencyRateInput.CharLimit = 24
	settingsCurrencyRateInput.Width = 20
	settingsPaymentMethodNameInput := textinput.New()
	settingsPaymentMethodNameInput.Placeholder = "Personal Visa"
	settingsPaymentMethodNameInput.CharLimit = 40
	settingsPaymentMethodNameInput.Width = 28
	settingsTaxTypeCountryInput := textinput.New()
	settingsTaxTypeCountryInput.Placeholder = "Netherlands"
	settingsTaxTypeCountryInput.CharLimit = 60
	settingsTaxTypeCountryInput.Width = 28
	settingsTaxTypeNameInput := textinput.New()
	settingsTaxTypeNameInput.Placeholder = "Income Tax"
	settingsTaxTypeNameInput.CharLimit = 80
	settingsTaxTypeNameInput.Width = 28
	settingsTaxTypeDescriptionInput := textinput.New()
	settingsTaxTypeDescriptionInput.Placeholder = "optional description"
	settingsTaxTypeDescriptionInput.CharLimit = 140
	settingsTaxTypeDescriptionInput.Width = 36
	settingsTaxTypeURLInput := textinput.New()
	settingsTaxTypeURLInput.Placeholder = "optional https://..."
	settingsTaxTypeURLInput.CharLimit = 180
	settingsTaxTypeURLInput.Width = 40
	settingsIncomeCategoryNameInput := textinput.New()
	settingsIncomeCategoryNameInput.Placeholder = "Salary"
	settingsIncomeCategoryNameInput.CharLimit = 80
	settingsIncomeCategoryNameInput.Width = 30
	settingsExpenseCategoryNameInput := textinput.New()
	settingsExpenseCategoryNameInput.Placeholder = "Groceries"
	settingsExpenseCategoryNameInput.CharLimit = 80
	settingsExpenseCategoryNameInput.Width = 30
	helpModel := help.New()
	helpModel.ShowAll = false
	helpModel.Width = 0

	status := databaseStatus(created, len(accounts), dbPath)
	if len(accounts) == 0 {
		status += " | no accounts yet"
	}

	return TheApplication{
		db:                               db,
		dbPath:                           dbPath,
		created:                          created,
		screen:                           screenMenu,
		accounts:                         accounts,
		subscriptions:                    subscriptions,
		debts:                            debts,
		goals:                            goals,
		taxes:                            taxes,
		invoices:                         invoices,
		cashflows:                        cashflows,
		debtMode:                         debtListOutgoing,
		debtCursor:                       0,
		debtLogs:                         nil,
		editingDebtID:                    0,
		goalMode:                         goalListActive,
		goalCursor:                       0,
		goalLogs:                         nil,
		editingGoalID:                    0,
		taxMode:                          taxListUnpaid,
		taxCursor:                        0,
		taxLogs:                          nil,
		editingTaxID:                     0,
		invoiceMode:                      invoiceListOutgoingUnpaid,
		invoiceCursor:                    0,
		invoicePage:                      0,
		editingInvoiceID:                 0,
		cashflowHistoryMonth:             beginningOfMonth(time.Now()),
		cashflowCursor:                   0,
		settings:                         settings,
		settingsCursor:                   0,
		settingsEditMode:                 settingsEditNone,
		settingsEditInput:                settingsInput,
		settingsCurrencyNameInput:        settingsCurrencyNameInput,
		settingsCurrencyRateInput:        settingsCurrencyRateInput,
		settingsCurrencyField:            0,
		settingsCurrencyEditingID:        0,
		settingsPaymentMethodNameInput:   settingsPaymentMethodNameInput,
		settingsPaymentMethodField:       0,
		settingsPaymentMethodEditingID:   0,
		settingsPaymentMethodTypeOptions: []string{"Card", "Crypto", "E-Wallet", "Other"},
		settingsPaymentMethodTypeIndex:   0,
		settingsPaymentMethodIsDefault:   false,
		settingsTaxTypeCountryInput:      settingsTaxTypeCountryInput,
		settingsTaxTypeNameInput:         settingsTaxTypeNameInput,
		settingsTaxTypeDescriptionInput:  settingsTaxTypeDescriptionInput,
		settingsTaxTypeURLInput:          settingsTaxTypeURLInput,
		settingsTaxTypeField:             0,
		settingsTaxTypeEditingID:         0,
		settingsIncomeCategoryNameInput:  settingsIncomeCategoryNameInput,
		settingsIncomeCategoryEditingID:  0,
		settingsExpenseCategoryNameInput: settingsExpenseCategoryNameInput,
		settingsExpenseCategoryEditingID: 0,
		settingsDeleteConfirm:            false,
		settingsDeleteTargetType:         "",
		settingsDeleteTargetID:           0,
		settingsDeleteTargetName:         "",
		settingsDeleteChoice:             1,
		subscriptionMode:                 subscriptionListActive,
		subscriptionCursor:               0,
		status:                           status,
		menuGroup:                        0,
		menuItem:                         0,
		accountSortField:                 accountSortBaseAmount,
		accountSortMenu:                  false,
		accountSortCursor:                0,
		addForm:                          addForm,
		addSubscriptionForm:              addSubForm,
		addDebtForm:                      addDebtForm,
		addGoalForm:                      addGoalForm,
		addTaxForm:                       addTaxForm,
		addInvoiceForm:                   addInvoiceForm,
		addCashflowForm:                  addCashflowForm,
		exportForm:                       exportForm,
		editSubscriptionForm:             subscription.NewEditSubscriptionForm(),
		editDebtForm:                     debt.NewEditDebtForm(),
		editGoalForm:                     goal.NewEditGoalForm(),
		editTaxForm:                      tax.NewEditTaxForm(),
		editInvoiceForm:                  invoice.NewEditInvoiceForm(currencyOptions, accountOptions),
		editInput:                        editInput,
		help:                             helpModel,
		keys:                             newKeyMap(),
	}
}

func (m TheApplication) rateToBase(currency string) (float64, bool) {
	target := strings.TrimSpace(currency)
	if target == "" {
		return 0, false
	}

	for _, entry := range m.settings.Currencies {
		if strings.EqualFold(strings.TrimSpace(entry.CurrencyName), target) && entry.RateToBase > 0 {
			return entry.RateToBase, true
		}
	}

	return 0, false
}

func (m TheApplication) filteredSubscriptions() []subscription.Subscription {
	if m.subscriptionMode == subscriptionListAll {
		return m.subscriptions
	}

	filtered := make([]subscription.Subscription, 0, len(m.subscriptions))

	for _, sub := range m.subscriptions {
		if sub.IsActive {
			filtered = append(filtered, sub)
		}
	}

	return filtered
}

func (m TheApplication) convertedAmountForBase(currency string, cents int64) string {
	base := m.baseCurrencyLabel()

	if strings.EqualFold(strings.TrimSpace(currency), base) {
		return ""
	}

	convertedCents, ok := m.convertToBaseCents(currency, cents)
	if !ok {
		return ""
	}

	return renderMoneyWithCurrency(base, convertedCents)
}

func (m TheApplication) baseCurrencyLabel() string {
	base := strings.TrimSpace(m.settings.BaseCurrency)
	if base == "" {
		return "$"
	}

	return base
}

func (m TheApplication) convertToBaseCents(currency string, cents int64) (int64, bool) {
	base := m.baseCurrencyLabel()
	if strings.EqualFold(strings.TrimSpace(currency), base) {
		return cents, true
	}

	rate, ok := m.rateToBase(currency)
	if !ok {
		return 0, false
	}

	return int64(math.Round(float64(cents) * rate)), true
}

func (m TheApplication) sumAccountsInBaseCents() int64 {
	var total int64

	for _, acct := range m.accounts {
		if acct.IgnoreInSummaries {
			continue
		}

		converted, ok := m.convertToBaseCents(acct.Currency, acct.BalanceCents)
		if !ok {
			continue
		}

		total += converted
	}

	return total
}

func (m TheApplication) subscriptionTotalsInBaseCents(subs []subscription.Subscription) (monthlyTotal int64, yearlyProjection int64) {
	var yearlyOnly int64

	for _, sub := range subs {
		converted, ok := m.convertToBaseCents(sub.Currency, sub.AmountCents)
		if !ok {
			continue
		}

		if strings.EqualFold(strings.TrimSpace(sub.Period), "month") {
			monthlyTotal += converted

			continue
		}

		if strings.EqualFold(strings.TrimSpace(sub.Period), "year") {
			yearlyOnly += converted
		}
	}

	yearlyProjection = yearlyOnly + monthlyTotal*12

	return monthlyTotal, yearlyProjection
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

func (m TheApplication) renderCashflowOverviewRow(width int, row cashflow.CashflowMonthlyOverviewRow, base string) string {
	monthWidth := 14
	incomeWidth := 14
	expenseWidth := 14
	netWidth := 14
	compareWidth := max(width-14-monthWidth-incomeWidth-expenseWidth-netWidth-12, 16)

	compareText := "-"

	if row.HasPrev {
		compareText = renderMoneyWithCurrency(base, row.DeltaFromPrev)
		if row.DeltaFromPrev > 0 {
			compareText = "+" + compareText
			compareText = positiveStyle.Render(compareText)
		} else if row.DeltaFromPrev < 0 {
			compareText = obligationStyle.Render(compareText)
		}
	}

	netText := renderSignedMoneyWithCurrency(base, row.NetBase)
	monthCell := lipgloss.NewStyle().Width(monthWidth).Align(lipgloss.Left).Render(row.Month.Format("2006-01"))
	incomeCell := lipgloss.NewStyle().Width(incomeWidth).Align(lipgloss.Left).Render(renderMoneyWithCurrency(base, row.IncomeBase))
	expenseCell := lipgloss.NewStyle().Width(expenseWidth).Align(lipgloss.Left).Render(renderMoneyWithCurrency(base, row.ExpenseBase))
	netCell := lipgloss.NewStyle().Width(netWidth).Align(lipgloss.Left).Render(netText)
	compareCell := lipgloss.NewStyle().Width(compareWidth).Align(lipgloss.Left).Render(compareText)
	line := strings.Join([]string{monthCell, incomeCell, expenseCell, netCell, compareCell}, " ")

	return rowStyle.Render(line)
}

func (m TheApplication) renderCashflowHistoryHeader(width int) string {
	dateWidth := 10
	typeWidth := 8
	currencyWidth := 8
	amountWidth := 12
	categoryWidth := 32
	accountWidth := 28

	commentWidth := width - 14 - dateWidth - typeWidth - currencyWidth - amountWidth - categoryWidth - accountWidth - 18
	if commentWidth < 12 {
		commentWidth = 12
	}

	header := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", "#", dateWidth, "Date", typeWidth, "Type", currencyWidth, "Curr", amountWidth, "Amount", categoryWidth, "Category", accountWidth, "Account", commentWidth, "Comment")

	return tableHeaderStyle.Render(header)
}

func (m TheApplication) renderCashflowHistoryRow(width int, index int, item cashflow.CashflowEntry) string {
	dateWidth := 10
	typeWidth := 8
	currencyWidth := 8
	amountWidth := 12
	categoryWidth := 32
	accountWidth := 28

	commentWidth := width - 14 - dateWidth - typeWidth - currencyWidth - amountWidth - categoryWidth - accountWidth - 18
	if commentWidth < 12 {
		commentWidth = 12
	}

	prefix := " "
	style := rowStyle

	if index == m.cashflowCursor {
		prefix = ">"
		style = selectedRowStyle
	}

	typeLabel := "expense"

	if item.IsIncome {
		typeLabel = "income"
	}

	accountLabel := strings.TrimSpace(item.AccountName)
	if accountLabel == "" {
		accountLabel = "-"
	}

	row := fmt.Sprintf("%s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", prefix, dateWidth, item.EntryDate.Local().Format("2006-01-02"), typeWidth, typeLabel, currencyWidth, truncateText(item.Currency, currencyWidth), amountWidth, renderMoneyWithCurrency(item.Currency, item.AmountCents), categoryWidth, truncateText(item.Category, categoryWidth), accountWidth, truncateText(accountLabel, accountWidth), commentWidth, truncateText(item.Comment, commentWidth))

	return style.Render(row)
}

func (m TheApplication) renderEditTaxField(field int, label string, value string) string {
	prefix := "  "

	if m.editTaxForm.ActiveField == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderSettings(width int) string {
	header := []string{sectionTitleStyle.Render("Settings"), hintStyle.Render("Use up/down to select rows. Enter edits selected row. Esc returns to menu."), ""}
	header = append(header, sectionTitleStyle.Render("General"))

	basePrefix := " "
	if m.settingsCursor == 0 {
		basePrefix = ">"
	}

	baseCurrencyValue := m.settings.BaseCurrency

	if m.settingsEditMode == settingsEditBaseCurrency {
		baseCurrencyValue = m.settingsEditInput.View()
	}

	rows := []string{fmt.Sprintf("%s %-18s %s", basePrefix, "base_currency", baseCurrencyValue), "", sectionTitleStyle.Render("Currencies (relation to base currency)")}

	for index, currency := range m.settings.Currencies {
		prefix := " "
		if m.settingsCursor == index+1 {
			prefix = ">"
		}

		rows = append(rows, fmt.Sprintf("%s %-16s rate=%s", prefix, truncateText(currency.CurrencyName, 16), formatRate(currency.RateToBase)))
	}

	addPrefix := " "
	if m.settingsCursor == len(m.settings.Currencies)+1 {
		addPrefix = ">"
	}

	rows = append(rows, fmt.Sprintf("%s + add currency", addPrefix))
	rows = append(rows, "", sectionTitleStyle.Render("Payment methods"))
	paymentStart := settingsPaymentMethodStartCursor(m.settings)

	for index, method := range m.settings.PaymentMethods {
		prefix := " "
		cursor := paymentStart + index

		if m.settingsCursor == cursor {
			prefix = ">"
		}

		defaultTag := ""

		if method.IsDefault {
			defaultTag = " [default]"
		}

		rows = append(rows, fmt.Sprintf("%s %-16s %-10s%s", prefix, truncateText(method.PaymentMethodName, 16), truncateText(method.PaymentMethodType, 10), defaultTag))
	}

	methodAddPrefix := " "

	if m.settingsCursor == settingsPaymentMethodAddCursor(m.settings) {
		methodAddPrefix = ">"
	}

	rows = append(rows, fmt.Sprintf("%s + add payment method", methodAddPrefix))
	rows = append(rows, "", sectionTitleStyle.Render("Tax types"))
	taxStart := settingsTaxTypeStartCursor(m.settings)

	for index, taxType := range m.settings.TaxTypes {
		prefix := " "
		cursor := taxStart + index

		if m.settingsCursor == cursor {
			prefix = ">"
		}

		description := strings.TrimSpace(taxType.Description)
		if description == "" {
			description = "-"
		}

		rows = append(rows, fmt.Sprintf("%s %-14s %-16s %s", prefix, truncateText(taxType.Country, 14), truncateText(taxType.TaxTypeName, 16), truncateText(description, 26)))
	}

	taxAddPrefix := " "
	if m.settingsCursor == settingsTaxTypeAddCursor(m.settings) {
		taxAddPrefix = ">"
	}

	rows = append(rows, fmt.Sprintf("%s + add tax type", taxAddPrefix))
	rows = append(rows, "", sectionTitleStyle.Render("Income categories"))
	incomeStart := settingsIncomeCategoryStartCursor(m.settings)

	for index, category := range m.settings.IncomeCategories {
		prefix := " "
		cursor := incomeStart + index

		if m.settingsCursor == cursor {
			prefix = ">"
		}

		rows = append(rows, fmt.Sprintf("%s %-24s", prefix, truncateText(category.CategoryName, 24)))
	}

	incomeAddPrefix := " "

	if m.settingsCursor == settingsIncomeCategoryAddCursor(m.settings) {
		incomeAddPrefix = ">"
	}

	rows = append(rows, fmt.Sprintf("%s + add income category", incomeAddPrefix))
	rows = append(rows, "", sectionTitleStyle.Render("Expense categories"))
	expenseStart := settingsExpenseCategoryStartCursor(m.settings)

	for index, category := range m.settings.ExpenseCategories {
		prefix := " "
		cursor := expenseStart + index

		if m.settingsCursor == cursor {
			prefix = ">"
		}

		rows = append(rows, fmt.Sprintf("%s %-24s", prefix, truncateText(category.CategoryName, 24)))
	}

	expenseAddPrefix := " "

	if m.settingsCursor == settingsExpenseCategoryAddCursor(m.settings) {
		expenseAddPrefix = ">"
	}

	rows = append(rows, fmt.Sprintf("%s + add expense category", expenseAddPrefix))

	if m.settingsEditMode == settingsEditBaseCurrency {
		rows = append(rows, "", mutedStyle.Render("Editing base currency: Enter saves, Esc exits settings."))
	}

	if m.settingsEditMode == settingsEditCurrency {
		namePrefix := " "
		ratePrefix := " "

		if m.settingsCurrencyField == 0 {
			namePrefix = ">"
		} else {
			ratePrefix = ">"
		}

		rows = append(rows, "")
		rows = append(rows, fieldLabelStyle.Render("Currency form"))
		rows = append(rows, fmt.Sprintf("%s Name  %s", namePrefix, m.settingsCurrencyNameInput.View()))
		rows = append(rows, fmt.Sprintf("%s Rate  %s", ratePrefix, m.settingsCurrencyRateInput.View()))
		rows = append(rows, mutedStyle.Render("Enter on Name moves to Rate. Enter on Rate saves. Esc exits settings."))
	}

	if m.settingsEditMode == settingsEditPaymentMethod {
		namePrefix := " "
		typePrefix := " "
		defaultPrefix := " "

		if m.settingsPaymentMethodField == 0 {
			namePrefix = ">"
		}

		if m.settingsPaymentMethodField == 1 {
			typePrefix = ">"
		}

		if m.settingsPaymentMethodField == 2 {
			defaultPrefix = ">"
		}

		typeOptions := make([]string, 0, len(m.settingsPaymentMethodTypeOptions))
		for i, option := range m.settingsPaymentMethodTypeOptions {
			style := buttonStyle

			if i == m.settingsPaymentMethodTypeIndex {
				style = buttonActiveStyle
			}

			typeOptions = append(typeOptions, style.Render(option))
		}

		defaultMarker := "[ ]"

		if m.settingsPaymentMethodIsDefault {
			defaultMarker = "[x]"
		}

		rows = append(rows, "")
		rows = append(rows, fieldLabelStyle.Render("Payment method form"))
		rows = append(rows, fmt.Sprintf("%s Name     %s", namePrefix, m.settingsPaymentMethodNameInput.View()))
		rows = append(rows, fmt.Sprintf("%s Type     %s", typePrefix, strings.Join(typeOptions, " ")))
		rows = append(rows, fmt.Sprintf("%s Default  %s", defaultPrefix, defaultMarker))
		rows = append(rows, mutedStyle.Render("Enter/Tab moves fields. Left/right changes type. Space toggles default. Enter on Default saves. Esc exits settings."))
	}

	if m.settingsEditMode == settingsEditTaxType {
		countryPrefix := " "
		namePrefix := " "
		descPrefix := " "
		urlPrefix := " "

		if m.settingsTaxTypeField == 0 {
			countryPrefix = ">"
		}

		if m.settingsTaxTypeField == 1 {
			namePrefix = ">"
		}

		if m.settingsTaxTypeField == 2 {
			descPrefix = ">"
		}

		if m.settingsTaxTypeField == 3 {
			urlPrefix = ">"
		}

		rows = append(rows, "")
		rows = append(rows, fieldLabelStyle.Render("Tax type form"))
		rows = append(rows, fmt.Sprintf("%s Country      %s", countryPrefix, m.settingsTaxTypeCountryInput.View()))
		rows = append(rows, fmt.Sprintf("%s Tax type     %s", namePrefix, m.settingsTaxTypeNameInput.View()))
		rows = append(rows, fmt.Sprintf("%s Description  %s", descPrefix, m.settingsTaxTypeDescriptionInput.View()))
		rows = append(rows, fmt.Sprintf("%s URL          %s", urlPrefix, m.settingsTaxTypeURLInput.View()))
		rows = append(rows, mutedStyle.Render("Enter/Tab moves fields. Enter on URL saves. Esc exits settings."))
	}

	if m.settingsEditMode == settingsEditIncomeCategory {
		rows = append(rows, "")
		rows = append(rows, fieldLabelStyle.Render("Income category form"))
		rows = append(rows, "> Name  "+m.settingsIncomeCategoryNameInput.View())
		rows = append(rows, mutedStyle.Render("Enter saves. Esc exits edit."))
	}

	if m.settingsEditMode == settingsEditExpenseCategory {
		rows = append(rows, "")
		rows = append(rows, fieldLabelStyle.Render("Expense category form"))
		rows = append(rows, "> Name  "+m.settingsExpenseCategoryNameInput.View())
		rows = append(rows, mutedStyle.Render("Enter saves. Esc exits edit."))
	}

	if m.settingsDeleteConfirm {
		yesStyle := buttonStyle
		noStyle := buttonStyle

		if m.settingsDeleteChoice == 0 {
			yesStyle = buttonActiveStyle
		} else {
			noStyle = buttonActiveStyle
		}

		rows = append(rows, "")
		rows = append(rows, fieldLabelStyle.Render("Confirm delete"))
		rows = append(rows, fmt.Sprintf("Are you sure you want to delete %s?", m.settingsDeleteTargetName))
		rows = append(rows, yesStyle.Render("Yes")+" "+noStyle.Render("No"))
		rows = append(rows, mutedStyle.Render("Left/Right selects. Enter confirms. Esc cancels."))
	}

	content := append(header, rows...)

	return panelStyle.Width(width).Render(strings.Join(content, "\n"))
}

func (m TheApplication) renderInvoiceRowText(field int, label string, value string) string {
	prefix := "  "
	if m.addInvoiceForm.Active == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderInvoiceChoiceRow(field int, label string, options []string, selected int) string {
	prefix := "  "
	if m.addInvoiceForm.Active == field {
		prefix = "> "
	}

	chips := make([]string, 0, len(options))

	for i, option := range options {
		style := buttonStyle
		if i == selected {
			style = buttonActiveStyle
		}

		chips = append(chips, style.Render(option))
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + strings.Join(chips, " ")
}

func (m TheApplication) renderEditInvoiceField(field int, label string, value string) string {
	prefix := "  "
	if m.editInvoiceForm.ActiveField == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderEditInvoiceChoiceRow(field int, label string, options []string, selected int) string {
	prefix := "  "
	if m.editInvoiceForm.ActiveField == field {
		prefix = "> "
	}

	chips := make([]string, 0, len(options))
	for i, option := range options {
		style := buttonStyle
		if i == selected {
			style = buttonActiveStyle
		}

		chips = append(chips, style.Render(option))
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + strings.Join(chips, " ")
}

func (m TheApplication) renderInvoiceTableHeader(width int) string {
	titleWidth := 14
	typeWidth := 8
	currencyWidth := 7
	amountWidth := 12
	paidWidth := 6
	peerWidth := 12
	invDateWidth := 10
	dueDateWidth := 10
	accountWidth := 16

	descWidth := width - 14 - titleWidth - typeWidth - currencyWidth - amountWidth - paidWidth - peerWidth - invDateWidth - dueDateWidth - accountWidth - 20
	if descWidth < 16 {
		descWidth = 16
	}

	header := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", "#", titleWidth, "Title", typeWidth, "Type", currencyWidth, "Curr", amountWidth, "Amount", paidWidth, "Paid", peerWidth, "Peer", invDateWidth, "Issued", dueDateWidth, "Due", accountWidth, "Account", descWidth, "Description")

	return tableHeaderStyle.Render(header)
}

func (m TheApplication) renderInvoiceTableRow(width int, index int, item invoice.Invoice) string {
	titleWidth := 14
	typeWidth := 8
	currencyWidth := 7
	amountWidth := 12
	paidWidth := 6
	peerWidth := 12
	invDateWidth := 10
	dueDateWidth := 10
	accountWidth := 16

	descWidth := width - 14 - titleWidth - typeWidth - currencyWidth - amountWidth - paidWidth - peerWidth - invDateWidth - dueDateWidth - accountWidth - 20
	if descWidth < 16 {
		descWidth = 16
	}

	prefix := " "
	style := rowStyle
	if index == m.invoiceCursor {
		prefix = ">"
		style = selectedRowStyle
	}

	typeLabel := "in"
	if !item.IsIncoming {
		typeLabel = "out"
	}

	paidLabel := "no"
	if item.Paid {
		paidLabel = "yes"
	}

	issued := "-"
	if item.InvoiceDate != nil {
		issued = item.InvoiceDate.Local().Format("2006-01-02")
	}

	due := "-"
	if item.DueDate != nil {
		due = item.DueDate.Local().Format("2006-01-02")
	}

	row := fmt.Sprintf("%s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", prefix, titleWidth, truncateText(item.Title, titleWidth), typeWidth, typeLabel, currencyWidth, truncateText(item.Currency, currencyWidth), amountWidth, renderMoneyWithCurrency(item.Currency, item.AmountCents), paidWidth, paidLabel, peerWidth, truncateText(item.Peer, peerWidth), invDateWidth, issued, dueDateWidth, due, accountWidth, truncateText(item.TargetAccount, accountWidth), descWidth, truncateText(item.Description, descWidth))

	return style.Render(row)
}

func (m TheApplication) renderCashflowNew(width int) string {
	title := "New expense"
	if m.addCashflowForm.IsIncome {
		title = "New income"
	}

	lines := []string{
		headlineStyle.Render(title),
		mutedStyle.Render("Currency, amount, date, category, optional account and comment. Date defaults to today."),
		"",
	}

	if len(m.addCashflowForm.CategoryOptions) == 0 {
		if m.addCashflowForm.IsIncome {
			lines = append(lines, mutedStyle.Render("No income categories configured. Add one in Settings first."))
		} else {
			lines = append(lines, mutedStyle.Render("No expense categories configured. Add one in Settings first."))
		}

		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	lines = append(lines,
		m.renderCashflowChoiceRow(cashflow.CashflowFieldCurrency, "Currency", m.addCashflowForm.CurrencyOptions, m.addCashflowForm.CurrencyIndex),
		m.renderCashflowTextRow(cashflow.CashflowFieldAmount, "Amount", m.addCashflowForm.Inputs[0].View()),
		m.renderCashflowTextRow(cashflow.CashflowFieldDate, "Date", m.addCashflowForm.Inputs[1].View()),
		m.renderCashflowChoiceRow(cashflow.CashflowFieldCategory, "Category", m.addCashflowForm.CategoryOptions, m.addCashflowForm.CategoryIndex),
		m.renderCashflowChoiceRow(cashflow.CashflowFieldAccount, "Account (optional)", cashflowAccountDisplayOptions(m.addCashflowForm.AccountOptions), m.addCashflowForm.AccountIndex),
		m.renderCashflowTextRow(cashflow.CashflowFieldComment, "Comment", m.addCashflowForm.Inputs[2].View()),
	)

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderCashflowTextRow(field int, label string, value string) string {
	prefix := "  "
	if m.addCashflowForm.Active == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderCashflowChoiceRow(field int, label string, options []string, selected int) string {
	prefix := "  "
	if m.addCashflowForm.Active == field {
		prefix = "> "
	}

	chips := make([]string, 0, len(options))

	for i, option := range options {
		style := buttonStyle
		if i == selected {
			style = buttonActiveStyle
		}

		chips = append(chips, style.Render(option))
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + strings.Join(chips, " ")
}

func (m TheApplication) renderCashflowHistory(width int) string {
	monthLabel := m.cashflowHistoryMonth.Format("January 2006")
	lines := []string{
		lipgloss.JoinHorizontal(lipgloss.Center, sectionTitleStyle.Render("Income/Expense history"), "  ", modeBadgeStyle.Render("Monthly")),
		hintStyle.Render("Left/right changes month. Up/down scrolls rows. Delete/Backspace asks confirmation. Esc returns to menu."),
		"",
		fieldLabelStyle.Render("Month") + "  " + buttonStyle.Render("<") + " " + buttonActiveStyle.Render(monthLabel) + " " + buttonStyle.Render(">"),
		"",
	}

	items := m.filteredCashflowsForMonth()
	if len(items) == 0 {
		lines = append(lines, mutedStyle.Render("No records for this month."))

		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	var incomeTotal int64
	var expenseTotal int64

	missingRates := 0
	for _, item := range items {
		amountBase, ok := m.convertToBaseCents(item.Currency, item.AmountCents)
		if !ok {
			missingRates++

			continue
		}

		if item.IsIncome {
			incomeTotal += amountBase
		} else {
			expenseTotal += amountBase
		}
	}

	net := incomeTotal - expenseTotal
	base := m.baseCurrencyLabel()
	netLabel := renderSignedMoneyWithCurrency(base, net)
	lines = append(lines, fmt.Sprintf("Income: %s  Expense: %s  Net: %s", renderMoneyWithCurrency(base, incomeTotal), renderMoneyWithCurrency(base, expenseTotal), netLabel))

	if missingRates > 0 {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("%d record(s) excluded from summary due to missing conversion rate.", missingRates)))
	}

	lines = append(lines, "")
	lines = append(lines, m.renderCashflowHistoryHeader(width))

	for i, item := range items {
		lines = append(lines, m.renderCashflowHistoryRow(width, i, item))
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderCashflowOverview(width int) string {
	base := m.baseCurrencyLabel()
	lines := []string{
		lipgloss.JoinHorizontal(lipgloss.Center, sectionTitleStyle.Render("Income/Expense overview"), "  ", modeBadgeStyle.Render("Monthly")),
		hintStyle.Render("Month-by-month totals in base currency (newest first). Up/down or left/right changes page. Esc returns to menu."),
		"",
	}

	rows, missingRates := m.monthlyCashflowOverviewRows()
	if len(rows) == 0 {
		lines = append(lines, mutedStyle.Render("No records available for overview."))
		if missingRates > 0 {
			lines = append(lines, mutedStyle.Render(fmt.Sprintf("%d record(s) skipped due to missing conversion rate.", missingRates)))
		}

		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	pageSize := m.cashflowOverviewPageSize()

	totalPages := (len(rows) + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}

	currentPage := clamp(m.cashflowOverviewPage, 0, totalPages-1)
	startFromEnd := len(rows) - currentPage*pageSize

	endFromEnd := startFromEnd - pageSize
	if endFromEnd < 0 {
		endFromEnd = 0
	}

	lines = append(lines, fmt.Sprintf("Page %d/%d", currentPage+1, totalPages), "")
	lines = append(lines, m.renderCashflowOverviewHeader(width, base))

	for i := startFromEnd - 1; i >= endFromEnd; i-- {
		lines = append(lines, m.renderCashflowOverviewRow(width, rows[i], base))
	}

	if missingRates > 0 {
		lines = append(lines, "", mutedStyle.Render(fmt.Sprintf("%d record(s) skipped due to missing conversion rate.", missingRates)))
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderCashflowOverviewHeader(width int, base string) string {
	monthWidth := 14
	incomeWidth := 14
	expenseWidth := 14
	netWidth := 14

	compareWidth := width - 14 - monthWidth - incomeWidth - expenseWidth - netWidth - 12
	if compareWidth < 16 {
		compareWidth = 16
	}

	header := fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s", monthWidth, "Month", incomeWidth, "Income ("+base+")", expenseWidth, "Expense ("+base+")", netWidth, "Net ("+base+")", compareWidth, "Net vs previous")

	return tableHeaderStyle.Render(header)
}

func (m TheApplication) renderGoalTableRow(width int, index int, item goal.Goal) string {
	nameWidth := 12
	currencyWidth := 8
	targetWidth := 12
	accumWidth := 12
	leftWidth := 12
	startedWidth := 10
	targetDateWidth := 10
	progressWidth := 8
	descWidth := width - 14 - nameWidth - currencyWidth - targetWidth - accumWidth - leftWidth - startedWidth - targetDateWidth - progressWidth - 18
	if descWidth < 18 {
		descWidth = 18
	}

	prefix := " "
	style := rowStyle
	if index == m.goalCursor {
		prefix = ">"
		style = selectedRowStyle
	}

	left := item.TargetAmountCents - item.AmountAccumulatedCents
	if left < 0 {
		left = 0
	}

	targetDate := "-"
	if item.TargetDate != nil {
		targetDate = item.TargetDate.Local().Format("2006-01-02")
	}

	progressValue := 0.0
	if item.TargetAmountCents > 0 {
		progressValue = (float64(item.AmountAccumulatedCents) / float64(item.TargetAmountCents)) * 100
		if progressValue < 0 {
			progressValue = 0
		}
		if progressValue > 100 {
			progressValue = 100
		}
	}

	row := fmt.Sprintf("%s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", prefix, nameWidth, truncateText(item.Name, nameWidth), currencyWidth, truncateText(item.Currency, currencyWidth), targetWidth, renderMoneyWithCurrency(item.Currency, item.TargetAmountCents), accumWidth, renderMoneyWithCurrency(item.Currency, item.AmountAccumulatedCents), leftWidth, renderMoneyWithCurrency(item.Currency, left), startedWidth, item.DateStartedAt.Local().Format("2006-01-02"), targetDateWidth, targetDate, progressWidth, fmt.Sprintf("%5.1f%%", progressValue), descWidth, truncateText(item.Description, descWidth))

	return style.Render(row)
}

func (m TheApplication) renderGoalEdit(width int) string {
	lines := []string{
		headlineStyle.Render("Edit goal"),
		mutedStyle.Render("Edit fields and press Enter on Description to save."),
		"",
		mutedStyle.Render("Goal: " + m.editGoalForm.NameLabel + " | Currency: " + m.editGoalForm.CurrencyLabel),
	}

	if idx := m.findGoalIndex(m.editingGoalID); idx >= 0 {
		item := m.goals[idx]
		lines = append(lines, fieldLabelStyle.Render("Accumulation progress"))
		lines = append(lines, "  "+m.renderProgressBar(item.AmountAccumulatedCents, item.TargetAmountCents, 28))
	}

	lines = append(lines,
		"",
		m.renderEditGoalField(goal.EditGoalFieldTargetAmount, "Target amount", m.editGoalForm.TargetAmountInput.View()),
		m.renderEditGoalField(goal.EditGoalFieldAccumulated, "Accumulated", m.editGoalForm.AccumulatedAmountInput.View()),
		m.renderEditGoalField(goal.EditGoalFieldDateStarted, "Date started", m.editGoalForm.DateStartedInput.View()),
		m.renderEditGoalField(goal.EditGoalFieldTargetDate, "Target date", m.editGoalForm.TargetDateInput.View()),
		m.renderEditGoalField(goal.EditGoalFieldDescription, "Description", m.editGoalForm.DescriptionInput.View()),
		"",
		fieldLabelStyle.Render("Add transaction"),
		mutedStyle.Render("Set delta/date/comment, then press Enter on Transaction comment to apply."),
		m.renderEditGoalField(goal.EditGoalFieldLogDelta, "Transaction delta", m.editGoalForm.LogDeltaInput.View()),
		m.renderEditGoalField(goal.EditGoalFieldLogDate, "Transaction date", m.editGoalForm.LogDateInput.View()),
		m.renderEditGoalField(goal.EditGoalFieldLogComment, "Transaction comment", m.editGoalForm.LogCommentInput.View()),
		"",
		fieldLabelStyle.Render("Logs"),
	)

	if len(m.goalLogs) == 0 {
		lines = append(lines, mutedStyle.Render("No log entries yet."))
	} else {
		for _, entry := range m.goalLogs {
			sign := "+"
			if entry.DeltaAccumulatedCents < 0 {
				sign = ""
			}

			note := strings.TrimSpace(entry.Note)
			if note != "" {
				lines = append(lines, fmt.Sprintf("%s%s at %s | %s", sign, formatAmount(entry.DeltaAccumulatedCents), entry.CreatedAt.Local().Format("2006-01-02 15:04"), note))
			} else {
				lines = append(lines, fmt.Sprintf("%s%s at %s", sign, formatAmount(entry.DeltaAccumulatedCents), entry.CreatedAt.Local().Format("2006-01-02 15:04")))
			}
		}
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderEditGoalField(field int, label string, value string) string {
	prefix := "  "
	if m.editGoalForm.ActiveField == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderTaxNew(width int) string {
	lines := []string{
		headlineStyle.Render("New tax"),
		mutedStyle.Render("Use up/down to move fields. Left/right changes tax type. Enter on last field saves."),
		"",
	}

	if len(m.addTaxForm.TaxTypeOptions) == 0 {
		lines = append(lines, mutedStyle.Render("No tax types configured. Add one in Settings first."))
		lines = append(lines, mutedStyle.Render("Press Esc to go back."))

		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	lines = append(lines,
		m.renderTaxChoiceRow(tax.TaxFieldTaxType, "Tax type", m.addTaxForm.TaxDisplayNames, m.addTaxForm.TaxTypeIndex),
		m.renderTaxRowText(tax.TaxFieldAmountDue, "Amount due", m.addTaxForm.Inputs[0].View()),
		m.renderTaxRowText(tax.TaxFieldAmountPaid, "Amount paid", m.addTaxForm.Inputs[1].View()),
		m.renderTaxRowText(tax.TaxFieldPeriod, "Period", m.addTaxForm.Inputs[2].View()),
		m.renderTaxRowText(tax.TaxFieldDueDate, "Due date", m.addTaxForm.Inputs[3].View()),
		m.renderTaxRowText(tax.TaxFieldComment, "Comment", m.addTaxForm.Inputs[4].View()),
	)

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderTaxRowText(field int, label string, value string) string {
	prefix := "  "
	if m.addTaxForm.Active == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderTaxChoiceRow(field int, label string, options []string, selected int) string {
	prefix := "  "
	if m.addTaxForm.Active == field {
		prefix = "> "
	}

	chips := make([]string, 0, len(options))

	for i, option := range options {
		style := buttonStyle
		if i == selected {
			style = buttonActiveStyle
		}

		chips = append(chips, style.Render(option))
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + strings.Join(chips, " ")
}

func (m TheApplication) renderTaxList(width int) string {
	title := "Unpaid taxes"
	if m.taxMode == taxListHistory {
		title = "Tax history (paid)"
	}

	lines := []string{lipgloss.JoinHorizontal(lipgloss.Center, sectionTitleStyle.Render(title), "  ", modeBadgeStyle.Render("Taxes")), hintStyle.Render("Use up/down to browse. Enter edits tax. Delete/Backspace asks confirmation. Esc returns to menu."), ""}

	filtered := m.filteredTaxes()
	if len(filtered) == 0 {
		lines = append(lines, mutedStyle.Render("No taxes found."))

		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	paid, total := taxProgressTotals(filtered)
	lines = append(lines, fieldLabelStyle.Render("Overall paid progress ("+m.baseCurrencyLabel()+")"))
	lines = append(lines, "  "+m.renderProgressBar(paid, total, 28))
	lines = append(lines, "")

	lines = append(lines, m.renderTaxTableHeader(width))

	for i, item := range filtered {
		lines = append(lines, m.renderTaxTableRow(width, i, item))
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderTaxTableHeader(width int) string {
	countryWidth := 10
	typeWidth := 12
	dueWidth := 10
	paidWidth := 12
	leftWidth := 12
	periodWidth := 10
	dueDateWidth := 10
	progressWidth := 8

	commentWidth := width - 14 - countryWidth - typeWidth - dueWidth - paidWidth - leftWidth - periodWidth - dueDateWidth - progressWidth - 18
	if commentWidth < 18 {
		commentWidth = 18
	}

	header := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", "#", countryWidth, "Country", typeWidth, "Tax type", dueWidth, "Due", paidWidth, "Paid", leftWidth, "Left", periodWidth, "Period", dueDateWidth, "Due date", progressWidth, "Done", commentWidth, "Comment")

	return tableHeaderStyle.Render(header)
}

func (m TheApplication) renderTaxTableRow(width int, index int, item tax.Tax) string {
	countryWidth := 10
	typeWidth := 12
	dueWidth := 10
	paidWidth := 12
	leftWidth := 12
	periodWidth := 10
	dueDateWidth := 10
	progressWidth := 8

	commentWidth := width - 14 - countryWidth - typeWidth - dueWidth - paidWidth - leftWidth - periodWidth - dueDateWidth - progressWidth - 18
	if commentWidth < 18 {
		commentWidth = 18
	}

	prefix := " "
	style := rowStyle

	if index == m.taxCursor {
		prefix = ">"
		style = selectedRowStyle
	}

	left := item.AmountDueCents - item.AmountPaidCents
	if left < 0 {
		left = 0
	}

	progressValue := 0.0

	if item.AmountDueCents > 0 {
		progressValue = (float64(item.AmountPaidCents) / float64(item.AmountDueCents)) * 100
		if progressValue < 0 {
			progressValue = 0
		}
		if progressValue > 100 {
			progressValue = 100
		}
	}

	base := m.baseCurrencyLabel()
	dueDateLabel := "-"

	if item.DueDate != nil {
		dueDateLabel = item.DueDate.Local().Format("2006-01-02")
	}

	row := fmt.Sprintf("%s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", prefix, countryWidth, truncateText(item.TaxCountry, countryWidth), typeWidth, truncateText(item.TaxTypeName, typeWidth), dueWidth, renderMoneyWithCurrency(base, item.AmountDueCents), paidWidth, renderMoneyWithCurrency(base, item.AmountPaidCents), leftWidth, renderMoneyWithCurrency(base, left), periodWidth, truncateText(item.Period, periodWidth), dueDateWidth, dueDateLabel, progressWidth, fmt.Sprintf("%5.1f%%", progressValue), commentWidth, truncateText(item.Comment, commentWidth))

	return style.Render(row)
}

func (m TheApplication) renderTaxEdit(width int) string {
	lines := []string{
		headlineStyle.Render("Edit tax"),
		mutedStyle.Render("Edit fields and press Enter on Comment to save."),
		"",
		mutedStyle.Render("Tax: " + m.editTaxForm.CountryLabel + " / " + m.editTaxForm.TaxTypeLabel),
	}

	if idx := m.findTaxIndex(m.editingTaxID); idx >= 0 {
		item := m.taxes[idx]
		lines = append(lines, fieldLabelStyle.Render("Paid progress"))
		lines = append(lines, "  "+m.renderProgressBar(item.AmountPaidCents, item.AmountDueCents, 28))
	}

	lines = append(lines,
		"",
		m.renderEditTaxField(tax.EditTaxFieldAmountDue, "Amount due", m.editTaxForm.AmountDueInput.View()),
		m.renderEditTaxField(tax.EditTaxFieldAmountPaid, "Amount paid", m.editTaxForm.AmountPaidInput.View()),
		m.renderEditTaxField(tax.EditTaxFieldPeriod, "Period", m.editTaxForm.PeriodInput.View()),
		m.renderEditTaxField(tax.EditTaxFieldDueDate, "Due date", m.editTaxForm.DueDateInput.View()),
		m.renderEditTaxField(tax.EditTaxFieldComment, "Comment", m.editTaxForm.CommentInput.View()),
		"",
		fieldLabelStyle.Render("Add transaction"),
		mutedStyle.Render("Set delta/date/comment, then press Enter on Transaction comment to apply."),
		m.renderEditTaxField(tax.EditTaxFieldLogDelta, "Transaction delta", m.editTaxForm.LogDeltaInput.View()),
		m.renderEditTaxField(tax.EditTaxFieldLogDate, "Transaction date", m.editTaxForm.LogDateInput.View()),
		m.renderEditTaxField(tax.EditTaxFieldLogComment, "Transaction comment", m.editTaxForm.LogCommentInput.View()),
		"",
		fieldLabelStyle.Render("Logs"),
	)

	if len(m.taxLogs) == 0 {
		lines = append(lines, mutedStyle.Render("No log entries yet."))
	} else {
		for _, entry := range m.taxLogs {
			sign := "+"
			if entry.DeltaPaidCents < 0 {
				sign = ""
			}

			note := strings.TrimSpace(entry.Note)
			if note != "" {
				lines = append(lines, fmt.Sprintf("%s%s at %s | %s", sign, formatAmount(entry.DeltaPaidCents), entry.CreatedAt.Local().Format("2006-01-02 15:04"), note))
			} else {
				lines = append(lines, fmt.Sprintf("%s%s at %s", sign, formatAmount(entry.DeltaPaidCents), entry.CreatedAt.Local().Format("2006-01-02 15:04")))
			}
		}
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderInvoiceNew(width int) string {
	typeIndex := 0
	if !m.addInvoiceForm.IsIncoming {
		typeIndex = 1
	}

	paidMarker := "[ ]"
	if m.addInvoiceForm.Paid {
		paidMarker = "[x]"
	}

	lines := []string{
		headlineStyle.Render("New invoice"),
		mutedStyle.Render("Only title and type are required. Pick target account or type custom override."),
		"",
		m.renderInvoiceRowText(invoice.InvoiceFieldTitle, "Title", m.addInvoiceForm.Inputs[0].View()),
		m.renderInvoiceChoiceRow(invoice.InvoiceFieldType, "Type", []string{"incoming (i must pay)", "outgoing (they pay me)"}, typeIndex),
		m.renderInvoiceChoiceRow(invoice.InvoiceFieldCurrency, "Currency", m.addInvoiceForm.CurrencyOptions, m.addInvoiceForm.CurrencyIndex),
		m.renderInvoiceRowText(invoice.InvoiceFieldAmount, "Amount", m.addInvoiceForm.Inputs[1].View()),
		m.renderInvoiceRowText(invoice.InvoiceFieldPaid, "Paid", paidMarker),
		m.renderInvoiceRowText(invoice.InvoiceFieldPeer, "Peer", m.addInvoiceForm.Inputs[2].View()),
		m.renderInvoiceRowText(invoice.InvoiceFieldInvoiceDate, "Invoice date", m.addInvoiceForm.Inputs[3].View()),
		m.renderInvoiceRowText(invoice.InvoiceFieldDueDate, "Due date", m.addInvoiceForm.Inputs[4].View()),
		m.renderInvoiceChoiceRow(invoice.InvoiceFieldTargetAccountChoice, "Target account (pick)", cashflowAccountDisplayOptions(m.addInvoiceForm.AccountOptions), m.addInvoiceForm.AccountIndex),
		m.renderInvoiceRowText(invoice.InvoiceFieldTargetAccountName, "Target account (type)", m.addInvoiceForm.Inputs[5].View()),
		m.renderInvoiceRowText(invoice.InvoiceFieldURL, "URL", m.addInvoiceForm.Inputs[6].View()),
		m.renderInvoiceRowText(invoice.InvoiceFieldDescription, "Description", m.addInvoiceForm.Inputs[7].View()),
		"",
		mutedStyle.Render("Space toggles Paid when active."),
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderInvoiceList(width int) string {
	title := "Outgoing unpaid invoices"
	if m.invoiceMode == invoiceListIncomingUnpaid {
		title = "Incoming unpaid invoices"
	}

	if m.invoiceMode == invoiceListHistoryPaid {
		title = "Invoice history (paid)"
	}

	lines := []string{lipgloss.JoinHorizontal(lipgloss.Center, sectionTitleStyle.Render(title), "  ", modeBadgeStyle.Render("Invoices")), hintStyle.Render("Up/down selects row. Left/right or PgUp/PgDn changes page. Enter edits invoice. Delete/Backspace asks confirmation. Esc returns to menu."), ""}

	filtered := m.filteredInvoices()
	if len(filtered) == 0 {
		lines = append(lines, mutedStyle.Render("No invoices found."))
		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	pageSize := m.invoiceListPageSize()

	totalPages := (len(filtered) + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}

	currentPage := clamp(m.invoicePage, 0, totalPages-1)

	start := currentPage * pageSize
	if start < 0 {
		start = 0
	}

	if start > len(filtered) {
		start = len(filtered)
	}

	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}

	lines = append(lines, fmt.Sprintf("Page %d/%d", currentPage+1, totalPages), "")
	lines = append(lines, m.renderInvoiceTableHeader(width))

	for i := start; i < end; i++ {
		lines = append(lines, m.renderInvoiceTableRow(width, i, filtered[i]))
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderInvoiceEdit(width int) string {
	typeIndex := 0
	if !m.editInvoiceForm.IsIncoming {
		typeIndex = 1
	}

	paidMarker := "[ ]"
	if m.editInvoiceForm.Paid {
		paidMarker = "[x]"
	}

	lines := []string{
		headlineStyle.Render("Edit invoice"),
		mutedStyle.Render("Edit any field and press Enter on Description to save."),
		"",
		m.renderEditInvoiceField(invoice.InvoiceFieldTitle, "Title", m.editInvoiceForm.TitleInput.View()),
		m.renderEditInvoiceChoiceRow(invoice.InvoiceFieldType, "Type", []string{"incoming (i must pay)", "outgoing (they pay me)"}, typeIndex),
		m.renderEditInvoiceChoiceRow(invoice.InvoiceFieldCurrency, "Currency", m.editInvoiceForm.CurrencyOptions, m.editInvoiceForm.CurrencyIndex),
		m.renderEditInvoiceField(invoice.InvoiceFieldAmount, "Amount", m.editInvoiceForm.AmountInput.View()),
		m.renderEditInvoiceField(invoice.InvoiceFieldPaid, "Paid", paidMarker),
		m.renderEditInvoiceField(invoice.InvoiceFieldPeer, "Peer", m.editInvoiceForm.PeerInput.View()),
		m.renderEditInvoiceField(invoice.InvoiceFieldInvoiceDate, "Invoice date", m.editInvoiceForm.InvoiceDateInput.View()),
		m.renderEditInvoiceField(invoice.InvoiceFieldDueDate, "Due date", m.editInvoiceForm.DueDateInput.View()),
		m.renderEditInvoiceChoiceRow(invoice.InvoiceFieldTargetAccountChoice, "Target account (pick)", cashflowAccountDisplayOptions(m.editInvoiceForm.AccountOptions), m.editInvoiceForm.AccountIndex),
		m.renderEditInvoiceField(invoice.InvoiceFieldTargetAccountName, "Target account (type)", m.editInvoiceForm.TargetAccountInput.View()),
		m.renderEditInvoiceField(invoice.InvoiceFieldURL, "URL", m.editInvoiceForm.URLInput.View()),
		m.renderEditInvoiceField(invoice.InvoiceFieldDescription, "Description", m.editInvoiceForm.DescriptionInput.View()),
		"",
		mutedStyle.Render("Space toggles Paid when active."),
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderDebtList(width int) string {
	title := "Outgoing debts"

	if m.debtMode == debtListIncoming {
		title = "Incoming debts"
	}

	if m.debtMode == debtListHistory {
		title = "Debt history (paid)"
	}

	lines := []string{lipgloss.JoinHorizontal(lipgloss.Center, sectionTitleStyle.Render(title), "  ", modeBadgeStyle.Render("Debts")), hintStyle.Render("Use up/down to browse. Enter edits debt. Delete/Backspace asks confirmation. Esc returns to menu."), ""}
	filtered := m.filteredDebts()

	if len(filtered) == 0 {
		lines = append(lines, mutedStyle.Render("No debts found."))

		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	paidBase, totalBase := m.debtProgressTotalsBase(filtered)
	lines = append(lines, fieldLabelStyle.Render("Overall paid progress ("+m.baseCurrencyLabel()+")"))
	lines = append(lines, "  "+m.renderProgressBar(paidBase, totalBase, 28))
	lines = append(lines, "")

	lines = append(lines, m.renderDebtTableHeader(width))

	for i, item := range filtered {
		lines = append(lines, m.renderDebtTableRow(width, i, item))
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderDebtTableHeader(width int) string {
	peerWidth := 12
	currencyWidth := 8
	amountWidth := 12
	paidWidth := 12
	leftWidth := 12
	dueWidth := 10
	dirWidth := 6

	commentWidth := width - 14 - peerWidth - currencyWidth - amountWidth - paidWidth - leftWidth - dueWidth - dirWidth - 18
	if commentWidth < 18 {
		commentWidth = 18
	}

	header := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", "#", peerWidth, "Peer", currencyWidth, "Curr", amountWidth, "Amount", paidWidth, "Paid", leftWidth, "Left", dueWidth, "Due", dirWidth, "Dir", commentWidth, "Comment")

	return tableHeaderStyle.Render(header)
}

func (m TheApplication) renderDebtTableRow(width int, index int, item debt.Debt) string {
	peerWidth := 12
	currencyWidth := 8
	amountWidth := 12
	paidWidth := 12
	leftWidth := 12
	dueWidth := 10
	dirWidth := 6

	commentWidth := width - 14 - peerWidth - currencyWidth - amountWidth - paidWidth - leftWidth - dueWidth - dirWidth - 18
	if commentWidth < 18 {
		commentWidth = 18
	}

	prefix := " "
	style := rowStyle

	if index == m.debtCursor {
		prefix = ">"
		style = selectedRowStyle
	}

	left := item.AmountCents - item.AmountPaidCents
	if left < 0 {
		left = 0
	}

	due := "-"
	if item.DueDate != nil {
		due = item.DueDate.Local().Format("2006-01-02")
	}

	dir := "out"
	if item.IsOwedToUser {
		dir = "in"
	}

	row := fmt.Sprintf("%s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", prefix, peerWidth, truncateText(item.Peer, peerWidth), currencyWidth, truncateText(item.Currency, currencyWidth), amountWidth, renderMoneyWithCurrency(item.Currency, item.AmountCents), paidWidth, renderMoneyWithCurrency(item.Currency, item.AmountPaidCents), leftWidth, renderMoneyWithCurrency(item.Currency, left), dueWidth, due, dirWidth, dir, commentWidth, truncateText(item.Comment, commentWidth))

	return style.Render(row)
}

func (m TheApplication) renderDebtEdit(width int) string {
	lines := []string{
		headlineStyle.Render("Edit debt"),
		mutedStyle.Render("Edit fields and press Enter on Comment to save."),
		"",
		mutedStyle.Render("Peer: " + m.editDebtForm.PeerLabel + " | Direction: " + m.editDebtForm.DirectionLabel + " | Currency: " + m.editDebtForm.CurrencyLabel),
	}

	if idx := m.findDebtIndex(m.editingDebtID); idx >= 0 {
		item := m.debts[idx]
		lines = append(lines, fieldLabelStyle.Render("Paid progress"))
		lines = append(lines, "  "+m.renderProgressBar(item.AmountPaidCents, item.AmountCents, 28))
	}

	lines = append(lines,
		"",
		m.renderEditDebtField(debt.EditDebtFieldAmount, "Amount", m.editDebtForm.AmountInput.View()),
		m.renderEditDebtField(debt.EditDebtFieldAmountPaid, "Amount paid", m.editDebtForm.AmountPaidInput.View()),
		m.renderEditDebtField(debt.EditDebtFieldDebtCreated, "Debt created", m.editDebtForm.DebtCreatedInput.View()),
		m.renderEditDebtField(debt.EditDebtFieldDueDate, "Due date", m.editDebtForm.DueDateInput.View()),
		m.renderEditDebtField(debt.EditDebtFieldComment, "Comment", m.editDebtForm.CommentInput.View()),
		"",
		fieldLabelStyle.Render("Add transaction"),
		mutedStyle.Render("Set delta/date/comment, then press Enter on Transaction comment to apply."),
		m.renderEditDebtField(debt.EditDebtFieldLogDelta, "Transaction delta", m.editDebtForm.LogDeltaInput.View()),
		m.renderEditDebtField(debt.EditDebtFieldLogDate, "Transaction date", m.editDebtForm.LogDateInput.View()),
		m.renderEditDebtField(debt.EditDebtFieldLogComment, "Transaction comment", m.editDebtForm.LogCommentInput.View()),
		"",
		fieldLabelStyle.Render("Logs"),
	)

	if len(m.debtLogs) == 0 {
		lines = append(lines, mutedStyle.Render("No log entries yet."))
	} else {
		for _, entry := range m.debtLogs {
			sign := "+"
			if entry.DeltaPaidCents < 0 {
				sign = ""
			}

			note := strings.TrimSpace(entry.Note)
			if note != "" {
				lines = append(lines, fmt.Sprintf("%s%s at %s | %s", sign, formatAmount(entry.DeltaPaidCents), entry.CreatedAt.Local().Format("2006-01-02 15:04"), note))
			} else {
				lines = append(lines, fmt.Sprintf("%s%s at %s", sign, formatAmount(entry.DeltaPaidCents), entry.CreatedAt.Local().Format("2006-01-02 15:04")))
			}
		}
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderEditDebtField(field int, label string, value string) string {
	prefix := "  "
	if m.editDebtForm.ActiveField == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderGoalNew(width int) string {
	lines := []string{
		headlineStyle.Render("New goal"),
		mutedStyle.Render("Use up/down to move fields. Left/right changes currency. Enter on last field saves."),
		"",
		m.renderGoalRowText(goal.GoalFieldName, "Goal", m.addGoalForm.Inputs[0].View()),
		m.renderGoalChoiceRow(goal.GoalFieldCurrency, "Currency", m.addGoalForm.CurrencyOptions, m.addGoalForm.CurrencyIndex),
		m.renderGoalRowText(goal.GoalFieldTargetAmount, "Target amount", m.addGoalForm.Inputs[1].View()),
		m.renderGoalRowText(goal.GoalFieldAccumulated, "Accumulated", m.addGoalForm.Inputs[2].View()),
		m.renderGoalRowText(goal.GoalFieldDescription, "Description", m.addGoalForm.Inputs[3].View()),
		m.renderGoalRowText(goal.GoalFieldDateStarted, "Date started", m.addGoalForm.Inputs[4].View()),
		m.renderGoalRowText(goal.GoalFieldTargetDate, "Target date", m.addGoalForm.Inputs[5].View()),
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderGoalRowText(field int, label string, value string) string {
	prefix := "  "
	if m.addGoalForm.Active == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderGoalChoiceRow(field int, label string, options []string, selected int) string {
	prefix := "  "
	if m.addGoalForm.Active == field {
		prefix = "> "
	}

	chips := make([]string, 0, len(options))

	for i, option := range options {
		style := buttonStyle
		if i == selected {
			style = buttonActiveStyle
		}
		chips = append(chips, style.Render(option))
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + strings.Join(chips, " ")
}

func (m TheApplication) renderGoalList(width int) string {
	title := "Active goals"

	if m.goalMode == goalListHistory {
		title = "Goal history (completed)"
	}

	lines := []string{lipgloss.JoinHorizontal(lipgloss.Center, sectionTitleStyle.Render(title), "  ", modeBadgeStyle.Render("Goals")), hintStyle.Render("Use up/down to browse. Enter edits goal. Delete/Backspace asks confirmation. Esc returns to menu."), ""}

	filtered := m.filteredGoals()
	if len(filtered) == 0 {
		lines = append(lines, mutedStyle.Render("No goals found."))
		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	accumulatedBase, targetBase := m.goalProgressTotalsBase(filtered)
	lines = append(lines, fieldLabelStyle.Render("Overall progress ("+m.baseCurrencyLabel()+")"))
	lines = append(lines, "  "+m.renderProgressBar(accumulatedBase, targetBase, 28))
	lines = append(lines, "")

	lines = append(lines, m.renderGoalTableHeader(width))
	for i, item := range filtered {
		lines = append(lines, m.renderGoalTableRow(width, i, item))
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderGoalTableHeader(width int) string {
	nameWidth := 12
	currencyWidth := 8
	targetWidth := 12
	accumWidth := 12
	leftWidth := 12
	startedWidth := 10
	targetDateWidth := 10
	progressWidth := 8

	descWidth := width - 14 - nameWidth - currencyWidth - targetWidth - accumWidth - leftWidth - startedWidth - targetDateWidth - progressWidth - 18
	if descWidth < 18 {
		descWidth = 18
	}

	header := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", "#", nameWidth, "Goal", currencyWidth, "Curr", targetWidth, "Target", accumWidth, "Saved", leftWidth, "Left", startedWidth, "Started", targetDateWidth, "Target dt", progressWidth, "Done", descWidth, "Description")

	return tableHeaderStyle.Render(header)
}

func (m TheApplication) renderSubscriptionNew(width int) string {
	activeMarker := "[ ]"

	if m.addSubscriptionForm.IsActive {
		activeMarker = "[x]"
	}

	lines := []string{
		headlineStyle.Render("New subscription"),
		mutedStyle.Render("Use up/down to move fields. Left/right changes Currency/Method/Period/Type. Space toggles Is Active. Enter on last field saves."),
		"",
		m.renderSubscriptionTextFieldRow(subscription.SubFieldName, "Name", m.addSubscriptionForm.Inputs[0].View()),
		m.renderSubscriptionOptionRow(subscription.SubFieldCurrency, "Currency", m.addSubscriptionForm.CurrencyOptions, m.addSubscriptionForm.CurrencyIndex),
		m.renderSubscriptionTextFieldRow(subscription.SubFieldAmount, "Amount", m.addSubscriptionForm.Inputs[2].View()),
		m.renderSubscriptionOptionRow(subscription.SubFieldPeriod, "Period", m.addSubscriptionForm.PeriodOptions, m.addSubscriptionForm.PeriodIndex),
		m.renderSubscriptionOptionRow(subscription.SubFieldPaymentMethodChoice, "Payment method", m.addSubscriptionForm.PaymentMethodOptions, m.addSubscriptionForm.PaymentMethodIndex),
		m.renderSubscriptionTextFieldRow(subscription.SubFieldPaymentMethod, "Payment method (custom)", m.addSubscriptionForm.Inputs[3].View()),
		m.renderSubscriptionOptionRow(subscription.SubFieldType, "Type", m.addSubscriptionForm.TypeOptions, m.addSubscriptionForm.TypeIndex),
		m.renderSubscriptionTextFieldRow(subscription.SubFieldIsActive, "Is Active", activeMarker),
		m.renderSubscriptionTextFieldRow(subscription.SubFieldDayYearly, "Payment date (yearly)", m.addSubscriptionForm.Inputs[4].View()),
		m.renderSubscriptionTextFieldRow(subscription.SubFieldDayMonthly, "Payment day (monthly)", m.addSubscriptionForm.Inputs[5].View()),
		"",
		mutedStyle.Render("Esc goes back to menu. Day fields are optional."),
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderSubscriptionEdit(width int) string {
	activeMarker := "[ ]"

	if m.editSubscriptionForm.IsActive {
		activeMarker = "[x]"
	}

	lines := []string{
		headlineStyle.Render("Edit subscription"),
		mutedStyle.Render("Use up/down to move fields. Space toggles Is Active. Enter on Active saves. Esc cancels."),
		"",
		mutedStyle.Render("Name: " + m.editSubscriptionForm.NameLabel + " | Type: " + m.editSubscriptionForm.TypeLabel + " | Period: " + m.editSubscriptionForm.PeriodLabel),
		mutedStyle.Render("Currency: " + m.editSubscriptionForm.CurrencyLabel + " | Yearly date: " + m.editSubscriptionForm.PaymentDateYearly + " | Monthly day: " + m.editSubscriptionForm.PaymentDayMonthly),
		"",
		m.renderEditSubscriptionField(subscription.EditSubFieldAmount, "Amount", m.editSubscriptionForm.AmountInput.View()),
		m.renderEditSubscriptionField(subscription.EditSubFieldPaymentMethod, "Payment method", m.editSubscriptionForm.PaymentMethodInput.View()),
		m.renderEditSubscriptionField(subscription.EditSubFieldIsActive, "Is Active", activeMarker),
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderEditSubscriptionField(field int, label string, value string) string {
	prefix := "  "

	if m.editSubscriptionForm.ActiveField == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderSubscriptionTextFieldRow(field int, label string, value string) string {
	prefix := "  "

	if m.addSubscriptionForm.Active == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderSubscriptionOptionRow(field int, label string, options []string, selected int) string {
	prefix := "  "

	if m.addSubscriptionForm.Active == field {
		prefix = "> "
	}

	chips := make([]string, 0, len(options))

	for idx, option := range options {
		style := buttonStyle
		if idx == selected {
			style = buttonActiveStyle
		}

		chips = append(chips, style.Render(option))
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + strings.Join(chips, " ")
}

func (m TheApplication) renderSubscriptionList(width int) string {
	modeTitle := "All subscriptions"

	if m.subscriptionMode == subscriptionListActive {
		modeTitle = "Active subscriptions"
	}

	lines := []string{lipgloss.JoinHorizontal(lipgloss.Center, sectionTitleStyle.Render(modeTitle), "  ", modeBadgeStyle.Render("Subscriptions")), hintStyle.Render("Use up/down to browse. Enter edits the selected subscription. Delete/Backspace asks confirmation. Esc returns to menu."), ""}

	filtered := m.filteredSubscriptions()
	if len(filtered) == 0 {
		lines = append(lines, mutedStyle.Render("No subscriptions found."))

		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	if m.subscriptionMode == subscriptionListAll {
		activeSubs, inactiveSubs := splitSubscriptionsByActivity(filtered)
		activeMonthlyBase, activeYearlyProjectionBase := m.subscriptionTotalsInBaseCents(activeSubs)
		inactiveMonthlyBase, inactiveYearlyProjectionBase := m.subscriptionTotalsInBaseCents(inactiveSubs)
		baseLabel := m.baseCurrencyLabel()
		lines = append(lines,
			fieldLabelStyle.Render("Active totals in "+baseLabel+":"),
			"  monthly: "+renderMoneyWithCurrency(baseLabel, activeMonthlyBase),
			"  yearly:  "+renderMoneyWithCurrency(baseLabel, activeYearlyProjectionBase),
			"",
			fieldLabelStyle.Render("Inactive totals in "+baseLabel+":"),
			"  monthly: "+renderMoneyWithCurrency(baseLabel, inactiveMonthlyBase),
			"  yearly:  "+renderMoneyWithCurrency(baseLabel, inactiveYearlyProjectionBase),
			"",
		)
	} else {
		monthlyBase, yearlyProjectionBase := m.subscriptionTotalsInBaseCents(filtered)
		baseLabel := m.baseCurrencyLabel()
		lines = append(lines,
			fieldLabelStyle.Render("Active totals in "+baseLabel+":"),
			"  monthly: "+renderMoneyWithCurrency(baseLabel, monthlyBase),
			"  yearly:  "+renderMoneyWithCurrency(baseLabel, yearlyProjectionBase),
			"",
		)
	}

	lines = append(lines, m.renderSubscriptionTableHeader(width))

	for idx, sub := range filtered {
		lines = append(lines, m.renderSubscriptionRow(width, idx, sub))
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderSubscriptionTableHeader(width int) string {
	nameWidth := 14
	currencyWidth := 8
	amountWidth := 12
	baseAmountWidth := 12
	periodWidth := 7
	typeWidth := 12
	activeWidth := 7

	descWidth := width - 12 - nameWidth - currencyWidth - amountWidth - baseAmountWidth - periodWidth - typeWidth - activeWidth - 14
	if descWidth < 18 {
		descWidth = 18
	}

	baseCurrencyTitle := strings.TrimSpace(m.settings.BaseCurrency)
	if baseCurrencyTitle == "" {
		baseCurrencyTitle = "$"
	}

	header := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", "#", nameWidth, "Name", currencyWidth, "Curr", amountWidth, "Amount", baseAmountWidth, truncateText(baseCurrencyTitle, baseAmountWidth), periodWidth, "Period", typeWidth, "Type", activeWidth, "Active", descWidth, "Payment method")

	return tableHeaderStyle.Render(header)
}

func (m TheApplication) renderSubscriptionRow(width int, index int, sub subscription.Subscription) string {
	nameWidth := 14
	currencyWidth := 8
	amountWidth := 12
	baseAmountWidth := 12
	periodWidth := 7
	typeWidth := 12
	activeWidth := 7

	descWidth := width - 12 - nameWidth - currencyWidth - amountWidth - baseAmountWidth - periodWidth - typeWidth - activeWidth - 14
	if descWidth < 18 {
		descWidth = 18
	}

	prefix := " "

	style := rowStyle
	if index == m.subscriptionCursor {
		prefix = ">"
		style = selectedRowStyle
	}

	active := "no"

	if sub.IsActive {
		active = "yes"
	}

	row := fmt.Sprintf("%s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", prefix, nameWidth, truncateText(sub.Name, nameWidth), currencyWidth, truncateText(sub.Currency, currencyWidth), amountWidth, renderMoneyWithCurrency(sub.Currency, sub.AmountCents), baseAmountWidth, m.convertedAmountForBase(sub.Currency, sub.AmountCents), periodWidth, truncateText(sub.Period, periodWidth), typeWidth, truncateText(sub.Type, typeWidth), activeWidth, active, descWidth, truncateText(sub.PaymentMethod, descWidth))

	return style.Render(row)
}

func (m TheApplication) renderDebtNew(width int) string {
	directionIndex := 0

	if m.addDebtForm.IsOwedToUser {
		directionIndex = 1
	}

	lines := []string{
		headlineStyle.Render("New debt"),
		mutedStyle.Render("Use up/down to move fields. Left/right changes direction and currency. Space also toggles direction. Enter on last field saves."),
		"",
		m.renderDebtChoiceRow(debt.DebtFieldDirection, "Direction", []string{"outgoing (i owe)", "incoming (owed to me)"}, directionIndex),
		m.renderDebtRowText(debt.DebtFieldPeer, "Peer", m.addDebtForm.Inputs[0].View()),
		m.renderDebtChoiceRow(debt.DebtFieldCurrency, "Currency", m.addDebtForm.CurrencyOptions, m.addDebtForm.CurrencyIndex),
		m.renderDebtRowText(debt.DebtFieldAmount, "Amount", m.addDebtForm.Inputs[1].View()),
		m.renderDebtRowText(debt.DebtFieldAmountPaid, "Amount paid", m.addDebtForm.Inputs[2].View()),
		m.renderDebtRowText(debt.DebtFieldDebtCreated, "Debt created", m.addDebtForm.Inputs[3].View()),
		m.renderDebtRowText(debt.DebtFieldDueDate, "Due date", m.addDebtForm.Inputs[4].View()),
		m.renderDebtRowText(debt.DebtFieldComment, "Comment", m.addDebtForm.Inputs[5].View()),
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderDebtRowText(field int, label string, value string) string {
	prefix := "  "

	if m.addDebtForm.Active == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderDebtChoiceRow(field int, label string, options []string, selected int) string {
	prefix := "  "

	if m.addDebtForm.Active == field {
		prefix = "> "
	}

	chips := make([]string, 0, len(options))

	for i, option := range options {
		style := buttonStyle

		if i == selected {
			style = buttonActiveStyle
		}

		chips = append(chips, style.Render(option))
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + strings.Join(chips, " ")
}

func (m TheApplication) renderDashboard(width int) string {
	base := m.baseCurrencyLabel()

	// Monthly net: income - expenses for current month
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

	// Build left column
	leftLines := []string{sectionTitleStyle.Render("Financial Summary")}
	leftLines = append(leftLines,
		fmt.Sprintf("Monthly net:        %s", renderMoneyWithCurrency(base, monthlyNet)),
		fmt.Sprintf("Monthly subscr:     %s", renderMoneyWithCurrency(base, monthlySubCost)),
		fmt.Sprintf("Yearly subscr:      %s", renderMoneyWithCurrency(base, yearlySubProjection)),
		fmt.Sprintf("Total accounts:     %s", renderMoneyWithCurrency(base, totalAccounts)),
	)
	leftColumn := strings.Join(leftLines, "\n")

	// Build right column
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

	// Combine columns
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

func (m TheApplication) debtProgressTotalsBase(items []debt.Debt) (paid int64, total int64) {
	for _, item := range items {
		itemTotal := item.AmountCents
		itemPaid := item.AmountPaidCents

		if itemTotal < 0 {
			itemTotal = 0
		}

		if itemPaid < 0 {
			itemPaid = 0
		}

		if itemPaid > itemTotal {
			itemPaid = itemTotal
		}

		totalBase, totalOK := m.convertToBaseCents(item.Currency, itemTotal)
		paidBase, paidOK := m.convertToBaseCents(item.Currency, itemPaid)

		if totalOK && paidOK {
			total += totalBase
			paid += paidBase

			continue
		}

		total += itemTotal
		paid += itemPaid
	}

	if paid > total {
		paid = total
	}

	if paid < 0 {
		paid = 0
	}

	if total < 0 {
		total = 0
	}

	return paid, total
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

func (m TheApplication) View() string {
	if m.quitting {
		return ""
	}

	width := m.width
	if width == 0 {
		width = 100
	}

	contentWidth := width - 6
	if contentWidth < 76 {
		contentWidth = 76
	}
	header := renderHeader(contentWidth)
	body := m.renderBody(contentWidth)
	footer := renderFooter(contentWidth, m.status)

	return lipgloss.JoinVertical(lipgloss.Left, header, "", body, "", footer)
}

func (m TheApplication) updateCashflowNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

		return m, nil
	case "up", "shift+tab":
		m.addCashflowForm = m.addCashflowForm.Prev()

		return m, nil
	case "down", "tab":
		m.addCashflowForm = m.addCashflowForm.Next()

		return m, nil
	case "left":
		if m.addCashflowForm.Active == cashflow.CashflowFieldCurrency && m.addCashflowForm.CurrencyIndex > 0 {
			m.addCashflowForm.CurrencyIndex--
		}

		if m.addCashflowForm.Active == cashflow.CashflowFieldCategory && m.addCashflowForm.CategoryIndex > 0 {
			m.addCashflowForm.CategoryIndex--
		}

		if m.addCashflowForm.Active == cashflow.CashflowFieldAccount && m.addCashflowForm.AccountIndex > 0 {
			m.addCashflowForm.AccountIndex--
		}

		return m, nil
	case "right":
		if m.addCashflowForm.Active == cashflow.CashflowFieldCurrency && m.addCashflowForm.CurrencyIndex < len(m.addCashflowForm.CurrencyOptions)-1 {
			m.addCashflowForm.CurrencyIndex++
		}

		if m.addCashflowForm.Active == cashflow.CashflowFieldCategory && m.addCashflowForm.CategoryIndex < len(m.addCashflowForm.CategoryOptions)-1 {
			m.addCashflowForm.CategoryIndex++
		}

		if m.addCashflowForm.Active == cashflow.CashflowFieldAccount && m.addCashflowForm.AccountIndex < len(m.addCashflowForm.AccountOptions)-1 {
			m.addCashflowForm.AccountIndex++
		}

		return m, nil
	case "enter":
		if m.addCashflowForm.Active == cashflow.CashflowFieldCount-1 {
			return m.saveCashflowFromForm()
		}

		m.addCashflowForm = m.addCashflowForm.Next()

		return m, nil
	}

	if inputIndex := m.addCashflowForm.InputIndexForField(m.addCashflowForm.Active); inputIndex >= 0 {
		var cmd tea.Cmd

		m.addCashflowForm.Inputs[inputIndex], cmd = m.addCashflowForm.Inputs[inputIndex].Update(msg)

		return m, cmd
	}

	return m, nil
}

func (m TheApplication) saveCashflowFromForm() (tea.Model, tea.Cmd) {
	if len(m.addCashflowForm.CategoryOptions) == 0 {
		if m.addCashflowForm.IsIncome {
			m.status = "no income categories configured; add one in settings"
		} else {
			m.status = "no expense categories configured; add one in settings"
		}

		return m, nil
	}

	amountRaw := strings.TrimSpace(m.addCashflowForm.Inputs[0].Value())
	dateRaw := strings.TrimSpace(m.addCashflowForm.Inputs[1].Value())
	comment := strings.TrimSpace(m.addCashflowForm.Inputs[2].Value())

	amount, err := parseAmountCents(amountRaw)
	if err != nil {
		m.status = "amount error: " + err.Error()

		return m, nil
	}

	entryDate, err := parseRequiredDate(dateRaw)
	if err != nil {
		m.status = "date must use DD.MM.YYYY format"

		return m, nil
	}

	category := selectedStringOption(m.addCashflowForm.CategoryOptions, m.addCashflowForm.CategoryIndex)
	if category == "" {
		m.status = "category is required"

		return m, nil
	}

	now := time.Now()
	entry := cashflow.CashflowEntry{
		IsIncome:      m.addCashflowForm.IsIncome,
		Currency:      selectedCurrencyOption(m.addCashflowForm.CurrencyOptions, m.addCashflowForm.CurrencyIndex),
		AmountCents:   amount,
		EntryDate:     entryDate,
		Category:      category,
		AccountName:   selectedStringOption(m.addCashflowForm.AccountOptions, m.addCashflowForm.AccountIndex),
		Comment:       comment,
		LastUpdatedAt: now,
	}

	if err := m.db.Create(&entry).Error; err != nil {
		m.status = "save failed: " + err.Error()

		return m, nil
	}

	m.cashflows = append([]cashflow.CashflowEntry{entry}, m.cashflows...)
	if entry.IsIncome {
		m.addCashflowForm = cashflow.NewAddCashflowForm(currencySelectionOptions(m.settings), incomeCategorySelectionOptions(m.settings), accountSelectionOptions(m.accounts), true)
		m.status = "saved income"
	} else {
		m.addCashflowForm = cashflow.NewAddCashflowForm(currencySelectionOptions(m.settings), expenseCategorySelectionOptions(m.settings), accountSelectionOptions(m.accounts), false)
		m.status = "saved expense"
	}

	m.screen = screenCashflowHistory
	m.cashflowHistoryMonth = beginningOfMonth(entry.EntryDate)
	m.cashflowCursor = 0

	return m, nil
}

func (m TheApplication) updateCashflowHistory(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if updatedModel, cmd, handled := m.handleDeleteConfirmation(msg); handled {
		return updatedModel, cmd
	}

	items := m.filteredCashflowsForMonth()

	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

		return m, nil
	case "left":
		m.cashflowHistoryMonth = monthShift(m.cashflowHistoryMonth, -1)
		m.cashflowCursor = 0

		return m, nil
	case "right":
		m.cashflowHistoryMonth = monthShift(m.cashflowHistoryMonth, 1)
		m.cashflowCursor = 0

		return m, nil
	case "up":
		if m.cashflowCursor > 0 {
			m.cashflowCursor--
		}

		return m, nil
	case "down":
		if m.cashflowCursor < len(items)-1 {
			m.cashflowCursor++
		}

		return m, nil
	case "backspace", "delete":
		if len(items) == 0 || m.cashflowCursor < 0 || m.cashflowCursor >= len(items) {
			m.status = "no entry selected"

			return m, nil
		}

		selected := items[m.cashflowCursor]
		targetName := selected.EntryDate.Local().Format("2006-01-02") + " " + selected.Category
		m = m.beginDeleteConfirmation("cashflow", selected.ID, targetName)

		return m, nil
	default:
		return m, nil
	}
}

func (m TheApplication) updateCashflowOverview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows, _ := m.monthlyCashflowOverviewRows()
	pageSize := m.cashflowOverviewPageSize()
	totalPages := 1

	if pageSize > 0 && len(rows) > 0 {
		totalPages = (len(rows) + pageSize - 1) / pageSize
	}

	m.cashflowOverviewPage = clamp(m.cashflowOverviewPage, 0, totalPages-1)

	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

		return m, nil
	case "left", "h", "up", "k", "pgup":
		if m.cashflowOverviewPage > 0 {
			m.cashflowOverviewPage--
		}

		return m, nil
	case "right", "l", "down", "j", "pgdown":
		if m.cashflowOverviewPage < totalPages-1 {
			m.cashflowOverviewPage++
		}

		return m, nil
	case "home":
		m.cashflowOverviewPage = 0

		return m, nil
	case "end":
		m.cashflowOverviewPage = totalPages - 1

		return m, nil
	default:
		return m, nil
	}
}

func (m TheApplication) cashflowOverviewPageSize() int {
	if m.height <= 0 {
		return 18
	}

	size := m.height - 16
	if size < 6 {
		size = 6
	}

	return size
}

func (m TheApplication) filteredCashflowsForMonth() []cashflow.CashflowEntry {
	month := beginningOfMonth(m.cashflowHistoryMonth)
	filtered := make([]cashflow.CashflowEntry, 0)

	for _, entry := range m.cashflows {
		itemMonth := beginningOfMonth(entry.EntryDate)
		if itemMonth.Year() == month.Year() && itemMonth.Month() == month.Month() {
			filtered = append(filtered, entry)
		}
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].EntryDate.Equal(filtered[j].EntryDate) {
			return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
		}

		return filtered[i].EntryDate.After(filtered[j].EntryDate)
	})

	return filtered
}

func (m TheApplication) monthlyCashflowOverviewRows() ([]cashflow.CashflowMonthlyOverviewRow, int) {
	if len(m.cashflows) == 0 {
		return nil, 0
	}

	totalsByMonth := make(map[time.Time]cashflow.CashflowMonthlyOverviewRow)
	missingRates := 0

	for _, entry := range m.cashflows {
		amountBase, ok := m.convertToBaseCents(entry.Currency, entry.AmountCents)
		if !ok {
			missingRates++
			continue
		}

		month := beginningOfMonth(entry.EntryDate)
		row := totalsByMonth[month]
		row.Month = month

		if entry.IsIncome {
			row.IncomeBase += amountBase
		} else {
			row.ExpenseBase += amountBase
		}

		totalsByMonth[month] = row
	}

	if len(totalsByMonth) == 0 {
		return nil, missingRates
	}

	minMonthSet := false

	var minMonth time.Time
	var maxMonth time.Time

	for month := range totalsByMonth {
		if !minMonthSet {
			minMonth = month
			maxMonth = month
			minMonthSet = true

			continue
		}

		if month.Before(minMonth) {
			minMonth = month
		}

		if month.After(maxMonth) {
			maxMonth = month
		}
	}

	rows := make([]cashflow.CashflowMonthlyOverviewRow, 0)

	var prevNet int64

	hasPrev := false

	for month := minMonth; !month.After(maxMonth); month = monthShift(month, 1) {
		row := totalsByMonth[month]
		row.Month = month
		row.NetBase = row.IncomeBase - row.ExpenseBase
		if hasPrev {
			row.HasPrev = true
			row.DeltaFromPrev = row.NetBase - prevNet
		}
		rows = append(rows, row)
		prevNet = row.NetBase
		hasPrev = true
	}

	return rows, missingRates
}

func (m TheApplication) saveInvoiceFromForm() (tea.Model, tea.Cmd) {
	title := strings.TrimSpace(m.addInvoiceForm.Inputs[0].Value())
	currency := selectedCurrencyOption(m.addInvoiceForm.CurrencyOptions, m.addInvoiceForm.CurrencyIndex)
	amountRaw := strings.TrimSpace(m.addInvoiceForm.Inputs[1].Value())
	peer := strings.TrimSpace(m.addInvoiceForm.Inputs[2].Value())
	invoiceDateRaw := strings.TrimSpace(m.addInvoiceForm.Inputs[3].Value())
	dueDateRaw := strings.TrimSpace(m.addInvoiceForm.Inputs[4].Value())
	targetAccount := strings.TrimSpace(m.addInvoiceForm.Inputs[5].Value())
	url := strings.TrimSpace(m.addInvoiceForm.Inputs[6].Value())
	description := strings.TrimSpace(m.addInvoiceForm.Inputs[7].Value())
	if targetAccount == "" {
		targetAccount = selectedStringOption(m.addInvoiceForm.AccountOptions, m.addInvoiceForm.AccountIndex)
	}

	if title == "" {
		m.status = "invoice title is required"

		return m, nil
	}

	amount, err := parseOptionalAmountCents(amountRaw)
	if err != nil {
		m.status = "amount error: " + err.Error()

		return m, nil
	}

	invoiceDate, err := parseOptionalDatePointer(invoiceDateRaw)
	if err != nil {
		m.status = "invoice date must use DD.MM.YYYY format"

		return m, nil
	}

	dueDate, err := parseOptionalDatePointer(dueDateRaw)
	if err != nil {
		m.status = "due date must use DD.MM.YYYY format"

		return m, nil
	}

	now := time.Now()
	item := invoice.Invoice{
		Title:         title,
		IsIncoming:    m.addInvoiceForm.IsIncoming,
		Currency:      currency,
		AmountCents:   amount,
		Paid:          m.addInvoiceForm.Paid,
		Peer:          peer,
		InvoiceDate:   invoiceDate,
		DueDate:       dueDate,
		TargetAccount: targetAccount,
		URL:           url,
		Description:   description,
		LastUpdatedAt: now,
	}

	if err := m.db.Create(&item).Error; err != nil {
		m.status = "save failed: " + err.Error()

		return m, nil
	}

	m.invoices = append([]invoice.Invoice{item}, m.invoices...)
	m.addInvoiceForm = invoice.NewAddInvoiceForm(currencySelectionOptions(m.settings), accountSelectionOptions(m.accounts))
	m.screen = screenInvoiceList

	if item.Paid {
		m.invoiceMode = invoiceListHistoryPaid
	} else if item.IsIncoming {
		m.invoiceMode = invoiceListIncomingUnpaid
	} else {
		m.invoiceMode = invoiceListOutgoingUnpaid
	}

	m.invoiceCursor = 0
	m.invoicePage = 0
	m.status = "saved invoice " + title

	return m, nil
}

func (m TheApplication) openInvoiceEditor(item invoice.Invoice) tea.Model {
	m.screen = screenInvoiceEdit
	m.editingInvoiceID = item.ID
	m.editInvoiceForm = invoice.NewEditInvoiceForm(currencySelectionOptions(m.settings), accountSelectionOptions(m.accounts))
	m.editInvoiceForm.TitleInput.SetValue(item.Title)

	if item.AmountCents != 0 {
		m.editInvoiceForm.AmountInput.SetValue(formatAmount(item.AmountCents))
	} else {
		m.editInvoiceForm.AmountInput.SetValue("")
	}

	m.editInvoiceForm.PeerInput.SetValue(item.Peer)
	if item.InvoiceDate != nil {
		m.editInvoiceForm.InvoiceDateInput.SetValue(item.InvoiceDate.Local().Format("02.01.2006"))
	}

	if item.DueDate != nil {
		m.editInvoiceForm.DueDateInput.SetValue(item.DueDate.Local().Format("02.01.2006"))
	}

	m.editInvoiceForm.TargetAccountInput.SetValue(item.TargetAccount)

	m.editInvoiceForm.URLInput.SetValue(item.URL)
	m.editInvoiceForm.DescriptionInput.SetValue(item.Description)
	m.editInvoiceForm.CurrencyIndex = 0
	m.editInvoiceForm.AccountIndex = 0

	for i := range m.editInvoiceForm.CurrencyOptions {
		if strings.EqualFold(strings.TrimSpace(m.editInvoiceForm.CurrencyOptions[i]), strings.TrimSpace(item.Currency)) {
			m.editInvoiceForm.CurrencyIndex = i

			break
		}
	}

	for i := range m.editInvoiceForm.AccountOptions {
		if strings.EqualFold(strings.TrimSpace(m.editInvoiceForm.AccountOptions[i]), strings.TrimSpace(item.TargetAccount)) {
			m.editInvoiceForm.AccountIndex = i

			break
		}
	}
	m.editInvoiceForm.IsIncoming = item.IsIncoming
	m.editInvoiceForm.Paid = item.Paid
	m.editInvoiceForm = m.editInvoiceForm.FocusActive()
	m.status = "editing invoice " + item.Title

	return m
}

func (m TheApplication) saveInvoiceEdit() (tea.Model, tea.Cmd) {
	index := m.findInvoiceIndex(m.editingInvoiceID)
	if index < 0 {
		m.status = "invoice not found"

		return m, nil
	}

	title := strings.TrimSpace(m.editInvoiceForm.TitleInput.Value())
	if title == "" {
		m.status = "invoice title is required"

		return m, nil
	}

	amount, err := parseOptionalAmountCents(strings.TrimSpace(m.editInvoiceForm.AmountInput.Value()))
	if err != nil {
		m.status = "amount error: " + err.Error()

		return m, nil
	}

	invoiceDate, err := parseOptionalDatePointer(strings.TrimSpace(m.editInvoiceForm.InvoiceDateInput.Value()))
	if err != nil {
		m.status = "invoice date must use DD.MM.YYYY format"

		return m, nil
	}
	dueDate, err := parseOptionalDatePointer(strings.TrimSpace(m.editInvoiceForm.DueDateInput.Value()))
	if err != nil {
		m.status = "due date must use DD.MM.YYYY format"

		return m, nil
	}

	selected := m.invoices[index]
	selected.Title = title
	selected.IsIncoming = m.editInvoiceForm.IsIncoming
	selected.Currency = selectedCurrencyOption(m.editInvoiceForm.CurrencyOptions, m.editInvoiceForm.CurrencyIndex)
	selected.AmountCents = amount
	selected.Paid = m.editInvoiceForm.Paid
	selected.Peer = strings.TrimSpace(m.editInvoiceForm.PeerInput.Value())
	selected.InvoiceDate = invoiceDate
	selected.DueDate = dueDate
	selected.TargetAccount = strings.TrimSpace(m.editInvoiceForm.TargetAccountInput.Value())
	if selected.TargetAccount == "" {
		selected.TargetAccount = selectedStringOption(m.editInvoiceForm.AccountOptions, m.editInvoiceForm.AccountIndex)
	}
	selected.URL = strings.TrimSpace(m.editInvoiceForm.URLInput.Value())
	selected.Description = strings.TrimSpace(m.editInvoiceForm.DescriptionInput.Value())
	selected.LastUpdatedAt = time.Now()

	if err := m.db.Save(&selected).Error; err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.invoices[index] = selected
	m.screen = screenInvoiceList
	m.status = "updated invoice " + selected.Title

	return m, nil
}

func (m TheApplication) filteredInvoices() []invoice.Invoice {
	filtered := make([]invoice.Invoice, 0, len(m.invoices))

	for _, item := range m.invoices {
		switch m.invoiceMode {
		case invoiceListOutgoingUnpaid:
			if !item.Paid && !item.IsIncoming {
				filtered = append(filtered, item)
			}
		case invoiceListIncomingUnpaid:
			if !item.Paid && item.IsIncoming {
				filtered = append(filtered, item)
			}
		case invoiceListHistoryPaid:
			if item.Paid {
				filtered = append(filtered, item)
			}
		}
	}

	sort.SliceStable(filtered, func(i int, j int) bool {
		left := filtered[i]
		right := filtered[j]

		if left.DueDate == nil && right.DueDate == nil {
			if left.CreatedAt.Equal(right.CreatedAt) {
				return left.ID > right.ID
			}

			return left.CreatedAt.After(right.CreatedAt)
		}

		if left.DueDate == nil {
			return false
		}

		if right.DueDate == nil {
			return true
		}

		if left.DueDate.Equal(*right.DueDate) {
			if left.CreatedAt.Equal(right.CreatedAt) {
				return left.ID > right.ID
			}

			return left.CreatedAt.After(right.CreatedAt)
		}

		return left.DueDate.After(*right.DueDate)
	})

	return filtered
}

func (m TheApplication) invoiceListPageSize() int {
	if m.height <= 0 {
		return 12
	}

	size := m.height - 18
	if size < 5 {
		size = 5
	}

	return size
}

func (m TheApplication) findInvoiceIndex(id uint) int {
	for i := range m.invoices {
		if m.invoices[i].ID == id {
			return i
		}
	}

	return -1
}

func (m TheApplication) updateInvoiceNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

		return m, nil
	case "up", "shift+tab":
		m.addInvoiceForm = m.addInvoiceForm.Prev()

		return m, nil
	case "down", "tab":
		m.addInvoiceForm = m.addInvoiceForm.Next()

		return m, nil
	case "left":
		if m.addInvoiceForm.Active == invoice.InvoiceFieldType {
			m.addInvoiceForm.IsIncoming = true
		}

		if m.addInvoiceForm.Active == invoice.InvoiceFieldCurrency && m.addInvoiceForm.CurrencyIndex > 0 {
			m.addInvoiceForm.CurrencyIndex--
		}

		if m.addInvoiceForm.Active == invoice.InvoiceFieldTargetAccountChoice && m.addInvoiceForm.AccountIndex > 0 {
			m.addInvoiceForm.AccountIndex--
		}

		return m, nil
	case "right":
		if m.addInvoiceForm.Active == invoice.InvoiceFieldType {
			m.addInvoiceForm.IsIncoming = false
		}

		if m.addInvoiceForm.Active == invoice.InvoiceFieldCurrency && m.addInvoiceForm.CurrencyIndex < len(m.addInvoiceForm.CurrencyOptions)-1 {
			m.addInvoiceForm.CurrencyIndex++
		}

		if m.addInvoiceForm.Active == invoice.InvoiceFieldTargetAccountChoice && m.addInvoiceForm.AccountIndex < len(m.addInvoiceForm.AccountOptions)-1 {
			m.addInvoiceForm.AccountIndex++
		}

		return m, nil
	case " ":
		if m.addInvoiceForm.Active == invoice.InvoiceFieldPaid {
			m.addInvoiceForm.Paid = !m.addInvoiceForm.Paid

			return m, nil
		}
	case "enter":
		if m.addInvoiceForm.Active == invoice.InvoiceFieldCount-1 {
			return m.saveInvoiceFromForm()
		}

		m.addInvoiceForm = m.addInvoiceForm.Next()

		return m, nil
	}

	if inputIndex := m.addInvoiceForm.InputIndexForField(m.addInvoiceForm.Active); inputIndex >= 0 {
		var cmd tea.Cmd

		m.addInvoiceForm.Inputs[inputIndex], cmd = m.addInvoiceForm.Inputs[inputIndex].Update(msg)

		return m, cmd
	}

	return m, nil
}

func (m TheApplication) updateInvoiceList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if updatedModel, cmd, handled := m.handleDeleteConfirmation(msg); handled {
		return updatedModel, cmd
	}

	filtered := m.filteredInvoices()
	pageSize := m.invoiceListPageSize()
	totalPages := 1
	if pageSize > 0 && len(filtered) > 0 {
		totalPages = (len(filtered) + pageSize - 1) / pageSize
	}
	m.invoicePage = clamp(m.invoicePage, 0, totalPages-1)

	pageStart := m.invoicePage * pageSize
	if pageStart < 0 {
		pageStart = 0
	}
	if pageStart > len(filtered) {
		pageStart = len(filtered)
	}
	pageEnd := pageStart + pageSize
	if pageEnd > len(filtered) {
		pageEnd = len(filtered)
	}

	if len(filtered) > 0 {
		if m.invoiceCursor < pageStart {
			m.invoiceCursor = pageStart
		}
		if m.invoiceCursor >= pageEnd {
			m.invoiceCursor = pageEnd - 1
		}
	}

	if len(filtered) == 0 {
		if msg.String() == "esc" {
			m.screen = screenMenu
			m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		}

		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

		return m, nil
	case "up":
		if m.invoiceCursor > pageStart {
			m.invoiceCursor--
		}

		return m, nil
	case "down":
		if m.invoiceCursor < pageEnd-1 {
			m.invoiceCursor++
		}

		return m, nil
	case "left", "h", "pgup":
		if m.invoicePage > 0 {
			m.invoicePage--
			pageStart = m.invoicePage * pageSize
			if pageStart >= len(filtered) {
				pageStart = len(filtered) - 1
			}
			if pageStart < 0 {
				pageStart = 0
			}
			m.invoiceCursor = pageStart
		}

		return m, nil
	case "right", "l", "pgdown":
		if m.invoicePage < totalPages-1 {
			m.invoicePage++
			pageStart = m.invoicePage * pageSize
			if pageStart >= len(filtered) {
				pageStart = len(filtered) - 1
			}
			if pageStart < 0 {
				pageStart = 0
			}
			m.invoiceCursor = pageStart
		}

		return m, nil
	case "home":
		m.invoicePage = 0
		m.invoiceCursor = 0

		return m, nil
	case "end":
		m.invoicePage = totalPages - 1
		m.invoiceCursor = m.invoicePage * pageSize
		if m.invoiceCursor >= len(filtered) {
			m.invoiceCursor = len(filtered) - 1
		}

		return m, nil
	case "enter":
		m = m.openInvoiceEditor(filtered[m.invoiceCursor]).(TheApplication)

		return m, nil
	case "backspace", "delete":
		selected := filtered[m.invoiceCursor]
		m = m.beginDeleteConfirmation("invoice", selected.ID, selected.Title)

		return m, nil
	default:
		return m, nil
	}
}

func (m TheApplication) updateInvoiceEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenInvoiceList
		m.status = "invoice edit cancelled"

		return m, nil
	case "up", "shift+tab":
		m.editInvoiceForm = m.editInvoiceForm.Prev()

		return m, nil
	case "down", "tab":
		m.editInvoiceForm = m.editInvoiceForm.Next()

		return m, nil
	case "left":
		if m.editInvoiceForm.ActiveField == invoice.InvoiceFieldType {
			m.editInvoiceForm.IsIncoming = true
		}

		if m.editInvoiceForm.ActiveField == invoice.InvoiceFieldCurrency && m.editInvoiceForm.CurrencyIndex > 0 {
			m.editInvoiceForm.CurrencyIndex--
		}

		if m.editInvoiceForm.ActiveField == invoice.InvoiceFieldTargetAccountChoice && m.editInvoiceForm.AccountIndex > 0 {
			m.editInvoiceForm.AccountIndex--
		}

		return m, nil
	case "right":
		if m.editInvoiceForm.ActiveField == invoice.InvoiceFieldType {
			m.editInvoiceForm.IsIncoming = false
		}

		if m.editInvoiceForm.ActiveField == invoice.InvoiceFieldCurrency && m.editInvoiceForm.CurrencyIndex < len(m.editInvoiceForm.CurrencyOptions)-1 {
			m.editInvoiceForm.CurrencyIndex++
		}

		if m.editInvoiceForm.ActiveField == invoice.InvoiceFieldTargetAccountChoice && m.editInvoiceForm.AccountIndex < len(m.editInvoiceForm.AccountOptions)-1 {
			m.editInvoiceForm.AccountIndex++
		}

		return m, nil
	case " ":
		if m.editInvoiceForm.ActiveField == invoice.InvoiceFieldPaid {
			m.editInvoiceForm.Paid = !m.editInvoiceForm.Paid

			return m, nil
		}
	case "enter":
		if m.editInvoiceForm.ActiveField == invoice.InvoiceFieldCount-1 {
			return m.saveInvoiceEdit()
		}

		m.editInvoiceForm = m.editInvoiceForm.Next()

		return m, nil
	}

	var cmd tea.Cmd
	switch m.editInvoiceForm.ActiveField {
	case invoice.InvoiceFieldTitle:
		m.editInvoiceForm.TitleInput, cmd = m.editInvoiceForm.TitleInput.Update(msg)
	case invoice.InvoiceFieldAmount:
		m.editInvoiceForm.AmountInput, cmd = m.editInvoiceForm.AmountInput.Update(msg)
	case invoice.InvoiceFieldPeer:
		m.editInvoiceForm.PeerInput, cmd = m.editInvoiceForm.PeerInput.Update(msg)
	case invoice.InvoiceFieldInvoiceDate:
		m.editInvoiceForm.InvoiceDateInput, cmd = m.editInvoiceForm.InvoiceDateInput.Update(msg)
	case invoice.InvoiceFieldDueDate:
		m.editInvoiceForm.DueDateInput, cmd = m.editInvoiceForm.DueDateInput.Update(msg)
	case invoice.InvoiceFieldTargetAccountName:
		m.editInvoiceForm.TargetAccountInput, cmd = m.editInvoiceForm.TargetAccountInput.Update(msg)
	case invoice.InvoiceFieldURL:
		m.editInvoiceForm.URLInput, cmd = m.editInvoiceForm.URLInput.Update(msg)
	case invoice.InvoiceFieldDescription:
		m.editInvoiceForm.DescriptionInput, cmd = m.editInvoiceForm.DescriptionInput.Update(msg)
	}

	return m, cmd
}

func (m TheApplication) saveTaxFromForm() (tea.Model, tea.Cmd) {
	if len(m.addTaxForm.TaxTypeOptions) == 0 {
		m.status = "no tax types configured; add one in settings"
		return m, nil
	}

	taxTypeIndex := m.addTaxForm.TaxTypeIndex
	if taxTypeIndex < 0 || taxTypeIndex >= len(m.addTaxForm.TaxTypeOptions) {
		taxTypeIndex = 0
	}
	selectedType := m.addTaxForm.TaxTypeOptions[taxTypeIndex]
	amountDueRaw := strings.TrimSpace(m.addTaxForm.Inputs[0].Value())
	amountPaidRaw := strings.TrimSpace(m.addTaxForm.Inputs[1].Value())
	period := strings.TrimSpace(m.addTaxForm.Inputs[2].Value())
	dueDateRaw := strings.TrimSpace(m.addTaxForm.Inputs[3].Value())
	comment := strings.TrimSpace(m.addTaxForm.Inputs[4].Value())

	if period == "" {
		m.status = "period is required"
		return m, nil
	}

	amountDue, err := parseAmountCents(amountDueRaw)
	if err != nil {
		m.status = "amount due error: " + err.Error()
		return m, nil
	}
	if amountDue <= 0 {
		m.status = "amount due must be greater than zero"
		return m, nil
	}

	amountPaid, err := parseAmountCents(amountPaidRaw)
	if err != nil {
		m.status = "amount paid error: " + err.Error()
		return m, nil
	}

	dueDate, err := parseOptionalDatePointer(dueDateRaw)
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	now := time.Now()
	newTax := tax.Tax{
		TaxTypeID:       selectedType.ID,
		TaxCountry:      strings.TrimSpace(selectedType.Country),
		TaxTypeName:     strings.TrimSpace(selectedType.TaxTypeName),
		AmountDueCents:  amountDue,
		AmountPaidCents: amountPaid,
		Period:          period,
		DueDate:         dueDate,
		Comment:         comment,
		LastUpdatedAt:   now,
	}

	if err := m.db.Create(&newTax).Error; err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.taxes = append([]tax.Tax{newTax}, m.taxes...)
	m.addTaxForm = tax.NewAddTaxForm(m.settings.TaxTypes)
	m.screen = screenTaxList
	if newTax.AmountPaidCents >= newTax.AmountDueCents {
		m.taxMode = taxListHistory
	} else {
		m.taxMode = taxListUnpaid
	}
	m.taxCursor = 0
	m.status = "saved tax " + newTax.TaxCountry + " / " + newTax.TaxTypeName
	return m, nil
}

func (m TheApplication) saveTaxEdit() (tea.Model, tea.Cmd) {
	index := m.findTaxIndex(m.editingTaxID)
	if index < 0 {
		m.status = "tax not found"
		return m, nil
	}

	amountDue, err := parseAmountCents(strings.TrimSpace(m.editTaxForm.AmountDueInput.Value()))
	if err != nil {
		m.status = "amount due error: " + err.Error()
		return m, nil
	}
	if amountDue <= 0 {
		m.status = "amount due must be greater than zero"
		return m, nil
	}

	amountPaid, err := parseAmountCents(strings.TrimSpace(m.editTaxForm.AmountPaidInput.Value()))
	if err != nil {
		m.status = "amount paid error: " + err.Error()
		return m, nil
	}

	period := strings.TrimSpace(m.editTaxForm.PeriodInput.Value())
	if period == "" {
		m.status = "period is required"
		return m, nil
	}

	dueDate, err := parseOptionalDatePointer(strings.TrimSpace(m.editTaxForm.DueDateInput.Value()))
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	selected := m.taxes[index]
	selected.AmountDueCents = amountDue
	selected.AmountPaidCents = amountPaid
	selected.Period = period
	selected.DueDate = dueDate
	selected.Comment = strings.TrimSpace(m.editTaxForm.CommentInput.Value())
	selected.LastUpdatedAt = time.Now()

	if err := m.db.Save(&selected).Error; err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.taxes[index] = selected
	m.screen = screenTaxList
	m.status = "updated tax " + selected.TaxCountry + " / " + selected.TaxTypeName
	return m, nil
}

func (m TheApplication) applyTaxLogDelta() (tea.Model, tea.Cmd) {
	index := m.findTaxIndex(m.editingTaxID)
	if index < 0 {
		m.status = "tax not found"
		return m, nil
	}

	delta, err := parseSignedAmountCents(strings.TrimSpace(m.editTaxForm.LogDeltaInput.Value()))
	if err != nil {
		m.status = "log delta error: " + err.Error()
		return m, nil
	}
	if delta == 0 {
		m.status = "delta cannot be zero"
		return m, nil
	}

	selected := m.taxes[index]
	nextPaid := selected.AmountPaidCents + delta
	if nextPaid < 0 {
		m.status = "delta makes amount paid negative"
		return m, nil
	}

	entryTime, err := parseLogDateOrToday(strings.TrimSpace(m.editTaxForm.LogDateInput.Value()))
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	now := time.Now()
	selected.AmountPaidCents = nextPaid
	selected.LastUpdatedAt = now

	if err := m.db.Save(&selected).Error; err != nil {
		m.status = "tax update failed: " + err.Error()

		return m, nil
	}

	note := strings.TrimSpace(m.editTaxForm.LogCommentInput.Value())
	if note == "" {
		note = "manual tax paid adjustment"
	}

	entry := tax.TaxLog{
		TaxID:          selected.ID,
		DeltaPaidCents: delta,
		Note:           note,
		CreatedAt:      entryTime,
	}

	if err := m.db.Create(&entry).Error; err != nil {
		m.status = "log save failed: " + err.Error()

		return m, nil
	}

	m.taxes[index] = selected
	m.taxLogs = append([]tax.TaxLog{entry}, m.taxLogs...)
	m.editTaxForm.AmountPaidInput.SetValue(formatAmount(selected.AmountPaidCents))
	m.editTaxForm.LogDeltaInput.SetValue("")
	m.editTaxForm.LogDateInput.SetValue(time.Now().Format("02.01.2006"))
	m.editTaxForm.LogCommentInput.SetValue("")
	m.status = "applied tax log delta"

	return m, nil
}

func (m TheApplication) findTaxIndex(id uint) int {
	for i := range m.taxes {
		if m.taxes[i].ID == id {
			return i
		}
	}

	return -1
}

func (m TheApplication) filteredTaxes() []tax.Tax {
	filtered := make([]tax.Tax, 0, len(m.taxes))

	for _, item := range m.taxes {
		paid := item.AmountPaidCents >= item.AmountDueCents
		switch m.taxMode {
		case taxListUnpaid:
			if !paid {
				filtered = append(filtered, item)
			}
		case taxListHistory:
			if paid {
				filtered = append(filtered, item)
			}
		}
	}

	return filtered
}

func (m TheApplication) saveGoalEdit() (tea.Model, tea.Cmd) {
	index := m.findGoalIndex(m.editingGoalID)
	if index < 0 {
		m.status = "goal not found"
		return m, nil
	}

	target, err := parseAmountCents(strings.TrimSpace(m.editGoalForm.TargetAmountInput.Value()))
	if err != nil {
		m.status = "target amount error: " + err.Error()
		return m, nil
	}
	if target <= 0 {
		m.status = "target amount must be greater than zero"
		return m, nil
	}

	accumulated, err := parseAmountCents(strings.TrimSpace(m.editGoalForm.AccumulatedAmountInput.Value()))
	if err != nil {
		m.status = "accumulated amount error: " + err.Error()
		return m, nil
	}
	if accumulated > target {
		m.status = "accumulated amount cannot be more than target"
		return m, nil
	}

	dateStarted, err := parseRequiredDate(strings.TrimSpace(m.editGoalForm.DateStartedInput.Value()))
	if err != nil {
		m.status = err.Error()
		return m, nil
	}
	targetDate, err := parseOptionalDatePointer(strings.TrimSpace(m.editGoalForm.TargetDateInput.Value()))
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	selected := m.goals[index]
	selected.TargetAmountCents = target
	selected.AmountAccumulatedCents = accumulated
	selected.DateStartedAt = dateStarted
	selected.TargetDate = targetDate
	selected.Description = strings.TrimSpace(m.editGoalForm.DescriptionInput.Value())
	selected.LastUpdatedAt = time.Now()

	if err := m.db.Save(&selected).Error; err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.goals[index] = selected
	m.screen = screenGoalList
	m.status = "updated goal " + selected.Name
	return m, nil
}

func (m TheApplication) applyGoalLogDelta() (tea.Model, tea.Cmd) {
	index := m.findGoalIndex(m.editingGoalID)
	if index < 0 {
		m.status = "goal not found"
		return m, nil
	}

	delta, err := parseSignedAmountCents(strings.TrimSpace(m.editGoalForm.LogDeltaInput.Value()))
	if err != nil {
		m.status = "log delta error: " + err.Error()
		return m, nil
	}
	if delta == 0 {
		m.status = "delta cannot be zero"
		return m, nil
	}

	selected := m.goals[index]
	nextAccumulated := selected.AmountAccumulatedCents + delta
	if nextAccumulated < 0 || nextAccumulated > selected.TargetAmountCents {
		m.status = "delta makes accumulated amount out of range"
		return m, nil
	}

	entryTime, err := parseLogDateOrToday(strings.TrimSpace(m.editGoalForm.LogDateInput.Value()))
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	now := time.Now()
	selected.AmountAccumulatedCents = nextAccumulated
	selected.LastUpdatedAt = now
	if err := m.db.Save(&selected).Error; err != nil {
		m.status = "goal update failed: " + err.Error()
		return m, nil
	}

	note := strings.TrimSpace(m.editGoalForm.LogCommentInput.Value())
	if note == "" {
		note = "manual accumulated adjustment"
	}
	entry := goal.GoalLog{
		GoalID:                selected.ID,
		DeltaAccumulatedCents: delta,
		Note:                  note,
		CreatedAt:             entryTime,
	}
	if err := m.db.Create(&entry).Error; err != nil {
		m.status = "log save failed: " + err.Error()
		return m, nil
	}

	m.goals[index] = selected
	m.goalLogs = append([]goal.GoalLog{entry}, m.goalLogs...)
	m.editGoalForm.AccumulatedAmountInput.SetValue(formatAmount(selected.AmountAccumulatedCents))
	m.editGoalForm.LogDeltaInput.SetValue("")
	m.editGoalForm.LogDateInput.SetValue(time.Now().Format("02.01.2006"))
	m.editGoalForm.LogCommentInput.SetValue("")
	m.status = "applied goal log delta"
	return m, nil
}

func (m TheApplication) findGoalIndex(id uint) int {
	for i := range m.goals {
		if m.goals[i].ID == id {
			return i
		}
	}
	return -1
}

func (m TheApplication) filteredGoals() []goal.Goal {
	filtered := make([]goal.Goal, 0, len(m.goals))
	for _, item := range m.goals {
		done := item.AmountAccumulatedCents >= item.TargetAmountCents
		switch m.goalMode {
		case goalListActive:
			if !done {
				filtered = append(filtered, item)
			}
		case goalListHistory:
			if done {
				filtered = append(filtered, item)
			}
		}
	}
	return filtered
}

func (m TheApplication) goalProgressTotalsBase(items []goal.Goal) (accumulated int64, target int64) {
	for _, item := range items {
		itemTarget := item.TargetAmountCents
		itemAccumulated := item.AmountAccumulatedCents
		if itemTarget < 0 {
			itemTarget = 0
		}
		if itemAccumulated < 0 {
			itemAccumulated = 0
		}
		if itemAccumulated > itemTarget {
			itemAccumulated = itemTarget
		}

		targetBase, targetOK := m.convertToBaseCents(item.Currency, itemTarget)
		accumulatedBase, accumulatedOK := m.convertToBaseCents(item.Currency, itemAccumulated)
		if targetOK && accumulatedOK {
			target += targetBase
			accumulated += accumulatedBase
			continue
		}

		target += itemTarget
		accumulated += itemAccumulated
	}

	if accumulated > target {
		accumulated = target
	}
	if accumulated < 0 {
		accumulated = 0
	}
	if target < 0 {
		target = 0
	}

	return accumulated, target
}

func (m TheApplication) updateTaxNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.addTaxForm.TaxTypeOptions) == 0 {
		if msg.String() == "esc" {
			m.screen = screenMenu
			m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		return m, nil
	case "up", "shift+tab":
		m.addTaxForm = m.addTaxForm.Prev()
		return m, nil
	case "down", "tab":
		m.addTaxForm = m.addTaxForm.Next()
		return m, nil
	case "left":
		if m.addTaxForm.Active == tax.TaxFieldTaxType && m.addTaxForm.TaxTypeIndex > 0 {
			m.addTaxForm.TaxTypeIndex--
		}
		return m, nil
	case "right":
		if m.addTaxForm.Active == tax.TaxFieldTaxType && m.addTaxForm.TaxTypeIndex < len(m.addTaxForm.TaxTypeOptions)-1 {
			m.addTaxForm.TaxTypeIndex++
		}
		return m, nil
	case "enter":
		if m.addTaxForm.Active == tax.TaxFieldCount-1 {
			return m.saveTaxFromForm()
		}
		m.addTaxForm = m.addTaxForm.Next()
		return m, nil
	}

	if inputIndex := m.addTaxForm.InputIndexForField(m.addTaxForm.Active); inputIndex >= 0 {
		var cmd tea.Cmd
		m.addTaxForm.Inputs[inputIndex], cmd = m.addTaxForm.Inputs[inputIndex].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m TheApplication) updateTaxList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if updatedModel, cmd, handled := m.handleDeleteConfirmation(msg); handled {
		return updatedModel, cmd
	}

	filtered := m.filteredTaxes()
	if len(filtered) == 0 {
		if msg.String() == "esc" {
			m.screen = screenMenu
			m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		return m, nil
	case "up":
		if m.taxCursor > 0 {
			m.taxCursor--
		}
		return m, nil
	case "down":
		if m.taxCursor < len(filtered)-1 {
			m.taxCursor++
		}
		return m, nil
	case "enter":
		m = m.openTaxEditor(filtered[m.taxCursor]).(TheApplication)
		return m, nil
	case "backspace", "delete":
		selected := filtered[m.taxCursor]
		m = m.beginDeleteConfirmation("tax", selected.ID, selected.TaxCountry+" / "+selected.TaxTypeName)
		return m, nil
	default:
		return m, nil
	}
}

func (m TheApplication) openTaxEditor(selected tax.Tax) tea.Model {
	m.screen = screenTaxEdit
	m.editingTaxID = selected.ID
	m.editTaxForm = tax.NewEditTaxForm()
	m.editTaxForm.AmountDueInput.SetValue(formatAmount(selected.AmountDueCents))
	m.editTaxForm.AmountPaidInput.SetValue(formatAmount(selected.AmountPaidCents))
	m.editTaxForm.PeriodInput.SetValue(selected.Period)
	if selected.DueDate != nil {
		m.editTaxForm.DueDateInput.SetValue(selected.DueDate.Local().Format("02.01.2006"))
	} else {
		m.editTaxForm.DueDateInput.SetValue("")
	}
	m.editTaxForm.CommentInput.SetValue(selected.Comment)
	m.editTaxForm.LogDateInput.SetValue(time.Now().Format("02.01.2006"))
	m.editTaxForm.LogCommentInput.SetValue("")
	m.editTaxForm.TaxTypeLabel = selected.TaxTypeName
	m.editTaxForm.CountryLabel = selected.TaxCountry
	m.editTaxForm = m.editTaxForm.FocusActive()

	var logs []tax.TaxLog
	if err := m.db.Where("tax_id = ?", selected.ID).Order("created_at desc, id desc").Limit(20).Find(&logs).Error; err == nil {
		m.taxLogs = logs
	} else {
		m.taxLogs = nil
	}

	m.status = "editing tax " + selected.TaxCountry + " / " + selected.TaxTypeName
	return m
}

func (m TheApplication) updateTaxEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenTaxList
		m.status = "tax edit cancelled"
		return m, nil
	case "up", "shift+tab":
		m.editTaxForm = m.editTaxForm.Prev()
		return m, nil
	case "down", "tab":
		m.editTaxForm = m.editTaxForm.Next()
		return m, nil
	case "enter":
		if m.editTaxForm.ActiveField == tax.EditTaxFieldLogComment {
			return m.applyTaxLogDelta()
		}
		if m.editTaxForm.ActiveField == tax.EditTaxFieldComment {
			return m.saveTaxEdit()
		}
		m.editTaxForm = m.editTaxForm.Next()
		return m, nil
	}

	var cmd tea.Cmd
	switch m.editTaxForm.ActiveField {
	case tax.EditTaxFieldAmountDue:
		m.editTaxForm.AmountDueInput, cmd = m.editTaxForm.AmountDueInput.Update(msg)
	case tax.EditTaxFieldAmountPaid:
		m.editTaxForm.AmountPaidInput, cmd = m.editTaxForm.AmountPaidInput.Update(msg)
	case tax.EditTaxFieldPeriod:
		m.editTaxForm.PeriodInput, cmd = m.editTaxForm.PeriodInput.Update(msg)
	case tax.EditTaxFieldDueDate:
		m.editTaxForm.DueDateInput, cmd = m.editTaxForm.DueDateInput.Update(msg)
	case tax.EditTaxFieldComment:
		m.editTaxForm.CommentInput, cmd = m.editTaxForm.CommentInput.Update(msg)
	case tax.EditTaxFieldLogDelta:
		m.editTaxForm.LogDeltaInput, cmd = m.editTaxForm.LogDeltaInput.Update(msg)
	case tax.EditTaxFieldLogDate:
		m.editTaxForm.LogDateInput, cmd = m.editTaxForm.LogDateInput.Update(msg)
	case tax.EditTaxFieldLogComment:
		m.editTaxForm.LogCommentInput, cmd = m.editTaxForm.LogCommentInput.Update(msg)
	}
	return m, cmd
}

func (m TheApplication) updateDebtNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		return m, nil
	case "up", "shift+tab":
		m.addDebtForm = m.addDebtForm.Prev()
		return m, nil
	case "down", "tab":
		m.addDebtForm = m.addDebtForm.Next()
		return m, nil
	case "left":
		if m.addDebtForm.Active == debt.DebtFieldDirection {
			m.addDebtForm.IsOwedToUser = false
		}
		if m.addDebtForm.Active == debt.DebtFieldCurrency && m.addDebtForm.CurrencyIndex > 0 {
			m.addDebtForm.CurrencyIndex--
		}
		return m, nil
	case "right":
		if m.addDebtForm.Active == debt.DebtFieldDirection {
			m.addDebtForm.IsOwedToUser = true
		}
		if m.addDebtForm.Active == debt.DebtFieldCurrency && m.addDebtForm.CurrencyIndex < len(m.addDebtForm.CurrencyOptions)-1 {
			m.addDebtForm.CurrencyIndex++
		}
		return m, nil
	case " ":
		if m.addDebtForm.Active == debt.DebtFieldDirection {
			m.addDebtForm.IsOwedToUser = !m.addDebtForm.IsOwedToUser
			return m, nil
		}
	case "enter":
		if m.addDebtForm.Active == debt.DebtFieldCount-1 {
			return m.saveDebtFromForm()
		}
		m.addDebtForm = m.addDebtForm.Next()
		return m, nil
	}

	if inputIndex := m.addDebtForm.InputIndexForField(m.addDebtForm.Active); inputIndex >= 0 {
		var cmd tea.Cmd
		m.addDebtForm.Inputs[inputIndex], cmd = m.addDebtForm.Inputs[inputIndex].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m TheApplication) updateDebtList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if updatedModel, cmd, handled := m.handleDeleteConfirmation(msg); handled {
		return updatedModel, cmd
	}

	filtered := m.filteredDebts()
	if len(filtered) == 0 {
		if msg.String() == "esc" {
			m.screen = screenMenu
			m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		return m, nil
	case "up":
		if m.debtCursor > 0 {
			m.debtCursor--
		}
		return m, nil
	case "down":
		if m.debtCursor < len(filtered)-1 {
			m.debtCursor++
		}
		return m, nil
	case "enter":
		m = m.openDebtEditor(filtered[m.debtCursor]).(TheApplication)
		return m, nil
	case "backspace", "delete":
		selected := filtered[m.debtCursor]
		m = m.beginDeleteConfirmation("debt", selected.ID, selected.Peer)
		return m, nil
	default:
		return m, nil
	}
}

func (m TheApplication) openDebtEditor(selected debt.Debt) tea.Model {
	m.screen = screenDebtEdit
	m.editingDebtID = selected.ID
	m.editDebtForm = debt.NewEditDebtForm()
	m.editDebtForm.AmountInput.SetValue(formatAmount(selected.AmountCents))
	m.editDebtForm.AmountPaidInput.SetValue(formatAmount(selected.AmountPaidCents))
	m.editDebtForm.DebtCreatedInput.SetValue(selected.DebtCreatedAt.Local().Format("02.01.2006"))
	if selected.DueDate != nil {
		m.editDebtForm.DueDateInput.SetValue(selected.DueDate.Local().Format("02.01.2006"))
	}
	m.editDebtForm.CommentInput.SetValue(selected.Comment)
	m.editDebtForm.LogDateInput.SetValue(time.Now().Format("02.01.2006"))
	m.editDebtForm.LogCommentInput.SetValue("")
	m.editDebtForm.PeerLabel = selected.Peer
	m.editDebtForm.CurrencyLabel = selected.Currency
	m.editDebtForm.DirectionLabel = debtDirectionLabel(selected.IsOwedToUser)
	m.editDebtForm = m.editDebtForm.FocusActive()

	var logs []debt.DebtLog
	if err := m.db.Where("debt_id = ?", selected.ID).Order("created_at desc, id desc").Limit(20).Find(&logs).Error; err == nil {
		m.debtLogs = logs
	} else {
		m.debtLogs = nil
	}

	m.status = "editing debt " + selected.Peer
	return m
}

func (m TheApplication) updateDebtEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenDebtList
		m.status = "debt edit cancelled"
		return m, nil
	case "up", "shift+tab":
		m.editDebtForm = m.editDebtForm.Prev()
		return m, nil
	case "down", "tab":
		m.editDebtForm = m.editDebtForm.Next()
		return m, nil
	case "enter":
		if m.editDebtForm.ActiveField == debt.EditDebtFieldLogComment {
			return m.applyDebtLogDelta()
		}
		if m.editDebtForm.ActiveField == debt.EditDebtFieldComment {
			return m.saveDebtEdit()
		}
		m.editDebtForm = m.editDebtForm.Next()
		return m, nil
	}

	var cmd tea.Cmd
	switch m.editDebtForm.ActiveField {
	case debt.EditDebtFieldAmount:
		m.editDebtForm.AmountInput, cmd = m.editDebtForm.AmountInput.Update(msg)
	case debt.EditDebtFieldAmountPaid:
		m.editDebtForm.AmountPaidInput, cmd = m.editDebtForm.AmountPaidInput.Update(msg)
	case debt.EditDebtFieldDebtCreated:
		m.editDebtForm.DebtCreatedInput, cmd = m.editDebtForm.DebtCreatedInput.Update(msg)
	case debt.EditDebtFieldDueDate:
		m.editDebtForm.DueDateInput, cmd = m.editDebtForm.DueDateInput.Update(msg)
	case debt.EditDebtFieldComment:
		m.editDebtForm.CommentInput, cmd = m.editDebtForm.CommentInput.Update(msg)
	case debt.EditDebtFieldLogDelta:
		m.editDebtForm.LogDeltaInput, cmd = m.editDebtForm.LogDeltaInput.Update(msg)
	case debt.EditDebtFieldLogDate:
		m.editDebtForm.LogDateInput, cmd = m.editDebtForm.LogDateInput.Update(msg)
	case debt.EditDebtFieldLogComment:
		m.editDebtForm.LogCommentInput, cmd = m.editDebtForm.LogCommentInput.Update(msg)
	}
	return m, cmd
}

func (m TheApplication) saveDebtFromForm() (tea.Model, tea.Cmd) {
	peer := strings.TrimSpace(m.addDebtForm.Inputs[0].Value())
	currency := selectedCurrencyOption(m.addDebtForm.CurrencyOptions, m.addDebtForm.CurrencyIndex)
	amountRaw := strings.TrimSpace(m.addDebtForm.Inputs[1].Value())
	amountPaidRaw := strings.TrimSpace(m.addDebtForm.Inputs[2].Value())
	debtCreatedRaw := strings.TrimSpace(m.addDebtForm.Inputs[3].Value())
	dueRaw := strings.TrimSpace(m.addDebtForm.Inputs[4].Value())
	comment := strings.TrimSpace(m.addDebtForm.Inputs[5].Value())

	if peer == "" {
		m.status = "peer is required"
		return m, nil
	}

	amount, err := parseAmountCents(amountRaw)
	if err != nil {
		m.status = "amount error: " + err.Error()
		return m, nil
	}

	amountPaid, err := parseAmountCents(amountPaidRaw)
	if err != nil {
		m.status = "amount paid error: " + err.Error()
		return m, nil
	}
	if amountPaid > amount {
		m.status = "amount paid cannot be more than amount"
		return m, nil
	}

	debtCreatedAt, err := parseRequiredDate(debtCreatedRaw)
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	dueDate, err := parseOptionalDatePointer(dueRaw)
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	now := time.Now()
	newDebt := debt.Debt{
		Peer:            peer,
		Currency:        currency,
		AmountCents:     amount,
		AmountPaidCents: amountPaid,
		IsOwedToUser:    m.addDebtForm.IsOwedToUser,
		DebtCreatedAt:   debtCreatedAt,
		DueDate:         dueDate,
		Comment:         comment,
		LastUpdatedAt:   now,
	}

	if err := m.db.Create(&newDebt).Error; err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.debts = append([]debt.Debt{newDebt}, m.debts...)
	m.addDebtForm = debt.NewAddDebtForm(currencySelectionOptions(m.settings))
	m.screen = screenDebtList
	if newDebt.IsOwedToUser {
		m.debtMode = debtListIncoming
	} else {
		m.debtMode = debtListOutgoing
	}
	m.debtCursor = 0
	m.status = "saved debt for " + peer
	return m, nil
}

func (m TheApplication) saveDebtEdit() (tea.Model, tea.Cmd) {
	index := m.findDebtIndex(m.editingDebtID)
	if index < 0 {
		m.status = "debt not found"
		return m, nil
	}

	amount, err := parseAmountCents(strings.TrimSpace(m.editDebtForm.AmountInput.Value()))
	if err != nil {
		m.status = "amount error: " + err.Error()
		return m, nil
	}
	amountPaid, err := parseAmountCents(strings.TrimSpace(m.editDebtForm.AmountPaidInput.Value()))
	if err != nil {
		m.status = "amount paid error: " + err.Error()
		return m, nil
	}
	if amountPaid > amount {
		m.status = "amount paid cannot be more than amount"
		return m, nil
	}

	debtCreatedAt, err := parseRequiredDate(strings.TrimSpace(m.editDebtForm.DebtCreatedInput.Value()))
	if err != nil {
		m.status = err.Error()
		return m, nil
	}
	dueDate, err := parseOptionalDatePointer(strings.TrimSpace(m.editDebtForm.DueDateInput.Value()))
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	selected := m.debts[index]
	selected.AmountCents = amount
	selected.AmountPaidCents = amountPaid
	selected.DebtCreatedAt = debtCreatedAt
	selected.DueDate = dueDate
	selected.Comment = strings.TrimSpace(m.editDebtForm.CommentInput.Value())
	selected.LastUpdatedAt = time.Now()

	if err := m.db.Save(&selected).Error; err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.debts[index] = selected
	m.screen = screenDebtList
	m.status = "updated debt for " + selected.Peer
	return m, nil
}

func (m TheApplication) applyDebtLogDelta() (tea.Model, tea.Cmd) {
	index := m.findDebtIndex(m.editingDebtID)
	if index < 0 {
		m.status = "debt not found"
		return m, nil
	}

	delta, err := parseSignedAmountCents(strings.TrimSpace(m.editDebtForm.LogDeltaInput.Value()))
	if err != nil {
		m.status = "log delta error: " + err.Error()
		return m, nil
	}
	if delta == 0 {
		m.status = "delta cannot be zero"
		return m, nil
	}

	selected := m.debts[index]
	nextPaid := selected.AmountPaidCents + delta
	if nextPaid < 0 || nextPaid > selected.AmountCents {
		m.status = "delta makes amount paid out of range"
		return m, nil
	}

	entryTime, err := parseLogDateOrToday(strings.TrimSpace(m.editDebtForm.LogDateInput.Value()))
	if err != nil {
		m.status = err.Error()
		return m, nil
	}
	now := time.Now()
	selected.AmountPaidCents = nextPaid
	selected.LastUpdatedAt = now
	if err := m.db.Save(&selected).Error; err != nil {
		m.status = "debt update failed: " + err.Error()
		return m, nil
	}

	note := strings.TrimSpace(m.editDebtForm.LogCommentInput.Value())
	if note == "" {
		note = "manual paid adjustment"
	}
	entry := debt.DebtLog{
		DebtID:         selected.ID,
		DeltaPaidCents: delta,
		Note:           note,
		CreatedAt:      entryTime,
	}
	if err := m.db.Create(&entry).Error; err != nil {
		m.status = "log save failed: " + err.Error()
		return m, nil
	}

	m.debts[index] = selected
	m.debtLogs = append([]debt.DebtLog{entry}, m.debtLogs...)
	m.editDebtForm.AmountPaidInput.SetValue(formatAmount(selected.AmountPaidCents))
	m.editDebtForm.LogDeltaInput.SetValue("")
	m.editDebtForm.LogDateInput.SetValue(time.Now().Format("02.01.2006"))
	m.editDebtForm.LogCommentInput.SetValue("")
	m.status = "applied log delta"
	return m, nil
}

func (m TheApplication) findDebtIndex(id uint) int {
	for i := range m.debts {
		if m.debts[i].ID == id {
			return i
		}
	}
	return -1
}

func (m TheApplication) filteredDebts() []debt.Debt {
	filtered := make([]debt.Debt, 0, len(m.debts))
	for _, item := range m.debts {
		paid := item.AmountPaidCents >= item.AmountCents
		switch m.debtMode {
		case debtListOutgoing:
			if !item.IsOwedToUser && !paid {
				filtered = append(filtered, item)
			}
		case debtListIncoming:
			if item.IsOwedToUser && !paid {
				filtered = append(filtered, item)
			}
		case debtListHistory:
			if paid {
				filtered = append(filtered, item)
			}
		}
	}
	return filtered
}

func (m TheApplication) updateGoalNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		return m, nil
	case "up", "shift+tab":
		m.addGoalForm = m.addGoalForm.Prev()
		return m, nil
	case "down", "tab":
		m.addGoalForm = m.addGoalForm.Next()
		return m, nil
	case "left":
		if m.addGoalForm.Active == goal.GoalFieldCurrency && m.addGoalForm.CurrencyIndex > 0 {
			m.addGoalForm.CurrencyIndex--
		}
		return m, nil
	case "right":
		if m.addGoalForm.Active == goal.GoalFieldCurrency && m.addGoalForm.CurrencyIndex < len(m.addGoalForm.CurrencyOptions)-1 {
			m.addGoalForm.CurrencyIndex++
		}
		return m, nil
	case "enter":
		if m.addGoalForm.Active == goal.GoalFieldCount-1 {
			return m.saveGoalFromForm()
		}
		m.addGoalForm = m.addGoalForm.Next()
		return m, nil
	}

	if inputIndex := m.addGoalForm.InputIndexForField(m.addGoalForm.Active); inputIndex >= 0 {
		var cmd tea.Cmd
		m.addGoalForm.Inputs[inputIndex], cmd = m.addGoalForm.Inputs[inputIndex].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m TheApplication) updateGoalList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if updatedModel, cmd, handled := m.handleDeleteConfirmation(msg); handled {
		return updatedModel, cmd
	}

	filtered := m.filteredGoals()
	if len(filtered) == 0 {
		if msg.String() == "esc" {
			m.screen = screenMenu
			m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		return m, nil
	case "up":
		if m.goalCursor > 0 {
			m.goalCursor--
		}
		return m, nil
	case "down":
		if m.goalCursor < len(filtered)-1 {
			m.goalCursor++
		}
		return m, nil
	case "enter":
		m = m.openGoalEditor(filtered[m.goalCursor]).(TheApplication)
		return m, nil
	case "backspace", "delete":
		selected := filtered[m.goalCursor]
		m = m.beginDeleteConfirmation("goal", selected.ID, selected.Name)
		return m, nil
	default:
		return m, nil
	}
}

func (m TheApplication) openGoalEditor(selected goal.Goal) tea.Model {
	m.screen = screenGoalEdit
	m.editingGoalID = selected.ID
	m.editGoalForm = goal.NewEditGoalForm()
	m.editGoalForm.TargetAmountInput.SetValue(formatAmount(selected.TargetAmountCents))
	m.editGoalForm.AccumulatedAmountInput.SetValue(formatAmount(selected.AmountAccumulatedCents))
	m.editGoalForm.DateStartedInput.SetValue(selected.DateStartedAt.Local().Format("02.01.2006"))
	if selected.TargetDate != nil {
		m.editGoalForm.TargetDateInput.SetValue(selected.TargetDate.Local().Format("02.01.2006"))
	}
	m.editGoalForm.DescriptionInput.SetValue(selected.Description)
	m.editGoalForm.LogDateInput.SetValue(time.Now().Format("02.01.2006"))
	m.editGoalForm.LogCommentInput.SetValue("")
	m.editGoalForm.NameLabel = selected.Name
	m.editGoalForm.CurrencyLabel = selected.Currency
	m.editGoalForm = m.editGoalForm.FocusActive()

	var logs []goal.GoalLog
	if err := m.db.Where("goal_id = ?", selected.ID).Order("created_at desc, id desc").Limit(20).Find(&logs).Error; err == nil {
		m.goalLogs = logs
	} else {
		m.goalLogs = nil
	}

	m.status = "editing goal " + selected.Name
	return m
}

func (m TheApplication) updateGoalEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenGoalList
		m.status = "goal edit cancelled"
		return m, nil
	case "up", "shift+tab":
		m.editGoalForm = m.editGoalForm.Prev()
		return m, nil
	case "down", "tab":
		m.editGoalForm = m.editGoalForm.Next()
		return m, nil
	case "enter":
		if m.editGoalForm.ActiveField == goal.EditGoalFieldLogComment {
			return m.applyGoalLogDelta()
		}
		if m.editGoalForm.ActiveField == goal.EditGoalFieldDescription {
			return m.saveGoalEdit()
		}
		m.editGoalForm = m.editGoalForm.Next()
		return m, nil
	}

	var cmd tea.Cmd
	switch m.editGoalForm.ActiveField {
	case goal.EditGoalFieldTargetAmount:
		m.editGoalForm.TargetAmountInput, cmd = m.editGoalForm.TargetAmountInput.Update(msg)
	case goal.EditGoalFieldAccumulated:
		m.editGoalForm.AccumulatedAmountInput, cmd = m.editGoalForm.AccumulatedAmountInput.Update(msg)
	case goal.EditGoalFieldDateStarted:
		m.editGoalForm.DateStartedInput, cmd = m.editGoalForm.DateStartedInput.Update(msg)
	case goal.EditGoalFieldTargetDate:
		m.editGoalForm.TargetDateInput, cmd = m.editGoalForm.TargetDateInput.Update(msg)
	case goal.EditGoalFieldDescription:
		m.editGoalForm.DescriptionInput, cmd = m.editGoalForm.DescriptionInput.Update(msg)
	case goal.EditGoalFieldLogDelta:
		m.editGoalForm.LogDeltaInput, cmd = m.editGoalForm.LogDeltaInput.Update(msg)
	case goal.EditGoalFieldLogDate:
		m.editGoalForm.LogDateInput, cmd = m.editGoalForm.LogDateInput.Update(msg)
	case goal.EditGoalFieldLogComment:
		m.editGoalForm.LogCommentInput, cmd = m.editGoalForm.LogCommentInput.Update(msg)
	}
	return m, cmd
}

func (m TheApplication) saveGoalFromForm() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.addGoalForm.Inputs[0].Value())
	currency := selectedCurrencyOption(m.addGoalForm.CurrencyOptions, m.addGoalForm.CurrencyIndex)
	targetRaw := strings.TrimSpace(m.addGoalForm.Inputs[1].Value())
	accumulatedRaw := strings.TrimSpace(m.addGoalForm.Inputs[2].Value())
	description := strings.TrimSpace(m.addGoalForm.Inputs[3].Value())
	dateStartedRaw := strings.TrimSpace(m.addGoalForm.Inputs[4].Value())
	targetDateRaw := strings.TrimSpace(m.addGoalForm.Inputs[5].Value())

	if name == "" {
		m.status = "goal name is required"
		return m, nil
	}

	target, err := parseAmountCents(targetRaw)
	if err != nil {
		m.status = "target amount error: " + err.Error()
		return m, nil
	}
	if target <= 0 {
		m.status = "target amount must be greater than zero"
		return m, nil
	}

	accumulated, err := parseAmountCents(accumulatedRaw)
	if err != nil {
		m.status = "accumulated amount error: " + err.Error()
		return m, nil
	}
	if accumulated > target {
		m.status = "accumulated amount cannot be more than target"
		return m, nil
	}

	dateStarted, err := parseRequiredDate(dateStartedRaw)
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	targetDate, err := parseOptionalDatePointer(targetDateRaw)
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	now := time.Now()
	newGoal := goal.Goal{
		Name:                   name,
		Currency:               currency,
		TargetAmountCents:      target,
		AmountAccumulatedCents: accumulated,
		Description:            description,
		DateStartedAt:          dateStarted,
		TargetDate:             targetDate,
		LastUpdatedAt:          now,
	}

	if err := m.db.Create(&newGoal).Error; err != nil {
		m.status = "save failed: " + err.Error()

		return m, nil
	}

	m.goals = append([]goal.Goal{newGoal}, m.goals...)
	m.addGoalForm = goal.NewAddGoalForm(currencySelectionOptions(m.settings))
	m.screen = screenGoalList

	if newGoal.AmountAccumulatedCents >= newGoal.TargetAmountCents {
		m.goalMode = goalListHistory
	} else {
		m.goalMode = goalListActive
	}

	m.goalCursor = 0
	m.status = "saved goal " + name

	return m, nil
}

func (m TheApplication) saveAccountFromForm() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.addForm.Fields[0].Value())
	description := strings.TrimSpace(m.addForm.Fields[1].Value())
	currency := selectedCurrencyOption(m.addForm.CurrencyOptions, m.addForm.CurrencyIndex)
	amountRaw := strings.TrimSpace(m.addForm.Fields[3].Value())

	if name == "" {
		m.status = "name is required"
		return m, nil
	}
	if description == "" {
		m.status = "description is required"
		return m, nil
	}
	if currency == "" {
		m.status = "currency is required"
		return m, nil
	}

	amount, err := parseAmountCents(amountRaw)
	if err != nil {
		m.status = "amount error: " + err.Error()
		return m, nil
	}

	now := time.Now()

	newAccount := account.Account{
		Name:              name,
		Description:       description,
		Currency:          currency,
		BalanceCents:      amount,
		LeftoverCents:     amount,
		IgnoreInSummaries: m.addForm.IgnoreInSummaries,
		LastUpdatedAt:     now,
	}

	if err := m.db.Create(&newAccount).Error; err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.accounts = append([]account.Account{newAccount}, m.accounts...)
	m.accounts = sortAccounts(m.accounts, m.settings, m.accountSortField)
	m.addForm = account.NewAddAccountForm(currencySelectionOptions(m.settings))
	m.screen = screenMenu
	m.status = "saved account " + name
	return m, nil
}

func (m TheApplication) saveSubscriptionFromForm() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.addSubscriptionForm.Inputs[0].Value())
	currency := selectedCurrencyOption(m.addSubscriptionForm.CurrencyOptions, m.addSubscriptionForm.CurrencyIndex)
	amountRaw := strings.TrimSpace(m.addSubscriptionForm.Inputs[2].Value())
	paymentMethodSelection := selectedPaymentMethodOption(m.addSubscriptionForm.PaymentMethodOptions, m.addSubscriptionForm.PaymentMethodIndex)
	paymentMethodCustom := strings.TrimSpace(m.addSubscriptionForm.Inputs[3].Value())
	paymentMethod := paymentMethodSelection
	if paymentMethodCustom != "" {
		paymentMethod = paymentMethodCustom
	}
	dayYearRaw := strings.TrimSpace(m.addSubscriptionForm.Inputs[4].Value())
	dayMonthRaw := strings.TrimSpace(m.addSubscriptionForm.Inputs[5].Value())

	if name == "" {
		m.status = "subscription name is required"
		return m, nil
	}
	if currency == "" {
		m.status = "currency is required"
		return m, nil
	}
	if paymentMethod == "" {
		m.status = "payment method is required"
		return m, nil
	}

	amount, err := parseAmountCents(amountRaw)
	if err != nil {
		m.status = "amount error: " + err.Error()
		return m, nil
	}

	dateYearly, err := parseOptionalDate(dayYearRaw)
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	dayMonthly, err := parseOptionalDay(dayMonthRaw, 1, 31, "monthly")
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	now := time.Now()
	newSubscription := subscription.Subscription{
		Name:              name,
		Currency:          currency,
		AmountCents:       amount,
		Period:            m.addSubscriptionForm.PeriodOptions[m.addSubscriptionForm.PeriodIndex],
		PaymentMethod:     paymentMethod,
		Type:              m.addSubscriptionForm.TypeOptions[m.addSubscriptionForm.TypeIndex],
		IsActive:          m.addSubscriptionForm.IsActive,
		PaymentDateYearly: dateYearly,
		PaymentDayMonthly: dayMonthly,
		LastUpdatedAt:     now,
	}

	if err := m.db.Create(&newSubscription).Error; err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.subscriptions = append([]subscription.Subscription{newSubscription}, m.subscriptions...)
	m.subscriptions = sortSubscriptionsByAmount(m.subscriptions)
	m.addSubscriptionForm = subscription.NewAddSubscriptionForm(currencySelectionOptions(m.settings), paymentMethodSelectionOptions(m.settings))
	m.screen = screenSubscriptionList
	m.subscriptionMode = subscriptionListAll
	m.subscriptionCursor = 0
	m.status = "saved subscription " + name
	return m, nil
}

func (m TheApplication) saveSubscriptionEdit() (tea.Model, tea.Cmd) {
	if m.editingSubscriptionID == 0 {
		m.status = "no subscription selected"
		return m, nil
	}

	amountRaw := strings.TrimSpace(m.editSubscriptionForm.AmountInput.Value())
	paymentMethod := strings.TrimSpace(m.editSubscriptionForm.PaymentMethodInput.Value())
	if paymentMethod == "" {
		m.status = "payment method is required"
		return m, nil
	}

	amount, err := parseAmountCents(amountRaw)
	if err != nil {
		m.status = "amount error: " + err.Error()
		return m, nil
	}

	selectedIndex := -1
	for i := range m.subscriptions {
		if m.subscriptions[i].ID == m.editingSubscriptionID {
			selectedIndex = i
			break
		}
	}
	if selectedIndex < 0 {
		m.status = "subscription not found"
		return m, nil
	}

	selected := m.subscriptions[selectedIndex]
	selected.AmountCents = amount
	selected.PaymentMethod = paymentMethod
	selected.IsActive = m.editSubscriptionForm.IsActive
	selected.LastUpdatedAt = time.Now()

	if err := m.db.Save(&selected).Error; err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.subscriptions[selectedIndex] = selected
	if m.subscriptionMode == subscriptionListAll {
		m.subscriptions = sortSubscriptionsByAmount(m.subscriptions)
	}
	m.screen = screenSubscriptionList
	filtered := m.filteredSubscriptions()
	if len(filtered) == 0 {
		m.subscriptionCursor = 0
	} else if m.subscriptionCursor >= len(filtered) {
		m.subscriptionCursor = len(filtered) - 1
	}
	m.status = "updated subscription " + selected.Name
	return m, nil
}

func (m TheApplication) beginEditAmount() tea.Model {
	if m.cursor < 0 || m.cursor >= len(m.accounts) {
		m.status = "no account selected"
		return m
	}

	current := m.accounts[m.cursor]
	m.editInput = textinput.New()
	m.editInput.Placeholder = "1234.56"
	m.editInput.CharLimit = 24
	m.editInput.Width = 20
	m.editInput.SetValue(formatAmount(current.BalanceCents))

	m.editAmountLogDateInput = textinput.New()
	m.editAmountLogDateInput.Placeholder = "DD.MM.YYYY"
	m.editAmountLogDateInput.CharLimit = 24
	m.editAmountLogDateInput.Width = 20
	m.editAmountLogDateInput.SetValue(time.Now().Format("02.01.2006"))

	m.editAmountLogValueInput = textinput.New()
	m.editAmountLogValueInput.Placeholder = "1234.56"
	m.editAmountLogValueInput.CharLimit = 24
	m.editAmountLogValueInput.Width = 20
	m.editAmountLogValueInput.SetValue(formatAmount(current.BalanceCents))

	m.editAmountActiveField = editAmountFieldCurrent
	m.editAmountUpdateLog = false
	m.editAmountIgnoreInSummaries = current.IgnoreInSummaries
	m.editInput.Focus()
	m.editAmountLogDateInput.Blur()
	m.editAmountLogValueInput.Blur()
	m.accountValueLogs = m.loadAccountValueLogs(current.ID)
	m.screen = screenEditAmount
	m.status = "editing account " + current.Name
	return m
}

func (m TheApplication) saveAmount() (tea.Model, tea.Cmd) {
	if m.cursor < 0 || m.cursor >= len(m.accounts) {
		m.status = "no account selected"
		return m, nil
	}

	amount, err := parseAmountCents(strings.TrimSpace(m.editInput.Value()))
	if err != nil {
		m.status = "amount error: " + err.Error()
		return m, nil
	}

	selected := m.accounts[m.cursor]
	now := time.Now()
	if err := m.db.Model(&account.Account{}).Where("id = ?", selected.ID).Updates(map[string]any{"balance_cents": amount, "leftover_cents": amount, "last_updated_at": now}).Error; err != nil {
		m.status = "update failed: " + err.Error()
		return m, nil
	}

	logWarn := ""
	if m.editAmountUpdateLog {
		if err := m.upsertAccountValueLog(selected.ID, now, amount); err != nil {
			logWarn = " (log update failed: " + err.Error() + ")"
		}
	}

	selected.BalanceCents = amount
	selected.LeftoverCents = amount
	selected.LastUpdatedAt = now
	selected.IgnoreInSummaries = m.editAmountIgnoreInSummaries
	m.accounts[m.cursor] = selected
	m.accounts = sortAccounts(m.accounts, m.settings, m.accountSortField)
	m.cursor = findAccountIndex(m.accounts, selected.ID)
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.screen = screenAccountTable
	m.status = "updated amount for " + selected.Name + logWarn
	return m, nil
}

func (m TheApplication) applyAccountLogValue() (tea.Model, tea.Cmd) {
	if m.cursor < 0 || m.cursor >= len(m.accounts) {
		m.status = "no account selected"
		return m, nil
	}

	rawDay := strings.TrimSpace(m.editAmountLogDateInput.Value())
	if rawDay == "" {
		m.status = "log date is required"
		return m, nil
	}
	day, err := time.ParseInLocation("02.01.2006", rawDay, time.Now().Location())
	if err != nil {
		m.status = "log date must use DD.MM.YYYY format"
		return m, nil
	}

	value, err := parseAmountCents(strings.TrimSpace(m.editAmountLogValueInput.Value()))
	if err != nil {
		m.status = "log value error: " + err.Error()
		return m, nil
	}

	selected := m.accounts[m.cursor]
	if err := m.upsertAccountValueLog(selected.ID, day, value); err != nil {
		m.status = "log save failed: " + err.Error()
		return m, nil
	}

	m.accountValueLogs = m.loadAccountValueLogs(selected.ID)
	m.status = "saved log value for " + accountLogDay(day).Format("2006-01-02")
	return m, nil
}

func (m TheApplication) beginDeleteConfirmation(targetType string, targetID uint, targetName string) TheApplication {
	m.deleteConfirmActive = true
	m.deleteConfirmType = targetType
	m.deleteConfirmID = targetID
	m.deleteConfirmName = strings.TrimSpace(targetName)

	displayName := m.deleteConfirmName
	if displayName == "" {
		displayName = fmt.Sprintf("id=%d", targetID)
	}
	m.status = fmt.Sprintf("confirm delete %s %s? press y to confirm, n to cancel", targetType, displayName)

	return m
}

func (m TheApplication) clearDeleteConfirmation(status string) TheApplication {
	m.deleteConfirmActive = false
	m.deleteConfirmType = ""
	m.deleteConfirmID = 0
	m.deleteConfirmName = ""
	if strings.TrimSpace(status) != "" {
		m.status = status
	}
	return m
}

func (m TheApplication) handleDeleteConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	if !m.deleteConfirmActive {
		return m, nil, false
	}

	switch strings.ToLower(msg.String()) {
	case "y", "enter":
		updatedModel, cmd := m.applyConfirmedDelete()
		return updatedModel, cmd, true
	case "n", "esc":
		m = m.clearDeleteConfirmation("delete cancelled")
		return m, nil, true
	default:
		return m, nil, true
	}
}

func (m TheApplication) applyConfirmedDelete() (tea.Model, tea.Cmd) {
	switch m.deleteConfirmType {
	case "account":
		return m.confirmDeleteAccount()
	case "subscription":
		return m.confirmDeleteSubscription()
	case "cashflow":
		return m.confirmDeleteCashflow()
	case "debt":
		return m.confirmDeleteDebt()
	case "goal":
		return m.confirmDeleteGoal()
	case "tax":
		return m.confirmDeleteTax()
	case "invoice":
		return m.confirmDeleteInvoice()
	default:
		m = m.clearDeleteConfirmation("delete target is invalid")
		return m, nil
	}
}

func (m TheApplication) confirmDeleteAccount() (tea.Model, tea.Cmd) {
	index := findAccountIndex(m.accounts, m.deleteConfirmID)
	if index < 0 {
		m = m.clearDeleteConfirmation("account not found")
		return m, nil
	}

	selected := m.accounts[index]
	if err := m.db.Where("account_id = ?", selected.ID).Delete(&account.AccountValueLog{}).Error; err != nil {
		m = m.clearDeleteConfirmation("delete failed: " + err.Error())
		return m, nil
	}
	if err := m.db.Delete(&account.Account{}, selected.ID).Error; err != nil {
		m = m.clearDeleteConfirmation("delete failed: " + err.Error())
		return m, nil
	}

	m.accounts = append(m.accounts[:index], m.accounts[index+1:]...)
	if m.cursor >= len(m.accounts) && m.cursor > 0 {
		m.cursor--
	}
	if len(m.accounts) == 0 {
		m.screen = screenMenu
		m.cursor = 0
	}

	m = m.clearDeleteConfirmation("deleted account " + selected.Name)
	return m, nil
}

func (m TheApplication) confirmDeleteSubscription() (tea.Model, tea.Cmd) {
	index := -1
	for i := range m.subscriptions {
		if m.subscriptions[i].ID == m.deleteConfirmID {
			index = i
			break
		}
	}
	if index < 0 {
		m = m.clearDeleteConfirmation("subscription not found")
		return m, nil
	}

	selected := m.subscriptions[index]
	if err := m.db.Delete(&subscription.Subscription{}, selected.ID).Error; err != nil {
		m = m.clearDeleteConfirmation("delete failed: " + err.Error())
		return m, nil
	}

	m.subscriptions = append(m.subscriptions[:index], m.subscriptions[index+1:]...)
	filteredAfter := m.filteredSubscriptions()
	if len(filteredAfter) == 0 {
		m.subscriptionCursor = 0
	} else if m.subscriptionCursor >= len(filteredAfter) {
		m.subscriptionCursor = len(filteredAfter) - 1
	}

	m = m.clearDeleteConfirmation("deleted subscription " + selected.Name)
	return m, nil
}

func (m TheApplication) confirmDeleteCashflow() (tea.Model, tea.Cmd) {
	index := -1
	for i := range m.cashflows {
		if m.cashflows[i].ID == m.deleteConfirmID {
			index = i
			break
		}
	}
	if index < 0 {
		m = m.clearDeleteConfirmation("entry not found")
		return m, nil
	}

	selected := m.cashflows[index]
	if err := m.db.Delete(&cashflow.CashflowEntry{}, selected.ID).Error; err != nil {
		m = m.clearDeleteConfirmation("delete failed: " + err.Error())
		return m, nil
	}

	m.cashflows = append(m.cashflows[:index], m.cashflows[index+1:]...)
	items := m.filteredCashflowsForMonth()
	if len(items) == 0 {
		m.cashflowCursor = 0
	} else if m.cashflowCursor >= len(items) {
		m.cashflowCursor = len(items) - 1
	}

	m = m.clearDeleteConfirmation("deleted cashflow entry")
	return m, nil
}

func (m TheApplication) confirmDeleteDebt() (tea.Model, tea.Cmd) {
	index := m.findDebtIndex(m.deleteConfirmID)
	if index < 0 {
		m = m.clearDeleteConfirmation("debt not found")
		return m, nil
	}

	selected := m.debts[index]
	if err := m.db.Where("debt_id = ?", selected.ID).Delete(&debt.DebtLog{}).Error; err != nil {
		m = m.clearDeleteConfirmation("delete failed: " + err.Error())
		return m, nil
	}
	if err := m.db.Delete(&debt.Debt{}, selected.ID).Error; err != nil {
		m = m.clearDeleteConfirmation("delete failed: " + err.Error())
		return m, nil
	}

	m.debts = append(m.debts[:index], m.debts[index+1:]...)
	filteredAfter := m.filteredDebts()
	if len(filteredAfter) == 0 {
		m.debtCursor = 0
	} else if m.debtCursor >= len(filteredAfter) {
		m.debtCursor = len(filteredAfter) - 1
	}

	m = m.clearDeleteConfirmation("deleted debt " + selected.Peer)
	return m, nil
}

func (m TheApplication) confirmDeleteGoal() (tea.Model, tea.Cmd) {
	index := m.findGoalIndex(m.deleteConfirmID)
	if index < 0 {
		m = m.clearDeleteConfirmation("goal not found")
		return m, nil
	}

	selected := m.goals[index]
	if err := m.db.Where("goal_id = ?", selected.ID).Delete(&goal.GoalLog{}).Error; err != nil {
		m = m.clearDeleteConfirmation("delete failed: " + err.Error())
		return m, nil
	}
	if err := m.db.Delete(&goal.Goal{}, selected.ID).Error; err != nil {
		m = m.clearDeleteConfirmation("delete failed: " + err.Error())
		return m, nil
	}

	m.goals = append(m.goals[:index], m.goals[index+1:]...)
	filteredAfter := m.filteredGoals()
	if len(filteredAfter) == 0 {
		m.goalCursor = 0
	} else if m.goalCursor >= len(filteredAfter) {
		m.goalCursor = len(filteredAfter) - 1
	}

	m = m.clearDeleteConfirmation("deleted goal " + selected.Name)
	return m, nil
}

func (m TheApplication) confirmDeleteTax() (tea.Model, tea.Cmd) {
	index := m.findTaxIndex(m.deleteConfirmID)
	if index < 0 {
		m = m.clearDeleteConfirmation("tax not found")
		return m, nil
	}

	selected := m.taxes[index]
	if err := m.db.Where("tax_id = ?", selected.ID).Delete(&tax.TaxLog{}).Error; err != nil {
		m = m.clearDeleteConfirmation("delete failed: " + err.Error())
		return m, nil
	}
	if err := m.db.Delete(&tax.Tax{}, selected.ID).Error; err != nil {
		m = m.clearDeleteConfirmation("delete failed: " + err.Error())
		return m, nil
	}

	m.taxes = append(m.taxes[:index], m.taxes[index+1:]...)
	filteredAfter := m.filteredTaxes()
	if len(filteredAfter) == 0 {
		m.taxCursor = 0
	} else if m.taxCursor >= len(filteredAfter) {
		m.taxCursor = len(filteredAfter) - 1
	}

	m = m.clearDeleteConfirmation("deleted tax " + selected.TaxCountry + " / " + selected.TaxTypeName)
	return m, nil
}

func (m TheApplication) confirmDeleteInvoice() (tea.Model, tea.Cmd) {
	index := m.findInvoiceIndex(m.deleteConfirmID)
	if index < 0 {
		m = m.clearDeleteConfirmation("invoice not found")
		return m, nil
	}

	selected := m.invoices[index]

	if err := m.db.Delete(&invoice.Invoice{}, selected.ID).Error; err != nil {
		m = m.clearDeleteConfirmation("delete failed: " + err.Error())
		return m, nil
	}

	m.invoices = append(m.invoices[:index], m.invoices[index+1:]...)
	filteredAfter := m.filteredInvoices()

	if len(filteredAfter) == 0 {
		m.invoiceCursor = 0
	} else if m.invoiceCursor >= len(filteredAfter) {
		m.invoiceCursor = len(filteredAfter) - 1
	}

	m = m.clearDeleteConfirmation("deleted invoice " + selected.Title)

	return m, nil
}

func (m TheApplication) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.settingsDeleteConfirm {
		switch msg.String() {
		case "esc":
			m.settingsDeleteConfirm = false
			m.status = "delete cancelled"

			return m, nil
		case "left", "h", "shift+tab":
			m.settingsDeleteChoice = 0

			return m, nil
		case "right", "l", "tab":
			m.settingsDeleteChoice = 1

			return m, nil
		case "enter":
			if m.settingsDeleteChoice == 1 {
				m.settingsDeleteConfirm = false
				m.status = "delete cancelled"

				return m, nil
			}

			if m.settingsDeleteTargetType == "currency" {
				if err := m.db.Delete(&settings.SettingCurrency{}, m.settingsDeleteTargetID).Error; err != nil {
					m.status = "currency delete failed: " + err.Error()

					return m, nil
				}
			}

			if m.settingsDeleteTargetType == "payment_method" {
				if err := m.db.Delete(&settings.SettingPaymentMethod{}, m.settingsDeleteTargetID).Error; err != nil {
					m.status = "payment method delete failed: " + err.Error()

					return m, nil
				}
				if err := storage.EnsurePaymentMethodDefaults(m.db); err != nil {
					m.status = "payment method default repair failed: " + err.Error()

					return m, nil
				}
			}

			if m.settingsDeleteTargetType == "tax_type" {
				if err := m.db.Delete(&settings.SettingTaxType{}, m.settingsDeleteTargetID).Error; err != nil {
					m.status = "tax type delete failed: " + err.Error()

					return m, nil
				}
			}

			if m.settingsDeleteTargetType == "income_category" {
				if err := m.db.Delete(&settings.SettingIncomeCategory{}, m.settingsDeleteTargetID).Error; err != nil {
					m.status = "income category delete failed: " + err.Error()

					return m, nil
				}
			}

			if m.settingsDeleteTargetType == "expense_category" {
				if err := m.db.Delete(&settings.SettingExpenseCategory{}, m.settingsDeleteTargetID).Error; err != nil {
					m.status = "expense category delete failed: " + err.Error()

					return m, nil
				}
			}

			updated, err := storage.LoadAppSettings(m.db)
			if err != nil {
				m.status = "settings reload failed: " + err.Error()

				return m, nil
			}

			m.settings = updated
			m.accounts = sortAccounts(m.accounts, m.settings, m.accountSortField)
			deletedName := m.settingsDeleteTargetName
			deletedType := m.settingsDeleteTargetType
			m.settingsDeleteConfirm = false
			m.settingsDeleteTargetType = ""
			m.settingsDeleteTargetID = 0
			m.settingsDeleteTargetName = ""
			m.settingsDeleteChoice = 1
			m.settingsCursor = 0
			m.status = "deleted " + deletedType + " " + deletedName

			return m, nil
		}

		return m, nil
	}

	if m.settingsEditMode == settingsEditBaseCurrency {
		switch msg.String() {
		case "esc":
			m.settingsEditMode = settingsEditNone
			m.settingsEditInput.Blur()
			m.screen = screenMenu
			m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

			return m, nil
		case "enter":
			value := strings.TrimSpace(m.settingsEditInput.Value())
			if value == "" {
				m.status = "base currency cannot be empty"

				return m, nil
			}

			record := settings.SettingRecord{SettingID: "base_currency", SettingValue: value}
			if err := m.db.Save(&record).Error; err != nil {
				m.status = "settings save failed: " + err.Error()

				return m, nil
			}

			updated, err := storage.LoadAppSettings(m.db)
			if err != nil {
				m.status = "settings reload failed: " + err.Error()

				return m, nil
			}

			m.settings = updated
			m.accounts = sortAccounts(m.accounts, m.settings, m.accountSortField)
			m.settingsEditMode = settingsEditNone
			m.settingsEditInput.Blur()
			m.status = "saved setting base_currency"

			return m, nil
		}

		var cmd tea.Cmd

		m.settingsEditInput, cmd = m.settingsEditInput.Update(msg)

		return m, cmd
	}

	if m.settingsEditMode == settingsEditCurrency {
		switch msg.String() {
		case "esc":
			m.settingsEditMode = settingsEditNone
			m.settingsCurrencyNameInput.Blur()
			m.settingsCurrencyRateInput.Blur()
			m.screen = screenMenu
			m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

			return m, nil
		case "tab", "down":
			m.settingsCurrencyField = 1
			m = m.focusCurrencyFormField()

			return m, nil
		case "shift+tab", "up":
			m.settingsCurrencyField = 0
			m = m.focusCurrencyFormField()

			return m, nil
		case "enter":
			if m.settingsCurrencyField == 0 {
				m.settingsCurrencyField = 1
				m = m.focusCurrencyFormField()

				return m, nil
			}

			name := strings.TrimSpace(m.settingsCurrencyNameInput.Value())
			rateRaw := strings.TrimSpace(m.settingsCurrencyRateInput.Value())
			if name == "" {
				m.status = "currency name is required"

				return m, nil
			}

			rate, err := strconv.ParseFloat(rateRaw, 64)
			if err != nil {
				m.status = "rate must be a number"

				return m, nil
			}
			if rate <= 0 {
				m.status = "rate must be greater than zero"

				return m, nil
			}

			now := time.Now()
			record := settings.SettingCurrency{
				ID:            m.settingsCurrencyEditingID,
				CurrencyName:  name,
				RateToBase:    rate,
				LastUpdatedAt: now,
			}
			if record.ID == 0 {
				record.CreatedAt = now
			}

			if err := m.db.Save(&record).Error; err != nil {
				m.status = "currency save failed: " + err.Error()

				return m, nil
			}

			updated, err := storage.LoadAppSettings(m.db)
			if err != nil {
				m.status = "settings reload failed: " + err.Error()

				return m, nil
			}

			m.settings = updated
			m.accounts = sortAccounts(m.accounts, m.settings, m.accountSortField)
			m.settingsEditMode = settingsEditNone
			m.settingsCurrencyNameInput.Blur()
			m.settingsCurrencyRateInput.Blur()
			m.settingsCurrencyEditingID = 0
			m.settingsCurrencyField = 0
			m.settingsCursor = settingsCursorByName(m.settings.Currencies, name)
			m.status = "saved currency " + name

			return m, nil
		}

		var cmd tea.Cmd

		if m.settingsCurrencyField == 0 {
			m.settingsCurrencyNameInput, cmd = m.settingsCurrencyNameInput.Update(msg)

			return m, cmd
		}

		m.settingsCurrencyRateInput, cmd = m.settingsCurrencyRateInput.Update(msg)

		return m, cmd
	}

	if m.settingsEditMode == settingsEditPaymentMethod {
		switch msg.String() {
		case "esc":
			m.settingsEditMode = settingsEditNone
			m.settingsPaymentMethodNameInput.Blur()
			m.screen = screenMenu
			m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

			return m, nil
		case "tab", "down":
			if m.settingsPaymentMethodField < 2 {
				m.settingsPaymentMethodField++
			}
			m = m.focusPaymentMethodFormField()

			return m, nil
		case "shift+tab", "up":
			if m.settingsPaymentMethodField > 0 {
				m.settingsPaymentMethodField--
			}
			m = m.focusPaymentMethodFormField()

			return m, nil
		case "left":
			if m.settingsPaymentMethodField == 1 && m.settingsPaymentMethodTypeIndex > 0 {
				m.settingsPaymentMethodTypeIndex--
			}

			return m, nil
		case "right":
			if m.settingsPaymentMethodField == 1 && m.settingsPaymentMethodTypeIndex < len(m.settingsPaymentMethodTypeOptions)-1 {
				m.settingsPaymentMethodTypeIndex++
			}

			return m, nil
		case " ":
			if m.settingsPaymentMethodField == 2 {
				m.settingsPaymentMethodIsDefault = !m.settingsPaymentMethodIsDefault

				return m, nil
			}
		case "enter":
			if m.settingsPaymentMethodField < 2 {
				m.settingsPaymentMethodField++
				m = m.focusPaymentMethodFormField()

				return m, nil
			}

			name := strings.TrimSpace(m.settingsPaymentMethodNameInput.Value())
			if name == "" {
				m.status = "payment method name is required"

				return m, nil
			}

			methodType := selectedPaymentMethodType(m.settingsPaymentMethodTypeOptions, m.settingsPaymentMethodTypeIndex)
			isDefault := m.settingsPaymentMethodIsDefault
			if len(m.settings.PaymentMethods) == 0 && m.settingsPaymentMethodEditingID == 0 {
				isDefault = true
			}

			now := time.Now()

			record := settings.SettingPaymentMethod{
				ID:                m.settingsPaymentMethodEditingID,
				PaymentMethodName: name,
				PaymentMethodType: methodType,
				IsDefault:         isDefault,
				LastUpdatedAt:     now,
			}
			if record.ID == 0 {
				record.CreatedAt = now
			}

			if record.IsDefault {
				if err := m.db.Model(&settings.SettingPaymentMethod{}).Where("id <> ?", record.ID).Update("is_default", false).Error; err != nil {
					m.status = "payment method save failed: " + err.Error()

					return m, nil
				}
			}

			if err := m.db.Save(&record).Error; err != nil {
				m.status = "payment method save failed: " + err.Error()

				return m, nil
			}

			var defaultCount int64

			if err := m.db.Model(&settings.SettingPaymentMethod{}).Where("is_default = ?", true).Count(&defaultCount).Error; err == nil && defaultCount == 0 {
				_ = m.db.Model(&settings.SettingPaymentMethod{}).Where("id = ?", record.ID).Update("is_default", true).Error
			}

			updated, err := storage.LoadAppSettings(m.db)
			if err != nil {
				m.status = "settings reload failed: " + err.Error()

				return m, nil
			}

			m.settings = updated
			m.settingsEditMode = settingsEditNone
			m.settingsPaymentMethodNameInput.Blur()
			m.settingsPaymentMethodEditingID = 0
			m.settingsPaymentMethodField = 0
			m.settingsCursor = settingsCursorByPaymentMethodName(m.settings, name)
			m.status = "saved payment method " + name

			return m, nil
		}

		var cmd tea.Cmd

		if m.settingsPaymentMethodField == 0 {
			m.settingsPaymentMethodNameInput, cmd = m.settingsPaymentMethodNameInput.Update(msg)

			return m, cmd
		}

		return m, nil
	}

	if m.settingsEditMode == settingsEditTaxType {
		switch msg.String() {
		case "esc":
			m.settingsEditMode = settingsEditNone
			m.settingsTaxTypeCountryInput.Blur()
			m.settingsTaxTypeNameInput.Blur()
			m.settingsTaxTypeDescriptionInput.Blur()
			m.settingsTaxTypeURLInput.Blur()
			m.settingsTaxTypeEditingID = 0
			m.settingsTaxTypeField = 0
			m.status = "tax type edit cancelled"

			return m, nil
		case "down", "tab":
			if m.settingsTaxTypeField < 3 {
				m.settingsTaxTypeField++
			}
			m = m.focusTaxTypeFormField()

			return m, nil
		case "up", "shift+tab":
			if m.settingsTaxTypeField > 0 {
				m.settingsTaxTypeField--
			}
			m = m.focusTaxTypeFormField()

			return m, nil
		case "enter":
			if m.settingsTaxTypeField < 3 {
				m.settingsTaxTypeField++
				m = m.focusTaxTypeFormField()

				return m, nil
			}

			country := strings.TrimSpace(m.settingsTaxTypeCountryInput.Value())
			name := strings.TrimSpace(m.settingsTaxTypeNameInput.Value())
			description := strings.TrimSpace(m.settingsTaxTypeDescriptionInput.Value())
			url := strings.TrimSpace(m.settingsTaxTypeURLInput.Value())
			if country == "" {
				m.status = "country is required"

				return m, nil
			}

			if name == "" {
				m.status = "tax type name is required"

				return m, nil
			}

			now := time.Now()

			record := settings.SettingTaxType{
				ID:            m.settingsTaxTypeEditingID,
				Country:       country,
				TaxTypeName:   name,
				Description:   description,
				URL:           url,
				LastUpdatedAt: now,
			}
			if record.ID == 0 {
				record.CreatedAt = now
			}

			if err := m.db.Save(&record).Error; err != nil {
				m.status = "tax type save failed: " + err.Error()

				return m, nil
			}

			updated, err := storage.LoadAppSettings(m.db)
			if err != nil {
				m.status = "settings reload failed: " + err.Error()

				return m, nil
			}

			m.settings = updated
			m.settingsEditMode = settingsEditNone
			m.settingsTaxTypeCountryInput.Blur()
			m.settingsTaxTypeNameInput.Blur()
			m.settingsTaxTypeDescriptionInput.Blur()
			m.settingsTaxTypeURLInput.Blur()
			m.settingsTaxTypeEditingID = 0
			m.settingsTaxTypeField = 0
			m.settingsCursor = settingsCursorByTaxTypeID(m.settings, record.ID, country, name)
			m.status = "saved tax type " + country + " / " + name

			return m, nil
		}

		var cmd tea.Cmd
		switch m.settingsTaxTypeField {
		case 0:
			m.settingsTaxTypeCountryInput, cmd = m.settingsTaxTypeCountryInput.Update(msg)
		case 1:
			m.settingsTaxTypeNameInput, cmd = m.settingsTaxTypeNameInput.Update(msg)
		case 2:
			m.settingsTaxTypeDescriptionInput, cmd = m.settingsTaxTypeDescriptionInput.Update(msg)
		case 3:
			m.settingsTaxTypeURLInput, cmd = m.settingsTaxTypeURLInput.Update(msg)
		}
		return m, cmd
	}

	if m.settingsEditMode == settingsEditIncomeCategory {
		switch msg.String() {
		case "esc":
			m.settingsEditMode = settingsEditNone
			m.settingsIncomeCategoryNameInput.Blur()
			m.settingsIncomeCategoryEditingID = 0
			m.status = "income category edit cancelled"
			return m, nil
		case "enter":
			name := strings.TrimSpace(m.settingsIncomeCategoryNameInput.Value())
			if name == "" {
				m.status = "income category name is required"
				return m, nil
			}
			now := time.Now()
			record := settings.SettingIncomeCategory{ID: m.settingsIncomeCategoryEditingID, CategoryName: name, LastUpdatedAt: now}
			if record.ID == 0 {
				record.CreatedAt = now
			}
			if err := m.db.Save(&record).Error; err != nil {
				m.status = "income category save failed: " + err.Error()
				return m, nil
			}
			updated, err := storage.LoadAppSettings(m.db)
			if err != nil {
				m.status = "settings reload failed: " + err.Error()
				return m, nil
			}
			m.settings = updated
			m.settingsEditMode = settingsEditNone
			m.settingsIncomeCategoryNameInput.Blur()
			m.settingsIncomeCategoryEditingID = 0
			m.settingsCursor = settingsCursorByIncomeCategoryName(m.settings, name)
			m.status = "saved income category " + name
			return m, nil
		}

		var cmd tea.Cmd
		m.settingsIncomeCategoryNameInput, cmd = m.settingsIncomeCategoryNameInput.Update(msg)
		return m, cmd
	}

	if m.settingsEditMode == settingsEditExpenseCategory {
		switch msg.String() {
		case "esc":
			m.settingsEditMode = settingsEditNone
			m.settingsExpenseCategoryNameInput.Blur()
			m.settingsExpenseCategoryEditingID = 0
			m.status = "expense category edit cancelled"
			return m, nil
		case "enter":
			name := strings.TrimSpace(m.settingsExpenseCategoryNameInput.Value())
			if name == "" {
				m.status = "expense category name is required"
				return m, nil
			}
			now := time.Now()
			record := settings.SettingExpenseCategory{ID: m.settingsExpenseCategoryEditingID, CategoryName: name, LastUpdatedAt: now}
			if record.ID == 0 {
				record.CreatedAt = now
			}
			if err := m.db.Save(&record).Error; err != nil {
				m.status = "expense category save failed: " + err.Error()
				return m, nil
			}
			updated, err := storage.LoadAppSettings(m.db)
			if err != nil {
				m.status = "settings reload failed: " + err.Error()
				return m, nil
			}
			m.settings = updated
			m.settingsEditMode = settingsEditNone
			m.settingsExpenseCategoryNameInput.Blur()
			m.settingsExpenseCategoryEditingID = 0
			m.settingsCursor = settingsCursorByExpenseCategoryName(m.settings, name)
			m.status = "saved expense category " + name
			return m, nil
		}

		var cmd tea.Cmd
		m.settingsExpenseCategoryNameInput, cmd = m.settingsExpenseCategoryNameInput.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		return m, nil
	case "backspace", "delete":
		paymentStart := settingsPaymentMethodStartCursor(m.settings)
		paymentAdd := settingsPaymentMethodAddCursor(m.settings)
		taxStart := settingsTaxTypeStartCursor(m.settings)
		taxAdd := settingsTaxTypeAddCursor(m.settings)
		incomeStart := settingsIncomeCategoryStartCursor(m.settings)
		incomeAdd := settingsIncomeCategoryAddCursor(m.settings)
		expenseStart := settingsExpenseCategoryStartCursor(m.settings)
		expenseAdd := settingsExpenseCategoryAddCursor(m.settings)
		if m.settingsCursor > 0 && m.settingsCursor <= len(m.settings.Currencies) {
			selected := m.settings.Currencies[m.settingsCursor-1]
			m.settingsDeleteConfirm = true
			m.settingsDeleteTargetType = "currency"
			m.settingsDeleteTargetID = selected.ID
			m.settingsDeleteTargetName = selected.CurrencyName
			m.settingsDeleteChoice = 1
			m.status = "confirm currency delete"
			return m, nil
		}
		if m.settingsCursor >= paymentStart && m.settingsCursor < paymentAdd {
			selected := m.settings.PaymentMethods[m.settingsCursor-paymentStart]
			m.settingsDeleteConfirm = true
			m.settingsDeleteTargetType = "payment_method"
			m.settingsDeleteTargetID = selected.ID
			m.settingsDeleteTargetName = selected.PaymentMethodName
			m.settingsDeleteChoice = 1
			m.status = "confirm payment method delete"
			return m, nil
		}
		if m.settingsCursor >= taxStart && m.settingsCursor < taxAdd {
			selected := m.settings.TaxTypes[m.settingsCursor-taxStart]
			m.settingsDeleteConfirm = true
			m.settingsDeleteTargetType = "tax_type"
			m.settingsDeleteTargetID = selected.ID
			m.settingsDeleteTargetName = strings.TrimSpace(selected.Country + " / " + selected.TaxTypeName)
			m.settingsDeleteChoice = 1
			m.status = "confirm tax type delete"
			return m, nil
		}
		if m.settingsCursor >= incomeStart && m.settingsCursor < incomeAdd {
			selected := m.settings.IncomeCategories[m.settingsCursor-incomeStart]
			m.settingsDeleteConfirm = true
			m.settingsDeleteTargetType = "income_category"
			m.settingsDeleteTargetID = selected.ID
			m.settingsDeleteTargetName = selected.CategoryName
			m.settingsDeleteChoice = 1
			m.status = "confirm income category delete"
			return m, nil
		}
		if m.settingsCursor >= expenseStart && m.settingsCursor < expenseAdd {
			selected := m.settings.ExpenseCategories[m.settingsCursor-expenseStart]
			m.settingsDeleteConfirm = true
			m.settingsDeleteTargetType = "expense_category"
			m.settingsDeleteTargetID = selected.ID
			m.settingsDeleteTargetName = selected.CategoryName
			m.settingsDeleteChoice = 1
			m.status = "confirm expense category delete"
			return m, nil
		}
		return m, nil
	case "up":
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
		return m, nil
	case "down":
		maxCursor := settingsExpenseCategoryAddCursor(m.settings)
		if m.settingsCursor < maxCursor {
			m.settingsCursor++
		}
		return m, nil
	case "enter":
		paymentStart := settingsPaymentMethodStartCursor(m.settings)
		paymentAdd := settingsPaymentMethodAddCursor(m.settings)
		taxStart := settingsTaxTypeStartCursor(m.settings)
		taxAdd := settingsTaxTypeAddCursor(m.settings)
		incomeStart := settingsIncomeCategoryStartCursor(m.settings)
		incomeAdd := settingsIncomeCategoryAddCursor(m.settings)
		expenseStart := settingsExpenseCategoryStartCursor(m.settings)
		expenseAdd := settingsExpenseCategoryAddCursor(m.settings)

		if m.settingsCursor == 0 {
			m.settingsEditMode = settingsEditBaseCurrency
			m.settingsEditInput.SetValue(m.settings.BaseCurrency)
			m.settingsEditInput.Focus()
			m.status = "editing setting base_currency"
			return m, nil
		}

		if m.settingsCursor == len(m.settings.Currencies)+1 {
			m.settingsEditMode = settingsEditCurrency
			m.settingsCurrencyEditingID = 0
			m.settingsCurrencyField = 0
			m.settingsCurrencyNameInput.SetValue("")
			m.settingsCurrencyRateInput.SetValue("")
			m = m.focusCurrencyFormField()
			m.status = "adding new currency"
			return m, nil
		}

		if m.settingsCursor == paymentAdd {
			m.settingsEditMode = settingsEditPaymentMethod
			m.settingsPaymentMethodEditingID = 0
			m.settingsPaymentMethodField = 0
			m.settingsPaymentMethodTypeIndex = 0
			m.settingsPaymentMethodIsDefault = len(m.settings.PaymentMethods) == 0
			m.settingsPaymentMethodNameInput.SetValue("")
			m = m.focusPaymentMethodFormField()
			m.status = "adding new payment method"
			return m, nil
		}

		if m.settingsCursor == taxAdd {
			m.settingsEditMode = settingsEditTaxType
			m.settingsTaxTypeEditingID = 0
			m.settingsTaxTypeField = 0
			m.settingsTaxTypeCountryInput.SetValue("")
			m.settingsTaxTypeNameInput.SetValue("")
			m.settingsTaxTypeDescriptionInput.SetValue("")
			m.settingsTaxTypeURLInput.SetValue("")
			m = m.focusTaxTypeFormField()
			m.status = "adding new tax type"
			return m, nil
		}

		if m.settingsCursor == incomeAdd {
			m.settingsEditMode = settingsEditIncomeCategory
			m.settingsIncomeCategoryEditingID = 0
			m.settingsIncomeCategoryNameInput.SetValue("")
			m.settingsIncomeCategoryNameInput.Focus()
			m.status = "adding new income category"
			return m, nil
		}

		if m.settingsCursor == expenseAdd {
			m.settingsEditMode = settingsEditExpenseCategory
			m.settingsExpenseCategoryEditingID = 0
			m.settingsExpenseCategoryNameInput.SetValue("")
			m.settingsExpenseCategoryNameInput.Focus()
			m.status = "adding new expense category"
			return m, nil
		}

		index := m.settingsCursor - 1
		if index >= 0 && index < len(m.settings.Currencies) {
			selected := m.settings.Currencies[index]
			m.settingsEditMode = settingsEditCurrency
			m.settingsCurrencyEditingID = selected.ID
			m.settingsCurrencyField = 1
			m.settingsCurrencyNameInput.SetValue(selected.CurrencyName)
			m.settingsCurrencyRateInput.SetValue(formatRate(selected.RateToBase))
			m = m.focusCurrencyFormField()
			m.status = "editing currency " + selected.CurrencyName
			return m, nil
		}

		if m.settingsCursor >= paymentStart && m.settingsCursor < paymentAdd {
			methodIndex := m.settingsCursor - paymentStart
			selected := m.settings.PaymentMethods[methodIndex]
			m.settingsEditMode = settingsEditPaymentMethod
			m.settingsPaymentMethodEditingID = selected.ID
			m.settingsPaymentMethodField = 0
			m.settingsPaymentMethodNameInput.SetValue(selected.PaymentMethodName)
			m.settingsPaymentMethodTypeIndex = paymentMethodTypeIndex(m.settingsPaymentMethodTypeOptions, selected.PaymentMethodType)
			m.settingsPaymentMethodIsDefault = selected.IsDefault
			m = m.focusPaymentMethodFormField()
			m.status = "editing payment method " + selected.PaymentMethodName
			return m, nil
		}

		if m.settingsCursor >= taxStart && m.settingsCursor < taxAdd {
			taxTypeIndex := m.settingsCursor - taxStart
			selected := m.settings.TaxTypes[taxTypeIndex]
			m.settingsEditMode = settingsEditTaxType
			m.settingsTaxTypeEditingID = selected.ID
			m.settingsTaxTypeField = 0
			m.settingsTaxTypeCountryInput.SetValue(selected.Country)
			m.settingsTaxTypeNameInput.SetValue(selected.TaxTypeName)
			m.settingsTaxTypeDescriptionInput.SetValue(selected.Description)
			m.settingsTaxTypeURLInput.SetValue(selected.URL)
			m = m.focusTaxTypeFormField()
			m.status = "editing tax type " + selected.Country + " / " + selected.TaxTypeName

			return m, nil
		}

		if m.settingsCursor >= incomeStart && m.settingsCursor < incomeAdd {
			idx := m.settingsCursor - incomeStart
			selected := m.settings.IncomeCategories[idx]
			m.settingsEditMode = settingsEditIncomeCategory
			m.settingsIncomeCategoryEditingID = selected.ID
			m.settingsIncomeCategoryNameInput.SetValue(selected.CategoryName)
			m.settingsIncomeCategoryNameInput.Focus()
			m.status = "editing income category " + selected.CategoryName

			return m, nil
		}

		if m.settingsCursor >= expenseStart && m.settingsCursor < expenseAdd {
			idx := m.settingsCursor - expenseStart
			selected := m.settings.ExpenseCategories[idx]
			m.settingsEditMode = settingsEditExpenseCategory
			m.settingsExpenseCategoryEditingID = selected.ID
			m.settingsExpenseCategoryNameInput.SetValue(selected.CategoryName)
			m.settingsExpenseCategoryNameInput.Focus()
			m.status = "editing expense category " + selected.CategoryName

			return m, nil
		}

		return m, nil
	default:
		return m, nil
	}
}

func (m TheApplication) updateDataExport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

		return m, nil
	case "tab", "down":
		m.exportForm = m.exportForm.Next()

		return m, nil
	case "shift+tab", "up":
		m.exportForm = m.exportForm.Prev()

		return m, nil
	case "left", "h":
		if m.exportForm.Active == export.ExportFieldDataset && m.exportForm.DatasetIndex > 0 {
			m.exportForm.DatasetIndex--
		}

		if m.exportForm.Active == export.ExportFieldFormat && m.exportForm.FormatIndex > 0 {
			m.exportForm.FormatIndex--
		}

		return m, nil
	case "right", "l":
		if m.exportForm.Active == export.ExportFieldDataset && m.exportForm.DatasetIndex < len(m.exportForm.DatasetOptions)-1 {
			m.exportForm.DatasetIndex++
		}

		if m.exportForm.Active == export.ExportFieldFormat && m.exportForm.FormatIndex < len(m.exportForm.FormatOptions)-1 {
			m.exportForm.FormatIndex++
		}

		return m, nil
	case "enter":
		if m.exportForm.Active < export.ExportFieldRun {
			m.exportForm = m.exportForm.Next()

			return m, nil
		}

		result, err := storage.ExportData(m.db, storage.ExportRequest{
			Dataset: selectedExportDataset(m.exportForm),
			Format:  selectedExportFormat(m.exportForm),
			Path:    strings.TrimSpace(m.exportForm.PathInput.Value()),
		})
		if err != nil {
			m.status = "export failed: " + err.Error()

			return m, nil
		}

		m.status = fmt.Sprintf("exported %d file(s) to %s", result.FileCount, result.Path)

		return m, nil
	}

	var cmd tea.Cmd

	if m.exportForm.Active == export.ExportFieldPath {
		m.exportForm.PathInput, cmd = m.exportForm.PathInput.Update(msg)

		return m, cmd
	}

	return m, nil
}

func (m TheApplication) focusCurrencyFormField() TheApplication {
	m.settingsCurrencyNameInput.Blur()
	m.settingsCurrencyRateInput.Blur()

	if m.settingsCurrencyField == 0 {
		m.settingsCurrencyNameInput.Focus()
	} else {
		m.settingsCurrencyRateInput.Focus()
	}

	return m
}

func (m TheApplication) focusPaymentMethodFormField() TheApplication {
	m.settingsPaymentMethodNameInput.Blur()
	if m.settingsPaymentMethodField == 0 {
		m.settingsPaymentMethodNameInput.Focus()
	}

	return m
}

func (m TheApplication) focusTaxTypeFormField() TheApplication {
	m.settingsTaxTypeCountryInput.Blur()
	m.settingsTaxTypeNameInput.Blur()
	m.settingsTaxTypeDescriptionInput.Blur()
	m.settingsTaxTypeURLInput.Blur()

	switch m.settingsTaxTypeField {
	case 0:
		m.settingsTaxTypeCountryInput.Focus()
	case 1:
		m.settingsTaxTypeNameInput.Focus()
	case 2:
		m.settingsTaxTypeDescriptionInput.Focus()
	case 3:
		m.settingsTaxTypeURLInput.Focus()
	}

	return m
}

func (m TheApplication) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	groups := appMenuGroups()
	switch msg.String() {
	case "?":
		m.help.ShowAll = !m.help.ShowAll

		return m, nil
	case "left", "h", "shift+tab":
		if m.menuItem > 0 {
			m.menuItem--

			return m, nil
		}

		if targetGroup, ok := menuMoveAcrossColumns(m.menuGroup, -1); ok {
			m.menuGroup = targetGroup
			m.menuItem = 0
		}

		return m, nil
	case "right", "tab":
		if m.menuItem < len(groups[m.menuGroup].items)-1 {
			m.menuItem++

			return m, nil
		}

		if targetGroup, ok := menuMoveAcrossColumns(m.menuGroup, 1); ok {
			m.menuGroup = targetGroup
			m.menuItem = 0
		}

		return m, nil
	case "up":
		if targetGroup, ok := menuMoveWithinColumn(m.menuGroup, -1); ok {
			m.menuGroup = targetGroup
			m.menuItem = minInt(m.menuItem, len(groups[m.menuGroup].items)-1)
		}

		return m, nil
	case "down", "j":
		if targetGroup, ok := menuMoveWithinColumn(m.menuGroup, 1); ok {
			m.menuGroup = targetGroup
			m.menuItem = minInt(m.menuItem, len(groups[m.menuGroup].items)-1)
		}

		return m, nil
	case "enter":
		return m.activateMenuSelection()
	case "a":
		m.menuGroup = 1
		m.menuItem = 0

		return m.activateMenuSelection()
	case "e":
		m.menuGroup = 1
		m.menuItem = 1

		return m.activateMenuSelection()
	case "q":
		m.quitting = true

		return m, tea.Quit
	default:
		return m, nil
	}
}

func (m TheApplication) activateMenuSelection() (tea.Model, tea.Cmd) {
	if m.menuGroup == 0 && m.menuItem == 0 {
		m.screen = screenCashflowNew
		m.addCashflowForm = cashflow.NewAddCashflowForm(currencySelectionOptions(m.settings), expenseCategorySelectionOptions(m.settings), accountSelectionOptions(m.accounts), false)
		m.status = "new expense"

		return m, nil
	}

	if m.menuGroup == 0 && m.menuItem == 1 {
		m.screen = screenCashflowNew
		m.addCashflowForm = cashflow.NewAddCashflowForm(currencySelectionOptions(m.settings), incomeCategorySelectionOptions(m.settings), accountSelectionOptions(m.accounts), true)
		m.status = "new income"

		return m, nil
	}

	if m.menuGroup == 0 && m.menuItem == 2 {
		m.screen = screenCashflowHistory
		m.cashflowHistoryMonth = beginningOfMonth(time.Now())
		m.cashflowCursor = 0
		m.status = "income/expense history"

		return m, nil
	}

	if m.menuGroup == 0 && m.menuItem == 3 {
		m.screen = screenCashflowOverview
		m.cashflowOverviewPage = 0
		m.status = "income/expense monthly overview"

		return m, nil
	}

	if m.menuGroup == 1 && m.menuItem == 0 {
		m.screen = screenAddAccount
		m.addForm = account.NewAddAccountForm(currencySelectionOptions(m.settings))
		m.status = "add a new account"

		return m, nil
	}

	if m.menuGroup == 1 && m.menuItem == 1 {
		if len(m.accounts) == 0 {
			m.status = "no accounts to list"

			return m, nil
		}

		m.screen = screenAccountTable
		m.cursor = 0
		m.status = "accounts page: all accounts shown. use arrows to pick, enter to edit amount, delete/backspace to delete"

		return m, nil
	}

	if m.menuGroup == 2 && m.menuItem == 0 {
		m.screen = screenSubscriptionNew
		m.addSubscriptionForm = subscription.NewAddSubscriptionForm(currencySelectionOptions(m.settings), paymentMethodSelectionOptions(m.settings))
		m.status = "new subscription"

		return m, nil
	}

	if m.menuGroup == 2 && m.menuItem == 1 {
		m.screen = screenSubscriptionList
		m.subscriptionMode = subscriptionListActive
		m.subscriptionCursor = 0
		m.status = "active subscriptions"

		return m, nil
	}

	if m.menuGroup == 2 && m.menuItem == 2 {
		m.screen = screenSubscriptionList
		m.subscriptionMode = subscriptionListAll
		m.subscriptionCursor = 0
		m.status = "all subscriptions"

		return m, nil
	}

	if m.menuGroup == 3 && m.menuItem == 0 {
		m.screen = screenInvoiceNew
		m.addInvoiceForm = invoice.NewAddInvoiceForm(currencySelectionOptions(m.settings), accountSelectionOptions(m.accounts))
		m.status = "new invoice"

		return m, nil
	}

	if m.menuGroup == 3 && m.menuItem == 1 {
		m.screen = screenInvoiceList
		m.invoiceMode = invoiceListOutgoingUnpaid
		m.invoiceCursor = 0
		m.invoicePage = 0
		m.status = "outgoing unpaid invoices"

		return m, nil
	}

	if m.menuGroup == 3 && m.menuItem == 2 {
		m.screen = screenInvoiceList
		m.invoiceMode = invoiceListIncomingUnpaid
		m.invoiceCursor = 0
		m.invoicePage = 0
		m.status = "incoming unpaid invoices"

		return m, nil
	}

	if m.menuGroup == 3 && m.menuItem == 3 {
		m.screen = screenInvoiceList
		m.invoiceMode = invoiceListHistoryPaid
		m.invoiceCursor = 0
		m.invoicePage = 0
		m.status = "invoice history"

		return m, nil
	}

	if m.menuGroup == 4 && m.menuItem == 0 {
		m.screen = screenDebtNew
		m.addDebtForm = debt.NewAddDebtForm(currencySelectionOptions(m.settings))
		m.status = "new debt"

		return m, nil
	}

	if m.menuGroup == 4 && m.menuItem == 1 {
		m.screen = screenDebtList
		m.debtMode = debtListOutgoing
		m.debtCursor = 0
		m.status = "outgoing debts"

		return m, nil
	}

	if m.menuGroup == 4 && m.menuItem == 2 {
		m.screen = screenDebtList
		m.debtMode = debtListIncoming
		m.debtCursor = 0
		m.status = "incoming debts"

		return m, nil
	}

	if m.menuGroup == 4 && m.menuItem == 3 {
		m.screen = screenDebtList
		m.debtMode = debtListHistory
		m.debtCursor = 0
		m.status = "paid debt history"

		return m, nil
	}

	if m.menuGroup == 5 && m.menuItem == 0 {
		m.screen = screenGoalNew
		m.addGoalForm = goal.NewAddGoalForm(currencySelectionOptions(m.settings))
		m.status = "new goal"

		return m, nil
	}

	if m.menuGroup == 5 && m.menuItem == 1 {
		m.screen = screenGoalList
		m.goalMode = goalListActive
		m.goalCursor = 0
		m.status = "active goals"

		return m, nil
	}

	if m.menuGroup == 5 && m.menuItem == 2 {
		m.screen = screenGoalList
		m.goalMode = goalListHistory
		m.goalCursor = 0
		m.status = "goal history"

		return m, nil
	}

	if m.menuGroup == 6 && m.menuItem == 0 {
		m.screen = screenTaxNew
		m.addTaxForm = tax.NewAddTaxForm(m.settings.TaxTypes)
		m.status = "new tax"

		return m, nil
	}

	if m.menuGroup == 6 && m.menuItem == 1 {
		m.screen = screenTaxList
		m.taxMode = taxListUnpaid
		m.taxCursor = 0
		m.status = "unpaid taxes"

		return m, nil
	}

	if m.menuGroup == 6 && m.menuItem == 2 {
		m.screen = screenTaxList
		m.taxMode = taxListHistory
		m.taxCursor = 0
		m.status = "tax history"

		return m, nil
	}

	if m.menuGroup == 7 && m.menuItem == 0 {
		m.screen = screenSettings
		m.settingsCursor = 0
		m.settingsEditMode = settingsEditNone
		m.status = "settings"

		return m, nil
	}

	if m.menuGroup == 7 && m.menuItem == 1 {
		m.screen = screenDataExport
		m.exportForm = export.NewExportForm(defaultExportPath())
		m.status = "data export"

		return m, nil
	}

	groups := appMenuGroups()
	m.status = strings.ToLower(groups[m.menuGroup].title) + " / " + groups[m.menuGroup].items[m.menuItem] + " is a template action (coming next)"

	return m, nil
}

func (m TheApplication) updateAddAccount(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

		return m, nil
	case "up", "shift+tab":
		m.addForm = m.addForm.Prev()

		return m, nil
	case "down", "tab":
		m.addForm = m.addForm.Next()

		return m, nil
	case "left":
		if m.addForm.Active == 2 && m.addForm.CurrencyIndex > 0 {
			m.addForm.CurrencyIndex--
		}
		if m.addForm.Active == 4 {
			m.addForm.IgnoreInSummaries = false
		}

		return m, nil
	case "right":
		if m.addForm.Active == 2 && m.addForm.CurrencyIndex < len(m.addForm.CurrencyOptions)-1 {
			m.addForm.CurrencyIndex++
		}
		if m.addForm.Active == 4 {
			m.addForm.IgnoreInSummaries = true
		}

		return m, nil
	case " ":
		if m.addForm.Active == 4 {
			m.addForm.IgnoreInSummaries = !m.addForm.IgnoreInSummaries

			return m, nil
		}
	case "enter":
		if m.addForm.Active == len(m.addForm.Fields) {
			return m.saveAccountFromForm()
		}

		m.addForm = m.addForm.Next()

		return m, nil
	}

	if m.addForm.Active == 2 {
		return m, nil
	}

	if m.addForm.Active == 4 {
		return m, nil
	}

	var cmd tea.Cmd

	m.addForm.Fields[m.addForm.Active], cmd = m.addForm.Fields[m.addForm.Active].Update(msg)

	return m, cmd
}

func (m TheApplication) updateAccountTable(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if updatedModel, cmd, handled := m.handleDeleteConfirmation(msg); handled {
		return updatedModel, cmd
	}

	if len(m.accounts) == 0 {
		m.screen = screenMenu
		m.status = "no accounts available"

		return m, nil
	}

	if m.accountSortMenu {
		switch strings.ToLower(msg.String()) {
		case "esc", "s":
			m.accountSortMenu = false
			m.status = "account sort unchanged"

			return m, nil
		case "up", "k":
			if m.accountSortCursor > 0 {
				m.accountSortCursor--
			}

			return m, nil
		case "down", "j":
			if m.accountSortCursor < len(accountSortOptions())-1 {
				m.accountSortCursor++
			}

			return m, nil
		case "enter":
			selectedID := uint(0)
			if m.cursor >= 0 && m.cursor < len(m.accounts) {
				selectedID = m.accounts[m.cursor].ID
			}

			m.accountSortField = accountSortField(m.accountSortCursor)
			m.accounts = sortAccounts(m.accounts, m.settings, m.accountSortField)
			m.accountSortMenu = false
			if selectedID != 0 {
				m.cursor = findAccountIndex(m.accounts, selectedID)
			}
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.status = "sorting accounts by " + accountSortLabel(m.accountSortField)

			return m, nil
		default:
			return m, nil
		}
	}

	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

		return m, nil
	case "up":
		if m.cursor > 0 {
			m.cursor--
		}

		return m, nil
	case "down":
		if m.cursor < len(m.accounts)-1 {
			m.cursor++
		}

		return m, nil
	case "s", "S":
		m.accountSortMenu = true
		m.accountSortCursor = int(m.accountSortField)
		m.status = "choose account sorting"

		return m, nil
	case "enter":
		return m.beginEditAmount(), nil
	case "backspace", "delete":
		selected := m.accounts[m.cursor]
		m = m.beginDeleteConfirmation("account", selected.ID, selected.Name)

		return m, nil
	default:
		return m, nil
	}
}

func (m TheApplication) updateEditAmount(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenAccountTable
		m.status = "amount edit cancelled"

		return m, nil
	case "up", "shift+tab":
		if m.editAmountActiveField > 0 {
			m.editAmountActiveField--
		}

		return m.focusEditAmountField(), nil
	case "down", "tab":
		if m.editAmountActiveField < editAmountFieldCount-1 {
			m.editAmountActiveField++
		}

		return m.focusEditAmountField(), nil
	case " ":
		if m.editAmountActiveField == editAmountFieldUpdateLog {
			m.editAmountUpdateLog = !m.editAmountUpdateLog

			return m, nil
		}
		if m.editAmountActiveField == editAmountFieldIgnore {
			m.editAmountIgnoreInSummaries = !m.editAmountIgnoreInSummaries

			return m, nil
		}
	case "enter":
		switch m.editAmountActiveField {
		case editAmountFieldCurrent:
			return m.saveAmount()
		case editAmountFieldIgnore:
			m.editAmountIgnoreInSummaries = !m.editAmountIgnoreInSummaries

			return m, nil
		case editAmountFieldLogValue:
			return m.applyAccountLogValue()
		default:
			return m, nil
		}
	}

	var cmd tea.Cmd

	switch m.editAmountActiveField {
	case editAmountFieldCurrent:
		m.editInput, cmd = m.editInput.Update(msg)
	case editAmountFieldLogDate:
		m.editAmountLogDateInput, cmd = m.editAmountLogDateInput.Update(msg)
	case editAmountFieldLogValue:
		m.editAmountLogValueInput, cmd = m.editAmountLogValueInput.Update(msg)
	default:
		return m, nil
	}

	return m, cmd
}

func (m TheApplication) focusEditAmountField() TheApplication {
	m.editInput.Blur()
	m.editAmountLogDateInput.Blur()
	m.editAmountLogValueInput.Blur()

	switch m.editAmountActiveField {
	case editAmountFieldCurrent:
		m.editInput.Focus()
	case editAmountFieldLogDate:
		m.editAmountLogDateInput.Focus()
	case editAmountFieldLogValue:
		m.editAmountLogValueInput.Focus()
	}

	return m
}

func (m TheApplication) updateSubscriptionNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		return m, nil
	case "up", "shift+tab":
		m.addSubscriptionForm = m.addSubscriptionForm.Prev()
		return m, nil
	case "down", "tab":
		m.addSubscriptionForm = m.addSubscriptionForm.Next()
		return m, nil
	case "left":
		switch m.addSubscriptionForm.Active {
		case subscription.SubFieldCurrency:
			if m.addSubscriptionForm.CurrencyIndex > 0 {
				m.addSubscriptionForm.CurrencyIndex--
			}
		case subscription.SubFieldPaymentMethodChoice:
			if m.addSubscriptionForm.PaymentMethodIndex > 0 {
				m.addSubscriptionForm.PaymentMethodIndex--
			}
		case subscription.SubFieldPeriod:
			if m.addSubscriptionForm.PeriodIndex > 0 {
				m.addSubscriptionForm.PeriodIndex--
			}
		case subscription.SubFieldType:
			if m.addSubscriptionForm.TypeIndex > 0 {
				m.addSubscriptionForm.TypeIndex--
			}
		}
		return m, nil
	case "right":
		switch m.addSubscriptionForm.Active {
		case subscription.SubFieldCurrency:
			if m.addSubscriptionForm.CurrencyIndex < len(m.addSubscriptionForm.CurrencyOptions)-1 {
				m.addSubscriptionForm.CurrencyIndex++
			}
		case subscription.SubFieldPaymentMethodChoice:
			if m.addSubscriptionForm.PaymentMethodIndex < len(m.addSubscriptionForm.PaymentMethodOptions)-1 {
				m.addSubscriptionForm.PaymentMethodIndex++
			}
		case subscription.SubFieldPeriod:
			if m.addSubscriptionForm.PeriodIndex < len(m.addSubscriptionForm.PeriodOptions)-1 {
				m.addSubscriptionForm.PeriodIndex++
			}
		case subscription.SubFieldType:
			if m.addSubscriptionForm.TypeIndex < len(m.addSubscriptionForm.TypeOptions)-1 {
				m.addSubscriptionForm.TypeIndex++
			}
		}
		return m, nil
	case " ":
		if m.addSubscriptionForm.Active == subscription.SubFieldIsActive {
			m.addSubscriptionForm.IsActive = !m.addSubscriptionForm.IsActive
			return m, nil
		}
		// Let text inputs receive spaces when a text field is active.
	case "enter":
		if m.addSubscriptionForm.Active == subscription.SubFieldCount-1 {
			return m.saveSubscriptionFromForm()
		}
		m.addSubscriptionForm = m.addSubscriptionForm.Next()
		return m, nil
	}

	if inputIndex := m.addSubscriptionForm.InputIndexForField(m.addSubscriptionForm.Active); inputIndex >= 0 {
		var cmd tea.Cmd
		m.addSubscriptionForm.Inputs[inputIndex], cmd = m.addSubscriptionForm.Inputs[inputIndex].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m TheApplication) updateSubscriptionList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if updatedModel, cmd, handled := m.handleDeleteConfirmation(msg); handled {
		return updatedModel, cmd
	}

	filtered := m.filteredSubscriptions()
	if len(filtered) == 0 {
		if msg.String() == "esc" {
			m.screen = screenMenu
			m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		return m, nil
	case "up":
		if m.subscriptionCursor > 0 {
			m.subscriptionCursor--
		}
		return m, nil
	case "down":
		if m.subscriptionCursor < len(filtered)-1 {
			m.subscriptionCursor++
		}
		return m, nil
	case "enter":
		m = m.openSubscriptionEditor(filtered[m.subscriptionCursor]).(TheApplication)
		return m, nil
	case "backspace", "delete":
		selected := filtered[m.subscriptionCursor]
		m = m.beginDeleteConfirmation("subscription", selected.ID, selected.Name)
		return m, nil
	default:
		return m, nil
	}
}

func (m TheApplication) openSubscriptionEditor(selected subscription.Subscription) tea.Model {
	m.screen = screenSubscriptionEdit
	m.editingSubscriptionID = selected.ID
	m.editingSubscriptionMode = m.subscriptionMode
	m.editSubscriptionForm = subscription.NewEditSubscriptionForm()
	m.editSubscriptionForm.AmountInput.SetValue(formatAmount(selected.AmountCents))
	m.editSubscriptionForm.PaymentMethodInput.SetValue(selected.PaymentMethod)
	m.editSubscriptionForm.IsActive = selected.IsActive
	m.editSubscriptionForm.PeriodLabel = selected.Period
	m.editSubscriptionForm.TypeLabel = selected.Type
	m.editSubscriptionForm.CurrencyLabel = selected.Currency
	m.editSubscriptionForm.NameLabel = selected.Name
	m.editSubscriptionForm.PaymentDateYearly = selected.PaymentDateYearly
	if selected.PaymentDayMonthly != nil {
		m.editSubscriptionForm.PaymentDayMonthly = strconv.Itoa(*selected.PaymentDayMonthly)
	}
	m.editSubscriptionForm = m.editSubscriptionForm.FocusActive()
	m.status = "editing subscription " + selected.Name
	return m
}

func (m TheApplication) updateSubscriptionEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenSubscriptionList
		m.status = "edit cancelled"

		return m, nil
	case "up", "shift+tab":
		m.editSubscriptionForm = m.editSubscriptionForm.Prev()

		return m, nil
	case "down", "tab":
		m.editSubscriptionForm = m.editSubscriptionForm.Next()

		return m, nil
	case " ":
		if m.editSubscriptionForm.ActiveField == subscription.EditSubFieldIsActive {
			m.editSubscriptionForm.IsActive = !m.editSubscriptionForm.IsActive

			return m, nil
		}
	case "enter":
		if m.editSubscriptionForm.ActiveField == subscription.EditSubFieldCount-1 {
			return m.saveSubscriptionEdit()
		}

		m.editSubscriptionForm = m.editSubscriptionForm.Next()

		return m, nil
	}

	if m.editSubscriptionForm.ActiveField == subscription.EditSubFieldAmount {
		var cmd tea.Cmd

		m.editSubscriptionForm.AmountInput, cmd = m.editSubscriptionForm.AmountInput.Update(msg)

		return m, cmd
	}

	if m.editSubscriptionForm.ActiveField == subscription.EditSubFieldPaymentMethod {
		var cmd tea.Cmd

		m.editSubscriptionForm.PaymentMethodInput, cmd = m.editSubscriptionForm.PaymentMethodInput.Update(msg)

		return m, cmd
	}

	return m, nil
}

func (m TheApplication) Init() tea.Cmd {
	return textinput.Blink
}

func (m TheApplication) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	m.help, cmd = m.help.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width - 6
		if m.help.Width < 60 {
			m.help.Width = 60
		}

		return m, cmd
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

		switch m.screen {
		case screenMenu:
			return m.updateMenu(msg)
		case screenAddAccount:
			return m.updateAddAccount(msg)
		case screenAccountTable:
			return m.updateAccountTable(msg)
		case screenEditAmount:
			return m.updateEditAmount(msg)
		case screenSubscriptionNew:
			return m.updateSubscriptionNew(msg)
		case screenSubscriptionEdit:
			return m.updateSubscriptionEdit(msg)
		case screenSubscriptionList:
			return m.updateSubscriptionList(msg)
		case screenSettings:
			return m.updateSettings(msg)
		case screenDebtNew:
			return m.updateDebtNew(msg)
		case screenDebtList:
			return m.updateDebtList(msg)
		case screenDebtEdit:
			return m.updateDebtEdit(msg)
		case screenGoalNew:
			return m.updateGoalNew(msg)
		case screenGoalList:
			return m.updateGoalList(msg)
		case screenGoalEdit:
			return m.updateGoalEdit(msg)
		case screenTaxNew:
			return m.updateTaxNew(msg)
		case screenTaxList:
			return m.updateTaxList(msg)
		case screenTaxEdit:
			return m.updateTaxEdit(msg)
		case screenInvoiceNew:
			return m.updateInvoiceNew(msg)
		case screenInvoiceList:
			return m.updateInvoiceList(msg)
		case screenInvoiceEdit:
			return m.updateInvoiceEdit(msg)
		case screenCashflowNew:
			return m.updateCashflowNew(msg)
		case screenCashflowHistory:
			return m.updateCashflowHistory(msg)
		case screenCashflowOverview:
			return m.updateCashflowOverview(msg)
		case screenDataExport:
			return m.updateDataExport(msg)
		}
	}

	if m.screen == screenAddAccount {
		if m.addForm.Active >= len(m.addForm.Fields) {
			return m, nil
		}

		m.addForm.Fields[m.addForm.Active], cmd = m.addForm.Fields[m.addForm.Active].Update(msg)

		return m, cmd
	}

	if m.screen == screenEditAmount {
		m.editInput, cmd = m.editInput.Update(msg)

		return m, cmd
	}

	if m.screen == screenSubscriptionNew {
		if inputIndex := m.addSubscriptionForm.InputIndexForField(m.addSubscriptionForm.Active); inputIndex >= 0 {
			m.addSubscriptionForm.Inputs[inputIndex], cmd = m.addSubscriptionForm.Inputs[inputIndex].Update(msg)

			return m, cmd
		}
	}

	if m.screen == screenSubscriptionEdit {
		switch m.editSubscriptionForm.ActiveField {
		case subscription.EditSubFieldAmount:
			m.editSubscriptionForm.AmountInput, cmd = m.editSubscriptionForm.AmountInput.Update(msg)

			return m, cmd
		case subscription.EditSubFieldPaymentMethod:
			m.editSubscriptionForm.PaymentMethodInput, cmd = m.editSubscriptionForm.PaymentMethodInput.Update(msg)

			return m, cmd
		}
	}

	return m, cmd
}
