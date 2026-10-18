package bot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lazybark/cents/flows/budget"
	"github.com/lazybark/cents/telegram"
)

// Kinds of reminder.
const (
	remindPayments = "payments"
	remindTaxes    = "taxes"
	remindInvoices = "invoices"
	remindDebts    = "debts"
	remindBudgets  = "budgets"
)

var reminderKinds = []string{remindPayments, remindTaxes, remindInvoices, remindDebts, remindBudgets}

// Reminders is the config of the reminders the bot sends by itself.
type Reminders struct {
	Enabled bool `yaml:"enabled"`
	// DaysAhead is how many days before a due date the first reminder
	// comes (another comes on the day).
	DaysAhead int `yaml:"days_ahead"`
	// From and Until are when reminders may be sent (HH:MM, the bot's local
	// time), so none comes at night; one due meanwhile waits for From.
	From  string `yaml:"from"`
	Until string `yaml:"until"`
	// Kinds are the reminders to send (payments, taxes, invoices, debts,
	// budgets); empty is all of them.
	Kinds []string `yaml:"kinds"`
	// StateFile is where the bot keeps which reminders it sent, so none
	// goes twice; empty is bot-state.json next to the config.
	StateFile string `yaml:"state_file"`
}

// checkEvery is how often the bot looks for reminders to send.
const checkEvery = 15 * time.Minute

// overdueEvery is how often an overdue reminder comes again.
const overdueEvery = 7

// keepSent is how long a sent reminder is remembered: long after its date
// has passed, so it can't come again.
const keepSent = 90 * 24 * time.Hour

func (r Reminders) check() error {
	if !r.Enabled {
		return nil
	}

	if r.DaysAhead < 0 || r.DaysAhead > 30 {
		return errors.New("reminders.days_ahead: from 0 to 30")
	}

	from, until, err := r.window()
	if err != nil {
		return err
	}

	if from >= until {
		return errors.New("reminders: from must be before until")
	}

	for _, kind := range r.Kinds {
		if !contains(reminderKinds, kind) {
			return fmt.Errorf("reminders.kinds: %q isn't one of %s", kind, strings.Join(reminderKinds, ", "))
		}
	}

	return nil
}

// window is From and Until as minutes into the day.
func (r Reminders) window() (int, int, error) {
	minutes := func(value, fallback, name string) (int, error) {
		if strings.TrimSpace(value) == "" {
			value = fallback
		}

		t, err := time.Parse("15:04", strings.TrimSpace(value))
		if err != nil {
			return 0, fmt.Errorf("reminders.%s: use HH:MM, like %s", name, fallback)
		}

		return t.Hour()*60 + t.Minute(), nil
	}

	from, err := minutes(r.From, "09:00", "from")
	if err != nil {
		return 0, 0, err
	}

	until, err := minutes(r.Until, "21:00", "until")
	return from, until, err
}

func (r Reminders) wants(kind string) bool {
	return len(r.Kinds) == 0 || contains(r.Kinds, kind)
}

// DefaultStateFile is the state file next to the config at configPath.
func DefaultStateFile(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "bot-state.json")
}

// reminder is something to tell once: key says what, for which date and
// at which stage, so the same one is known when it's due again.
type reminder struct {
	key      string
	text     string
	keyboard telegram.Keyboard
}

// sentStore remembers the reminders sent, by chat and key, in a file.
type sentStore struct {
	mu   sync.Mutex
	path string
	sent map[string]time.Time
}

func openSentStore(path string) (*sentStore, error) {
	s := &sentStore{path: path, sent: map[string]time.Time{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}

	if err != nil {
		return nil, fmt.Errorf("reminders: reading %s: %w", path, err)
	}

	var state struct {
		Sent map[string]time.Time `json:"sent"`
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("reminders: %s isn't valid (delete it to start over): %w", path, err)
	}

	if state.Sent != nil {
		s.sent = state.Sent
	}

	return s, nil
}

func sentKey(chat int64, key string) string {
	return strconv.FormatInt(chat, 10) + "|" + key
}

func (s *sentStore) has(chat int64, key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.sent[sentKey(chat, key)]
	return ok
}

// mark remembers a sent reminder, forgetting those long past, and saves.
func (s *sentStore) mark(chat int64, key string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sent[sentKey(chat, key)] = now
	for k, at := range s.sent {
		if now.Sub(at) > keepSent {
			delete(s.sent, k)
		}
	}

	data, err := json.MarshalIndent(struct {
		Sent map[string]time.Time `json:"sent"`
	}{s.sent}, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}

	// Written next to it first, so a crash leaves the old file whole.
	temp := s.path + ".partial"
	if err := os.WriteFile(temp, data, 0o600); err != nil {
		return err
	}

	return os.Rename(temp, s.path)
}

// EnableReminders makes Run send reminders as r says, remembering them in
// r.StateFile.
func (bt *Bot) EnableReminders(r Reminders) error {
	if err := r.check(); err != nil {
		return err
	}

	store, err := openSentStore(r.StateFile)
	if err != nil {
		return err
	}

	bt.reminders, bt.sentStore = &r, store
	return nil
}

// remind checks for reminders now and every checkEvery until ctx ends.
func (bt *Bot) remind(ctx context.Context) {
	ticker := time.NewTicker(checkEvery)
	defer ticker.Stop()

	for {
		bt.sendReminders(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// sendReminders sends every reminder due that a chat hasn't had yet, if
// it's within the hours they're sent.
func (bt *Bot) sendReminders(ctx context.Context) {
	now := bt.now()
	from, until, _ := bt.reminders.window()
	if minute := now.Hour()*60 + now.Minute(); minute < from || minute >= until {
		return
	}

	due, err := bt.due(now, *bt.reminders)
	if err != nil {
		bt.log.Warn("checking reminders failed", "error", err)
		return
	}

	for _, r := range due {
		for chat := range bt.allowed {
			if bt.sentStore.has(chat, r.key) {
				continue
			}

			if _, err := bt.tg.Send(ctx, chat, r.text, r.keyboard); err != nil {
				// Tried again at the next check.
				bt.log.Warn("sending a reminder failed", "chat", chat, "reminder", r.key, "error", err)
				continue
			}

			if err := bt.sentStore.mark(chat, r.key, now); err != nil {
				bt.log.Error("remembering a reminder failed: it may come again", "reminder", r.key, "error", err)
			}
		}
	}
}

// stage is where a due date stands today: "soon" within days ahead, "today",
// "overdue-N" (N weeks since the first overdue day), or "" not yet.
func stage(dueDate string, today time.Time, ahead int) (string, int) {
	due, err := time.ParseInLocation("2006-01-02", dueDate, time.Local)
	if err != nil {
		return "", 0
	}

	day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local)
	days := int(math.Round(due.Sub(day).Hours() / 24))
	switch {
	case days > ahead:
		return "", days
	case days > 0:
		return "soon", days
	case days == 0:
		return "today", 0
	default:
		return fmt.Sprintf("overdue-%d", (-days-1)/overdueEvery), days
	}
}

// when says how far off a due date is: "tomorrow", "in 3 days", "today",
// "2 days overdue".
func when(days int) string {
	switch {
	case days == 0:
		return "today"
	case days == 1:
		return "tomorrow"
	case days > 1:
		return fmt.Sprintf("in %d days", days)
	case days == -1:
		return "1 day overdue"
	default:
		return fmt.Sprintf("%d days overdue", -days)
	}
}

// due lists the reminders r asks for that are due today, sent before or
// not.
func (bt *Bot) due(today time.Time, r Reminders) ([]reminder, error) {
	var due []reminder
	ahead := r.DaysAhead

	if r.wants(remindPayments) {
		view, err := bt.b.Subscriptions("all", "subscription")
		if err != nil {
			return nil, err
		}

		obligations, err := bt.b.Subscriptions("all", "obligation")
		if err != nil {
			return nil, err
		}

		for _, sub := range append(view.Subscriptions, obligations.Subscriptions...) {
			if !sub.IsActive || sub.NextPayment == "" {
				continue
			}

			st, days := stage(sub.NextPayment, today, ahead)
			// A card charged automatically is never overdue.
			if st == "" || strings.HasPrefix(st, "overdue") && !sub.PaidManually {
				continue
			}

			text := fmt.Sprintf("🔔 <b>%s</b>: %s, %s", telegram.Escape(sub.Name), formatMoney(sub.Currency, sub.AmountCents), dueWhen(days))
			var keyboard telegram.Keyboard
			if sub.PaidManually {
				keyboard = telegram.Keyboard{{{Text: "Mark paid", Data: fmt.Sprintf("p:%d:%s", sub.ID, sub.NextPayment)}}}
			} else {
				text += " (charged automatically)"
			}

			due = append(due, reminder{key: fmt.Sprintf("payment:%d:%s:%s", sub.ID, sub.NextPayment, st), text: text, keyboard: keyboard})
		}
	}

	if r.wants(remindTaxes) {
		view, err := bt.b.Taxes("unpaid")
		if err != nil {
			return nil, err
		}

		for _, tax := range view.Taxes {
			st, days := stage(tax.DueDate, today, ahead)
			if st == "" || tax.LeftCents <= 0 {
				continue
			}

			name := strings.TrimSpace(tax.TypeName + " " + tax.Period)
			text := fmt.Sprintf("🔔 Tax <b>%s</b>: %s left to pay, %s", telegram.Escape(name), formatMoney(tax.Currency, tax.LeftCents), dueWhen(days))
			due = append(due, reminder{key: fmt.Sprintf("tax:%d:%s:%s", tax.ID, tax.DueDate, st), text: text})
		}
	}

	if r.wants(remindInvoices) {
		for _, mode := range []string{"incoming", "outgoing"} {
			view, err := bt.b.Invoices(mode)
			if err != nil {
				return nil, err
			}

			for _, inv := range view.Invoices {
				st, days := stage(inv.DueDate, today, ahead)
				// Ones paid to me are only worth a word once late.
				if st == "" || inv.Paid || !inv.IsIncoming && !strings.HasPrefix(st, "overdue") {
					continue
				}

				var text string
				if inv.IsIncoming {
					text = fmt.Sprintf("🔔 Invoice <b>%s</b> to %s: %s, %s", telegram.Escape(inv.Title), telegram.Escape(inv.Peer), formatMoney(inv.Currency, inv.AmountCents), dueWhen(days))
				} else {
					text = fmt.Sprintf("🔔 Invoice <b>%s</b>: %s hasn't paid %s, %s", telegram.Escape(inv.Title), peerOr(inv.Peer, "the client"), formatMoney(inv.Currency, inv.AmountCents), when(days))
				}

				due = append(due, reminder{key: fmt.Sprintf("invoice:%d:%s:%s", inv.ID, inv.DueDate, st), text: text})
			}
		}
	}

	if r.wants(remindDebts) {
		for _, mode := range []string{"outgoing", "incoming"} {
			view, err := bt.b.Debts(mode)
			if err != nil {
				return nil, err
			}

			for _, d := range view.Debts {
				st, days := stage(d.DueDate, today, ahead)
				if st == "" || d.LeftCents <= 0 || d.IsOwedToUser && !strings.HasPrefix(st, "overdue") {
					continue
				}

				var text string
				if d.IsOwedToUser {
					text = fmt.Sprintf("🔔 %s owes you %s, %s", peerOr(d.Peer, "Someone"), formatMoney(d.Currency, d.LeftCents), when(days))
				} else {
					text = fmt.Sprintf("🔔 You owe %s %s, %s", telegram.Escape(d.Peer), formatMoney(d.Currency, d.LeftCents), dueWhen(days))
				}

				due = append(due, reminder{key: fmt.Sprintf("debt:%d:%s:%s", d.ID, d.DueDate, st), text: text})
			}
		}
	}

	if r.wants(remindBudgets) {
		view, err := bt.b.Budgets("")
		if err != nil {
			return nil, err
		}

		for _, row := range view.Rows {
			if row.State == budget.StateOK {
				continue
			}

			text := budgetLine(row.State, row.Name, row.SpentCents, row.LimitCents, row.Percent, view.BaseCurrency)
			if row.State == budget.StateOver {
				text += ": over the budget"
			} else {
				text += ": close to the budget"
			}

			due = append(due, reminder{key: fmt.Sprintf("budget:%s:%s:%s", row.Name, view.Month, row.State), text: text})
		}
	}

	return due, nil
}

// dueWhen says when something is due: "due tomorrow", "due today", "2
// days overdue".
func dueWhen(days int) string {
	if days < 0 {
		return when(days)
	}

	return "due " + when(days)
}

func peerOr(peer, fallback string) string {
	if strings.TrimSpace(peer) == "" {
		return fallback
	}

	return telegram.Escape(peer)
}

// markPaid marks a regular payment paid from its reminder's button, if
// the payment it was for is still the one due.
func (bt *Bot) markPaid(id uint, nextPayment string) (string, error) {
	for _, kind := range []string{"subscription", "obligation"} {
		view, err := bt.b.Subscriptions("all", kind)
		if err != nil {
			return "", err
		}

		for _, sub := range view.Subscriptions {
			if sub.ID != id {
				continue
			}

			if sub.NextPayment != nextPayment {
				return "", errors.New("that payment is marked paid already")
			}

			row, err := bt.b.MarkSubscriptionPaid(id)
			if err != nil {
				return "", err
			}

			reply := fmt.Sprintf("✅ <b>%s</b> paid for %s", telegram.Escape(row.Name), day(nextPayment))
			if row.NextPayment != "" {
				reply += "; next one " + day(row.NextPayment)
			}

			return reply, nil
		}
	}

	return "", errors.New("that payment isn't there anymore")
}

// dueNow lists what's due now (the /due command), reminded or not.
func (bt *Bot) dueNow() (string, error) {
	// What reminders would say, a week ahead, whatever they're set to.
	due, err := bt.due(bt.now(), Reminders{DaysAhead: 7})
	if err != nil {
		return "", err
	}

	if len(due) == 0 {
		return "Nothing due in the next 7 days, and nothing overdue.", nil
	}

	lines := []string{"<b>Due soon</b>"}
	for _, item := range due {
		lines = append(lines, strings.TrimPrefix(item.text, "🔔 "))
	}

	return strings.Join(lines, "\n"), nil
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}

	return false
}
