package invoice

import (
	"testing"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/settings"
)

func TestEditValidatesAndTrims(t *testing.T) {
	now := time.Now()

	if _, err := New(Fields{Title: " "}, settings.AppSettings{}, dates.ISO, now); err == nil || err.Error() != "invoice title is required" {
		t.Fatalf("want title error, got %v", err)
	}

	if _, err := New(Fields{Title: "A", DueDate: "1.2.2026"}, settings.AppSettings{}, dates.ISO, now); err == nil || err.Error() != "due date must use YYYY-MM-DD format" {
		t.Fatalf("want due date error, got %v", err)
	}

	if _, err := New(Fields{Title: "A", InvoiceDate: "x"}, settings.AppSettings{}, dates.TUI, now); err == nil || err.Error() != "invoice date must use DD.MM.YYYY format" {
		t.Fatalf("want invoice date error, got %v", err)
	}

	item, err := New(Fields{Title: " Hosting ", Amount: " ", Peer: " ACME ", URL: " https://x.test "}, settings.AppSettings{}, dates.ISO, now)
	if err != nil || item.Title != "Hosting" || item.AmountCents != 0 || item.Peer != "ACME" || item.URL != "https://x.test" || item.Mode() != ListOutgoing {
		t.Fatalf("unexpected invoice %+v %v", item, err)
	}

	item.ID = 7
	edited, err := item.Edit(Fields{Title: "Hosting", IsIncoming: true, Amount: "12.50"}, settings.AppSettings{}, dates.ISO, now)
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

func TestUnpaidInBaseCentsAtRecordedRates(t *testing.T) {
	now := time.Now()
	stts := settings.AppSettings{Currencies: []settings.SettingCurrency{{CurrencyName: "EUR", RateToBase: 2}}}
	newInvoice := func(f Fields) Invoice {
		item, err := New(f, stts, dates.ISO, now)
		if err != nil {
			t.Fatal(err)
		}

		return item
	}

	items := []Invoice{
		newInvoice(Fields{Title: "paid to me", Currency: "EUR", Amount: "1"}),
		newInvoice(Fields{Title: "I pay", Currency: "$", Amount: "0.30", IsIncoming: true, Rate: "9"}),
		newInvoice(Fields{Title: "no currency", Amount: "9.99", Rate: "3"}),
		newInvoice(Fields{Title: "paid", Currency: "EUR", Amount: "9.99", Paid: true}),
	}

	if items[1].RateToBase != 1 || items[2].RateToBase != 0 {
		t.Fatalf("base currency should use 1 and no currency no rate: %+v", items)
	}

	// A later rate change in settings doesn't move recorded invoices.
	stts.Currencies[0].RateToBase = 5

	toMe, byMe := UnpaidInBaseCents(items)
	if toMe != 200 || byMe != 30 {
		t.Fatalf("got to me %d, by me %d", toMe, byMe)
	}

	kept, err := items[0].Edit(Fields{Title: "paid to me", Currency: "eur", Amount: "3"}, stts, dates.ISO, now)
	if err != nil || kept.RateToBase != 2 || kept.AmountBaseCents != 600 {
		t.Fatalf("same currency should keep the recorded rate: %+v %v", kept, err)
	}

	typed, err := items[0].Edit(Fields{Title: "paid to me", Currency: "EUR", Amount: "3", Rate: "1.5"}, stts, dates.ISO, now)
	if err != nil || typed.RateToBase != 1.5 || typed.AmountBaseCents != 450 {
		t.Fatalf("typed rate should win: %+v %v", typed, err)
	}

	moved, err := items[1].Edit(Fields{Title: "I pay", Currency: "EUR", Amount: "1"}, stts, dates.ISO, now)
	if err != nil || moved.RateToBase != 5 {
		t.Fatalf("a new currency should take its rate in settings now: %+v %v", moved, err)
	}

	if _, err := items[0].Edit(Fields{Title: "x", Currency: "EUR", Rate: "-1"}, stts, dates.ISO, now); err == nil || err.Error() != "rate must be greater than zero" {
		t.Fatalf("expected rate error, got %v", err)
	}
}
