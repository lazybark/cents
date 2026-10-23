package adminapp

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/lazybark/cents/client"
	"github.com/lazybark/cents/config"
	"github.com/lazybark/cents/logview"
)

// API exposes administration only; it never opens a budget database.
type API struct {
	mu         sync.Mutex
	path       string
	server     *config.Server
	remote     *client.Client
	remembered bool
	ctx        context.Context
	cancel     context.CancelFunc
}

type Status struct {
	URL        string `json:"url"`
	Connected  bool   `json:"connected"`
	Remembered bool   `json:"remembered"`
}

type Connection struct {
	URL         string `json:"url"`
	Token       string `json:"token"`
	Certificate string `json:"certificate"`
	Remember    bool   `json:"remember"`
}

type ConnectionResult struct {
	Connected   bool   `json:"connected"`
	Untrusted   bool   `json:"untrusted"`
	Certificate string `json:"certificate"`
}

func newAPI(path string) (*API, error) {
	cfg, err := config.Load(path)
	if err != nil && !errors.Is(err, config.ErrNotFound) {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &API{path: path, server: cfg.Server, remembered: cfg.Server != nil, ctx: ctx, cancel: cancel}, nil
}

func (a *API) startup(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cancel()
	a.ctx, a.cancel = context.WithCancel(ctx)
}

// Status never exposes saved credentials to the frontend.
func (a *API) Status() Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := Status{Connected: a.remote != nil, Remembered: a.remembered}
	if a.server != nil {
		s.URL = a.server.URL
	}
	return s
}

func (a *API) Connect(input Connection) (ConnectionResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := config.Server{URL: strings.TrimRight(strings.TrimSpace(input.URL), "/"), Token: strings.TrimSpace(input.Token), Certificate: strings.TrimSpace(input.Certificate)}
	if s.URL != "" && !strings.Contains(s.URL, "://") {
		s.URL = "https://" + s.URL
	}
	remote, err := connect(a.ctx, s)
	if client.Untrusted(err) && s.Certificate == "" {
		fingerprint, err := client.Fingerprint(s.URL)
		if err != nil {
			return ConnectionResult{}, err
		}
		return ConnectionResult{Untrusted: true, Certificate: fingerprint}, nil
	}
	if err != nil {
		return ConnectionResult{}, err
	}
	cfg := config.Config{}
	if input.Remember {
		cfg.Server = &s
	}
	if err := config.Save(a.path, cfg); err != nil {
		return ConnectionResult{}, err
	}
	a.server, a.remote = &s, remote
	a.remembered = input.Remember
	return ConnectionResult{Connected: true}, nil
}

func (a *API) Reconnect() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.server == nil {
		return errors.New("no remembered server; enter its address and admin token")
	}
	remote, err := connect(a.ctx, *a.server)
	if err != nil {
		a.remote = nil
		return err
	}
	a.remote = remote
	return nil
}

func (a *API) Disconnect() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := config.Save(a.path, config.Config{}); err != nil {
		return err
	}
	a.server, a.remote = nil, nil
	a.remembered = false
	return nil
}

func (a *API) ReadLogs(query logview.Query) (logview.Page, error) {
	a.mu.Lock()
	remote, ctx := a.remote, a.ctx
	a.mu.Unlock()
	if remote == nil {
		return logview.Page{}, errors.New("connect to a server first")
	}
	return remote.ReadLogs(ctx, query)
}

func connect(ctx context.Context, s config.Server) (*client.Client, error) {
	if s.Certificate != "" {
		u, err := url.Parse(s.URL)
		if err != nil || u.Scheme != "https" {
			return nil, errors.New("certificate trust requires an HTTPS address")
		}
	}
	opts := []client.Option{client.WithHTTPClient(&http.Client{Timeout: 15 * time.Second})}
	if s.Certificate != "" {
		opts = append(opts, client.WithCertificate(s.Certificate))
	}
	remote, err := client.New(s.URL, s.Token, opts...)
	if err != nil {
		return nil, err
	}
	// Check the actual administration permission, not just general API access.
	if _, err := remote.ReadLogs(ctx, logview.Query{Limit: 1}); err != nil {
		return nil, err
	}
	return remote, nil
}
