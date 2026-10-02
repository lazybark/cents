package desktop

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/summary"
)

// Setting kinds, as storage.DeleteSetting names them.
const (
	settingCurrency        = "currency"
	settingPaymentMethod   = "payment_method"
	settingTaxType         = "tax_type"
	settingIncomeCategory  = "income_category"
	settingExpenseCategory = "expense_category"
)

var errSettingNotFound = errors.New("setting not found")

// UsedBy counts the records that refer to a setting by name (or, for tax
// types, by id), so the frontend can warn before a rename or delete.

type CurrencySetting struct {
	ID         uint    `json:"id"`
	Name       string  `json:"name"`
	RateToBase float64 `json:"rateToBase"`
	UsedBy     int     `json:"usedBy"`
}

type PaymentMethodSetting struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	IsDefault bool   `json:"isDefault"`
	UsedBy    int    `json:"usedBy"`
}

type TaxTypeSetting struct {
	ID          uint   `json:"id"`
	Country     string `json:"country"`
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	UsedBy      int    `json:"usedBy"`
}

type CategorySetting struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
	UsedBy   int    `json:"usedBy"`
}

type SettingsView struct {
	DBPath             string                 `json:"dbPath"`
	BaseCurrency       string                 `json:"baseCurrency"`
	BaseUsedBy         int                    `json:"baseUsedBy"`
	Currencies         []CurrencySetting      `json:"currencies"`
	PaymentMethods     []PaymentMethodSetting `json:"paymentMethods"`
	PaymentMethodTypes []string               `json:"paymentMethodTypes"`
	TaxTypes           []TaxTypeSetting       `json:"taxTypes"`
	IncomeCategories   []CategorySetting      `json:"incomeCategories"`
	ExpenseCategories  []CategorySetting      `json:"expenseCategories"`
}

type CurrencyInput struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Rate string `json:"rate"`
}

type PaymentMethodInput struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	IsDefault bool   `json:"isDefault"`
}

type TaxTypeInput struct {
	ID          uint   `json:"id"`
	Country     string `json:"country"`
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

type CategoryInput struct {
	// IsIncome picks income categories; otherwise expense categories.
	IsIncome bool   `json:"isIncome"`
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	// Archived keeps the category off new entries and out of statistics.
	Archived bool `json:"archived"`
}

func (a *API) Settings() (SettingsView, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return SettingsView{}, err
	}

	data, err := loadSummaryData(storage)
	if err != nil {
		return SettingsView{}, err
	}

	a.mu.Lock()
	dbPath := a.dbPath
	a.mu.Unlock()

	stts := data.Settings
	usage := countUsage(data)

	view := SettingsView{
		DBPath:             dbPath,
		BaseCurrency:       stts.BaseCurrencyLabel(),
		BaseUsedBy:         usage.currencies[usageKey(stts.BaseCurrencyLabel())],
		Currencies:         make([]CurrencySetting, 0, len(stts.Currencies)),
		PaymentMethods:     make([]PaymentMethodSetting, 0, len(stts.PaymentMethods)),
		PaymentMethodTypes: settings.PaymentMethodTypeOptions(),
		TaxTypes:           make([]TaxTypeSetting, 0, len(stts.TaxTypes)),
		IncomeCategories:   make([]CategorySetting, 0, len(stts.IncomeCategories)),
		ExpenseCategories:  make([]CategorySetting, 0, len(stts.ExpenseCategories)),
	}

	for _, c := range stts.Currencies {
		view.Currencies = append(view.Currencies, CurrencySetting{ID: c.ID, Name: c.CurrencyName, RateToBase: c.RateToBase, UsedBy: usage.currencies[usageKey(c.CurrencyName)]})
	}

	for _, p := range stts.PaymentMethods {
		view.PaymentMethods = append(view.PaymentMethods, PaymentMethodSetting{ID: p.ID, Name: p.PaymentMethodName, Type: p.PaymentMethodType, IsDefault: p.IsDefault, UsedBy: usage.paymentMethods[usageKey(p.PaymentMethodName)]})
	}

	for _, t := range stts.TaxTypes {
		view.TaxTypes = append(view.TaxTypes, TaxTypeSetting{ID: t.ID, Country: t.Country, Name: t.TaxTypeName, Description: t.Description, URL: t.URL, UsedBy: usage.taxTypes[t.ID]})
	}

	for _, c := range stts.IncomeCategories {
		view.IncomeCategories = append(view.IncomeCategories, CategorySetting{ID: c.ID, Name: c.CategoryName, Archived: c.Archived, UsedBy: usage.incomeCategories[usageKey(c.CategoryName)]})
	}

	for _, c := range stts.ExpenseCategories {
		view.ExpenseCategories = append(view.ExpenseCategories, CategorySetting{ID: c.ID, Name: c.CategoryName, Archived: c.Archived, UsedBy: usage.expenseCategories[usageKey(c.CategoryName)]})
	}

	return view, nil
}

func (a *API) SaveBaseCurrency(value string) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	stts, err := storage.LoadAppSettings()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	record, err := settings.BaseCurrencyRecord(value)
	if err != nil {
		return err
	}

	for _, c := range stts.Currencies {
		if strings.EqualFold(strings.TrimSpace(c.CurrencyName), record.SettingValue) {
			return fmt.Errorf("%q is in the currency list: delete it there first", record.SettingValue)
		}
	}

	if err := storage.SaveSettingRecord(&record); err != nil {
		return fmt.Errorf("settings save failed: %w", err)
	}

	return nil
}

func (a *API) SaveCurrency(input CurrencyInput) error {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return err
	}

	var record settings.SettingCurrency
	names := make([]string, 0, len(stts.Currencies))
	for _, c := range stts.Currencies {
		if c.ID == input.ID {
			record = c
		} else {
			names = append(names, c.CurrencyName)
		}
	}

	if input.ID != 0 && record.ID == 0 {
		return errSettingNotFound
	}

	record, err = record.Apply(input.Name, input.Rate, time.Now())
	if err != nil {
		return err
	}

	if strings.EqualFold(record.CurrencyName, stts.BaseCurrencyLabel()) {
		return fmt.Errorf("%q is the base currency", record.CurrencyName)
	}

	if err := requireUnique("currency", record.CurrencyName, names); err != nil {
		return err
	}

	if err := storage.SaveSettingCurrency(&record); err != nil {
		return fmt.Errorf("currency save failed: %w", err)
	}

	return nil
}

func (a *API) SavePaymentMethod(input PaymentMethodInput) error {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return err
	}

	var record settings.SettingPaymentMethod
	names := make([]string, 0, len(stts.PaymentMethods))
	for _, p := range stts.PaymentMethods {
		if p.ID == input.ID {
			record = p
		} else {
			names = append(names, p.PaymentMethodName)
		}
	}

	if input.ID != 0 && record.ID == 0 {
		return errSettingNotFound
	}

	record, err = record.Apply(input.Name, input.Type, input.IsDefault, len(stts.PaymentMethods) == 0, time.Now())
	if err != nil {
		return err
	}

	if err := requireUnique("payment method", record.PaymentMethodName, names); err != nil {
		return err
	}

	if err := storage.SaveSettingPaymentMethod(&record); err != nil {
		return fmt.Errorf("payment method save failed: %w", err)
	}

	return nil
}

func (a *API) SaveTaxType(input TaxTypeInput) error {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return err
	}

	var record settings.SettingTaxType
	names := make([]string, 0, len(stts.TaxTypes))
	for _, t := range stts.TaxTypes {
		if t.ID == input.ID {
			record = t
		} else {
			names = append(names, taxTypeLabel(t))
		}
	}

	if input.ID != 0 && record.ID == 0 {
		return errSettingNotFound
	}

	record, err = record.Apply(input.Country, input.Name, input.Description, input.URL, time.Now())
	if err != nil {
		return err
	}

	if err := requireUnique("tax type", taxTypeLabel(record), names); err != nil {
		return err
	}

	if err := storage.SaveSettingTaxType(&record); err != nil {
		return fmt.Errorf("tax type save failed: %w", err)
	}

	return nil
}

func (a *API) SaveCategory(input CategoryInput) error {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return err
	}

	now := time.Now()

	if input.IsIncome {
		var record settings.SettingIncomeCategory
		names := make([]string, 0, len(stts.IncomeCategories))
		for _, c := range stts.IncomeCategories {
			if c.ID == input.ID {
				record = c
			} else {
				names = append(names, c.CategoryName)
			}
		}

		if input.ID != 0 && record.ID == 0 {
			return errSettingNotFound
		}

		if record, err = record.Apply(input.Name, now); err != nil {
			return err
		}

		if err := requireUnique("income category", record.CategoryName, names); err != nil {
			return err
		}

		record.Archived = input.Archived

		if err := storage.SaveSettingIncomeCategory(&record); err != nil {
			return fmt.Errorf("income category save failed: %w", err)
		}

		return nil
	}

	var record settings.SettingExpenseCategory
	names := make([]string, 0, len(stts.ExpenseCategories))
	for _, c := range stts.ExpenseCategories {
		if c.ID == input.ID {
			record = c
		} else {
			names = append(names, c.CategoryName)
		}
	}

	if input.ID != 0 && record.ID == 0 {
		return errSettingNotFound
	}

	if record, err = record.Apply(input.Name, now); err != nil {
		return err
	}

	if err := requireUnique("expense category", record.CategoryName, names); err != nil {
		return err
	}

	record.Archived = input.Archived

	if err := storage.SaveSettingExpenseCategory(&record); err != nil {
		return fmt.Errorf("expense category save failed: %w", err)
	}

	return nil
}

// DeleteSetting removes one setting. kind is "currency", "payment_method",
// "tax_type", "income_category" or "expense_category". Records that used it
// keep their stored names.
func (a *API) DeleteSetting(kind string, id uint) error {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return err
	}

	ids := map[string][]uint{}
	for _, c := range stts.Currencies {
		ids[settingCurrency] = append(ids[settingCurrency], c.ID)
	}

	for _, p := range stts.PaymentMethods {
		ids[settingPaymentMethod] = append(ids[settingPaymentMethod], p.ID)
	}

	for _, t := range stts.TaxTypes {
		ids[settingTaxType] = append(ids[settingTaxType], t.ID)
	}

	for _, c := range stts.IncomeCategories {
		ids[settingIncomeCategory] = append(ids[settingIncomeCategory], c.ID)
	}

	for _, c := range stts.ExpenseCategories {
		ids[settingExpenseCategory] = append(ids[settingExpenseCategory], c.ID)
	}

	found := false
	for _, existing := range ids[kind] {
		if existing == id {
			found = true

			break
		}
	}

	if !found {
		return errSettingNotFound
	}

	if err := storage.DeleteSetting(kind, id); err != nil {
		return fmt.Errorf("settings delete failed: %w", err)
	}

	return nil
}

func (a *API) storageAndSettings() (StorageWorker, settings.AppSettings, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return nil, settings.AppSettings{}, err
	}

	stts, err := storage.LoadAppSettings()
	if err != nil {
		return nil, settings.AppSettings{}, fmt.Errorf("failed to load settings: %w", err)
	}

	return storage, stts, nil
}

// requireUnique refuses a name another setting of the same kind already has,
// ignoring case, before the database's unique index does it less readably.
func requireUnique(kind, name string, others []string) error {
	for _, other := range others {
		if strings.EqualFold(strings.TrimSpace(other), name) {
			return fmt.Errorf("a %s named %q already exists", kind, name)
		}
	}

	return nil
}

func taxTypeLabel(t settings.SettingTaxType) string {
	return strings.TrimSpace(t.Country) + " / " + strings.TrimSpace(t.TaxTypeName)
}

type usageCounts struct {
	currencies        map[string]int
	paymentMethods    map[string]int
	taxTypes          map[uint]int
	incomeCategories  map[string]int
	expenseCategories map[string]int
}

func usageKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func countUsage(data summary.Data) usageCounts {
	usage := usageCounts{
		currencies:        map[string]int{},
		paymentMethods:    map[string]int{},
		taxTypes:          map[uint]int{},
		incomeCategories:  map[string]int{},
		expenseCategories: map[string]int{},
	}

	currency := func(name string) { usage.currencies[usageKey(name)]++ }

	for _, r := range data.Accounts {
		currency(r.Currency)
	}

	for _, r := range data.Subscriptions {
		currency(r.Currency)
		usage.paymentMethods[usageKey(r.PaymentMethod)]++
	}

	for _, r := range data.Debts {
		currency(r.Currency)
	}

	for _, r := range data.Goals {
		currency(r.Currency)
	}

	for _, r := range data.Invoices {
		currency(r.Currency)
	}

	for _, r := range data.Cashflows {
		currency(r.Currency)
		if r.IsIncome {
			usage.incomeCategories[usageKey(r.Category)]++
		} else {
			usage.expenseCategories[usageKey(r.Category)]++
		}
	}

	for _, r := range data.Taxes {
		usage.taxTypes[r.TaxTypeID]++
	}

	return usage
}
