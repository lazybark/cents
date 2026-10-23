package cloud

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lazybark/cents/bot"
	"github.com/lazybark/cents/server"
	storage "github.com/lazybark/cents/storage/sqlite"
)

// Run shares a lifecycle across all three services. An unexpected exit cancels
// its siblings and returns an error; Fly restarts the whole bundle, without
// overlapping pollers or database owners inside the bundle.
func Run(ctx context.Context, cfg Config, output io.Writer) error {
	proxies, err := cfg.check()
	if err != nil {
		return err
	}
	sc, err := server.LoadConfig(cfg.ServerConfig)
	if err != nil {
		return fmt.Errorf("server config: %w", err)
	}
	bc, err := bot.LoadConfig(cfg.BotConfig)
	if err != nil {
		return fmt.Errorf("bot config: %w", err)
	}
	if sc.Listen == "" {
		sc.Listen = server.DefaultListen
	}
	if bc.WebApp.Listen == "" {
		bc.WebApp.Listen = bot.DefaultWebAppListen
	}
	api, err := loopbackURL(sc.Listen)
	if err != nil {
		return fmt.Errorf("server listen: %w", err)
	}
	mini, err := loopbackURL(bc.WebApp.Listen)
	if err != nil {
		return fmt.Errorf("Mini App listen: %w", err)
	}
	if sc.TLS.Cert != "" || sc.TLS.Key != "" {
		return fmt.Errorf("cloud mode uses HTTP on loopback; leave server tls empty (Fly terminates HTTPS)")
	}
	if err := miniURL(bc.WebApp.URL); err != nil {
		return err
	}
	if bc.Server == nil || bc.Server.URL != api.String() || bc.Database != "" {
		return fmt.Errorf("bot server.url must be %s, with no bot database", api)
	}
	if err := bc.Check(); err != nil {
		return fmt.Errorf("bot config: %w", err)
	}
	matched := false
	for _, token := range sc.Tokens {
		if strings.EqualFold(strings.TrimSpace(token.SHA256), server.HashToken(strings.TrimSpace(bc.Server.Token))) && !token.ReadOnly {
			matched = true
		}
	}
	if !matched {
		return fmt.Errorf("bot server token must match a writable token in server config")
	}
	budgetPath, err := filepath.Abs(sc.Database)
	if err != nil || sc.Database == "" {
		return fmt.Errorf("server database is required")
	}
	logPath, err := filepath.Abs(cfg.LogsDatabase)
	if err != nil {
		return err
	}
	if budgetPath == logPath {
		return fmt.Errorf("logs_database must be separate from the budget database")
	}
	budgetFile, budgetErr := os.Stat(budgetPath)
	logFile, logErr := os.Stat(logPath)
	if budgetErr == nil && logErr == nil && os.SameFile(budgetFile, logFile) {
		return fmt.Errorf("logs_database points to the budget database")
	}
	// Only this gateway's loopback connection supplies original addresses.
	sc.TrustedProxies = []string{"127.0.0.1", "::1"}
	sc.Log.Format, bc.Log.Format = "json", "json"
	if bc.Reminders.StateFile == "" {
		bc.Reminders.StateFile = bot.DefaultStateFile(cfg.BotConfig)
	}
	logs, err := openLogs(cfg.LogsDatabase, cfg.LogRetentionDays, output, []string{bc.Telegram.Token, bc.Server.Token})
	if err != nil {
		return fmt.Errorf("logs database: %w", err)
	}
	defer logs.close()
	log := slog.New(slog.NewJSONHandler(logs.writer("cloud"), nil))
	if os.IsNotExist(budgetErr) && cfg.InitializeDatabase {
		db, _, err := storage.OpenDatabase(budgetPath)
		if err != nil {
			return fmt.Errorf("initialize budget database: %w", err)
		}
		if err := storage.NewSQLiteStorage(db).Close(); err != nil {
			return err
		}
		log.Info("initialized empty budget database")
	}
	listener, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return fmt.Errorf("gateway listen: %w", err)
	}
	defer listener.Close()
	log.Info("cloud starting", "listen", listener.Addr().String(), "mini_app", bc.WebApp.URL, "logs_database", cfg.LogsDatabase)
	apiLog := slog.New(slog.NewJSONHandler(logs.writer("gateway"), nil))
	return supervise(ctx, log, []task{
		{"server", func(ctx context.Context) error { return server.Run(ctx, sc, logs.writer("server")) }},
		{"bot", func(ctx context.Context) error {
			if err := waitForAPI(ctx, api.String()); err != nil {
				return err
			}
			return bot.Run(ctx, bc, logs.writer("bot"))
		}},
		{"gateway", func(ctx context.Context) error {
			return serveGateway(ctx, listener, gateway(api, mini, proxies, apiLog), apiLog)
		}},
	})
}

type task struct {
	name string
	run  func(context.Context) error
}

func supervise(ctx context.Context, log *slog.Logger, tasks []task) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct {
		name string
		err  error
	}
	ended := make(chan result, len(tasks))
	for _, t := range tasks {
		go func() {
			var err error
			defer func() {
				if recover() != nil {
					err = fmt.Errorf("service panicked")
				}
				ended <- result{t.name, err}
			}()
			err = t.run(ctx)
		}()
	}
	var failure error
	for range tasks {
		r := <-ended
		if ctx.Err() == nil {
			failure = fmt.Errorf("%s exited unexpectedly", r.name)
			// Details go through the redacting log writer, not the CLI's stderr.
			log.Error("service exited; stopping bundle", "service", r.name, "error", r.err)
			cancel()
		}
	}
	log.Info("cloud stopped")
	return failure
}

func waitForAPI(ctx context.Context, address string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil}}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := probe(ctx, client, address+"/healthz"); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
