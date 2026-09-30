package cashflow

import "github.com/lazybark/cents/flows/settings"

// WithoutArchived drops entries in archived categories, for statistics:
// they stay in the month's entries and its totals, but not in the charts.
// left is how many were dropped.
func WithoutArchived(entries []CashflowEntry, stts settings.AppSettings) (kept []CashflowEntry, left int) {
	kept = make([]CashflowEntry, 0, len(entries))
	for _, entry := range entries {
		if stts.IsArchivedCategory(entry.IsIncome, entry.Category) {
			left++

			continue
		}

		kept = append(kept, entry)
	}

	return kept, left
}
