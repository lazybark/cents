package cloud

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCloudConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cloud.yml")
	if err := os.WriteFile(path, []byte("listen: 127.0.0.1:8080\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogsDatabase != filepath.Join(dir, "logs.db") || cfg.LogRetentionDays != 30 || len(cfg.TrustedProxies) != 0 {
		t.Fatalf("bad defaults: %+v", cfg)
	}
	if _, err := cfg.check(); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"listen: 8080\n", "trusted_proxies: [nonsense]\n", "log_retention_days: -1\n", "log_retention_days: 3651\n"} {
		if err := os.WriteFile(path, []byte(invalid), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadConfig(path)
		if err == nil {
			_, err = cfg.check()
		}
		if err == nil {
			t.Fatalf("accepted %s", invalid)
		}
	}
	if err := os.WriteFile(path, []byte("log_databse: typo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("accepted typo")
	}
	if _, err := loopbackURL("0.0.0.0:8740"); err == nil {
		t.Fatal("accepted public upstream")
	}
	if _, err := loopbackURL("[::1]:8740"); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"https://example.com/", "http://example.com/app/", "https://example.com/app/?token=x"} {
		if miniURL(raw) == nil {
			t.Fatalf("accepted Mini App URL %s", raw)
		}
	}
}
