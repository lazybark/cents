package report

import (
	"bytes"
	"encoding/csv"
)

// bom marks the file as UTF-8, so spreadsheets (Excel especially) read
// currency signs like € and ₾ right.
const bom = "\uFEFF"

// CSV is t as a CSV file: comma-separated, header first.
func CSV(t Table) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(bom)

	w := csv.NewWriter(&buf)
	if err := w.Write(t.Header); err != nil {
		return nil, err
	}

	if err := w.WriteAll(t.Rows); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
