package invoice

import (
	"testing"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/settings"
)

func TestEditValidatesAndTrims(t *testing.T) {
	now := time.Now()

	if _, err := New(Fields{Title: " "}, dates.ISO, now); err == nil || err.Error() != "invoice title is required" {
		t.Fatalf("want title error, got %v", err)
	}

	if _, err := New(Fields{Title: "A", DueDate: "1.2.2026"}, dates.ISO, now); err == nil || err.Error() != "due date must use YYYY-MM-DD format" {
		t.Fatalf("want due date error, got %v", err)
	}

	if _, err := New(Fields{Title: "A", InvoiceDate: "x"}, dates.TUI, now); err == nil || err.Error() != "invoice date must use DD.MM.YYYY format" {
		t.Fatalf("want invoice date error, got %v", err)
	}

	item, err := New(Fields{Title: " Hosting ", Amount: " ", Peer: " ACME ", URL: " https://x.test "}, dates.ISO, now)
	if err != nil || item.Title != "Hosting" || item.AmountCents != 0 || item.Peer != "ACME" || item.URL != "https://x.test" || item.Mode() != ListOutgoing {
		t.Fatalf("unexpected invoice %+v %v", item, err)
	}

	item.ID = 7
	edited, err := item.Edit(Fields{Title: "Hosting", IsIncoming: true, Amount: "12.50"}, dates.ISO, now)
	if err != nil || edited.ID != 7 || edited.AmountCents != 1250 || edited.Mode() != ListIncoming {
		t.Fatalf("unexpected edit %+v %v", edited, err)
	}
}

func TestFilterSortsLikeTheTUI(t *testing.T) {
	day := func(d int) *time.Time {
		value := time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC)
		return &value
	}
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	items := []Invoice{
		{ID: 1, CreatedAt: created},
		{ID: 2, DueDate: day(5), CreatedAt: created},
		{ID: 3, DueDate: day(9), CreatedAt: created},
		{ID: 4, CreatedAt: created},
		{ID: 5, DueDate: day(5), CreatedAt: created.Add(time.Hour)},
		{ID: 6, IsIncoming: true},
		{ID: 7, Paid: true},
	}

	got := Filter(items, ListOutgoing)
	want := []uint{3, 5, 2, 4, 1}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}

	for i := range want {
		if got[i].ID != want[i] {
			t.Fatalf("order: want %v, got %+v", want, got)
		}
	}

	if incoming := Filter(items, ListIncoming); len(incoming) != 1 || incoming[0].ID != 6 {
		t.Fatalf("incoming: %+v", incoming)
	}

	if paid := Filter(items, ListPaid); len(paid) != 1 || paid[0].ID != 7 {
		t.Fatalf("paid: %+v", paid)
	}
}

func TestUnpaidInBaseCentsByDirection(t *testing.T) {
	stts := settings.AppSettings{Currencies: []settings.SettingCurrency{{CurrencyName: "EUR", RateToBase: 2}}}
	items := []Invoice{
		{Currency: "EUR", AmountCents: 100},                // paid to me
		{Currency: "$", AmountCents: 30, IsIncoming: true}, // I pay
		{Currency: "", AmountCents: 999},                   // no currency: left out
		{Currency: "EUR", AmountCents: 999, Paid: true},    // paid already
	}

	toMe, byMe := UnpaidInBaseCents(items, stts)
	if toMe != 200 || byMe != 30 {
		t.Fatalf("got to me %d, by me %d", toMe, byMe)
	}
}
