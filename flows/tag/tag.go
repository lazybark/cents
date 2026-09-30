// Package tag is about tags on incomes and expenses: labels that cut
// across categories, like "Trip to Japan", so everything about one thing
// can be found and added up together.
package tag

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// MaxLength is the longest a tag can be, in characters.
const MaxLength = 40

// Tag is a label entries can have. Entries link it by UID, so renaming it
// changes nothing else; archived tags aren't suggested for new entries.
type Tag struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	LastUpdatedAt time.Time `gorm:"not null;default:1970-01-01 00:00:00"`
	Name          string    `gorm:"not null;uniqueIndex"`
	UID           string    `gorm:"not null;default:'';index"`
	Archived      bool      `gorm:"not null;default:false"`
}

// EntryTag puts a tag on an entry.
type EntryTag struct {
	EntryID uint   `gorm:"primaryKey;autoIncrement:false"`
	TagUID  string `gorm:"primaryKey"`
}

// TableName keeps the join table's name plain.
func (EntryTag) TableName() string {
	return "cashflow_entry_tags"
}

// CleanName checks a tag's name: trimmed, not empty, no commas (they
// separate tags), at most MaxLength characters.
func CleanName(name string) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	switch {
	case name == "":
		return "", errors.New("a tag needs a name")
	case strings.Contains(name, ","):
		return "", errors.New("a tag can't have a comma: commas separate tags")
	case utf8.RuneCountInString(name) > MaxLength:
		return "", fmt.Errorf("a tag can be at most %d characters", MaxLength)
	}

	return name, nil
}

// Clean checks names and drops repeats (ignoring case), sorted.
func Clean(names []string) ([]string, error) {
	seen := map[string]bool{}
	result := make([]string, 0, len(names))
	for _, raw := range names {
		if strings.TrimSpace(raw) == "" {
			continue
		}

		name, err := CleanName(raw)
		if err != nil {
			return nil, fmt.Errorf("tag %q: %w", strings.TrimSpace(raw), err)
		}

		if key := strings.ToLower(name); !seen[key] {
			seen[key] = true
			result = append(result, name)
		}
	}

	sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i]) < strings.ToLower(result[j]) })

	return result, nil
}

// Has reports whether tags has name, ignoring case.
func Has(tags []string, name string) bool {
	for _, t := range tags {
		if strings.EqualFold(strings.TrimSpace(t), strings.TrimSpace(name)) {
			return true
		}
	}

	return false
}
