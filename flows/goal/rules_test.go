package goal

import (
	"testing"
	"time"

	"github.com/lazybark/cents/dates"
)

func TestNewValidatesLikeTheTUI(t *testing.T) {
	now := time.Now()
	cases := map[string][]string{
		"goal name is required":                         {" ", "100", "0", "2026-01-01", ""},
		"target amount must be greater than zero":       {"Car", "0", "0", "2026-01-01", ""},
		"accumulated amount cannot be more than target": {"Car", "100", "101", "2026-01-01", ""},
		"start date is required":                        {"Car", "100", "0", "", ""},
		"target date must use YYYY-MM-DD format":        {"Car", "100", "0", "2026-01-01", "01.02.2027"},
	}

	for want, in := range cases {
		if _, err := New(in[0], "$", in[1], in[2], "", in[3], in[4], dates.ISO, now); err == nil || err.Error() != want {
			t.Errorf("%v: want %q, got %v", in, want, err)
		}
	}

	g, err := New(" Car ", "$", "100", "100", " new ", "2026-01-01", "2027-01-01", dates.ISO, now)
	if err != nil || g.Name != "Car" || g.Description != "new" || g.TargetDate == nil || !g.IsDone() || g.LeftCents() != 0 || g.Percent() != 100 {
		t.Fatalf("unexpected goal %+v %v", g, err)
	}
}

func TestRemoveDeltaUndoesApplyDelta(t *testing.T) {
	now := time.Now()
	g := Goal{ID: 3, TargetAmountCents: 10000, AmountAccumulatedCents: 2000}

	updated, entry, err := g.ApplyDelta("+30", "", "", dates.ISO, now)
	if err != nil || updated.AmountAccumulatedCents != 5000 || entry.Note != "manual accumulated adjustment" || entry.GoalID != 3 {
		t.Fatalf("apply: %+v %+v %v", updated, entry, err)
	}

	if _, _, err := g.ApplyDelta("-21", "", "", dates.ISO, now); err == nil || err.Error() != "delta makes accumulated amount out of range" {
		t.Fatalf("expected range error, got %v", err)
	}

	back, err := updated.RemoveDelta(entry, now)
	if err != nil || back.AmountAccumulatedCents != 2000 {
		t.Fatalf("remove: %+v %v", back, err)
	}

	// Lowered by hand since: undoing the +30 would go below zero.
	updated.AmountAccumulatedCents = 1000
	if _, err := updated.RemoveDelta(entry, now); err == nil {
		t.Fatal("expected out of range error")
	}

	if _, err := (Goal{ID: 4, TargetAmountCents: 1}).RemoveDelta(entry, now); err == nil {
		t.Fatal("expected another goal's entry to be refused")
	}
}

func TestFilterSplitsActiveAndDone(t *testing.T) {
	items := []Goal{{ID: 1, TargetAmountCents: 10, AmountAccumulatedCents: 10}, {ID: 2, TargetAmountCents: 10}}

	if active := Filter(items, ListActive); len(active) != 1 || active[0].ID != 2 {
		t.Fatalf("active: %+v", active)
	}

	if done := Filter(items, ListDone); len(done) != 1 || done[0].ID != 1 {
		t.Fatalf("done: %+v", done)
	}
}
