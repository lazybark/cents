package main

import (
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/debt"
	"github.com/lazybark/cents/flows/goal"
	"github.com/lazybark/cents/flows/invoice"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/subscription"
	"github.com/lazybark/cents/flows/tax"
	"gorm.io/gorm"
)

type settingsEditMode int

const (
	settingsEditNone settingsEditMode = iota
	settingsEditBaseCurrency
	settingsEditCurrency
	settingsEditPaymentMethod
	settingsEditTaxType
	settingsEditIncomeCategory
	settingsEditExpenseCategory
)

type screen int

const (
	screenMenu screen = iota
	screenAddAccount
	screenAccountTable
	screenEditAmount
	screenSubscriptionNew
	screenSubscriptionEdit
	screenSubscriptionList
	screenSettings
	screenDebtNew
	screenDebtList
	screenDebtEdit
	screenGoalNew
	screenGoalList
	screenGoalEdit
	screenTaxNew
	screenTaxList
	screenTaxEdit
	screenInvoiceNew
	screenInvoiceList
	screenInvoiceEdit
	screenCashflowNew
	screenCashflowHistory
	screenCashflowOverview
)

type subscriptionListMode int

const (
	subscriptionListActive subscriptionListMode = iota
	subscriptionListAll
)

type debtListMode int

const (
	debtListOutgoing debtListMode = iota
	debtListIncoming
	debtListHistory
)

type goalListMode int

const (
	goalListActive goalListMode = iota
	goalListHistory
)

type taxListMode int

const (
	taxListUnpaid taxListMode = iota
	taxListHistory
)

type invoiceListMode int

const (
	invoiceListOutgoingUnpaid invoiceListMode = iota
	invoiceListIncomingUnpaid
	invoiceListHistoryPaid
)

type model struct {
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
	addForm                          addAccountForm
	addSubscriptionForm              addSubscriptionForm
	addDebtForm                      addDebtForm
	addGoalForm                      addGoalForm
	addTaxForm                       addTaxForm
	addInvoiceForm                   addInvoiceForm
	addCashflowForm                  addCashflowForm
	editSubscriptionForm             editSubscriptionForm
	editDebtForm                     editDebtForm
	editGoalForm                     editGoalForm
	editTaxForm                      editTaxForm
	editInvoiceForm                  editInvoiceForm
	editingSubscriptionID            uint
	editingSubscriptionMode          subscriptionListMode
	editInput                        textinput.Model
	editAmountLogDateInput           textinput.Model
	editAmountLogValueInput          textinput.Model
	editAmountActiveField            int
	editAmountUpdateLog              bool
	accountValueLogs                 []account.AccountValueLog
	help                             help.Model
	keys                             keyMap
}

const (
	editAmountFieldCurrent = iota
	editAmountFieldUpdateLog
	editAmountFieldLogDate
	editAmountFieldLogValue
	editAmountFieldCount
)
