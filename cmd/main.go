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

type appSettings struct {
	BaseCurrency   string
	Currencies     []settingCurrency
	PaymentMethods []settingPaymentMethod
}

type settingsEditMode int

const (
	settingsEditNone settingsEditMode = iota
	settingsEditBaseCurrency
	settingsEditCurrency
	settingsEditPaymentMethod
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
)

type subscriptionListMode int

const (
	subscriptionListActive subscriptionListMode = iota
	subscriptionListAll
)

type model struct {
	db                               *gorm.DB
	dbPath                           string
	created                          bool
	screen                           screen
	accounts                         []account
	subscriptions                    []subscription
	subscriptionMode                 subscriptionListMode
	subscriptionCursor               int
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
	status                           string
	width                            int
	height                           int
	quitting                         bool
	menuGroup                        int
	menuItem                         int
	cursor                           int
	addForm                          addAccountForm
	addSubscriptionForm              addSubscriptionForm
	editSubscriptionForm             editSubscriptionForm
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

const (
	editSubFieldAmount = iota
	editSubFieldPaymentMethod
	editSubFieldIsActive
	editSubFieldCount
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

var (
	appTitleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F4E9D8"))
	mutedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#9C927F"))
	headlineStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E7C96D"))
	statusStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#D0CABD"))
	panelStyle        = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#6F5F47")).Padding(0, 1)
	buttonStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#F4E9D8")).Background(lipgloss.Color("#4E4334")).Padding(0, 1)
	buttonActiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#1F1A17")).Background(lipgloss.Color("#E7C96D")).Bold(true).Padding(0, 1)
	fieldLabelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#E7C96D")).Bold(true)
	selectedRowStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#1F1A17")).Background(lipgloss.Color("#E7C96D"))
	rowStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#F4E9D8"))
	inputBoxStyle     = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#6F5F47")).Padding(0, 1)
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

	settings, err := loadAppSettings(db)
	if err != nil {
		fmt.Fprintln(os.Stderr, "settings read failed:", err)
		os.Exit(1)
	}

	m := newModel(db, dbPath, created, accounts, subscriptions, settings)
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

	if err := db.AutoMigrate(&account{}, &subscription{}, &settingRecord{}, &settingCurrency{}, &settingPaymentMethod{}); err != nil {
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

	return settings, nil
}

func newModel(db *gorm.DB, dbPath string, created bool, accounts []account, subscriptions []subscription, settings appSettings) model {
	accounts = sortAccountsByBaseAmount(accounts, settings)
	currencyOptions := currencySelectionOptions(settings)
	paymentMethodOptions := paymentMethodSelectionOptions(settings)
	addForm := newAddAccountForm(currencyOptions)
	addSubForm := newAddSubscriptionForm(currencyOptions, paymentMethodOptions)
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
		subscriptionMode:                 subscriptionListActive,
		subscriptionCursor:               0,
		status:                           status,
		menuGroup:                        0,
		menuItem:                         0,
		addForm:                          addForm,
		addSubscriptionForm:              addSubForm,
		editSubscriptionForm:             newEditSubscriptionForm(),
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
		{title: "Incomes and Expences", items: []string{"New Expence", "New Income", "History"}},
		{title: "Accounts", items: []string{"add account", "list accounts"}},
		{title: "Subscriptions", items: []string{"new", "active", "all"}},
		{title: "Invoices", items: []string{"new", "outgoing", "incoming"}},
		{title: "Debts", items: []string{"new", "outgoing", "incoming"}},
		{title: "Goals", items: []string{"new", "all"}},
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
	case "tab", "enter":
		if m.addForm.active == len(m.addForm.fields)-1 && msg.String() == "enter" {
			return m.saveAccountFromForm()
		}
		m.addForm = m.addForm.next()
		return m, nil
	case "shift+tab":
		m.addForm = m.addForm.prev()
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
		return m.deleteSelectedAccount()
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
	case "up", "k":
		if m.subscriptionCursor > 0 {
			m.subscriptionCursor--
		}
		return m, nil
	case "down", "j":
		if m.subscriptionCursor < len(filtered)-1 {
			m.subscriptionCursor++
		}
		return m, nil
	case "enter", "l":
		m = m.openSubscriptionEditor(filtered[m.subscriptionCursor]).(model)
		return m, nil
	case "backspace", "delete":
		return m.deleteSelectedSubscription(filtered)
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

	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		return m, nil
	case "up":
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
		return m, nil
	case "down":
		maxCursor := settingsPaymentMethodAddCursor(m.settings)
		if m.settingsCursor < maxCursor {
			m.settingsCursor++
		}
		return m, nil
	case "enter":
		paymentStart := settingsPaymentMethodStartCursor(m.settings)
		paymentAdd := settingsPaymentMethodAddCursor(m.settings)

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
	subtitle := mutedStyle.Render("personal finance accounts with one editable amount")
	line := lipgloss.JoinVertical(lipgloss.Left, title, subtitle)
	return panelStyle.Width(width).Render(line)
}

func renderFooter(width int, status string) string {
	return panelStyle.Width(width).Render(statusStyle.Render(status))
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
	default:
		return m.renderMenu(width)
	}
}

func (m model) renderMenu(width int) string {
	overview := m.renderReadOnlyAccountOverview(width)
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
		overview,
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

func (m model) renderReadOnlyAccountOverview(width int) string {
	lines := []string{headlineStyle.Render("Top 5 accounts by amount")}
	if len(m.accounts) == 0 {
		lines = append(lines, mutedStyle.Render("No accounts yet. Add one to get started."))
		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	lines = append(lines, m.renderAccountTableHeader(width))
	for _, acct := range topAccountsByBaseAmount(m.accounts, 5, m.settings) {
		lines = append(lines, m.renderReadOnlyAccountRow(width, acct))
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func topAccountsByBaseAmount(accounts []account, maxCount int, settings appSettings) []account {
	if maxCount <= 0 || len(accounts) == 0 {
		return nil
	}

	cloned := sortAccountsByBaseAmount(accounts, settings)

	if len(cloned) > maxCount {
		cloned = cloned[:maxCount]
	}

	return cloned
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

func (m model) renderReadOnlyAccountRow(width int, acct account) string {
	nameWidth := 18
	currencyWidth := 10
	amountWidth := 14
	baseAmountWidth := 14
	updatedWidth := 16
	descWidth := width - 14 - nameWidth - currencyWidth - amountWidth - baseAmountWidth - updatedWidth - 12
	if descWidth < 16 {
		descWidth = 16
	}

	row := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s", "", nameWidth, truncateText(acct.Name, nameWidth), currencyWidth, truncateText(acct.Currency, currencyWidth), amountWidth, renderMoneyWithCurrency(acct.Currency, acct.BalanceCents), baseAmountWidth, m.convertedAmountForBase(acct.Currency, acct.BalanceCents), updatedWidth, formatUpdatedAt(acct.LastUpdatedAt), descWidth, truncateText(acct.Description, descWidth))
	return rowStyle.Render(row)
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
	lines := []string{headlineStyle.Render("Accounts")}
	lines = append(lines, mutedStyle.Render("Use up/down to move, Enter to edit amount, Delete/Backspace to delete, Esc to go back."))
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
	return mutedStyle.Render(header)
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
		m.renderSubscriptionOptionRow(subFieldPaymentMethodChoice, "Payment method", m.addSubscriptionForm.paymentMethodOptions, m.addSubscriptionForm.paymentMethodIndex),
		m.renderSubscriptionTextFieldRow(subFieldPaymentMethod, "Payment method (custom)", m.addSubscriptionForm.inputs[3].View()),
		m.renderSubscriptionOptionRow(subFieldPeriod, "Period", m.addSubscriptionForm.periodOptions, m.addSubscriptionForm.periodIndex),
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

	lines := []string{headlineStyle.Render(modeTitle), mutedStyle.Render("Use up/down to browse. Enter edits the selected subscription. Delete/Backspace removes it. Esc returns to menu."), ""}
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
	return mutedStyle.Render(header)
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

func (m model) renderSettings(width int) string {
	header := []string{headlineStyle.Render("Settings"), mutedStyle.Render("Use up/down to select rows. Enter edits selected row. Esc returns to menu."), ""}
	header = append(header, mutedStyle.Render("General"))

	basePrefix := " "
	if m.settingsCursor == 0 {
		basePrefix = ">"
	}
	baseCurrencyValue := m.settings.BaseCurrency
	if m.settingsEditMode == settingsEditBaseCurrency {
		baseCurrencyValue = m.settingsEditInput.View()
	}

	rows := []string{fmt.Sprintf("%s %-18s %s", basePrefix, "base_currency", baseCurrencyValue), "", mutedStyle.Render("Currencies (relation to base currency)")}

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

	rows = append(rows, "", mutedStyle.Render("Payment methods"))
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

func formatAmount(cents int64) string {
	return fmt.Sprintf("%.2f", float64(cents)/100)
}

func renderMoneyWithCurrency(currency string, cents int64) string {
	if currency == "" {
		return formatAmount(cents)
	}

	return currency + " " + formatAmount(cents)
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
