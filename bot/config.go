// Package bot is cents' Telegram bot: adding incomes and expenses with a
// quick message, and reports (the month, budgets, accounts, an overview).
// It works on any service.Backend: a cents server's (the usual way) or a
// database file's.
package bot

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Config is the bot's YAML config file. Secrets can come from the
// environment instead: CENTS_BOT_TELEGRAM_TOKEN and CENTS_BOT_SERVER_TOKEN.
type Config struct {
	Telegram Telegram `yaml:"telegram"`
	// Server is the cents server the bot uses; Database a database file
	// instead (one or the other).
	Server    *Server   `yaml:"server"`
	Database  string    `yaml:"database"`
	Reminders Reminders `yaml:"reminders"`
	WebApp    WebApp    `yaml:"web_app"`
	Log       Log       `yaml:"log"`
}

type Telegram struct {
	// Token is the bot's, from @BotFather.
	Token string `yaml:"token"`
	// AllowedUsers are the Telegram user IDs the bot answers; everyone else
	// is told their ID and nothing more.
	AllowedUsers []int64 `yaml:"allowed_users"`
	// API is the Bot API's address; empty is Telegram's.
	API string `yaml:"api"`
}

type Server struct {
	URL   string `yaml:"url"`
	Token string `yaml:"token"`
	// Certificate is the SHA-256 fingerprint of the server's certificate,
	// for one the system doesn't trust.
	Certificate string `yaml:"certificate_sha256"`
}

// Log says how events are logged: format text or json, level debug, info,
// warn or error.
type Log struct {
	Format string `yaml:"format"`
	Level  string `yaml:"level"`
}

// LoadConfig reads the config file at path, then the secrets from the
// environment. Unknown keys are an error, so a typo doesn't quietly drop a
// setting.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read the config: %w", err)
	}

	var cfg Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("failed to read the config %s: %w", path, err)
	}

	if token := os.Getenv("CENTS_BOT_TELEGRAM_TOKEN"); token != "" {
		cfg.Telegram.Token = token
	}

	if token := os.Getenv("CENTS_BOT_SERVER_TOKEN"); token != "" && cfg.Server != nil {
		cfg.Server.Token = token
	}

	return cfg, nil
}

// Check says what's missing or wrong, if anything.
func (cfg Config) Check() error {
	if strings.TrimSpace(cfg.Telegram.Token) == "" {
		return errors.New("telegram.token: make a bot with @BotFather in Telegram and put its token here (or in CENTS_BOT_TELEGRAM_TOKEN)")
	}

	if len(cfg.Telegram.AllowedUsers) == 0 {
		return errors.New("telegram.allowed_users: add your Telegram user ID (the bot tells it to anyone who writes to it)")
	}

	hasServer := cfg.Server != nil && strings.TrimSpace(cfg.Server.URL) != ""
	hasDatabase := strings.TrimSpace(cfg.Database) != ""
	switch {
	case hasServer && hasDatabase:
		return errors.New("set server or database, not both")
	case !hasServer && !hasDatabase:
		return errors.New("server: set the cents server to use (or database, for a database file)")
	case hasServer && strings.TrimSpace(cfg.Server.Token) == "":
		return errors.New("server.token: make one on the server with cents-server token new --name bot (or put it in CENTS_BOT_SERVER_TOKEN)")
	}

	if err := cfg.Reminders.check(); err != nil {
		return err
	}

	return cfg.WebApp.check()
}

// ExampleConfig is a starting point for the config file.
const ExampleConfig = `# cents Telegram bot config.

telegram:
  # The bot's token: make a bot with @BotFather in Telegram. It can come
  # from CENTS_BOT_TELEGRAM_TOKEN instead.
  token: ""
  # Telegram user IDs the bot answers. Anyone else who writes to it is told
  # their ID, so write to the bot once to learn yours.
  allowed_users: []

# The cents server to use. Make a token for the bot on the server with
# cents-server token new --name bot (it can come from CENTS_BOT_SERVER_TOKEN).
server:
  url: https://127.0.0.1:8740
  token: ""
  # For a certificate the system doesn't trust: its SHA-256 fingerprint,
  # as the server logs it when it starts.
  certificate_sha256: ""

# Or a database file instead of a server (not one an app has open).
# database: /path/to/cents.db

# Reminders the bot sends by itself, to every allowed user: payments,
# taxes, invoices and debts coming due (and overdue), budgets getting
# close or going over. Each is sent once; the state file keeps which were.
reminders:
  enabled: true
  # The first reminder comes this many days before the due date; another
  # comes on the day. A payment made by hand that's missed is reminded once
  # a week.
  days_ahead: 1
  # Sent only between these times (the bot's local time).
  from: "09:00"
  until: "21:00"
  # Which: payments, taxes, invoices, debts, budgets. Empty is all.
  kinds: []
  # Where sent reminders are kept; empty is bot-state.json next to this file.
  state_file: ""

# The Mini App: a page opened in Telegram from the bot's menu button, with
# the month, budgets and a form to add entries. Telegram opens it only from
# a public https address: run a tunnel (Cloudflare Tunnel, say) from that
# address to listen. The bot points its menu button at url when it starts,
# so a new address only needs a new url here. Empty url turns it off.
web_app:
  url: ""
  listen: 127.0.0.1:8750
  button: cents

# Logs: format text or json; level debug, info, warn or error.
log:
  format: text
  level: info
`
