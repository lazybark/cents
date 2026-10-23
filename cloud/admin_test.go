package cloud

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lazybark/cents/client"
	"github.com/lazybark/cents/logview"
	"github.com/lazybark/cents/server"
)

// Read through the same API and client as the admin window while the actual
// cloud log writer has its WAL database open.
func TestAdminReadsLiveCloudLogs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs.db")
	store, err := openLogs(path, 30, io.Discard, []string{"telegram-secret"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.close()
	handler, err := server.Handler(server.Config{
		LogsDatabase: path,
		Tokens:       []server.Token{{Name: "admin", SHA256: server.HashToken("admin-secret"), Admin: true, ReadOnly: true}},
	}, nil, nil, store.writer("server"))
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	remote, err := client.New(httpServer.URL, "admin-secret")
	if err != nil {
		t.Fatal(err)
	}
	since := time.Now().UTC().Format(time.RFC3339Nano)
	slog.New(slog.NewJSONHandler(store.writer("bot"), nil)).Error("poll failed", "error", "telegram-secret")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		page, err := remote.ReadLogs(ctx, logview.Query{Component: "bot", Level: "ERROR", Search: "[REDACTED]", From: since})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Entries) == 1 {
			if page.Entries[0].Message != "poll failed" || strings.Contains(page.Entries[0].Attributes, "telegram-secret") {
				t.Fatalf("incorrect or unredacted event: %+v", page.Entries[0])
			}
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("cloud logs did not become visible through the administration API")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
