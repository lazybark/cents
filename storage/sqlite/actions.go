package sqlite

import (
	"errors"
	"fmt"
	"time"

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

func (s *SQLiteStorage) LoadAppSettings() (settings.AppSettings, error) {
	return LoadAppSettings(s.db)
}

func (s *SQLiteStorage) ExportData(request ExportRequest) (ExportResult, error) {
	return ExportData(s.db, request)
}

func (s *SQLiteStorage) LoadAccountValueLogs(accountID uint) ([]account.AccountValueLog, error) {
	var logs []account.AccountValueLog

	if err := s.db.Where("account_id = ?", accountID).Order("log_date desc, id desc").Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to load account value logs: %w", err)
	}

	return logs, nil
}

func (s *SQLiteStorage) UpsertAccountValueLog(accountID uint, day time.Time, valueCents int64) error {
	if accountID == 0 {
		return errors.New("account is required")
	}

	normalizedDay := accountLogDay(day)
	now := time.Now()
	entry := account.AccountValueLog{}

	err := s.db.Where("account_id = ? AND log_date = ?", accountID, normalizedDay).First(&entry).Error
	if err == nil {
		entry.ValueCents = valueCents
		entry.UpdatedAt = now
		return s.db.Save(&entry).Error
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to check for existing account value log: %w", err)
	}

	entry = account.AccountValueLog{
		AccountID:  accountID,
		LogDate:    normalizedDay,
		ValueCents: valueCents,
		UpdatedAt:  now,
	}

	err = s.db.Create(&entry).Error
	if err != nil {
		return fmt.Errorf("failed to create account value log: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) CreateAccount(entry *account.Account) error {
	err := s.db.Create(entry).Error
	if err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) UpdateAccountAmount(id uint, amountCents int64, ignoreInSummaries bool, updatedAt time.Time) error {
	return s.db.Model(&account.Account{}).Where("id = ?", id).Updates(map[string]any{
		"balance_cents":       amountCents,
		"leftover_cents":      amountCents,
		"ignore_in_summaries": ignoreInSummaries,
		"last_updated_at":     updatedAt,
	}).Error
}

func (s *SQLiteStorage) DeleteAccount(id uint) error {
	if err := s.db.Where("account_id = ?", id).Delete(&account.AccountValueLog{}).Error; err != nil {
		return fmt.Errorf("failed to delete account value logs: %w", err)
	}

	return s.db.Delete(&account.Account{}, id).Error
}

func (s *SQLiteStorage) CreateSubscription(entry *subscription.Subscription) error {
	err := s.db.Create(entry).Error
	if err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) SaveSubscription(entry *subscription.Subscription) error {
	err := s.db.Save(entry).Error
	if err != nil {
		return fmt.Errorf("failed to save subscription: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) DeleteSubscription(id uint) error {
	err := s.db.Delete(&subscription.Subscription{}, id).Error
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) CreateDebt(entry *debt.Debt) error {
	err := s.db.Create(entry).Error
	if err != nil {
		return fmt.Errorf("failed to create debt: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) SaveDebt(entry *debt.Debt) error {
	err := s.db.Save(entry).Error
	if err != nil {
		return fmt.Errorf("failed to save debt: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) CreateDebtLog(entry *debt.DebtLog) error {
	err := s.db.Create(entry).Error
	if err != nil {
		return fmt.Errorf("failed to create debt log: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) LoadDebtLogs(debtID uint) ([]debt.DebtLog, error) {
	var logs []debt.DebtLog

	err := s.db.Where("debt_id = ?", debtID).Order("created_at desc, id desc").Limit(20).Find(&logs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to load debt logs: %w", err)
	}

	return logs, nil
}

func (s *SQLiteStorage) DeleteDebt(id uint) error {
	if err := s.db.Where("debt_id = ?", id).Delete(&debt.DebtLog{}).Error; err != nil {
		return fmt.Errorf("failed to delete debt logs: %w", err)
	}

	return s.db.Delete(&debt.Debt{}, id).Error
}

func (s *SQLiteStorage) CreateGoal(entry *goal.Goal) error {
	err := s.db.Create(entry).Error
	if err != nil {
		return fmt.Errorf("failed to create goal: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) SaveGoal(entry *goal.Goal) error {
	err := s.db.Save(entry).Error
	if err != nil {
		return fmt.Errorf("failed to save goal: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) CreateGoalLog(entry *goal.GoalLog) error {
	err := s.db.Create(entry).Error
	if err != nil {
		return fmt.Errorf("failed to create goal log: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) LoadGoalLogs(goalID uint) ([]goal.GoalLog, error) {
	var logs []goal.GoalLog

	err := s.db.Where("goal_id = ?", goalID).Order("created_at desc, id desc").Limit(20).Find(&logs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to load goal logs: %w", err)
	}

	return logs, nil
}

func (s *SQLiteStorage) DeleteGoal(id uint) error {
	if err := s.db.Where("goal_id = ?", id).Delete(&goal.GoalLog{}).Error; err != nil {
		return fmt.Errorf("failed to delete goal logs: %w", err)
	}

	return s.db.Delete(&goal.Goal{}, id).Error
}

func (s *SQLiteStorage) CreateTax(entry *tax.Tax) error {
	err := s.db.Create(entry).Error
	if err != nil {
		return fmt.Errorf("failed to create tax: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) SaveTax(entry *tax.Tax) error {
	err := s.db.Save(entry).Error
	if err != nil {
		return fmt.Errorf("failed to save tax: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) CreateTaxLog(entry *tax.TaxLog) error {
	err := s.db.Create(entry).Error
	if err != nil {
		return fmt.Errorf("failed to create tax log: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) LoadTaxLogs(taxID uint) ([]tax.TaxLog, error) {
	var logs []tax.TaxLog

	err := s.db.Where("tax_id = ?", taxID).Order("created_at desc, id desc").Limit(20).Find(&logs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to load tax logs: %w", err)
	}

	return logs, nil
}

func (s *SQLiteStorage) DeleteTax(id uint) error {
	err := s.db.Where("tax_id = ?", id).Delete(&tax.TaxLog{}).Error
	if err != nil {
		return fmt.Errorf("failed to delete tax logs: %w", err)
	}

	err = s.db.Delete(&tax.Tax{}, id).Error
	if err != nil {
		return fmt.Errorf("failed to delete tax: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) CreateCashflow(entry *cashflow.CashflowEntry) error {
	err := s.db.Create(entry).Error
	if err != nil {
		return fmt.Errorf("failed to create cashflow entry: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) DeleteCashflow(id uint) error {
	err := s.db.Delete(&cashflow.CashflowEntry{}, id).Error
	if err != nil {
		return fmt.Errorf("failed to delete cashflow entry: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) CreteInvoice(entry *invoice.Invoice) error {
	err := s.db.Create(entry).Error
	if err != nil {
		return fmt.Errorf("failed to create invoice entry: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) DeleteSetting(targetType string, id uint) error {
	switch targetType {
	case "currency":
		return s.db.Delete(&settings.SettingCurrency{}, id).Error
	case "payment_method":
		if err := s.db.Delete(&settings.SettingPaymentMethod{}, id).Error; err != nil {
			return err
		}
		return EnsurePaymentMethodDefaults(s.db)
	case "tax_type":
		return s.db.Delete(&settings.SettingTaxType{}, id).Error
	case "income_category":
		return s.db.Delete(&settings.SettingIncomeCategory{}, id).Error
	case "expense_category":
		return s.db.Delete(&settings.SettingExpenseCategory{}, id).Error
	default:
		return fmt.Errorf("unknown setting type %q", targetType)
	}
}

func (s *SQLiteStorage) SaveSettingRecord(entry *settings.SettingRecord) error {
	err := s.db.Save(entry).Error
	if err != nil {
		return fmt.Errorf("failed to save setting record: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) SaveSettingCurrency(entry *settings.SettingCurrency) error {
	err := s.db.Save(entry).Error
	if err != nil {
		return fmt.Errorf("failed to save setting currency: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) SaveSettingPaymentMethod(entry *settings.SettingPaymentMethod) error {
	if entry.IsDefault {
		if err := s.db.Model(&settings.SettingPaymentMethod{}).Where("id <> ?", entry.ID).Update("is_default", false).Error; err != nil {
			return fmt.Errorf("failed to unset other default payment methods: %w", err)
		}
	}

	if err := s.db.Save(entry).Error; err != nil {
		return fmt.Errorf("failed to save setting payment method: %w", err)
	}

	var defaultCount int64
	if err := s.db.Model(&settings.SettingPaymentMethod{}).Where("is_default = ?", true).Count(&defaultCount).Error; err == nil && defaultCount == 0 {
		return s.db.Model(&settings.SettingPaymentMethod{}).Where("id = ?", entry.ID).Update("is_default", true).Error
	}

	return nil
}

func (s *SQLiteStorage) SaveSettingTaxType(entry *settings.SettingTaxType) error {
	err := s.db.Save(entry).Error
	if err != nil {
		return fmt.Errorf("failed to save setting tax type: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) SaveSettingIncomeCategory(entry *settings.SettingIncomeCategory) error {
	err := s.db.Save(entry).Error
	if err != nil {
		return fmt.Errorf("failed to save setting income category: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) SaveSettingExpenseCategory(entry *settings.SettingExpenseCategory) error {
	err := s.db.Save(entry).Error
	if err != nil {
		return fmt.Errorf("failed to save setting expense category: %w", err)
	}

	return nil
}

func accountLogDay(value time.Time) time.Time {
	local := value.Local()

	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
}

func (s *SQLiteStorage) SaveInvoice(entry *invoice.Invoice) error {
	err := s.db.Save(entry).Error
	if err != nil {
		return fmt.Errorf("failed to save invoice entry: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) DeleteInvoice(id uint) error {
	err := s.db.Delete(&invoice.Invoice{}, id).Error
	if err != nil {
		return fmt.Errorf("failed to delete invoice entry: %w", err)
	}

	return nil
}
