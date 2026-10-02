package currency

import "testing"

func TestFindAndCodeFor(t *testing.T) {
	if c, ok := Find(" eur "); !ok || c.Name != "Euro" || c.Symbol != "€" {
		t.Fatalf("unexpected %+v %v", c, ok)
	}

	cases := map[string]string{"EUR": "EUR", "usd": "USD", "€": "EUR", "₾": "GEL", "₸": "KZT", "₽": "RUB", "₴": "UAH", "$": "", "¥": "", "Shells": ""}
	for name, want := range cases {
		if got := CodeFor(name); got != want {
			t.Errorf("%q: want %q, got %q", name, want, got)
		}
	}

	seen := map[string]bool{}
	for _, c := range All() {
		if seen[c.Code] || c.Name == "" || c.Symbol == "" {
			t.Errorf("bad or repeated entry %+v", c)
		}
		seen[c.Code] = true
	}

	for symbol, code := range symbols {
		if _, ok := Find(code); !ok {
			t.Errorf("symbol %q points to unknown %q", symbol, code)
		}
	}
}
