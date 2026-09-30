package sqlite

import (
	"fmt"

	"github.com/lazybark/cents/flows/budget"
)

func (s *SQLiteStorage) LoadBudgets() ([]budget.Budget, error) {
	var items []budget.Budget

	if err := s.db.Order("id asc").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to load budgets: %w", err)
	}

	l, err := loadLinks(s.db)
	if err != nil {
		return nil, err
	}

	l.fillBudgets(items)

	return items, nil
}

func (s *SQLiteStorage) CreateBudget(entry *budget.Budget) error {
	if err := s.linkBudget(entry); err != nil {
		return err
	}

	if err := s.db.Create(entry).Error; err != nil {
		return fmt.Errorf("failed to create budget: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) SaveBudget(entry *budget.Budget) error {
	if err := s.linkBudget(entry); err != nil {
		return err
	}

	if err := s.db.Save(entry).Error; err != nil {
		return fmt.Errorf("failed to save budget: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) DeleteBudget(id uint) error {
	result := s.db.Delete(&budget.Budget{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete budget: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("budget %d not found", id)
	}

	return nil
}

func (s *SQLiteStorage) linkBudget(entry *budget.Budget) error {
	l, err := loadLinks(s.db)
	if err != nil {
		return err
	}

	return l.linkBudget(entry)
}
