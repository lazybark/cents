package cloud

import (
	"bytes"
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLogsPersistRedactFlushAndRetain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs.db")
	var out bytes.Buffer
	s, err := openLogs(path, 30, &out, []string{"123:telegram-secret", "cents_server-secret"})
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewJSONHandler(s.writer("bot"), nil))
	log.Error("poll failed", "error", fmt.Errorf("Post https://api.telegram.org/bot123:telegram-secret/getUpdates: failed"), "token", "cents_server-secret", "user", int64(9007199254740993))
	var writers sync.WaitGroup
	for range 8 {
		writers.Go(func() { slog.New(slog.NewJSONHandler(s.writer("gateway"), nil)).Info("request", "status", 200) })
	}
	writers.Wait()
	if err := s.close(); err != nil {
		t.Fatal(err)
	}
	// Late cancellation logs must not panic after the queue is closed.
	log.Info("late log")
	for _, secret := range []string{"123:telegram-secret", "cents_server-secret"} {
		if strings.Contains(out.String(), secret) {
			t.Fatalf("stdout leaked %s", secret)
		}
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow("SELECT count(*) FROM logs").Scan(&count); err != nil || count != 9 {
		t.Fatalf("flush count %d: %v", count, err)
	}
	var attrs string
	if err := db.QueryRow("SELECT attributes FROM logs WHERE component = 'bot'").Scan(&attrs); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(attrs, "[REDACTED]") || strings.Contains(attrs, "telegram-secret") || strings.Contains(attrs, "server-secret") || !strings.Contains(attrs, "9007199254740993") {
		t.Fatalf("bad stored event: %s", attrs)
	}
	_, err = db.Exec("INSERT INTO logs(timestamp,level,component,message,attributes) VALUES (?, 'INFO', 'old', 'expired', '{}')", time.Now().Add(-31*24*time.Hour).UTC().Format(logTimeFormat))
	if err != nil {
		t.Fatal(err)
	}
	s, err = openLogs(path, 30, &out, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.close(); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT count(*) FROM logs WHERE component = 'old'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("retention count %d: %v", count, err)
	}
}

func TestLogStorageFailureKeepsStdout(t *testing.T) {
	var out bytes.Buffer
	s, err := openLogs(filepath.Join(t.TempDir(), "logs.db"), 0, &out, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a database that becomes unwritable after successful startup.
	if _, err := s.db.Exec("PRAGMA query_only = ON"); err != nil {
		t.Fatal(err)
	}
	slog.New(slog.NewJSONHandler(s.writer("gateway"), nil)).Info("request processed")
	if err := s.close(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "request processed") || !strings.Contains(out.String(), "could not persist log") {
		t.Fatalf("expected stdout event and storage failure: %s", out.String())
	}
}
