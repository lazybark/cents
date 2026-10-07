// Package client calls a cents server. Client implements service.Backend
// and service.Files, so code written against those works the same on a
// server as on a local database (a service.Service).
package client

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/lazybark/cents/service"
)

// Client calls the server at one address with one token.
type Client struct {
	base  *url.URL
	token string
	http  *http.Client
}

var (
	_ service.Backend = (*Client)(nil)
	_ service.Files   = (*Client)(nil)
)

// Option changes how a client calls the server.
type Option func(*Client)

// WithHTTPClient makes requests with c: for a certificate the system
// doesn't trust, say, or another timeout.
func WithHTTPClient(c *http.Client) Option {
	return func(client *Client) { client.http = c }
}

// New is a client of the server at address (like https://cents.lan:8740)
// with token.
func New(address, token string, options ...Option) (*Client, error) {
	base, err := url.Parse(strings.TrimRight(strings.TrimSpace(address), "/"))
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return nil, fmt.Errorf("the server's address %q must be like https://host:port", address)
	}

	if strings.TrimSpace(token) == "" {
		return nil, errors.New("a token is required")
	}

	c := &Client{base: base, token: strings.TrimSpace(token), http: &http.Client{Timeout: time.Minute}}
	for _, option := range options {
		option(c)
	}

	return c, nil
}

// Error is the server's answer to a call it refused or failed. Its message
// is the same a local service gives, and it matches service.ErrNotFound,
// service.ErrUpstream and service.ErrNoDatabase with errors.Is like the
// service's errors do.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func (e *Error) Is(target error) bool {
	switch target {
	case service.ErrNotFound:
		return e.Code == "not_found"
	case service.ErrUpstream:
		return e.Code == "rates_unavailable"
	case service.ErrNoDatabase:
		return e.Code == "no_database"
	}

	return false
}

// call sends a request to path (under /api/v1) with in as its JSON body
// (nil for none), and reads the JSON result into out (nil for none).
func (c *Client) call(method, path string, query url.Values, in, out any) error {
	res, err := c.send(method, path, query, in)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if out == nil {
		_, _ = io.Copy(io.Discard, res.Body)
		return nil
	}

	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("the server's answer to %s %s isn't valid: %w", method, path, err)
	}

	return nil
}

// send makes a request, returning a successful response (whose body the
// caller closes) or the error answered.
func (c *Client) send(method, path string, query url.Values, in any) (*http.Response, error) {
	target := *c.base
	target.Path += "/api/v1" + path
	target.RawQuery = query.Encode()

	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}

		body = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, target.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := c.http.Do(req)
	if err != nil {
		// The request itself (Get "https://…") says nothing new.
		var failed *url.Error
		if errors.As(err, &failed) {
			err = failed.Err
		}

		return nil, fmt.Errorf("couldn't reach the server: %w", err)
	}

	if res.StatusCode < 300 {
		return res, nil
	}

	defer res.Body.Close()
	var answer struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&answer); err != nil || answer.Error.Message == "" {
		return nil, &Error{Status: res.StatusCode, Code: "unknown", Message: fmt.Sprintf("the server answered %s", res.Status)}
	}

	return nil, &Error{Status: res.StatusCode, Code: answer.Error.Code, Message: answer.Error.Message}
}

func (c *Client) get(path string, query url.Values, out any) error {
	return c.call(http.MethodGet, path, query, nil, out)
}

// at is path with its {} filled with ids and other values, escaped.
func at(path string, values ...any) string {
	for _, value := range values {
		i := strings.Index(path, "{")
		j := strings.Index(path, "}")
		path = path[:i] + url.PathEscape(fmt.Sprint(value)) + path[j+1:]
	}

	return path
}

// values is a query of name, value pairs; empty values are left out.
func values(pairs ...any) url.Values {
	query := url.Values{}
	for i := 0; i+1 < len(pairs); i += 2 {
		if value := fmt.Sprint(pairs[i+1]); value != "" {
			query.Set(pairs[i].(string), value)
		}
	}

	return query
}

// fields is a query of the string fields of the struct v, by their JSON
// names.
func fields(v any) url.Values {
	query := url.Values{}
	value := reflect.ValueOf(v)
	for i := 0; i < value.NumField(); i++ {
		f := value.Type().Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name != "" && name != "-" && f.Type.Kind() == reflect.String && value.Field(i).String() != "" {
			query.Set(name, value.Field(i).String())
		}
	}

	return query
}

type changed struct {
	Changed int `json:"changed"`
}

type mergeInto struct {
	IntoID uint `json:"intoId"`
}

// settingLists are the paths of the setting kinds.
var settingLists = map[string]string{
	"currency":         "currencies",
	"payment_method":   "payment-methods",
	"tax_type":         "tax-types",
	"income_category":  "income-categories",
	"expense_category": "expense-categories",
	"tag":              "tags",
}

// WriteBackup downloads a copy of the whole database into w; savedTo is
// remembered as where the last backup is kept.
func (c *Client) WriteBackup(w io.Writer, savedTo string) (service.BackupResult, error) {
	res, err := c.send(http.MethodPost, "/backup", nil, struct {
		SavedTo string `json:"savedTo"`
	}{savedTo})
	if err != nil {
		return service.BackupResult{}, err
	}
	defer res.Body.Close()

	if _, err := io.Copy(w, res.Body); err != nil {
		return service.BackupResult{}, fmt.Errorf("failed to download the backup: %w", err)
	}

	return service.BackupResult{Path: strings.TrimSpace(savedTo), At: res.Header.Get("Cents-Backup-At")}, nil
}

// WriteCSV downloads a dataset as CSV into w.
func (c *Client) WriteCSV(w io.Writer, dataset, from, to string) (int, error) {
	res, err := c.send(http.MethodGet, at("/export/{dataset}", dataset), values("from", from, "to", to), nil)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	if _, err := io.Copy(w, res.Body); err != nil {
		return 0, fmt.Errorf("failed to download %s: %w", dataset, err)
	}

	rows, _ := strconv.Atoi(res.Header.Get("Cents-Rows"))
	return rows, nil
}

// WithCertificate trusts the server's certificate by its SHA-256
// fingerprint (see Fingerprint) instead of by the system's authorities:
// for a server with a certificate it made itself. Any other certificate is
// refused, even one the system would trust.
func WithCertificate(fingerprint string) Option {
	return func(c *Client) {
		want := normalFingerprint(fingerprint)
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			// The certificate is checked below, by its fingerprint, instead.
			InsecureSkipVerify: true, //nolint:gosec
			VerifyPeerCertificate: func(raw [][]byte, _ [][]*x509.Certificate) error {
				if len(raw) == 0 {
					return errors.New("the server sent no certificate")
				}

				if got := certificateFingerprint(raw[0]); normalFingerprint(got) != want {
					return &CertificateChangedError{Fingerprint: got}
				}

				return nil
			},
		}
		c.http.Transport = transport
	}
}

// CertificateChangedError is a server's certificate other than the one
// trusted: if the server didn't get a new one, someone may be in between.
type CertificateChangedError struct {
	// Fingerprint is the certificate the server sent.
	Fingerprint string
}

func (e *CertificateChangedError) Error() string {
	return "the server's certificate isn't the one trusted before (it's " + e.Fingerprint + "); if the server has a new one, connect again to trust it"
}

// Untrusted reports whether err is a server's certificate the system
// doesn't trust (one the server made itself, say): it can be trusted by its
// fingerprint instead, with WithCertificate.
func Untrusted(err error) bool {
	var unknown x509.UnknownAuthorityError
	var invalid x509.CertificateInvalidError
	var hostname x509.HostnameError
	return errors.As(err, &unknown) || errors.As(err, &invalid) || errors.As(err, &hostname)
}

// Fingerprint is the SHA-256 fingerprint of the certificate the server at
// address presents, like AB:CD:…, for the user to compare with the one the
// server logs when it starts.
func Fingerprint(address string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(address))
	if err != nil || base.Scheme != "https" || base.Host == "" {
		return "", fmt.Errorf("%q isn't an https address", address)
	}

	host := base.Host
	if base.Port() == "" {
		host = net.JoinHostPort(base.Hostname(), "443")
	}

	dialer := &tls.Dialer{NetDialer: &net.Dialer{Timeout: 10 * time.Second}, Config: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}} //nolint:gosec // only to read the certificate
	conn, err := dialer.Dial("tcp", host)
	if err != nil {
		return "", fmt.Errorf("couldn't reach the server: %w", err)
	}
	defer conn.Close()

	certificates := conn.(*tls.Conn).ConnectionState().PeerCertificates
	if len(certificates) == 0 {
		return "", errors.New("the server sent no certificate")
	}

	return certificateFingerprint(certificates[0].Raw), nil
}

// certificateFingerprint is a certificate's SHA-256 as AB:CD:… (the way
// openssl and browsers show it).
func certificateFingerprint(der []byte) string {
	sum := sha256.Sum256(der)
	parts := make([]string, len(sum))
	for i, b := range sum {
		parts[i] = fmt.Sprintf("%02X", b)
	}

	return strings.Join(parts, ":")
}

// normalFingerprint makes fingerprints comparable however they're written.
func normalFingerprint(fingerprint string) string {
	return strings.ToUpper(strings.NewReplacer(":", "", " ", "").Replace(strings.TrimSpace(fingerprint)))
}
