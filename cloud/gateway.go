// Package cloud runs cents' API, Telegram bot and public gateway together.
package cloud

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lazybark/cents/server"
)

func peerAddr(remote string) netip.Addr {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	addr, _ := netip.ParseAddr(host)
	return addr.Unmap()
}

func originalAddr(r *http.Request, proxies []netip.Prefix) netip.Addr {
	peer := peerAddr(r.RemoteAddr)
	for _, prefix := range proxies {
		if prefix.Contains(peer) && r.Header.Get("Fly-Client-IP") != "" {
			addr, _ := netip.ParseAddr(r.Header.Get("Fly-Client-IP"))
			return addr.Unmap()
		}
	}
	return peer
}

func proxyTo(target *url.URL, proxies []netip.Prefix, log *slog.Logger) http.Handler {
	return &httputil.ReverseProxy{
		Rewrite: func(p *httputil.ProxyRequest) {
			p.SetURL(target)
			p.Out.Header.Del("Fly-Client-IP")
			if addr := originalAddr(p.In, proxies); addr.IsValid() {
				p.Out.Header.Set("Fly-Client-IP", addr.String())
			} else {
				// A malformed assertion from a trusted proxy must not silently
				// become the trusted loopback address at the API.
				p.Out.Header.Set("Fly-Client-IP", "invalid")
			}
		},
		Transport: &http.Transport{
			Proxy:                 nil, // Loopback traffic must never follow HTTP_PROXY.
			DialContext:           (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			ResponseHeaderTimeout: 2 * time.Minute,
			IdleConnTimeout:       60 * time.Second,
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				http.Error(w, "database upload exceeds the size limit", http.StatusRequestEntityTooLarge)
				return
			}
			log.Error("upstream unavailable", "request_id", r.Header.Get("X-Request-ID"))
			http.Error(w, "service temporarily unavailable", http.StatusBadGateway)
		},
	}
}

type response struct {
	http.ResponseWriter
	status, bytes int
}

func (w *response) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *response) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}
func (w *response) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func gateway(api, mini *url.URL, proxies []netip.Prefix, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/", proxyTo(api, proxies, log))
	mux.Handle("/app/", http.StripPrefix("/app", proxyTo(mini, proxies, log)))
	redirect := func(w http.ResponseWriter, r *http.Request) {
		u := *r.URL
		u.Path, u.RawPath = "/app/", ""
		http.Redirect(w, r, u.RequestURI(), http.StatusPermanentRedirect)
	}
	mux.HandleFunc("/app", redirect)
	mux.HandleFunc("/{$}", redirect)
	probeClient := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := probe(r.Context(), probeClient, api.String()+"/healthz"); err != nil {
			http.Error(w, "API not ready", http.StatusServiceUnavailable)
			return
		}
		if err := probe(r.Context(), probeClient, mini.String()+"/"); err != nil {
			http.Error(w, "Mini App not ready", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		id := uuid.NewString()
		// Do not accept a caller's identifier (or log their headers/body/query).
		r.Header.Set("X-Request-ID", id)
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		sw := &response{ResponseWriter: w}
		defer func() {
			if p := recover(); p != nil {
				if p == http.ErrAbortHandler {
					panic(p)
				}
				// Panic values may contain financial data; omit them.
				log.Error("gateway request panicked", "request_id", id)
				if sw.status == 0 {
					http.Error(sw, "internal server error", http.StatusInternalServerError)
				}
			}
			status := sw.status
			if status == 0 {
				status = http.StatusOK
			}
			level := slog.LevelInfo
			if status >= 500 {
				level = slog.LevelError
			} else if status >= 400 {
				level = slog.LevelWarn
			}
			// Health probes are useful when failing, noisy when successful.
			if r.URL.Path != "/healthz" || status >= 400 {
				log.Log(r.Context(), level, "request", "request_id", id, "method", r.Method, "route", routeName(r.URL.Path), "status", status, "duration_ms", time.Since(started).Milliseconds(), "bytes", sw.bytes, "remote", originalAddr(r, proxies).String())
			}
		}()
		limit := server.BodyLimit(r.Method, r.URL.Path)
		if limit > 1<<20 {
			if r.ContentLength > limit {
				http.Error(sw, "database upload exceeds the size limit", http.StatusRequestEntityTooLarge)
				return
			}
			deadline := time.Now().Add(2 * time.Minute)
			controller := http.NewResponseController(sw)
			_ = controller.SetReadDeadline(deadline)
			_ = controller.SetWriteDeadline(deadline)
		}
		r.Body = http.MaxBytesReader(sw, r.Body, limit)
		mux.ServeHTTP(sw, r)
	})
}

func routeName(path string) string {
	switch {
	case path == "/healthz":
		return "health"
	case strings.HasPrefix(path, "/app/api/"):
		return "mini_app_api"
	case strings.HasPrefix(path, "/app/"):
		return "mini_app"
	case strings.HasPrefix(path, "/api/"):
		return "desktop_api"
	default:
		return "other"
	}
}

func probe(ctx context.Context, client *http.Client, address string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", res.StatusCode)
	}
	return nil
}

func serveGateway(ctx context.Context, listener net.Listener, h http.Handler, log *slog.Logger) error {
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 65 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10, ErrorLog: slog.NewLogLogger(log.Handler(), slog.LevelWarn)}
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdown); err != nil {
			_ = srv.Close()
		}
	}()
	err := srv.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		<-stopped
		return nil
	}
	return err
}
