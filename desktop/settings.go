package desktop

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lazybark/cents/flows/currency"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/summary"
)

// Setting kinds, as storage.DeleteSetting names them.
const (
	settingCurrency        = "currency"
	settingPaymentMethod   = "payment_method"
	settingTag             = "tag"
	settingTaxType         = "tax_type"
	settingIncomeCategory  = "income_category"
	settingExpenseCategory = "expense_category"
)

var errSettingNotFound = errors.New("setting not found")

// UsedBy counts the records that refer to a setting by name (or, for tax
// types, by id), so the frontend can warn before a rename or delete.

// CurrencySetting is a currency; Code is the known currency it's linked to
// (stored or worked out from its name), and Linked says its rate can be
// fetched.
type CurrencySetting struct {
	ID         uint    `json:"id"`
	Name       string  `json:"name"`
	Code       string  `json:"code"`
	CodeName   string  `json:"codeName"`
	Linked     bool    `json:"linked"`
	RateToBase float64 `json:"rateToBase"`
	UsedBy     int     `json:"usedBy"`
}

type PaymentMethodSetting struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	IsDefault bool   `json:"isDefault"`
	// Currency is what it pays in, "" for any.
	Currency string `json:"currency"`
	// Archived methods aren't offered for new subscriptions.
	Archived bool `json:"archived"`
	UsedBy   int  `json:"usedBy"`
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
	BaseCurrencyID     uint                   `json:"baseCurrencyId"`
	BaseCode           string                 `json:"baseCode"`
	BaseCodeName       string                 `json:"baseCodeName"`
	BaseUsedBy         int                    `json:"baseUsedBy"`
	Rates              RatesStatus            `json:"rates"`
	Currencies         []CurrencySetting      `json:"currencies"`
	PaymentMethods     []PaymentMethodSetting `json:"paymentMethods"`
	PaymentMethodTypes []string               `json:"paymentMethodTypes"`
	TaxTypes           []TaxTypeSetting       `json:"taxTypes"`
	IncomeCategories   []CategorySetting      `json:"incomeCategories"`
	ExpenseCategories  []CategorySetting      `json:"expenseCategories"`
	Tags               []CategorySetting      `json:"tags"`
}

// CurrencyInput is a currency to save. Code links it to a known currency
// (empty: worked out from the name); Name defaults to the code. An empty
// Rate is fetched for a linked currency.
type CurrencyInput struct {
	ID   uint   `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
	Rate string `json:"rate"`
}

// BaseCurrencyInput changes the base currency: Value is what records use,
// Code the known currency it stands for (empty: worked out from Value).
type BaseCurrencyInput struct {
	Value string `json:"value"`
	Code  string `json:"code"`
}

type PaymentMethodInput struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	IsDefault bool   `json:"isDefault"`
	// Currency is what it pays in, "" for any.
	Currency string `json:"currency"`
	Archived bool   `json:"archived"`
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
		BaseCurrencyID:     stts.BaseCurrencyID,
		BaseCode:           stts.BaseCurrencyCode,
		BaseCodeName:       codeName(stts.BaseCurrencyCode),
		Rates:              a.ratesStatus(stts),
		BaseUsedBy:         usage.currencies[usageKey(stts.BaseCurrencyLabel())],
		Currencies:         make([]CurrencySetting, 0, len(stts.Currencies)),
		PaymentMethods:     make([]PaymentMethodSetting, 0, len(stts.PaymentMethods)),
		PaymentMethodTypes: settings.PaymentMethodTypeOptions(),
		TaxTypes:           make([]TaxTypeSetting, 0, len(stts.TaxTypes)),
		IncomeCategories:   make([]CategorySetting, 0, len(stts.IncomeCategories)),
		ExpenseCategories:  make([]CategorySetting, 0, len(stts.ExpenseCategories)),
	}

	for _, c := range stts.Currencies {
		view.Currencies = append(view.Currencies, CurrencySetting{
			ID:         c.ID,
			Name:       c.CurrencyName,
			Code:       c.LinkedCode(),
			CodeName:   codeName(c.LinkedCode()),
			Linked:     stts.Linkable(c),
			RateToBase: c.RateToBase,
			UsedBy:     usage.currencies[usageKey(c.CurrencyName)],
		})
	}

	for _, p := range stts.PaymentMethods {
		view.PaymentMethods = append(view.PaymentMethods, PaymentMethodSetting{ID: p.ID, Name: p.PaymentMethodName, Type: p.PaymentMethodType, IsDefault: p.IsDefault, Currency: p.Currency, Archived: p.Archived, UsedBy: usage.paymentMethods[usageKey(p.PaymentMethodName)]})
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

	tags, err := storage.LoadTags()
	if err != nil {
		return SettingsView{}, err
	}

	tagUsage, err := storage.TagUsage()
	if err != nil {
		return SettingsView{}, err
	}

	view.Tags = make([]CategorySetting, 0, len(tags))
	for _, t := range tags {
		view.Tags = append(view.Tags, CategorySetting{ID: t.ID, Name: t.Name, Archived: t.Archived, UsedBy: tagUsage[t.UID]})
	}

	return view, nil
}

// SaveBaseCurrency changes the base currency and its link. With daily
// updates on, rates are fetched again for the new base.
func (a *API) SaveBaseCurrency(input BaseCurrencyInput) error {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return err
	}

	code, err := knownCode(input.Code)
	if err != nil {
		return err
	}

	value := strings.TrimSpace(input.Value)
	if value == "" {
		value = code
	}

	record, err := settings.BaseCurrencyRecord(value)
	if err != nil {
		return err
	}

	if code == "" {
		code = settings.BaseCodeFor(record.SettingValue, "", "")
	}

	for _, c := range stts.Currencies {
		if strings.EqualFold(strings.TrimSpace(c.CurrencyName), record.SettingValue) || (code != "" && c.LinkedCode() == code) {
			return fmt.Errorf("%q is in the currency list: delete it there first", c.CurrencyName)
		}
	}

	records := []settings.SettingRecord{
		record,
		{SettingID: settings.BaseCurrencyCodeID, SettingValue: code},
		{SettingID: settings.BaseCurrencyCodeForID, SettingValue: record.SettingValue},
	}

	if err := storage.SaveSettingRecords(records); err != nil {
		return fmt.Errorf("settings save failed: %w", err)
	}

	// Rates are relative to the base, so a new base needs new ones.
	if stts.Rates.Auto && code != "" && code != stts.BaseCurrencyCode {
		_, _ = a.refreshRates(a.context())
	}

	return nil
}

// knownCode checks raw is a known currency's code ("" stays "").
func knownCode(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	c, ok := currency.Find(raw)
	if !ok {
		return "", fmt.Errorf("unknown currency %q: pick one from the list", raw)
	}

	return c.Code, nil
}

func codeName(code string) string {
	if c, ok := currency.Find(code); ok {
		return c.Name
	}

	return ""
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

	code, err := knownCode(input.Code)
	if err != nil {
		return err
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = code
	}

	linked := code
	if linked == "" {
		linked = settings.SettingCurrency{CurrencyName: name}.LinkedCode()
	}

	if linked != "" && linked == stts.BaseCurrencyCode {
		return fmt.Errorf("%s is the base currency", linked)
	}

	for _, c := range stts.Currencies {
		if c.ID != record.ID && linked != "" && c.LinkedCode() == linked {
			return fmt.Errorf("%s is already in the list as %q", linked, c.CurrencyName)
		}
	}

	// A linked currency's rate can be left empty: it's fetched now.
	rate := input.Rate
	if strings.TrimSpace(rate) == "" && linked != "" && stts.BaseCurrencyCode != "" {
		quote, err := a.fetch(a.context(), stts.BaseCurrencyCode)
		if err != nil {
			return fmt.Errorf("couldn't fetch the rate, so type one: %w", err)
		}

		value, ok := quote.RateToBase(linked)
		if !ok {
			return fmt.Errorf("no rate is known for %s, so type one", linked)
		}

		rate = strconv.FormatFloat(value, 'g', 8, 64)
	}

	record, err = record.Apply(name, rate, time.Now())
	if err != nil {
		return err
	}

	record.Code = code

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

	// A currency in settings, or the one it already has (which may have
	// left settings since).
	currency := strings.TrimSpace(input.Currency)
	if currency != "" {
		var ok bool
		if currency, ok = matchOption(append(stts.CurrencyOptions(), record.Currency), currency); !ok {
			return fmt.Errorf("unknown currency %q: add it in settings first", input.Currency)
		}
	}

	record.Currency = currency
	record.Archived = input.Archived

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

	// Tags can go whenever: their entries just lose them.
	if kind == settingTag {
		return storage.DeleteTag(id)
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

	for _, r := range data.Budgets {
		if !r.IsTotal() {
			usage.expenseCategories[usageKey(r.Category)]++
		}
	}

	return usage
}
