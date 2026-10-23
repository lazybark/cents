package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

// maxBody is the largest request body accepted.
const maxBody = 1 << 20

// request is what's known about a request, for its log line.
type request struct {
	client string
}

type requestKey struct{}

func info(r *http.Request) *request {
	if req, ok := r.Context().Value(requestKey{}).(*request); ok {
		return req
	}

	return &request{}
}

// statusWriter remembers the status written, for the log.
type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}

	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// remoteAddr is the connecting address. Headers like X-Forwarded-For are
// ignored: anyone can send them.
func remoteAddr(r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}

	return addr.Unmap()
}

// Only a configured proxy may assert an original client address. An invalid
// header fails closed (the address allowlist rejects an invalid address).
func clientAddr(r *http.Request, trusted []netip.Prefix) netip.Addr {
	peer := remoteAddr(r)
	if allowed(trusted, peer) && r.Header.Get("Fly-Client-IP") != "" {
		addr, _ := netip.ParseAddr(r.Header.Get("Fly-Client-IP"))
		return addr.Unmap()
	}
	return peer
}

type apiError struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, apiError{Error: errorBody{Code: code, Message: message}})
}

// guard wraps the API with what every request goes through, outermost
// first: recovering from a panic, logging, security headers and a body
// limit, the address allowlist, and (for /api/) the token.
func guard(s options, failures *throttle, next http.Handler) http.Handler {
	log := s.logger
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		req := &request{}
		r = r.WithContext(context.WithValue(r.Context(), requestKey{}, req))
		sw := &statusWriter{ResponseWriter: w}
		addr := clientAddr(r, s.trustedProxies)

		defer func() {
			if p := recover(); p != nil {
				log.Error("request failed", "panic", p, "method", r.Method, "path", r.URL.Path)
				if sw.status == 0 {
					writeError(sw, http.StatusInternalServerError, "internal", "something went wrong on the server")
				}
			}

			level := slog.LevelInfo
			if sw.status >= 500 {
				level = slog.LevelError
			} else if sw.status >= 400 {
				level = slog.LevelWarn
			}

			log.Log(r.Context(), level, "request", "method", r.Method, "path", r.URL.Path, "status", sw.status, "duration_ms", time.Since(started).Milliseconds(), "remote", addr.String(), "client", req.client, "bytes", sw.bytes)
		}()

		h := sw.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Cache-Control", "no-store")
		h.Set("Referrer-Policy", "no-referrer")
		limit := BodyLimit(r.Method, r.URL.Path)
		if limit > maxBody {
			deadline := time.Now().Add(2 * time.Minute)
			controller := http.NewResponseController(sw)
			_ = controller.SetReadDeadline(deadline)
			_ = controller.SetWriteDeadline(deadline)
		}
		r.Body = http.MaxBytesReader(sw, r.Body, limit)

		if !allowed(s.allow, addr) {
			writeError(sw, http.StatusForbidden, "forbidden", "this address isn't allowed")
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/") {
			if failures.blocked(addr) {
				sw.Header().Set("Retry-After", "60")
				writeError(sw, http.StatusTooManyRequests, "too_many_attempts", "too many failed attempts; try again later")
				return
			}

			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			c, valid := clientOf(s.tokens, strings.TrimSpace(token))
			if !ok || !valid {
				failures.fail(addr)
				sw.Header().Set("WWW-Authenticate", `Bearer realm="cents"`)
				writeError(sw, http.StatusUnauthorized, "unauthorized", "a valid token is required")
				return
			}

			req.client = c.name
			if strings.HasPrefix(r.URL.Path, apiPrefix+"/admin/") && !c.admin {
				writeError(sw, http.StatusForbidden, "admin_required", "this token does not have administrator permission")
				return
			}
			if c.readOnly && r.Method != http.MethodGet && r.Method != http.MethodHead {
				writeError(sw, http.StatusForbidden, "read_only", "this token can only read")
				return
			}
		}

		next.ServeHTTP(sw, r)
	})
}

func allowed(allow []netip.Prefix, addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}

	for _, prefix := range allow {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}
