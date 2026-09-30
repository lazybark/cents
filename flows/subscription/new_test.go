package subscription

import (
	"strings"
	"testing"
	"time"
)

func TestNewValidatesInTUIOrder(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name, currency, amount, period, method, subType, yearly, monthly string
		want                                                             string
	}{
		{"", "", "x", "", "", "", "", "", "subscription name is required"},
		{"Git", "", "x", "", "", "", "", "", "currency is required"},
		{"Git", "$", "x", "", "", "", "", "", "payment method is required"},
		{"Git", "$", "x", "", "Card", "", "", "", "amount error: amount must be a number"},
		{"Git", "$", "-1", "", "Card", "", "", "", "amount error: amount cannot be negative"},
		{"Git", "$", "4", "", "Card", "", "2026-01-01", "", "yearly date must use DD.MM.YYYY format"},
		{"Git", "$", "4", "", "Card", "", "", "32", "monthly day must be between 1 and 31"},
		{"Git", "$", "4", "week", "Card", "", "", "", "period must be one of"},
		{"Git", "$", "4", "month", "Card", "Toy", "", "", "type must be one of"},
	}

	for _, c := range cases {
		_, err := New(c.name, c.currency, c.amount, c.period, c.method, c.subType, true, c.yearly, c.monthly, now)
		if err == nil || !strings.HasPrefix(err.Error(), c.want) {
			t.Errorf("expected %q, got %v", c.want, err)
		}
	}

	sub, err := New(" Git ", "$", "4.50", "year", "Card", "Software", false, "12.12.2026", "", now)
	if err != nil || sub.Name != "Git" || sub.AmountCents != 450 || sub.PaymentDateYearly != "12.12.2026" || sub.PaymentDayMonthly != nil || sub.IsActive {
		t.Fatalf("unexpected subscription %+v, %v", sub, err)
	}

	edited, err := sub.Edit("5", " Visa ", true, now)
	if err != nil || edited.AmountCents != 500 || edited.PaymentMethod != "Visa" || !edited.IsActive || edited.Name != "Git" {
		t.Fatalf("unexpected edit %+v, %v", edited, err)
	}

	if _, err := sub.Edit("5", " ", true, now); err == nil || err.Error() != "payment method is required" {
		t.Fatalf("expected payment method error, got %v", err)
	}
}
