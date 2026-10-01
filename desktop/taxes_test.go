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

func TestTaxInAnotherCurrencyKeepsItsRate(t *testing.T) {
	api := newTestAPI(t)
	if err := api.SaveTaxType(TaxTypeInput{Country: "Germany", Name: "Income"}); err != nil {
		t.Fatal(err)
	}

	view, _ := api.Taxes("unpaid")
	typeID := view.TaxTypes[0].ID

	if len(view.Currencies) != 2 || view.Currencies[0] != (TaxCurrency{Name: "$", Rate: 1}) || view.Currencies[1] != (TaxCurrency{Name: "EUR", Rate: 1.08}) {
		t.Fatalf("unexpected currency options %+v", view.Currencies)
	}

	if _, err := api.CreateTax(NewTaxInput{TaxTypeID: typeID, Currency: "GBP", AmountDue: "1", AmountPaid: "0", Period: "2026"}); err == nil || err.Error() != `unknown currency "GBP": add it in settings first` {
		t.Fatalf("expected unknown currency, got %v", err)
	}

	if _, err := api.CreateTax(NewTaxInput{TaxTypeID: typeID, Currency: "eur", Rate: "abc", AmountDue: "1", AmountPaid: "0", Period: "2026"}); err == nil || err.Error() != "rate must be a number" {
		t.Fatalf("expected rate error, got %v", err)
	}

	// An empty rate takes the one in settings; a typed one wins.
	if _, err := api.CreateTax(NewTaxInput{TaxTypeID: typeID, Currency: "eur", AmountDue: "100", AmountPaid: "0", Period: "2025"}); err != nil {
		t.Fatal(err)
	}

	if _, err := api.CreateTax(NewTaxInput{TaxTypeID: typeID, Currency: "EUR", Rate: "1.2", AmountDue: "1000", AmountPaid: "100", Period: "2026"}); err != nil {
		t.Fatal(err)
	}

	// Base currency taxes always use rate 1, whatever is typed.
	if _, err := api.CreateTax(NewTaxInput{TaxTypeID: typeID, Currency: "$", Rate: "5", AmountDue: "10", AmountPaid: "0", Period: "2024"}); err != nil {
		t.Fatal(err)
	}

	byPeriod := func() map[string]TaxRow {
		view, err := api.Taxes("unpaid")
		if err != nil {
			t.Fatal(err)
		}

		rows := map[string]TaxRow{}
		for _, row := range view.Taxes {
			rows[row.Period] = row
		}

		return rows
	}

	rows := byPeriod()
	if r := rows["2025"]; r.Currency != "EUR" || r.RateToBase != 1.08 || r.BaseDueCents != 10800 || r.IsBase {
		t.Fatalf("settings rate not recorded: %+v", r)
	}

	if r := rows["2026"]; r.RateToBase != 1.2 || r.DueCents != 100000 || r.BaseDueCents != 120000 || r.BaseLeftCents != 108000 {
		t.Fatalf("typed rate not recorded: %+v", r)
	}

	if r := rows["2024"]; !r.IsBase || r.RateToBase != 1 || r.BaseDueCents != 1000 {
		t.Fatalf("base tax should have rate 1: %+v", r)
	}

	// Changing the rate in settings leaves recorded taxes alone.
	if err := api.SaveCurrency(CurrencyInput{ID: 1, Name: "EUR", Rate: "2"}); err != nil {
		t.Fatal(err)
	}

	tax2026 := byPeriod()["2026"]
	if tax2026.BaseDueCents != 120000 {
		t.Fatalf("settings rate change moved a recorded tax: %+v", tax2026)
	}

	overview, err := api.Overview()
	if err != nil || overview.UnpaidTaxes != 10800+108000+1000 {
		t.Fatalf("overview should use recorded base amounts: %+v %v", overview, err)
	}

	row, err := api.AddTaxPayment(PaymentInput{ID: tax2026.ID, Delta: "100"})
	if err != nil || row.PaidCents != 20000 || row.BasePaidCents != 24000 {
		t.Fatalf("payment should convert at the recorded rate: %+v %v", row, err)
	}

	if err := api.UpdateTax(TaxUpdateInput{ID: tax2026.ID, Rate: "0", AmountDue: "1000", AmountPaid: "200", Period: "2026"}); err == nil || err.Error() != "rate must be greater than zero" {
		t.Fatalf("expected rate error, got %v", err)
	}

	if err := api.UpdateTax(TaxUpdateInput{ID: tax2026.ID, Rate: "1.1", AmountDue: "1000", AmountPaid: "200", Period: "2026"}); err != nil {
		t.Fatal(err)
	}

	if r := byPeriod()["2026"]; r.RateToBase != 1.1 || r.BaseDueCents != 110000 || r.BasePaidCents != 22000 {
		t.Fatalf("edited rate not applied: %+v", r)
	}

	if err := api.UpdateTax(TaxUpdateInput{ID: tax2026.ID, AmountDue: "1000", AmountPaid: "200", Period: "2026"}); err != nil {
		t.Fatal(err)
	}

	if r := byPeriod()["2026"]; r.RateToBase != 1.1 {
		t.Fatalf("an empty rate should keep the recorded one: %+v", r)
	}
}
