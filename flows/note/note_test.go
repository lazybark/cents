package note

import (
	"strings"
	"testing"
	"time"
)

func TestParseAndClean(t *testing.T) {
	if m, err := ParseMonth(" 2026-09 "); err != nil || !m.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected %v %v", m, err)
	}

	for _, bad := range []string{"", "2026-13", "09.2026"} {
		if _, err := ParseMonth(bad); err == nil {
			t.Errorf("expected %q to fail", bad)
		}
	}

	if got, err := Clean("  Trip to Dubai\r\nflights + hotel  "); err != nil || got != "Trip to Dubai\nflights + hotel" {
		t.Fatalf("unexpected %q %v", got, err)
	}

	if got, _ := Clean("   "); got != "" {
		t.Fatal("blank is no note")
	}

	if _, err := Clean(strings.Repeat("ж", MaxLength)); err != nil {
		t.Fatalf("500 characters fit, whatever the bytes: %v", err)
	}

	if _, err := Clean(strings.Repeat("a", MaxLength+1)); err == nil {
		t.Fatal("expected a long note to fail")
	}
}
