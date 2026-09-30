package sqlite

import (
	"strings"
	"testing"
	"time"

	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/tag"
)

func TestTags(t *testing.T) {
	s, _ := openTest(t)
	if err := s.SaveSettingExpenseCategory(&settings.SettingExpenseCategory{CategoryName: "Food"}); err != nil {
		t.Fatal(err)
	}

	tagsOf := func() map[string]string {
		entries, err := s.LoadCashflows()
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		for _, e := range entries {
			got[e.Comment] = strings.Join(e.Tags, ",")
		}
		return got
	}

	for _, e := range []cashflow.CashflowEntry{
		{Category: "Food", Comment: "sushi", Tags: []string{"Trip to Japan", " food ", "trip to japan"}, EntryDate: time.Now()},
		{Category: "Food", Comment: "ramen", Tags: []string{"trip to japan"}, EntryDate: time.Now()},
		{Category: "Food", Comment: "bread", EntryDate: time.Now()},
	} {
		if err := s.CreateCashflow(&e); err != nil {
			t.Fatal(err)
		}
	}

	if got := tagsOf(); got["sushi"] != "food,Trip to Japan" || got["ramen"] != "Trip to Japan" || got["bread"] != "" {
		t.Fatalf("unexpected tags %v", got)
	}

	tags, _ := s.LoadTags()
	if len(tags) != 2 || tags[0].UID == "" {
		t.Fatalf("expected two tags with UIDs: %+v", tags)
	}

	// Editing an entry sets its tags to what it says.
	entries, _ := s.LoadCashflows()
	for i := range entries {
		if entries[i].Comment == "sushi" {
			entries[i].Tags = []string{"Trip to Japan"}
		}
	}
	if err := s.SaveCashflows(entries); err != nil {
		t.Fatal(err)
	}
	if got := tagsOf(); got["sushi"] != "Trip to Japan" || got["ramen"] != "Trip to Japan" {
		t.Fatalf("unexpected tags %v", got)
	}

	if err := s.CreateCashflow(&cashflow.CashflowEntry{Category: "Food", Tags: []string{"a,b"}, EntryDate: time.Now()}); err == nil {
		t.Fatal("expected a comma in a tag to fail")
	}

	usage, _ := s.TagUsage()
	japan := tags[1]
	if japan.Name != "Trip to Japan" || usage[japan.UID] != 2 {
		t.Fatalf("unexpected usage %v for %+v", usage, japan)
	}

	// Renaming shows everywhere; a name another tag has is refused.
	japan.Name = "Japan 2026"
	if err := s.SaveTag(&japan); err != nil {
		t.Fatal(err)
	}
	if got := tagsOf(); got["ramen"] != "Japan 2026" {
		t.Fatalf("expected the new name: %v", got)
	}
	clash := tag.Tag{ID: japan.ID, Name: "FOOD"}
	if err := s.SaveTag(&clash); err == nil {
		t.Fatal("expected a clash to fail")
	}

	// Merging: an entry with both keeps one.
	food := tags[0]
	entries, _ = s.LoadCashflows()
	for i := range entries {
		if entries[i].Comment == "ramen" {
			entries[i].Tags = []string{"food", "Japan 2026"}
		}
	}
	if err := s.SaveCashflows(entries); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Merge("tag", food.ID, japan.ID); err != nil {
		t.Fatal(err)
	}
	if got := tagsOf(); got["ramen"] != "Japan 2026" || got["sushi"] != "Japan 2026" {
		t.Fatalf("unexpected after merge %v", got)
	}

	// Deleting a tag takes it off entries; deleting an entry its links.
	if err := s.DeleteTag(japan.ID); err != nil {
		t.Fatal(err)
	}
	if got := tagsOf(); got["ramen"] != "" {
		t.Fatalf("expected no tags: %v", got)
	}

	entry := cashflow.CashflowEntry{Category: "Food", Tags: []string{"x"}, EntryDate: time.Now()}
	if err := s.CreateCashflow(&entry); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCashflow(entry.ID); err != nil {
		t.Fatal(err)
	}
	var left int64
	s.db.Model(&tag.EntryTag{}).Where("entry_id = ?", entry.ID).Count(&left)
	if left != 0 {
		t.Fatal("expected the entry's tags gone with it")
	}
}
