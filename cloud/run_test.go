package cloud

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/lazybark/cents/bot"
	"github.com/lazybark/cents/dbimport"
	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/server"
	storage "github.com/lazybark/cents/storage/sqlite"
	"go.yaml.in/yaml/v3"
)

func freeAddress(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

func TestCloudBundle(t *testing.T) {
	dir := t.TempDir()
	const telegramToken = "123:private-telegram-token"
	const apiToken = "cents_private-api-token"
	tg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			_, _ = io.WriteString(w, `{"ok":true,"result":{"id":123,"is_bot":true,"username":"test_bot"}}`)
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			// Read the request body so the server can observe client cancellation.
			_, _ = io.Copy(io.Discard, r.Body)
			select {
			case <-r.Context().Done():
				return
			case <-time.After(100 * time.Millisecond):
			}
			_, _ = io.WriteString(w, `{"ok":true,"result":[]}`)
		default:
			_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
		}
	}))
	defer tg.Close()
	apiAddress, miniAddress, publicAddress := freeAddress(t), freeAddress(t), freeAddress(t)
	sc := server.Config{Listen: apiAddress, Database: filepath.Join(dir, "cents.db"), Allow: []string{"0.0.0.0/0", "::/0"}, Tokens: []server.Token{{Name: "bot", SHA256: server.HashToken(apiToken)}}}
	bc := bot.Config{Telegram: bot.Telegram{Token: telegramToken, API: tg.URL, AllowedUsers: []int64{42}}, Server: &bot.Server{URL: "http://" + apiAddress, Token: apiToken}, WebApp: bot.WebApp{URL: "https://cents.example/app/", Listen: miniAddress}}
	writeConfig := func(path string, cfg any) {
		t.Helper()
		p, err := yaml.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, p, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := Config{Listen: publicAddress, ServerConfig: filepath.Join(dir, "server.yml"), BotConfig: filepath.Join(dir, "bot.yml"), LogsDatabase: filepath.Join(dir, "logs.db"), LogRetentionDays: 30, InitializeDatabase: true}
	writeConfig(cfg.ServerConfig, sc)
	writeConfig(cfg.BotConfig, bc)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, cfg, io.Discard) }()
	t.Cleanup(cancel)
	client := &http.Client{Timeout: time.Second}
	base := "http://" + publicAddress
	deadline := time.Now().Add(10 * time.Second)
	for {
		if err := probe(ctx, client, base+"/healthz"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("bundle never became ready")
		}
		select {
		case err := <-done:
			t.Fatalf("bundle exited during startup: %v", err)
		case <-time.After(20 * time.Millisecond):
		}
	}
	initData := telegramSignIn(telegramToken)
	for _, tc := range []struct {
		path, auth, contains string
		status               int
	}{
		{"/app/", "", "telegram-web-app.js", 200},
		{"/app/app.js", "", "api/v1", 200},
		{"/api/v1/info", "", "valid token", 401},
		{"/api/v1/info", "Bearer " + apiToken, "baseCurrency", 200},
		{"/app/api/v1/info", "", "open the app", 401},
		{"/app/api/v1/info", "Bearer " + apiToken, "open the app", 401},
		{"/app/api/v1/info", "tma " + initData, "baseCurrency", 200},
		{"/api/v1/info", "tma " + initData, "valid token", 401},
	} {
		req, _ := http.NewRequest("GET", base+tc.path, nil)
		req.Header.Set("Authorization", tc.auth)
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if res.StatusCode != tc.status || !strings.Contains(string(body), tc.contains) {
			t.Errorf("%s: %d %s", tc.path, res.StatusCode, body)
		}
	}
	// Upload a real snapshot larger than the normal 1 MB JSON body limit.
	// The request crosses the public gateway -> API -> service.
	sourcePath := filepath.Join(t.TempDir(), "seed.db")
	sourceDB, _, err := storage.OpenDatabase(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	sourceStore := storage.NewSQLiteStorage(sourceDB)
	if err := sourceStore.CreateAccount(&account.Account{Name: "Seed bank", Currency: "$", BalanceCents: 12500}); err != nil {
		t.Fatal(err)
	}
	if err := sourceDB.Exec("CREATE TABLE padding (data BLOB)").Error; err != nil {
		t.Fatal(err)
	}
	if err := sourceDB.Exec("INSERT INTO padding(data) VALUES (zeroblob(?))", 2<<20).Error; err != nil {
		t.Fatal(err)
	}
	if err := sourceStore.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, preview := range []bool{true, false, true} {
		req, _ := http.NewRequest("POST", base+"/api/v1/import?preview="+fmt.Sprint(preview), bytes.NewReader(data))
		req.Header.Set("Authorization", "Bearer "+apiToken)
		req.Header.Set("Content-Type", "application/vnd.sqlite3")
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var result dbimport.Result
		if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("import via bundle: %d %+v", res.StatusCode, result)
		}
		check, _ := http.NewRequest("GET", base+"/api/v1/accounts", nil)
		check.Header.Set("Authorization", "Bearer "+apiToken)
		view, err := client.Do(check)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(view.Body)
		_ = view.Body.Close()
		present := strings.Contains(string(body), "Seed bank")
		if result.Added > 0 && preview && present {
			t.Fatal("preview imported data")
		}
		if !preview && !present {
			t.Fatalf("commit failed: %s", body)
		}
		if result.Added == 0 && !present {
			t.Fatal("nothing imported")
		}
	}
	tgImport, _ := http.NewRequest("POST", base+"/app/api/v1/import?preview=false", bytes.NewReader(data[:100]))
	tgImport.Header.Set("Authorization", "tma "+initData)
	tgImport.Header.Set("Content-Type", "application/vnd.sqlite3")
	denied, err := client.Do(tgImport)
	if err != nil {
		t.Fatal(err)
	}
	_ = denied.Body.Close()
	if denied.StatusCode != 404 {
		t.Fatalf("Telegram exposed database import: %d", denied.StatusCode)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("bundle did not stop")
	}
	for _, addr := range []string{apiAddress, miniAddress, publicAddress} {
		l, err := net.Listen("tcp", addr)
		if err != nil {
			t.Fatalf("listener not released: %s: %v", addr, err)
		}
		_ = l.Close()
	}
	db, err := sql.Open("sqlite3", cfg.LogsDatabase)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query("SELECT DISTINCT component FROM logs ORDER BY component")
	if err != nil {
		t.Fatal(err)
	}
	var components []string
	for rows.Next() {
		var component string
		if err := rows.Scan(&component); err != nil {
			t.Fatal(err)
		}
		components = append(components, component)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(components, ",") != "bot,cloud,gateway,server" {
		t.Fatalf("missing component logs: %v", components)
	}
	var leaked int
	if err := db.QueryRow("SELECT count(*) FROM logs WHERE attributes LIKE ? OR attributes LIKE ?", "%"+telegramToken+"%", "%"+apiToken+"%").Scan(&leaked); err != nil || leaked != 0 {
		t.Fatalf("secret in logs: %d %v", leaked, err)
	}
}

func telegramSignIn(token string) string {
	v := url.Values{"auth_date": {fmt.Sprint(time.Now().Unix())}, "user": {`{"id":42}`}}
	var lines []string
	for key := range v {
		lines = append(lines, key+"="+v.Get(key))
	}
	sort.Strings(lines)
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(token))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(lines, "\n")))
	v.Set("hash", hex.EncodeToString(mac.Sum(nil)))
	return v.Encode()
}
