package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lazybark/cents/dbimport"
)

func TestImportAuthenticationAndInputErrors(t *testing.T) {
	h, token, _ := testServer(t, Config{})
	for _, tc := range []struct {
		auth, contentType, path string
		want                    int
	}{
		{"", "application/vnd.sqlite3", "/api/v1/import", 401},
		{"Bearer " + token, "application/json", "/api/v1/import", 400},
		{"Bearer " + token, "application/vnd.sqlite3", "/api/v1/import?preview=wrong", 400},
		{"Bearer " + token, "application/vnd.sqlite3", "/api/v1/import?preview=true", 422},
	} {
		r := httptest.NewRequest("POST", tc.path, strings.NewReader("not a SQLite file"))
		r.RemoteAddr = "127.0.0.1:1234"
		r.Header.Set("Authorization", tc.auth)
		r.Header.Set("Content-Type", tc.contentType)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s %s: %d %s", tc.path, tc.auth, w.Code, w.Body.String())
		}
	}
	readerToken, tok := testToken(t)
	tok.ReadOnly = true
	tok.Name = "reader"
	h, _, _ = testServer(t, Config{Tokens: []Token{tok}})
	r := httptest.NewRequest("POST", "/api/v1/import?preview=false", strings.NewReader("not a database"))
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("Authorization", "Bearer "+readerToken)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("read-only import: %d", w.Code)
	}
	if BodyLimit("POST", "/api/v1/import") != dbimport.MaxBytes || BodyLimit("PUT", "/api/v1/import") != maxBody || BodyLimit("POST", "/api/v1/cashflow/entries") != maxBody {
		t.Fatal("incorrect upload body limits")
	}
}
