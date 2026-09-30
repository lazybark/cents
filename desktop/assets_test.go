package desktop

import (
	"strings"
	"testing"
)

func TestAssetLifecycle(t *testing.T) {
	api := newTestAPI(t)

	if _, err := api.Assets("boats"); err == nil {
		t.Fatal("expected unknown kind")
	}

	if _, err := api.CreateAsset(AssetInput{Kind: "property", Currency: "GBP", Name: "Flat", Type: "Apartment", Value: "1"}); err == nil || !strings.Contains(err.Error(), "unknown currency") {
		t.Fatalf("expected unknown currency, got %v", err)
	}

	flat, err := api.CreateAsset(AssetInput{Kind: "property", Currency: "eur", Name: "Flat", Type: "Apartment", Value: "200000", Cost: "150000", AcquiredAt: "2019-04-01"})
	if err != nil || flat.Currency != "EUR" || flat.BaseCents != 21600000 || !flat.HasGain || flat.GainCents != 5000000 {
		t.Fatalf("create: %+v %v", flat, err)
	}

	if _, err := api.CreateAsset(AssetInput{Kind: "investment", Currency: "$", Name: "Index fund", Type: "ETFs", Value: "1000"}); err != nil {
		t.Fatal(err)
	}

	if _, err := api.CreateAsset(AssetInput{Kind: "property", Currency: "$", Name: "Home", Type: "House", Value: "50", IgnoreInNetWorth: true}); err != nil {
		t.Fatal(err)
	}

	view, err := api.Assets("property")
	if err != nil || len(view.Assets) != 2 || view.TotalCents != 21600000 || view.Ignored != 1 || view.GainCents != 5400000 || len(view.Types) == 0 || view.Types[0] != "Apartment" {
		t.Fatalf("property view %+v %v", view, err)
	}

	overview, err := api.Overview()
	if err != nil || overview.Property != 21600000 || overview.Investments != 100000 || overview.NetWorth != 21600000+100000 {
		t.Fatalf("overview should count assets: %+v %v", overview, err)
	}

	logs, err := api.AssetValueLogs(flat.ID)
	if err != nil || len(logs) != 1 || logs[0].ValueCents != 20000000 {
		t.Fatalf("creating should log today's value: %+v %v", logs, err)
	}

	if err := api.SaveAssetValueLog(AssetValueLogInput{AssetID: flat.ID, Date: "2024-01-01", Value: "180000"}); err != nil {
		t.Fatal(err)
	}

	updated, err := api.UpdateAsset(AssetInput{ID: flat.ID, Name: "Flat", Type: "Apartment", Value: "210000", Cost: "150000", Kind: "investment", Currency: "$"})
	if err != nil || updated.Kind != "property" || updated.Currency != "EUR" || updated.ValueCents != 21000000 || updated.AcquiredAt != "" {
		t.Fatalf("update: %+v %v", updated, err)
	}

	logs, _ = api.AssetValueLogs(flat.ID)
	if len(logs) != 2 || logs[0].ValueCents != 21000000 || logs[1].Date != "2024-01-01" {
		t.Fatalf("a new value should replace today's entry: %+v", logs)
	}

	if err := api.DeleteAssetValueLog(flat.ID, logs[1].ID); err != nil {
		t.Fatal(err)
	}

	if err := api.DeleteAssetValueLog(flat.ID, logs[1].ID); err == nil || err.Error() != "log entry not found" {
		t.Fatalf("expected log not found, got %v", err)
	}

	if err := api.DeleteAsset(flat.ID); err != nil {
		t.Fatal(err)
	}

	if err := api.DeleteAsset(flat.ID); err != errAssetNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}
