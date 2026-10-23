package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/netip"
	"strings"
	"sync"
	"time"
)

// tokenPrefix marks cents tokens, so they're easy to spot (and to find if
// one ever leaks into a file).
const tokenPrefix = "cents_"

// NewToken makes a client token, returning it (shown once, given to the
// client) and its SHA-256 (kept in the server's config).
func NewToken() (token, hash string, err error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", "", err
	}

	token = tokenPrefix + base64.RawURLEncoding.EncodeToString(secret)
	return token, HashToken(token), nil
}

// HashToken is a token's SHA-256, in hex. Tokens are long random strings,
// so a plain hash is enough to keep them out of the config.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func parseHash(raw string) ([32]byte, error) {
	var hash [32]byte
	decoded, err := hex.DecodeString(strings.TrimSpace(raw))
	if err != nil || len(decoded) != len(hash) {
		return hash, errors.New("sha256 must be 64 hex characters")
	}

	copy(hash[:], decoded)
	return hash, nil
}

// client is who a token lets in.
type client struct {
	name     string
	readOnly bool
	admin    bool
}

// clientOf is the client of the token presented, if it's one of tokens.
// Hashing first means the lookup takes the same time whatever is sent.
func clientOf(tokens map[[32]byte]client, presented string) (client, bool) {
	c, ok := tokens[sha256.Sum256([]byte(presented))]
	return c, ok
}

// throttle slows down guessing: an address that fails to authenticate
// maxFailures times within window is turned away until the window passes.
type throttle struct {
	mu       sync.Mutex
	failures map[netip.Addr][]time.Time
	max      int
	window   time.Duration
	now      func() time.Time
}

func newThrottle(max int, window time.Duration) *throttle {
	return &throttle{failures: map[netip.Addr][]time.Time{}, max: max, window: window, now: time.Now}
}

// recent drops failures older than the window and returns what's left.
func (t *throttle) recent(addr netip.Addr) []time.Time {
	cutoff := t.now().Add(-t.window)
	kept := t.failures[addr][:0]
	for _, at := range t.failures[addr] {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}

	if len(kept) == 0 {
		delete(t.failures, addr)
	} else {
		t.failures[addr] = kept
	}

	return kept
}

// blocked reports whether addr has failed too often lately.
func (t *throttle) blocked(addr netip.Addr) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.recent(addr)) >= t.max
}

func (t *throttle) fail(addr netip.Addr) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.failures[addr] = append(t.recent(addr), t.now())
}
