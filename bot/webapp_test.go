package bot

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/lazybark/cents/service"
)

type panickingWebData struct{ Data }

func (panickingWebData) Info() (service.Info, error) { panic("private budget details") }

func TestWebAppRequestLogs(t *testing.T) {
	bt, _, svc := newTestBot(t)
	var logs bytes.Buffer
	bt.log = slog.New(slog.NewJSONHandler(&logs, nil))
	for _, tc := range []struct {
		path, auth string
		data       Data
		status     int
		client     string
	}{
		{"/api/v1/info?private=secret-query", "tma " + signIn(owner, testBotToken, time.Now()), svc, 200, "mini_app_api"},
		{"/api/v1/info", "tma invalid-secret-signin", svc, 401, "mini_app_api"},
		{"/api/v1/missing", "tma " + signIn(owner, testBotToken, time.Now()), svc, 404, "mini_app_api"},
		{"/missing.js", "", svc, 404, "mini_app"},
		{"/api/v1/info", "tma " + signIn(owner, testBotToken, time.Now()), panickingWebData{Data: svc}, 500, "mini_app_api"},
	} {
		logs.Reset()
		r := httptest.NewRequest("GET", tc.path, nil)
		r.Header.Set("Authorization", tc.auth)
		w := httptest.NewRecorder()
		bt.webAppHandler(tc.data, testBotToken).ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("%s: got %d want %d", tc.path, w.Code, tc.status)
		}
		var event map[string]any
		if err := json.Unmarshal(logs.Bytes(), &event); err != nil {
			t.Fatalf("invalid log: %v %s", err, logs.String())
		}
		if event["status"] != float64(tc.status) || event["client"] != tc.client || event["method"] != "GET" || event["path"] != r.URL.Path {
			t.Fatalf("missing request context: %+v", event)
		}
		for _, secret := range []string{tc.auth, "secret-query", "private budget details"} {
			if secret != "" && strings.Contains(logs.String(), secret) {
				t.Fatalf("request log leaked %q", secret)
			}
		}
		if tc.status >= 500 && event["level"] != "ERROR" {
			t.Fatal("server failure must be an error log")
		}
	}
	logs.Reset()
	w := httptest.NewRecorder()
	bt.webAppHandler(svc, testBotToken).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 200 || logs.Len() != 0 {
		t.Fatal("successful static health probes should not produce bot request logs")
	}
}

const testBotToken = "123456:test-token"

// signIn makes initData for user as Telegram does, signed with token.
func signIn(user int64, token string, at time.Time) string {
	values := url.Values{}
	values.Set("query_id", "AAH")
	values.Set("user", fmt.Sprintf(`{"id":%d,"first_name":"Test","username":"test"}`, user))
	values.Set("auth_date", fmt.Sprint(at.Unix()))

	var lines []string
	for key := range values {
		lines = append(lines, key+"="+values.Get(key))
	}

	sort.Strings(lines)
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(token))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(lines, "\n")))
	values.Set("hash", hex.EncodeToString(mac.Sum(nil)))

	return values.Encode()
}

func TestSignedInUser(t *testing.T) {
	now := time.Now()
	if user, err := signedInUser(signIn(owner, testBotToken, now), testBotToken, now); err != nil || user != owner {
		t.Fatalf("expected the owner: %d %v", user, err)
	}

	good := signIn(owner, testBotToken, now)
	for name, initData := range map[string]string{
		"nothing":       "",
		"another bot":   signIn(owner, "999:other", now),
		"no hash":       strings.Split(good, "&hash=")[0],
		"changed user":  strings.Replace(good, fmt.Sprint(owner), fmt.Sprint(stranger), 1),
		"bad hash":      good[:len(good)-2] + "zz",
		"not a query":   "%%%",
		"old":           signIn(owner, testBotToken, now.Add(-signInFor-time.Minute)),
		"no user in it": signIn(0, testBotToken, now),
	} {
		if _, err := signedInUser(initData, testBotToken, now); err == nil {
			t.Errorf("%s: expected refused", name)
		}
	}

	if _, err := signedInUser(signIn(owner, testBotToken, now.Add(-signInFor-time.Minute)), testBotToken, now); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected an old sign-in told it expired: %v", err)
	}
}

func TestWebAppHandler(t *testing.T) {
	bt, _, svc := newTestBot(t)
	srv := httptest.NewServer(bt.webAppHandler(svc, testBotToken))
	t.Cleanup(srv.Close)

	get := func(path, initData string) (*http.Response, string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, srv.URL+path, nil)
		if initData != "" {
			req.Header.Set("Authorization", "tma "+initData)
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()

		body, _ := io.ReadAll(res.Body)
		return res, string(body)
	}

	// The page needs no sign-in; the API does.
	if res, body := get("/", ""); res.StatusCode != 200 || !strings.Contains(body, "telegram-web-app.js") || res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("expected the page: %d", res.StatusCode)
	}

	if res, body := get("/app.js", ""); res.StatusCode != 200 || !strings.Contains(body, "initData") {
		t.Fatalf("expected the script: %d", res.StatusCode)
	}

	if res, body := get("/api/v1/info", ""); res.StatusCode != 401 || !strings.Contains(body, "open the app from the bot") {
		t.Fatalf("expected a sign-in asked for: %d %s", res.StatusCode, body)
	}

	if res, body := get("/api/v1/info", signIn(owner, "999:other", time.Now())); res.StatusCode != 401 {
		t.Fatalf("expected another bot's sign-in refused: %d %s", res.StatusCode, body)
	}

	if res, body := get("/api/v1/info", signIn(stranger, testBotToken, time.Now())); res.StatusCode != 403 || !strings.Contains(body, "666") {
		t.Fatalf("expected a stranger refused, told their ID: %d %s", res.StatusCode, body)
	}

	if res, body := get("/api/v1/accounts", signIn(owner, testBotToken, time.Now())); res.StatusCode != 200 || !strings.Contains(body, `"name":"Bank"`) {
		t.Fatalf("expected the owner's accounts: %d %s", res.StatusCode, body)
	}

	// Changes too, as through the server.
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/cashflow/entries", strings.NewReader(`{"currency":"$","amount":"7","date":"`+time.Now().Format("2006-01-02")+`","category":"Food"}`))
	req.Header.Set("Authorization", "tma "+signIn(owner, testBotToken, time.Now()))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != 201 {
		t.Fatalf("expected the entry added: %v %v", res.StatusCode, err)
	}

	if rows := entries(t, svc); len(rows) != 1 || rows[0].AmountCents != 700 {
		t.Fatalf("unexpected entries %+v", rows)
	}
}

func TestWebAppConfig(t *testing.T) {
	for want, w := range map[string]WebApp{
		"only from https":  {URL: "http://cents.example.com"},
		"web_app.listen":   {URL: "https://cents.example.com", Listen: "8750"},
		"only from https ": {URL: "cents.example.com"},
	} {
		if err := w.check(); err == nil || !strings.Contains(err.Error(), strings.TrimSpace(want)) {
			t.Errorf("expected %q, got %v", want, err)
		}
	}

	if err := (WebApp{URL: "https://cents.example.com/app/"}).check(); err != nil {
		t.Fatal(err)
	}

	if (WebApp{}).on() || (WebApp{}).listen() != DefaultWebAppListen || (WebApp{}).button() != "cents" {
		t.Fatal("unexpected defaults")
	}
}
