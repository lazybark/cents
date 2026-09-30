package export

import (
	"github.com/charmbracelet/bubbles/textinput"

	storage "github.com/lazybark/cents/storage/sqlite"
)

func NewExportForm(path string) ExportForm {
	pathInput := textinput.New()
	pathInput.Placeholder = path
	pathInput.CharLimit = 260
	pathInput.Width = 56
	pathInput.SetValue(path)

	form := ExportForm{
		PathInput:      pathInput,
		Active:         0,
		DatasetOptions: storage.ExportDatasetOptions(),
		DatasetIndex:   0,
		FormatOptions:  storage.ExportFormatOptions(),
		FormatIndex:    0,
	}

	return form.FocusActive()
}

func (f ExportForm) FocusActive() ExportForm {
	f.PathInput.Blur()

	if f.Active == ExportFieldPath {
		f.PathInput.Focus()
	}

	return f
}

func (f ExportForm) Next() ExportForm {
	if f.Active < ExportFieldCount-1 {
		f.Active++
	}

	return f.FocusActive()
}

func (f ExportForm) Prev() ExportForm {
	if f.Active > 0 {
		f.Active--
	}

	return f.FocusActive()
}
