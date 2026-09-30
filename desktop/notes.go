package desktop

import (
	"github.com/lazybark/cents/flows/note"
)

// MonthNoteInput sets the note on a month (YYYY-MM); empty text removes it.
type MonthNoteInput struct {
	Month string `json:"month"`
	Text  string `json:"text"`
}

// MonthNotes returns every month's note by month (YYYY-MM).
func (a *API) MonthNotes() (map[string]string, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return nil, err
	}

	items, err := storage.LoadMonthNotes()
	if err != nil {
		return nil, err
	}

	notes := make(map[string]string, len(items))
	for _, n := range items {
		notes[n.Month.UTC().Format(note.MonthLayout)] = n.Text
	}

	return notes, nil
}

func (a *API) SaveMonthNote(input MonthNoteInput) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	month, err := note.ParseMonth(input.Month)
	if err != nil {
		return err
	}

	text, err := note.Clean(input.Text)
	if err != nil {
		return err
	}

	return storage.SaveMonthNote(month, text)
}
