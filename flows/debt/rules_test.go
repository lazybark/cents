package debt

import (
	"strings"
	"testing"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/settings"
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
		if _, err := New(false, c.peer, "$", c.amount, c.paid, c.created, c.due, "", dates.TUI, now); err == nil || err.Error() != c.want {
			t.Errorf("expected %q, got %v", c.want, err)
		}
	}

	d, err := New(true, " Alex ", "$", "100", "20", "2026-01-01", "", " lunch ", dates.ISO, now)
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
	stts := settings.AppSettings{BaseCurrency: "$", Currencies: []settings.SettingCurrency{{CurrencyName: "EUR", RateToBase: 2}}}
	items := []Debt{
		{ID: 1, Currency: "$", AmountCents: 100, AmountPaidCents: 50},
		{ID: 2, Currency: "EUR", AmountCents: 100, AmountPaidCents: 100, IsOwedToUser: true},
		{ID: 3, Currency: "BTC", AmountCents: 10, AmountPaidCents: 20, IsOwedToUser: true},
		{ID: 4, Currency: "$", AmountCents: 100, IsOwedToUser: true},
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

	if paid, total := ProgressInBaseCents(items, stts); paid != 50+200+10 || total != 100+200+10+100 {
		t.Errorf("unexpected progress %d/%d", paid, total)
	}
}
