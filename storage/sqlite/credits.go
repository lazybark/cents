package sqlite

import (
	"fmt"

	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/credit"
	"gorm.io/gorm"
)

func (s *SQLiteStorage) LoadCredits() ([]credit.Credit, error) {
	var items []credit.Credit

	if err := s.db.Order("start_date desc, id desc").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to load credits: %w", err)
	}

	return items, nil
}

func (s *SQLiteStorage) CreateCredit(entry *credit.Credit) error {
	if err := s.db.Create(entry).Error; err != nil {
		return fmt.Errorf("failed to create credit: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) SaveCredit(entry *credit.Credit) error {
	if err := s.db.Save(entry).Error; err != nil {
		return fmt.Errorf("failed to save credit: %w", err)
	}

	return nil
}

// DeleteCredit removes a credit with its log.
func (s *SQLiteStorage) DeleteCredit(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("credit_id = ?", id).Delete(&credit.CreditLog{}).Error; err != nil {
			return fmt.Errorf("failed to delete credit logs: %w", err)
		}

		if err := tx.Delete(&credit.Credit{}, id).Error; err != nil {
			return fmt.Errorf("failed to delete credit: %w", err)
		}

		return nil
	})
}

// LoadCreditLogs returns a credit's log, newest first.
func (s *SQLiteStorage) LoadCreditLogs(creditID uint) ([]credit.CreditLog, error) {
	var logs []credit.CreditLog

	if err := s.db.Where("credit_id = ?", creditID).Order("created_at desc, id desc").Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to load credit logs: %w", err)
	}

	return logs, nil
}

// AddCreditLog saves entry (the credit with the log applied) and the log
// in the same transaction, so the two can't disagree, with cash, the
// expense made for a payment, when there is one.
func (s *SQLiteStorage) AddCreditLog(entry *credit.Credit, log *credit.CreditLog, cash *cashflow.CashflowEntry) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if cash != nil {
			if err := createCashflow(tx, cash); err != nil {
				return err
			}

			log.CashflowEntryID = cash.ID
		}

		if err := tx.Save(entry).Error; err != nil {
			return fmt.Errorf("failed to save credit: %w", err)
		}

		if err := tx.Create(log).Error; err != nil {
			return fmt.Errorf("failed to create credit log: %w", err)
		}

		return nil
	})
}

// DeleteCreditLog removes one log entry and saves entry (the credit with
// that entry undone) in the same transaction.
func (s *SQLiteStorage) DeleteCreditLog(entry *credit.Credit, logID uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var log credit.CreditLog
		if err := tx.Where("id = ? AND credit_id = ?", logID, entry.ID).First(&log).Error; err == nil {
			if err := deleteLinkedCashflow(tx, log.CashflowEntryID); err != nil {
				return err
			}
		}

		result := tx.Where("id = ? AND credit_id = ?", logID, entry.ID).Delete(&credit.CreditLog{})
		if result.Error != nil {
			return fmt.Errorf("failed to delete credit log: %w", result.Error)
		}

		if result.RowsAffected == 0 {
			return ErrLogNotFound
		}

		if err := tx.Save(entry).Error; err != nil {
			return fmt.Errorf("failed to save credit: %w", err)
		}

		return nil
	})
}
