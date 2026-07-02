package app

import (
	"github.com/lazybark/cents/flows/export"

	storage "github.com/lazybark/cents/storage/sqlite"
)

func selectedExportDataset(form export.ExportForm) storage.ExportDataset {
	if len(form.DatasetOptions) == 0 {
		return storage.ExportDatasetAll
	}

	if form.DatasetIndex < 0 || form.DatasetIndex >= len(form.DatasetOptions) {
		return form.DatasetOptions[0]
	}

	return form.DatasetOptions[form.DatasetIndex]
}

func selectedExportFormat(form export.ExportForm) storage.ExportFormat {
	if len(form.FormatOptions) == 0 {
		return storage.ExportFormatJSON
	}

	if form.FormatIndex < 0 || form.FormatIndex >= len(form.FormatOptions) {
		return form.FormatOptions[0]
	}

	return form.FormatOptions[form.FormatIndex]
}
