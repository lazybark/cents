package cloud

import (
	"bytes"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

type Config struct {
	Listen             string `yaml:"listen"`
	ServerConfig       string `yaml:"server_config"`
	BotConfig          string `yaml:"bot_config"`
	LogsDatabase       string `yaml:"logs_database"`
	LogRetentionDays   int    `yaml:"log_retention_days"`
	InitializeDatabase bool   `yaml:"initialize_database"`
	// Only these peers may provide Fly-Client-IP. No proxies trusted by default.
	TrustedProxies []string `yaml:"trusted_proxies"`
}

func LoadConfig(path string) (Config, error) {
	p, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	dir := filepath.Dir(path)
	cfg := Config{Listen: ":8080", ServerConfig: filepath.Join(dir, "server.yml"), BotConfig: filepath.Join(dir, "bot.yml"), LogsDatabase: filepath.Join(dir, "logs.db"), LogRetentionDays: 30}
	d := yaml.NewDecoder(bytes.NewReader(p))
	d.KnownFields(true)
	if err := d.Decode(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (cfg Config) check() ([]netip.Prefix, error) {
	if _, _, err := net.SplitHostPort(cfg.Listen); err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	if cfg.ServerConfig == "" || cfg.BotConfig == "" || cfg.LogsDatabase == "" {
		return nil, fmt.Errorf("server_config, bot_config and logs_database are required")
	}
	if cfg.LogRetentionDays < 0 || cfg.LogRetentionDays > 3650 {
		return nil, fmt.Errorf("log_retention_days must be 0 (keep forever) through 3650")
	}
	var proxies []netip.Prefix
	for _, raw := range cfg.TrustedProxies {
		p, err := netip.ParsePrefix(raw)
		if err != nil {
			return nil, fmt.Errorf("trusted_proxies: %w", err)
		}
		proxies = append(proxies, p.Masked())
	}
	return proxies, nil
}

// Upstreams must be HTTP on loopback: their private listeners must not bypass
// the gateway's trusted-proxy boundary or expose plain HTTP over the network.
func loopbackURL(listen string) (*url.URL, error) {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return nil, err
	}
	addr, err := netip.ParseAddr(host)
	if err != nil || !addr.IsLoopback() {
		return nil, fmt.Errorf("%q must use a loopback IP address", listen)
	}
	return &url.URL{Scheme: "http", Host: listen}, nil
}

func miniURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Path != "/app/" || u.RawQuery != "" || u.Fragment != "" || u.User != nil || strings.Contains(u.Host, " ") {
		return fmt.Errorf("web_app.url must be a public HTTPS address ending in /app/, such as https://YOUR_APP.fly.dev/app/")
	}
	return nil
}

const ExampleConfig = `# cents-cloud starts the API, bot (including Mini App) and gateway together.
listen: :8080
server_config: /data/server.yml
bot_config: /data/bot.yml
logs_database: /data/logs.db
log_retention_days: 30 # 0 keeps events forever; freed pages are reused
initialize_database: false # true creates an empty budget if the file is missing
# Fly's private IPv6 peers may assert Fly-Client-IP. Use [] outside Fly.
trusted_proxies: [fdaa::/16]
`
