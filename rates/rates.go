// Package rates fetches exchange rates from free sources that need no key,
// trying each in turn, and works out what they mean for the settings.
package rates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Quote is how many units of each currency (by upper-case code) one unit
// of Base buys, as of Date, from Source.
type Quote struct {
	Base    string
	Date    string
	Source  string
	PerBase map[string]float64
}

// RateToBase is the value of one unit of code in the base currency, like
// the settings keep it.
func (q Quote) RateToBase(code string) (float64, bool) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == q.Base {
		return 1, true
	}

	perBase, ok := q.PerBase[code]
	if !ok || perBase <= 0 {
		return 0, false
	}

	return 1 / perBase, true
}

// Source is one place to get rates from.
type Source struct {
	Name  string
	URL   func(base string) string
	Parse func(body []byte, base string) (Quote, error)
}

// Sources are tried in order. The currency-api
// (github.com/fawazahmed0/exchange-api) covers about 340 currencies, crypto
// included, from two hosts: the pages.dev one updates as soon as new rates
// are out, while jsDelivr's cache can lag a day. open.er-api.com covers the
// 160-odd ordinary currencies.
var Sources = []Source{
	{
		Name:  "currency-api",
		URL:   func(base string) string { return currencyAPIURL("https://latest.currency-api.pages.dev/v1", base) },
		Parse: parseCurrencyAPI,
	},
	{
		Name: "currency-api (jsDelivr)",
		URL: func(base string) string {
			return currencyAPIURL("https://cdn.jsdelivr.net/npm/@fawazahmed0/currency-api@latest/v1", base)
		},
		Parse: parseCurrencyAPI,
	},
	{
		Name:  "open.er-api.com",
		URL:   func(base string) string { return "https://open.er-api.com/v6/latest/" + strings.ToUpper(base) },
		Parse: parseOpenER,
	},
}

func currencyAPIURL(root, base string) string {
	return root + "/currencies/" + strings.ToLower(base) + ".min.json"
}

// parseCurrencyAPI reads {"date": "...", "eur": {"usd": 1.17, ...}}.
func parseCurrencyAPI(body []byte, base string) (Quote, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return Quote{}, err
	}

	var date string
	_ = json.Unmarshal(raw["date"], &date)

	var perBase map[string]float64
	if err := json.Unmarshal(raw[strings.ToLower(base)], &perBase); err != nil || len(perBase) == 0 {
		return Quote{}, fmt.Errorf("no rates for %s", base)
	}

	return Quote{Date: date, PerBase: upperKeys(perBase)}, nil
}

// parseOpenER reads {"result": "success", "time_last_update_utc": "...",
// "rates": {"USD": 1.17, ...}}.
func parseOpenER(body []byte, base string) (Quote, error) {
	var raw struct {
		Result     string             `json:"result"`
		LastUpdate string             `json:"time_last_update_utc"`
		Rates      map[string]float64 `json:"rates"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return Quote{}, err
	}

	if raw.Result != "success" || len(raw.Rates) == 0 {
		return Quote{}, fmt.Errorf("no rates for %s", base)
	}

	date := raw.LastUpdate
	if parsed, err := time.Parse(time.RFC1123Z, raw.LastUpdate); err == nil {
		date = parsed.UTC().Format("2006-01-02")
	}

	return Quote{Date: date, PerBase: upperKeys(raw.Rates)}, nil
}

func upperKeys(in map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(in))
	for key, value := range in {
		out[strings.ToUpper(key)] = value
	}

	return out
}

// Fetcher gets the latest rates for a base currency.
type Fetcher interface {
	Fetch(ctx context.Context, base string) (Quote, error)
}

// HTTP fetches from Sources over the network.
type HTTP struct {
	Client  *http.Client
	Sources []Source
}

// NewHTTP fetches from every source with a short timeout each.
func NewHTTP() HTTP {
	return HTTP{Client: &http.Client{Timeout: 15 * time.Second}, Sources: Sources}
}

// Fetch returns the first source's quote that works, or every source's
// error when none does.
func (h HTTP) Fetch(ctx context.Context, base string) (Quote, error) {
	base = strings.ToUpper(strings.TrimSpace(base))
	if base == "" {
		return Quote{}, errors.New("the base currency isn't linked to a known currency")
	}

	var errs []error
	for _, source := range h.Sources {
		quote, err := h.fetch(ctx, source, base)
		if err == nil {
			return quote, nil
		}

		errs = append(errs, fmt.Errorf("%s: %w", source.Name, err))
	}

	return Quote{}, fmt.Errorf("couldn't get rates: %w", errors.Join(errs...))
}

func (h HTTP) fetch(ctx context.Context, source Source, base string) (Quote, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL(base), nil)
	if err != nil {
		return Quote{}, err
	}

	res, err := h.Client.Do(req)
	if err != nil {
		return Quote{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return Quote{}, fmt.Errorf("status %s", res.Status)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return Quote{}, err
	}

	quote, err := source.Parse(body, base)
	if err != nil {
		return Quote{}, err
	}

	quote.Base = base
	quote.Source = source.Name

	return quote, nil
}
