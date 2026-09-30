package desktop

import (
	"strings"
	"testing"
)

func TestMonthNotes(t *testing.T) {
	api := newTestAPI(t)

	if notes, err := api.MonthNotes(); err != nil || len(notes) != 0 {
		t.Fatalf("expected none: %v %v", notes, err)
	}

	if err := api.SaveMonthNote(MonthNoteInput{Month: "2026-09", Text: "  Trip to Dubai "}); err != nil {
		t.Fatal(err)
	}
	if err := api.SaveMonthNote(MonthNoteInput{Month: "2026-08", Text: "Laptop"}); err != nil {
		t.Fatal(err)
	}

	// Saving again replaces it.
	if err := api.SaveMonthNote(MonthNoteInput{Month: "2026-09", Text: "Trip to Dubai: flights and hotel"}); err != nil {
		t.Fatal(err)
	}

	notes, _ := api.MonthNotes()
	if len(notes) != 2 || notes["2026-09"] != "Trip to Dubai: flights and hotel" || notes["2026-08"] != "Laptop" {
		t.Fatalf("unexpected notes %v", notes)
	}

	// Empty removes it.
	if err := api.SaveMonthNote(MonthNoteInput{Month: "2026-08", Text: " "}); err != nil {
		t.Fatal(err)
	}
	if notes, _ = api.MonthNotes(); len(notes) != 1 {
		t.Fatalf("expected August's gone: %v", notes)
	}

	for _, bad := range []MonthNoteInput{{Month: "Sept", Text: "x"}, {Month: "2026-09", Text: strings.Repeat("a", 501)}} {
		if err := api.SaveMonthNote(bad); err == nil {
			t.Errorf("expected %+v to fail", bad)
		}
	}
}
