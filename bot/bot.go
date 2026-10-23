package bot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lazybark/cents/service"
	"github.com/lazybark/cents/telegram"
)

// Messenger sends messages to Telegram; telegram.Client is one.
type Messenger interface {
	Send(ctx context.Context, chat int64, text string, keyboard telegram.Keyboard) (int64, error)
	Edit(ctx context.Context, chat, message int64, text string, keyboard telegram.Keyboard) error
	Answer(ctx context.Context, callback, text string) error
}

// Poller waits for updates; telegram.Client is one.
type Poller interface {
	Updates(ctx context.Context, offset int64) ([]telegram.Update, error)
}

// Bot answers the allowed users' messages from b.
type Bot struct {
	b       service.Backend
	tg      Messenger
	allowed map[int64]bool
	log     *slog.Logger
	// now is the time, which tests set; days are the bot's local ones.
	now func() time.Time

	mu sync.Mutex
	// pending are entries waiting for their category, by key.
	pending map[string]pendingEntry

	// reminders, when set, are sent as they say; sentStore keeps which
	// were sent. Both are set before Run.
	reminders *Reminders
	sentStore *sentStore
}

// pendingEntry is an entry waiting for a category button.
type pendingEntry struct {
	entry   entry
	expires time.Time
}

// pendingFor is how long the category buttons work.
const pendingFor = 30 * time.Minute

// New is a bot on b, answering the users with the allowed Telegram IDs.
func New(b service.Backend, tg Messenger, allowed []int64, log *slog.Logger) *Bot {
	bt := &Bot{b: b, tg: tg, allowed: map[int64]bool{}, log: log, now: time.Now, pending: map[string]pendingEntry{}}
	for _, id := range allowed {
		bt.allowed[id] = true
	}

	return bt
}

// Commands are the bot's commands, for Telegram's menu.
var Commands = []telegram.Command{
	{Command: "month", Description: "This month's incomes and expenses"},
	{Command: "last", Description: "The latest entries"},
	{Command: "budgets", Description: "Budgets this month"},
	{Command: "accounts", Description: "Accounts and their balances"},
	{Command: "overview", Description: "Net worth, debts and payments coming up"},
	{Command: "due", Description: "What's due in the next 7 days, or overdue"},
	{Command: "help", Description: "How to add an income or expense"},
}

// Run answers updates from poller until ctx ends. Failures to get updates
// are retried, slower each time, except a refused token.
func (bt *Bot) Run(ctx context.Context, poller Poller) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if bt.reminders != nil && bt.reminders.Enabled {
		reminded := make(chan struct{})
		go func() { defer close(reminded); bt.remind(ctx) }()
		defer func() { cancel(); <-reminded }()
	}

	var offset int64
	wait := time.Second

	for {
		updates, err := poller.Updates(ctx, offset)
		if ctx.Err() != nil {
			return nil
		}

		if err != nil {
			var refused *telegram.Error
			if errors.As(err, &refused) && (refused.Code == 401 || refused.Code == 404) {
				return fmt.Errorf("telegram refused the bot's token: %w", err)
			}

			pause := wait
			if refused != nil && refused.RetryAfter > 0 {
				pause = refused.RetryAfter
			}

			bt.log.Warn("getting updates failed", "error", err, "retry_in", pause.String())
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(pause):
			}

			wait = min(wait*2, time.Minute)
			continue
		}

		wait = time.Second
		for _, u := range updates {
			offset = u.UpdateID + 1
			bt.Handle(ctx, u)
		}
	}
}

// Handle answers one update.
func (bt *Bot) Handle(ctx context.Context, u telegram.Update) {
	switch {
	case u.Message != nil && u.Message.From != nil:
		bt.message(ctx, u.Message)
	case u.CallbackQuery != nil:
		bt.button(ctx, u.CallbackQuery)
	}
}

func (bt *Bot) message(ctx context.Context, m *telegram.Message) {
	if !bt.allowed[m.From.ID] {
		bt.log.Warn("message from a user not allowed", "user", m.From.ID, "username", m.From.Username)
		bt.send(ctx, m.Chat.ID, fmt.Sprintf("This bot is private. Your Telegram user ID is <code>%d</code>: its owner can allow it in the bot's config.", m.From.ID), nil)
		return
	}

	text := strings.TrimSpace(m.Text)
	if text == "" {
		return
	}

	if strings.HasPrefix(text, "/") {
		bt.command(ctx, m.Chat.ID, text)
		return
	}

	bt.add(ctx, m.Chat.ID, text)
}

// command answers "/name args", also as "/name@bot args".
func (bt *Bot) command(ctx context.Context, chat int64, text string) {
	fields := strings.Fields(text)
	name, _, _ := strings.Cut(strings.TrimPrefix(fields[0], "/"), "@")
	args := fields[1:]

	var reply string
	var err error
	switch strings.ToLower(name) {
	case "start", "help":
		reply = helpText
	case "month":
		month := ""
		if len(args) > 0 {
			month = args[0]
		}
		reply, err = bt.month(month)
	case "last":
		n := 10
		if len(args) > 0 {
			if parsed, perr := strconv.Atoi(args[0]); perr == nil && parsed > 0 {
				n = min(parsed, 50)
			}
		}
		reply, err = bt.last(n)
	case "budgets":
		reply, err = bt.budgets()
	case "accounts":
		reply, err = bt.accounts()
	case "overview":
		reply, err = bt.overview()
	case "due":
		reply, err = bt.dueNow()
	default:
		reply = "I don't know that command. /help says what I can do."
	}

	if err != nil {
		reply = failed(err)
	}

	bt.send(ctx, chat, reply, nil)
}

// add adds the entry text describes, or asks for its category.
func (bt *Bot) add(ctx context.Context, chat int64, text string) {
	c, err := bt.choices()
	if err != nil {
		bt.send(ctx, chat, failed(err), nil)
		return
	}

	e, err := parseEntry(text, c, bt.now())
	if errors.Is(err, errNotEntry) {
		bt.send(ctx, chat, "Start with the amount, like <code>12.50 food lunch</code>. /help has more.", nil)
		return
	}

	if err != nil {
		bt.send(ctx, chat, failed(err), nil)
		return
	}

	if e.Category == "" {
		bt.askCategory(ctx, chat, e, c.categories(e.IsIncome))
		return
	}

	reply, keyboard := bt.save(e)
	bt.send(ctx, chat, reply, keyboard)
}

// save adds e, returning the reply and its Undo button.
func (bt *Bot) save(e entry) (string, telegram.Keyboard) {
	result, err := bt.b.CreateCashflow(serviceInput(e))
	if err != nil {
		return failed(err), nil
	}

	reply := "✅ " + describe(e, bt.now())
	if note := bt.budgetNote(e, result.Month); note != "" {
		reply += "\n" + note
	}

	return reply, telegram.Keyboard{{{Text: "Undo", Data: fmt.Sprintf("u:%d", result.ID)}}}
}

// askCategory keeps e and asks for its category with a button each.
func (bt *Bot) askCategory(ctx context.Context, chat int64, e entry, names []string) {
	if len(names) == 0 {
		bt.send(ctx, chat, "There are no categories yet: add some in the app first.", nil)
		return
	}

	key := bt.keep(e)
	var keyboard telegram.Keyboard
	for i, name := range names {
		if i%3 == 0 {
			keyboard = append(keyboard, nil)
		}

		row := &keyboard[len(keyboard)-1]
		*row = append(*row, telegram.Button{Text: name, Data: fmt.Sprintf("c:%s:%d", key, i)})
	}

	keyboard = append(keyboard, []telegram.Button{{Text: "Cancel", Data: "x:" + key}})
	bt.send(ctx, chat, "Which category? "+describe(e, bt.now()), keyboard)
}

// keep stores e for the category buttons, returning its key.
func (bt *Bot) keep(e entry) string {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	now := bt.now()
	for key, p := range bt.pending {
		if now.After(p.expires) {
			delete(bt.pending, key)
		}
	}

	raw := make([]byte, 4)
	_, _ = rand.Read(raw)
	key := hex.EncodeToString(raw)
	bt.pending[key] = pendingEntry{entry: e, expires: now.Add(pendingFor)}

	return key
}

// take removes and returns the entry kept under key.
func (bt *Bot) take(key string) (entry, bool) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	p, ok := bt.pending[key]
	delete(bt.pending, key)

	return p.entry, ok && bt.now().Before(p.expires)
}

// button answers a press of one of the bot's buttons.
func (bt *Bot) button(ctx context.Context, q *telegram.CallbackQuery) {
	if !bt.allowed[q.From.ID] || q.Message == nil {
		bt.answer(ctx, q.ID, "This bot is private.")
		return
	}

	chat, message := q.Message.Chat.ID, q.Message.MessageID
	kind, arg, _ := strings.Cut(q.Data, ":")
	switch kind {
	case "u":
		id, err := strconv.ParseUint(arg, 10, 32)
		if err == nil {
			err = bt.b.DeleteCashflow(uint(id))
		}

		if err != nil {
			bt.answer(ctx, q.ID, "Couldn't undo it: "+err.Error())
			return
		}

		bt.edit(ctx, chat, message, "<s>"+telegram.Escape(firstLine(q.Message.Text))+"</s>\nUndone: it's deleted.", nil)
		bt.answer(ctx, q.ID, "Deleted")
	case "c":
		key, index, _ := strings.Cut(arg, ":")
		e, ok := bt.take(key)
		if !ok {
			bt.edit(ctx, chat, message, "That one has expired: send it again.", nil)
			bt.answer(ctx, q.ID, "")
			return
		}

		c, err := bt.choices()
		names := c.categories(e.IsIncome)
		i, ierr := strconv.Atoi(index)
		if err != nil || ierr != nil || i < 0 || i >= len(names) {
			bt.edit(ctx, chat, message, "The categories have changed meanwhile: send it again.", nil)
			bt.answer(ctx, q.ID, "")
			return
		}

		e.Category = names[i]
		reply, keyboard := bt.save(e)
		bt.edit(ctx, chat, message, reply, keyboard)
		bt.answer(ctx, q.ID, "")
	case "p":
		id, nextPayment, _ := strings.Cut(arg, ":")
		parsed, err := strconv.ParseUint(id, 10, 32)
		reply := ""
		if err == nil {
			reply, err = bt.markPaid(uint(parsed), nextPayment)
		}

		if err != nil {
			bt.answer(ctx, q.ID, "Couldn't mark it paid: "+err.Error())
			return
		}

		bt.edit(ctx, chat, message, reply, nil)
		bt.answer(ctx, q.ID, "Marked paid")
	case "x":
		bt.take(arg)
		bt.edit(ctx, chat, message, "Cancelled.", nil)
		bt.answer(ctx, q.ID, "")
	default:
		bt.answer(ctx, q.ID, "")
	}
}

// choices loads what entries can name.
func (bt *Bot) choices() (choices, error) {
	month, err := bt.b.CashflowMonth("")
	if err != nil {
		return choices{}, err
	}

	stts, err := bt.b.Settings()
	if err != nil {
		return choices{}, err
	}

	c := choices{Base: stts.BaseCurrency, Currencies: map[string]string{}, Income: month.Options.IncomeCategories, Expense: month.Options.ExpenseCategories, Accounts: month.Options.Accounts}
	add := func(name, code string) {
		for _, alias := range []string{name, code} {
			if alias != "" {
				c.Currencies[strings.ToLower(alias)] = name
			}
		}
	}

	add(stts.BaseCurrency, stts.BaseCode)
	for _, currency := range stts.Currencies {
		add(currency.Name, currency.Code)
	}

	return c, nil
}

func serviceInput(e entry) service.NewCashflowInput {
	return service.NewCashflowInput{IsIncome: e.IsIncome, Currency: e.Currency, Amount: e.Amount, Date: e.Date, Category: e.Category, Account: e.Account, Comment: e.Comment, Tags: e.Tags}
}

func (bt *Bot) send(ctx context.Context, chat int64, text string, keyboard telegram.Keyboard) {
	if _, err := bt.tg.Send(ctx, chat, text, keyboard); err != nil {
		bt.log.Error("sending a message failed", "chat", chat, "error", err)
	}
}

func (bt *Bot) edit(ctx context.Context, chat, message int64, text string, keyboard telegram.Keyboard) {
	if err := bt.tg.Edit(ctx, chat, message, text, keyboard); err != nil {
		bt.log.Error("editing a message failed", "chat", chat, "error", err)
	}
}

func (bt *Bot) answer(ctx context.Context, callback, text string) {
	if err := bt.tg.Answer(ctx, callback, text); err != nil {
		bt.log.Warn("answering a button failed", "error", err)
	}
}

func failed(err error) string {
	return "⚠️ " + telegram.Escape(err.Error())
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}
