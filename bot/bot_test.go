package bot

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/service"
	storage "github.com/lazybark/cents/storage/sqlite"
	"github.com/lazybark/cents/telegram"
)

// sent is a message the bot sent or edited.
type sent struct {
	chat, message int64
	text          string
	keyboard      telegram.Keyboard
}

// fakeTelegram records what the bot sends.
type fakeTelegram struct {
	mu      sync.Mutex
	sent    []sent
	edits   []sent
	answers []string
}

func (f *fakeTelegram) Send(_ context.Context, chat int64, text string, keyboard telegram.Keyboard) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.sent = append(f.sent, sent{chat: chat, message: int64(len(f.sent) + 1), text: text, keyboard: keyboard})
	return int64(len(f.sent)), nil
}

func (f *fakeTelegram) Edit(_ context.Context, chat, message int64, text string, keyboard telegram.Keyboard) error {
	f.edits = append(f.edits, sent{chat: chat, message: message, text: text, keyboard: keyboard})
	return nil
}

func (f *fakeTelegram) Answer(_ context.Context, _ string, text string) error {
	f.answers = append(f.answers, text)
	return nil
}

func (f *fakeTelegram) last() sent { return f.sent[len(f.sent)-1] }

func (f *fakeTelegram) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.sent)
}

const owner, stranger = 1001, 666

func newTestBot(t *testing.T) (*Bot, *fakeTelegram, *service.Service) {
	t.Helper()

	db, _, err := storage.OpenDatabase(filepath.Join(t.TempDir(), "cents.db"))
	if err != nil {
		t.Fatal(err)
	}

	store := storage.NewSQLiteStorage(db)
	t.Cleanup(func() { _ = store.Close() })
	for _, row := range []any{
		&settings.SettingCurrency{CurrencyName: "EUR", RateToBase: 1.1},
		&settings.SettingIncomeCategory{CategoryName: "Salary"},
		&settings.SettingExpenseCategory{CategoryName: "Food"},
		&settings.SettingExpenseCategory{CategoryName: "Rent"},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	svc := service.On(store)
	if err := svc.CreateAccount(service.NewAccountInput{Name: "Bank", Description: "main", Currency: "$", Amount: "1000"}); err != nil {
		t.Fatal(err)
	}

	tg := &fakeTelegram{}
	bt := New(svc, tg, []int64{owner}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	return bt, tg, svc
}

func say(bt *Bot, from int64, text string) {
	bt.Handle(context.Background(), telegram.Update{Message: &telegram.Message{From: &telegram.User{ID: from}, Chat: telegram.Chat{ID: from}, Text: text}})
}

func press(bt *Bot, from int64, message sent, data string) {
	bt.Handle(context.Background(), telegram.Update{CallbackQuery: &telegram.CallbackQuery{
		ID: "q", From: telegram.User{ID: from}, Data: data,
		Message: &telegram.Message{MessageID: message.message, Chat: telegram.Chat{ID: message.chat}, Text: strings.NewReplacer("<b>", "", "</b>", "").Replace(message.text)},
	}})
}

func entries(t *testing.T, svc *service.Service) []service.CashflowRow {
	t.Helper()

	month, err := svc.CashflowMonth("")
	if err != nil {
		t.Fatal(err)
	}

	return month.Entries
}

func TestStrangersAreTurnedAway(t *testing.T) {
	bt, tg, svc := newTestBot(t)

	say(bt, stranger, "12 food")
	if reply := tg.last(); reply.chat != stranger || !strings.Contains(reply.text, "<code>666</code>") || !strings.Contains(reply.text, "private") {
		t.Fatalf("expected the stranger told their ID: %+v", reply)
	}

	say(bt, stranger, "/accounts")
	if strings.Contains(tg.last().text, "Bank") || len(entries(t, svc)) != 0 {
		t.Fatalf("a stranger gets nothing: %+v", tg.last())
	}

	press(bt, stranger, sent{chat: stranger, message: 1}, "u:1")
	if len(tg.answers) != 1 || tg.answers[0] != "This bot is private." || len(tg.edits) != 0 {
		t.Fatalf("a stranger's button does nothing: %v %v", tg.answers, tg.edits)
	}
}

func TestAddingAndUndoing(t *testing.T) {
	bt, tg, svc := newTestBot(t)

	say(bt, owner, "12.50 food lunch #work")
	added := tg.last()
	if !strings.Contains(added.text, "✅ <b>Expense</b> $ 12.50 · Food · lunch #work") || len(added.keyboard) != 1 || added.keyboard[0][0].Text != "Undo" {
		t.Fatalf("unexpected reply %+v", added)
	}

	rows := entries(t, svc)
	if len(rows) != 1 || rows[0].AmountCents != 1250 || rows[0].Category != "Food" || rows[0].Comment != "lunch" || rows[0].Tags[0] != "work" || rows[0].Date != time.Now().Format("2006-01-02") {
		t.Fatalf("unexpected entries %+v", rows)
	}

	press(bt, owner, added, added.keyboard[0][0].Data)
	if len(entries(t, svc)) != 0 || !strings.HasPrefix(tg.edits[0].text, "<s>✅ Expense $ 12.50") || tg.edits[0].keyboard != nil {
		t.Fatalf("expected it undone: %+v", tg.edits)
	}

	press(bt, owner, added, added.keyboard[0][0].Data)
	if tg.answers[len(tg.answers)-1] != "Couldn't undo it: entry not found" {
		t.Fatalf("expected a second undo refused: %v", tg.answers)
	}

	// An income in another currency, into an account.
	say(bt, owner, "+3000 eur salary @bank")
	if rows := entries(t, svc); len(rows) != 1 || !rows[0].IsIncome || rows[0].Currency != "EUR" || rows[0].Account != "Bank" {
		t.Fatalf("unexpected income %+v %+v", rows, tg.last())
	}

	for text, want := range map[string]string{
		"hello":      "Start with the amount",
		"12btc food": "unknown currency",
		"12 food @x": `no account starts with "x"`,
	} {
		say(bt, owner, text)
		if !strings.Contains(tg.last().text, want) {
			t.Errorf("%q: expected %q, got %q", text, want, tg.last().text)
		}
	}
}

func TestAskingForACategory(t *testing.T) {
	bt, tg, svc := newTestBot(t)

	say(bt, owner, "30 groceries")
	ask := tg.last()
	if !strings.HasPrefix(ask.text, "Which category?") || len(ask.keyboard) != 2 || ask.keyboard[0][0].Text != "Food" || ask.keyboard[1][0].Text != "Cancel" {
		t.Fatalf("expected the expense categories offered: %+v", ask)
	}

	press(bt, owner, ask, ask.keyboard[0][1].Data) // Rent
	if rows := entries(t, svc); len(rows) != 1 || rows[0].Category != "Rent" || rows[0].Comment != "groceries" {
		t.Fatalf("expected it saved as Rent: %+v", rows)
	}

	if edit := tg.edits[0]; !strings.HasPrefix(edit.text, "✅") || edit.keyboard[0][0].Text != "Undo" {
		t.Fatalf("expected the question turned into the entry: %+v", edit)
	}

	// The same button again: the entry was taken.
	press(bt, owner, ask, ask.keyboard[0][1].Data)
	if !strings.Contains(tg.edits[1].text, "expired") || len(entries(t, svc)) != 1 {
		t.Fatalf("expected one entry only: %+v", tg.edits)
	}

	say(bt, owner, "5 snack")
	cancel := tg.last()
	press(bt, owner, cancel, cancel.keyboard[len(cancel.keyboard)-1][0].Data)
	if tg.edits[2].text != "Cancelled." || len(entries(t, svc)) != 1 {
		t.Fatalf("expected it cancelled: %+v", tg.edits)
	}

	// The buttons expire.
	say(bt, owner, "5 snack")
	late := tg.last()
	bt.now = func() time.Time { return time.Now().Add(pendingFor + time.Minute) }
	press(bt, owner, late, late.keyboard[0][0].Data)
	if !strings.Contains(tg.edits[3].text, "expired") || len(entries(t, svc)) != 1 {
		t.Fatalf("expected it expired: %+v", tg.edits)
	}
}

func TestReports(t *testing.T) {
	bt, tg, svc := newTestBot(t)
	for _, text := range []string{"+3000 salary", "900 rent", "40 food", "12.5 eur food dinner"} {
		say(bt, owner, text)
	}

	if err := svc.SaveBudget(service.BudgetInput{Category: "Food", Limit: "50"}); err != nil {
		t.Fatal(err)
	}

	say(bt, owner, "5 food")
	if !strings.Contains(tg.last().text, "🔴 Food: $ 58.75 of $ 50.00 (118%)") {
		t.Fatalf("expected the budget over: %q", tg.last().text)
	}

	month := time.Now().Format("January 2006")
	for command, wants := range map[string][]string{
		"/month":           {"<b>" + month + "</b>", "Income: $ 3000.00", "Expenses: $ 958.75", "Net: $ 2041.25", "Rent: $ 900.00", "Food: $ 58.75"},
		"/month@cents_bot": {"<b>" + month + "</b>"},
		"/month 2020-01":   {"January 2020", "Nothing yet."},
		"/month september": {"⚠️ month must use YYYY-MM format"},
		"/last 2":          {"Latest entries", "$ 5.00 · Food", "EUR 12.50 · Food · dinner"},
		"/budgets":         {"Budgets, " + month, "🔴 Food"},
		"/accounts":        {"<b>Accounts</b>: $ 1000.00", "Bank: $ 1000.00"},
		"/overview":        {"<b>Net worth</b>: $ 1000.00", "This month so far: $ 2041.25"},
		"/help":            {"<code>12.50 food lunch</code>"},
		"/start":           {"Send an amount"},
		"/nope":            {"I don't know that command"},
	} {
		say(bt, owner, command)
		for _, want := range wants {
			if !strings.Contains(tg.last().text, want) {
				t.Errorf("%s: expected %q in %q", command, want, tg.last().text)
			}
		}
	}
}

// poller hands out updates once, then fails as told.
type poller struct {
	batches [][]telegram.Update
	err     error
	offsets []int64
	cancel  func()
}

func (p *poller) Updates(_ context.Context, offset int64) ([]telegram.Update, error) {
	p.offsets = append(p.offsets, offset)
	if len(p.batches) > 0 {
		batch := p.batches[0]
		p.batches = p.batches[1:]
		return batch, nil
	}

	if p.err != nil {
		return nil, p.err
	}

	p.cancel()
	return nil, context.Canceled
}

func TestRun(t *testing.T) {
	bt, tg, _ := newTestBot(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	update := func(id int64, text string) telegram.Update {
		return telegram.Update{UpdateID: id, Message: &telegram.Message{From: &telegram.User{ID: owner}, Chat: telegram.Chat{ID: owner}, Text: text}}
	}

	p := &poller{batches: [][]telegram.Update{{update(7, "/help"), update(8, "/accounts")}, {update(9, "/budgets")}}, cancel: cancel}
	if err := bt.Run(ctx, p); err != nil {
		t.Fatal(err)
	}

	// Each request confirms the updates before it.
	if len(tg.sent) != 3 || len(p.offsets) != 3 || p.offsets[0] != 0 || p.offsets[1] != 9 || p.offsets[2] != 10 {
		t.Fatalf("unexpected run: %d sent, offsets %v", len(tg.sent), p.offsets)
	}

	// A refused token stops it.
	refused := &poller{err: &telegram.Error{Code: 401, Description: "Unauthorized"}}
	if err := bt.Run(context.Background(), refused); err == nil || !errors.Is(err, refused.err) {
		t.Fatalf("expected the token refused: %v", err)
	}
}
