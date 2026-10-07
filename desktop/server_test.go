package desktop

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lazybark/cents/server"
	"github.com/lazybark/cents/service"
	storage "github.com/lazybark/cents/storage/sqlite"
)

// testServer serves a database of its own, with an account on it, over
// plain HTTP (or HTTPS with a certificate of its own); it returns the
// server and a token for it.
func testServer(t *testing.T, tls bool) (*httptest.Server, string) {
	t.Helper()

	db, _, err := storage.OpenDatabase(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}

	store := storage.NewSQLiteStorage(db)
	t.Cleanup(func() { _ = store.Close() })
	svc := service.On(store).WithRates(offlineRates{})
	if err := svc.CreateAccount(service.NewAccountInput{Name: "On the server", Description: "x", Currency: "$", Amount: "5"}); err != nil {
		t.Fatal(err)
	}

	token, hash, err := server.NewToken()
	if err != nil {
		t.Fatal(err)
	}

	h, err := server.Handler(server.Config{Tokens: []server.Token{{Name: "desktop", SHA256: hash}}}, svc, svc, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewUnstartedServer(h)
	if tls {
		srv.StartTLS()
	} else {
		srv.Start()
	}

	t.Cleanup(srv.Close)

	return srv, token
}

func accountNames(t *testing.T, api *API) string {
	t.Helper()

	view, err := api.Accounts(0)
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, a := range view.Accounts {
		names = append(names, a.Name)
	}

	return strings.Join(names, ",")
}

func TestConnectingToAServer(t *testing.T) {
	api := newTestAPI(t)
	local, _ := api.currentStorage()
	var saved []Server
	api.opts.SaveServer = func(s Server) error {
		saved = append(saved, s)
		return nil
	}

	srv, token := testServer(t, false)

	for want, input := range map[string]ServerInput{
		"type the server's address":            {Token: token},
		"type the token":                       {URL: srv.URL},
		"a valid token is required":            {URL: srv.URL, Token: "cents_wrong"},
		"must be like https://host:port":       {URL: "ftp://cents.lan", Token: token},
		"couldn't reach the server":            {URL: "http://127.0.0.1:1", Token: token},
		"only be trusted for an https address": {URL: srv.URL, Token: token, Certificate: "AB:CD"},
	} {
		if _, err := api.ConnectServer(input); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("expected %q, got %v", want, err)
		}
	}

	if status := api.Status(); status.Server != "" || len(saved) != 0 || accountNames(t, api) != "" {
		t.Fatalf("a failed connection changes nothing: %+v %v", status, saved)
	}

	result, err := api.ConnectServer(ServerInput{URL: srv.URL + "/", Token: " " + token + " "})
	if err != nil || !result.Connected {
		t.Fatalf("expected connected: %+v %v", result, err)
	}

	status := api.Status()
	if !status.Ready || status.Server != srv.URL || status.DBPath != "" || status.NeedsCurrencies != true || accountNames(t, api) != "On the server" {
		t.Fatalf("expected the server in use: %+v", status)
	}

	if len(saved) != 1 || saved[0] != (Server{URL: srv.URL, Token: token}) {
		t.Fatalf("expected the server remembered: %+v", saved)
	}

	if _, err := local.LoadAccounts(); err == nil {
		t.Fatal("expected the database file closed")
	}

	if err := api.SetPassword(PasswordInput{New: "battery staple"}); err == nil || !strings.Contains(err.Error(), "server") {
		t.Fatalf("expected the password left to the server: %v", err)
	}

	// A backup downloads the server's database.
	dir := t.TempDir()
	api.pick = &fakePickers{file: filepath.Join(dir, "from-server")}
	backup, err := api.BackupDatabase()
	if err != nil || storage.CheckDatabase(backup.Path) != nil {
		t.Fatalf("expected the server's database saved: %+v %v", backup, err)
	}

	if view, _ := api.ExportOptions(); view.LastBackupPath != backup.Path || view.DBPath != "" {
		t.Fatalf("expected the backup remembered on the server: %+v", view)
	}

	// Picking a file goes back to it.
	other := filepath.Join(dir, "other.db")
	api.opts.Choose = func(path string, _ bool) (StorageWorker, string, error) {
		db, _, err := storage.OpenDatabase(path)
		if err != nil {
			return nil, "", err
		}

		return storage.NewSQLiteStorage(db), path, nil
	}
	api.pick = &fakePickers{file: other}
	if ok, err := api.CreateDatabase(); !ok || err != nil {
		t.Fatalf("expected the new file: %v", err)
	}

	if status := api.Status(); status.Server != "" || status.DBPath != other || accountNames(t, api) != "" {
		t.Fatalf("expected the file in use: %+v", status)
	}
}

func TestReconnecting(t *testing.T) {
	srv, token := testServer(t, false)

	api := offline(newAPI(Options{Server: &Server{URL: srv.URL, Token: token}}))
	if status := api.Status(); status.Ready || status.Server != srv.URL || status.ServerError != "" {
		t.Fatalf("expected the server waiting to be connected to: %+v", status)
	}

	if err := api.Reconnect(); err != nil || !api.Status().Ready || accountNames(t, api) != "On the server" {
		t.Fatalf("expected connected: %v %+v", err, api.Status())
	}

	// A server that's down: the error is kept to show, and trying again
	// works once it's back.
	down := offline(newAPI(Options{Server: &Server{URL: "http://127.0.0.1:1", Token: token}}))
	if err := down.Reconnect(); err == nil || down.Status().Ready || !strings.Contains(down.Status().ServerError, "couldn't reach") {
		t.Fatalf("expected the failure kept: %v %+v", err, down.Status())
	}

	if err := offline(newAPI(Options{})).Reconnect(); err == nil {
		t.Fatal("expected no server to connect to")
	}
}

func TestTrustingACertificate(t *testing.T) {
	srv, token := testServer(t, true)
	sum := sha256.Sum256(srv.Certificate().Raw)
	var want []string
	for _, b := range sum {
		want = append(want, fmt.Sprintf("%02X", b))
	}

	api := newTestAPI(t)
	result, err := api.ConnectServer(ServerInput{URL: srv.URL, Token: token})
	if err != nil || result.Connected || !result.Untrusted || result.Certificate != strings.Join(want, ":") {
		t.Fatalf("expected the fingerprint to trust: %+v %v", result, err)
	}

	if api.Status().Server != "" {
		t.Fatal("expected nothing switched before the certificate is trusted")
	}

	if _, err := api.ConnectServer(ServerInput{URL: srv.URL, Token: token, Certificate: strings.Repeat("00:", 31) + "00"}); err == nil || !strings.Contains(err.Error(), "isn't the one trusted") {
		t.Fatalf("expected another certificate refused: %v", err)
	}

	result, err = api.ConnectServer(ServerInput{URL: srv.URL, Token: token, Certificate: strings.ToLower(strings.ReplaceAll(result.Certificate, ":", ""))})
	if err != nil || !result.Connected || accountNames(t, api) != "On the server" {
		t.Fatalf("expected connected once trusted: %+v %v", result, err)
	}
}
