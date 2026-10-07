package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// apiCall calls h as 127.0.0.1 with token and a JSON body ("" for none).
func apiCall(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}

	r.RemoteAddr = "127.0.0.1:5000"
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	return w
}

func decoded(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("not JSON: %s", w.Body)
	}

	return body
}

func errorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()

	code, _ := decoded(t, w)["error"].(map[string]any)["code"].(string)
	return code
}

func TestEndpoints(t *testing.T) {
	h, token, _ := testServer(t, Config{})

	w := apiCall(h, "GET", "/api/v1/info", token, "")
	if info := decoded(t, w); w.Code != 200 || info["baseCurrency"] != "$" || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("unexpected info %d %v", w.Code, info)
	}

	w = apiCall(h, "GET", "/api/v1/accounts", token, "")
	accounts := decoded(t, w)["accounts"].([]any)
	if bank := accounts[0].(map[string]any); w.Code != 200 || len(accounts) != 1 || bank["name"] != "Bank" || bank["baseCents"] != 11000.0 {
		t.Fatalf("unexpected accounts %d %s", w.Code, w.Body)
	}

	w = apiCall(h, "GET", "/api/v1/cashflow/months/current", token, "")
	if month := decoded(t, w); w.Code != 200 || month["month"] != time.Now().Format("2006-01") {
		t.Fatalf("expected the running month: %d %s", w.Code, w.Body)
	}

	w = apiCall(h, "GET", "/api/v1/cashflow/months/2026-09", token, "")
	if month := decoded(t, w); w.Code != 200 || len(month["entries"].([]any)) != 2 {
		t.Fatalf("unexpected month %d %s", w.Code, w.Body)
	}

	// A change: 201 for something new, 204 for no result.
	if w := apiCall(h, "POST", "/api/v1/accounts", token, `{"name":"Cash","description":"wallet","currency":"$","amount":"20"}`); w.Code != 201 || w.Body.Len() != 0 {
		t.Fatalf("expected created: %d %s", w.Code, w.Body)
	}

	if w := apiCall(h, "PUT", "/api/v1/notes/2026-09", token, `{"text":"trip"}`); w.Code != 204 {
		t.Fatalf("expected no content: %d %s", w.Code, w.Body)
	}

	if notes := decoded(t, apiCall(h, "GET", "/api/v1/notes", token, "")); notes["2026-09"] != "trip" {
		t.Fatalf("expected the note: %v", notes)
	}

	// The search reads its filters from the query.
	w = apiCall(h, "GET", "/api/v1/cashflow/entries?text=lunch&tag=work", token, "")
	if found := decoded(t, w); w.Code != 200 || len(found["entries"].([]any)) != 1 {
		t.Fatalf("unexpected search %d %s", w.Code, w.Body)
	}

	for name, c := range map[string]struct {
		method, path, body string
		status             int
		code               string
	}{
		"value to fix":        {"POST", "/api/v1/accounts", `{"name":"x","description":"y","currency":"$","amount":"lots"}`, 422, "invalid"},
		"unknown field":       {"POST", "/api/v1/accounts", `{"nmae":"x"}`, 400, "bad_request"},
		"no body":             {"POST", "/api/v1/accounts", "", 400, "bad_request"},
		"two bodies":          {"PUT", "/api/v1/rates/auto", `{"on":false}{"on":true}`, 400, "bad_request"},
		"bad id":              {"DELETE", "/api/v1/accounts/abc", "", 400, "bad_request"},
		"zero id":             {"DELETE", "/api/v1/accounts/0", "", 400, "bad_request"},
		"bad number":          {"GET", "/api/v1/analytics?months=many", "", 400, "bad_request"},
		"bad flag":            {"GET", "/api/v1/cashflow/months?includeArchived=maybe", "", 400, "bad_request"},
		"missing record":      {"DELETE", "/api/v1/accounts/99", "", 404, "not_found"},
		"missing log":         {"DELETE", "/api/v1/accounts/1/values/99", "", 404, "not_found"},
		"unknown list":        {"DELETE", "/api/v1/settings/colours/1", "", 404, "not_found"},
		"unknown dataset":     {"GET", "/api/v1/export/nope", "", 404, "not_found"},
		"in use":              {"DELETE", "/api/v1/settings/expense-categories/1", "", 409, "in_use"},
		"rates source down":   {"POST", "/api/v1/rates/refresh", "", 422, "invalid"},
		"no such endpoint":    {"GET", "/api/v1/nope", "", 404, "not_found"},
		"wrong method":        {"PATCH", "/api/v1/accounts/1", `{}`, 405, "method_not_allowed"},
		"wrong method, list":  {"DELETE", "/api/v1/accounts", "", 405, "method_not_allowed"},
		"wrong method, merge": {"GET", "/api/v1/settings/tags/1/merge", "", 405, "method_not_allowed"},
	} {
		w := apiCall(h, c.method, c.path, token, c.body)
		if w.Code != c.status || errorCode(t, w) != c.code {
			t.Errorf("%s: expected %d %s, got %d %s", name, c.status, c.code, w.Code, w.Body)
		}
	}

	if w := apiCall(h, "PATCH", "/api/v1/accounts/1", token, `{}`); w.Header().Get("Allow") != "PUT, DELETE" {
		t.Fatalf("expected the methods allowed: %v", w.Header())
	}

	r := httptest.NewRequest("POST", "/api/v1/accounts", strings.NewReader("name=x"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Authorization", "Bearer "+token)
	r.RemoteAddr = "127.0.0.1:5000"
	if w := httptest.NewRecorder(); func() bool { h.ServeHTTP(w, r); return w.Code != 400 }() {
		t.Fatalf("expected a form body refused: %d", w.Code)
	}
}

func TestDownloads(t *testing.T) {
	h, token, _ := testServer(t, Config{})

	w := apiCall(h, "GET", "/api/v1/export/incomes_expenses?from=2026-09-04", token, "")
	if w.Code != 200 || w.Header().Get("Content-Type") != "text/csv; charset=utf-8" || w.Header().Get("Cents-Rows") != "1" || !strings.Contains(w.Header().Get("Content-Disposition"), "incomes_expenses.csv") {
		t.Fatalf("unexpected CSV %d %v", w.Code, w.Header())
	}

	if w := apiCall(h, "GET", "/api/v1/export/debts?from=yesterday", token, ""); w.Code != 422 {
		t.Fatalf("expected a bad date refused: %d %s", w.Code, w.Body)
	}

	w = apiCall(h, "POST", "/api/v1/backup", token, "")
	if w.Code != 200 || !strings.HasPrefix(w.Body.String(), "SQLite format 3") || w.Header().Get("Cents-Backup-At") == "" {
		t.Fatalf("unexpected backup %d %v", w.Code, w.Header())
	}

	if export := decoded(t, apiCall(h, "GET", "/api/v1/export", token, "")); export["lastBackupPath"] != "" {
		t.Fatalf("a backup without savedTo isn't remembered: %v", export)
	}

	if w := apiCall(h, "POST", "/api/v1/backup", token, `{"savedTo":"/backups/cents.db"}`); w.Code != 200 {
		t.Fatalf("unexpected backup %d", w.Code)
	}

	if export := decoded(t, apiCall(h, "GET", "/api/v1/export", token, "")); export["lastBackupPath"] != "/backups/cents.db" {
		t.Fatalf("expected the backup remembered: %v", export)
	}
}

func TestReadOnlyTokens(t *testing.T) {
	value, tok := testToken(t)
	tok.Name, tok.ReadOnly = "dashboard", true
	h, token, _ := testServer(t, Config{Tokens: []Token{tok}})

	if w := apiCall(h, "PUT", "/api/v1/notes/2026-09", token, `{"text":"x"}`); w.Code != 204 {
		t.Fatalf("a full token changes: %d %s", w.Code, w.Body)
	}

	if w := apiCall(h, "GET", "/api/v1/overview", value, ""); w.Code != 200 {
		t.Fatalf("a read-only token reads: %d %s", w.Code, w.Body)
	}

	if w := apiCall(h, "PUT", "/api/v1/notes/2026-09", value, `{"text":"x"}`); w.Code != 403 || errorCode(t, w) != "read_only" {
		t.Fatalf("expected the change refused: %d %s", w.Code, w.Body)
	}
}

func TestDescription(t *testing.T) {
	h, token, _ := testServer(t, Config{})

	w := apiCall(h, "GET", "/api/v1", token, "")
	doc := decoded(t, w)
	if w.Code != 200 || doc["openapi"] != "3.0.3" {
		t.Fatalf("unexpected %d %v", w.Code, doc)
	}

	paths := doc["paths"].(map[string]any)
	debt := paths["/api/v1/debts/{id}/payments"].(map[string]any)
	post := debt["post"].(map[string]any)
	if post["responses"].(map[string]any)["201"] == nil || post["requestBody"] == nil || len(post["parameters"].([]any)) != 1 {
		t.Fatalf("unexpected operation %v", post)
	}

	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	update := schemas["CashflowUpdateInput"]
	if update == nil {
		// Updates take the new entry's fields; the id is in the path.
		update = schemas["NewCashflowInput"]
	}

	if update.(map[string]any)["properties"].(map[string]any)["amount"] == nil || schemas["DebtRow"] == nil {
		t.Fatalf("expected the schemas: %v", schemas["NewCashflowInput"])
	}

	// Every operation has a name of its own.
	names := map[string]bool{}
	for _, item := range paths {
		for _, op := range item.(map[string]any) {
			name, _ := op.(map[string]any)["operationId"].(string)
			if name == "" || names[name] {
				t.Fatalf("expected a name of its own: %q", name)
			}

			names[name] = true
		}
	}

	// Every route is described, once per method.
	described := 0
	for _, item := range paths {
		described += len(item.(map[string]any))
	}

	if described != len(apiRoutes(nil, nil))+1 {
		t.Fatalf("expected every route described: %d of %d", described, len(apiRoutes(nil, nil))+1)
	}
}
