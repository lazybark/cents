package bot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/lazybark/cents/client"
	"github.com/lazybark/cents/service"
	storage "github.com/lazybark/cents/storage/sqlite"
	"github.com/lazybark/cents/telegram"
)

// Run starts the bot as cfg says and answers messages until ctx ends.
// Logs go to logOutput.
func Run(ctx context.Context, cfg Config, logOutput io.Writer) error {
	log, err := newLogger(cfg.Log, logOutput)
	if err != nil {
		return err
	}

	b, done, err := Connect(cfg)
	if err != nil {
		return err
	}
	defer done()

	tg := telegram.New(apiOf(cfg), cfg.Telegram.Token)
	me, err := tg.Me(ctx)
	if err != nil {
		return fmt.Errorf("telegram: checking the bot's token: %w", err)
	}

	if err := tg.SetCommands(ctx, Commands); err != nil {
		log.Warn("setting the command menu failed", "error", err)
	}

	bt := New(b, tg, cfg.Telegram.AllowedUsers, log)
	if cfg.Reminders.Enabled {
		if err := bt.EnableReminders(cfg.Reminders); err != nil {
			return err
		}
	}

	ctx, stop := context.WithCancel(ctx)
	defer stop()

	failed := make(chan error, 1)
	if cfg.WebApp.on() {
		go func() {
			if err := bt.serveWebApp(ctx, cfg.WebApp, b, cfg.Telegram.Token); err != nil {
				failed <- err
				stop()
			}
		}()

		// The menu button follows url, so a new tunnel address needs no
		// trip to @BotFather.
		setMenuButtons(ctx, tg, cfg, log)
	}

	log.Info("cents bot started", "bot", "@"+me.Username, "backend", backendName(cfg), "users", len(cfg.Telegram.AllowedUsers), "reminders", cfg.Reminders.Enabled, "web_app", cfg.WebApp.URL)
	err = bt.Run(ctx, tg)
	select {
	case serveErr := <-failed:
		err = serveErr
	default:
	}

	log.Info("cents bot stopped")

	return err
}

// setMenuButtons points the menu button at the Mini App: every chat's,
// and each allowed user's own (which would otherwise win over it, if one
// was ever set to the commands).
func setMenuButtons(ctx context.Context, tg *telegram.Client, cfg Config, log *slog.Logger) {
	chats := append([]int64{0}, cfg.Telegram.AllowedUsers...)
	set := 0
	for _, chat := range chats {
		if err := tg.SetMenuButton(ctx, chat, cfg.WebApp.button(), cfg.WebApp.URL); err != nil {
			log.Warn("setting the menu button failed", "chat", chat, "error", err)
			continue
		}

		set++
	}

	if set == len(chats) {
		log.Info("menu button set", "url", cfg.WebApp.URL)
	}
}

// Data is what the bot works on: every call, and the files the Mini App
// can download.
type Data interface {
	service.Backend
	service.Files
}

// Connect opens the backend cfg names: a client of its server, or the
// service on its database file; done closes it.
func Connect(cfg Config) (Data, func(), error) {
	if err := cfg.Check(); err != nil {
		return nil, nil, err
	}

	if cfg.Server != nil && strings.TrimSpace(cfg.Server.URL) != "" {
		options := []client.Option{client.WithHTTPClient(&http.Client{Timeout: time.Minute})}
		if cfg.Server.Certificate != "" {
			options = append(options, client.WithCertificate(cfg.Server.Certificate))
		}

		remote, err := client.New(cfg.Server.URL, cfg.Server.Token, options...)
		if err != nil {
			return nil, nil, fmt.Errorf("server: %w", err)
		}

		return remote, func() {}, nil
	}

	db, _, err := storage.OpenDatabase(cfg.Database)
	if errors.Is(err, storage.ErrEncrypted) {
		return nil, nil, fmt.Errorf("database %s has a password; the bot can't open those: use it through a cents server", cfg.Database)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("database: %w", err)
	}

	store := storage.NewSQLiteStorage(db)
	svc := service.On(store)

	return svc, func() { _ = store.Close() }, nil
}

// CheckSetup checks cfg works: the bot's token, and the backend answering.
// It returns what it found.
func CheckSetup(ctx context.Context, cfg Config) (string, error) {
	b, done, err := Connect(cfg)
	if err != nil {
		return "", err
	}
	defer done()

	me, err := telegram.New(apiOf(cfg), cfg.Telegram.Token).Me(ctx)
	if err != nil {
		return "", fmt.Errorf("telegram: checking the bot's token: %w", err)
	}

	info, err := b.Info()
	if err != nil {
		return "", fmt.Errorf("%s: %w", backendName(cfg), err)
	}

	report := fmt.Sprintf("Telegram: @%s\nData: %s (base currency %s)\nAllowed users: %d", me.Username, backendName(cfg), info.BaseCurrency, len(cfg.Telegram.AllowedUsers))

	// What Telegram shows as the menu button, which the bot sets when it
	// starts with web_app.url.
	tg := telegram.New(apiOf(cfg), cfg.Telegram.Token)
	describe := func(button telegram.MenuButton) string {
		switch button.Type {
		case "web_app":
			return fmt.Sprintf("opens %s (%q)", button.WebApp.URL, button.Text)
		case "commands":
			return "shows the list of commands"
		default:
			return "the same as every chat's"
		}
	}

	if cfg.WebApp.on() {
		report += "\nMini App: " + cfg.WebApp.URL + ", served on " + cfg.WebApp.listen()
	} else {
		report += "\nMini App: off (no web_app.url)"
	}

	if button, err := tg.GetMenuButton(ctx, 0); err == nil {
		report += "\nMenu button, every chat: " + describe(button)
	}

	for _, user := range cfg.Telegram.AllowedUsers {
		if button, err := tg.GetMenuButton(ctx, user); err == nil {
			report += fmt.Sprintf("\nMenu button, chat with %d: %s", user, describe(button))
		}
	}

	return report, nil
}

func apiOf(cfg Config) string {
	if api := strings.TrimSpace(cfg.Telegram.API); api != "" {
		return api
	}

	return telegram.DefaultAPI
}

func backendName(cfg Config) string {
	if cfg.Server != nil && strings.TrimSpace(cfg.Server.URL) != "" {
		return cfg.Server.URL
	}

	return cfg.Database
}

func newLogger(cfg Log, output io.Writer) (*slog.Logger, error) {
	levels := map[string]slog.Level{"": slog.LevelInfo, "info": slog.LevelInfo, "debug": slog.LevelDebug, "warn": slog.LevelWarn, "error": slog.LevelError}
	level, ok := levels[strings.ToLower(strings.TrimSpace(cfg.Level))]
	if !ok {
		return nil, fmt.Errorf("log level %q: use debug, info, warn or error", cfg.Level)
	}

	options := &slog.HandlerOptions{Level: level}
	switch strings.ToLower(strings.TrimSpace(cfg.Format)) {
	case "", "text":
		return slog.New(slog.NewTextHandler(output, options)), nil
	case "json":
		return slog.New(slog.NewJSONHandler(output, options)), nil
	default:
		return nil, fmt.Errorf("log format %q: use text or json", cfg.Format)
	}
}
