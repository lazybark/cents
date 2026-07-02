package app

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
	screenDataExport
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
