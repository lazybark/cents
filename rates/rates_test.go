package rates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchFallsBackAndParsesBothFormats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/broken/currencies/eur.min.json":
			http.Error(w, "down", http.StatusBadGateway)
		case "/ok/currencies/eur.min.json":
			_, _ = w.Write([]byte(`{"date":"2026-10-02","eur":{"usd":1.25,"gel":3.2,"btc":0.00001}}`))
		case "/v6/latest/EUR":
			_, _ = w.Write([]byte(`{"result":"success","time_last_update_utc":"Fri, 02 Oct 2026 00:02:31 +0000","rates":{"EUR":1,"USD":1.2}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	source := func(name, root string) Source {
		return Source{Name: name, URL: func(base string) string { return currencyAPIURL(server.URL+root, base) }, Parse: parseCurrencyAPI}
	}
	er := Source{Name: "er", URL: func(base string) string { return server.URL + "/v6/latest/" + base }, Parse: parseOpenER}

	fetcher := HTTP{Client: server.Client(), Sources: []Source{source("broken", "/broken"), source("ok", "/ok"), er}}
	quote, err := fetcher.Fetch(context.Background(), "eur")
	if err != nil || quote.Source != "ok" || quote.Base != "EUR" || quote.Date != "2026-10-02" {
		t.Fatalf("expected the second source, got %+v %v", quote, err)
	}

	if rate, ok := quote.RateToBase("usd"); !ok || rate != 0.8 {
		t.Fatalf("1 USD should be 0.8 EUR, got %v %v", rate, ok)
	}

	if rate, ok := quote.RateToBase("EUR"); !ok || rate != 1 {
		t.Fatalf("the base is 1, got %v", rate)
	}

	if _, ok := quote.RateToBase("XXX"); ok {
		t.Fatal("unknown currency should have no rate")
	}

	fetcher.Sources = []Source{source("broken", "/broken"), er}
	quote, err = fetcher.Fetch(context.Background(), "EUR")
	if err != nil || quote.Source != "er" || quote.Date != "2026-10-02" {
		t.Fatalf("expected the fallback, got %+v %v", quote, err)
	}

	fetcher.Sources = []Source{source("broken", "/broken")}
	if _, err := fetcher.Fetch(context.Background(), "EUR"); err == nil || !strings.Contains(err.Error(), "broken: status 502") {
		t.Fatalf("expected every source's error, got %v", err)
	}

	if _, err := fetcher.Fetch(context.Background(), " "); err == nil {
		t.Fatal("expected an error without a base")
	}
}
