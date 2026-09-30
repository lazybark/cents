package cashflow

import (
	"testing"
	"time"
)

func TestSearch(t *testing.T) {
	day := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC) }
	entries := []CashflowEntry{
		{ID: 1, Currency: "€", AmountCents: 104037, RateToBase: 1, AmountBaseCents: 104037, Category: "Groceries & living", Comment: "Big shop", EntryDate: day(9, 1)},
		{ID: 2, Currency: "$", AmountCents: 350800, RateToBase: 0.88, AmountBaseCents: 308704, Category: "Anadea", AccountName: "TBC Business USD", IsIncome: true, EntryDate: day(9, 28)},
		{ID: 3, Currency: "€", AmountCents: 1200, RateToBase: 1, AmountBaseCents: 1200, Category: "Coffee", Comment: "Flat white in Dubai", Tags: []string{"Trip to Dubai"}, EntryDate: day(8, 15)},
		{ID: 4, Currency: "₾", AmountCents: 5000, Category: "Coffee", Comment: "no rate", EntryDate: day(7, 2)},
	}
	ids := func(q Query) []uint {
		var got []uint
		for _, e := range Search(entries, q) {
			got = append(got, e.ID)
		}
		return got
	}
	check := func(name string, q Query, want ...uint) {
		t.Helper()
		got := ids(q)
		if len(got) != len(want) {
			t.Errorf("%s: want %v, got %v", name, want, got)
			return
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s: want %v, got %v", name, want, got)
				return
			}
		}
	}

	low, high := int64(1000), int64(200000)
	check("everything, newest first", Query{}, 2, 1, 3, 4)
	check("text in the comment, any case", Query{Text: "dubai"}, 3)
	check("text in the category", Query{Text: "coffee"}, 3, 4)
	check("text in the account", Query{Text: "tbc"}, 2)
	check("an amount as typed", Query{Text: "1040.37"}, 1)
	check("a whole amount", Query{Text: "3508"}, 2)
	check("kind", Query{Kind: "income"}, 2)
	check("category, ignoring case", Query{Category: "coffee", Kind: "expense"}, 3, 4)
	check("account", Query{Account: "tbc business usd"}, 2)
	check("currency", Query{Currency: "₾"}, 4)
	check("tag, ignoring case", Query{Tag: "trip to dubai"}, 3)
	check("text in a tag", Query{Text: "trip"}, 3)
	check("dates, both ends in", Query{From: day(8, 15), To: day(9, 1)}, 1, 3)
	check("amount in base, unconverted left out", Query{MinBase: &low, MaxBase: &high}, 1, 3)
}
