package debt

import (
	"strings"
	"testing"
	"time"

	"github.com/lazybark/cents/dates"
)

func TestNewValidatesInTUIOrder(t *testing.T) {
	now := time.Now()
	cases := []struct {
		peer, amount, paid, created, due string
		want                             string
	}{
		{"", "x", "x", "", "", "peer is required"},
		{"Bank", "x", "x", "", "", "amount error: amount must be a number"},
		{"Bank", "100", "-1", "", "", "amount paid error: amount cannot be negative"},
		{"Bank", "100", "101", "", "", "amount paid cannot be more than amount"},
		{"Bank", "100", "0", "", "", "created date is required"},
		{"Bank", "100", "0", "2026-01-01", "", "created date must use DD.MM.YYYY format"},
		{"Bank", "100", "0", "01.01.2026", "x", "due date must use DD.MM.YYYY format"},
	}

	for _, c := range cases {
		if _, err := New(false, c.peer, "$", 1, c.amount, c.paid, c.created, c.due, "", dates.TUI, now); err == nil || err.Error() != c.want {
			t.Errorf("expected %q, got %v", c.want, err)
		}
	}

	d, err := New(true, " Alex ", "$", 1, "100", "20", "2026-01-01", "", " lunch ", dates.ISO, now)
	if err != nil || d.Peer != "Alex" || d.AmountPaidCents != 2000 || d.DueDate != nil || d.Comment != "lunch" || !d.IsOwedToUser || d.LeftCents() != 8000 {
		t.Fatalf("unexpected debt %+v, %v", d, err)
	}
}

func TestApplyPaymentKeepsPaidInRange(t *testing.T) {
	now := time.Date(2026, 10, 1, 15, 30, 0, 0, time.Local)
	d := Debt{ID: 7, AmountCents: 10000, AmountPaidCents: 2000}

	for delta, want := range map[string]string{"": "log delta error: delta is required", "0": "delta cannot be zero", "-30": "delta makes amount paid out of range", "81": "delta makes amount paid out of range"} {
		if _, _, err := d.ApplyPayment(delta, "", "", dates.ISO, now); err == nil || err.Error() != want {
			t.Errorf("delta %q: expected %q, got %v", delta, want, err)
		}
	}

	paid, entry, err := d.ApplyPayment("+80", "2026-09-15", "", dates.ISO, now)
	if err != nil || !paid.IsPaid() || entry.DebtID != 7 || entry.DeltaPaidCents != 8000 || entry.Note != "manual paid adjustment" {
		t.Fatalf("unexpected payment %+v %+v %v", paid, entry, err)
	}

	if entry.CreatedAt.Day() != 15 || entry.CreatedAt.Hour() != 15 {
		t.Fatalf("expected log on the given day at the current time, got %v", entry.CreatedAt)
	}
}

func TestFilterAndProgress(t *testing.T) {
	// EUR was recorded at 2; BTC has no rate and counts with raw amounts.
	items := []Debt{
		Debt{ID: 1, Currency: "$", AmountCents: 100, AmountPaidCents: 50, RateToBase: 1}.withBaseAmounts(),
		Debt{ID: 2, Currency: "EUR", AmountCents: 100, AmountPaidCents: 100, IsOwedToUser: true, RateToBase: 2}.withBaseAmounts(),
		{ID: 3, Currency: "BTC", AmountCents: 10, AmountPaidCents: 20, IsOwedToUser: true},
		Debt{ID: 4, Currency: "$", AmountCents: 100, IsOwedToUser: true, RateToBase: 1}.withBaseAmounts(),
	}

	ids := func(list []Debt) string {
		parts := []string{}
		for _, d := range list {
			parts = append(parts, string(rune('0'+d.ID)))
		}

		return strings.Join(parts, ",")
	}

	if got := ids(Filter(items, ListOutgoing)); got != "1" {
		t.Errorf("outgoing: %s", got)
	}

	if got := ids(Filter(items, ListIncoming)); got != "4" {
		t.Errorf("incoming: %s", got)
	}

	if got := ids(Filter(items, ListPaid)); got != "2,3" {
		t.Errorf("paid: %s", got)
	}

	if paid, total := ProgressInBaseCents(items); paid != 50+200+10 || total != 100+200+10+100 {
		t.Errorf("unexpected progress %d/%d", paid, total)
	}
}

func TestRemovePaymentUndoesIt(t *testing.T) {
	now := time.Now()
	d := Debt{ID: 3, AmountCents: 10000, AmountPaidCents: 3000}

	undone, err := d.RemovePayment(DebtLog{DebtID: 3, DeltaPaidCents: 1000}, now)
	if err != nil || undone.AmountPaidCents != 2000 || !undone.LastUpdatedAt.Equal(now) {
		t.Fatalf("unexpected %+v %v", undone, err)
	}

	undone, err = d.RemovePayment(DebtLog{DebtID: 3, DeltaPaidCents: -2000}, now)
	if err != nil || undone.AmountPaidCents != 5000 {
		t.Fatalf("removing a negative payment adds it back: %+v %v", undone, err)
	}

	if _, err := d.RemovePayment(DebtLog{DebtID: 3, DeltaPaidCents: 4000}, now); err == nil || !strings.HasPrefix(err.Error(), "removing this payment makes amount paid out of range") {
		t.Fatalf("expected range error, got %v", err)
	}

	if _, err := d.RemovePayment(DebtLog{DebtID: 9, DeltaPaidCents: 1}, now); err == nil {
		t.Fatal("expected error for another debt's payment")
	}
}

func TestRecordedRateKeepsBaseAmounts(t *testing.T) {
	now := time.Now()
	d, err := New(false, "Bank", "EUR", 1.2, "100", "10", "2026-01-01", "", "", dates.ISO, now)
	if err != nil || d.AmountBaseCents != 12000 || d.AmountPaidBaseCents != 1200 {
		t.Fatalf("unexpected debt %+v %v", d, err)
	}

	if left, ok := d.LeftBaseCents(); !ok || left != 10800 {
		t.Fatalf("unexpected left %d %v", left, ok)
	}

	paid, _, err := d.ApplyPayment("40", "", "", dates.ISO, now)
	if err != nil || paid.AmountPaidBaseCents != 6000 {
		t.Fatalf("payment should convert at the recorded rate: %+v %v", paid, err)
	}

	back, err := paid.RemovePayment(DebtLog{DebtID: paid.ID, DeltaPaidCents: 4000}, now)
	if err != nil || back.AmountPaidBaseCents != 1200 {
		t.Fatalf("undo should convert at the recorded rate: %+v %v", back, err)
	}

	fixed, err := paid.WithRate(2, now)
	if err != nil || fixed.AmountBaseCents != 20000 || fixed.AmountPaidBaseCents != 10000 {
		t.Fatalf("new rate not applied: %+v %v", fixed, err)
	}

	if _, err := paid.WithRate(0, now); err == nil {
		t.Fatal("expected a zero rate to be refused")
	}

	none, _ := New(false, "Friend", "BTC", 0, "100", "40", "2026-01-01", "", "", dates.ISO, now)
	if _, ok := none.LeftBaseCents(); ok {
		t.Fatal("a debt without a rate has no base amount")
	}

	// Without a rate a debt counts with its raw amounts in progress.
	if p, total := ProgressInBaseCents([]Debt{d, none}); p != 1200+4000 || total != 12000+10000 {
		t.Fatalf("unexpected progress %d/%d", p, total)
	}
}
