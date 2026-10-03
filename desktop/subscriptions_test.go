package desktop

import (
	"strings"
	"testing"
	"time"

	"github.com/lazybark/cents/flows/subscription"
)

func inDays(n int) string {
	return time.Now().AddDate(0, 0, n).Format(logDateLayout)
}

func TestSubscriptionLifecycle(t *testing.T) {
	api := newTestAPI(t)

	inputs := []SubscriptionInput{
		{Name: "Music", Type: "Multimedia", Currency: "$", Amount: "10", Period: "month", PaymentMethod: "Other", NextPayment: inDays(3), IsActive: true},
		{Name: "Domain", Type: "Domain", Currency: "eur", Amount: "20", Period: "year", PaymentMethod: "Visa", NextPayment: inDays(60), IsActive: true},
		{Name: "Rent", Type: "Rent", Currency: "$", Amount: "900", Period: "month", PaymentMethod: "Bank", NextPayment: inDays(10), IsActive: true},
		{Name: "Insurance", Type: "Insurance", Currency: "$", Amount: "300", Period: "quarter", PaymentMethod: "Bank", IsActive: true},
		{Name: "Old", Type: "My own type", Currency: "$", Amount: "3", Period: "month", PaymentMethod: "Other", NextPayment: inDays(1), IsActive: false},
	}
	for _, input := range inputs {
		if err := api.CreateSubscription(input); err != nil {
			t.Fatal(err)
		}
	}

	view, err := api.Subscriptions("active", "subscription")
	if err != nil {
		t.Fatal(err)
	}

	names := []string{}
	for _, s := range view.Subscriptions {
		names = append(names, s.Name)
	}

	if strings.Join(names, ",") != "Music,Rent,Domain,Insurance" {
		t.Fatalf("expected next payment first, unscheduled last: %v", names)
	}

	music, domain := view.Subscriptions[0], view.Subscriptions[2]
	if !music.DueSoon || music.DueInDays != 3 || music.NextPayment != inDays(3) || domain.DueSoon || domain.Currency != "EUR" || domain.BaseCents != 2160 || view.Subscriptions[3].NextPayment != "" {
		t.Fatalf("unexpected rows %+v", view.Subscriptions)
	}

	if view.Active.MonthlyCents != 1000+90000 || view.Active.YearlyCents != 12000+2160+1080000+120000 || view.Inactive.MonthlyCents != 300 {
		t.Fatalf("unexpected totals %+v %+v", view.Active, view.Inactive)
	}

	if strings.Join(view.Options.Periods, ",") != "week,month,quarter,year" || !containsFold(view.Options.Types, "Insurance") || !containsFold(view.Options.Types, "My own type") {
		t.Fatalf("unexpected options %+v", view.Options)
	}

	// Everything can change: name, type, currency, period, next payment.
	if err := api.UpdateSubscription(SubscriptionInput{ID: domain.ID, Name: "example.com", Type: "Domain", Currency: "$", Amount: "25", Period: "month", PaymentMethod: "Mastercard", NextPayment: inDays(2), IsActive: true}); err != nil {
		t.Fatal(err)
	}

	view, _ = api.Subscriptions("active", "subscription")
	var edited SubscriptionRow
	for _, s := range view.Subscriptions {
		if s.ID == domain.ID {
			edited = s
		}
	}

	if edited.Name != "example.com" || edited.Currency != "$" || edited.Period != "month" || edited.NextPayment != inDays(2) || !edited.DueSoon || edited.PaymentMethod != "Mastercard" {
		t.Fatalf("edit not applied: %+v", edited)
	}

	// Paying early moves the next payment a month on.
	paid, err := api.MarkSubscriptionPaid(music.ID)
	// (A month on, clamped at month ends: 28 to 31 days later.)
	if err != nil || paid.DueInDays < 3+28 || paid.DueInDays > 3+31 || paid.DueSoon {
		t.Fatalf("mark paid: %+v %v", paid, err)
	}

	if _, err := api.MarkSubscriptionPaid(view.Subscriptions[len(view.Subscriptions)-1].ID); err == nil {
		t.Fatal("expected an error for a subscription without a schedule")
	}

	// The overview lists what's due soon: the domain (2 days), not the paid
	// music, the far rent or the inactive one.
	overview, err := api.Overview()
	if err != nil || len(overview.Upcoming) != 1 || overview.Upcoming[0].Name != "example.com" {
		t.Fatalf("unexpected upcoming %+v %v", overview.Upcoming, err)
	}

	if err := api.UpdateSubscription(SubscriptionInput{ID: domain.ID, Name: "x", Type: "x", Currency: "$", Amount: "1", Period: "month", PaymentMethod: " "}); err == nil || err.Error() != "payment method is required" {
		t.Fatalf("expected payment method error, got %v", err)
	}

	if err := api.DeleteSubscription(domain.ID); err != nil {
		t.Fatal(err)
	}

	if err := api.DeleteSubscription(domain.ID); err != errSubscriptionNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestCreateSubscriptionValidates(t *testing.T) {
	api := newTestAPI(t)
	ok := SubscriptionInput{Name: "x", Type: "Other", Currency: "$", Amount: "1", Period: "month", PaymentMethod: "Other"}

	cases := map[string]func(*SubscriptionInput){
		"subscription name is required":         func(i *SubscriptionInput) { i.Name = "" },
		"currency is required":                  func(i *SubscriptionInput) { i.Currency = "" },
		"unknown currency":                      func(i *SubscriptionInput) { i.Currency = "BTC" },
		"period must be one of":                 func(i *SubscriptionInput) { i.Period = "daily" },
		"next payment date must use YYYY-MM-DD": func(i *SubscriptionInput) { i.NextPayment = "24.12.2026" },
		"type is required":                      func(i *SubscriptionInput) { i.Type = "" },
	}

	for want, change := range cases {
		input := ok
		change(&input)
		if err := api.CreateSubscription(input); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("expected error containing %q, got %v", want, err)
		}
	}
}

// A subscription the TUI made (yearly date / monthly day) still gets a
// next payment.
func TestTUISubscriptionsGetANextPayment(t *testing.T) {
	api := newTestAPI(t)
	day := 18
	yearly := time.Now().AddDate(0, 0, 20)
	storage, _ := api.currentStorage()

	if err := storage.CreateSubscription(subscriptionTUI{"Yearly", "year", yearly.Format("02.01.2006"), nil}.build()); err != nil {
		t.Fatal(err)
	}

	if err := storage.CreateSubscription(subscriptionTUI{"Monthly", "month", "", &day}.build()); err != nil {
		t.Fatal(err)
	}

	view, _ := api.Subscriptions("active", "subscription")
	for _, s := range view.Subscriptions {
		if s.NextPayment == "" {
			t.Fatalf("expected a next payment for %+v", s)
		}

		if s.Name == "Yearly" && (s.NextPayment != yearly.Format(logDateLayout) || !s.DueSoon) {
			t.Fatalf("unexpected yearly %+v", s)
		}

		if s.Name == "Monthly" && !strings.HasSuffix(s.NextPayment, "-18") {
			t.Fatalf("unexpected monthly %+v", s)
		}
	}
}

type subscriptionTUI struct {
	name, period, yearly string
	day                  *int
}

func (s subscriptionTUI) build() *subscription.Subscription {
	return &subscription.Subscription{Name: s.name, Currency: "$", AmountCents: 100, Period: s.period, PaymentMethod: "Other", Type: "Other", IsActive: true, PaymentDateYearly: s.yearly, PaymentDayMonthly: s.day}
}
