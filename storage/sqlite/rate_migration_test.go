package sqlite

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/debt"
	"github.com/lazybark/cents/flows/invoice"
	"github.com/lazybark/cents/flows/settings"
)

func TestOlderRecordsGetARate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cents.db")
	db, _, err := OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Create(&settings.SettingCurrency{CurrencyName: "EUR", RateToBase: 1.08}).Error; err != nil {
		t.Fatal(err)
	}

	day := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	flows := []cashflow.CashflowEntry{
		{Currency: "$", AmountCents: 1000, EntryDate: day},
		{Currency: " eur ", AmountCents: 1001, EntryDate: day},
		{Currency: "BTC", AmountCents: 7, EntryDate: day},
		{Currency: "EUR", AmountCents: 500, EntryDate: day, RateToBase: 1.5, AmountBaseCents: 750},
	}
	debts := []debt.Debt{{Currency: "EUR", AmountCents: 10000, AmountPaidCents: 2500}}
	invoices := []invoice.Invoice{{Title: "none", AmountCents: 300}, {Title: "eur", Currency: "EUR", AmountCents: 200}}

	for _, rows := range []any{&flows, &debts, &invoices} {
		if err := db.Create(rows).Error; err != nil {
			t.Fatal(err)
		}
	}

	// Records saved before rates were kept have none at all; the last
	// cashflow entry already has its own.
	for _, table := range []string{"cashflow_entries", "debts", "invoices"} {
		if err := db.Exec("UPDATE " + table + " SET rate_to_base = NULL WHERE NOT (rate_to_base = 1.5)").Error; err != nil {
			t.Fatal(err)
		}
	}

	if db, _, err = OpenDatabase(path); err != nil {
		t.Fatal(err)
	}

	store := NewSQLiteStorage(db)
	gotFlows, _ := store.LoadCashflows()
	want := map[int64][2]float64{1000: {1, 1000}, 1001: {1.08, 1081}, 7: {0, 0}, 500: {1.5, 750}}
	for _, entry := range gotFlows {
		if w := want[entry.AmountCents]; entry.RateToBase != w[0] || float64(entry.AmountBaseCents) != w[1] {
			t.Errorf("cashflow %d: rate %v base %d, want %v", entry.AmountCents, entry.RateToBase, entry.AmountBaseCents, w)
		}
	}

	gotDebts, _ := store.LoadDebts()
	if d := gotDebts[0]; d.RateToBase != 1.08 || d.AmountBaseCents != 10800 || d.AmountPaidBaseCents != 2700 {
		t.Errorf("debt not backfilled: %+v", d)
	}

	gotInvoices, _ := store.LoadInvoices()
	for _, item := range gotInvoices {
		if (item.Title == "none" && (item.RateToBase != 0 || item.AmountBaseCents != 0)) || (item.Title == "eur" && (item.RateToBase != 1.08 || item.AmountBaseCents != 216)) {
			t.Errorf("invoice not backfilled: %+v", item)
		}
	}
}
