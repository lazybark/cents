package desktop

import (
	"strings"
	"testing"
)

func TestTags(t *testing.T) {
	api := newTestAPI(t) // Rent (expense), Salary (income); EUR at 1.08

	for _, input := range []NewCashflowInput{
		{Currency: "$", Amount: "100", Date: "2026-03-02", Category: "Rent", Tags: []string{"Trip to Japan", "hotel"}},
		{Currency: "EUR", Amount: "50", Date: "2026-03-05", Category: "Rent", Tags: []string{"trip to japan"}},
		{IsIncome: true, Currency: "$", Amount: "20", Date: "2026-03-09", Category: "Salary", Tags: []string{"Trip to Japan"}},
		{Currency: "$", Amount: "7", Date: "2026-04-01", Category: "Rent"},
	} {
		if _, err := api.CreateCashflow(input); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := api.CreateCashflow(NewCashflowInput{Currency: "$", Amount: "1", Date: "2026-04-01", Category: "Rent", Tags: []string{"a, b"}}); err == nil {
		t.Fatal("expected a comma in a tag to fail")
	}

	month, _ := api.CashflowMonth("2026-03")
	if len(month.Options.Tags) != 2 {
		t.Fatalf("expected both tags suggested: %v", month.Options.Tags)
	}
	for _, e := range month.Entries {
		if e.Comment == "" && e.AmountCents == 10000 && strings.Join(e.Tags, ",") != "hotel,Trip to Japan" {
			t.Fatalf("unexpected tags %v", e.Tags)
		}
	}

	view, err := api.CashflowTags()
	if err != nil || len(view.Tags) != 2 || view.Unused != 0 {
		t.Fatalf("unexpected %+v %v", view, err)
	}
	japan := view.Tags[0]
	if japan.Name != "Trip to Japan" || japan.Count != 3 || japan.ExpenseCents != 10000+5400 || japan.IncomeCents != 2000 || japan.First != "2026-03-02" || japan.Last != "2026-03-09" || len(japan.Categories) != 2 {
		t.Fatalf("unexpected %+v", japan)
	}

	// Edit drops one; search finds by tag; bulk adds and removes.
	var hotelEntry CashflowRow
	for _, e := range month.Entries {
		if len(e.Tags) == 2 {
			hotelEntry = e
		}
	}
	if _, err := api.UpdateCashflow(CashflowUpdateInput{ID: hotelEntry.ID, NewCashflowInput: NewCashflowInput{Currency: "$", Amount: "100", Date: "2026-03-02", Category: "Rent", Tags: []string{"Trip to Japan"}}}); err != nil {
		t.Fatal(err)
	}

	found, _ := api.CashflowSearch(CashflowSearchInput{Tag: "trip to japan"})
	if found.Total != 3 || len(found.Options.Tags) != 2 {
		t.Fatalf("unexpected search %+v", found)
	}

	ids := []uint{}
	for _, e := range found.Entries {
		ids = append(ids, e.ID)
	}
	if _, err := api.ChangeCashflows(CashflowBulkInput{IDs: ids, AddTag: "2026"}); err != nil {
		t.Fatal(err)
	}
	if _, err := api.ChangeCashflows(CashflowBulkInput{IDs: ids, RemoveTag: "TRIP TO JAPAN"}); err != nil {
		t.Fatal(err)
	}
	if found, _ = api.CashflowSearch(CashflowSearchInput{Tag: "2026"}); found.Total != 3 || strings.Join(found.Entries[0].Tags, ",") != "2026" {
		t.Fatalf("expected the tag swapped: %+v", found.Entries)
	}

	// Settings: hotel and Japan have no entries now; rename, merge, delete.
	settings, _ := api.Settings()
	ids2 := map[string]CategorySetting{}
	for _, tg := range settings.Tags {
		ids2[tg.Name] = tg
	}
	if ids2["2026"].UsedBy != 3 || ids2["hotel"].UsedBy != 0 {
		t.Fatalf("unexpected usage %+v", settings.Tags)
	}
	if view, _ = api.CashflowTags(); view.Unused != 2 {
		t.Fatalf("expected two unused tags: %+v", view)
	}

	if err := api.SaveTag(TagInput{ID: ids2["2026"].ID, Name: "Japan 2026"}); err != nil {
		t.Fatal(err)
	}
	if err := api.SaveTag(TagInput{ID: ids2["hotel"].ID, Name: "japan 2026"}); err == nil {
		t.Fatal("expected a clash to fail")
	}
	// Merge the renamed tag into hotel: its three entries move.
	if result, err := api.Merge(MergeInput{Kind: "tag", FromID: ids2["2026"].ID, IntoID: ids2["hotel"].ID}); err != nil || result.Moved != 3 {
		t.Fatalf("unexpected merge %+v %v", result, err)
	}
	if found, _ = api.CashflowSearch(CashflowSearchInput{Tag: "hotel"}); found.Total != 3 {
		t.Fatalf("expected three on hotel: %+v", found.Entries)
	}
	if err := api.DeleteSetting("tag", ids2["Trip to Japan"].ID); err != nil {
		t.Fatal(err)
	}
}
