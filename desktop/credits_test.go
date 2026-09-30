package desktop

import (
	"strings"
	"testing"
)

func TestCreditLifecycle(t *testing.T) {
	api := newTestAPI(t)

	if _, err := api.CreateCredit(CreditInput{Currency: "GBP", Name: "x", Total: "1", StartDate: "2026-01-01"}); err == nil || !strings.Contains(err.Error(), "unknown currency") {
		t.Fatalf("expected unknown currency, got %v", err)
	}

	created, err := api.CreateCredit(CreditInput{Currency: "eur", Name: "Mortgage", Purpose: "Mortgage", Issuer: "ING", Total: "100000", Paid: "10000", InterestPercent: "3.2", StartDate: "2020-01-01", DueDate: "2021-01-01"})
	if err != nil || created.Mode != "active" {
		t.Fatalf("create: %+v %v", created, err)
	}

	if _, err := api.CreateCredit(CreditInput{Currency: "$", Name: "Old card", Total: "500", Paid: "500", StartDate: "2019-01-01"}); err != nil {
		t.Fatal(err)
	}

	view, err := api.Credits("active")
	if err != nil || len(view.Credits) != 1 || view.Counts["active"] != 1 || view.Counts["paid"] != 1 || len(view.Purposes) == 0 {
		t.Fatalf("view %+v %v", view, err)
	}

	mortgage := view.Credits[0]
	if mortgage.Currency != "EUR" || mortgage.RateToBase != 1.08 || mortgage.BaseLeftCents != 9720000 || !mortgage.Overdue || mortgage.InterestPercent != 3.2 || mortgage.Issuer != "ING" {
		t.Fatalf("unexpected row %+v", mortgage)
	}

	if view.LeftCents != 9720000 || view.Progress != (Progress{PaidCents: 1080000, TotalCents: 10800000}) {
		t.Fatalf("unexpected totals %+v", view)
	}

	overview, err := api.Overview()
	if err != nil || overview.Credits != 9720000 || overview.NetWorth != -9720000 {
		t.Fatalf("overview should count credits as owed: %+v %v", overview, err)
	}

	row, err := api.AddCreditLog(CreditLogInput{ID: mortgage.ID, Kind: "payment", Amount: "5000", Date: "2026-03-01"})
	if err != nil || row.PaidCents != 1500000 {
		t.Fatalf("payment: %+v %v", row, err)
	}

	row, err = api.AddCreditLog(CreditLogInput{ID: mortgage.ID, Kind: "addition", Amount: "200", Note: "fee"})
	if err != nil || row.TotalCents != 10020000 || row.BaseTotalCents != 10821600 {
		t.Fatalf("addition: %+v %v", row, err)
	}

	logs, err := api.CreditLogs(mortgage.ID)
	if err != nil || len(logs) != 2 || logs[0].Kind != "addition" || logs[0].DeltaCents != 20000 || logs[0].Note != "fee" || logs[1].Kind != "payment" || !strings.HasPrefix(logs[1].When, "2026-03-01 ") {
		t.Fatalf("logs: %+v %v", logs, err)
	}

	row, err = api.DeleteCreditLog(mortgage.ID, logs[0].ID)
	if err != nil || row.TotalCents != 10000000 {
		t.Fatalf("undo addition: %+v %v", row, err)
	}

	if _, err := api.DeleteCreditLog(mortgage.ID, logs[0].ID); err != errLogNotFound {
		t.Fatalf("expected log not found, got %v", err)
	}

	moved, err := api.UpdateCredit(CreditInput{ID: mortgage.ID, Rate: "1.2", Name: "Mortgage", Total: "100000", Paid: "100000", StartDate: "2020-01-01"})
	if err != nil || moved.Mode != "paid" {
		t.Fatalf("pay off by edit: %+v %v", moved, err)
	}

	view, _ = api.Credits("paid")
	for _, r := range view.Credits {
		if r.ID == mortgage.ID && (r.RateToBase != 1.2 || r.BaseTotalCents != 12000000 || r.Overdue) {
			t.Fatalf("edit not applied: %+v", r)
		}
	}

	if err := api.DeleteCredit(mortgage.ID); err != nil {
		t.Fatal(err)
	}

	if err := api.DeleteCredit(mortgage.ID); err != errCreditNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}
