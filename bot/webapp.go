package bot

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lazybark/cents/server"
)

// WebApp is the Mini App: a page opened in Telegram from the bot's menu
// button. The bot serves it, with the cents API behind Telegram's sign-in.
type WebApp struct {
	// URL is where Telegram opens it: the public HTTPS address of a tunnel
	// (or proxy) to Listen. Empty turns it off.
	URL string `yaml:"url"`
	// Listen is the address the bot serves it on, for the tunnel to reach.
	Listen string `yaml:"listen"`
	// Button is the menu button's label.
	Button string `yaml:"button"`
}

// DefaultWebAppListen is where the Mini App is served unless told
// otherwise: this machine only, for a tunnel on it.
const DefaultWebAppListen = "127.0.0.1:8750"

// signInFor is how long a Mini App's sign-in lasts; reopening it signs in
// again.
const signInFor = 24 * time.Hour

func (w WebApp) on() bool { return strings.TrimSpace(w.URL) != "" }

func (w WebApp) check() error {
	if !w.on() {
		return nil
	}

	link, err := url.Parse(strings.TrimSpace(w.URL))
	if err != nil || link.Scheme != "https" || link.Host == "" {
		return errors.New("web_app.url: Telegram opens Mini Apps only from https addresses, like https://cents.example.com")
	}

	if _, _, err := net.SplitHostPort(w.listen()); err != nil {
		return fmt.Errorf("web_app.listen: %w", err)
	}

	return nil
}

func (w WebApp) listen() string {
	if strings.TrimSpace(w.Listen) == "" {
		return DefaultWebAppListen
	}

	return strings.TrimSpace(w.Listen)
}

func (w WebApp) button() string {
	if strings.TrimSpace(w.Button) == "" {
		return "cents"
	}

	return strings.TrimSpace(w.Button)
}

//go:embed webapp
var webappFiles embed.FS

// errSignIn is a request without a valid Telegram sign-in.
var errSignIn = errors.New("open the app from the bot in Telegram")

// signedInUser checks initData, the sign-in Telegram gives a Mini App: its
// hash must be the bot token's (HMAC-SHA256, as Telegram documents), and
// it must be recent. It returns the Telegram user it's for.
func signedInUser(initData, botToken string, now time.Time) (int64, error) {
	values, err := url.ParseQuery(initData)
	if err != nil || values.Get("hash") == "" {
		return 0, errSignIn
	}

	var lines []string
	for key := range values {
		if key != "hash" {
			lines = append(lines, key+"="+values.Get(key))
		}
	}

	sort.Strings(lines)
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(botToken))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(lines, "\n")))

	got, err := hex.DecodeString(values.Get("hash"))
	if err != nil || !hmac.Equal(got, mac.Sum(nil)) {
		return 0, errSignIn
	}

	signed, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil || now.Sub(time.Unix(signed, 0)) > signInFor {
		return 0, errors.New("the sign-in has expired: close the app and open it again")
	}

	var user struct {
		ID int64 `json:"id"`
	}

	if err := json.Unmarshal([]byte(values.Get("user")), &user); err != nil || user.ID == 0 {
		return 0, errSignIn
	}

	return user.ID, nil
}

// webAppHandler serves the page, and the cents API (under /api/) to the
// allowed users signed in by Telegram.
func (bt *Bot) webAppHandler(data Data, botToken string) http.Handler {
	page, _ := fs.Sub(webappFiles, "webapp")
	api := server.Routes(data, data, bt.log)

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(page)))
	mux.Handle("/api/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		initData, ok := strings.CutPrefix(r.Header.Get("Authorization"), "tma ")
		user, err := signedInUser(initData, botToken, bt.now())
		switch {
		case !ok || err != nil:
			if err == nil {
				err = errSignIn
			}

			writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
			return
		case !bt.allowed[user]:
			bt.log.Warn("mini app opened by a user not allowed", "user", user)
			writeError(w, http.StatusForbidden, "forbidden", fmt.Sprintf("This app is private. Your Telegram user ID is %d: its owner can allow it in the bot's config.", user))
			return
		}

		api.ServeHTTP(w, r)
	}))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		sw := &webResponse{ResponseWriter: w}
		defer func() {
			panicked := false
			if p := recover(); p != nil {
				if p == http.ErrAbortHandler {
					panic(p)
				}
				panicked = true
				if sw.status == 0 {
					writeError(sw, http.StatusInternalServerError, "internal", "something went wrong on the server")
				}
			}
			status := sw.status
			if status == 0 {
				status = http.StatusOK
			}
			client := "mini_app"
			isAPI := strings.HasPrefix(r.URL.Path, "/api/")
			if isAPI {
				client = "mini_app_api"
			}
			// Successful static requests include gateway health probes. The
			// gateway records assets; log API activity and failures here.
			if !isAPI && status < 400 && !panicked {
				return
			}
			level, message := slog.LevelInfo, "request"
			if status >= 500 || panicked {
				level = slog.LevelError
			} else if status >= 400 {
				level = slog.LevelWarn
			}
			if panicked {
				message = "request panicked"
			}
			bt.log.Log(r.Context(), level, message, "client", client, "method", r.Method, "path", r.URL.Path, "status", status, "duration_ms", time.Since(started).Milliseconds(), "bytes", sw.bytes)
		}()
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		r.Body = http.MaxBytesReader(sw, r.Body, 1<<20)
		mux.ServeHTTP(sw, r)
	})
}

type webResponse struct {
	http.ResponseWriter
	status, bytes int
}

func (w *webResponse) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *webResponse) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}

func (w *webResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
}

// serveWebApp serves the Mini App at w.Listen until ctx ends.
func (bt *Bot) serveWebApp(ctx context.Context, w WebApp, data Data, botToken string) error {
	listener, err := net.Listen("tcp", w.listen())
	if err != nil {
		return fmt.Errorf("web_app: listening on %s: %w", w.listen(), err)
	}

	srv := &http.Server{
		Handler:           bt.webAppHandler(data, botToken),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(bt.log.Handler(), slog.LevelWarn),
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdown); err != nil {
			_ = srv.Close()
		}
	}()

	bt.log.Info("mini app served", "listen", listener.Addr().String(), "url", w.URL)
	if err := srv.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	<-stopped

	return nil
}
