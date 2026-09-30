package analytics

import (
	"testing"
	"time"

	"github.com/lazybark/cents/flows/account"
)

func TestManual(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.Local)

	got, err := Manual(" 2025-11 ", "-1234.5", now)
	if err != nil || !got.Month.Equal(d(2025, 11, 1)) || got.NetWorthCents != -123450 || !got.Manual {
		t.Fatalf("unexpected %+v %v", got, err)
	}

	if got, err := Manual("2026-04", "0", now); err != nil || got.NetWorthCents != 0 {
		t.Fatalf("this month and zero are fine: %+v %v", got, err)
	}

	for _, bad := range [][2]string{{"", "1"}, {"2026-13", "1"}, {"04.2026", "1"}, {"2026-05", "1"}, {"2026-01", ""}, {"2026-01", "abc"}} {
		if _, err := Manual(bad[0], bad[1], now); err == nil {
			t.Errorf("expected %q / %q to fail", bad[0], bad[1])
		}
	}

	// An entered month shows as entered, and isn't estimated; March has
	// nothing to go by and is left out, April is estimated from a log.
	h := History{
		Settings:    stts,
		Snapshots:   []NetWorthSnapshot{{Month: d(2026, 2, 1), NetWorthCents: 900, Manual: true}},
		Accounts:    []account.Account{{ID: 1, Currency: "€"}},
		AccountLogs: []account.AccountValueLog{{AccountID: 1, LogDate: d(2026, 4, 2), ValueCents: 400}},
	}
	history := NetWorthHistory(h, now)
	if len(history) != 2 || !history[0].Manual || history[0].Estimated || history[0].Cents != 900 || !history[1].Estimated || history[1].Month.Month() != time.April {
		t.Fatalf("unexpected %+v", history)
	}
}
