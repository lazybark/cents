package sqlite

import (
	"fmt"
	"time"

	"github.com/lazybark/cents/flows/note"
	"gorm.io/gorm/clause"
)

// LoadMonthNotes returns every month's note, oldest first.
func (s *SQLiteStorage) LoadMonthNotes() ([]note.MonthNote, error) {
	var items []note.MonthNote

	if err := s.db.Order("month asc").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to load month notes: %w", err)
	}

	return items, nil
}

// SaveMonthNote keeps text as month's note (a UTC first day), replacing the
// one it had; empty text deletes it.
func (s *SQLiteStorage) SaveMonthNote(month time.Time, text string) error {
	if text == "" {
		if err := s.db.Where("month = ?", month).Delete(&note.MonthNote{}).Error; err != nil {
			return fmt.Errorf("failed to delete month note: %w", err)
		}

		return nil
	}

	entry := note.MonthNote{Month: month, Text: text}
	err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "month"}},
		DoUpdates: clause.AssignmentColumns([]string{"updated_at", "text"}),
	}).Create(&entry).Error
	if err != nil {
		return fmt.Errorf("failed to save month note: %w", err)
	}

	return nil
}
