// Package server is cents' storage server: it owns the SQLite database
// and offers it over a JSON REST API, so other clients (the desktop app, a
// bot, a phone) can use one database. Every request must come from an
// allowed address and carry a valid token.
package server

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

// DefaultListen is where the server listens unless told otherwise: this
// machine only.
const DefaultListen = "127.0.0.1:8740"

// Config is the server's YAML config file.
type Config struct {
	// Listen is the address to listen on, like 127.0.0.1:8740.
	Listen string `yaml:"listen"`
	// Database is the SQLite database file.
	Database string `yaml:"database"`
	// Allow lists the addresses and networks (CIDR) that may connect; empty
	// allows this machine only.
	Allow []string `yaml:"allow"`
	// TrustedProxies may supply the original client in Fly-Client-IP.
	// Empty ignores forwarding headers, as for a directly exposed server.
	TrustedProxies []string `yaml:"trusted_proxies"`
	TLS            TLS      `yaml:"tls"`
	// InsecureHTTP allows plain HTTP on an address other machines can
	// reach. Only for testing on a network you trust: tokens and data
	// travel unencrypted.
	InsecureHTTP bool `yaml:"insecure_http"`
	// Tokens are the clients allowed in, by name, with their tokens'
	// SHA-256 (see `cents-server token new`).
	Tokens []Token `yaml:"tokens"`
	Log    Log     `yaml:"log"`
}

// TLS is the certificate and key to serve HTTPS with; both or neither.
type TLS struct {
	Cert string `yaml:"cert"`
	Key  string `yaml:"key"`
}

// Token lets a client in. Only the token's hash is kept here. A read-only
// token can't change anything.
type Token struct {
	Name     string `yaml:"name"`
	SHA256   string `yaml:"sha256"`
	ReadOnly bool   `yaml:"read_only"`
}

// Log says how requests and events are logged: format text or json, level
// debug, info, warn or error.
type Log struct {
	Format string `yaml:"format"`
	Level  string `yaml:"level"`
}

// LoadConfig reads the config file at path; unknown keys are an error, so
// a typo doesn't quietly drop a setting.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read the config: %w", err)
	}

	var cfg Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("failed to read the config %s: %w", path, err)
	}

	return cfg, nil
}

// options is a checked Config, ready to use.
type options struct {
	listen         string
	database       string
	allow          []netip.Prefix
	trustedProxies []netip.Prefix
	tls            TLS
	tokens         map[[32]byte]client
	logger         *slog.Logger
}

// check validates cfg and fills in defaults. It refuses setups that would
// expose the database: no tokens, or plain HTTP beyond this machine
// (unless InsecureHTTP says so).
func (cfg Config) check(logOutput io.Writer) (options, error) {
	s := options{listen: strings.TrimSpace(cfg.Listen), database: strings.TrimSpace(cfg.Database), tls: cfg.TLS, tokens: map[[32]byte]client{}}
	if s.listen == "" {
		s.listen = DefaultListen
	}

	host, _, err := net.SplitHostPort(s.listen)
	if err != nil {
		return options{}, fmt.Errorf("listen %q: %w", s.listen, err)
	}

	if len(cfg.Allow) == 0 {
		cfg.Allow = []string{"127.0.0.1", "::1"}
	}

	for _, raw := range cfg.Allow {
		prefix, err := parseAllowed(raw)
		if err != nil {
			return options{}, err
		}

		s.allow = append(s.allow, prefix)
	}
	for _, raw := range cfg.TrustedProxies {
		prefix, err := parseAllowed(raw)
		if err != nil {
			return options{}, fmt.Errorf("trusted_proxies: %w", err)
		}
		s.trustedProxies = append(s.trustedProxies, prefix)
	}

	if (cfg.TLS.Cert == "") != (cfg.TLS.Key == "") {
		return options{}, errors.New("tls: give both the certificate and the key, or neither")
	}

	for _, file := range []string{cfg.TLS.Cert, cfg.TLS.Key} {
		if file != "" {
			if _, err := os.Stat(file); err != nil {
				return options{}, fmt.Errorf("tls: %w", err)
			}
		}
	}

	if cfg.TLS.Cert == "" && !isLoopback(host) && !cfg.InsecureHTTP {
		return options{}, fmt.Errorf("listen %q is reachable from other machines: set tls (cert and key), or insecure_http: true for testing on a network you trust", s.listen)
	}

	if len(cfg.Tokens) == 0 {
		return options{}, errors.New("tokens: add at least one (cents-server token new --name NAME)")
	}

	names := map[string]bool{}
	for _, t := range cfg.Tokens {
		name := strings.TrimSpace(t.Name)
		if name == "" || names[name] {
			return options{}, fmt.Errorf("tokens: each needs a name of its own (%q)", t.Name)
		}

		hash, err := parseHash(t.SHA256)
		if err != nil {
			return options{}, fmt.Errorf("tokens: %s: %w", name, err)
		}

		names[name] = true
		s.tokens[hash] = client{name: name, readOnly: t.ReadOnly}
	}

	s.logger, err = newLogger(cfg.Log, logOutput)
	if err != nil {
		return options{}, err
	}

	return s, nil
}

// parseAllowed reads an address (1.2.3.4) or network (10.0.0.0/8).
func parseAllowed(raw string) (netip.Prefix, error) {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "/") {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("allow %q: not an address or network", raw)
		}

		return prefix.Masked(), nil
	}

	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("allow %q: not an address or network", raw)
	}

	addr = addr.Unmap()
	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

// isLoopback reports whether a listen host only this machine can reach.
func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}

	addr, err := netip.ParseAddr(host)
	return err == nil && addr.IsLoopback()
}

func newLogger(cfg Log, output io.Writer) (*slog.Logger, error) {
	var level slog.Level
	switch strings.ToLower(strings.TrimSpace(cfg.Level)) {
	case "", "info":
		level = slog.LevelInfo
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return nil, fmt.Errorf("log level %q: use debug, info, warn or error", cfg.Level)
	}

	options := &slog.HandlerOptions{Level: level}
	switch strings.ToLower(strings.TrimSpace(cfg.Format)) {
	case "", "text":
		return slog.New(slog.NewTextHandler(output, options)), nil
	case "json":
		return slog.New(slog.NewJSONHandler(output, options)), nil
	default:
		return nil, fmt.Errorf("log format %q: use text or json", cfg.Format)
	}
}
