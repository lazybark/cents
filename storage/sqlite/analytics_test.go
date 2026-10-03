package sqlite

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/lazybark/cents/flows/analytics"
)

func TestEnteredNetWorthStays(t *testing.T) {
	db, _, err := OpenDatabase(filepath.Join(t.TempDir(), "cents.db"))
	if err != nil {
		t.Fatal(err)
	}
	s := NewSQLiteStorage(db)
	month := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	if err := s.SaveNetWorthSnapshot(&analytics.NetWorthSnapshot{Month: month, NetWorthCents: 100, OwnedCents: 100}); err != nil {
		t.Fatal(err)
	}

	if err := s.SetNetWorthSnapshot(&analytics.NetWorthSnapshot{Month: month, NetWorthCents: 500, Manual: true}); err != nil {
		t.Fatal(err)
	}

	// The app keeping this month again leaves the entered value.
	if err := s.SaveNetWorthSnapshot(&analytics.NetWorthSnapshot{Month: month, NetWorthCents: 200, OwnedCents: 200}); err != nil {
		t.Fatal(err)
	}

	items, _ := s.LoadNetWorthSnapshots()
	if len(items) != 1 || items[0].NetWorthCents != 500 || !items[0].Manual || items[0].OwnedCents != 0 {
		t.Fatalf("expected the entered value: %+v", items)
	}

	if err := s.DeleteNetWorthSnapshot(month); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteNetWorthSnapshot(month); err == nil {
		t.Fatal("expected nothing left to delete")
	}

	// Once deleted, the app keeps the month again.
	if err := s.SaveNetWorthSnapshot(&analytics.NetWorthSnapshot{Month: month, NetWorthCents: 300}); err != nil {
		t.Fatal(err)
	}

	if items, _ := s.LoadNetWorthSnapshots(); len(items) != 1 || items[0].NetWorthCents != 300 || items[0].Manual {
		t.Fatalf("unexpected %+v", items)
	}
}
