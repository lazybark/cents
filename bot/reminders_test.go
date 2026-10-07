package bot

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/lazybark/cents/service"
	"github.com/lazybark/cents/telegram"
)

func TestStage(t *testing.T) {
	today := time.Date(2026, 10, 8, 10, 0, 0, 0, time.Local)
	for due, want := range map[string]string{
		"2026-10-12": "",
		"2026-10-10": "soon",
		"2026-10-09": "soon",
		"2026-10-08": "today",
		"2026-10-07": "overdue-0",
		"2026-10-01": "overdue-0",
		"2026-09-30": "overdue-1",
		"2026-09-23": "overdue-2",
		"":           "",
		"8.10.2026":  "",
	} {
		if got, _ := stage(due, today, 2); got != want {
			t.Errorf("%q: want %q, got %q", due, want, got)
		}
	}

	// Across a change of clocks, days are still days.
	if _, days := stage("2026-10-26", time.Date(2026, 10, 24, 23, 30, 0, 0, time.Local), 5); days != 2 {
		t.Errorf("expected 2 days, got %d", days)
	}
}

func TestReminderConfig(t *testing.T) {
	for want, r := range map[string]Reminders{
		"days_ahead":          {Enabled: true, DaysAhead: -1},
		"reminders.from: use": {Enabled: true, From: "9am"},
		"from must be before": {Enabled: true, From: "21:00", Until: "09:00"},
		`"bills" isn't one`:   {Enabled: true, Kinds: []string{"bills"}},
	} {
		if err := r.check(); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("expected %q, got %v", want, err)
		}
	}

	if err := (Reminders{Enabled: true, Kinds: []string{"payments", "budgets"}}).check(); err != nil {
		t.Fatal(err)
	}

	if err := (Reminders{DaysAhead: -5}).check(); err != nil {
		t.Fatal("reminders that are off aren't checked")
	}
}

// remindersBot is a bot with reminders on and something of each kind due.
func remindersBot(t *testing.T, state string) (*Bot, *fakeTelegram, *service.Service, time.Time) {
	t.Helper()

	bt, tg, svc := newTestBot(t)
	now := time.Now()
	morning := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, time.Local)
	bt.now = func() time.Time { return morning }
	day := func(offset int) string { return morning.AddDate(0, 0, offset).Format("2006-01-02") }

	// created drops what a create returns but its error.
	created := func(_ service.CreatedIn, err error) error { return err }
	steps := []error{
		svc.CreateSubscription(service.SubscriptionInput{Name: "Rent", Type: "Rent", Currency: "$", Amount: "900", Period: "month", PaymentMethod: "Bank", NextPayment: day(1), IsActive: true, IsObligation: true, PaidManually: true}),
		svc.CreateSubscription(service.SubscriptionInput{Name: "Music", Type: "Multimedia", Currency: "$", Amount: "10", Period: "month", PaymentMethod: "Card", NextPayment: day(0), IsActive: true}),
		svc.CreateSubscription(service.SubscriptionInput{Name: "Far off", Type: "Software", Currency: "$", Amount: "5", Period: "year", PaymentMethod: "Card", NextPayment: day(20), IsActive: true}),
		svc.SaveTaxType(service.TaxTypeInput{Country: "NL", Name: "VAT"}),
		created(svc.CreateDebt(service.NewDebtInput{Peer: "Ann", Currency: "$", Amount: "50", AmountPaid: "0", CreatedAt: day(-30), DueDate: day(1)})),
		created(svc.CreateDebt(service.NewDebtInput{IsOwedToUser: true, Peer: "Bob", Currency: "$", Amount: "20", AmountPaid: "0", CreatedAt: day(-30), DueDate: day(-3)})),
		created(svc.CreateDebt(service.NewDebtInput{IsOwedToUser: true, Peer: "Cid", Currency: "$", Amount: "20", AmountPaid: "0", CreatedAt: day(-30), DueDate: day(1)})),
		svc.SaveBudget(service.BudgetInput{Category: "Food", Limit: "10"}),
	}
	for _, err := range steps {
		if err != nil {
			t.Fatal(err)
		}
	}

	view, _ := svc.Settings()
	for _, step := range []func() error{
		func() error {
			_, err := svc.CreateTax(service.NewTaxInput{TaxTypeID: view.TaxTypes[0].ID, Currency: "$", AmountDue: "300", AmountPaid: "100", Period: "Q3", DueDate: day(0)})
			return err
		},
		func() error {
			_, err := svc.CreateInvoice(service.InvoiceInput{Title: "Hosting", IsIncoming: true, Currency: "$", Amount: "30", Peer: "Host & Co", InvoiceDate: day(-10), DueDate: day(1)})
			return err
		},
		func() error {
			_, err := svc.CreateInvoice(service.InvoiceInput{Title: "Design", Currency: "$", Amount: "500", Peer: "Acme", InvoiceDate: day(-20), DueDate: day(-3)})
			return err
		},
		func() error {
			_, err := svc.CreateCashflow(service.NewCashflowInput{Currency: "$", Amount: "12", Date: day(0), Category: "Food"})
			return err
		},
	} {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}

	if err := bt.EnableReminders(Reminders{Enabled: true, DaysAhead: 1, StateFile: state}); err != nil {
		t.Fatal(err)
	}

	return bt, tg, svc, morning
}

func texts(messages []sent) []string {
	var out []string
	for _, m := range messages {
		out = append(out, m.text)
	}

	sort.Strings(out)
	return out
}

func TestReminders(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state", "bot-state.json")
	bt, tg, svc, morning := remindersBot(t, state)
	ctx := context.Background()

	bt.sendReminders(ctx)
	got := strings.Join(texts(tg.sent), "\n")
	for _, want := range []string{
		"🔔 <b>Rent</b>: $ 900.00, due tomorrow",
		"🔔 <b>Music</b>: $ 10.00, due today (charged automatically)",
		"🔔 Tax <b>VAT Q3</b>: $ 200.00 left to pay, due today",
		"🔔 Invoice <b>Hosting</b> to Host &amp; Co: $ 30.00, due tomorrow",
		"🔔 Invoice <b>Design</b>: Acme hasn't paid $ 500.00, 3 days overdue",
		"🔔 You owe Ann $ 50.00, due tomorrow",
		"🔔 Bob owes you $ 20.00, 3 days overdue",
		"🔴 Food: $ 12.00 of $ 10.00 (120%): over the budget",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in:\n%s", want, got)
		}
	}

	// Not yet due, owed to me but not late: nothing.
	if len(tg.sent) != 8 || strings.Contains(got, "Far off") || strings.Contains(got, "Cid") {
		t.Fatalf("expected 8 reminders, got %d:\n%s", len(tg.sent), got)
	}

	if info, err := os.Stat(state); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("expected the state kept privately: %v", err)
	}

	// Checking again sends nothing new; neither does a bot started again on
	// the same state.
	bt.sendReminders(ctx)
	again, againTG, _, _ := remindersBot(t, state)
	again.b = svc
	again.sendReminders(ctx)
	if len(tg.sent) != 8 || len(againTG.sent) != 0 {
		t.Fatalf("expected no reminder twice: %d, %d", len(tg.sent), len(againTG.sent))
	}

	// The next day: what was due tomorrow is due today. Budgets and the
	// overdue ones were said already.
	bt.now = func() time.Time { return morning.AddDate(0, 0, 1) }
	bt.sendReminders(ctx)
	next := texts(tg.sent[8:])
	if len(next) != 4 || !strings.Contains(strings.Join(next, "\n"), "<b>Rent</b>: $ 900.00, due today") {
		t.Fatalf("expected the day's reminders: %q", next)
	}

	// A week after the first overdue day, the overdue ones come again.
	bt.now = func() time.Time { return morning.AddDate(0, 0, 5) }
	before := len(tg.sent)
	bt.sendReminders(ctx)
	late := strings.Join(texts(tg.sent[before:]), "\n")
	if !strings.Contains(late, "Bob owes you $ 20.00, 8 days overdue") || !strings.Contains(late, "Acme hasn't paid") || !strings.Contains(late, "<b>Rent</b>") {
		t.Fatalf("expected the overdue ones again: %s", late)
	}

	// Never at night.
	bt.now = func() time.Time { return morning.AddDate(0, 0, 12).Add(13 * time.Hour) }
	before = len(tg.sent)
	bt.sendReminders(ctx)
	if len(tg.sent) != before {
		t.Fatalf("expected nothing at 23:00: %q", texts(tg.sent[before:]))
	}
}

func TestMarkingPaidFromAReminder(t *testing.T) {
	bt, tg, svc, morning := remindersBot(t, filepath.Join(t.TempDir(), "state.json"))
	bt.reminders.Kinds = []string{"payments"}
	bt.sendReminders(context.Background())

	var rent sent
	for _, m := range tg.sent {
		if strings.Contains(m.text, "Rent") {
			rent = m
		}
	}

	if len(tg.sent) != 2 || rent.keyboard == nil || rent.keyboard[0][0].Text != "Mark paid" {
		t.Fatalf("expected only payments, Rent with a button: %q", texts(tg.sent))
	}

	press(bt, owner, rent, rent.keyboard[0][0].Data)
	tomorrow := morning.AddDate(0, 0, 1)
	if len(tg.edits) != 1 || !strings.Contains(tg.edits[0].text, "✅ <b>Rent</b> paid for "+tomorrow.Format("02 Jan")+"; next one "+tomorrow.AddDate(0, 1, 0).Format("02 Jan")) {
		t.Fatalf("expected it marked paid: %+v", tg.edits)
	}

	payments, _ := svc.SubscriptionPayments(1)
	if len(payments) != 1 {
		t.Fatalf("expected one payment: %+v", payments)
	}

	press(bt, owner, rent, rent.keyboard[0][0].Data)
	if tg.answers[len(tg.answers)-1] != "Couldn't mark it paid: that payment is marked paid already" {
		t.Fatalf("expected a second press refused: %v", tg.answers)
	}

	// /due lists a week ahead, whatever the reminders are set to.
	say(bt, owner, "/due")
	due := tg.last().text
	if !strings.HasPrefix(due, "<b>Due soon</b>") || !strings.Contains(due, "Music") || !strings.Contains(due, "Bob owes you") || strings.Contains(due, "Far off") {
		t.Fatalf("unexpected /due: %s", due)
	}
}

func TestRemindersRunWithTheBot(t *testing.T) {
	bt, tg, _, _ := remindersBot(t, filepath.Join(t.TempDir(), "state.json"))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- bt.Run(ctx, waitingPoller{}) }()

	// The first check runs as the bot starts.
	deadline := time.Now().Add(5 * time.Second)
	for tg.count() < 8 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	if err := <-done; err != nil || tg.count() != 8 {
		t.Fatalf("expected the reminders sent as it starts: %d %v", tg.count(), err)
	}
}

// waitingPoller has no updates: it waits until ctx ends.
type waitingPoller struct{}

func (waitingPoller) Updates(ctx context.Context, _ int64) ([]telegram.Update, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
