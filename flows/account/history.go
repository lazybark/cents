package account

import (
	"sort"
	"time"
)

// MonthValue is an account's value in a month: the last one logged in it.
type MonthValue struct {
	Month      time.Time
	Day        time.Time
	ValueCents int64
}

// MonthlyValues takes each month's last logged value, oldest month first.
// Months without a logged value are left out.
func MonthlyValues(logs []AccountValueLog) []MonthValue {
	byMonth := map[time.Time]AccountValueLog{}

	for _, entry := range logs {
		day := entry.LogDate.Local()
		month := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.Local)

		last, seen := byMonth[month]
		if !seen || entry.LogDate.After(last.LogDate) || (entry.LogDate.Equal(last.LogDate) && entry.ID > last.ID) {
			byMonth[month] = entry
		}
	}

	values := make([]MonthValue, 0, len(byMonth))
	for month, entry := range byMonth {
		values = append(values, MonthValue{Month: month, Day: entry.LogDate, ValueCents: entry.ValueCents})
	}

	sort.Slice(values, func(i, j int) bool { return values[i].Month.Before(values[j].Month) })

	return values
}
