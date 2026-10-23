package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestGatewayRoutingAndProxyBoundary(t *testing.T) {
	upstream := func(name string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Seen-IP", r.Header.Get("Fly-Client-IP"))
			w.Header().Set("Seen-Forwarded", r.Header.Get("X-Forwarded-For"))
			w.Header().Set("Seen-Auth", r.Header.Get("Authorization"))
			_, _ = io.WriteString(w, name+":"+r.URL.RequestURI())
		}))
	}
	api, mini := upstream("api"), upstream("mini")
	defer api.Close()
	defer mini.Close()
	apiURL, _ := url.Parse(api.URL)
	miniURL, _ := url.Parse(mini.URL)
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	h := gateway(apiURL, miniURL, []netip.Prefix{netip.MustParsePrefix("fdaa::/16")}, log)
	for _, tc := range []struct {
		path, want string
		status     int
	}{
		{"/app/", "mini:/", 200},
		{"/app/app.js", "mini:/app.js", 200},
		{"/app/api/v1/info?private=secret-query", "mini:/api/v1/info?private=secret-query", 200},
		{"/api/v1/info", "api:/api/v1/info", 200},
		{"/api/v1/admin/logs", "api:/api/v1/admin/logs", 200},
		{"/app", "", 308}, {"/", "", 308}, {"/unknown", "", 404}, {"/healthz", `{"status":"ok"}`, 200},
	} {
		before := logs.Len()
		r := httptest.NewRequest("GET", tc.path, nil)
		r.RemoteAddr = "203.0.113.7:1234"
		r.Header.Set("Authorization", "tma secret-signin")
		r.Header.Set("Fly-Client-IP", "192.0.2.99")
		r.Header.Set("X-Forwarded-For", "192.0.2.99")
		r.Header.Set("X-Request-ID", "untrusted-id")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if tc.path != "/healthz" {
			var event map[string]any
			if err := json.Unmarshal(logs.Bytes()[before:], &event); err != nil {
				t.Fatal(err)
			}
			if event["client"] != routeName(r.URL.Path) || event["status"] != float64(tc.status) {
				t.Fatalf("missing request context: %+v", event)
			}
			if _, ok := event["route"]; ok {
				t.Fatal("route classification should now be in client")
			}
		}
		if w.Code != tc.status || tc.want != "" && w.Body.String() != tc.want {
			t.Fatalf("%s: got %d %s", tc.path, w.Code, w.Body.String())
		}
		if tc.status == 308 && w.Header().Get("Location") != "/app/" {
			t.Errorf("redirect: %s", w.Header().Get("Location"))
		}
		if w.Header().Get("Seen-IP") != "" {
			if w.Header().Get("Seen-IP") != "203.0.113.7" || w.Header().Get("Seen-Forwarded") != "" {
				t.Fatal("caller spoofed proxy identity")
			}
			if w.Header().Get("Seen-Auth") != "tma secret-signin" {
				t.Fatal("authentication was lost")
			}
		}
		if id := w.Header().Get("X-Request-ID"); id == "" || id == "untrusted-id" {
			t.Fatal("expected generated request ID")
		}
	}
	for _, secret := range []string{"secret-signin", "secret-query", "untrusted-id"} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("logs contain %q", secret)
		}
	}
	for _, header := range []string{"198.51.100.9", "malformed"} {
		r := httptest.NewRequest("GET", "/api/v1/info", nil)
		r.RemoteAddr = "[fdaa::1]:1234"
		r.Header.Set("Fly-Client-IP", header)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		want := header
		if header == "malformed" {
			want = "invalid"
		}
		if w.Header().Get("Seen-IP") != want {
			t.Fatalf("trusted proxy header %s: %s", header, w.Header().Get("Seen-IP"))
		}
	}
	mini.Close()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
	if w.Code != 503 {
		t.Fatalf("unhealthy Mini App: %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/app/", nil))
	if w.Code != 502 {
		t.Fatalf("unavailable upstream: %d", w.Code)
	}
}

func TestSupervisorStopsAndWaitsForSiblings(t *testing.T) {
	for _, panicService := range []bool{false, true} {
		stopped := make(chan struct{})
		ready := make(chan struct{})
		log := slog.New(slog.NewJSONHandler(io.Discard, nil))
		err := supervise(context.Background(), log, []task{
			{"bot", func(context.Context) error {
				<-ready
				if panicService {
					panic("private panic detail")
				}
				return nil
			}},
			{"server", func(ctx context.Context) error { close(ready); <-ctx.Done(); close(stopped); return nil }},
		})
		if err == nil || err.Error() != "bot exited unexpectedly" {
			t.Fatalf("unexpected result: %v", err)
		}
		select {
		case <-stopped:
		default:
			t.Fatal("returned before sibling stopped")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := supervise(ctx, slog.Default(), []task{{"server", func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }}}); err != nil {
		t.Fatalf("normal shutdown: %v", err)
	}
}

func TestWaitForAPICancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := waitForAPI(ctx, "http://127.0.0.1:1"); err == nil {
		t.Fatal("expected cancellation")
	}
}
