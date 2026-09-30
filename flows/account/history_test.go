package account

import (
	"testing"
	"time"
)

func TestMonthlyValuesTakeEachMonthsLastValue(t *testing.T) {
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.Local) }
	logs := []AccountValueLog{
		{ID: 1, LogDate: day(2026, 3, 31), ValueCents: 300},
		{ID: 2, LogDate: day(2026, 1, 5), ValueCents: 100},
		{ID: 3, LogDate: day(2026, 1, 28), ValueCents: 150},
		{ID: 4, LogDate: day(2026, 1, 10), ValueCents: 120},
	}

	got := MonthlyValues(logs)
	if len(got) != 2 {
		t.Fatalf("expected January and March only, got %+v", got)
	}

	if got[0].Month != day(2026, 1, 1) || got[0].ValueCents != 150 || got[0].Day != day(2026, 1, 28) {
		t.Fatalf("unexpected January %+v", got[0])
	}

	if got[1].Month != day(2026, 3, 1) || got[1].ValueCents != 300 {
		t.Fatalf("unexpected March %+v", got[1])
	}

	if MonthlyValues(nil) == nil || len(MonthlyValues(nil)) != 0 {
		t.Fatal("expected an empty list")
	}
}
