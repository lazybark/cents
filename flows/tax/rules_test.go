package tax

import (
	"testing"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/settings"
)

func TestNewAndEditValidateInTUIOrder(t *testing.T) {
	now := time.Now()
	taxType := settings.SettingTaxType{ID: 4, Country: " NL ", TaxTypeName: " VAT "}
	cases := []struct {
		due, paid, period, dueDate string
		want                       string
	}{
		{"x", "x", "", "", "period is required"},
		{"x", "x", "Q1", "", "amount due error: amount must be a number"},
		{"0", "x", "Q1", "", "amount due must be greater than zero"},
		{"10", "x", "Q1", "", "amount paid error: amount must be a number"},
		{"10", "0", "Q1", "31.31.2026", "due date must use DD.MM.YYYY format"},
	}

	for _, c := range cases {
		if _, err := New(taxType, "$", 1, c.due, c.paid, c.period, c.dueDate, "", dates.TUI, now); err == nil || err.Error() != c.want {
			t.Errorf("expected %q, got %v", c.want, err)
		}
	}

	created, err := New(taxType, "$", 1, "100", "150", " Q1 2026 ", "31.03.2026", "", dates.TUI, now)
	if err != nil || created.TaxTypeID != 4 || created.TaxCountry != "NL" || created.TaxTypeName != "VAT" || created.Period != "Q1 2026" || created.DueDate == nil || !created.IsPaid() || created.PaidPercent() != 100 || created.LeftCents() != 0 {
		t.Fatalf("unexpected tax %+v, %v", created, err)
	}

	if _, err := created.Edit("100", "0", " ", "", "", dates.TUI, now); err == nil || err.Error() != "period is required" {
		t.Fatalf("expected period error on edit, got %v", err)
	}
}

func TestApplyPaymentAndTotals(t *testing.T) {
	now := time.Now()
	item := Tax{ID: 2, AmountDueCents: 1000, AmountPaidCents: 200, RateToBase: 1}.withBaseAmounts()

	if _, _, err := item.ApplyPayment("-3", "", "", dates.ISO, now); err == nil || err.Error() != "delta makes amount paid negative" {
		t.Fatalf("unexpected %v", err)
	}

	over, entry, err := item.ApplyPayment("20", "", " early ", dates.ISO, now)
	if err != nil || over.AmountPaidCents != 2200 || entry.Note != "early" || entry.TaxID != 2 || !entry.CreatedAt.Equal(now) {
		t.Fatalf("paying more than due should work: %+v %+v %v", over, entry, err)
	}

	items := []Tax{item, over, Tax{AmountDueCents: 500, RateToBase: 1}.withBaseAmounts()}
	if len(Filter(items, ListUnpaid)) != 2 || len(Filter(items, ListPaid)) != 1 {
		t.Fatal("unexpected filter result")
	}

	if paid, total := ProgressTotals(items); paid != 200+1000 || total != 2500 {
		t.Fatalf("unexpected totals %d/%d", paid, total)
	}
}

func TestForeignCurrencyKeepsBaseAmountsAtRecordedRate(t *testing.T) {
	now := time.Now()
	taxType := settings.SettingTaxType{ID: 1, Country: "DE", TaxTypeName: "Income"}

	if _, err := New(taxType, " ", 1, "100", "0", "2026", "", "", dates.ISO, now); err == nil || err.Error() != "currency is required" {
		t.Fatalf("expected currency error, got %v", err)
	}

	if _, err := New(taxType, "EUR", 0, "100", "0", "2026", "", "", dates.ISO, now); err == nil || err.Error() != "rate must be greater than zero" {
		t.Fatalf("expected rate error, got %v", err)
	}

	item, err := New(taxType, " EUR ", 1.1, "100", "10", "2026", "", "", dates.ISO, now)
	if err != nil || item.Currency != "EUR" || item.AmountDueBaseCents != 11000 || item.AmountPaidBaseCents != 1100 || item.LeftBaseCents() != 9900 {
		t.Fatalf("unexpected tax %+v %v", item, err)
	}

	paid, _, err := item.ApplyPayment("20", "", "", dates.ISO, now)
	if err != nil || paid.AmountPaidCents != 3000 || paid.AmountPaidBaseCents != 3300 {
		t.Fatalf("payment should convert at the recorded rate: %+v %v", paid, err)
	}

	edited, err := paid.Edit("200", "30", "2026", "", "", dates.ISO, now)
	if err != nil || edited.RateToBase != 1.1 || edited.AmountDueBaseCents != 22000 {
		t.Fatalf("edit should keep the rate: %+v %v", edited, err)
	}

	fixed, err := edited.WithRate(2, now)
	if err != nil || fixed.AmountDueBaseCents != 40000 || fixed.AmountPaidBaseCents != 6000 {
		t.Fatalf("new rate should recompute base amounts: %+v %v", fixed, err)
	}

	if _, err := edited.WithRate(-1, now); err == nil {
		t.Fatal("expected a negative rate to be refused")
	}

	if paidBase, total := ProgressTotals([]Tax{fixed}); paidBase != 6000 || total != 40000 {
		t.Fatalf("totals should use base amounts: %d/%d", paidBase, total)
	}
}
