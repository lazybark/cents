package server

import (
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lazybark/cents/logview"
)

func TestAdminLogsAuthorizationAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE logs(id INTEGER PRIMARY KEY, timestamp TEXT, level TEXT, component TEXT, message TEXT, attributes TEXT);
	INSERT INTO logs VALUES(1,'2026-10-23T10:00:00.000000000Z','ERROR','bot','poll failed','{"status":500,"client":"mini_app_api"}')`); err != nil {
		t.Fatal(err)
	}
	admin, tok := testToken(t)
	tok.Name, tok.Admin, tok.ReadOnly = "admin", true, true
	h, ordinary, logs := testServer(t, Config{LogsDatabase: path, Tokens: []Token{tok}})
	for _, tc := range []struct {
		method, path, token string
		code                int
		contains            string
	}{
		{"GET", "/api/v1/admin/logs", "", 401, "unauthorized"},
		{"GET", "/api/v1/admin/logs", ordinary, 403, "admin_required"},
		{"GET", "/api/v1/admin/logs?level=error", admin, 200, "poll failed"},
		{"GET", "/api/v1/admin/logs?status=5xx&client=mini_app_api", admin, 200, "poll failed"},
		{"GET", "/api/v1/admin/logs?status=600", admin, 400, "HTTP status"},
		{"GET", "/api/v1/admin/logs?limit=wat", admin, 400, "limit"},
		{"GET", "/api/v1/admin/logs?limit=501", admin, 400, "limit"},
		{"GET", "/api/v1/admin/logs?beforeId=-2", admin, 400, "beforeId"},
		{"GET", "/api/v1/admin/logs?from=yesterday", admin, 400, "timestamp"},
		{"POST", "/api/v1/admin/logs", admin, 403, "read_only"},
	} {
		w := call(h, tc.method, tc.path, "127.0.0.1:5000", tc.token)
		if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.contains) {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("logs must never be cached")
		}
		if tc.code == 200 {
			var p logview.Page
			if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || len(p.Entries) != 1 {
				t.Fatalf("invalid log page: %+v %v", p, err)
			}
		}
	}
	if strings.Contains(logs.String(), admin) || strings.Contains(logs.String(), ordinary) {
		t.Fatal("request log leaked credentials")
	}
	// The Mini App's unguarded route collection must not expose administration.
	mini := Routes(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if w := call(mini, "GET", "/api/v1/admin/logs", "127.0.0.1:5000", admin); w.Code != 404 {
		t.Fatalf("Mini App exposes logs: %d", w.Code)
	}
	tok.ReadOnly = false
	h, _, _ = testServer(t, Config{LogsDatabase: path, Tokens: []Token{tok}})
	if w := call(h, "POST", "/api/v1/admin/logs", "127.0.0.1:5000", admin); w.Code != 405 || w.Header().Get("Allow") != "GET" {
		t.Fatalf("expected GET only: %d", w.Code)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if w := call(h, "GET", "/api/v1/admin/logs", "127.0.0.1:5000", admin); w.Code != 503 || strings.Contains(w.Body.String(), path) {
		t.Fatalf("missing DB must fail without exposing path: %d %s", w.Code, w.Body)
	}
}
