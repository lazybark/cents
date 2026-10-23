package server

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lazybark/cents/service"
	storage "github.com/lazybark/cents/storage/sqlite"
)

// Failed authentications an address may have within throttleWindow before
// it's turned away for a while.
const (
	throttleFailures = 10
	throttleWindow   = time.Minute
)

// handler is the whole server: the endpoints behind the guard.
func handler(s options, b service.Backend, files service.Files) http.Handler {
	mux := routes(b, files, s.logger)
	if s.logsDatabase != "" {
		mux.Handle(apiPrefix+"/admin/logs", adminLogs(s.logsDatabase, s.logger))
	}
	return guard(s, newThrottle(throttleFailures, throttleWindow), mux)
}

// Handler serves b and files as cfg says (its database and listen address
// aside), logging to logOutput: to embed the server in another program, or
// to test a client against it. Run serves a database.
func Handler(cfg Config, b service.Backend, files service.Files, logOutput io.Writer) (http.Handler, error) {
	s, err := cfg.check(logOutput)
	if err != nil {
		return nil, err
	}

	return handler(s, b, files), nil
}

// Run serves cfg until ctx ends, then shuts down: requests in flight get a
// few seconds to finish, and the database is closed. Logs go to logOutput.
func Run(ctx context.Context, cfg Config, logOutput io.Writer) error {
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	s, err := cfg.check(logOutput)
	if err != nil {
		return err
	}

	log := s.logger
	if s.database == "" {
		return errors.New("database: the database file is required")
	}

	if _, err := os.Stat(s.database); err != nil {
		return fmt.Errorf("database %s: %w (create it with the desktop app first)", s.database, err)
	}

	db, _, err := storage.OpenDatabase(s.database)
	if errors.Is(err, storage.ErrEncrypted) {
		return fmt.Errorf("database %s is encrypted with a password; the server can't open encrypted databases yet", s.database)
	}
	if err != nil {
		return fmt.Errorf("failed to open the database: %w", err)
	}

	store := storage.NewSQLiteStorage(db)
	defer func() {
		if err := store.Close(); err != nil {
			log.Error("failed to close the database", "error", err)
		}
	}()

	svc := service.On(store)
	ratesDone := make(chan struct{})
	go func() {
		defer close(ratesDone)
		svc.KeepRatesFresh(ctx, func() { log.Info("exchange rates updated") })
	}()
	defer func() { stop(); <-ratesDone }()

	srv := &http.Server{
		Handler:           handler(s, svc, svc),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS12},
	}

	listener, err := net.Listen("tcp", s.listen)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.listen, err)
	}

	allowed := make([]string, 0, len(s.allow))
	for _, prefix := range s.allow {
		allowed = append(allowed, prefix.String())
	}

	useTLS := s.tls.Cert != ""
	log.Info("cents server started", "version", Version, "listen", listener.Addr().String(), "tls", useTLS, "allow", allowed, "clients", len(s.tokens), "database", s.database)
	if useTLS {
		// Clients trusting the certificate by its fingerprint (one the
		// system doesn't trust) compare it with this.
		fingerprint, err := certificateFingerprint(s.tls.Cert, s.tls.Key)
		if err != nil {
			_ = listener.Close()
			return fmt.Errorf("tls: %w", err)
		}

		log.Info("tls certificate", "sha256", fingerprint)
	}
	if !useTLS && !isLoopback(hostOf(s.listen)) {
		log.Warn("serving plain HTTP to other machines: tokens and data travel unencrypted")
	}

	served := make(chan error, 1)
	go func() {
		if useTLS {
			served <- srv.ServeTLS(listener, s.tls.Cert, s.tls.Key)
		} else {
			served <- srv.Serve(listener)
		}
	}()

	select {
	case err := <-served:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("shutting down")
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdown); err != nil {
			log.Error("shutdown", "error", err)
			_ = srv.Close()
		}
	}

	log.Info("cents server stopped")
	return nil
}

// certificateFingerprint is the SHA-256 of the certificate in certFile, as
// AB:CD:… (the way clients show it).
func certificateFingerprint(certFile, keyFile string) (string, error) {
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(pair.Certificate[0])
	parts := make([]string, len(sum))
	for i, b := range sum {
		parts[i] = fmt.Sprintf("%02X", b)
	}

	return strings.Join(parts, ":"), nil
}

func hostOf(listen string) string {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return listen
	}

	return host
}
