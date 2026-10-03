package sqlite

import (
	"errors"
	"fmt"
	"time"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/analytics"
	"github.com/lazybark/cents/flows/asset"
	"gorm.io/gorm/clause"
)

// LoadNetWorthSnapshots returns every month's net worth kept, oldest first.
func (s *SQLiteStorage) LoadNetWorthSnapshots() ([]analytics.NetWorthSnapshot, error) {
	var items []analytics.NetWorthSnapshot

	if err := s.db.Order("month asc").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to load net worth snapshots: %w", err)
	}

	return items, nil
}

// SaveNetWorthSnapshot keeps entry as its month's net worth, replacing what
// the app kept for that month before; a value the user entered stays.
func (s *SQLiteStorage) SaveNetWorthSnapshot(entry *analytics.NetWorthSnapshot) error {
	err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "month"}},
		Where:     clause.Where{Exprs: []clause.Expression{clause.Eq{Column: clause.Column{Table: "net_worth_snapshots", Name: "manual"}, Value: false}}},
		DoUpdates: clause.AssignmentColumns([]string{"updated_at", "net_worth_cents", "owned_cents", "owed_to_me_cents", "owed_by_me_cents"}),
	}).Create(entry).Error
	if err != nil {
		return fmt.Errorf("failed to save net worth snapshot: %w", err)
	}

	return nil
}

// SetNetWorthSnapshot keeps a net worth the user entered as its month's,
// replacing whatever was kept for it.
func (s *SQLiteStorage) SetNetWorthSnapshot(entry *analytics.NetWorthSnapshot) error {
	err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "month"}},
		DoUpdates: clause.AssignmentColumns([]string{"updated_at", "net_worth_cents", "owned_cents", "owed_to_me_cents", "owed_by_me_cents", "manual"}),
	}).Create(entry).Error
	if err != nil {
		return fmt.Errorf("failed to save net worth: %w", err)
	}

	return nil
}

// DeleteNetWorthSnapshot deletes what was kept for month, a UTC midnight.
func (s *SQLiteStorage) DeleteNetWorthSnapshot(month time.Time) error {
	result := s.db.Where("month = ?", month).Delete(&analytics.NetWorthSnapshot{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete net worth: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return errors.New("nothing is kept for that month")
	}

	return nil
}

// LoadAllAccountValueLogs returns the value history of every account.
func (s *SQLiteStorage) LoadAllAccountValueLogs() ([]account.AccountValueLog, error) {
	var logs []account.AccountValueLog

	if err := s.db.Order("log_date asc, id asc").Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to load account value logs: %w", err)
	}

	return logs, nil
}

// LoadAllAssetValueLogs returns the value history of all property and
// investments.
func (s *SQLiteStorage) LoadAllAssetValueLogs() ([]asset.AssetValueLog, error) {
	var logs []asset.AssetValueLog

	if err := s.db.Order("log_date asc, id asc").Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to load asset value logs: %w", err)
	}

	return logs, nil
}
