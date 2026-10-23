package server

import (
	"net/http/httptest"
	"testing"
)

func TestTrustedProxyAllowlistAndIndependentThrottling(t *testing.T) {
	h, token, _ := testServer(t, Config{Allow: []string{"198.51.100.0/24"}, TrustedProxies: []string{"127.0.0.1"}})
	call := func(peer, original, bearer string) int {
		r := httptest.NewRequest("GET", "/api/v1/info", nil)
		r.RemoteAddr = peer
		r.Header.Set("Fly-Client-IP", original)
		r.Header.Set("Authorization", "Bearer "+bearer)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if got := call("127.0.0.1:1234", "198.51.100.1", token); got != 200 {
		t.Fatalf("proxied client: %d", got)
	}
	if got := call("203.0.113.1:1234", "198.51.100.1", token); got != 403 {
		t.Fatalf("spoofed proxy: %d", got)
	}
	if got := call("127.0.0.1:1234", "not-an-ip", token); got != 403 {
		t.Fatalf("invalid assertion: %d", got)
	}
	for range throttleFailures {
		call("127.0.0.1:1234", "198.51.100.1", "bad-token")
	}
	if got := call("127.0.0.1:1234", "198.51.100.1", token); got != 429 {
		t.Fatalf("blocked client: %d", got)
	}
	if got := call("127.0.0.1:1234", "198.51.100.2", token); got != 200 {
		t.Fatalf("another client behind the same proxy: %d", got)
	}
}
