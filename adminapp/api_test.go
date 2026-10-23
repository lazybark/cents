package adminapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lazybark/cents/config"
	"github.com/lazybark/cents/logview"
)

func logServer(t *testing.T, tls bool) *httptest.Server {
	t.Helper()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/admin/logs" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer admin-secret" {
			w.WriteHeader(403)
			fmt.Fprint(w, `{"error":{"code":"admin_required","message":"administrator permission required"}}`)
			return
		}
		if r.URL.Query().Get("limit") == "2" && (r.URL.Query().Get("search") != "error & x" || r.URL.Query().Get("beforeId") != "42" || r.URL.Query().Get("client") != "mini_app_api" || r.URL.Query().Get("status") != "4xx") {
			t.Error("filters were not encoded correctly")
		}
		fmt.Fprint(w, `{"entries":[{"id":1,"message":"started","attributes":"{}"}],"components":["server"],"hasMore":false,"nextBefore":0}`)
	})
	var s *httptest.Server
	if tls {
		s = httptest.NewTLSServer(h)
	} else {
		s = httptest.NewServer(h)
	}
	t.Cleanup(s.Close)
	return s
}

func TestConnectionPermissionsRememberAndDisconnect(t *testing.T) {
	s := logServer(t, false)
	dir := t.TempDir()
	budgetPath := filepath.Join(dir, "config.yml")
	if err := config.Save(budgetPath, config.Config{DBPath: "budget.db"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "admin.yml")
	a, err := newAPI(path)
	if err != nil {
		t.Fatal(err)
	}
	defer a.cancel()
	if _, err := a.ReadLogs(logview.Query{}); err == nil {
		t.Fatal("disconnected read accepted")
	}
	if _, err := a.Connect(Connection{URL: s.URL, Token: "ordinary-secret", Remember: true}); err == nil || !strings.Contains(err.Error(), "administrator") {
		t.Fatalf("must check admin permission: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("failed connection saved credentials")
	}
	result, err := a.Connect(Connection{URL: s.URL + "/", Token: " admin-secret ", Remember: true})
	if err != nil || !result.Connected {
		t.Fatalf("connect: %+v %v", result, err)
	}
	page, err := a.ReadLogs(logview.Query{Limit: 2, BeforeID: 42, Search: "error & x", Client: "mini_app_api", Status: "4xx"})
	if err != nil || len(page.Entries) != 1 {
		t.Fatalf("read: %+v %v", page, err)
	}
	status, _ := json.Marshal(a.Status())
	if strings.Contains(string(status), "secret") {
		t.Fatal("status leaked token")
	}
	stat, err := os.Stat(path)
	if err != nil || stat.Mode().Perm() != 0o600 {
		t.Fatalf("credential file permissions: %v %v", stat, err)
	}
	restarted, err := newAPI(path)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.cancel()
	if restarted.Status().Connected || !restarted.Status().Remembered {
		t.Fatal("remembered connection status incorrect")
	}
	if err := restarted.Reconnect(); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Disconnect(); err != nil {
		t.Fatal(err)
	}
	if restarted.Status().Connected || restarted.Status().Remembered {
		t.Fatal("disconnect left connection")
	}
	cfg, err := config.Load(path)
	if err != nil || cfg.Server != nil {
		t.Fatal("disconnect retained credentials")
	}
	if _, err := a.Connect(Connection{URL: s.URL, Token: "admin-secret", Remember: false}); err != nil {
		t.Fatal(err)
	}
	cfg, err = config.Load(path)
	if err != nil || cfg.Server != nil || a.Status().Remembered {
		t.Fatal("session-only token saved")
	}
	budget, err := config.Load(budgetPath)
	if err != nil || budget.DBPath != "budget.db" {
		t.Fatal("admin overwrote desktop settings")
	}
}

func TestCertificateRequiresExplicitTrust(t *testing.T) {
	s := logServer(t, true)
	a, err := newAPI(filepath.Join(t.TempDir(), "admin.yml"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.cancel()
	input := Connection{URL: s.URL, Token: "admin-secret"}
	result, err := a.Connect(input)
	if err != nil || !result.Untrusted || result.Connected || result.Certificate == "" || a.Status().Connected {
		t.Fatalf("expected trust prompt: %+v %v", result, err)
	}
	input.Certificate = result.Certificate
	result, err = a.Connect(input)
	if err != nil || !result.Connected {
		t.Fatalf("explicit trust failed: %+v %v", result, err)
	}
	input.Certificate = strings.Repeat("00", 32)
	if _, err := a.Connect(input); err == nil {
		t.Fatal("certificate mismatch was accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := connect(ctx, config.Server{URL: s.URL, Token: "admin-secret", Certificate: input.Certificate}); err == nil {
		t.Fatal("cancelled connect accepted")
	}
}
