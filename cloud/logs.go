package cloud

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// logStore duplicates JSON logs to stdout and an independent SQLite database.
// A bounded queue keeps a slow/full log disk from stopping budget requests.
type logStore struct {
	db        *sql.DB
	out       io.Writer
	mu        sync.Mutex
	queue     chan map[string]any
	done      chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
	retention time.Duration
	redact    *strings.Replacer
	dropped   int
	closed    bool
}

func openLogs(path string, days int, out io.Writer, secrets []string) (*logStore, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: path}
	u.RawQuery = "_journal_mode=WAL&_busy_timeout=1000&_synchronous=NORMAL"
	db, err := sql.Open("sqlite3", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS logs (
		id INTEGER PRIMARY KEY, timestamp TEXT NOT NULL, level TEXT NOT NULL,
		component TEXT NOT NULL, message TEXT NOT NULL, attributes TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS logs_timestamp ON logs(timestamp);
	CREATE INDEX IF NOT EXISTS logs_component ON logs(component, id);`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	var pairs []string
	for _, secret := range secrets {
		if secret != "" {
			pairs = append(pairs, secret, "[REDACTED]")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &logStore{db: db, out: out, queue: make(chan map[string]any, 256), done: make(chan struct{}), ctx: ctx, cancel: cancel, retention: time.Duration(days) * 24 * time.Hour, redact: strings.NewReplacer(pairs...)}
	go s.run()
	return s, nil
}

func (s *logStore) writer(component string) io.Writer { return componentWriter{s, component} }

type componentWriter struct {
	store     *logStore
	component string
}

func (w componentWriter) Write(p []byte) (int, error) {
	s := w.store
	// Redact the complete serialized event, including secrets in error URLs.
	clean := s.redact.Replace(string(p))
	var event map[string]any
	decoder := json.NewDecoder(strings.NewReader(clean))
	decoder.UseNumber()
	if decoder.Decode(&event) != nil || event == nil {
		event = map[string]any{"time": time.Now().UTC().Format(time.RFC3339Nano), "level": "ERROR", "msg": strings.TrimSpace(clean)}
	}
	stamp, _ := event["time"].(string)
	at, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		at = time.Now()
	}
	event["time"] = at.UTC().Format(logTimeFormat)
	event["component"] = w.component
	line, _ := json.Marshal(event)
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.out.Write(append(line, '\n'))
	if s.closed {
		return len(p), nil
	}
	select {
	case s.queue <- event:
	default:
		s.dropped++
		if s.dropped == 1 || s.dropped%100 == 0 {
			s.warningLocked("log queue full", s.dropped)
		}
	}
	return len(p), nil
}

func (s *logStore) warningLocked(message string, count int) {
	line, _ := json.Marshal(map[string]any{"time": time.Now().UTC().Format(time.RFC3339Nano), "level": "WARN", "component": "logging", "msg": message, "count": count})
	_, _ = s.out.Write(append(line, '\n'))
}

func (s *logStore) run() {
	defer close(s.done)
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	s.prune()
	for {
		select {
		case <-s.ctx.Done():
			s.mu.Lock()
			s.warningLocked("log flush deadline reached; remaining events are in stdout", len(s.queue))
			s.mu.Unlock()
			return
		case event, ok := <-s.queue:
			if !ok {
				return
			}
			attrs := make(map[string]any, len(event))
			for k, v := range event {
				if k != "time" && k != "level" && k != "component" && k != "msg" {
					attrs[k] = v
				}
			}
			encoded, _ := json.Marshal(attrs)
			ctx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
			_, err := s.db.ExecContext(ctx, "INSERT INTO logs(timestamp, level, component, message, attributes) VALUES (?, ?, ?, ?, ?)", event["time"], event["level"], event["component"], event["msg"], string(encoded))
			cancel()
			if err != nil {
				s.mu.Lock()
				s.warningLocked("could not persist log; event remains in stdout", 1)
				s.mu.Unlock()
			}
		case <-ticker.C:
			s.prune()
		}
	}
}

func (s *logStore) prune() {
	if s.retention <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
	defer cancel()
	_, err := s.db.ExecContext(ctx, "DELETE FROM logs WHERE timestamp < ?", time.Now().Add(-s.retention).UTC().Format(logTimeFormat))
	if err != nil {
		s.mu.Lock()
		s.warningLocked("could not prune old logs", 1)
		s.mu.Unlock()
	}
}

// close is called after all services have stopped writing and drains the queue.
func (s *logStore) close() error {
	s.mu.Lock()
	s.closed = true
	close(s.queue)
	s.mu.Unlock()
	// Bound shutdown even when another SQLite connection holds a write lock.
	timer := time.AfterFunc(10*time.Second, s.cancel)
	<-s.done
	timer.Stop()
	s.cancel()
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("closing logs database: %w", err)
	}
	return nil
}

const logTimeFormat = "2006-01-02T15:04:05.000000000Z"
