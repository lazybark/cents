package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type account struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	Name          string
	Description   string
	Currency      string
	BalanceCents  int64
	LeftoverCents int64
}

type subscription struct {
	ID                uint `gorm:"primaryKey"`
	CreatedAt         time.Time
	LastUpdatedAt     time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	Name              string
	Currency          string
	AmountCents       int64
	Period            string
	PaymentMethod     string
	Type              string
	IsActive          bool
	PaymentDateYearly string
	PaymentDayMonthly *int
}

type debt struct {
	ID              uint `gorm:"primaryKey"`
	CreatedAt       time.Time
	LastUpdatedAt   time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	Peer            string
	Currency        string
	AmountCents     int64
	AmountPaidCents int64
	IsOwedToUser    bool
	DebtCreatedAt   time.Time
	DueDate         *time.Time
	Comment         string
}

type debtLog struct {
	ID             uint `gorm:"primaryKey"`
	CreatedAt      time.Time
	DebtID         uint `gorm:"index;not null"`
	DeltaPaidCents int64
	Note           string
}

type goal struct {
	ID                     uint `gorm:"primaryKey"`
	CreatedAt              time.Time
	LastUpdatedAt          time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	Name                   string
	Currency               string
	TargetAmountCents      int64
	AmountAccumulatedCents int64
	Description            string
	DateStartedAt          time.Time
	TargetDate             *time.Time
}

type goalLog struct {
	ID                    uint `gorm:"primaryKey"`
	CreatedAt             time.Time
	GoalID                uint `gorm:"index;not null"`
	DeltaAccumulatedCents int64
	Note                  string
}

type tax struct {
	ID              uint `gorm:"primaryKey"`
	CreatedAt       time.Time
	LastUpdatedAt   time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	TaxTypeID       uint
	TaxCountry      string
	TaxTypeName     string
	AmountDueCents  int64
	AmountPaidCents int64
	Period          string
	DueDate         *time.Time
	Comment         string
}

type taxLog struct {
	ID             uint `gorm:"primaryKey"`
	CreatedAt      time.Time
	TaxID          uint `gorm:"index;not null"`
	DeltaPaidCents int64
	Note           string
}

type invoice struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	Title         string
	IsIncoming    bool
	Currency      string
	AmountCents   int64
	Paid          bool
	Peer          string
	InvoiceDate   *time.Time
	DueDate       *time.Time
	URL           string
	Description   string
}

type cashflowEntry struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	IsIncome      bool
	Currency      string
	AmountCents   int64
	EntryDate     time.Time
	Category      string
	AccountName   string
	Comment       string
}

type settingRecord struct {
	SettingID    string `gorm:"primaryKey"`
	SettingValue string
}

type settingCurrency struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time
	CurrencyName  string  `gorm:"not null;uniqueIndex"`
	RateToBase    float64 `gorm:"not null"`
}

type settingPaymentMethod struct {
	ID                uint `gorm:"primaryKey"`
	CreatedAt         time.Time
	LastUpdatedAt     time.Time
	PaymentMethodName string `gorm:"not null;uniqueIndex"`
	PaymentMethodType string `gorm:"not null"`
	IsDefault         bool   `gorm:"not null;default:false"`
}

type settingTaxType struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time
	Country       string `gorm:"not null;uniqueIndex:idx_tax_type_country_name"`
	TaxTypeName   string `gorm:"not null;uniqueIndex:idx_tax_type_country_name"`
	Description   string
	URL           string
}

type settingIncomeCategory struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time
	CategoryName  string `gorm:"not null;uniqueIndex"`
}

type settingExpenseCategory struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time
	CategoryName  string `gorm:"not null;uniqueIndex"`
}

type appSettings struct {
	BaseCurrency      string
	Currencies        []settingCurrency
	PaymentMethods    []settingPaymentMethod
	TaxTypes          []settingTaxType
	IncomeCategories  []settingIncomeCategory
	ExpenseCategories []settingExpenseCategory
}

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
	accounts                         []account
	subscriptions                    []subscription
	debts                            []debt
	goals                            []goal
	taxes                            []tax
	invoices                         []invoice
	cashflows                        []cashflowEntry
	subscriptionMode                 subscriptionListMode
	subscriptionCursor               int
	debtMode                         debtListMode
	debtCursor                       int
	debtLogs                         []debtLog
	editingDebtID                    uint
	goalMode                         goalListMode
	goalCursor                       int
	goalLogs                         []goalLog
	editingGoalID                    uint
	taxMode                          taxListMode
	taxCursor                        int
	taxLogs                          []taxLog
	editingTaxID                     uint
	invoiceMode                      invoiceListMode
	invoiceCursor                    int
	editingInvoiceID                 uint
	cashflowHistoryMonth             time.Time
	cashflowCursor                   int
	cashflowOverviewPage             int
	settings                         appSettings
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
	help                             help.Model
	keys                             keyMap
}

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Enter    key.Binding
	Back     key.Binding
	Quit     key.Binding
	Add      key.Binding
	List     key.Binding
	Help     key.Binding
	MoveLeft key.Binding
	MoveRght key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Enter, k.Back, k.Add, k.List, k.Quit},
	}
}

type menuGroup struct {
	title string
	items []string
}

type cashflowMonthlyOverviewRow struct {
	Month         time.Time
	IncomeBase    int64
	ExpenseBase   int64
	NetBase       int64
	DeltaFromPrev int64
	HasPrev       bool
}

type addAccountForm struct {
	fields          []textinput.Model
	labels          []string
	currencyOptions []string
	currencyIndex   int
	active          int
}

type addSubscriptionForm struct {
	inputs               []textinput.Model
	active               int
	currencyOptions      []string
	currencyIndex        int
	paymentMethodOptions []string
	paymentMethodIndex   int
	periodOptions        []string
	periodIndex          int
	typeOptions          []string
	typeIndex            int
	isActive             bool
}

type editSubscriptionForm struct {
	amountInput        textinput.Model
	paymentMethodInput textinput.Model
	isActive           bool
	activeField        int
	periodLabel        string
	typeLabel          string
	currencyLabel      string
	nameLabel          string
	paymentDateYearly  string
	paymentDayMonthly  string
}

type addDebtForm struct {
	inputs          []textinput.Model
	active          int
	currencyOptions []string
	currencyIndex   int
	isOwedToUser    bool
}

type editDebtForm struct {
	amountInput      textinput.Model
	amountPaidInput  textinput.Model
	debtCreatedInput textinput.Model
	dueDateInput     textinput.Model
	commentInput     textinput.Model
	logDeltaInput    textinput.Model
	logDateInput     textinput.Model
	logCommentInput  textinput.Model
	activeField      int
	peerLabel        string
	currencyLabel    string
	directionLabel   string
}

type addGoalForm struct {
	inputs          []textinput.Model
	active          int
	currencyOptions []string
	currencyIndex   int
}

type editGoalForm struct {
	targetAmountInput      textinput.Model
	accumulatedAmountInput textinput.Model
	dateStartedInput       textinput.Model
	targetDateInput        textinput.Model
	descriptionInput       textinput.Model
	logDeltaInput          textinput.Model
	logDateInput           textinput.Model
	logCommentInput        textinput.Model
	activeField            int
	nameLabel              string
	currencyLabel          string
}

type addTaxForm struct {
	inputs          []textinput.Model
	active          int
	taxTypeOptions  []settingTaxType
	taxTypeIndex    int
	taxDisplayNames []string
}

type editTaxForm struct {
	amountDueInput  textinput.Model
	amountPaidInput textinput.Model
	periodInput     textinput.Model
	dueDateInput    textinput.Model
	commentInput    textinput.Model
	logDeltaInput   textinput.Model
	logDateInput    textinput.Model
	logCommentInput textinput.Model
	activeField     int
	taxTypeLabel    string
	countryLabel    string
}

type addInvoiceForm struct {
	inputs          []textinput.Model
	active          int
	currencyOptions []string
	currencyIndex   int
	isIncoming      bool
	paid            bool
}

type editInvoiceForm struct {
	titleInput       textinput.Model
	amountInput      textinput.Model
	peerInput        textinput.Model
	invoiceDateInput textinput.Model
	dueDateInput     textinput.Model
	urlInput         textinput.Model
	descriptionInput textinput.Model
	activeField      int
	currencyOptions  []string
	currencyIndex    int
	isIncoming       bool
	paid             bool
}

type addCashflowForm struct {
	inputs          []textinput.Model
	active          int
	currencyOptions []string
	currencyIndex   int
	isIncome        bool
	categoryOptions []string
	categoryIndex   int
	accountOptions  []string
	accountIndex    int
}

const (
	editSubFieldAmount = iota
	editSubFieldPaymentMethod
	editSubFieldIsActive
	editSubFieldCount
)

const (
	editDebtFieldAmount = iota
	editDebtFieldAmountPaid
	editDebtFieldDebtCreated
	editDebtFieldDueDate
	editDebtFieldComment
	editDebtFieldLogDelta
	editDebtFieldLogDate
	editDebtFieldLogComment
	editDebtFieldCount
)

const (
	editGoalFieldTargetAmount = iota
	editGoalFieldAccumulated
	editGoalFieldDateStarted
	editGoalFieldTargetDate
	editGoalFieldDescription
	editGoalFieldLogDelta
	editGoalFieldLogDate
	editGoalFieldLogComment
	editGoalFieldCount
)

const (
	editTaxFieldAmountDue = iota
	editTaxFieldAmountPaid
	editTaxFieldPeriod
	editTaxFieldDueDate
	editTaxFieldComment
	editTaxFieldLogDelta
	editTaxFieldLogDate
	editTaxFieldLogComment
	editTaxFieldCount
)

const (
	subFieldName = iota
	subFieldCurrency
	subFieldAmount
	subFieldPeriod
	subFieldPaymentMethodChoice
	subFieldPaymentMethod
	subFieldType
	subFieldIsActive
	subFieldDayYearly
	subFieldDayMonthly
	subFieldCount
)

const (
	debtFieldDirection = iota
	debtFieldPeer
	debtFieldCurrency
	debtFieldAmount
	debtFieldAmountPaid
	debtFieldDebtCreated
	debtFieldDueDate
	debtFieldComment
	debtFieldCount
)

const (
	goalFieldName = iota
	goalFieldCurrency
	goalFieldTargetAmount
	goalFieldAccumulated
	goalFieldDescription
	goalFieldDateStarted
	goalFieldTargetDate
	goalFieldCount
)

const (
	taxFieldTaxType = iota
	taxFieldAmountDue
	taxFieldAmountPaid
	taxFieldPeriod
	taxFieldDueDate
	taxFieldComment
	taxFieldCount
)

const (
	invoiceFieldTitle = iota
	invoiceFieldType
	invoiceFieldCurrency
	invoiceFieldAmount
	invoiceFieldPaid
	invoiceFieldPeer
	invoiceFieldInvoiceDate
	invoiceFieldDueDate
	invoiceFieldURL
	invoiceFieldDescription
	invoiceFieldCount
)

const (
	cashflowFieldCurrency = iota
	cashflowFieldAmount
	cashflowFieldDate
	cashflowFieldCategory
	cashflowFieldAccount
	cashflowFieldComment
	cashflowFieldCount
)

var (
	appTitleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F4E9D8"))
	mutedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#9C927F"))
	headlineStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E7C96D"))
	statusStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#D0CABD"))
	statusLabelStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#1F1A17")).Background(lipgloss.Color("#D2B574")).Padding(0, 1)
	sectionTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E7C96D")).Underline(true)
	tableHeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#DCC48A"))
	hintStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#B5AC9D")).Italic(true)
	badgeStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#1F1A17")).Background(lipgloss.Color("#C9A86A")).Bold(true).Padding(0, 1)
	modeBadgeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F4E9D8")).Background(lipgloss.Color("#5F4C2F")).Padding(0, 1)
	progressFillStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8EC07C"))
	progressRestStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5F5A52"))
	panelStyle        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#6F5F47")).Padding(0, 1)
	buttonStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#F4E9D8")).Background(lipgloss.Color("#4E4334")).Padding(0, 1)
	buttonActiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#1F1A17")).Background(lipgloss.Color("#E7C96D")).Bold(true).Padding(0, 1)
	fieldLabelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#E7C96D")).Bold(true)
	selectedRowStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#1F1A17")).Background(lipgloss.Color("#E7C96D"))
	rowStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#F4E9D8"))
	inputBoxStyle     = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#6F5F47")).Padding(0, 1)
	obligationStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B")).Bold(true)
	positiveStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#8EC07C")).Bold(true)
)

func main() {
	db, dbPath, created, err := openDatabase()
	if err != nil {
		fmt.Fprintln(os.Stderr, "database init failed:", err)
		os.Exit(1)
	}

	accounts, err := loadAccounts(db)
	if err != nil {
		fmt.Fprintln(os.Stderr, "database read failed:", err)
		os.Exit(1)
	}

	subscriptions, err := loadSubscriptions(db)
	if err != nil {
		fmt.Fprintln(os.Stderr, "subscriptions read failed:", err)
		os.Exit(1)
	}

	debts, err := loadDebts(db)
	if err != nil {
		fmt.Fprintln(os.Stderr, "debts read failed:", err)
		os.Exit(1)
	}

	goals, err := loadGoals(db)
	if err != nil {
		fmt.Fprintln(os.Stderr, "goals read failed:", err)
		os.Exit(1)
	}

	taxes, err := loadTaxes(db)
	if err != nil {
		fmt.Fprintln(os.Stderr, "taxes read failed:", err)
		os.Exit(1)
	}

	invoices, err := loadInvoices(db)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invoices read failed:", err)
		os.Exit(1)
	}

	cashflows, err := loadCashflows(db)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cashflows read failed:", err)
		os.Exit(1)
	}

	settings, err := loadAppSettings(db)
	if err != nil {
		fmt.Fprintln(os.Stderr, "settings read failed:", err)
		os.Exit(1)
	}

	m := newModel(db, dbPath, created, accounts, subscriptions, debts, goals, taxes, invoices, cashflows, settings)
	program := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "program failed:", err)
		os.Exit(1)
	}
}

func openDatabase() (*gorm.DB, string, bool, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return nil, "", false, err
	}

	dbPath := filepath.Join(workingDir, "cents.db")
	_, statErr := os.Stat(dbPath)
	created := os.IsNotExist(statErr)
	if statErr != nil && !os.IsNotExist(statErr) {
		return nil, "", false, statErr
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, "", false, err
	}

	if err := db.AutoMigrate(&account{}, &subscription{}, &debt{}, &debtLog{}, &goal{}, &goalLog{}, &tax{}, &taxLog{}, &invoice{}, &cashflowEntry{}, &settingRecord{}, &settingCurrency{}, &settingPaymentMethod{}, &settingTaxType{}, &settingIncomeCategory{}, &settingExpenseCategory{}); err != nil {
		return nil, "", false, err
	}

	if err := ensureSettingsDefaults(db); err != nil {
		return nil, "", false, err
	}

	return db, dbPath, created, nil
}

func loadAccounts(db *gorm.DB) ([]account, error) {
	var accounts []account
	if err := db.Order("balance_cents desc, created_at desc, id desc").Find(&accounts).Error; err != nil {
		return nil, err
	}

	return accounts, nil
}

func loadSubscriptions(db *gorm.DB) ([]subscription, error) {
	var subscriptions []subscription
	if err := db.Order("amount_cents desc, created_at desc, id desc").Find(&subscriptions).Error; err != nil {
		return nil, err
	}

	return subscriptions, nil
}

func loadDebts(db *gorm.DB) ([]debt, error) {
	var debts []debt
	if err := db.Order("amount_cents desc, created_at desc, id desc").Find(&debts).Error; err != nil {
		return nil, err
	}
	return debts, nil
}

func loadGoals(db *gorm.DB) ([]goal, error) {
	var goals []goal
	if err := db.Order("target_amount_cents desc, created_at desc, id desc").Find(&goals).Error; err != nil {
		return nil, err
	}
	return goals, nil
}

func loadTaxes(db *gorm.DB) ([]tax, error) {
	var taxes []tax
	if err := db.Order("amount_due_cents desc, created_at desc, id desc").Find(&taxes).Error; err != nil {
		return nil, err
	}
	return taxes, nil
}

func loadInvoices(db *gorm.DB) ([]invoice, error) {
	var invoices []invoice
	if err := db.Order("created_at desc, id desc").Find(&invoices).Error; err != nil {
		return nil, err
	}
	return invoices, nil
}

func loadCashflows(db *gorm.DB) ([]cashflowEntry, error) {
	var entries []cashflowEntry
	if err := db.Order("entry_date desc, created_at desc, id desc").Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func ensureSettingsDefaults(db *gorm.DB) error {
	var count int64
	if err := db.Model(&settingRecord{}).Where("setting_id = ?", "base_currency").Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		if err := db.Create(&settingRecord{SettingID: "base_currency", SettingValue: "$"}).Error; err != nil {
			return err
		}
	}

	count = 0
	if err := db.Model(&settingPaymentMethod{}).Where("is_default = ?", true).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		if err := db.Create(&settingPaymentMethod{PaymentMethodName: "Other", PaymentMethodType: "Other", IsDefault: true}).Error; err != nil {
			return err
		}
	}

	if err := ensurePaymentMethodDefaults(db); err != nil {
		return err
	}

	return nil
}

func ensurePaymentMethodDefaults(db *gorm.DB) error {
	var methods []settingPaymentMethod
	if err := db.Order("created_at asc, id asc").Find(&methods).Error; err != nil {
		return err
	}

	if len(methods) == 0 {
		return db.Create(&settingPaymentMethod{PaymentMethodName: "Other", PaymentMethodType: "Other", IsDefault: true}).Error
	}

	defaultCount := 0
	for _, method := range methods {
		if method.IsDefault {
			defaultCount++
		}
	}

	if defaultCount == 0 {
		return db.Model(&settingPaymentMethod{}).Where("id = ?", methods[0].ID).Update("is_default", true).Error
	}

	if defaultCount > 1 {
		first := true
		for _, method := range methods {
			if method.IsDefault {
				if first {
					first = false
					continue
				}
				if err := db.Model(&settingPaymentMethod{}).Where("id = ?", method.ID).Update("is_default", false).Error; err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func loadAppSettings(db *gorm.DB) (appSettings, error) {
	var rows []settingRecord
	if err := db.Order("setting_id asc").Find(&rows).Error; err != nil {
		return appSettings{}, err
	}

	settings := appSettings{BaseCurrency: "$"}
	for _, row := range rows {
		switch row.SettingID {
		case "base_currency":
			if strings.TrimSpace(row.SettingValue) != "" {
				settings.BaseCurrency = row.SettingValue
			}
		}
	}

	var currencies []settingCurrency
	if err := db.Order("currency_name asc, id asc").Find(&currencies).Error; err != nil {
		return appSettings{}, err
	}
	settings.Currencies = currencies

	var paymentMethods []settingPaymentMethod
	if err := db.Order("is_default desc, payment_method_name asc, id asc").Find(&paymentMethods).Error; err != nil {
		return appSettings{}, err
	}
	settings.PaymentMethods = paymentMethods

	var taxTypes []settingTaxType
	if err := db.Order("country asc, tax_type_name asc, id asc").Find(&taxTypes).Error; err != nil {
		return appSettings{}, err
	}
	settings.TaxTypes = taxTypes

	var incomeCategories []settingIncomeCategory
	if err := db.Order("category_name asc, id asc").Find(&incomeCategories).Error; err != nil {
		return appSettings{}, err
	}
	settings.IncomeCategories = incomeCategories

	var expenseCategories []settingExpenseCategory
	if err := db.Order("category_name asc, id asc").Find(&expenseCategories).Error; err != nil {
		return appSettings{}, err
	}
	settings.ExpenseCategories = expenseCategories

	return settings, nil
}

func newModel(db *gorm.DB, dbPath string, created bool, accounts []account, subscriptions []subscription, debts []debt, goals []goal, taxes []tax, invoices []invoice, cashflows []cashflowEntry, settings appSettings) model {
	accounts = sortAccountsByBaseAmount(accounts, settings)
	currencyOptions := currencySelectionOptions(settings)
	paymentMethodOptions := paymentMethodSelectionOptions(settings)
	addForm := newAddAccountForm(currencyOptions)
	addSubForm := newAddSubscriptionForm(currencyOptions, paymentMethodOptions)
	addDebtForm := newAddDebtForm(currencyOptions)
	addGoalForm := newAddGoalForm(currencyOptions)
	addTaxForm := newAddTaxForm(settings.TaxTypes)
	addInvoiceForm := newAddInvoiceForm(currencyOptions)
	addCashflowForm := newAddCashflowForm(currencyOptions, incomeCategorySelectionOptions(settings), accountSelectionOptions(accounts), true)
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

	return model{
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
		addForm:                          addForm,
		addSubscriptionForm:              addSubForm,
		addDebtForm:                      addDebtForm,
		addGoalForm:                      addGoalForm,
		addTaxForm:                       addTaxForm,
		addInvoiceForm:                   addInvoiceForm,
		addCashflowForm:                  addCashflowForm,
		editSubscriptionForm:             newEditSubscriptionForm(),
		editDebtForm:                     newEditDebtForm(),
		editGoalForm:                     newEditGoalForm(),
		editTaxForm:                      newEditTaxForm(),
		editInvoiceForm:                  newEditInvoiceForm(currencyOptions),
		editInput:                        editInput,
		help:                             helpModel,
		keys:                             newKeyMap(),
	}
}

func newEditSubscriptionForm() editSubscriptionForm {
	amountInput := textinput.New()
	amountInput.Placeholder = "9.99"
	amountInput.CharLimit = 24
	amountInput.Width = 20

	paymentMethodInput := textinput.New()
	paymentMethodInput.Placeholder = "Card **** 1234"
	paymentMethodInput.CharLimit = 80
	paymentMethodInput.Width = 28

	return editSubscriptionForm{
		amountInput:        amountInput,
		paymentMethodInput: paymentMethodInput,
		isActive:           true,
		activeField:        0,
	}
}

func (f editSubscriptionForm) focusActive() editSubscriptionForm {
	f.amountInput.Blur()
	f.paymentMethodInput.Blur()
	if f.activeField == editSubFieldAmount {
		f.amountInput.Focus()
	}
	if f.activeField == editSubFieldPaymentMethod {
		f.paymentMethodInput.Focus()
	}
	return f
}

func (f editSubscriptionForm) next() editSubscriptionForm {
	if f.activeField < editSubFieldCount-1 {
		f.activeField++
	}
	return f.focusActive()
}

func (f editSubscriptionForm) prev() editSubscriptionForm {
	if f.activeField > 0 {
		f.activeField--
	}
	return f.focusActive()
}

func newAddSubscriptionForm(currencyOptions []string, paymentMethodOptions []string) addSubscriptionForm {
	inputs := make([]textinput.Model, 6)
	placeholders := []string{"GitHub", "", "9.99", "Card **** 1234", "12.12.2012", "18"}
	for i := range inputs {
		field := textinput.New()
		field.Placeholder = placeholders[i]
		field.CharLimit = 80
		field.Width = 28
		inputs[i] = field
	}

	form := addSubscriptionForm{
		inputs:               inputs,
		active:               0,
		currencyOptions:      append([]string(nil), currencyOptions...),
		currencyIndex:        0,
		paymentMethodOptions: append([]string(nil), paymentMethodOptions...),
		paymentMethodIndex:   0,
		periodOptions:        []string{"month", "year"},
		periodIndex:          0,
		typeOptions:          []string{"Software", "Domain", "Service", "Multimedia", "Other"},
		typeIndex:            0,
		isActive:             true,
	}

	return form.focusActive()
}

func (f addSubscriptionForm) inputIndexForField(field int) int {
	switch field {
	case subFieldName:
		return 0
	case subFieldCurrency:
		return -1
	case subFieldAmount:
		return 2
	case subFieldPaymentMethodChoice:
		return -1
	case subFieldPaymentMethod:
		return 3
	case subFieldDayYearly:
		return 4
	case subFieldDayMonthly:
		return 5
	default:
		return -1
	}
}

func (f addSubscriptionForm) focusActive() addSubscriptionForm {
	for i := range f.inputs {
		f.inputs[i].Blur()
	}

	if inputIndex := f.inputIndexForField(f.active); inputIndex >= 0 {
		f.inputs[inputIndex].Focus()
	}

	return f
}

func (f addSubscriptionForm) next() addSubscriptionForm {
	if f.active < subFieldCount-1 {
		f.active++
	}

	return f.focusActive()
}

func (f addSubscriptionForm) prev() addSubscriptionForm {
	if f.active > 0 {
		f.active--
	}

	return f.focusActive()
}

func newAddDebtForm(currencyOptions []string) addDebtForm {
	inputs := make([]textinput.Model, 6)
	placeholders := []string{"John Doe", "1000.00", "0.00", time.Now().Format("02.01.2006"), "", "Optional comment"}
	for i := range inputs {
		field := textinput.New()
		field.Placeholder = placeholders[i]
		field.CharLimit = 120
		field.Width = 34
		inputs[i] = field
	}

	form := addDebtForm{
		inputs:          inputs,
		active:          0,
		currencyOptions: append([]string(nil), currencyOptions...),
		currencyIndex:   0,
		isOwedToUser:    false,
	}

	return form.focusActive()
}

func (f addDebtForm) inputIndexForField(field int) int {
	switch field {
	case debtFieldDirection:
		return -1
	case debtFieldPeer:
		return 0
	case debtFieldCurrency:
		return -1
	case debtFieldAmount:
		return 1
	case debtFieldAmountPaid:
		return 2
	case debtFieldDebtCreated:
		return 3
	case debtFieldDueDate:
		return 4
	case debtFieldComment:
		return 5
	default:
		return -1
	}
}

func (f addDebtForm) focusActive() addDebtForm {
	for i := range f.inputs {
		f.inputs[i].Blur()
	}
	if inputIndex := f.inputIndexForField(f.active); inputIndex >= 0 {
		f.inputs[inputIndex].Focus()
	}
	return f
}

func (f addDebtForm) next() addDebtForm {
	if f.active < debtFieldCount-1 {
		f.active++
	}
	return f.focusActive()
}

func (f addDebtForm) prev() addDebtForm {
	if f.active > 0 {
		f.active--
	}
	return f.focusActive()
}

func newEditDebtForm() editDebtForm {
	amountInput := textinput.New()
	amountInput.Placeholder = "1000.00"
	amountInput.CharLimit = 24
	amountInput.Width = 20

	amountPaidInput := textinput.New()
	amountPaidInput.Placeholder = "0.00"
	amountPaidInput.CharLimit = 24
	amountPaidInput.Width = 20

	debtCreatedInput := textinput.New()
	debtCreatedInput.Placeholder = "02.01.2006"
	debtCreatedInput.CharLimit = 24
	debtCreatedInput.Width = 20

	dueDateInput := textinput.New()
	dueDateInput.Placeholder = "optional DD.MM.YYYY"
	dueDateInput.CharLimit = 24
	dueDateInput.Width = 20

	commentInput := textinput.New()
	commentInput.Placeholder = "comment"
	commentInput.CharLimit = 120
	commentInput.Width = 36

	logDeltaInput := textinput.New()
	logDeltaInput.Placeholder = "+10.00 or -5.00"
	logDeltaInput.CharLimit = 24
	logDeltaInput.Width = 24

	logDateInput := textinput.New()
	logDateInput.Placeholder = "DD.MM.YYYY (optional)"
	logDateInput.CharLimit = 24
	logDateInput.Width = 24

	logCommentInput := textinput.New()
	logCommentInput.Placeholder = "transaction note (optional)"
	logCommentInput.CharLimit = 120
	logCommentInput.Width = 36

	form := editDebtForm{
		amountInput:      amountInput,
		amountPaidInput:  amountPaidInput,
		debtCreatedInput: debtCreatedInput,
		dueDateInput:     dueDateInput,
		commentInput:     commentInput,
		logDeltaInput:    logDeltaInput,
		logDateInput:     logDateInput,
		logCommentInput:  logCommentInput,
		activeField:      0,
	}

	return form.focusActive()
}

func (f editDebtForm) focusActive() editDebtForm {
	f.amountInput.Blur()
	f.amountPaidInput.Blur()
	f.debtCreatedInput.Blur()
	f.dueDateInput.Blur()
	f.commentInput.Blur()
	f.logDeltaInput.Blur()
	f.logDateInput.Blur()
	f.logCommentInput.Blur()

	switch f.activeField {
	case editDebtFieldAmount:
		f.amountInput.Focus()
	case editDebtFieldAmountPaid:
		f.amountPaidInput.Focus()
	case editDebtFieldDebtCreated:
		f.debtCreatedInput.Focus()
	case editDebtFieldDueDate:
		f.dueDateInput.Focus()
	case editDebtFieldComment:
		f.commentInput.Focus()
	case editDebtFieldLogDelta:
		f.logDeltaInput.Focus()
	case editDebtFieldLogDate:
		f.logDateInput.Focus()
	case editDebtFieldLogComment:
		f.logCommentInput.Focus()
	}
	return f
}

func (f editDebtForm) next() editDebtForm {
	if f.activeField < editDebtFieldCount-1 {
		f.activeField++
	}
	return f.focusActive()
}

func (f editDebtForm) prev() editDebtForm {
	if f.activeField > 0 {
		f.activeField--
	}
	return f.focusActive()
}

func newAddGoalForm(currencyOptions []string) addGoalForm {
	today := time.Now().Format("02.01.2006")
	inputs := make([]textinput.Model, 6)
	placeholders := []string{"Emergency Fund", "10000.00", "0.00", "Optional description", today, "optional DD.MM.YYYY"}
	for i := range inputs {
		field := textinput.New()
		field.Placeholder = placeholders[i]
		field.CharLimit = 120
		field.Width = 34
		if i == 4 {
			field.SetValue(today)
		}
		inputs[i] = field
	}

	form := addGoalForm{
		inputs:          inputs,
		active:          0,
		currencyOptions: append([]string(nil), currencyOptions...),
		currencyIndex:   0,
	}

	return form.focusActive()
}

func (f addGoalForm) inputIndexForField(field int) int {
	switch field {
	case goalFieldName:
		return 0
	case goalFieldCurrency:
		return -1
	case goalFieldTargetAmount:
		return 1
	case goalFieldAccumulated:
		return 2
	case goalFieldDescription:
		return 3
	case goalFieldDateStarted:
		return 4
	case goalFieldTargetDate:
		return 5
	default:
		return -1
	}
}

func (f addGoalForm) focusActive() addGoalForm {
	for i := range f.inputs {
		f.inputs[i].Blur()
	}
	if inputIndex := f.inputIndexForField(f.active); inputIndex >= 0 {
		f.inputs[inputIndex].Focus()
	}
	return f
}

func (f addGoalForm) next() addGoalForm {
	if f.active < goalFieldCount-1 {
		f.active++
	}
	return f.focusActive()
}

func (f addGoalForm) prev() addGoalForm {
	if f.active > 0 {
		f.active--
	}
	return f.focusActive()
}

func newEditGoalForm() editGoalForm {
	targetAmountInput := textinput.New()
	targetAmountInput.Placeholder = "10000.00"
	targetAmountInput.CharLimit = 24
	targetAmountInput.Width = 20

	accumulatedAmountInput := textinput.New()
	accumulatedAmountInput.Placeholder = "0.00"
	accumulatedAmountInput.CharLimit = 24
	accumulatedAmountInput.Width = 20

	dateStartedInput := textinput.New()
	dateStartedInput.Placeholder = "02.01.2006"
	dateStartedInput.CharLimit = 24
	dateStartedInput.Width = 20

	targetDateInput := textinput.New()
	targetDateInput.Placeholder = "optional DD.MM.YYYY"
	targetDateInput.CharLimit = 24
	targetDateInput.Width = 24

	descriptionInput := textinput.New()
	descriptionInput.Placeholder = "description"
	descriptionInput.CharLimit = 120
	descriptionInput.Width = 36

	logDeltaInput := textinput.New()
	logDeltaInput.Placeholder = "+100.00 or -50.00"
	logDeltaInput.CharLimit = 24
	logDeltaInput.Width = 24

	logDateInput := textinput.New()
	logDateInput.Placeholder = "DD.MM.YYYY (optional)"
	logDateInput.CharLimit = 24
	logDateInput.Width = 24

	logCommentInput := textinput.New()
	logCommentInput.Placeholder = "transaction note (optional)"
	logCommentInput.CharLimit = 120
	logCommentInput.Width = 36

	form := editGoalForm{
		targetAmountInput:      targetAmountInput,
		accumulatedAmountInput: accumulatedAmountInput,
		dateStartedInput:       dateStartedInput,
		targetDateInput:        targetDateInput,
		descriptionInput:       descriptionInput,
		logDeltaInput:          logDeltaInput,
		logDateInput:           logDateInput,
		logCommentInput:        logCommentInput,
		activeField:            0,
	}

	return form.focusActive()
}

func (f editGoalForm) focusActive() editGoalForm {
	f.targetAmountInput.Blur()
	f.accumulatedAmountInput.Blur()
	f.dateStartedInput.Blur()
	f.targetDateInput.Blur()
	f.descriptionInput.Blur()
	f.logDeltaInput.Blur()
	f.logDateInput.Blur()
	f.logCommentInput.Blur()

	switch f.activeField {
	case editGoalFieldTargetAmount:
		f.targetAmountInput.Focus()
	case editGoalFieldAccumulated:
		f.accumulatedAmountInput.Focus()
	case editGoalFieldDateStarted:
		f.dateStartedInput.Focus()
	case editGoalFieldTargetDate:
		f.targetDateInput.Focus()
	case editGoalFieldDescription:
		f.descriptionInput.Focus()
	case editGoalFieldLogDelta:
		f.logDeltaInput.Focus()
	case editGoalFieldLogDate:
		f.logDateInput.Focus()
	case editGoalFieldLogComment:
		f.logCommentInput.Focus()
	}
	return f
}

func (f editGoalForm) next() editGoalForm {
	if f.activeField < editGoalFieldCount-1 {
		f.activeField++
	}
	return f.focusActive()
}

func (f editGoalForm) prev() editGoalForm {
	if f.activeField > 0 {
		f.activeField--
	}
	return f.focusActive()
}

func newAddTaxForm(taxTypeOptions []settingTaxType) addTaxForm {
	today := time.Now().Format("02.01.2006")
	inputs := make([]textinput.Model, 5)
	placeholders := []string{"1000.00", "0.00", "Q1 2026", today, "Optional comment"}
	for i := range inputs {
		field := textinput.New()
		field.Placeholder = placeholders[i]
		field.CharLimit = 120
		field.Width = 34
		if i == 3 {
			field.SetValue(today)
		}
		inputs[i] = field
	}

	display := make([]string, 0, len(taxTypeOptions))
	for _, item := range taxTypeOptions {
		label := strings.TrimSpace(item.Country)
		name := strings.TrimSpace(item.TaxTypeName)
		if label == "" {
			label = "Unknown"
		}
		if name == "" {
			name = "Tax"
		}
		display = append(display, label+" / "+name)
	}

	form := addTaxForm{
		inputs:          inputs,
		active:          0,
		taxTypeOptions:  append([]settingTaxType(nil), taxTypeOptions...),
		taxTypeIndex:    0,
		taxDisplayNames: display,
	}

	return form.focusActive()
}

func (f addTaxForm) inputIndexForField(field int) int {
	switch field {
	case taxFieldTaxType:
		return -1
	case taxFieldAmountDue:
		return 0
	case taxFieldAmountPaid:
		return 1
	case taxFieldPeriod:
		return 2
	case taxFieldDueDate:
		return 3
	case taxFieldComment:
		return 4
	default:
		return -1
	}
}

func (f addTaxForm) focusActive() addTaxForm {
	for i := range f.inputs {
		f.inputs[i].Blur()
	}
	if inputIndex := f.inputIndexForField(f.active); inputIndex >= 0 {
		f.inputs[inputIndex].Focus()
	}
	return f
}

func (f addTaxForm) next() addTaxForm {
	if f.active < taxFieldCount-1 {
		f.active++
	}
	return f.focusActive()
}

func (f addTaxForm) prev() addTaxForm {
	if f.active > 0 {
		f.active--
	}
	return f.focusActive()
}

func newEditTaxForm() editTaxForm {
	amountDueInput := textinput.New()
	amountDueInput.Placeholder = "1000.00"
	amountDueInput.CharLimit = 24
	amountDueInput.Width = 20

	amountPaidInput := textinput.New()
	amountPaidInput.Placeholder = "0.00"
	amountPaidInput.CharLimit = 24
	amountPaidInput.Width = 20

	periodInput := textinput.New()
	periodInput.Placeholder = "Q1 2026"
	periodInput.CharLimit = 60
	periodInput.Width = 24

	dueDateInput := textinput.New()
	dueDateInput.Placeholder = "optional DD.MM.YYYY"
	dueDateInput.CharLimit = 24
	dueDateInput.Width = 20

	commentInput := textinput.New()
	commentInput.Placeholder = "comment"
	commentInput.CharLimit = 140
	commentInput.Width = 36

	logDeltaInput := textinput.New()
	logDeltaInput.Placeholder = "+10.00 or -5.00"
	logDeltaInput.CharLimit = 24
	logDeltaInput.Width = 24

	logDateInput := textinput.New()
	logDateInput.Placeholder = "DD.MM.YYYY (optional)"
	logDateInput.CharLimit = 24
	logDateInput.Width = 24

	logCommentInput := textinput.New()
	logCommentInput.Placeholder = "transaction note (optional)"
	logCommentInput.CharLimit = 120
	logCommentInput.Width = 36

	form := editTaxForm{
		amountDueInput:  amountDueInput,
		amountPaidInput: amountPaidInput,
		periodInput:     periodInput,
		dueDateInput:    dueDateInput,
		commentInput:    commentInput,
		logDeltaInput:   logDeltaInput,
		logDateInput:    logDateInput,
		logCommentInput: logCommentInput,
		activeField:     0,
	}

	return form.focusActive()
}

func (f editTaxForm) focusActive() editTaxForm {
	f.amountDueInput.Blur()
	f.amountPaidInput.Blur()
	f.periodInput.Blur()
	f.dueDateInput.Blur()
	f.commentInput.Blur()
	f.logDeltaInput.Blur()
	f.logDateInput.Blur()
	f.logCommentInput.Blur()

	switch f.activeField {
	case editTaxFieldAmountDue:
		f.amountDueInput.Focus()
	case editTaxFieldAmountPaid:
		f.amountPaidInput.Focus()
	case editTaxFieldPeriod:
		f.periodInput.Focus()
	case editTaxFieldDueDate:
		f.dueDateInput.Focus()
	case editTaxFieldComment:
		f.commentInput.Focus()
	case editTaxFieldLogDelta:
		f.logDeltaInput.Focus()
	case editTaxFieldLogDate:
		f.logDateInput.Focus()
	case editTaxFieldLogComment:
		f.logCommentInput.Focus()
	}
	return f
}

func (f editTaxForm) next() editTaxForm {
	if f.activeField < editTaxFieldCount-1 {
		f.activeField++
	}
	return f.focusActive()
}

func (f editTaxForm) prev() editTaxForm {
	if f.activeField > 0 {
		f.activeField--
	}
	return f.focusActive()
}

func newAddInvoiceForm(currencyOptions []string) addInvoiceForm {
	currencyOptions = invoiceCurrencySelectionOptions(currencyOptions)
	today := time.Now().Format("02.01.2006")
	inputs := make([]textinput.Model, 7)
	placeholders := []string{"Invoice title", "1000.00", "Peer", today, "optional DD.MM.YYYY", "optional https://...", "optional description"}
	for i := range inputs {
		field := textinput.New()
		field.Placeholder = placeholders[i]
		field.CharLimit = 180
		field.Width = 36
		inputs[i] = field
	}

	form := addInvoiceForm{
		inputs:          inputs,
		active:          0,
		currencyOptions: append([]string(nil), currencyOptions...),
		currencyIndex:   0,
		isIncoming:      true,
		paid:            false,
	}

	return form.focusActive()
}

func (f addInvoiceForm) inputIndexForField(field int) int {
	switch field {
	case invoiceFieldTitle:
		return 0
	case invoiceFieldType:
		return -1
	case invoiceFieldCurrency:
		return -1
	case invoiceFieldAmount:
		return 1
	case invoiceFieldPaid:
		return -1
	case invoiceFieldPeer:
		return 2
	case invoiceFieldInvoiceDate:
		return 3
	case invoiceFieldDueDate:
		return 4
	case invoiceFieldURL:
		return 5
	case invoiceFieldDescription:
		return 6
	default:
		return -1
	}
}

func (f addInvoiceForm) focusActive() addInvoiceForm {
	for i := range f.inputs {
		f.inputs[i].Blur()
	}
	if inputIndex := f.inputIndexForField(f.active); inputIndex >= 0 {
		f.inputs[inputIndex].Focus()
	}
	return f
}

func (f addInvoiceForm) next() addInvoiceForm {
	if f.active < invoiceFieldCount-1 {
		f.active++
	}
	return f.focusActive()
}

func (f addInvoiceForm) prev() addInvoiceForm {
	if f.active > 0 {
		f.active--
	}
	return f.focusActive()
}

func newEditInvoiceForm(currencyOptions []string) editInvoiceForm {
	currencyOptions = invoiceCurrencySelectionOptions(currencyOptions)
	titleInput := textinput.New()
	titleInput.Placeholder = "Invoice title"
	titleInput.CharLimit = 120
	titleInput.Width = 36

	amountInput := textinput.New()
	amountInput.Placeholder = "optional 1000.00"
	amountInput.CharLimit = 24
	amountInput.Width = 20

	peerInput := textinput.New()
	peerInput.Placeholder = "peer"
	peerInput.CharLimit = 120
	peerInput.Width = 30

	invoiceDateInput := textinput.New()
	invoiceDateInput.Placeholder = "optional DD.MM.YYYY"
	invoiceDateInput.CharLimit = 24
	invoiceDateInput.Width = 24

	dueDateInput := textinput.New()
	dueDateInput.Placeholder = "optional DD.MM.YYYY"
	dueDateInput.CharLimit = 24
	dueDateInput.Width = 24

	urlInput := textinput.New()
	urlInput.Placeholder = "optional https://..."
	urlInput.CharLimit = 180
	urlInput.Width = 42

	descriptionInput := textinput.New()
	descriptionInput.Placeholder = "optional description"
	descriptionInput.CharLimit = 180
	descriptionInput.Width = 42

	form := editInvoiceForm{
		titleInput:       titleInput,
		amountInput:      amountInput,
		peerInput:        peerInput,
		invoiceDateInput: invoiceDateInput,
		dueDateInput:     dueDateInput,
		urlInput:         urlInput,
		descriptionInput: descriptionInput,
		activeField:      0,
		currencyOptions:  append([]string(nil), currencyOptions...),
		currencyIndex:    0,
		isIncoming:       true,
		paid:             false,
	}

	return form.focusActive()
}

func (f editInvoiceForm) focusActive() editInvoiceForm {
	f.titleInput.Blur()
	f.amountInput.Blur()
	f.peerInput.Blur()
	f.invoiceDateInput.Blur()
	f.dueDateInput.Blur()
	f.urlInput.Blur()
	f.descriptionInput.Blur()

	switch f.activeField {
	case invoiceFieldTitle:
		f.titleInput.Focus()
	case invoiceFieldAmount:
		f.amountInput.Focus()
	case invoiceFieldPeer:
		f.peerInput.Focus()
	case invoiceFieldInvoiceDate:
		f.invoiceDateInput.Focus()
	case invoiceFieldDueDate:
		f.dueDateInput.Focus()
	case invoiceFieldURL:
		f.urlInput.Focus()
	case invoiceFieldDescription:
		f.descriptionInput.Focus()
	}
	return f
}

func (f editInvoiceForm) next() editInvoiceForm {
	if f.activeField < invoiceFieldCount-1 {
		f.activeField++
	}
	return f.focusActive()
}

func (f editInvoiceForm) prev() editInvoiceForm {
	if f.activeField > 0 {
		f.activeField--
	}
	return f.focusActive()
}

func newAddCashflowForm(currencyOptions []string, categoryOptions []string, accountOptions []string, isIncome bool) addCashflowForm {
	today := time.Now().Format("02.01.2006")
	inputs := make([]textinput.Model, 3)
	placeholders := []string{"1000.00", today, "optional comment"}
	for i := range inputs {
		field := textinput.New()
		field.Placeholder = placeholders[i]
		field.CharLimit = 140
		field.Width = 34
		if i == 1 {
			field.SetValue(today)
		}
		inputs[i] = field
	}

	form := addCashflowForm{
		inputs:          inputs,
		active:          0,
		currencyOptions: append([]string(nil), currencyOptions...),
		currencyIndex:   0,
		isIncome:        isIncome,
		categoryOptions: append([]string(nil), categoryOptions...),
		categoryIndex:   0,
		accountOptions:  append([]string(nil), accountOptions...),
		accountIndex:    0,
	}

	return form.focusActive()
}

func (f addCashflowForm) inputIndexForField(field int) int {
	switch field {
	case cashflowFieldCurrency:
		return -1
	case cashflowFieldAmount:
		return 0
	case cashflowFieldDate:
		return 1
	case cashflowFieldCategory:
		return -1
	case cashflowFieldAccount:
		return -1
	case cashflowFieldComment:
		return 2
	default:
		return -1
	}
}

func (f addCashflowForm) focusActive() addCashflowForm {
	for i := range f.inputs {
		f.inputs[i].Blur()
	}
	if inputIndex := f.inputIndexForField(f.active); inputIndex >= 0 {
		f.inputs[inputIndex].Focus()
	}
	return f
}

func (f addCashflowForm) next() addCashflowForm {
	if f.active < cashflowFieldCount-1 {
		f.active++
	}
	return f.focusActive()
}

func (f addCashflowForm) prev() addCashflowForm {
	if f.active > 0 {
		f.active--
	}
	return f.focusActive()
}

func incomeCategorySelectionOptions(settings appSettings) []string {
	items := make([]string, 0, len(settings.IncomeCategories))
	for _, category := range settings.IncomeCategories {
		name := strings.TrimSpace(category.CategoryName)
		if name != "" {
			items = append(items, name)
		}
	}
	return items
}

func expenseCategorySelectionOptions(settings appSettings) []string {
	items := make([]string, 0, len(settings.ExpenseCategories))
	for _, category := range settings.ExpenseCategories {
		name := strings.TrimSpace(category.CategoryName)
		if name != "" {
			items = append(items, name)
		}
	}
	return items
}

func accountSelectionOptions(accounts []account) []string {
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

func newKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "prev group"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "next group"),
		),
		Left: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←", "prev item"),
		),
		Right: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("→", "next item"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Add: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add account"),
		),
		List: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "list accounts"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
	}
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
		{title: "Settings", items: []string{"edit", "backup"}},
	}
}

func newAddAccountForm(currencyOptions []string) addAccountForm {
	labels := []string{"Name", "Description", "Currency", "Amount"}
	placeholders := []string{"Emergency Fund", "Rainy day savings", "", "2500.00"}
	fields := make([]textinput.Model, len(labels))

	for i := range fields {
		field := textinput.New()
		field.Placeholder = placeholders[i]
		field.CharLimit = 80
		field.Width = 34
		fields[i] = field
	}

	form := addAccountForm{fields: fields, labels: labels, currencyOptions: append([]string(nil), currencyOptions...), currencyIndex: 0}
	return form.focusActive()
}

func (f addAccountForm) focusActive() addAccountForm {
	for i := range f.fields {
		if i == f.active {
			f.fields[i].Focus()
		} else {
			f.fields[i].Blur()
		}
	}

	return f
}

func (f addAccountForm) next() addAccountForm {
	if f.active < len(f.fields)-1 {
		f.active++
	}

	return f.focusActive()
}

func (f addAccountForm) prev() addAccountForm {
	if f.active > 0 {
		f.active--
	}

	return f.focusActive()
}

func databaseStatus(created bool, count int, dbPath string) string {
	if created {
		return "created " + dbPath + " and loaded " + strconv.Itoa(count) + " accounts"
	}

	return "opened " + dbPath + " with " + strconv.Itoa(count) + " accounts"
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.help, cmd = m.help.Update(msg)
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = clamp(msg.Width-6, 60, 120)
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
		}
	}

	if m.screen == screenAddAccount {
		m.addForm.fields[m.addForm.active], cmd = m.addForm.fields[m.addForm.active].Update(msg)
		return m, cmd
	}

	if m.screen == screenEditAmount {
		m.editInput, cmd = m.editInput.Update(msg)
		return m, cmd
	}

	if m.screen == screenSubscriptionNew {
		if inputIndex := m.addSubscriptionForm.inputIndexForField(m.addSubscriptionForm.active); inputIndex >= 0 {
			m.addSubscriptionForm.inputs[inputIndex], cmd = m.addSubscriptionForm.inputs[inputIndex].Update(msg)
			return m, cmd
		}
	}

	if m.screen == screenSubscriptionEdit {
		switch m.editSubscriptionForm.activeField {
		case editSubFieldAmount:
			m.editSubscriptionForm.amountInput, cmd = m.editSubscriptionForm.amountInput.Update(msg)
			return m, cmd
		case editSubFieldPaymentMethod:
			m.editSubscriptionForm.paymentMethodInput, cmd = m.editSubscriptionForm.paymentMethodInput.Update(msg)
			return m, cmd
		}
	}

	return m, cmd
}

func (m model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func (m model) activateMenuSelection() (tea.Model, tea.Cmd) {
	if m.menuGroup == 0 && m.menuItem == 0 {
		m.screen = screenCashflowNew
		m.addCashflowForm = newAddCashflowForm(currencySelectionOptions(m.settings), expenseCategorySelectionOptions(m.settings), accountSelectionOptions(m.accounts), false)
		m.status = "new expense"
		return m, nil
	}

	if m.menuGroup == 0 && m.menuItem == 1 {
		m.screen = screenCashflowNew
		m.addCashflowForm = newAddCashflowForm(currencySelectionOptions(m.settings), incomeCategorySelectionOptions(m.settings), accountSelectionOptions(m.accounts), true)
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
		m.addForm = newAddAccountForm(currencySelectionOptions(m.settings))
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
		m.addSubscriptionForm = newAddSubscriptionForm(currencySelectionOptions(m.settings), paymentMethodSelectionOptions(m.settings))
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
		m.addInvoiceForm = newAddInvoiceForm(currencySelectionOptions(m.settings))
		m.status = "new invoice"
		return m, nil
	}

	if m.menuGroup == 3 && m.menuItem == 1 {
		m.screen = screenInvoiceList
		m.invoiceMode = invoiceListOutgoingUnpaid
		m.invoiceCursor = 0
		m.status = "outgoing unpaid invoices"
		return m, nil
	}

	if m.menuGroup == 3 && m.menuItem == 2 {
		m.screen = screenInvoiceList
		m.invoiceMode = invoiceListIncomingUnpaid
		m.invoiceCursor = 0
		m.status = "incoming unpaid invoices"
		return m, nil
	}

	if m.menuGroup == 3 && m.menuItem == 3 {
		m.screen = screenInvoiceList
		m.invoiceMode = invoiceListHistoryPaid
		m.invoiceCursor = 0
		m.status = "invoice history"
		return m, nil
	}

	if m.menuGroup == 4 && m.menuItem == 0 {
		m.screen = screenDebtNew
		m.addDebtForm = newAddDebtForm(currencySelectionOptions(m.settings))
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
		m.addGoalForm = newAddGoalForm(currencySelectionOptions(m.settings))
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
		m.addTaxForm = newAddTaxForm(m.settings.TaxTypes)
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

	groups := appMenuGroups()
	m.status = strings.ToLower(groups[m.menuGroup].title) + " / " + groups[m.menuGroup].items[m.menuItem] + " is a template action (coming next)"
	return m, nil
}

func (m model) updateAddAccount(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		return m, nil
	case "up", "shift+tab":
		m.addForm = m.addForm.prev()
		return m, nil
	case "down", "tab":
		m.addForm = m.addForm.next()
		return m, nil
	case "left":
		if m.addForm.active == 2 && m.addForm.currencyIndex > 0 {
			m.addForm.currencyIndex--
		}
		return m, nil
	case "right":
		if m.addForm.active == 2 && m.addForm.currencyIndex < len(m.addForm.currencyOptions)-1 {
			m.addForm.currencyIndex++
		}
		return m, nil
	case "enter":
		if m.addForm.active == len(m.addForm.fields)-1 && msg.String() == "enter" {
			return m.saveAccountFromForm()
		}
		m.addForm = m.addForm.next()
		return m, nil
	}

	if m.addForm.active == 2 {
		return m, nil
	}

	var cmd tea.Cmd
	m.addForm.fields[m.addForm.active], cmd = m.addForm.fields[m.addForm.active].Update(msg)
	return m, cmd
}

func (m model) updateAccountTable(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if updatedModel, cmd, handled := m.handleDeleteConfirmation(msg); handled {
		return updatedModel, cmd
	}

	if len(m.accounts) == 0 {
		m.screen = screenMenu
		m.status = "no accounts available"
		return m, nil
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

func (m model) updateEditAmount(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenAccountTable
		m.status = "amount edit cancelled"
		return m, nil
	case "enter":
		return m.saveAmount()
	}

	var cmd tea.Cmd
	m.editInput, cmd = m.editInput.Update(msg)
	return m, cmd
}

func (m model) updateSubscriptionNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		return m, nil
	case "up", "shift+tab":
		m.addSubscriptionForm = m.addSubscriptionForm.prev()
		return m, nil
	case "down", "tab":
		m.addSubscriptionForm = m.addSubscriptionForm.next()
		return m, nil
	case "left":
		switch m.addSubscriptionForm.active {
		case subFieldCurrency:
			if m.addSubscriptionForm.currencyIndex > 0 {
				m.addSubscriptionForm.currencyIndex--
			}
		case subFieldPaymentMethodChoice:
			if m.addSubscriptionForm.paymentMethodIndex > 0 {
				m.addSubscriptionForm.paymentMethodIndex--
			}
		case subFieldPeriod:
			if m.addSubscriptionForm.periodIndex > 0 {
				m.addSubscriptionForm.periodIndex--
			}
		case subFieldType:
			if m.addSubscriptionForm.typeIndex > 0 {
				m.addSubscriptionForm.typeIndex--
			}
		}
		return m, nil
	case "right":
		switch m.addSubscriptionForm.active {
		case subFieldCurrency:
			if m.addSubscriptionForm.currencyIndex < len(m.addSubscriptionForm.currencyOptions)-1 {
				m.addSubscriptionForm.currencyIndex++
			}
		case subFieldPaymentMethodChoice:
			if m.addSubscriptionForm.paymentMethodIndex < len(m.addSubscriptionForm.paymentMethodOptions)-1 {
				m.addSubscriptionForm.paymentMethodIndex++
			}
		case subFieldPeriod:
			if m.addSubscriptionForm.periodIndex < len(m.addSubscriptionForm.periodOptions)-1 {
				m.addSubscriptionForm.periodIndex++
			}
		case subFieldType:
			if m.addSubscriptionForm.typeIndex < len(m.addSubscriptionForm.typeOptions)-1 {
				m.addSubscriptionForm.typeIndex++
			}
		}
		return m, nil
	case " ":
		if m.addSubscriptionForm.active == subFieldIsActive {
			m.addSubscriptionForm.isActive = !m.addSubscriptionForm.isActive
			return m, nil
		}
		// Let text inputs receive spaces when a text field is active.
	case "enter":
		if m.addSubscriptionForm.active == subFieldCount-1 {
			return m.saveSubscriptionFromForm()
		}
		m.addSubscriptionForm = m.addSubscriptionForm.next()
		return m, nil
	}

	if inputIndex := m.addSubscriptionForm.inputIndexForField(m.addSubscriptionForm.active); inputIndex >= 0 {
		var cmd tea.Cmd
		m.addSubscriptionForm.inputs[inputIndex], cmd = m.addSubscriptionForm.inputs[inputIndex].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) updateSubscriptionList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		m = m.openSubscriptionEditor(filtered[m.subscriptionCursor]).(model)
		return m, nil
	case "backspace", "delete":
		selected := filtered[m.subscriptionCursor]
		m = m.beginDeleteConfirmation("subscription", selected.ID, selected.Name)
		return m, nil
	default:
		return m, nil
	}
}

func (m model) openSubscriptionEditor(selected subscription) tea.Model {
	m.screen = screenSubscriptionEdit
	m.editingSubscriptionID = selected.ID
	m.editingSubscriptionMode = m.subscriptionMode
	m.editSubscriptionForm = newEditSubscriptionForm()
	m.editSubscriptionForm.amountInput.SetValue(formatAmount(selected.AmountCents))
	m.editSubscriptionForm.paymentMethodInput.SetValue(selected.PaymentMethod)
	m.editSubscriptionForm.isActive = selected.IsActive
	m.editSubscriptionForm.periodLabel = selected.Period
	m.editSubscriptionForm.typeLabel = selected.Type
	m.editSubscriptionForm.currencyLabel = selected.Currency
	m.editSubscriptionForm.nameLabel = selected.Name
	m.editSubscriptionForm.paymentDateYearly = selected.PaymentDateYearly
	if selected.PaymentDayMonthly != nil {
		m.editSubscriptionForm.paymentDayMonthly = strconv.Itoa(*selected.PaymentDayMonthly)
	}
	m.editSubscriptionForm = m.editSubscriptionForm.focusActive()
	m.status = "editing subscription " + selected.Name
	return m
}

func (m model) updateSubscriptionEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenSubscriptionList
		m.status = "edit cancelled"
		return m, nil
	case "up", "shift+tab":
		m.editSubscriptionForm = m.editSubscriptionForm.prev()
		return m, nil
	case "down", "tab":
		m.editSubscriptionForm = m.editSubscriptionForm.next()
		return m, nil
	case " ":
		if m.editSubscriptionForm.activeField == editSubFieldIsActive {
			m.editSubscriptionForm.isActive = !m.editSubscriptionForm.isActive
			return m, nil
		}
	case "enter":
		if m.editSubscriptionForm.activeField == editSubFieldCount-1 {
			return m.saveSubscriptionEdit()
		}
		m.editSubscriptionForm = m.editSubscriptionForm.next()
		return m, nil
	}

	if m.editSubscriptionForm.activeField == editSubFieldAmount {
		var cmd tea.Cmd
		m.editSubscriptionForm.amountInput, cmd = m.editSubscriptionForm.amountInput.Update(msg)
		return m, cmd
	}
	if m.editSubscriptionForm.activeField == editSubFieldPaymentMethod {
		var cmd tea.Cmd
		m.editSubscriptionForm.paymentMethodInput, cmd = m.editSubscriptionForm.paymentMethodInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
				if err := m.db.Delete(&settingCurrency{}, m.settingsDeleteTargetID).Error; err != nil {
					m.status = "currency delete failed: " + err.Error()
					return m, nil
				}
			}

			if m.settingsDeleteTargetType == "payment_method" {
				if err := m.db.Delete(&settingPaymentMethod{}, m.settingsDeleteTargetID).Error; err != nil {
					m.status = "payment method delete failed: " + err.Error()
					return m, nil
				}
				if err := ensurePaymentMethodDefaults(m.db); err != nil {
					m.status = "payment method default repair failed: " + err.Error()
					return m, nil
				}
			}

			if m.settingsDeleteTargetType == "tax_type" {
				if err := m.db.Delete(&settingTaxType{}, m.settingsDeleteTargetID).Error; err != nil {
					m.status = "tax type delete failed: " + err.Error()
					return m, nil
				}
			}

			if m.settingsDeleteTargetType == "income_category" {
				if err := m.db.Delete(&settingIncomeCategory{}, m.settingsDeleteTargetID).Error; err != nil {
					m.status = "income category delete failed: " + err.Error()
					return m, nil
				}
			}

			if m.settingsDeleteTargetType == "expense_category" {
				if err := m.db.Delete(&settingExpenseCategory{}, m.settingsDeleteTargetID).Error; err != nil {
					m.status = "expense category delete failed: " + err.Error()
					return m, nil
				}
			}

			updated, err := loadAppSettings(m.db)
			if err != nil {
				m.status = "settings reload failed: " + err.Error()
				return m, nil
			}

			m.settings = updated
			m.accounts = sortAccountsByBaseAmount(m.accounts, m.settings)
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

			record := settingRecord{SettingID: "base_currency", SettingValue: value}
			if err := m.db.Save(&record).Error; err != nil {
				m.status = "settings save failed: " + err.Error()
				return m, nil
			}

			updated, err := loadAppSettings(m.db)
			if err != nil {
				m.status = "settings reload failed: " + err.Error()
				return m, nil
			}

			m.settings = updated
			m.accounts = sortAccountsByBaseAmount(m.accounts, m.settings)
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
			record := settingCurrency{
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

			updated, err := loadAppSettings(m.db)
			if err != nil {
				m.status = "settings reload failed: " + err.Error()
				return m, nil
			}

			m.settings = updated
			m.accounts = sortAccountsByBaseAmount(m.accounts, m.settings)
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
			record := settingPaymentMethod{
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
				if err := m.db.Model(&settingPaymentMethod{}).Where("id <> ?", record.ID).Update("is_default", false).Error; err != nil {
					m.status = "payment method save failed: " + err.Error()
					return m, nil
				}
			}

			if err := m.db.Save(&record).Error; err != nil {
				m.status = "payment method save failed: " + err.Error()
				return m, nil
			}

			var defaultCount int64
			if err := m.db.Model(&settingPaymentMethod{}).Where("is_default = ?", true).Count(&defaultCount).Error; err == nil && defaultCount == 0 {
				_ = m.db.Model(&settingPaymentMethod{}).Where("id = ?", record.ID).Update("is_default", true).Error
			}

			updated, err := loadAppSettings(m.db)
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
			record := settingTaxType{
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

			updated, err := loadAppSettings(m.db)
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
			record := settingIncomeCategory{ID: m.settingsIncomeCategoryEditingID, CategoryName: name, LastUpdatedAt: now}
			if record.ID == 0 {
				record.CreatedAt = now
			}
			if err := m.db.Save(&record).Error; err != nil {
				m.status = "income category save failed: " + err.Error()
				return m, nil
			}
			updated, err := loadAppSettings(m.db)
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
			record := settingExpenseCategory{ID: m.settingsExpenseCategoryEditingID, CategoryName: name, LastUpdatedAt: now}
			if record.ID == 0 {
				record.CreatedAt = now
			}
			if err := m.db.Save(&record).Error; err != nil {
				m.status = "expense category save failed: " + err.Error()
				return m, nil
			}
			updated, err := loadAppSettings(m.db)
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
			m.settingsCurrencyField = 0
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

func (m model) focusCurrencyFormField() model {
	m.settingsCurrencyNameInput.Blur()
	m.settingsCurrencyRateInput.Blur()
	if m.settingsCurrencyField == 0 {
		m.settingsCurrencyNameInput.Focus()
	} else {
		m.settingsCurrencyRateInput.Focus()
	}
	return m
}

func (m model) focusPaymentMethodFormField() model {
	m.settingsPaymentMethodNameInput.Blur()
	if m.settingsPaymentMethodField == 0 {
		m.settingsPaymentMethodNameInput.Focus()
	}
	return m
}

func (m model) focusTaxTypeFormField() model {
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

func settingsCursorByName(currencies []settingCurrency, name string) int {
	for index := range currencies {
		if strings.EqualFold(currencies[index].CurrencyName, name) {
			return index + 1
		}
	}
	return 0
}

func settingsPaymentMethodStartCursor(settings appSettings) int {
	return len(settings.Currencies) + 2
}

func settingsPaymentMethodAddCursor(settings appSettings) int {
	return settingsPaymentMethodStartCursor(settings) + len(settings.PaymentMethods)
}

func settingsCursorByPaymentMethodName(settings appSettings, name string) int {
	start := settingsPaymentMethodStartCursor(settings)
	for index := range settings.PaymentMethods {
		if strings.EqualFold(strings.TrimSpace(settings.PaymentMethods[index].PaymentMethodName), strings.TrimSpace(name)) {
			return start + index
		}
	}
	return settingsPaymentMethodAddCursor(settings)
}

func settingsTaxTypeStartCursor(settings appSettings) int {
	return settingsPaymentMethodAddCursor(settings) + 1
}

func settingsTaxTypeAddCursor(settings appSettings) int {
	return settingsTaxTypeStartCursor(settings) + len(settings.TaxTypes)
}

func settingsCursorByTaxTypeID(settings appSettings, id uint, country string, name string) int {
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

func settingsIncomeCategoryStartCursor(settings appSettings) int {
	return settingsTaxTypeAddCursor(settings) + 1
}

func settingsIncomeCategoryAddCursor(settings appSettings) int {
	return settingsIncomeCategoryStartCursor(settings) + len(settings.IncomeCategories)
}

func settingsCursorByIncomeCategoryName(settings appSettings, name string) int {
	start := settingsIncomeCategoryStartCursor(settings)
	for index := range settings.IncomeCategories {
		if strings.EqualFold(strings.TrimSpace(settings.IncomeCategories[index].CategoryName), strings.TrimSpace(name)) {
			return start + index
		}
	}
	return settingsIncomeCategoryAddCursor(settings)
}

func settingsExpenseCategoryStartCursor(settings appSettings) int {
	return settingsIncomeCategoryAddCursor(settings) + 1
}

func settingsExpenseCategoryAddCursor(settings appSettings) int {
	return settingsExpenseCategoryStartCursor(settings) + len(settings.ExpenseCategories)
}

func settingsCursorByExpenseCategoryName(settings appSettings, name string) int {
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

func (m model) saveAccountFromForm() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.addForm.fields[0].Value())
	description := strings.TrimSpace(m.addForm.fields[1].Value())
	currency := selectedCurrencyOption(m.addForm.currencyOptions, m.addForm.currencyIndex)
	amountRaw := strings.TrimSpace(m.addForm.fields[3].Value())

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

	newAccount := account{
		Name:          name,
		Description:   description,
		Currency:      currency,
		BalanceCents:  amount,
		LeftoverCents: amount,
		LastUpdatedAt: now,
	}

	if err := m.db.Create(&newAccount).Error; err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.accounts = append([]account{newAccount}, m.accounts...)
	m.accounts = sortAccountsByBaseAmount(m.accounts, m.settings)
	m.addForm = newAddAccountForm(currencySelectionOptions(m.settings))
	m.screen = screenMenu
	m.status = "saved account " + name
	return m, nil
}

func (m model) saveSubscriptionFromForm() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.addSubscriptionForm.inputs[0].Value())
	currency := selectedCurrencyOption(m.addSubscriptionForm.currencyOptions, m.addSubscriptionForm.currencyIndex)
	amountRaw := strings.TrimSpace(m.addSubscriptionForm.inputs[2].Value())
	paymentMethodSelection := selectedPaymentMethodOption(m.addSubscriptionForm.paymentMethodOptions, m.addSubscriptionForm.paymentMethodIndex)
	paymentMethodCustom := strings.TrimSpace(m.addSubscriptionForm.inputs[3].Value())
	paymentMethod := paymentMethodSelection
	if paymentMethodCustom != "" {
		paymentMethod = paymentMethodCustom
	}
	dayYearRaw := strings.TrimSpace(m.addSubscriptionForm.inputs[4].Value())
	dayMonthRaw := strings.TrimSpace(m.addSubscriptionForm.inputs[5].Value())

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
	newSubscription := subscription{
		Name:              name,
		Currency:          currency,
		AmountCents:       amount,
		Period:            m.addSubscriptionForm.periodOptions[m.addSubscriptionForm.periodIndex],
		PaymentMethod:     paymentMethod,
		Type:              m.addSubscriptionForm.typeOptions[m.addSubscriptionForm.typeIndex],
		IsActive:          m.addSubscriptionForm.isActive,
		PaymentDateYearly: dateYearly,
		PaymentDayMonthly: dayMonthly,
		LastUpdatedAt:     now,
	}

	if err := m.db.Create(&newSubscription).Error; err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.subscriptions = append([]subscription{newSubscription}, m.subscriptions...)
	m.subscriptions = sortSubscriptionsByAmount(m.subscriptions)
	m.addSubscriptionForm = newAddSubscriptionForm(currencySelectionOptions(m.settings), paymentMethodSelectionOptions(m.settings))
	m.screen = screenSubscriptionList
	m.subscriptionMode = subscriptionListAll
	m.subscriptionCursor = 0
	m.status = "saved subscription " + name
	return m, nil
}

func (m model) saveSubscriptionEdit() (tea.Model, tea.Cmd) {
	if m.editingSubscriptionID == 0 {
		m.status = "no subscription selected"
		return m, nil
	}

	amountRaw := strings.TrimSpace(m.editSubscriptionForm.amountInput.Value())
	paymentMethod := strings.TrimSpace(m.editSubscriptionForm.paymentMethodInput.Value())
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
	selected.IsActive = m.editSubscriptionForm.isActive
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

func (m model) beginEditAmount() tea.Model {
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
	m.editInput.Focus()
	m.screen = screenEditAmount
	m.status = "editing amount only for " + current.Name
	return m
}

func (m model) saveAmount() (tea.Model, tea.Cmd) {
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
	if err := m.db.Model(&account{}).Where("id = ?", selected.ID).Updates(map[string]any{"balance_cents": amount, "leftover_cents": amount, "last_updated_at": now}).Error; err != nil {
		m.status = "update failed: " + err.Error()
		return m, nil
	}

	selected.BalanceCents = amount
	selected.LeftoverCents = amount
	selected.LastUpdatedAt = now
	m.accounts[m.cursor] = selected
	m.accounts = sortAccountsByBaseAmount(m.accounts, m.settings)
	m.cursor = findAccountIndex(m.accounts, selected.ID)
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.screen = screenAccountTable
	m.status = "updated amount for " + selected.Name
	return m, nil
}

func (m model) deleteSelectedAccount() (tea.Model, tea.Cmd) {
	if m.cursor < 0 || m.cursor >= len(m.accounts) {
		m.status = "no account selected"
		return m, nil
	}

	selected := m.accounts[m.cursor]
	if err := m.db.Delete(&account{}, selected.ID).Error; err != nil {
		m.status = "delete failed: " + err.Error()
		return m, nil
	}

	m.accounts = append(m.accounts[:m.cursor], m.accounts[m.cursor+1:]...)
	if m.cursor >= len(m.accounts) && m.cursor > 0 {
		m.cursor--
	}

	m.status = "deleted account " + selected.Name
	if len(m.accounts) == 0 {
		m.screen = screenMenu
	}

	return m, nil
}

func (m model) deleteSelectedSubscription(filtered []subscription) (tea.Model, tea.Cmd) {
	if m.subscriptionCursor < 0 || m.subscriptionCursor >= len(filtered) {
		m.status = "no subscription selected"
		return m, nil
	}

	selected := filtered[m.subscriptionCursor]
	if err := m.db.Delete(&subscription{}, selected.ID).Error; err != nil {
		m.status = "delete failed: " + err.Error()
		return m, nil
	}

	index := -1
	for i := range m.subscriptions {
		if m.subscriptions[i].ID == selected.ID {
			index = i
			break
		}
	}

	if index >= 0 {
		m.subscriptions = append(m.subscriptions[:index], m.subscriptions[index+1:]...)
	}

	filteredAfter := m.filteredSubscriptions()
	if len(filteredAfter) == 0 {
		m.subscriptionCursor = 0
	} else if m.subscriptionCursor >= len(filteredAfter) {
		m.subscriptionCursor = len(filteredAfter) - 1
	}

	m.status = "deleted subscription " + selected.Name
	return m, nil
}

func (m model) beginDeleteConfirmation(targetType string, targetID uint, targetName string) model {
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

func (m model) clearDeleteConfirmation(status string) model {
	m.deleteConfirmActive = false
	m.deleteConfirmType = ""
	m.deleteConfirmID = 0
	m.deleteConfirmName = ""
	if strings.TrimSpace(status) != "" {
		m.status = status
	}
	return m
}

func (m model) handleDeleteConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
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

func (m model) applyConfirmedDelete() (tea.Model, tea.Cmd) {
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

func (m model) confirmDeleteAccount() (tea.Model, tea.Cmd) {
	index := findAccountIndex(m.accounts, m.deleteConfirmID)
	if index < 0 {
		m = m.clearDeleteConfirmation("account not found")
		return m, nil
	}

	selected := m.accounts[index]
	if err := m.db.Delete(&account{}, selected.ID).Error; err != nil {
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

func (m model) confirmDeleteSubscription() (tea.Model, tea.Cmd) {
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
	if err := m.db.Delete(&subscription{}, selected.ID).Error; err != nil {
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

func (m model) confirmDeleteCashflow() (tea.Model, tea.Cmd) {
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
	if err := m.db.Delete(&cashflowEntry{}, selected.ID).Error; err != nil {
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

func (m model) confirmDeleteDebt() (tea.Model, tea.Cmd) {
	index := m.findDebtIndex(m.deleteConfirmID)
	if index < 0 {
		m = m.clearDeleteConfirmation("debt not found")
		return m, nil
	}

	selected := m.debts[index]
	if err := m.db.Where("debt_id = ?", selected.ID).Delete(&debtLog{}).Error; err != nil {
		m = m.clearDeleteConfirmation("delete failed: " + err.Error())
		return m, nil
	}
	if err := m.db.Delete(&debt{}, selected.ID).Error; err != nil {
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

func (m model) confirmDeleteGoal() (tea.Model, tea.Cmd) {
	index := m.findGoalIndex(m.deleteConfirmID)
	if index < 0 {
		m = m.clearDeleteConfirmation("goal not found")
		return m, nil
	}

	selected := m.goals[index]
	if err := m.db.Where("goal_id = ?", selected.ID).Delete(&goalLog{}).Error; err != nil {
		m = m.clearDeleteConfirmation("delete failed: " + err.Error())
		return m, nil
	}
	if err := m.db.Delete(&goal{}, selected.ID).Error; err != nil {
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

func (m model) confirmDeleteTax() (tea.Model, tea.Cmd) {
	index := m.findTaxIndex(m.deleteConfirmID)
	if index < 0 {
		m = m.clearDeleteConfirmation("tax not found")
		return m, nil
	}

	selected := m.taxes[index]
	if err := m.db.Where("tax_id = ?", selected.ID).Delete(&taxLog{}).Error; err != nil {
		m = m.clearDeleteConfirmation("delete failed: " + err.Error())
		return m, nil
	}
	if err := m.db.Delete(&tax{}, selected.ID).Error; err != nil {
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

func (m model) confirmDeleteInvoice() (tea.Model, tea.Cmd) {
	index := m.findInvoiceIndex(m.deleteConfirmID)
	if index < 0 {
		m = m.clearDeleteConfirmation("invoice not found")
		return m, nil
	}

	selected := m.invoices[index]
	if err := m.db.Delete(&invoice{}, selected.ID).Error; err != nil {
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

func (m model) updateDebtNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		return m, nil
	case "up", "shift+tab":
		m.addDebtForm = m.addDebtForm.prev()
		return m, nil
	case "down", "tab":
		m.addDebtForm = m.addDebtForm.next()
		return m, nil
	case "left":
		if m.addDebtForm.active == debtFieldDirection {
			m.addDebtForm.isOwedToUser = false
		}
		if m.addDebtForm.active == debtFieldCurrency && m.addDebtForm.currencyIndex > 0 {
			m.addDebtForm.currencyIndex--
		}
		return m, nil
	case "right":
		if m.addDebtForm.active == debtFieldDirection {
			m.addDebtForm.isOwedToUser = true
		}
		if m.addDebtForm.active == debtFieldCurrency && m.addDebtForm.currencyIndex < len(m.addDebtForm.currencyOptions)-1 {
			m.addDebtForm.currencyIndex++
		}
		return m, nil
	case " ":
		if m.addDebtForm.active == debtFieldDirection {
			m.addDebtForm.isOwedToUser = !m.addDebtForm.isOwedToUser
			return m, nil
		}
	case "enter":
		if m.addDebtForm.active == debtFieldCount-1 {
			return m.saveDebtFromForm()
		}
		m.addDebtForm = m.addDebtForm.next()
		return m, nil
	}

	if inputIndex := m.addDebtForm.inputIndexForField(m.addDebtForm.active); inputIndex >= 0 {
		var cmd tea.Cmd
		m.addDebtForm.inputs[inputIndex], cmd = m.addDebtForm.inputs[inputIndex].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) updateDebtList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		m = m.openDebtEditor(filtered[m.debtCursor]).(model)
		return m, nil
	case "backspace", "delete":
		selected := filtered[m.debtCursor]
		m = m.beginDeleteConfirmation("debt", selected.ID, selected.Peer)
		return m, nil
	default:
		return m, nil
	}
}

func (m model) openDebtEditor(selected debt) tea.Model {
	m.screen = screenDebtEdit
	m.editingDebtID = selected.ID
	m.editDebtForm = newEditDebtForm()
	m.editDebtForm.amountInput.SetValue(formatAmount(selected.AmountCents))
	m.editDebtForm.amountPaidInput.SetValue(formatAmount(selected.AmountPaidCents))
	m.editDebtForm.debtCreatedInput.SetValue(selected.DebtCreatedAt.Local().Format("02.01.2006"))
	if selected.DueDate != nil {
		m.editDebtForm.dueDateInput.SetValue(selected.DueDate.Local().Format("02.01.2006"))
	}
	m.editDebtForm.commentInput.SetValue(selected.Comment)
	m.editDebtForm.logDateInput.SetValue(time.Now().Format("02.01.2006"))
	m.editDebtForm.logCommentInput.SetValue("")
	m.editDebtForm.peerLabel = selected.Peer
	m.editDebtForm.currencyLabel = selected.Currency
	m.editDebtForm.directionLabel = debtDirectionLabel(selected.IsOwedToUser)
	m.editDebtForm = m.editDebtForm.focusActive()

	var logs []debtLog
	if err := m.db.Where("debt_id = ?", selected.ID).Order("created_at desc, id desc").Limit(20).Find(&logs).Error; err == nil {
		m.debtLogs = logs
	} else {
		m.debtLogs = nil
	}

	m.status = "editing debt " + selected.Peer
	return m
}

func (m model) updateDebtEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenDebtList
		m.status = "debt edit cancelled"
		return m, nil
	case "up", "shift+tab":
		m.editDebtForm = m.editDebtForm.prev()
		return m, nil
	case "down", "tab":
		m.editDebtForm = m.editDebtForm.next()
		return m, nil
	case "enter":
		if m.editDebtForm.activeField == editDebtFieldLogComment {
			return m.applyDebtLogDelta()
		}
		if m.editDebtForm.activeField == editDebtFieldComment {
			return m.saveDebtEdit()
		}
		m.editDebtForm = m.editDebtForm.next()
		return m, nil
	}

	var cmd tea.Cmd
	switch m.editDebtForm.activeField {
	case editDebtFieldAmount:
		m.editDebtForm.amountInput, cmd = m.editDebtForm.amountInput.Update(msg)
	case editDebtFieldAmountPaid:
		m.editDebtForm.amountPaidInput, cmd = m.editDebtForm.amountPaidInput.Update(msg)
	case editDebtFieldDebtCreated:
		m.editDebtForm.debtCreatedInput, cmd = m.editDebtForm.debtCreatedInput.Update(msg)
	case editDebtFieldDueDate:
		m.editDebtForm.dueDateInput, cmd = m.editDebtForm.dueDateInput.Update(msg)
	case editDebtFieldComment:
		m.editDebtForm.commentInput, cmd = m.editDebtForm.commentInput.Update(msg)
	case editDebtFieldLogDelta:
		m.editDebtForm.logDeltaInput, cmd = m.editDebtForm.logDeltaInput.Update(msg)
	case editDebtFieldLogDate:
		m.editDebtForm.logDateInput, cmd = m.editDebtForm.logDateInput.Update(msg)
	case editDebtFieldLogComment:
		m.editDebtForm.logCommentInput, cmd = m.editDebtForm.logCommentInput.Update(msg)
	}
	return m, cmd
}

func (m model) saveDebtFromForm() (tea.Model, tea.Cmd) {
	peer := strings.TrimSpace(m.addDebtForm.inputs[0].Value())
	currency := selectedCurrencyOption(m.addDebtForm.currencyOptions, m.addDebtForm.currencyIndex)
	amountRaw := strings.TrimSpace(m.addDebtForm.inputs[1].Value())
	amountPaidRaw := strings.TrimSpace(m.addDebtForm.inputs[2].Value())
	debtCreatedRaw := strings.TrimSpace(m.addDebtForm.inputs[3].Value())
	dueRaw := strings.TrimSpace(m.addDebtForm.inputs[4].Value())
	comment := strings.TrimSpace(m.addDebtForm.inputs[5].Value())

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
	newDebt := debt{
		Peer:            peer,
		Currency:        currency,
		AmountCents:     amount,
		AmountPaidCents: amountPaid,
		IsOwedToUser:    m.addDebtForm.isOwedToUser,
		DebtCreatedAt:   debtCreatedAt,
		DueDate:         dueDate,
		Comment:         comment,
		LastUpdatedAt:   now,
	}

	if err := m.db.Create(&newDebt).Error; err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.debts = append([]debt{newDebt}, m.debts...)
	m.addDebtForm = newAddDebtForm(currencySelectionOptions(m.settings))
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

func (m model) saveDebtEdit() (tea.Model, tea.Cmd) {
	index := m.findDebtIndex(m.editingDebtID)
	if index < 0 {
		m.status = "debt not found"
		return m, nil
	}

	amount, err := parseAmountCents(strings.TrimSpace(m.editDebtForm.amountInput.Value()))
	if err != nil {
		m.status = "amount error: " + err.Error()
		return m, nil
	}
	amountPaid, err := parseAmountCents(strings.TrimSpace(m.editDebtForm.amountPaidInput.Value()))
	if err != nil {
		m.status = "amount paid error: " + err.Error()
		return m, nil
	}
	if amountPaid > amount {
		m.status = "amount paid cannot be more than amount"
		return m, nil
	}

	debtCreatedAt, err := parseRequiredDate(strings.TrimSpace(m.editDebtForm.debtCreatedInput.Value()))
	if err != nil {
		m.status = err.Error()
		return m, nil
	}
	dueDate, err := parseOptionalDatePointer(strings.TrimSpace(m.editDebtForm.dueDateInput.Value()))
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	selected := m.debts[index]
	selected.AmountCents = amount
	selected.AmountPaidCents = amountPaid
	selected.DebtCreatedAt = debtCreatedAt
	selected.DueDate = dueDate
	selected.Comment = strings.TrimSpace(m.editDebtForm.commentInput.Value())
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

func (m model) applyDebtLogDelta() (tea.Model, tea.Cmd) {
	index := m.findDebtIndex(m.editingDebtID)
	if index < 0 {
		m.status = "debt not found"
		return m, nil
	}

	delta, err := parseSignedAmountCents(strings.TrimSpace(m.editDebtForm.logDeltaInput.Value()))
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

	entryTime, err := parseLogDateOrToday(strings.TrimSpace(m.editDebtForm.logDateInput.Value()))
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

	note := strings.TrimSpace(m.editDebtForm.logCommentInput.Value())
	if note == "" {
		note = "manual paid adjustment"
	}
	entry := debtLog{
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
	m.debtLogs = append([]debtLog{entry}, m.debtLogs...)
	m.editDebtForm.amountPaidInput.SetValue(formatAmount(selected.AmountPaidCents))
	m.editDebtForm.logDeltaInput.SetValue("")
	m.editDebtForm.logDateInput.SetValue(time.Now().Format("02.01.2006"))
	m.editDebtForm.logCommentInput.SetValue("")
	m.status = "applied log delta"
	return m, nil
}

func (m model) findDebtIndex(id uint) int {
	for i := range m.debts {
		if m.debts[i].ID == id {
			return i
		}
	}
	return -1
}

func (m model) filteredDebts() []debt {
	filtered := make([]debt, 0, len(m.debts))
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

func (m model) updateGoalNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		return m, nil
	case "up", "shift+tab":
		m.addGoalForm = m.addGoalForm.prev()
		return m, nil
	case "down", "tab":
		m.addGoalForm = m.addGoalForm.next()
		return m, nil
	case "left":
		if m.addGoalForm.active == goalFieldCurrency && m.addGoalForm.currencyIndex > 0 {
			m.addGoalForm.currencyIndex--
		}
		return m, nil
	case "right":
		if m.addGoalForm.active == goalFieldCurrency && m.addGoalForm.currencyIndex < len(m.addGoalForm.currencyOptions)-1 {
			m.addGoalForm.currencyIndex++
		}
		return m, nil
	case "enter":
		if m.addGoalForm.active == goalFieldCount-1 {
			return m.saveGoalFromForm()
		}
		m.addGoalForm = m.addGoalForm.next()
		return m, nil
	}

	if inputIndex := m.addGoalForm.inputIndexForField(m.addGoalForm.active); inputIndex >= 0 {
		var cmd tea.Cmd
		m.addGoalForm.inputs[inputIndex], cmd = m.addGoalForm.inputs[inputIndex].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) updateGoalList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		m = m.openGoalEditor(filtered[m.goalCursor]).(model)
		return m, nil
	case "backspace", "delete":
		selected := filtered[m.goalCursor]
		m = m.beginDeleteConfirmation("goal", selected.ID, selected.Name)
		return m, nil
	default:
		return m, nil
	}
}

func (m model) openGoalEditor(selected goal) tea.Model {
	m.screen = screenGoalEdit
	m.editingGoalID = selected.ID
	m.editGoalForm = newEditGoalForm()
	m.editGoalForm.targetAmountInput.SetValue(formatAmount(selected.TargetAmountCents))
	m.editGoalForm.accumulatedAmountInput.SetValue(formatAmount(selected.AmountAccumulatedCents))
	m.editGoalForm.dateStartedInput.SetValue(selected.DateStartedAt.Local().Format("02.01.2006"))
	if selected.TargetDate != nil {
		m.editGoalForm.targetDateInput.SetValue(selected.TargetDate.Local().Format("02.01.2006"))
	}
	m.editGoalForm.descriptionInput.SetValue(selected.Description)
	m.editGoalForm.logDateInput.SetValue(time.Now().Format("02.01.2006"))
	m.editGoalForm.logCommentInput.SetValue("")
	m.editGoalForm.nameLabel = selected.Name
	m.editGoalForm.currencyLabel = selected.Currency
	m.editGoalForm = m.editGoalForm.focusActive()

	var logs []goalLog
	if err := m.db.Where("goal_id = ?", selected.ID).Order("created_at desc, id desc").Limit(20).Find(&logs).Error; err == nil {
		m.goalLogs = logs
	} else {
		m.goalLogs = nil
	}

	m.status = "editing goal " + selected.Name
	return m
}

func (m model) updateGoalEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenGoalList
		m.status = "goal edit cancelled"
		return m, nil
	case "up", "shift+tab":
		m.editGoalForm = m.editGoalForm.prev()
		return m, nil
	case "down", "tab":
		m.editGoalForm = m.editGoalForm.next()
		return m, nil
	case "enter":
		if m.editGoalForm.activeField == editGoalFieldLogComment {
			return m.applyGoalLogDelta()
		}
		if m.editGoalForm.activeField == editGoalFieldDescription {
			return m.saveGoalEdit()
		}
		m.editGoalForm = m.editGoalForm.next()
		return m, nil
	}

	var cmd tea.Cmd
	switch m.editGoalForm.activeField {
	case editGoalFieldTargetAmount:
		m.editGoalForm.targetAmountInput, cmd = m.editGoalForm.targetAmountInput.Update(msg)
	case editGoalFieldAccumulated:
		m.editGoalForm.accumulatedAmountInput, cmd = m.editGoalForm.accumulatedAmountInput.Update(msg)
	case editGoalFieldDateStarted:
		m.editGoalForm.dateStartedInput, cmd = m.editGoalForm.dateStartedInput.Update(msg)
	case editGoalFieldTargetDate:
		m.editGoalForm.targetDateInput, cmd = m.editGoalForm.targetDateInput.Update(msg)
	case editGoalFieldDescription:
		m.editGoalForm.descriptionInput, cmd = m.editGoalForm.descriptionInput.Update(msg)
	case editGoalFieldLogDelta:
		m.editGoalForm.logDeltaInput, cmd = m.editGoalForm.logDeltaInput.Update(msg)
	case editGoalFieldLogDate:
		m.editGoalForm.logDateInput, cmd = m.editGoalForm.logDateInput.Update(msg)
	case editGoalFieldLogComment:
		m.editGoalForm.logCommentInput, cmd = m.editGoalForm.logCommentInput.Update(msg)
	}
	return m, cmd
}

func (m model) saveGoalFromForm() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.addGoalForm.inputs[0].Value())
	currency := selectedCurrencyOption(m.addGoalForm.currencyOptions, m.addGoalForm.currencyIndex)
	targetRaw := strings.TrimSpace(m.addGoalForm.inputs[1].Value())
	accumulatedRaw := strings.TrimSpace(m.addGoalForm.inputs[2].Value())
	description := strings.TrimSpace(m.addGoalForm.inputs[3].Value())
	dateStartedRaw := strings.TrimSpace(m.addGoalForm.inputs[4].Value())
	targetDateRaw := strings.TrimSpace(m.addGoalForm.inputs[5].Value())

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
	newGoal := goal{
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

	m.goals = append([]goal{newGoal}, m.goals...)
	m.addGoalForm = newAddGoalForm(currencySelectionOptions(m.settings))
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

func (m model) saveGoalEdit() (tea.Model, tea.Cmd) {
	index := m.findGoalIndex(m.editingGoalID)
	if index < 0 {
		m.status = "goal not found"
		return m, nil
	}

	target, err := parseAmountCents(strings.TrimSpace(m.editGoalForm.targetAmountInput.Value()))
	if err != nil {
		m.status = "target amount error: " + err.Error()
		return m, nil
	}
	if target <= 0 {
		m.status = "target amount must be greater than zero"
		return m, nil
	}

	accumulated, err := parseAmountCents(strings.TrimSpace(m.editGoalForm.accumulatedAmountInput.Value()))
	if err != nil {
		m.status = "accumulated amount error: " + err.Error()
		return m, nil
	}
	if accumulated > target {
		m.status = "accumulated amount cannot be more than target"
		return m, nil
	}

	dateStarted, err := parseRequiredDate(strings.TrimSpace(m.editGoalForm.dateStartedInput.Value()))
	if err != nil {
		m.status = err.Error()
		return m, nil
	}
	targetDate, err := parseOptionalDatePointer(strings.TrimSpace(m.editGoalForm.targetDateInput.Value()))
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	selected := m.goals[index]
	selected.TargetAmountCents = target
	selected.AmountAccumulatedCents = accumulated
	selected.DateStartedAt = dateStarted
	selected.TargetDate = targetDate
	selected.Description = strings.TrimSpace(m.editGoalForm.descriptionInput.Value())
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

func (m model) applyGoalLogDelta() (tea.Model, tea.Cmd) {
	index := m.findGoalIndex(m.editingGoalID)
	if index < 0 {
		m.status = "goal not found"
		return m, nil
	}

	delta, err := parseSignedAmountCents(strings.TrimSpace(m.editGoalForm.logDeltaInput.Value()))
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

	entryTime, err := parseLogDateOrToday(strings.TrimSpace(m.editGoalForm.logDateInput.Value()))
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

	note := strings.TrimSpace(m.editGoalForm.logCommentInput.Value())
	if note == "" {
		note = "manual accumulated adjustment"
	}
	entry := goalLog{
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
	m.goalLogs = append([]goalLog{entry}, m.goalLogs...)
	m.editGoalForm.accumulatedAmountInput.SetValue(formatAmount(selected.AmountAccumulatedCents))
	m.editGoalForm.logDeltaInput.SetValue("")
	m.editGoalForm.logDateInput.SetValue(time.Now().Format("02.01.2006"))
	m.editGoalForm.logCommentInput.SetValue("")
	m.status = "applied goal log delta"
	return m, nil
}

func (m model) findGoalIndex(id uint) int {
	for i := range m.goals {
		if m.goals[i].ID == id {
			return i
		}
	}
	return -1
}

func (m model) filteredGoals() []goal {
	filtered := make([]goal, 0, len(m.goals))
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

func (m model) goalProgressTotalsBase(items []goal) (accumulated int64, target int64) {
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

func (m model) updateTaxNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.addTaxForm.taxTypeOptions) == 0 {
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
		m.addTaxForm = m.addTaxForm.prev()
		return m, nil
	case "down", "tab":
		m.addTaxForm = m.addTaxForm.next()
		return m, nil
	case "left":
		if m.addTaxForm.active == taxFieldTaxType && m.addTaxForm.taxTypeIndex > 0 {
			m.addTaxForm.taxTypeIndex--
		}
		return m, nil
	case "right":
		if m.addTaxForm.active == taxFieldTaxType && m.addTaxForm.taxTypeIndex < len(m.addTaxForm.taxTypeOptions)-1 {
			m.addTaxForm.taxTypeIndex++
		}
		return m, nil
	case "enter":
		if m.addTaxForm.active == taxFieldCount-1 {
			return m.saveTaxFromForm()
		}
		m.addTaxForm = m.addTaxForm.next()
		return m, nil
	}

	if inputIndex := m.addTaxForm.inputIndexForField(m.addTaxForm.active); inputIndex >= 0 {
		var cmd tea.Cmd
		m.addTaxForm.inputs[inputIndex], cmd = m.addTaxForm.inputs[inputIndex].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) updateTaxList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		m = m.openTaxEditor(filtered[m.taxCursor]).(model)
		return m, nil
	case "backspace", "delete":
		selected := filtered[m.taxCursor]
		m = m.beginDeleteConfirmation("tax", selected.ID, selected.TaxCountry+" / "+selected.TaxTypeName)
		return m, nil
	default:
		return m, nil
	}
}

func (m model) openTaxEditor(selected tax) tea.Model {
	m.screen = screenTaxEdit
	m.editingTaxID = selected.ID
	m.editTaxForm = newEditTaxForm()
	m.editTaxForm.amountDueInput.SetValue(formatAmount(selected.AmountDueCents))
	m.editTaxForm.amountPaidInput.SetValue(formatAmount(selected.AmountPaidCents))
	m.editTaxForm.periodInput.SetValue(selected.Period)
	if selected.DueDate != nil {
		m.editTaxForm.dueDateInput.SetValue(selected.DueDate.Local().Format("02.01.2006"))
	} else {
		m.editTaxForm.dueDateInput.SetValue("")
	}
	m.editTaxForm.commentInput.SetValue(selected.Comment)
	m.editTaxForm.logDateInput.SetValue(time.Now().Format("02.01.2006"))
	m.editTaxForm.logCommentInput.SetValue("")
	m.editTaxForm.taxTypeLabel = selected.TaxTypeName
	m.editTaxForm.countryLabel = selected.TaxCountry
	m.editTaxForm = m.editTaxForm.focusActive()

	var logs []taxLog
	if err := m.db.Where("tax_id = ?", selected.ID).Order("created_at desc, id desc").Limit(20).Find(&logs).Error; err == nil {
		m.taxLogs = logs
	} else {
		m.taxLogs = nil
	}

	m.status = "editing tax " + selected.TaxCountry + " / " + selected.TaxTypeName
	return m
}

func (m model) updateTaxEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenTaxList
		m.status = "tax edit cancelled"
		return m, nil
	case "up", "shift+tab":
		m.editTaxForm = m.editTaxForm.prev()
		return m, nil
	case "down", "tab":
		m.editTaxForm = m.editTaxForm.next()
		return m, nil
	case "enter":
		if m.editTaxForm.activeField == editTaxFieldLogComment {
			return m.applyTaxLogDelta()
		}
		if m.editTaxForm.activeField == editTaxFieldComment {
			return m.saveTaxEdit()
		}
		m.editTaxForm = m.editTaxForm.next()
		return m, nil
	}

	var cmd tea.Cmd
	switch m.editTaxForm.activeField {
	case editTaxFieldAmountDue:
		m.editTaxForm.amountDueInput, cmd = m.editTaxForm.amountDueInput.Update(msg)
	case editTaxFieldAmountPaid:
		m.editTaxForm.amountPaidInput, cmd = m.editTaxForm.amountPaidInput.Update(msg)
	case editTaxFieldPeriod:
		m.editTaxForm.periodInput, cmd = m.editTaxForm.periodInput.Update(msg)
	case editTaxFieldDueDate:
		m.editTaxForm.dueDateInput, cmd = m.editTaxForm.dueDateInput.Update(msg)
	case editTaxFieldComment:
		m.editTaxForm.commentInput, cmd = m.editTaxForm.commentInput.Update(msg)
	case editTaxFieldLogDelta:
		m.editTaxForm.logDeltaInput, cmd = m.editTaxForm.logDeltaInput.Update(msg)
	case editTaxFieldLogDate:
		m.editTaxForm.logDateInput, cmd = m.editTaxForm.logDateInput.Update(msg)
	case editTaxFieldLogComment:
		m.editTaxForm.logCommentInput, cmd = m.editTaxForm.logCommentInput.Update(msg)
	}
	return m, cmd
}

func (m model) saveTaxFromForm() (tea.Model, tea.Cmd) {
	if len(m.addTaxForm.taxTypeOptions) == 0 {
		m.status = "no tax types configured; add one in settings"
		return m, nil
	}

	taxTypeIndex := m.addTaxForm.taxTypeIndex
	if taxTypeIndex < 0 || taxTypeIndex >= len(m.addTaxForm.taxTypeOptions) {
		taxTypeIndex = 0
	}
	selectedType := m.addTaxForm.taxTypeOptions[taxTypeIndex]
	amountDueRaw := strings.TrimSpace(m.addTaxForm.inputs[0].Value())
	amountPaidRaw := strings.TrimSpace(m.addTaxForm.inputs[1].Value())
	period := strings.TrimSpace(m.addTaxForm.inputs[2].Value())
	dueDateRaw := strings.TrimSpace(m.addTaxForm.inputs[3].Value())
	comment := strings.TrimSpace(m.addTaxForm.inputs[4].Value())

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
	newTax := tax{
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

	m.taxes = append([]tax{newTax}, m.taxes...)
	m.addTaxForm = newAddTaxForm(m.settings.TaxTypes)
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

func (m model) saveTaxEdit() (tea.Model, tea.Cmd) {
	index := m.findTaxIndex(m.editingTaxID)
	if index < 0 {
		m.status = "tax not found"
		return m, nil
	}

	amountDue, err := parseAmountCents(strings.TrimSpace(m.editTaxForm.amountDueInput.Value()))
	if err != nil {
		m.status = "amount due error: " + err.Error()
		return m, nil
	}
	if amountDue <= 0 {
		m.status = "amount due must be greater than zero"
		return m, nil
	}

	amountPaid, err := parseAmountCents(strings.TrimSpace(m.editTaxForm.amountPaidInput.Value()))
	if err != nil {
		m.status = "amount paid error: " + err.Error()
		return m, nil
	}

	period := strings.TrimSpace(m.editTaxForm.periodInput.Value())
	if period == "" {
		m.status = "period is required"
		return m, nil
	}

	dueDate, err := parseOptionalDatePointer(strings.TrimSpace(m.editTaxForm.dueDateInput.Value()))
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	selected := m.taxes[index]
	selected.AmountDueCents = amountDue
	selected.AmountPaidCents = amountPaid
	selected.Period = period
	selected.DueDate = dueDate
	selected.Comment = strings.TrimSpace(m.editTaxForm.commentInput.Value())
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

func (m model) applyTaxLogDelta() (tea.Model, tea.Cmd) {
	index := m.findTaxIndex(m.editingTaxID)
	if index < 0 {
		m.status = "tax not found"
		return m, nil
	}

	delta, err := parseSignedAmountCents(strings.TrimSpace(m.editTaxForm.logDeltaInput.Value()))
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

	entryTime, err := parseLogDateOrToday(strings.TrimSpace(m.editTaxForm.logDateInput.Value()))
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

	note := strings.TrimSpace(m.editTaxForm.logCommentInput.Value())
	if note == "" {
		note = "manual tax paid adjustment"
	}
	entry := taxLog{
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
	m.taxLogs = append([]taxLog{entry}, m.taxLogs...)
	m.editTaxForm.amountPaidInput.SetValue(formatAmount(selected.AmountPaidCents))
	m.editTaxForm.logDeltaInput.SetValue("")
	m.editTaxForm.logDateInput.SetValue(time.Now().Format("02.01.2006"))
	m.editTaxForm.logCommentInput.SetValue("")
	m.status = "applied tax log delta"
	return m, nil
}

func (m model) findTaxIndex(id uint) int {
	for i := range m.taxes {
		if m.taxes[i].ID == id {
			return i
		}
	}
	return -1
}

func (m model) filteredTaxes() []tax {
	filtered := make([]tax, 0, len(m.taxes))
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

func taxProgressTotals(items []tax) (paid int64, total int64) {
	for _, item := range items {
		itemTotal := item.AmountDueCents
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

func (m model) updateInvoiceNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		return m, nil
	case "up", "shift+tab":
		m.addInvoiceForm = m.addInvoiceForm.prev()
		return m, nil
	case "down", "tab":
		m.addInvoiceForm = m.addInvoiceForm.next()
		return m, nil
	case "left":
		if m.addInvoiceForm.active == invoiceFieldType {
			m.addInvoiceForm.isIncoming = true
		}
		if m.addInvoiceForm.active == invoiceFieldCurrency && m.addInvoiceForm.currencyIndex > 0 {
			m.addInvoiceForm.currencyIndex--
		}
		return m, nil
	case "right":
		if m.addInvoiceForm.active == invoiceFieldType {
			m.addInvoiceForm.isIncoming = false
		}
		if m.addInvoiceForm.active == invoiceFieldCurrency && m.addInvoiceForm.currencyIndex < len(m.addInvoiceForm.currencyOptions)-1 {
			m.addInvoiceForm.currencyIndex++
		}
		return m, nil
	case " ":
		if m.addInvoiceForm.active == invoiceFieldPaid {
			m.addInvoiceForm.paid = !m.addInvoiceForm.paid
			return m, nil
		}
	case "enter":
		if m.addInvoiceForm.active == invoiceFieldCount-1 {
			return m.saveInvoiceFromForm()
		}
		m.addInvoiceForm = m.addInvoiceForm.next()
		return m, nil
	}

	if inputIndex := m.addInvoiceForm.inputIndexForField(m.addInvoiceForm.active); inputIndex >= 0 {
		var cmd tea.Cmd
		m.addInvoiceForm.inputs[inputIndex], cmd = m.addInvoiceForm.inputs[inputIndex].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) updateInvoiceList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if updatedModel, cmd, handled := m.handleDeleteConfirmation(msg); handled {
		return updatedModel, cmd
	}

	filtered := m.filteredInvoices()
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
		if m.invoiceCursor > 0 {
			m.invoiceCursor--
		}
		return m, nil
	case "down":
		if m.invoiceCursor < len(filtered)-1 {
			m.invoiceCursor++
		}
		return m, nil
	case "enter":
		m = m.openInvoiceEditor(filtered[m.invoiceCursor]).(model)
		return m, nil
	case "backspace", "delete":
		selected := filtered[m.invoiceCursor]
		m = m.beginDeleteConfirmation("invoice", selected.ID, selected.Title)
		return m, nil
	default:
		return m, nil
	}
}

func (m model) updateInvoiceEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenInvoiceList
		m.status = "invoice edit cancelled"
		return m, nil
	case "up", "shift+tab":
		m.editInvoiceForm = m.editInvoiceForm.prev()
		return m, nil
	case "down", "tab":
		m.editInvoiceForm = m.editInvoiceForm.next()
		return m, nil
	case "left":
		if m.editInvoiceForm.activeField == invoiceFieldType {
			m.editInvoiceForm.isIncoming = true
		}
		if m.editInvoiceForm.activeField == invoiceFieldCurrency && m.editInvoiceForm.currencyIndex > 0 {
			m.editInvoiceForm.currencyIndex--
		}
		return m, nil
	case "right":
		if m.editInvoiceForm.activeField == invoiceFieldType {
			m.editInvoiceForm.isIncoming = false
		}
		if m.editInvoiceForm.activeField == invoiceFieldCurrency && m.editInvoiceForm.currencyIndex < len(m.editInvoiceForm.currencyOptions)-1 {
			m.editInvoiceForm.currencyIndex++
		}
		return m, nil
	case " ":
		if m.editInvoiceForm.activeField == invoiceFieldPaid {
			m.editInvoiceForm.paid = !m.editInvoiceForm.paid
			return m, nil
		}
	case "enter":
		if m.editInvoiceForm.activeField == invoiceFieldCount-1 {
			return m.saveInvoiceEdit()
		}
		m.editInvoiceForm = m.editInvoiceForm.next()
		return m, nil
	}

	var cmd tea.Cmd
	switch m.editInvoiceForm.activeField {
	case invoiceFieldTitle:
		m.editInvoiceForm.titleInput, cmd = m.editInvoiceForm.titleInput.Update(msg)
	case invoiceFieldAmount:
		m.editInvoiceForm.amountInput, cmd = m.editInvoiceForm.amountInput.Update(msg)
	case invoiceFieldPeer:
		m.editInvoiceForm.peerInput, cmd = m.editInvoiceForm.peerInput.Update(msg)
	case invoiceFieldInvoiceDate:
		m.editInvoiceForm.invoiceDateInput, cmd = m.editInvoiceForm.invoiceDateInput.Update(msg)
	case invoiceFieldDueDate:
		m.editInvoiceForm.dueDateInput, cmd = m.editInvoiceForm.dueDateInput.Update(msg)
	case invoiceFieldURL:
		m.editInvoiceForm.urlInput, cmd = m.editInvoiceForm.urlInput.Update(msg)
	case invoiceFieldDescription:
		m.editInvoiceForm.descriptionInput, cmd = m.editInvoiceForm.descriptionInput.Update(msg)
	}
	return m, cmd
}

func (m model) saveInvoiceFromForm() (tea.Model, tea.Cmd) {
	title := strings.TrimSpace(m.addInvoiceForm.inputs[0].Value())
	currency := selectedCurrencyOption(m.addInvoiceForm.currencyOptions, m.addInvoiceForm.currencyIndex)
	amountRaw := strings.TrimSpace(m.addInvoiceForm.inputs[1].Value())
	peer := strings.TrimSpace(m.addInvoiceForm.inputs[2].Value())
	invoiceDateRaw := strings.TrimSpace(m.addInvoiceForm.inputs[3].Value())
	dueDateRaw := strings.TrimSpace(m.addInvoiceForm.inputs[4].Value())
	url := strings.TrimSpace(m.addInvoiceForm.inputs[5].Value())
	description := strings.TrimSpace(m.addInvoiceForm.inputs[6].Value())

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
	item := invoice{
		Title:         title,
		IsIncoming:    m.addInvoiceForm.isIncoming,
		Currency:      currency,
		AmountCents:   amount,
		Paid:          m.addInvoiceForm.paid,
		Peer:          peer,
		InvoiceDate:   invoiceDate,
		DueDate:       dueDate,
		URL:           url,
		Description:   description,
		LastUpdatedAt: now,
	}

	if err := m.db.Create(&item).Error; err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.invoices = append([]invoice{item}, m.invoices...)
	m.addInvoiceForm = newAddInvoiceForm(currencySelectionOptions(m.settings))
	m.screen = screenInvoiceList
	if item.Paid {
		m.invoiceMode = invoiceListHistoryPaid
	} else if item.IsIncoming {
		m.invoiceMode = invoiceListIncomingUnpaid
	} else {
		m.invoiceMode = invoiceListOutgoingUnpaid
	}
	m.invoiceCursor = 0
	m.status = "saved invoice " + title
	return m, nil
}

func (m model) openInvoiceEditor(item invoice) tea.Model {
	m.screen = screenInvoiceEdit
	m.editingInvoiceID = item.ID
	m.editInvoiceForm = newEditInvoiceForm(currencySelectionOptions(m.settings))
	m.editInvoiceForm.titleInput.SetValue(item.Title)
	if item.AmountCents != 0 {
		m.editInvoiceForm.amountInput.SetValue(formatAmount(item.AmountCents))
	} else {
		m.editInvoiceForm.amountInput.SetValue("")
	}
	m.editInvoiceForm.peerInput.SetValue(item.Peer)
	if item.InvoiceDate != nil {
		m.editInvoiceForm.invoiceDateInput.SetValue(item.InvoiceDate.Local().Format("02.01.2006"))
	}
	if item.DueDate != nil {
		m.editInvoiceForm.dueDateInput.SetValue(item.DueDate.Local().Format("02.01.2006"))
	}
	m.editInvoiceForm.urlInput.SetValue(item.URL)
	m.editInvoiceForm.descriptionInput.SetValue(item.Description)
	m.editInvoiceForm.currencyIndex = 0
	for i := range m.editInvoiceForm.currencyOptions {
		if strings.EqualFold(strings.TrimSpace(m.editInvoiceForm.currencyOptions[i]), strings.TrimSpace(item.Currency)) {
			m.editInvoiceForm.currencyIndex = i
			break
		}
	}
	m.editInvoiceForm.isIncoming = item.IsIncoming
	m.editInvoiceForm.paid = item.Paid
	m.editInvoiceForm = m.editInvoiceForm.focusActive()
	m.status = "editing invoice " + item.Title
	return m
}

func (m model) saveInvoiceEdit() (tea.Model, tea.Cmd) {
	index := m.findInvoiceIndex(m.editingInvoiceID)
	if index < 0 {
		m.status = "invoice not found"
		return m, nil
	}

	title := strings.TrimSpace(m.editInvoiceForm.titleInput.Value())
	if title == "" {
		m.status = "invoice title is required"
		return m, nil
	}

	amount, err := parseOptionalAmountCents(strings.TrimSpace(m.editInvoiceForm.amountInput.Value()))
	if err != nil {
		m.status = "amount error: " + err.Error()
		return m, nil
	}

	invoiceDate, err := parseOptionalDatePointer(strings.TrimSpace(m.editInvoiceForm.invoiceDateInput.Value()))
	if err != nil {
		m.status = "invoice date must use DD.MM.YYYY format"
		return m, nil
	}
	dueDate, err := parseOptionalDatePointer(strings.TrimSpace(m.editInvoiceForm.dueDateInput.Value()))
	if err != nil {
		m.status = "due date must use DD.MM.YYYY format"
		return m, nil
	}

	selected := m.invoices[index]
	selected.Title = title
	selected.IsIncoming = m.editInvoiceForm.isIncoming
	selected.Currency = selectedCurrencyOption(m.editInvoiceForm.currencyOptions, m.editInvoiceForm.currencyIndex)
	selected.AmountCents = amount
	selected.Paid = m.editInvoiceForm.paid
	selected.Peer = strings.TrimSpace(m.editInvoiceForm.peerInput.Value())
	selected.InvoiceDate = invoiceDate
	selected.DueDate = dueDate
	selected.URL = strings.TrimSpace(m.editInvoiceForm.urlInput.Value())
	selected.Description = strings.TrimSpace(m.editInvoiceForm.descriptionInput.Value())
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

func (m model) filteredInvoices() []invoice {
	filtered := make([]invoice, 0, len(m.invoices))
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
	return filtered
}

func (m model) findInvoiceIndex(id uint) int {
	for i := range m.invoices {
		if m.invoices[i].ID == id {
			return i
		}
	}
	return -1
}

func (m model) updateCashflowNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		return m, nil
	case "up", "shift+tab":
		m.addCashflowForm = m.addCashflowForm.prev()
		return m, nil
	case "down", "tab":
		m.addCashflowForm = m.addCashflowForm.next()
		return m, nil
	case "left":
		if m.addCashflowForm.active == cashflowFieldCurrency && m.addCashflowForm.currencyIndex > 0 {
			m.addCashflowForm.currencyIndex--
		}
		if m.addCashflowForm.active == cashflowFieldCategory && m.addCashflowForm.categoryIndex > 0 {
			m.addCashflowForm.categoryIndex--
		}
		if m.addCashflowForm.active == cashflowFieldAccount && m.addCashflowForm.accountIndex > 0 {
			m.addCashflowForm.accountIndex--
		}
		return m, nil
	case "right":
		if m.addCashflowForm.active == cashflowFieldCurrency && m.addCashflowForm.currencyIndex < len(m.addCashflowForm.currencyOptions)-1 {
			m.addCashflowForm.currencyIndex++
		}
		if m.addCashflowForm.active == cashflowFieldCategory && m.addCashflowForm.categoryIndex < len(m.addCashflowForm.categoryOptions)-1 {
			m.addCashflowForm.categoryIndex++
		}
		if m.addCashflowForm.active == cashflowFieldAccount && m.addCashflowForm.accountIndex < len(m.addCashflowForm.accountOptions)-1 {
			m.addCashflowForm.accountIndex++
		}
		return m, nil
	case "enter":
		if m.addCashflowForm.active == cashflowFieldCount-1 {
			return m.saveCashflowFromForm()
		}
		m.addCashflowForm = m.addCashflowForm.next()
		return m, nil
	}

	if inputIndex := m.addCashflowForm.inputIndexForField(m.addCashflowForm.active); inputIndex >= 0 {
		var cmd tea.Cmd
		m.addCashflowForm.inputs[inputIndex], cmd = m.addCashflowForm.inputs[inputIndex].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) saveCashflowFromForm() (tea.Model, tea.Cmd) {
	if len(m.addCashflowForm.categoryOptions) == 0 {
		if m.addCashflowForm.isIncome {
			m.status = "no income categories configured; add one in settings"
		} else {
			m.status = "no expense categories configured; add one in settings"
		}
		return m, nil
	}

	amountRaw := strings.TrimSpace(m.addCashflowForm.inputs[0].Value())
	dateRaw := strings.TrimSpace(m.addCashflowForm.inputs[1].Value())
	comment := strings.TrimSpace(m.addCashflowForm.inputs[2].Value())

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

	category := selectedStringOption(m.addCashflowForm.categoryOptions, m.addCashflowForm.categoryIndex)
	if category == "" {
		m.status = "category is required"
		return m, nil
	}

	now := time.Now()
	entry := cashflowEntry{
		IsIncome:      m.addCashflowForm.isIncome,
		Currency:      selectedCurrencyOption(m.addCashflowForm.currencyOptions, m.addCashflowForm.currencyIndex),
		AmountCents:   amount,
		EntryDate:     entryDate,
		Category:      category,
		AccountName:   selectedStringOption(m.addCashflowForm.accountOptions, m.addCashflowForm.accountIndex),
		Comment:       comment,
		LastUpdatedAt: now,
	}

	if err := m.db.Create(&entry).Error; err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.cashflows = append([]cashflowEntry{entry}, m.cashflows...)
	if entry.IsIncome {
		m.addCashflowForm = newAddCashflowForm(currencySelectionOptions(m.settings), incomeCategorySelectionOptions(m.settings), accountSelectionOptions(m.accounts), true)
		m.status = "saved income"
	} else {
		m.addCashflowForm = newAddCashflowForm(currencySelectionOptions(m.settings), expenseCategorySelectionOptions(m.settings), accountSelectionOptions(m.accounts), false)
		m.status = "saved expense"
	}
	m.screen = screenCashflowHistory
	m.cashflowHistoryMonth = beginningOfMonth(entry.EntryDate)
	m.cashflowCursor = 0
	return m, nil
}

func (m model) updateCashflowHistory(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func (m model) updateCashflowOverview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func (m model) cashflowOverviewPageSize() int {
	if m.height <= 0 {
		return 18
	}

	size := m.height - 16
	if size < 6 {
		size = 6
	}
	return size
}

func (m model) filteredCashflowsForMonth() []cashflowEntry {
	month := beginningOfMonth(m.cashflowHistoryMonth)
	filtered := make([]cashflowEntry, 0)
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

func (m model) monthlyCashflowOverviewRows() ([]cashflowMonthlyOverviewRow, int) {
	if len(m.cashflows) == 0 {
		return nil, 0
	}

	totalsByMonth := make(map[time.Time]cashflowMonthlyOverviewRow)
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

	rows := make([]cashflowMonthlyOverviewRow, 0)
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

func beginningOfMonth(value time.Time) time.Time {
	local := value.Local()
	return time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, local.Location())
}

func monthShift(month time.Time, delta int) time.Time {
	base := beginningOfMonth(month)
	return base.AddDate(0, delta, 0)
}

func debtDirectionLabel(isOwedToUser bool) string {
	if isOwedToUser {
		return "incoming (someone owes me)"
	}
	return "outgoing (i owe someone)"
}

func (m model) debtProgressTotalsBase(items []debt) (paid int64, total int64) {
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

func (m model) renderProgressBar(paid int64, total int64, width int) string {
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

func (m model) View() string {
	if m.quitting {
		return ""
	}

	width := m.width
	if width == 0 {
		width = 100
	}

	contentWidth := clamp(width-6, 76, 120)
	header := renderHeader(contentWidth)
	body := m.renderBody(contentWidth)
	footer := renderFooter(contentWidth, m.status)

	return lipgloss.JoinVertical(lipgloss.Left, header, "", body, "", footer)
}

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

func (m model) renderBody(width int) string {
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
	default:
		return m.renderMenu(width)
	}
}

func (m model) renderMenu(width int) string {
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

func (m model) renderMenuGroupBlock(groupIndex int, group menuGroup) string {
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

func minInt(left int, right int) int {
	if left < right {
		return left
	}

	return right
}

func (m model) renderMoneyConditionalRed(base string, cents int64) string {
	formatted := renderMoneyWithCurrency(base, cents)
	if cents > 0 {
		return obligationStyle.Render(formatted)
	}
	return formatted
}

func (m model) renderDashboard(width int) string {
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

	// Subscription totals
	activeSubscriptions := make([]subscription, 0)
	for _, sub := range m.subscriptions {
		if sub.IsActive {
			activeSubscriptions = append(activeSubscriptions, sub)
		}
	}
	monthlySubCost, yearlySubProjection := m.subscriptionTotalsInBaseCents(activeSubscriptions)

	// Total accounts
	totalAccounts := m.sumAccountsInBaseCents()

	// Unpaid debts (separate by direction)
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

	// Unpaid taxes
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

	// Unpaid invoices (separate by direction)
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

	// Goals progress
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

func sortAccountsByBaseAmount(accounts []account, settings appSettings) []account {
	if len(accounts) < 2 {
		return accounts
	}

	cloned := make([]account, len(accounts))
	copy(cloned, accounts)
	sort.SliceStable(cloned, func(i, j int) bool {
		leftComparable := comparableAccountBaseCents(cloned[i], settings)
		rightComparable := comparableAccountBaseCents(cloned[j], settings)
		if leftComparable == rightComparable {
			if cloned[i].CreatedAt.Equal(cloned[j].CreatedAt) {
				return cloned[i].ID > cloned[j].ID
			}
			return cloned[i].CreatedAt.After(cloned[j].CreatedAt)
		}
		return leftComparable > rightComparable
	})

	return cloned
}

func comparableAccountBaseCents(acct account, settings appSettings) int64 {
	base := strings.TrimSpace(settings.BaseCurrency)
	if base == "" {
		base = "$"
	}

	if strings.EqualFold(strings.TrimSpace(acct.Currency), base) {
		return acct.BalanceCents
	}

	for _, entry := range settings.Currencies {
		if strings.EqualFold(strings.TrimSpace(entry.CurrencyName), strings.TrimSpace(acct.Currency)) && entry.RateToBase > 0 {
			return int64(math.Round(float64(acct.BalanceCents) * entry.RateToBase))
		}
	}

	return acct.BalanceCents
}

func findAccountIndex(accounts []account, id uint) int {
	for i := range accounts {
		if accounts[i].ID == id {
			return i
		}
	}

	return -1
}

func (m model) renderAddAccount(width int) string {
	lines := []string{
		headlineStyle.Render("Add account"),
		mutedStyle.Render("Fill the fields, choose currency with left/right, then press Enter on Amount to save."),
		"",
	}

	for i := range m.addForm.fields {
		if i == 2 {
			lines = append(lines, m.renderAccountCurrencyFieldRow(width))
			continue
		}
		label := fieldLabelStyle.Render(m.addForm.labels[i])
		value := inputBoxStyle.Width(width - 18).Render(m.addForm.fields[i].View())
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, label, "  ", value))
	}

	lines = append(lines, "")
	lines = append(lines, mutedStyle.Render("Tab moves forward, Shift+Tab moves back, Esc returns home."))

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m model) renderAccountCurrencyFieldRow(width int) string {
	options := make([]string, 0, len(m.addForm.currencyOptions))
	for index, option := range m.addForm.currencyOptions {
		style := buttonStyle
		if index == m.addForm.currencyIndex {
			style = buttonActiveStyle
		}
		options = append(options, style.Render(option))
	}

	prefix := " "
	if m.addForm.active == 2 {
		prefix = ">"
	}
	label := fieldLabelStyle.Render(prefix + " Currency")
	value := inputBoxStyle.Width(width - 18).Render(strings.Join(options, " "))
	return lipgloss.JoinHorizontal(lipgloss.Top, label, "  ", value)
}

func (m model) renderAccountTable(width int) string {
	lines := []string{sectionTitleStyle.Render("Accounts")}
	lines = append(lines, hintStyle.Render("Use up/down to move, Enter to edit amount, Delete/Backspace asks confirmation, Esc to go back."))
	lines = append(lines, "")

	if len(m.accounts) == 0 {
		lines = append(lines, mutedStyle.Render("No accounts yet."))
		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	accountBaseTotal := m.sumAccountsInBaseCents()
	baseLabel := m.baseCurrencyLabel()
	lines = append(lines, fieldLabelStyle.Render("Total in "+baseLabel+":"), "  "+renderMoneyWithCurrency(baseLabel, accountBaseTotal), "")

	lines = append(lines, m.renderAccountTableHeader(width))
	for i, acct := range m.accounts {
		lines = append(lines, m.renderAccountRow(width, i, acct))
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m model) renderAccountTableHeader(width int) string {
	nameWidth := 18
	currencyWidth := 10
	amountWidth := 14
	baseAmountWidth := 14
	updatedWidth := 16
	descWidth := width - 14 - nameWidth - currencyWidth - amountWidth - baseAmountWidth - updatedWidth - 12
	if descWidth < 16 {
		descWidth = 16
	}
	baseCurrencyTitle := strings.TrimSpace(m.settings.BaseCurrency)
	if baseCurrencyTitle == "" {
		baseCurrencyTitle = "$"
	}

	header := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s", "#", nameWidth, "Name", currencyWidth, "Currency", amountWidth, "Amount", baseAmountWidth, truncateText(baseCurrencyTitle, baseAmountWidth), updatedWidth, "Updated", descWidth, "Description")
	return tableHeaderStyle.Render(header)
}

func (m model) renderAccountRow(width int, index int, acct account) string {
	nameWidth := 18
	currencyWidth := 10
	amountWidth := 14
	baseAmountWidth := 14
	updatedWidth := 16
	descWidth := width - 14 - nameWidth - currencyWidth - amountWidth - baseAmountWidth - updatedWidth - 12
	if descWidth < 16 {
		descWidth = 16
	}

	prefix := " "
	style := rowStyle
	if index == m.cursor {
		prefix = ">"
		style = selectedRowStyle
	}

	row := fmt.Sprintf("%s %-*s %-*s %-*s %-*s %-*s %-*s", prefix, nameWidth, truncateText(acct.Name, nameWidth), currencyWidth, truncateText(acct.Currency, currencyWidth), amountWidth, renderMoneyWithCurrency(acct.Currency, acct.BalanceCents), baseAmountWidth, m.convertedAmountForBase(acct.Currency, acct.BalanceCents), updatedWidth, formatUpdatedAt(acct.LastUpdatedAt), descWidth, truncateText(acct.Description, descWidth))
	return style.Render(row)
}

func (m model) renderEditAmount(width int) string {
	if m.cursor < 0 || m.cursor >= len(m.accounts) {
		return panelStyle.Width(width).Render(mutedStyle.Render("No account selected."))
	}

	acct := m.accounts[m.cursor]
	lines := []string{
		headlineStyle.Render("Edit amount only"),
		mutedStyle.Render("Account: " + acct.Name + " | " + acct.Currency + " | amount " + renderMoneyWithCurrency(acct.Currency, acct.BalanceCents)),
		"",
		fieldLabelStyle.Render("Amount"),
		inputBoxStyle.Width(24).Render(m.editInput.View()),
		"",
		mutedStyle.Render("Enter saves, Esc cancels."),
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m model) renderSubscriptionNew(width int) string {
	activeMarker := "[ ]"
	if m.addSubscriptionForm.isActive {
		activeMarker = "[x]"
	}

	lines := []string{
		headlineStyle.Render("New subscription"),
		mutedStyle.Render("Use up/down to move fields. Left/right changes Currency/Method/Period/Type. Space toggles Is Active. Enter on last field saves."),
		"",
		m.renderSubscriptionTextFieldRow(subFieldName, "Name", m.addSubscriptionForm.inputs[0].View()),
		m.renderSubscriptionOptionRow(subFieldCurrency, "Currency", m.addSubscriptionForm.currencyOptions, m.addSubscriptionForm.currencyIndex),
		m.renderSubscriptionTextFieldRow(subFieldAmount, "Amount", m.addSubscriptionForm.inputs[2].View()),
		m.renderSubscriptionOptionRow(subFieldPeriod, "Period", m.addSubscriptionForm.periodOptions, m.addSubscriptionForm.periodIndex),
		m.renderSubscriptionOptionRow(subFieldPaymentMethodChoice, "Payment method", m.addSubscriptionForm.paymentMethodOptions, m.addSubscriptionForm.paymentMethodIndex),
		m.renderSubscriptionTextFieldRow(subFieldPaymentMethod, "Payment method (custom)", m.addSubscriptionForm.inputs[3].View()),
		m.renderSubscriptionOptionRow(subFieldType, "Type", m.addSubscriptionForm.typeOptions, m.addSubscriptionForm.typeIndex),
		m.renderSubscriptionTextFieldRow(subFieldIsActive, "Is Active", activeMarker),
		m.renderSubscriptionTextFieldRow(subFieldDayYearly, "Payment date (yearly)", m.addSubscriptionForm.inputs[4].View()),
		m.renderSubscriptionTextFieldRow(subFieldDayMonthly, "Payment day (monthly)", m.addSubscriptionForm.inputs[5].View()),
		"",
		mutedStyle.Render("Esc goes back to menu. Day fields are optional."),
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m model) renderSubscriptionEdit(width int) string {
	activeMarker := "[ ]"
	if m.editSubscriptionForm.isActive {
		activeMarker = "[x]"
	}

	lines := []string{
		headlineStyle.Render("Edit subscription"),
		mutedStyle.Render("Use up/down to move fields. Space toggles Is Active. Enter on Active saves. Esc cancels."),
		"",
		mutedStyle.Render("Name: " + m.editSubscriptionForm.nameLabel + " | Type: " + m.editSubscriptionForm.typeLabel + " | Period: " + m.editSubscriptionForm.periodLabel),
		mutedStyle.Render("Currency: " + m.editSubscriptionForm.currencyLabel + " | Yearly date: " + m.editSubscriptionForm.paymentDateYearly + " | Monthly day: " + m.editSubscriptionForm.paymentDayMonthly),
		"",
		m.renderEditSubscriptionField(editSubFieldAmount, "Amount", m.editSubscriptionForm.amountInput.View()),
		m.renderEditSubscriptionField(editSubFieldPaymentMethod, "Payment method", m.editSubscriptionForm.paymentMethodInput.View()),
		m.renderEditSubscriptionField(editSubFieldIsActive, "Is Active", activeMarker),
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m model) renderEditSubscriptionField(field int, label string, value string) string {
	prefix := "  "
	if m.editSubscriptionForm.activeField == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m model) renderSubscriptionTextFieldRow(field int, label string, value string) string {
	prefix := "  "
	if m.addSubscriptionForm.active == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m model) renderSubscriptionOptionRow(field int, label string, options []string, selected int) string {
	prefix := "  "
	if m.addSubscriptionForm.active == field {
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

func (m model) renderSubscriptionList(width int) string {
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

func (m model) renderSubscriptionTableHeader(width int) string {
	nameWidth := 16
	currencyWidth := 8
	amountWidth := 12
	baseAmountWidth := 12
	periodWidth := 7
	typeWidth := 12
	activeWidth := 7
	descWidth := width - 12 - nameWidth - currencyWidth - amountWidth - baseAmountWidth - periodWidth - typeWidth - activeWidth - 14
	if descWidth < 12 {
		descWidth = 12
	}
	baseCurrencyTitle := strings.TrimSpace(m.settings.BaseCurrency)
	if baseCurrencyTitle == "" {
		baseCurrencyTitle = "$"
	}

	header := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", "#", nameWidth, "Name", currencyWidth, "Curr", amountWidth, "Amount", baseAmountWidth, truncateText(baseCurrencyTitle, baseAmountWidth), periodWidth, "Period", typeWidth, "Type", activeWidth, "Active", descWidth, "Payment method")
	return tableHeaderStyle.Render(header)
}

func (m model) renderSubscriptionRow(width int, index int, sub subscription) string {
	nameWidth := 16
	currencyWidth := 8
	amountWidth := 12
	baseAmountWidth := 12
	periodWidth := 7
	typeWidth := 12
	activeWidth := 7
	descWidth := width - 12 - nameWidth - currencyWidth - amountWidth - baseAmountWidth - periodWidth - typeWidth - activeWidth - 14
	if descWidth < 12 {
		descWidth = 12
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

func (m model) renderDebtNew(width int) string {
	directionIndex := 0
	if m.addDebtForm.isOwedToUser {
		directionIndex = 1
	}
	lines := []string{
		headlineStyle.Render("New debt"),
		mutedStyle.Render("Use up/down to move fields. Left/right changes direction and currency. Space also toggles direction. Enter on last field saves."),
		"",
		m.renderDebtChoiceRow(debtFieldDirection, "Direction", []string{"outgoing (i owe)", "incoming (owed to me)"}, directionIndex),
		m.renderDebtRowText(debtFieldPeer, "Peer", m.addDebtForm.inputs[0].View()),
		m.renderDebtChoiceRow(debtFieldCurrency, "Currency", m.addDebtForm.currencyOptions, m.addDebtForm.currencyIndex),
		m.renderDebtRowText(debtFieldAmount, "Amount", m.addDebtForm.inputs[1].View()),
		m.renderDebtRowText(debtFieldAmountPaid, "Amount paid", m.addDebtForm.inputs[2].View()),
		m.renderDebtRowText(debtFieldDebtCreated, "Debt created", m.addDebtForm.inputs[3].View()),
		m.renderDebtRowText(debtFieldDueDate, "Due date", m.addDebtForm.inputs[4].View()),
		m.renderDebtRowText(debtFieldComment, "Comment", m.addDebtForm.inputs[5].View()),
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m model) renderDebtRowText(field int, label string, value string) string {
	prefix := "  "
	if m.addDebtForm.active == field {
		prefix = "> "
	}
	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m model) renderDebtChoiceRow(field int, label string, options []string, selected int) string {
	prefix := "  "
	if m.addDebtForm.active == field {
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

func (m model) renderDebtList(width int) string {
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

func (m model) renderDebtTableHeader(width int) string {
	peerWidth := 14
	currencyWidth := 8
	amountWidth := 12
	paidWidth := 12
	leftWidth := 12
	dueWidth := 12
	dirWidth := 8
	commentWidth := width - 14 - peerWidth - currencyWidth - amountWidth - paidWidth - leftWidth - dueWidth - dirWidth - 18
	if commentWidth < 12 {
		commentWidth = 12
	}
	header := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", "#", peerWidth, "Peer", currencyWidth, "Curr", amountWidth, "Amount", paidWidth, "Paid", leftWidth, "Left", dueWidth, "Due", dirWidth, "Dir", commentWidth, "Comment")
	return tableHeaderStyle.Render(header)
}

func (m model) renderDebtTableRow(width int, index int, item debt) string {
	peerWidth := 14
	currencyWidth := 8
	amountWidth := 12
	paidWidth := 12
	leftWidth := 12
	dueWidth := 12
	dirWidth := 8
	commentWidth := width - 14 - peerWidth - currencyWidth - amountWidth - paidWidth - leftWidth - dueWidth - dirWidth - 18
	if commentWidth < 12 {
		commentWidth = 12
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

func (m model) renderDebtEdit(width int) string {
	lines := []string{
		headlineStyle.Render("Edit debt"),
		mutedStyle.Render("Edit fields and press Enter on Comment to save."),
		"",
		mutedStyle.Render("Peer: " + m.editDebtForm.peerLabel + " | Direction: " + m.editDebtForm.directionLabel + " | Currency: " + m.editDebtForm.currencyLabel),
	}

	if idx := m.findDebtIndex(m.editingDebtID); idx >= 0 {
		item := m.debts[idx]
		lines = append(lines, fieldLabelStyle.Render("Paid progress"))
		lines = append(lines, "  "+m.renderProgressBar(item.AmountPaidCents, item.AmountCents, 28))
	}

	lines = append(lines,
		"",
		m.renderEditDebtField(editDebtFieldAmount, "Amount", m.editDebtForm.amountInput.View()),
		m.renderEditDebtField(editDebtFieldAmountPaid, "Amount paid", m.editDebtForm.amountPaidInput.View()),
		m.renderEditDebtField(editDebtFieldDebtCreated, "Debt created", m.editDebtForm.debtCreatedInput.View()),
		m.renderEditDebtField(editDebtFieldDueDate, "Due date", m.editDebtForm.dueDateInput.View()),
		m.renderEditDebtField(editDebtFieldComment, "Comment", m.editDebtForm.commentInput.View()),
		"",
		fieldLabelStyle.Render("Add transaction"),
		mutedStyle.Render("Set delta/date/comment, then press Enter on Transaction comment to apply."),
		m.renderEditDebtField(editDebtFieldLogDelta, "Transaction delta", m.editDebtForm.logDeltaInput.View()),
		m.renderEditDebtField(editDebtFieldLogDate, "Transaction date", m.editDebtForm.logDateInput.View()),
		m.renderEditDebtField(editDebtFieldLogComment, "Transaction comment", m.editDebtForm.logCommentInput.View()),
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

func (m model) renderEditDebtField(field int, label string, value string) string {
	prefix := "  "
	if m.editDebtForm.activeField == field {
		prefix = "> "
	}
	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m model) renderGoalNew(width int) string {
	lines := []string{
		headlineStyle.Render("New goal"),
		mutedStyle.Render("Use up/down to move fields. Left/right changes currency. Enter on last field saves."),
		"",
		m.renderGoalRowText(goalFieldName, "Goal", m.addGoalForm.inputs[0].View()),
		m.renderGoalChoiceRow(goalFieldCurrency, "Currency", m.addGoalForm.currencyOptions, m.addGoalForm.currencyIndex),
		m.renderGoalRowText(goalFieldTargetAmount, "Target amount", m.addGoalForm.inputs[1].View()),
		m.renderGoalRowText(goalFieldAccumulated, "Accumulated", m.addGoalForm.inputs[2].View()),
		m.renderGoalRowText(goalFieldDescription, "Description", m.addGoalForm.inputs[3].View()),
		m.renderGoalRowText(goalFieldDateStarted, "Date started", m.addGoalForm.inputs[4].View()),
		m.renderGoalRowText(goalFieldTargetDate, "Target date", m.addGoalForm.inputs[5].View()),
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m model) renderGoalRowText(field int, label string, value string) string {
	prefix := "  "
	if m.addGoalForm.active == field {
		prefix = "> "
	}
	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m model) renderGoalChoiceRow(field int, label string, options []string, selected int) string {
	prefix := "  "
	if m.addGoalForm.active == field {
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

func (m model) renderGoalList(width int) string {
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

func (m model) renderGoalTableHeader(width int) string {
	nameWidth := 14
	currencyWidth := 8
	targetWidth := 12
	accumWidth := 12
	leftWidth := 12
	startedWidth := 12
	targetDateWidth := 12
	progressWidth := 8
	descWidth := width - 14 - nameWidth - currencyWidth - targetWidth - accumWidth - leftWidth - startedWidth - targetDateWidth - progressWidth - 18
	if descWidth < 10 {
		descWidth = 10
	}
	header := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", "#", nameWidth, "Goal", currencyWidth, "Curr", targetWidth, "Target", accumWidth, "Saved", leftWidth, "Left", startedWidth, "Started", targetDateWidth, "Target dt", progressWidth, "Done", descWidth, "Description")
	return tableHeaderStyle.Render(header)
}

func (m model) renderGoalTableRow(width int, index int, item goal) string {
	nameWidth := 14
	currencyWidth := 8
	targetWidth := 12
	accumWidth := 12
	leftWidth := 12
	startedWidth := 12
	targetDateWidth := 12
	progressWidth := 8
	descWidth := width - 14 - nameWidth - currencyWidth - targetWidth - accumWidth - leftWidth - startedWidth - targetDateWidth - progressWidth - 18
	if descWidth < 10 {
		descWidth = 10
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

func (m model) renderGoalEdit(width int) string {
	lines := []string{
		headlineStyle.Render("Edit goal"),
		mutedStyle.Render("Edit fields and press Enter on Description to save."),
		"",
		mutedStyle.Render("Goal: " + m.editGoalForm.nameLabel + " | Currency: " + m.editGoalForm.currencyLabel),
	}

	if idx := m.findGoalIndex(m.editingGoalID); idx >= 0 {
		item := m.goals[idx]
		lines = append(lines, fieldLabelStyle.Render("Accumulation progress"))
		lines = append(lines, "  "+m.renderProgressBar(item.AmountAccumulatedCents, item.TargetAmountCents, 28))
	}

	lines = append(lines,
		"",
		m.renderEditGoalField(editGoalFieldTargetAmount, "Target amount", m.editGoalForm.targetAmountInput.View()),
		m.renderEditGoalField(editGoalFieldAccumulated, "Accumulated", m.editGoalForm.accumulatedAmountInput.View()),
		m.renderEditGoalField(editGoalFieldDateStarted, "Date started", m.editGoalForm.dateStartedInput.View()),
		m.renderEditGoalField(editGoalFieldTargetDate, "Target date", m.editGoalForm.targetDateInput.View()),
		m.renderEditGoalField(editGoalFieldDescription, "Description", m.editGoalForm.descriptionInput.View()),
		"",
		fieldLabelStyle.Render("Add transaction"),
		mutedStyle.Render("Set delta/date/comment, then press Enter on Transaction comment to apply."),
		m.renderEditGoalField(editGoalFieldLogDelta, "Transaction delta", m.editGoalForm.logDeltaInput.View()),
		m.renderEditGoalField(editGoalFieldLogDate, "Transaction date", m.editGoalForm.logDateInput.View()),
		m.renderEditGoalField(editGoalFieldLogComment, "Transaction comment", m.editGoalForm.logCommentInput.View()),
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

func (m model) renderEditGoalField(field int, label string, value string) string {
	prefix := "  "
	if m.editGoalForm.activeField == field {
		prefix = "> "
	}
	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m model) renderTaxNew(width int) string {
	lines := []string{
		headlineStyle.Render("New tax"),
		mutedStyle.Render("Use up/down to move fields. Left/right changes tax type. Enter on last field saves."),
		"",
	}

	if len(m.addTaxForm.taxTypeOptions) == 0 {
		lines = append(lines, mutedStyle.Render("No tax types configured. Add one in Settings first."))
		lines = append(lines, mutedStyle.Render("Press Esc to go back."))
		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	lines = append(lines,
		m.renderTaxChoiceRow(taxFieldTaxType, "Tax type", m.addTaxForm.taxDisplayNames, m.addTaxForm.taxTypeIndex),
		m.renderTaxRowText(taxFieldAmountDue, "Amount due", m.addTaxForm.inputs[0].View()),
		m.renderTaxRowText(taxFieldAmountPaid, "Amount paid", m.addTaxForm.inputs[1].View()),
		m.renderTaxRowText(taxFieldPeriod, "Period", m.addTaxForm.inputs[2].View()),
		m.renderTaxRowText(taxFieldDueDate, "Due date", m.addTaxForm.inputs[3].View()),
		m.renderTaxRowText(taxFieldComment, "Comment", m.addTaxForm.inputs[4].View()),
	)

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m model) renderTaxRowText(field int, label string, value string) string {
	prefix := "  "
	if m.addTaxForm.active == field {
		prefix = "> "
	}
	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m model) renderTaxChoiceRow(field int, label string, options []string, selected int) string {
	prefix := "  "
	if m.addTaxForm.active == field {
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

func (m model) renderTaxList(width int) string {
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

func (m model) renderTaxTableHeader(width int) string {
	countryWidth := 12
	typeWidth := 14
	dueWidth := 12
	paidWidth := 12
	leftWidth := 12
	periodWidth := 12
	dueDateWidth := 12
	progressWidth := 8
	commentWidth := width - 14 - countryWidth - typeWidth - dueWidth - paidWidth - leftWidth - periodWidth - dueDateWidth - progressWidth - 18
	if commentWidth < 10 {
		commentWidth = 10
	}
	header := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", "#", countryWidth, "Country", typeWidth, "Tax type", dueWidth, "Due", paidWidth, "Paid", leftWidth, "Left", periodWidth, "Period", dueDateWidth, "Due date", progressWidth, "Done", commentWidth, "Comment")
	return tableHeaderStyle.Render(header)
}

func (m model) renderTaxTableRow(width int, index int, item tax) string {
	countryWidth := 12
	typeWidth := 14
	dueWidth := 12
	paidWidth := 12
	leftWidth := 12
	periodWidth := 12
	dueDateWidth := 12
	progressWidth := 8
	commentWidth := width - 14 - countryWidth - typeWidth - dueWidth - paidWidth - leftWidth - periodWidth - dueDateWidth - progressWidth - 18
	if commentWidth < 10 {
		commentWidth = 10
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

func (m model) renderTaxEdit(width int) string {
	lines := []string{
		headlineStyle.Render("Edit tax"),
		mutedStyle.Render("Edit fields and press Enter on Comment to save."),
		"",
		mutedStyle.Render("Tax: " + m.editTaxForm.countryLabel + " / " + m.editTaxForm.taxTypeLabel),
	}

	if idx := m.findTaxIndex(m.editingTaxID); idx >= 0 {
		item := m.taxes[idx]
		lines = append(lines, fieldLabelStyle.Render("Paid progress"))
		lines = append(lines, "  "+m.renderProgressBar(item.AmountPaidCents, item.AmountDueCents, 28))
	}

	lines = append(lines,
		"",
		m.renderEditTaxField(editTaxFieldAmountDue, "Amount due", m.editTaxForm.amountDueInput.View()),
		m.renderEditTaxField(editTaxFieldAmountPaid, "Amount paid", m.editTaxForm.amountPaidInput.View()),
		m.renderEditTaxField(editTaxFieldPeriod, "Period", m.editTaxForm.periodInput.View()),
		m.renderEditTaxField(editTaxFieldDueDate, "Due date", m.editTaxForm.dueDateInput.View()),
		m.renderEditTaxField(editTaxFieldComment, "Comment", m.editTaxForm.commentInput.View()),
		"",
		fieldLabelStyle.Render("Add transaction"),
		mutedStyle.Render("Set delta/date/comment, then press Enter on Transaction comment to apply."),
		m.renderEditTaxField(editTaxFieldLogDelta, "Transaction delta", m.editTaxForm.logDeltaInput.View()),
		m.renderEditTaxField(editTaxFieldLogDate, "Transaction date", m.editTaxForm.logDateInput.View()),
		m.renderEditTaxField(editTaxFieldLogComment, "Transaction comment", m.editTaxForm.logCommentInput.View()),
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

func (m model) renderInvoiceNew(width int) string {
	typeIndex := 0
	if !m.addInvoiceForm.isIncoming {
		typeIndex = 1
	}
	paidMarker := "[ ]"
	if m.addInvoiceForm.paid {
		paidMarker = "[x]"
	}

	lines := []string{
		headlineStyle.Render("New invoice"),
		mutedStyle.Render("Only title and type are required. Use Enter on last field to save."),
		"",
		m.renderInvoiceRowText(invoiceFieldTitle, "Title", m.addInvoiceForm.inputs[0].View()),
		m.renderInvoiceChoiceRow(invoiceFieldType, "Type", []string{"incoming (i must pay)", "outgoing (they pay me)"}, typeIndex),
		m.renderInvoiceChoiceRow(invoiceFieldCurrency, "Currency", m.addInvoiceForm.currencyOptions, m.addInvoiceForm.currencyIndex),
		m.renderInvoiceRowText(invoiceFieldAmount, "Amount", m.addInvoiceForm.inputs[1].View()),
		m.renderInvoiceRowText(invoiceFieldPaid, "Paid", paidMarker),
		m.renderInvoiceRowText(invoiceFieldPeer, "Peer", m.addInvoiceForm.inputs[2].View()),
		m.renderInvoiceRowText(invoiceFieldInvoiceDate, "Invoice date", m.addInvoiceForm.inputs[3].View()),
		m.renderInvoiceRowText(invoiceFieldDueDate, "Due date", m.addInvoiceForm.inputs[4].View()),
		m.renderInvoiceRowText(invoiceFieldURL, "URL", m.addInvoiceForm.inputs[5].View()),
		m.renderInvoiceRowText(invoiceFieldDescription, "Description", m.addInvoiceForm.inputs[6].View()),
		"",
		mutedStyle.Render("Space toggles Paid when active."),
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m model) renderInvoiceList(width int) string {
	title := "Outgoing unpaid invoices"
	if m.invoiceMode == invoiceListIncomingUnpaid {
		title = "Incoming unpaid invoices"
	}
	if m.invoiceMode == invoiceListHistoryPaid {
		title = "Invoice history (paid)"
	}

	lines := []string{lipgloss.JoinHorizontal(lipgloss.Center, sectionTitleStyle.Render(title), "  ", modeBadgeStyle.Render("Invoices")), hintStyle.Render("Use up/down to browse. Enter edits invoice. Delete/Backspace asks confirmation. Esc returns to menu."), ""}
	filtered := m.filteredInvoices()
	if len(filtered) == 0 {
		lines = append(lines, mutedStyle.Render("No invoices found."))
		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	lines = append(lines, m.renderInvoiceTableHeader(width))
	for i, item := range filtered {
		lines = append(lines, m.renderInvoiceTableRow(width, i, item))
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m model) renderInvoiceEdit(width int) string {
	typeIndex := 0
	if !m.editInvoiceForm.isIncoming {
		typeIndex = 1
	}
	paidMarker := "[ ]"
	if m.editInvoiceForm.paid {
		paidMarker = "[x]"
	}

	lines := []string{
		headlineStyle.Render("Edit invoice"),
		mutedStyle.Render("Edit any field and press Enter on Description to save."),
		"",
		m.renderEditInvoiceField(invoiceFieldTitle, "Title", m.editInvoiceForm.titleInput.View()),
		m.renderEditInvoiceChoiceRow(invoiceFieldType, "Type", []string{"incoming (i must pay)", "outgoing (they pay me)"}, typeIndex),
		m.renderEditInvoiceChoiceRow(invoiceFieldCurrency, "Currency", m.editInvoiceForm.currencyOptions, m.editInvoiceForm.currencyIndex),
		m.renderEditInvoiceField(invoiceFieldAmount, "Amount", m.editInvoiceForm.amountInput.View()),
		m.renderEditInvoiceField(invoiceFieldPaid, "Paid", paidMarker),
		m.renderEditInvoiceField(invoiceFieldPeer, "Peer", m.editInvoiceForm.peerInput.View()),
		m.renderEditInvoiceField(invoiceFieldInvoiceDate, "Invoice date", m.editInvoiceForm.invoiceDateInput.View()),
		m.renderEditInvoiceField(invoiceFieldDueDate, "Due date", m.editInvoiceForm.dueDateInput.View()),
		m.renderEditInvoiceField(invoiceFieldURL, "URL", m.editInvoiceForm.urlInput.View()),
		m.renderEditInvoiceField(invoiceFieldDescription, "Description", m.editInvoiceForm.descriptionInput.View()),
		"",
		mutedStyle.Render("Space toggles Paid when active."),
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m model) renderInvoiceRowText(field int, label string, value string) string {
	prefix := "  "
	if m.addInvoiceForm.active == field {
		prefix = "> "
	}
	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m model) renderInvoiceChoiceRow(field int, label string, options []string, selected int) string {
	prefix := "  "
	if m.addInvoiceForm.active == field {
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

func (m model) renderEditInvoiceField(field int, label string, value string) string {
	prefix := "  "
	if m.editInvoiceForm.activeField == field {
		prefix = "> "
	}
	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m model) renderEditInvoiceChoiceRow(field int, label string, options []string, selected int) string {
	prefix := "  "
	if m.editInvoiceForm.activeField == field {
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

func (m model) renderInvoiceTableHeader(width int) string {
	titleWidth := 16
	typeWidth := 8
	currencyWidth := 7
	amountWidth := 12
	paidWidth := 6
	peerWidth := 14
	invDateWidth := 10
	dueDateWidth := 10
	descWidth := width - 14 - titleWidth - typeWidth - currencyWidth - amountWidth - paidWidth - peerWidth - invDateWidth - dueDateWidth - 18
	if descWidth < 8 {
		descWidth = 8
	}
	header := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", "#", titleWidth, "Title", typeWidth, "Type", currencyWidth, "Curr", amountWidth, "Amount", paidWidth, "Paid", peerWidth, "Peer", invDateWidth, "Issued", dueDateWidth, "Due", descWidth, "Description")
	return tableHeaderStyle.Render(header)
}

func (m model) renderInvoiceTableRow(width int, index int, item invoice) string {
	titleWidth := 16
	typeWidth := 8
	currencyWidth := 7
	amountWidth := 12
	paidWidth := 6
	peerWidth := 14
	invDateWidth := 10
	dueDateWidth := 10
	descWidth := width - 14 - titleWidth - typeWidth - currencyWidth - amountWidth - paidWidth - peerWidth - invDateWidth - dueDateWidth - 18
	if descWidth < 8 {
		descWidth = 8
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

	row := fmt.Sprintf("%s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", prefix, titleWidth, truncateText(item.Title, titleWidth), typeWidth, typeLabel, currencyWidth, truncateText(item.Currency, currencyWidth), amountWidth, renderMoneyWithCurrency(item.Currency, item.AmountCents), paidWidth, paidLabel, peerWidth, truncateText(item.Peer, peerWidth), invDateWidth, issued, dueDateWidth, due, descWidth, truncateText(item.Description, descWidth))
	return style.Render(row)
}

func (m model) renderCashflowNew(width int) string {
	title := "New expense"
	if m.addCashflowForm.isIncome {
		title = "New income"
	}

	lines := []string{
		headlineStyle.Render(title),
		mutedStyle.Render("Currency, amount, date, category, optional account and comment. Date defaults to today."),
		"",
	}

	if len(m.addCashflowForm.categoryOptions) == 0 {
		if m.addCashflowForm.isIncome {
			lines = append(lines, mutedStyle.Render("No income categories configured. Add one in Settings first."))
		} else {
			lines = append(lines, mutedStyle.Render("No expense categories configured. Add one in Settings first."))
		}
		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	lines = append(lines,
		m.renderCashflowChoiceRow(cashflowFieldCurrency, "Currency", m.addCashflowForm.currencyOptions, m.addCashflowForm.currencyIndex),
		m.renderCashflowTextRow(cashflowFieldAmount, "Amount", m.addCashflowForm.inputs[0].View()),
		m.renderCashflowTextRow(cashflowFieldDate, "Date", m.addCashflowForm.inputs[1].View()),
		m.renderCashflowChoiceRow(cashflowFieldCategory, "Category", m.addCashflowForm.categoryOptions, m.addCashflowForm.categoryIndex),
		m.renderCashflowChoiceRow(cashflowFieldAccount, "Account (optional)", cashflowAccountDisplayOptions(m.addCashflowForm.accountOptions), m.addCashflowForm.accountIndex),
		m.renderCashflowTextRow(cashflowFieldComment, "Comment", m.addCashflowForm.inputs[2].View()),
	)

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m model) renderCashflowTextRow(field int, label string, value string) string {
	prefix := "  "
	if m.addCashflowForm.active == field {
		prefix = "> "
	}
	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m model) renderCashflowChoiceRow(field int, label string, options []string, selected int) string {
	prefix := "  "
	if m.addCashflowForm.active == field {
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

func (m model) renderCashflowHistory(width int) string {
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

func (m model) renderCashflowOverview(width int) string {
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

func (m model) renderCashflowOverviewHeader(width int, base string) string {
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

func (m model) renderCashflowOverviewRow(width int, row cashflowMonthlyOverviewRow, base string) string {
	monthWidth := 14
	incomeWidth := 14
	expenseWidth := 14
	netWidth := 14
	compareWidth := width - 14 - monthWidth - incomeWidth - expenseWidth - netWidth - 12
	if compareWidth < 16 {
		compareWidth = 16
	}

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

func (m model) renderCashflowHistoryHeader(width int) string {
	dateWidth := 10
	typeWidth := 8
	currencyWidth := 8
	amountWidth := 12
	categoryWidth := 16
	accountWidth := 14
	commentWidth := width - 14 - dateWidth - typeWidth - currencyWidth - amountWidth - categoryWidth - accountWidth - 18
	if commentWidth < 12 {
		commentWidth = 12
	}
	header := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", "#", dateWidth, "Date", typeWidth, "Type", currencyWidth, "Curr", amountWidth, "Amount", categoryWidth, "Category", accountWidth, "Account", commentWidth, "Comment")
	return tableHeaderStyle.Render(header)
}

func (m model) renderCashflowHistoryRow(width int, index int, item cashflowEntry) string {
	dateWidth := 10
	typeWidth := 8
	currencyWidth := 8
	amountWidth := 12
	categoryWidth := 16
	accountWidth := 14
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

func (m model) renderEditTaxField(field int, label string, value string) string {
	prefix := "  "
	if m.editTaxForm.activeField == field {
		prefix = "> "
	}
	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m model) renderSettings(width int) string {
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

func formatRate(value float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", value), "0"), ".")
}

func currencySelectionOptions(settings appSettings) []string {
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

func paymentMethodSelectionOptions(settings appSettings) []string {
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

func invoiceCurrencySelectionOptions(currencyOptions []string) []string {
	options := []string{""}
	seen := map[string]struct{}{"": {}}
	for _, item := range currencyOptions {
		value := strings.TrimSpace(item)
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		options = append(options, value)
	}
	return options
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

func (m model) convertedAmountForBase(currency string, cents int64) string {
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

func (m model) baseCurrencyLabel() string {
	base := strings.TrimSpace(m.settings.BaseCurrency)
	if base == "" {
		return "$"
	}
	return base
}

func (m model) convertToBaseCents(currency string, cents int64) (int64, bool) {
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

func (m model) sumAccountsInBaseCents() int64 {
	var total int64
	for _, acct := range m.accounts {
		converted, ok := m.convertToBaseCents(acct.Currency, acct.BalanceCents)
		if !ok {
			continue
		}
		total += converted
	}
	return total
}

func (m model) subscriptionTotalsInBaseCents(subs []subscription) (monthlyTotal int64, yearlyProjection int64) {
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

func splitSubscriptionsByActivity(subs []subscription) (active []subscription, inactive []subscription) {
	active = make([]subscription, 0, len(subs))
	inactive = make([]subscription, 0, len(subs))
	for _, sub := range subs {
		if sub.IsActive {
			active = append(active, sub)
			continue
		}
		inactive = append(inactive, sub)
	}
	return active, inactive
}

func (m model) rateToBase(currency string) (float64, bool) {
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

func (m model) filteredSubscriptions() []subscription {
	if m.subscriptionMode == subscriptionListAll {
		return m.subscriptions
	}

	filtered := make([]subscription, 0, len(m.subscriptions))
	for _, sub := range m.subscriptions {
		if sub.IsActive {
			filtered = append(filtered, sub)
		}
	}

	return filtered
}

func sortSubscriptionsByAmount(values []subscription) []subscription {
	if len(values) < 2 {
		return values
	}

	cloned := make([]subscription, len(values))
	copy(cloned, values)
	sort.SliceStable(cloned, func(i, j int) bool {
		if cloned[i].AmountCents == cloned[j].AmountCents {
			return cloned[i].CreatedAt.After(cloned[j].CreatedAt)
		}
		return cloned[i].AmountCents > cloned[j].AmountCents
	})

	return cloned
}

func parseAmountCents(raw string) (int64, error) {
	amount, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, errors.New("amount must be a number")
	}

	if amount < 0 {
		return 0, errors.New("amount cannot be negative")
	}

	return int64(math.Round(amount * 100)), nil
}

func parseOptionalAmountCents(raw string) (int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	return parseAmountCents(trimmed)
}

func parseOptionalDay(raw string, minValue int, maxValue int, label string) (*int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return nil, errors.New(label + " day must be an integer")
	}
	if value < minValue || value > maxValue {
		return nil, fmt.Errorf("%s day must be between %d and %d", label, minValue, maxValue)
	}

	return &value, nil
}

func parseOptionalDate(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	if _, err := time.Parse("02.01.2006", trimmed); err != nil {
		return "", errors.New("yearly date must use DD.MM.YYYY format")
	}

	return trimmed, nil
}

func parseRequiredDate(raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, errors.New("created date is required")
	}
	value, err := time.Parse("02.01.2006", trimmed)
	if err != nil {
		return time.Time{}, errors.New("created date must use DD.MM.YYYY format")
	}
	return value, nil
}

func parseOptionalDatePointer(raw string) (*time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	value, err := time.Parse("02.01.2006", trimmed)
	if err != nil {
		return nil, errors.New("due date must use DD.MM.YYYY format")
	}
	return &value, nil
}

func parseSignedAmountCents(raw string) (int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, errors.New("delta is required")
	}
	sign := int64(1)
	if strings.HasPrefix(trimmed, "+") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "+"))
	}
	if strings.HasPrefix(trimmed, "-") {
		sign = -1
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
	}
	amount, err := parseAmountCents(trimmed)
	if err != nil {
		return 0, err
	}
	return sign * amount, nil
}

func parseLogDateOrToday(raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Now(), nil
	}
	value, err := time.Parse("02.01.2006", trimmed)
	if err != nil {
		return time.Time{}, errors.New("log date must use DD.MM.YYYY format")
	}
	now := time.Now()
	return time.Date(value.Year(), value.Month(), value.Day(), now.Hour(), now.Minute(), now.Second(), 0, now.Location()), nil
}

func formatAmount(cents int64) string {
	return fmt.Sprintf("%.2f", float64(cents)/100)
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

func formatUpdatedAt(value time.Time) string {
	if value.IsZero() || value.Year() < 1971 {
		return "1970-01-01"
	}

	return value.Local().Format("2006-01-02 15:04")
}

func truncateText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return ""
	}

	if len(value) <= limit {
		return value
	}

	if limit <= 1 {
		return value[:limit]
	}

	return value[:limit-1] + "…"
}

func clamp(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}

	if value > maximum {
		return maximum
	}

	return value
}
