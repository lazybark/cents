package dates

import (
	"testing"
	"time"
)

func TestDatesReadAsSaved(t *testing.T) {
	west := time.FixedZone("west", -8*3600)
	east := time.FixedZone("east", 3*3600)

	for _, saved := range []time.Time{
		time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), // typed in a form
		time.Date(2026, 3, 1, 0, 0, 0, 0, east),     // a log's day, typed east of UTC
		time.Date(2026, 3, 1, 0, 0, 0, 0, west),
		time.Date(2026, 3, 1, 23, 30, 0, 0, east), // a moment that day
	} {
		if got := Day(saved); !got.Equal(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("%v: day %v", saved, got)
		}

		if got := MonthOf(saved); got.Year() != 2026 || got.Month() != time.March || got.Day() != 1 || got.Location() != time.Local {
			t.Errorf("%v: month %v", saved, got)
		}

		if got := Text(saved); got != "2026-03-01" {
			t.Errorf("%v: text %q", saved, got)
		}
	}
}
