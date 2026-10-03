package sqlite

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/analytics"
	"github.com/lazybark/cents/flows/asset"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/credit"
	"github.com/lazybark/cents/flows/debt"
	"github.com/lazybark/cents/flows/goal"
	"github.com/lazybark/cents/flows/invoice"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/subscription"
	"github.com/lazybark/cents/flows/tax"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type SQLiteStorage struct {
	db *gorm.DB
}

func NewSQLiteStorage(db *gorm.DB) *SQLiteStorage {
	return &SQLiteStorage{db: db}
}

// OpenDatabase opens (creating it if missing) the database at dbPath and
// migrates it to the current schema. created reports whether the file was new.
func OpenDatabase(dbPath string) (*gorm.DB, bool, error) {
	_, statErr := os.Stat(dbPath)
	created := os.IsNotExist(statErr)
	if statErr != nil && !os.IsNotExist(statErr) {
		return nil, false, statErr
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, false, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	if err := db.AutoMigrate(&account.Account{}, &account.AccountValueLog{}, &subscription.Subscription{}, &debt.Debt{}, &debt.DebtLog{}, &goal.Goal{}, &goal.GoalLog{}, &tax.Tax{}, &tax.TaxLog{}, &invoice.Invoice{}, &cashflow.CashflowEntry{}, &settings.SettingRecord{}, &settings.SettingCurrency{}, &settings.SettingPaymentMethod{}, &settings.SettingTaxType{}, &settings.SettingIncomeCategory{}, &settings.SettingExpenseCategory{}, &asset.Asset{}, &asset.AssetValueLog{}, &credit.Credit{}, &credit.CreditLog{}, &subscription.SubscriptionPayment{}, &analytics.NetWorthSnapshot{}); err != nil {
		return nil, false, fmt.Errorf("failed to auto-migrate SQLite database: %w", err)
	}

	if err := EnsureSettingsDefaults(db); err != nil {
		return nil, false, fmt.Errorf("failed to ensure settings defaults: %w", err)
	}

	if err := backfillTaxCurrencies(db); err != nil {
		return nil, false, fmt.Errorf("failed to set currencies of older taxes: %w", err)
	}

	if err := backfillRecordedRates(db); err != nil {
		return nil, false, fmt.Errorf("failed to set rates of older records: %w", err)
	}

	if err := markCurrenciesSetUp(db); err != nil {
		return nil, false, fmt.Errorf("failed to check currency setup: %w", err)
	}

	if err := sortOutObligations(db); err != nil {
		return nil, false, fmt.Errorf("failed to sort out obligations: %w", err)
	}

	if err := recordPaidMarks(db); err != nil {
		return nil, false, fmt.Errorf("failed to record payments marked paid: %w", err)
	}

	return db, created, nil
}

// CheckDatabase reports whether dbPath is an existing cents database, without
// modifying it. Any SQLite file that has the settings table qualifies.
func CheckDatabase(dbPath string) error {
	info, err := os.Stat(dbPath)
	if err != nil {
		return fmt.Errorf("failed to read database file: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("%s is a directory, not a database file", dbPath)
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to open SQLite database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to access SQLite connection: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	var count int64
	if err := db.Raw("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", "setting_records").Scan(&count).Error; err != nil {
		return fmt.Errorf("%s is not a SQLite database: %w", dbPath, err)
	}

	if count == 0 {
		return fmt.Errorf("%s is not a cents database (no settings table)", dbPath)
	}

	return nil
}

// backfillTaxCurrencies gives taxes recorded before taxes had a currency
// (they have no rate) the base currency they were entered in: rate 1 and
// base amounts equal to their amounts.
func backfillTaxCurrencies(db *gorm.DB) error {
	var base settings.SettingRecord
	if err := db.Where("setting_id = ?", settings.BaseCurrencySettingID).First(&base).Error; err != nil {
		return err
	}

	return db.Model(&tax.Tax{}).
		Where("rate_to_base IS NULL OR rate_to_base <= 0").
		Updates(map[string]any{
			"currency":               strings.TrimSpace(base.SettingValue),
			"rate_to_base":           1,
			"amount_due_base_cents":  gorm.Expr("amount_due_cents"),
			"amount_paid_base_cents": gorm.Expr("amount_paid_cents"),
		}).Error
}

// recordedRateTables are the dated records that keep the rate they were
// entered with, and their amount columns with the base amount column each
// is kept in.
var recordedRateTables = []struct {
	table   string
	amounts map[string]string
}{
	{"cashflow_entries", map[string]string{"amount_cents": "amount_base_cents"}},
	{"debts", map[string]string{"amount_cents": "amount_base_cents", "amount_paid_cents": "amount_paid_base_cents"}},
	{"invoices", map[string]string{"amount_cents": "amount_base_cents"}},
}

// backfillRecordedRates gives records saved before they kept a rate (it is
// NULL) the best guess there is: 1 in the base currency, otherwise the
// currency's rate in settings now, or 0 (no rate) when it has none. The
// rates can be corrected afterwards.
func backfillRecordedRates(db *gorm.DB) error {
	stts, err := LoadAppSettings(db)
	if err != nil {
		return err
	}

	rates := []struct {
		currency string
		rate     float64
	}{{stts.BaseCurrencyLabel(), 1}}

	for _, currency := range stts.Currencies {
		if currency.RateToBase > 0 && !stts.IsBase(currency.CurrencyName) {
			rates = append(rates, struct {
				currency string
				rate     float64
			}{strings.TrimSpace(currency.CurrencyName), currency.RateToBase})
		}
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, t := range recordedRateTables {
			for _, r := range rates {
				updates := map[string]any{"rate_to_base": r.rate}
				for amount, base := range t.amounts {
					updates[base] = gorm.Expr("CAST(ROUND("+amount+" * ?) AS INTEGER)", r.rate)
				}

				if err := tx.Table(t.table).Where("rate_to_base IS NULL AND lower(trim(currency)) = lower(?)", r.currency).Updates(updates).Error; err != nil {
					return fmt.Errorf("%s: %w", t.table, err)
				}
			}

			updates := map[string]any{"rate_to_base": 0}
			for _, base := range t.amounts {
				updates[base] = 0
			}

			if err := tx.Table(t.table).Where("rate_to_base IS NULL").Updates(updates).Error; err != nil {
				return fmt.Errorf("%s: %w", t.table, err)
			}
		}

		return nil
	})
}

// recordPaidMarks turns a latest-paid date set before payments were
// recorded into records: one per payment it covers (the last 12 at most),
// so they show in the log and can be deleted. Subscriptions that have
// records already are left alone.
func recordPaidMarks(db *gorm.DB) error {
	var subs []subscription.Subscription
	if err := db.Where("last_paid_date IS NOT NULL AND id NOT IN (?)", db.Model(&subscription.SubscriptionPayment{}).Select("subscription_id")).Find(&subs).Error; err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, sub := range subs {
			for _, paidFor := range sub.PaidUpTo(time.Now(), 12) {
				if err := tx.Create(&subscription.SubscriptionPayment{SubscriptionID: sub.ID, PaidFor: paidFor}).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})
}

// obligationsSortedID marks a database whose older subscriptions were
// sorted into subscriptions and obligations, which happens once.
const obligationsSortedID = "obligations_sorted"

// sortOutObligations makes older subscriptions of obligation-like types
// (rent, insurance…) obligations, once; after that, what the user picks
// stays as it is.
func sortOutObligations(db *gorm.DB) error {
	var marked int64
	if err := db.Model(&settings.SettingRecord{}).Where("setting_id = ?", obligationsSortedID).Count(&marked).Error; err != nil || marked > 0 {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		types := []string{}
		for _, t := range subscription.ObligationTypesByDefault() {
			types = append(types, strings.ToLower(t))
		}

		if err := tx.Model(&subscription.Subscription{}).Where("lower(trim(type)) IN ?", types).Update("is_obligation", true).Error; err != nil {
			return err
		}

		return tx.Save(&settings.SettingRecord{SettingID: obligationsSortedID, SettingValue: "1"}).Error
	})
}

// markCurrenciesSetUp treats a database that already has currencies or
// records as set up, so only a new, empty one asks to pick currencies.
func markCurrenciesSetUp(db *gorm.DB) error {
	var marked int64
	if err := db.Model(&settings.SettingRecord{}).Where("setting_id = ?", settings.CurrenciesSetUpID).Count(&marked).Error; err != nil || marked > 0 {
		return err
	}

	for _, model := range []any{&settings.SettingCurrency{}, &account.Account{}, &cashflow.CashflowEntry{}, &debt.Debt{}, &invoice.Invoice{}, &tax.Tax{}} {
		var count int64
		if err := db.Model(model).Count(&count).Error; err != nil {
			return err
		}

		if count > 0 {
			return db.Save(&settings.SettingRecord{SettingID: settings.CurrenciesSetUpID, SettingValue: "1"}).Error
		}
	}

	return nil
}

func EnsureSettingsDefaults(db *gorm.DB) error {
	var count int64

	if err := db.Model(&settings.SettingRecord{}).Where("setting_id = ?", "base_currency").Count(&count).Error; err != nil {
		return fmt.Errorf("failed to count base_currency settings: %w", err)
	}

	if count == 0 {
		if err := db.Create(&settings.SettingRecord{SettingID: "base_currency", SettingValue: "$"}).Error; err != nil {
			return fmt.Errorf("failed to create default base_currency setting: %w", err)
		}
	}

	count = 0
	if err := db.Model(&settings.SettingPaymentMethod{}).Where("is_default = ?", true).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to count default payment methods: %w", err)
	}

	if count == 0 {
		if err := db.Create(&settings.SettingPaymentMethod{PaymentMethodName: "Other", PaymentMethodType: "Other", IsDefault: true}).Error; err != nil {
			return fmt.Errorf("failed to create default payment method: %w", err)
		}
	}

	if err := EnsurePaymentMethodDefaults(db); err != nil {
		return fmt.Errorf("failed to ensure payment method defaults: %w", err)
	}

	return nil
}

func EnsurePaymentMethodDefaults(db *gorm.DB) error {
	var methods []settings.SettingPaymentMethod

	if err := db.Order("created_at asc, id asc").Find(&methods).Error; err != nil {
		return fmt.Errorf("failed to find payment methods: %w", err)
	}

	if len(methods) == 0 {
		return db.Create(&settings.SettingPaymentMethod{PaymentMethodName: "Other", PaymentMethodType: "Other", IsDefault: true}).Error
	}

	defaultCount := 0
	for _, method := range methods {
		if method.IsDefault {
			defaultCount++
		}
	}

	if defaultCount == 0 {
		return db.Model(&settings.SettingPaymentMethod{}).Where("id = ?", methods[0].ID).Update("is_default", true).Error
	}

	if defaultCount > 1 {
		first := true
		for _, method := range methods {
			if method.IsDefault {
				if first {
					first = false
					continue
				}
				if err := db.Model(&settings.SettingPaymentMethod{}).Where("id = ?", method.ID).Update("is_default", false).Error; err != nil {
					return fmt.Errorf("failed to update payment method default status: %w", err)
				}
			}
		}
	}

	return nil
}

func LoadAppSettings(db *gorm.DB) (settings.AppSettings, error) {
	var rows []settings.SettingRecord

	if err := db.Order("setting_id asc").Find(&rows).Error; err != nil {
		return settings.AppSettings{}, fmt.Errorf("failed to load app settings: %w", err)
	}

	stts := settings.AppSettings{BaseCurrency: "$"}
	var baseCode, baseCodeFor string
	for _, row := range rows {
		value := strings.TrimSpace(row.SettingValue)
		switch row.SettingID {
		case settings.BaseCurrencySettingID:
			if value != "" {
				stts.BaseCurrency = row.SettingValue
			}
		case settings.BaseCurrencyCodeID:
			baseCode = value
		case settings.BaseCurrencyCodeForID:
			baseCodeFor = value
		case settings.RatesAutoID:
			stts.Rates.Auto = value == "1"
		case settings.RatesUpdatedAtID:
			stts.Rates.UpdatedAt, _ = time.Parse(time.RFC3339, value)
		case settings.RatesSourceID:
			stts.Rates.Source = value
		case settings.RatesDateID:
			stts.Rates.Date = value
		case settings.CurrenciesSetUpID:
			stts.CurrenciesSetUp = value == "1"
		}
	}

	stts.BaseCurrencyCode = settings.BaseCodeFor(stts.BaseCurrency, baseCode, baseCodeFor)

	var currencies []settings.SettingCurrency

	if err := db.Order("currency_name asc, id asc").Find(&currencies).Error; err != nil {
		return settings.AppSettings{}, fmt.Errorf("failed to load currencies: %w", err)
	}

	stts.Currencies = currencies

	var paymentMethods []settings.SettingPaymentMethod

	if err := db.Order("is_default desc, payment_method_name asc, id asc").Find(&paymentMethods).Error; err != nil {
		return settings.AppSettings{}, fmt.Errorf("failed to load payment methods: %w", err)
	}

	stts.PaymentMethods = paymentMethods

	var taxTypes []settings.SettingTaxType

	if err := db.Order("country asc, tax_type_name asc, id asc").Find(&taxTypes).Error; err != nil {
		return settings.AppSettings{}, fmt.Errorf("failed to load tax types: %w", err)
	}

	stts.TaxTypes = taxTypes

	var incomeCategories []settings.SettingIncomeCategory

	if err := db.Order("category_name asc, id asc").Find(&incomeCategories).Error; err != nil {
		return settings.AppSettings{}, fmt.Errorf("failed to load income categories: %w", err)
	}

	stts.IncomeCategories = incomeCategories

	var expenseCategories []settings.SettingExpenseCategory

	if err := db.Order("category_name asc, id asc").Find(&expenseCategories).Error; err != nil {
		return settings.AppSettings{}, fmt.Errorf("failed to load expense categories: %w", err)
	}

	stts.ExpenseCategories = expenseCategories

	return stts, nil
}

func LoadAccounts(db *gorm.DB) ([]account.Account, error) {
	var accounts []account.Account

	if err := db.Order("balance_cents desc, created_at desc, id desc").Find(&accounts).Error; err != nil {
		return nil, fmt.Errorf("failed to load accounts: %w", err)
	}

	return accounts, nil
}

func LoadDebts(db *gorm.DB) ([]debt.Debt, error) {
	var debts []debt.Debt

	if err := db.Order("amount_cents desc, created_at desc, id desc").Find(&debts).Error; err != nil {
		return nil, fmt.Errorf("failed to load debts: %w", err)
	}

	return debts, nil
}

func LoadCashflows(db *gorm.DB) ([]cashflow.CashflowEntry, error) {
	var entries []cashflow.CashflowEntry

	if err := db.Order("entry_date desc, created_at desc, id desc").Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("failed to load cashflows: %w", err)
	}

	return entries, nil
}

func LoadTaxes(db *gorm.DB) ([]tax.Tax, error) {
	var taxes []tax.Tax

	if err := db.Order("amount_due_cents desc, created_at desc, id desc").Find(&taxes).Error; err != nil {
		return nil, fmt.Errorf("failed to load taxes: %w", err)
	}

	return taxes, nil
}

func LoadSubscriptions(db *gorm.DB) ([]subscription.Subscription, error) {
	var subscriptions []subscription.Subscription

	if err := db.Order("amount_cents desc, created_at desc, id desc").Find(&subscriptions).Error; err != nil {
		return nil, fmt.Errorf("failed to load subscriptions: %w", err)
	}

	return subscriptions, nil
}

func LoadGoals(db *gorm.DB) ([]goal.Goal, error) {
	var goals []goal.Goal

	if err := db.Order("target_amount_cents desc, created_at desc, id desc").Find(&goals).Error; err != nil {
		return nil, fmt.Errorf("failed to load goals: %w", err)
	}

	return goals, nil
}

func LoadInvoices(db *gorm.DB) ([]invoice.Invoice, error) {
	var invoices []invoice.Invoice

	if err := db.Order("created_at desc, id desc").Find(&invoices).Error; err != nil {
		return nil, fmt.Errorf("failed to load invoices: %w", err)
	}

	return invoices, nil
}
