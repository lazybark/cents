package desktop

import "testing"

func TestTaxLifecycle(t *testing.T) {
	api := newTestAPI(t)

	if _, err := api.CreateTax(NewTaxInput{TaxTypeID: 1, AmountDue: "1", AmountPaid: "0", Period: "Q1"}); err == nil || err.Error() != "no tax types configured; add one in settings" {
		t.Fatalf("expected missing tax types error, got %v", err)
	}

	if err := api.SaveTaxType(TaxTypeInput{Country: "Netherlands", Name: "VAT"}); err != nil {
		t.Fatal(err)
	}

	view, err := api.Taxes("unpaid")
	if err != nil || len(view.TaxTypes) != 1 || view.TaxTypes[0].Label != "Netherlands / VAT" {
		t.Fatalf("expected tax type option, got %+v %v", view.TaxTypes, err)
	}

	typeID := view.TaxTypes[0].ID

	if _, err := api.CreateTax(NewTaxInput{TaxTypeID: typeID + 9, AmountDue: "1", AmountPaid: "0", Period: "Q1"}); err == nil || err.Error() != "unknown tax type" {
		t.Fatalf("expected unknown type error, got %v", err)
	}

	created, err := api.CreateTax(NewTaxInput{TaxTypeID: typeID, AmountDue: "2000", AmountPaid: "500", Period: "Q3 2026", DueDate: "2026-10-31"})
	if err != nil || created.Mode != "unpaid" {
		t.Fatalf("create: %+v %v", created, err)
	}

	view, _ = api.Taxes("unpaid")
	vat := view.Taxes[0]
	if len(view.Taxes) != 1 || vat.Country != "Netherlands" || vat.TypeName != "VAT" || vat.LeftCents != 150000 || vat.PaidPercent != 25 || vat.DueDate != "2026-10-31" {
		t.Fatalf("unexpected tax %+v", vat)
	}

	if view.Progress != (Progress{PaidCents: 50000, TotalCents: 200000}) || view.Counts["unpaid"] != 1 || view.Counts["paid"] != 0 {
		t.Fatalf("unexpected progress/counts %+v %+v", view.Progress, view.Counts)
	}

	if _, err := api.AddTaxPayment(PaymentInput{ID: vat.ID, Delta: "-600"}); err == nil || err.Error() != "delta makes amount paid negative" {
		t.Fatalf("expected negative error, got %v", err)
	}

	row, err := api.AddTaxPayment(PaymentInput{ID: vat.ID, Delta: "1600"})
	if err != nil || row.PaidCents != 210000 || row.PaidPercent != 100 || row.LeftCents != 0 {
		t.Fatalf("overpaying a tax is allowed: %+v %v", row, err)
	}

	logs, err := api.TaxLogs(vat.ID)
	if err != nil || len(logs) != 1 || logs[0].Note != "manual tax paid adjustment" {
		t.Fatalf("logs: %+v %v", logs, err)
	}

	view, _ = api.Taxes("paid")
	if len(view.Taxes) != 1 || view.Counts["unpaid"] != 0 {
		t.Fatalf("expected tax in paid list, got %+v", view)
	}

	if err := api.UpdateTax(TaxUpdateInput{ID: vat.ID, AmountDue: "0", AmountPaid: "0", Period: "Q3"}); err == nil || err.Error() != "amount due must be greater than zero" {
		t.Fatalf("expected amount due error, got %v", err)
	}

	if err := api.UpdateTax(TaxUpdateInput{ID: vat.ID, AmountDue: "3000", AmountPaid: "2100", Period: "Q3 2026", Comment: "corrected"}); err != nil {
		t.Fatal(err)
	}

	view, _ = api.Taxes("unpaid")
	if len(view.Taxes) != 1 || view.Taxes[0].Comment != "corrected" || view.Taxes[0].DueDate != "" {
		t.Fatalf("expected edited tax back in unpaid, got %+v", view.Taxes)
	}

	if err := api.DeleteTax(vat.ID); err != nil {
		t.Fatal(err)
	}

	if err := api.DeleteTax(vat.ID); err != errTaxNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}
