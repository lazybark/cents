package desktop

import "testing"

func TestPaidInvoiceAddedToCashflow(t *testing.T) {
	api := newTestAPI(t)
	mustCreate(t, api, NewAccountInput{Name: "Checking", Description: "main", Currency: "$", Amount: "0"})

	month := func(m string) []CashflowRow {
		t.Helper()
		data, err := api.CashflowMonth(m)
		if err != nil {
			t.Fatal(err)
		}
		return data.Entries
	}

	// Refused while unpaid, or without a currency or amount.
	for want, input := range map[string]InvoiceInput{
		"only a paid invoice can be added to incomes and expenses":                       {Title: "A", Currency: "$", Amount: "10", Cashflow: PaymentCashflow{Add: true, Category: "Rent"}},
		"an invoice without a currency can't be added to incomes and expenses: pick one": {Title: "A", Amount: "10", Paid: true, Cashflow: PaymentCashflow{Add: true, Category: "Rent"}},
		"an invoice without an amount can't be added to incomes and expenses":            {Title: "A", Currency: "$", Paid: true, Cashflow: PaymentCashflow{Add: true, Category: "Rent"}},
	} {
		if _, err := api.CreateInvoice(input); err == nil || err.Error() != want {
			t.Errorf("want %q, got %v", want, err)
		}
	}

	if invoices, _ := api.Invoices("paid"); invoices.Counts["paid"]+invoices.Counts["outgoing"]+invoices.Counts["incoming"] != 0 {
		t.Fatal("a refused entry shouldn't save the invoice")
	}

	// An invoice I pay, created paid: an expense.
	if _, err := api.CreateInvoice(InvoiceInput{Title: "Hosting", IsIncoming: true, Peer: "ACME", Currency: "EUR", Amount: "30", Paid: true, PaidOn: "2026-03-05", Cashflow: PaymentCashflow{Add: true, Category: "Rent", Account: "Checking"}}); err != nil {
		t.Fatal(err)
	}

	entries := month("2026-03")
	if len(entries) != 1 || entries[0].IsIncome || entries[0].AmountCents != 3000 || entries[0].Currency != "EUR" || entries[0].Comment != `Invoice "Hosting" (ACME)` || entries[0].Account != "Checking" {
		t.Fatalf("unexpected expense %+v", entries)
	}

	paid, _ := api.Invoices("paid")
	hosting := paid.Invoices[0]
	if hosting.CashflowID != entries[0].ID || len(paid.Cashflow.IncomeCategories) == 0 {
		t.Fatalf("the invoice should know its expense: %+v", hosting)
	}

	// Saving it again doesn't add it twice.
	if _, err := api.UpdateInvoice(InvoiceInput{ID: hosting.ID, Title: "Hosting", IsIncoming: true, Currency: "EUR", Amount: "30", Paid: true, Cashflow: PaymentCashflow{Add: true, Category: "Rent"}}); err == nil || err.Error() != "this invoice is already in incomes and expenses" {
		t.Fatalf("expected a second entry to be refused, got %v", err)
	}

	if _, err := api.UpdateInvoice(InvoiceInput{ID: hosting.ID, Title: "Hosting (fixed)", IsIncoming: true, Currency: "EUR", Amount: "30", Paid: true}); err != nil {
		t.Fatal(err)
	}

	if len(month("2026-03")) != 1 {
		t.Fatal("editing a paid invoice shouldn't touch its expense")
	}

	// Unmarking paid deletes the expense; marking it paid again can add one.
	if _, err := api.UpdateInvoice(InvoiceInput{ID: hosting.ID, Title: "Hosting", IsIncoming: true, Currency: "EUR", Amount: "30"}); err != nil {
		t.Fatal(err)
	}

	if len(month("2026-03")) != 0 {
		t.Fatal("unmarking paid should delete the expense")
	}

	if _, err := api.UpdateInvoice(InvoiceInput{ID: hosting.ID, Title: "Hosting", IsIncoming: true, Currency: "EUR", Amount: "30", Paid: true, PaidOn: "2026-03-09", Cashflow: PaymentCashflow{Add: true, Category: "Rent"}}); err != nil {
		t.Fatal(err)
	}

	if e := month("2026-03"); len(e) != 1 || e[0].Date != "2026-03-09" {
		t.Fatalf("expected the expense again: %+v", e)
	}

	// Deleting the invoice deletes it too.
	if err := api.DeleteInvoice(hosting.ID); err != nil {
		t.Fatal(err)
	}

	if len(month("2026-03")) != 0 {
		t.Fatal("deleting the invoice should delete its expense")
	}

	// An invoice paid to me: an income, marked paid by an edit.
	if _, err := api.CreateInvoice(InvoiceInput{Title: "Design", Currency: "$", Amount: "500"}); err != nil {
		t.Fatal(err)
	}

	outgoing, _ := api.Invoices("outgoing")
	if _, err := api.UpdateInvoice(InvoiceInput{ID: outgoing.Invoices[0].ID, Title: "Design", Currency: "$", Amount: "500", Paid: true, PaidOn: "2026-04-02", Cashflow: PaymentCashflow{Add: true, Category: "Salary"}}); err != nil {
		t.Fatal(err)
	}

	if e := month("2026-04"); len(e) != 1 || !e[0].IsIncome || e[0].Comment != `Invoice "Design"` || e[0].Category != "Salary" {
		t.Fatalf("unexpected income %+v", e)
	}
}
