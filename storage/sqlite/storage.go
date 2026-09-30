package sqlite

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/cashflow"
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

func NewSQLiteStorage() (*SQLiteStorage, error) {
	db, _, _, err := OpenDatabase()
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	return &SQLiteStorage{db: db}, nil
}

func OpenDatabase() (*gorm.DB, string, bool, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return nil, "", false, fmt.Errorf("failed to get working directory: %w", err)
	}

	dbPath := filepath.Join(workingDir, "cents.db")
	_, statErr := os.Stat(dbPath)
	created := os.IsNotExist(statErr)
	if statErr != nil && !os.IsNotExist(statErr) {
		return nil, "", false, statErr
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, "", false, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	if err := db.AutoMigrate(&account.Account{}, &account.AccountValueLog{}, &subscription.Subscription{}, &debt.Debt{}, &debt.DebtLog{}, &goal.Goal{}, &goal.GoalLog{}, &tax.Tax{}, &tax.TaxLog{}, &invoice.Invoice{}, &cashflow.CashflowEntry{}, &settings.SettingRecord{}, &settings.SettingCurrency{}, &settings.SettingPaymentMethod{}, &settings.SettingTaxType{}, &settings.SettingIncomeCategory{}, &settings.SettingExpenseCategory{}); err != nil {
		return nil, "", false, fmt.Errorf("failed to auto-migrate SQLite database: %w", err)
	}

	if err := EnsureSettingsDefaults(db); err != nil {
		return nil, "", false, fmt.Errorf("failed to ensure settings defaults: %w", err)
	}

	return db, dbPath, created, nil
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
	for _, row := range rows {
		switch row.SettingID {
		case "base_currency":
			if strings.TrimSpace(row.SettingValue) != "" {
				stts.BaseCurrency = row.SettingValue
			}
		}
	}

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
