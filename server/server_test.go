package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/rates"
	"github.com/lazybark/cents/service"
	storage "github.com/lazybark/cents/storage/sqlite"
)

func testToken(t *testing.T) (string, Token) {
	t.Helper()
	value, hash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(value, tokenPrefix) || len(value) < 40 || HashToken(value) != hash {
		t.Fatalf("unexpected token %q %q", value, hash)
	}

	return value, Token{Name: "desktop", SHA256: hash}
}

func TestConfigChecks(t *testing.T) {
	_, tok := testToken(t)
	dir := t.TempDir()
	cert := filepath.Join(dir, "cert.pem")
	if err := os.WriteFile(cert, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	base := Config{Database: "cents.db", Tokens: []Token{tok}}
	s, err := base.check(&bytes.Buffer{})
	if err != nil || s.listen != DefaultListen || len(s.allow) != 2 || len(s.tokens) != 1 {
		t.Fatalf("expected defaults: %+v %v", s, err)
	}

	for name, c := range map[string]struct {
		change func(*Config)
		want   string
	}{
		"no tokens":        {func(c *Config) { c.Tokens = nil }, "at least one"},
		"bad hash":         {func(c *Config) { c.Tokens = []Token{{Name: "x", SHA256: "abc"}} }, "64 hex"},
		"same name twice":  {func(c *Config) { c.Tokens = append(c.Tokens, tok) }, "name of its own"},
		"bad network":      {func(c *Config) { c.Allow = []string{"10.0.0.0/33"} }, "not an address"},
		"bad listen":       {func(c *Config) { c.Listen = "8740" }, "listen"},
		"open over http":   {func(c *Config) { c.Listen = "0.0.0.0:8740" }, "reachable from other machines"},
		"half tls":         {func(c *Config) { c.TLS = TLS{Cert: cert} }, "both"},
		"missing tls file": {func(c *Config) { c.TLS = TLS{Cert: cert, Key: filepath.Join(dir, "nope")} }, "tls"},
		"bad log level":    {func(c *Config) { c.Log.Level = "loud" }, "log level"},
		"bad log format":   {func(c *Config) { c.Log.Format = "xml" }, "log format"},
	} {
		cfg := base
		cfg.Tokens = append([]Token{}, base.Tokens...)
		c.change(&cfg)
		if _, err := cfg.check(&bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: expected %q, got %v", name, c.want, err)
		}
	}

	if err := Run(context.Background(), Config{Tokens: []Token{tok}}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "database") {
		t.Fatalf("expected the database asked for: %v", err)
	}

	open := base
	open.Listen, open.InsecureHTTP = "0.0.0.0:8740", true
	open.Allow = []string{"192.168.1.0/24", "10.1.2.3", "::ffff:10.9.9.9"}
	if s, err := open.check(&bytes.Buffer{}); err != nil || s.allow[0].String() != "192.168.1.0/24" || s.allow[1].String() != "10.1.2.3/32" || s.allow[2].String() != "10.9.9.9/32" {
		t.Fatalf("unexpected %+v %v", s.allow, err)
	}

	// Unknown keys are an error, so a typo doesn't drop a setting.
	path := filepath.Join(dir, "server.yml")
	if err := os.WriteFile(path, []byte("listen: 127.0.0.1:1\nalow: [10.0.0.0/8]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil || !strings.Contains(err.Error(), "alow") {
		t.Fatalf("expected the typo caught: %v", err)
	}
	if err := os.WriteFile(path, []byte(ExampleConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	if cfg, err := LoadConfig(path); err != nil || cfg.Listen != DefaultListen {
		t.Fatalf("the example config should read: %+v %v", cfg, err)
	}
}

// testServer serves a database with an account and two entries, from
// 127.0.0.1 allowed and the token returned.
func testServer(t *testing.T, cfg Config) (http.Handler, string, *bytes.Buffer) {
	t.Helper()
	db, _, err := storage.OpenDatabase(filepath.Join(t.TempDir(), "cents.db"))
	if err != nil {
		t.Fatal(err)
	}

	store := storage.NewSQLiteStorage(db)
	t.Cleanup(func() { _ = store.Close() })
	for _, c := range []settings.SettingExpenseCategory{{CategoryName: "Food"}} {
		if err := store.SaveSettingExpenseCategory(&c); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SaveSettingCurrency(&settings.SettingCurrency{CurrencyName: "EUR", RateToBase: 1.1}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAccount(&account.Account{Name: "Bank", Currency: "EUR", BalanceCents: 10000}); err != nil {
		t.Fatal(err)
	}
	for _, e := range []cashflow.CashflowEntry{
		{Currency: "$", AmountCents: 1000, RateToBase: 1, AmountBaseCents: 1000, Category: "Food", Comment: "lunch", Tags: []string{"work"}, EntryDate: time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)},
		{Currency: "EUR", AmountCents: 500, Category: "Food", EntryDate: time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)},
	} {
		if err := store.CreateCashflow(&e); err != nil {
			t.Fatal(err)
		}
	}

	value, tok := testToken(t)
	cfg.Database = "unused"
	cfg.Tokens = append(cfg.Tokens, tok)
	logs := &bytes.Buffer{}
	s, err := cfg.check(logs)
	if err != nil {
		t.Fatal(err)
	}

	svc := service.On(store).WithRates(offlineRates{})
	return handler(s, svc, svc), value, logs
}

type offlineRates struct{}

func (offlineRates) Fetch(context.Context, string) (rates.Quote, error) {
	return rates.Quote{}, errors.New("offline in tests")
}

func call(h http.Handler, method, path, remote, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	r.RemoteAddr = remote
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestGuard(t *testing.T) {
	h, token, logs := testServer(t, Config{Allow: []string{"127.0.0.1", "192.168.1.0/24"}})

	if w := call(h, "GET", "/healthz", "127.0.0.1:5000", ""); w.Code != 200 || !strings.Contains(w.Body.String(), Version) {
		t.Fatalf("health needs no token from an allowed address: %d %s", w.Code, w.Body)
	}

	if w := call(h, "GET", "/api/v1/info", "10.0.0.7:5000", token); w.Code != 403 {
		t.Fatalf("expected an outside address refused even with a token: %d", w.Code)
	}

	if w := call(h, "GET", "/healthz", "10.0.0.7:5000", ""); w.Code != 403 {
		t.Fatalf("expected an outside address refused for health too: %d", w.Code)
	}

	w := call(h, "GET", "/api/v1/info", "192.168.1.20:5000", "")
	if w.Code != 401 || w.Header().Get("WWW-Authenticate") == "" || !strings.Contains(w.Body.String(), `"unauthorized"`) {
		t.Fatalf("expected a token asked for: %d %v %s", w.Code, w.Header(), w.Body)
	}

	if w := call(h, "GET", "/api/v1/info", "192.168.1.20:5000", token); w.Code != 200 {
		t.Fatalf("expected the token accepted: %d %s", w.Code, w.Body)
	}

	if w := call(h, "GET", "/api/v1/info", "127.0.0.1:5000", token); w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("expected security headers: %v", w.Header())
	}

	// Guessing: after ten failures the address is turned away, even with
	// the right token, until the window passes. Others aren't.
	for range throttleFailures {
		call(h, "GET", "/api/v1/info", "192.168.1.66:5000", "cents_guess")
	}
	if w := call(h, "GET", "/api/v1/info", "192.168.1.66:5000", token); w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatalf("expected the guesser throttled: %d", w.Code)
	}
	if w := call(h, "GET", "/api/v1/info", "192.168.1.67:5000", token); w.Code != 200 {
		t.Fatalf("other addresses go on: %d", w.Code)
	}

	// Logs name the client, never the token.
	if !strings.Contains(logs.String(), "client=desktop") || strings.Contains(logs.String(), token) || strings.Contains(logs.String(), "cents_guess") {
		t.Fatalf("unexpected logs:\n%s", logs)
	}
}

func TestThrottleWindow(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	th := newThrottle(2, time.Minute)
	th.now = func() time.Time { return now }
	addr := remoteAddr(&http.Request{RemoteAddr: "10.0.0.1:1"})

	th.fail(addr)
	th.fail(addr)
	if !th.blocked(addr) {
		t.Fatal("expected blocked")
	}

	now = now.Add(61 * time.Second)
	if th.blocked(addr) {
		t.Fatal("expected the window to pass")
	}
}
