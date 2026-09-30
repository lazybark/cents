package app

import (
	"fmt"
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
	sqliteStorage "github.com/lazybark/cents/storage/sqlite"
)

type StorageWorker interface {
	LoadAppSettings() (settings.AppSettings, error)
	ExportData(request sqliteStorage.ExportRequest) (sqliteStorage.ExportResult, error)
	LoadAccountValueLogs(accountID uint) ([]account.AccountValueLog, error)
	UpsertAccountValueLog(accountID uint, day time.Time, valueCents int64) error
	CreateAccount(entry *account.Account) error
	UpdateAccountAmount(id uint, amountCents int64, ignoreInSummaries bool, updatedAt time.Time) error
	DeleteAccount(id uint) error
	CreateSubscription(entry *subscription.Subscription) error
	SaveSubscription(entry *subscription.Subscription) error
	DeleteSubscription(id uint) error
	CreateDebt(entry *debt.Debt) error
	SaveDebt(entry *debt.Debt) error
	CreateDebtLog(entry *debt.DebtLog) error
	LoadDebtLogs(debtID uint) ([]debt.DebtLog, error)
	DeleteDebt(id uint) error
	CreateGoal(entry *goal.Goal) error
	SaveGoal(entry *goal.Goal) error
	CreateGoalLog(entry *goal.GoalLog) error
	LoadGoalLogs(goalID uint) ([]goal.GoalLog, error)
	DeleteGoal(id uint) error
	CreateTax(entry *tax.Tax) error
	SaveTax(entry *tax.Tax) error
	CreateTaxLog(entry *tax.TaxLog) error
	LoadTaxLogs(taxID uint) ([]tax.TaxLog, error)
	DeleteTax(id uint) error
	CreateCashflow(entry *cashflow.CashflowEntry) error
	DeleteCashflow(id uint) error
	CreteInvoice(entry *invoice.Invoice) error
	SaveInvoice(entry *invoice.Invoice) error
	DeleteInvoice(id uint) error
	DeleteSetting(targetType string, id uint) error
	SaveSettingRecord(entry *settings.SettingRecord) error
	SaveSettingCurrency(entry *settings.SettingCurrency) error
	SaveSettingPaymentMethod(entry *settings.SettingPaymentMethod) error
	SaveSettingTaxType(entry *settings.SettingTaxType) error
	SaveSettingIncomeCategory(entry *settings.SettingIncomeCategory) error
	SaveSettingExpenseCategory(entry *settings.SettingExpenseCategory) error
}

type TheApplication struct {
	storage                          StorageWorker
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

func NewApp(storage StorageWorker, dbPath string, created bool, accounts []account.Account, subscriptions []subscription.Subscription, debts []debt.Debt, goals []goal.Goal, taxes []tax.Tax, invoices []invoice.Invoice, cashflows []cashflow.CashflowEntry, settings settings.AppSettings) TheApplication {
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
		storage:                          storage,
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

func (m TheApplication) confirmDeleteGoal() (tea.Model, tea.Cmd) {
	index := m.findGoalIndex(m.deleteConfirmID)
	if index < 0 {
		m = m.clearDeleteConfirmation("goal not found")

		return m, nil
	}

	selected := m.goals[index]
	if err := m.storage.DeleteGoal(selected.ID); err != nil {
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
