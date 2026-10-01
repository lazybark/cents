package desktop

import "time"

// Debts and taxes both track an amount paid that moves through logged
// payments; these types are shared by their API methods.

type Progress struct {
	PaidCents  int64 `json:"paidCents"`
	TotalCents int64 `json:"totalCents"`
}

type PaymentLog struct {
	When       string `json:"when"`
	DeltaCents int64  `json:"deltaCents"`
	Note       string `json:"note"`
}

// PaymentInput adds Delta (signed, like "-10") to an amount paid on Date
// (YYYY-MM-DD, empty for now).
type PaymentInput struct {
	ID    uint   `json:"id"`
	Delta string `json:"delta"`
	Date  string `json:"date"`
	Note  string `json:"note"`
}

// CreatedIn names the list a new debt or tax shows up in.
type CreatedIn struct {
	Mode string `json:"mode"`
}

func formatDay(value time.Time) string {
	return value.Local().Format(logDateLayout)
}

func formatOptionalDay(value *time.Time) string {
	if value == nil {
		return ""
	}

	return formatDay(*value)
}

// overdue reports whether an unpaid item's due date is before today.
func overdue(due *time.Time, paid bool, now time.Time) bool {
	if due == nil || paid {
		return false
	}

	return formatDay(*due) < now.Format(logDateLayout)
}
