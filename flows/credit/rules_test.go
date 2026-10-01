package credit

import (
	"testing"
	"time"

	"github.com/lazybark/cents/dates"
)

func TestNewValidates(t *testing.T) {
	now := time.Now()
	ok := Fields{Name: "Mortgage", Total: "100000", StartDate: "2020-01-01"}
	with := func(change func(*Fields)) Fields {
		f := ok
		change(&f)
		return f
	}

	cases := map[string]Fields{
		"name is required":                         with(func(f *Fields) { f.Name = " " }),
		"amount must be greater than zero":         with(func(f *Fields) { f.Total = "0" }),
		"amount paid cannot be more than amount":   with(func(f *Fields) { f.Paid = "100001" }),
		"interest must be a number":                with(func(f *Fields) { f.InterestPercent = "five" }),
		"interest cannot be negative":              with(func(f *Fields) { f.InterestPercent = "-1" }),
		"start date is required":                   with(func(f *Fields) { f.StartDate = "" }),
		"due date cannot be before the start date": with(func(f *Fields) { f.DueDate = "2019-12-31" }),
		"due date must use YYYY-MM-DD format":      with(func(f *Fields) { f.DueDate = "31.12.2040" }),
	}

	for want, f := range cases {
		if _, err := New("$", 1, f, dates.ISO, now); err == nil || err.Error() != want {
			t.Errorf("want %q, got %v", want, err)
		}
	}

	c, err := New(" EUR ", 1.1, with(func(f *Fields) { f.InterestPercent = " 4,5% "; f.Paid = "1000"; f.Issuer = " Bank " }), dates.ISO, now)
	if err != nil || c.Currency != "EUR" || c.InterestPercent != 4.5 || c.Issuer != "Bank" || c.TotalBaseCents != 11000000 || c.PaidBaseCents != 110000 || c.LeftCents() != 9900000 {
		t.Fatalf("unexpected credit %+v %v", c, err)
	}
}

func TestLogsApplyAndUndo(t *testing.T) {
	now := time.Now()
	c, _ := New("EUR", 2, Fields{Name: "Car", Total: "1000", StartDate: "2026-01-01"}, dates.ISO, now)
	c.ID = 5

	paid, payment, err := c.ApplyLog(LogPayment, "300", "2026-02-01", "", dates.ISO, now)
	if err != nil || paid.PaidCents != 30000 || paid.PaidBaseCents != 60000 || payment.DeltaPaidCents != 30000 || payment.Note != "payment" || payment.Kind() != LogPayment || payment.CreditID != 5 {
		t.Fatalf("payment: %+v %+v %v", paid, payment, err)
	}

	if _, _, err := paid.ApplyLog(LogPayment, "700.01", "", "", dates.ISO, now); err == nil || err.Error() != "payment is more than what is left; add to the credit first" {
		t.Fatalf("expected overpayment error, got %v", err)
	}

	grown, addition, err := paid.ApplyLog(LogAddition, "50", "", " interest ", dates.ISO, now)
	if err != nil || grown.TotalCents != 105000 || grown.TotalBaseCents != 210000 || addition.Note != "interest" || addition.Kind() != LogAddition {
		t.Fatalf("addition: %+v %+v %v", grown, addition, err)
	}

	if _, _, err := grown.ApplyLog(LogKind("refund"), "1", "", "", dates.ISO, now); err == nil {
		t.Fatal("expected unknown kind error")
	}

	back, err := grown.RemoveLog(addition, now)
	if err != nil || back.TotalCents != 100000 || back.PaidCents != 30000 {
		t.Fatalf("undo addition: %+v %v", back, err)
	}

	back, err = back.RemoveLog(payment, now)
	if err != nil || back.PaidCents != 0 || back.PaidBaseCents != 0 {
		t.Fatalf("undo payment: %+v %v", back, err)
	}

	// Paid down by hand since: undoing the addition would leave paid > total.
	grown.PaidCents = 105000
	if _, err := grown.RemoveLog(addition, now); err == nil {
		t.Fatal("expected out of range error")
	}

	if _, err := (Credit{ID: 9, TotalCents: 1}).RemoveLog(payment, now); err == nil {
		t.Fatal("expected another credit's entry to be refused")
	}
}

func TestTotals(t *testing.T) {
	now := time.Now()
	a, _ := New("EUR", 2, Fields{Name: "a", Total: "100", Paid: "25", StartDate: "2026-01-01"}, dates.ISO, now)
	b, _ := New("BTC", 0, Fields{Name: "b", Total: "10", Paid: "10", StartDate: "2026-01-01"}, dates.ISO, now)
	c, _ := New("$", 1, Fields{Name: "c", Total: "50", StartDate: "2026-01-01"}, dates.ISO, now)
	items := []Credit{a, b, c}

	if len(Filter(items, ListActive)) != 2 || len(Filter(items, ListPaid)) != 1 {
		t.Fatal("unexpected filter")
	}

	if paid, total := ProgressInBaseCents(items); paid != 5000+1000 || total != 20000+1000+5000 {
		t.Fatalf("unexpected progress %d/%d", paid, total)
	}

	if left := UnpaidInBaseCents(items); left != 15000+5000 {
		t.Fatalf("unexpected unpaid %d", left)
	}
}
