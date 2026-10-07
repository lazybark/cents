package desktop

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lazybark/cents/client"
)

// Server is a cents server to connect to: its address, the token it gave
// this app, and the fingerprint of its certificate when that's trusted by
// fingerprint (one the system doesn't trust).
type Server struct {
	URL         string
	Token       string
	Certificate string
}

// ServerInput is a server as typed into the connect form. Certificate is
// the fingerprint the user agreed to trust, after a ServerResult asked.
type ServerInput struct {
	URL         string `json:"url"`
	Token       string `json:"token"`
	Certificate string `json:"certificate"`
}

// ServerResult says how connecting went: Connected, or Untrusted when the
// server's certificate isn't one the system trusts; Certificate is then its
// fingerprint, to compare with the one the server logs and to trust.
type ServerResult struct {
	Connected   bool   `json:"connected"`
	Untrusted   bool   `json:"untrusted"`
	Certificate string `json:"certificate"`
}

// Calls to a server time out after requestTimeout; checking one when
// connecting, sooner.
const (
	requestTimeout = 2 * time.Minute
	checkTimeout   = 15 * time.Second
)

// ConnectServer connects to a server instead of the database open, which is
// closed. The server is remembered, so the next launch connects to it too.
func (a *API) ConnectServer(input ServerInput) (ServerResult, error) {
	server := Server{URL: strings.TrimSpace(input.URL), Token: strings.TrimSpace(input.Token), Certificate: strings.TrimSpace(input.Certificate)}
	// Typed without a scheme, it's HTTPS.
	if !strings.Contains(server.URL, "://") && server.URL != "" {
		server.URL = "https://" + server.URL
	}

	server.URL = strings.TrimRight(server.URL, "/")

	remote, err := connect(server)
	if client.Untrusted(err) && server.Certificate == "" {
		fingerprint, ferr := client.Fingerprint(server.URL)
		if ferr != nil {
			return ServerResult{}, ferr
		}

		return ServerResult{Untrusted: true, Certificate: fingerprint}, nil
	}

	if err != nil {
		return ServerResult{}, err
	}

	if a.opts.SaveServer != nil {
		if err := a.opts.SaveServer(server); err != nil {
			return ServerResult{}, err
		}
	}

	a.useServer(server, remote)

	return ServerResult{Connected: true}, nil
}

// Reconnect connects to the server set to connect to (Status.Server), the
// one remembered from before: at launch, or again after it failed.
func (a *API) Reconnect() error {
	a.mu.Lock()
	server := a.server
	a.mu.Unlock()

	if server == nil {
		return errors.New("no server to connect to")
	}

	remote, err := connect(*server)
	if err != nil {
		a.mu.Lock()
		a.serverError = err.Error()
		a.mu.Unlock()

		return err
	}

	a.useServer(*server, remote)

	return nil
}

// connect makes a client of server and checks it answers, and takes the
// token.
func connect(server Server) (*client.Client, error) {
	if server.URL == "" {
		return nil, errors.New("type the server's address")
	}

	if server.Token == "" {
		return nil, errors.New("type the token the server gave this app")
	}

	if server.Certificate != "" {
		if link, err := url.Parse(server.URL); err == nil && link.Scheme != "https" {
			return nil, errors.New("a certificate can only be trusted for an https address")
		}
	}

	// Each client has an http.Client of its own, with its own timeout.
	newClient := func(timeout time.Duration) (*client.Client, error) {
		options := []client.Option{client.WithHTTPClient(&http.Client{Timeout: timeout})}
		if server.Certificate != "" {
			options = append(options, client.WithCertificate(server.Certificate))
		}

		return client.New(server.URL, server.Token, options...)
	}

	check, err := newClient(checkTimeout)
	if err != nil {
		return nil, err
	}

	if _, err := check.Info(); err != nil {
		return nil, err
	}

	remote, err := newClient(requestTimeout)
	if err != nil {
		return nil, err
	}

	return remote, nil
}

// useServer switches to remote, a client of server, closing the database
// open, if any.
func (a *API) useServer(server Server, remote *client.Client) {
	a.mu.Lock()
	previous := a.storage
	a.storage, a.dbPath, a.locked = nil, "", ""
	a.server, a.connected, a.serverError = &server, true, ""
	a.sw.To(remote, remote)
	a.mu.Unlock()

	if previous != nil {
		_ = previous.Close()
	}
}

// remote reports whether a server is in use.
func (a *API) remote() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.connected
}
