package desktop

import (
	"strings"
	"testing"
)

func TestInvoiceLifecycle(t *testing.T) {
	api := newTestAPI(t)
	mustCreate(t, api, NewAccountInput{Name: "Checking", Description: "main", Currency: "$", Amount: "0"})

	if _, err := api.CreateInvoice(InvoiceInput{Title: "X", Currency: "GBP"}); err == nil || !strings.Contains(err.Error(), "unknown currency") {
		t.Fatalf("expected unknown currency, got %v", err)
	}

	created, err := api.CreateInvoice(InvoiceInput{Title: " Design work ", Currency: "eur", Amount: "100", Peer: "ACME", DueDate: "2020-01-01", TargetAccount: "Checking", URL: "https://example.test/1"})
	if err != nil || created.Mode != "outgoing" {
		t.Fatalf("create outgoing: %+v %v", created, err)
	}

	created, err = api.CreateInvoice(InvoiceInput{Title: "Hosting", IsIncoming: true, Currency: "$", Amount: "30"})
	if err != nil || created.Mode != "incoming" {
		t.Fatalf("create incoming: %+v %v", created, err)
	}

	// No currency, like the TUI allows: listed but not counted.
	if _, err := api.CreateInvoice(InvoiceInput{Title: "Draft", Amount: "5"}); err != nil {
		t.Fatal(err)
	}

	view, err := api.Invoices("outgoing")
	if err != nil {
		t.Fatal(err)
	}

	if len(view.Invoices) != 2 || view.Counts["outgoing"] != 2 || view.Counts["incoming"] != 1 || view.Counts["paid"] != 0 {
		t.Fatalf("unexpected lists %+v %+v", view.Invoices, view.Counts)
	}

	if view.UnpaidToMeCents != 10800 || view.UnpaidByMeCents != 3000 || view.NotCounted != 1 {
		t.Fatalf("unexpected totals %+v", view)
	}

	if len(view.Accounts) != 1 || view.Accounts[0] != "Checking" {
		t.Fatalf("expected account names, got %+v", view.Accounts)
	}

	design := view.Invoices[0]
	if design.Title != "Design work" || design.Currency != "EUR" || !design.Overdue || design.TargetAccount != "Checking" {
		t.Fatalf("unexpected row %+v", design)
	}

	// The overview counts what's owed to me as an asset.
	sum, err := api.Overview()
	if err != nil {
		t.Fatal(err)
	}

	if sum.InvoicesToMe != 10800 || sum.InvoicesByMe != 3000 {
		t.Fatalf("overview invoices %+v", sum)
	}

	moved, err := api.UpdateInvoice(InvoiceInput{ID: design.ID, Title: "Design work", Currency: "EUR", Amount: "100", Paid: true})
	if err != nil || moved.Mode != "paid" {
		t.Fatalf("mark paid: %+v %v", moved, err)
	}

	view, _ = api.Invoices("paid")
	if len(view.Invoices) != 1 || view.Invoices[0].Overdue || view.Invoices[0].DueDate != "" || view.UnpaidToMeCents != 0 {
		t.Fatalf("expected paid invoice with cleared due date, got %+v", view)
	}

	if _, err := api.UpdateInvoice(InvoiceInput{ID: design.ID, Title: " "}); err == nil || err.Error() != "invoice title is required" {
		t.Fatalf("expected title error, got %v", err)
	}

	if err := api.OpenInvoiceURL(view.Invoices[0].ID); err == nil {
		t.Fatal("expected the cleared link to be refused")
	}

	if err := api.DeleteInvoice(design.ID); err != nil {
		t.Fatal(err)
	}

	if err := api.DeleteInvoice(design.ID); err != errInvoiceNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}
