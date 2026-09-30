package sqlite

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/tag"
	"gorm.io/gorm"
)

// Entries have tags through cashflow_entry_tags, linked by UID like
// categories (see links.go): loading fills each entry's Tags with names,
// saving an entry makes its tags what Tags says, adding a tag for a name
// there's none for yet.

// fillTags puts each entry's tag names on it, sorted.
func fillTags(db *gorm.DB, entries []cashflow.CashflowEntry) error {
	var tags []tag.Tag
	if err := db.Find(&tags).Error; err != nil {
		return fmt.Errorf("failed to load tags: %w", err)
	}

	names := make(map[string]string, len(tags))
	for _, t := range tags {
		names[t.UID] = t.Name
	}

	var links []tag.EntryTag
	if err := db.Find(&links).Error; err != nil {
		return fmt.Errorf("failed to load entry tags: %w", err)
	}

	byEntry := map[uint][]string{}
	for _, l := range links {
		if name, ok := names[l.TagUID]; ok {
			byEntry[l.EntryID] = append(byEntry[l.EntryID], name)
		}
	}

	for i := range entries {
		got := byEntry[entries[i].ID]
		sort.Slice(got, func(a, b int) bool { return strings.ToLower(got[a]) < strings.ToLower(got[b]) })
		entries[i].Tags = got
	}

	return nil
}

// syncTags makes e's tags the ones its Tags names.
func syncTags(tx *gorm.DB, e *cashflow.CashflowEntry) error {
	names, err := tag.Clean(e.Tags)
	if err != nil {
		return err
	}

	var tags []tag.Tag
	if err := tx.Order("archived asc, id asc").Find(&tags).Error; err != nil {
		return fmt.Errorf("failed to load tags: %w", err)
	}

	uids := map[string]string{}
	for _, t := range tags {
		if _, ok := uids[nameKey(t.Name)]; !ok {
			uids[nameKey(t.Name)] = t.UID
		}
	}

	if err := tx.Where("entry_id = ?", e.ID).Delete(&tag.EntryTag{}).Error; err != nil {
		return fmt.Errorf("failed to clear the entry's tags: %w", err)
	}

	for _, name := range names {
		uid, ok := uids[nameKey(name)]
		if !ok {
			t := tag.Tag{Name: name, CreatedAt: time.Now(), LastUpdatedAt: time.Now()}
			if err := tx.Create(&t).Error; err != nil {
				return fmt.Errorf("failed to add tag %q: %w", name, err)
			}

			uid = t.UID
			uids[nameKey(name)] = uid
		}

		if err := tx.Create(&tag.EntryTag{EntryID: e.ID, TagUID: uid}).Error; err != nil {
			return fmt.Errorf("failed to tag the entry: %w", err)
		}
	}

	e.Tags = names
	return nil
}

func untagEntry(tx *gorm.DB, id uint) error {
	if err := tx.Where("entry_id = ?", id).Delete(&tag.EntryTag{}).Error; err != nil {
		return fmt.Errorf("failed to remove the entry's tags: %w", err)
	}

	return nil
}

// LoadTags returns every tag, by name (ignoring case).
func (s *SQLiteStorage) LoadTags() ([]tag.Tag, error) {
	var items []tag.Tag
	if err := s.db.Order("lower(name) asc, id asc").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to load tags: %w", err)
	}

	return items, nil
}

// TagUsage counts the entries with each tag, by UID.
func (s *SQLiteStorage) TagUsage() (map[string]int, error) {
	var rows []struct {
		TagUID string
		N      int
	}
	if err := s.db.Model(&tag.EntryTag{}).Select("tag_uid, count(*) AS n").Group("tag_uid").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to count tags: %w", err)
	}

	usage := make(map[string]int, len(rows))
	for _, r := range rows {
		usage[r.TagUID] = r.N
	}

	return usage, nil
}

// SaveTag adds a tag (ID 0) or renames or archives one; names are unique,
// ignoring case.
func (s *SQLiteStorage) SaveTag(entry *tag.Tag) error {
	name, err := tag.CleanName(entry.Name)
	if err != nil {
		return err
	}

	var clash int64
	if err := s.db.Model(&tag.Tag{}).Where("lower(name) = lower(?) AND id <> ?", name, entry.ID).Count(&clash).Error; err != nil {
		return err
	}

	// SQLite's lower() only knows ASCII; check the rest here.
	var all []tag.Tag
	if err := s.db.Where("id <> ?", entry.ID).Find(&all).Error; err != nil {
		return err
	}
	for _, t := range all {
		if nameKey(t.Name) == nameKey(name) {
			clash++
		}
	}

	if clash > 0 {
		return fmt.Errorf("there's a tag called %q already", name)
	}

	entry.Name = name
	entry.LastUpdatedAt = time.Now()
	if entry.ID == 0 {
		entry.CreatedAt = entry.LastUpdatedAt
		return s.db.Create(entry).Error
	}

	return s.db.Model(&tag.Tag{}).Where("id = ?", entry.ID).Updates(map[string]any{"name": entry.Name, "archived": entry.Archived, "last_updated_at": entry.LastUpdatedAt}).Error
}

// DeleteTag deletes a tag, taking it off every entry that had it.
func (s *SQLiteStorage) DeleteTag(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		uid, err := uidOf(tx, "tags", id)
		if err != nil {
			return err
		}

		if err := tx.Where("tag_uid = ?", uid).Delete(&tag.EntryTag{}).Error; err != nil {
			return err
		}

		return tx.Delete(&tag.Tag{}, id).Error
	})
}

// mergeTags moves every entry with the tag from onto into (entries with
// both keep one) and deletes from.
func mergeTags(tx *gorm.DB, from, into uint) (int64, error) {
	if from == into {
		return 0, errors.New("pick another one to merge into")
	}

	fromUID, err := uidOf(tx, "tags", from)
	if err != nil {
		return 0, err
	}

	intoUID, err := uidOf(tx, "tags", into)
	if err != nil {
		return 0, err
	}

	both := tx.Model(&tag.EntryTag{}).Select("entry_id").Where("tag_uid = ?", intoUID)
	if err := tx.Where("tag_uid = ? AND entry_id IN (?)", fromUID, both).Delete(&tag.EntryTag{}).Error; err != nil {
		return 0, err
	}

	result := tx.Model(&tag.EntryTag{}).Where("tag_uid = ?", fromUID).Update("tag_uid", intoUID)
	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, tx.Delete(&tag.Tag{}, from).Error
}
