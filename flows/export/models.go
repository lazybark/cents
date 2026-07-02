package export

import (
	"github.com/charmbracelet/bubbles/textinput"

	storage "github.com/lazybark/cents/storage/sqlite"
)

type ExportForm struct {
	PathInput      textinput.Model
	Active         int
	DatasetOptions []storage.ExportDataset
	DatasetIndex   int
	FormatOptions  []storage.ExportFormat
	FormatIndex    int
}

const (
	ExportFieldDataset = iota
	ExportFieldFormat
	ExportFieldPath
	ExportFieldRun
	ExportFieldCount
)
