package sqlite

import (
	"path/filepath"
	"testing"

	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/tax"
)

func TestOlderTaxesGetTheBaseCurrency(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cents.db")
	db, _, err := OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Model(&settings.SettingRecord{}).Where("setting_id = ?", settings.BaseCurrencySettingID).Update("setting_value", " USD ").Error; err != nil {
		t.Fatal(err)
	}

	recorded := tax.Tax{Currency: "EUR", RateToBase: 1.2, AmountDueCents: 100, AmountDueBaseCents: 120}
	older := tax.Tax{AmountDueCents: 5000, AmountPaidCents: 1500}
	for _, entry := range []*tax.Tax{&recorded, &older} {
		if err := db.Create(entry).Error; err != nil {
			t.Fatal(err)
		}
	}

	// Taxes saved before the columns existed have no rate at all.
	if err := db.Exec("UPDATE taxes SET rate_to_base = NULL WHERE id = ?", older.ID).Error; err != nil {
		t.Fatal(err)
	}

	db, _, err = OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}

	items, err := NewSQLiteStorage(db).LoadTaxes()
	if err != nil {
		t.Fatal(err)
	}

	for _, item := range items {
		switch item.ID {
		case older.ID:
			if item.Currency != "USD" || item.RateToBase != 1 || item.AmountDueBaseCents != 5000 || item.AmountPaidBaseCents != 1500 {
				t.Fatalf("older tax not backfilled: %+v", item)
			}
		case recorded.ID:
			if item.Currency != "EUR" || item.RateToBase != 1.2 || item.AmountDueBaseCents != 120 {
				t.Fatalf("recorded tax changed: %+v", item)
			}
		}
	}
}
