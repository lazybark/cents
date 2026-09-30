package sqlite

import (
	"errors"
	"fmt"
	"time"

	"github.com/lazybark/cents/flows/asset"
	"gorm.io/gorm"
)

func (s *SQLiteStorage) LoadAssets() ([]asset.Asset, error) {
	var items []asset.Asset

	if err := s.db.Order("name asc, id asc").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to load assets: %w", err)
	}

	return items, nil
}

func (s *SQLiteStorage) CreateAsset(entry *asset.Asset) error {
	if err := s.db.Create(entry).Error; err != nil {
		return fmt.Errorf("failed to create asset: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) SaveAsset(entry *asset.Asset) error {
	if err := s.db.Save(entry).Error; err != nil {
		return fmt.Errorf("failed to save asset: %w", err)
	}

	return nil
}

// DeleteAsset removes an asset with its value history.
func (s *SQLiteStorage) DeleteAsset(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("asset_id = ?", id).Delete(&asset.AssetValueLog{}).Error; err != nil {
			return fmt.Errorf("failed to delete asset value logs: %w", err)
		}

		if err := tx.Delete(&asset.Asset{}, id).Error; err != nil {
			return fmt.Errorf("failed to delete asset: %w", err)
		}

		return nil
	})
}

// LoadAssetValueLogs returns an asset's value history, newest first.
func (s *SQLiteStorage) LoadAssetValueLogs(assetID uint) ([]asset.AssetValueLog, error) {
	var logs []asset.AssetValueLog

	if err := s.db.Where("asset_id = ?", assetID).Order("log_date desc, id desc").Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to load asset value logs: %w", err)
	}

	return logs, nil
}

// UpsertAssetValueLog records the asset's value on day, replacing that
// day's entry if there is one.
func (s *SQLiteStorage) UpsertAssetValueLog(assetID uint, day time.Time, valueCents int64) error {
	if assetID == 0 {
		return errors.New("asset is required")
	}

	normalizedDay := accountLogDay(day)
	entry := asset.AssetValueLog{}

	err := s.db.Where("asset_id = ? AND log_date = ?", assetID, normalizedDay).First(&entry).Error
	if err == nil {
		entry.ValueCents = valueCents

		return s.db.Save(&entry).Error
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to check for existing asset value log: %w", err)
	}

	entry = asset.AssetValueLog{AssetID: assetID, LogDate: normalizedDay, ValueCents: valueCents}
	if err := s.db.Create(&entry).Error; err != nil {
		return fmt.Errorf("failed to create asset value log: %w", err)
	}

	return nil
}

// DeleteAssetValueLog removes one value history entry of an asset.
func (s *SQLiteStorage) DeleteAssetValueLog(assetID uint, logID uint) error {
	result := s.db.Where("id = ? AND asset_id = ?", logID, assetID).Delete(&asset.AssetValueLog{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete asset value log: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrLogNotFound
	}

	return nil
}
