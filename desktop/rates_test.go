package desktop

import "testing"

func TestCashflowKeepsItsRate(t *testing.T) {
	api := newTestAPI(t)
	add := func(currency, amount, date, rate string) {
		t.Helper()
		if _, err := api.CreateCashflow(NewCashflowInput{IsIncome: true, Currency: currency, Amount: amount, Date: date, Category: "Salary", Rate: rate}); err != nil {
			t.Fatal(err)
		}
	}

	add("EUR", "100", "2026-03-02", "")
	add("eur", "200", "2026-03-20", "1.2")
	add("EUR", "50", "2026-04-01", "")
	add("$", "10", "2026-03-05", "9")

	if _, err := api.CreateCashflow(NewCashflowInput{IsIncome: true, Currency: "EUR", Amount: "1", Date: "2026-03-01", Category: "Salary", Rate: "0"}); err == nil || err.Error() != "rate must be greater than zero" {
		t.Fatalf("expected rate error, got %v", err)
	}

	month, err := api.CashflowMonth("2026-03")
	if err != nil {
		t.Fatal(err)
	}

	if month.IncomeCents != 10800+24000+1000 || len(month.Options.Rates) != 2 || month.Options.Rates[1] != (CurrencyRate{Name: "EUR", Rate: 1.08}) {
		t.Fatalf("unexpected month %+v", month)
	}

	// A later rate change in settings leaves recorded entries alone.
	if err := api.SaveCurrency(CurrencyInput{ID: 1, Name: "EUR", Rate: "2"}); err != nil {
		t.Fatal(err)
	}

	month, _ = api.CashflowMonth("2026-03")
	if month.IncomeCents != 10800+24000+1000 {
		t.Fatalf("settings change moved recorded entries: %d", month.IncomeCents)
	}

	ids := map[int64]CashflowRow{}
	for _, row := range month.Entries {
		ids[row.AmountCents] = row
	}

	if r := ids[1000]; !r.IsBase || r.RateToBase != 1 {
		t.Fatalf("base entry should use rate 1: %+v", r)
	}

	if _, err := api.SetCashflowRate(CashflowRateInput{ID: ids[1000].ID, Rate: "2"}); err == nil {
		t.Fatal("expected base entry rate change to be refused")
	}

	changed, err := api.SetCashflowRate(CashflowRateInput{ID: ids[10000].ID, Rate: "1.1"})
	if err != nil || changed != 1 {
		t.Fatalf("single rate: %d %v", changed, err)
	}

	month, _ = api.CashflowMonth("2026-03")
	if month.IncomeCents != 11000+24000+1000 {
		t.Fatalf("single rate not applied: %d", month.IncomeCents)
	}

	// The whole month's EUR entries take the rate; April's doesn't.
	changed, err = api.SetCashflowRate(CashflowRateInput{ID: ids[10000].ID, Rate: "1.5", WholeMonth: true})
	if err != nil || changed != 2 {
		t.Fatalf("month rate: %d %v", changed, err)
	}

	month, _ = api.CashflowMonth("2026-03")
	april, _ := api.CashflowMonth("2026-04")
	if month.IncomeCents != 15000+30000+1000 || april.IncomeCents != 5400 {
		t.Fatalf("month rate not applied right: %d %d", month.IncomeCents, april.IncomeCents)
	}
}

func TestDebtAndInvoiceKeepTheirRates(t *testing.T) {
	api := newTestAPI(t)

	if _, err := api.CreateDebt(NewDebtInput{Peer: "Bank", Currency: "EUR", Rate: "1.25", Amount: "100", AmountPaid: "20", CreatedAt: "2026-01-01"}); err != nil {
		t.Fatal(err)
	}

	view, _ := api.Debts("outgoing")
	bank := view.Debts[0]
	if bank.RateToBase != 1.25 || bank.BaseAmountCents != 12500 || bank.BaseLeftCents != 10000 || !bank.HasRate || view.Progress.TotalCents != 12500 {
		t.Fatalf("unexpected debt %+v %+v", bank, view.Progress)
	}

	if err := api.UpdateDebt(DebtUpdateInput{ID: bank.ID, Rate: "1.1", Amount: "100", AmountPaid: "20", CreatedAt: "2026-01-01"}); err != nil {
		t.Fatal(err)
	}

	view, _ = api.Debts("outgoing")
	if r := view.Debts[0]; r.RateToBase != 1.1 || r.BaseLeftCents != 8800 {
		t.Fatalf("edited rate not applied: %+v", r)
	}

	if err := api.SaveCurrency(CurrencyInput{ID: 1, Name: "EUR", Rate: "3"}); err != nil {
		t.Fatal(err)
	}

	if _, err := api.CreateInvoice(InvoiceInput{Title: "Work", Currency: "EUR", Amount: "10"}); err != nil {
		t.Fatal(err)
	}

	invoices, _ := api.Invoices("outgoing")
	work := invoices.Invoices[0]
	if work.RateToBase != 3 || work.BaseCents != 3000 || invoices.UnpaidToMeCents != 3000 {
		t.Fatalf("invoice should take the settings rate: %+v", work)
	}

	if _, err := api.UpdateInvoice(InvoiceInput{ID: work.ID, Title: "Work", Currency: "EUR", Amount: "20", Rate: "2.5"}); err != nil {
		t.Fatal(err)
	}

	if err := api.SaveCurrency(CurrencyInput{ID: 1, Name: "EUR", Rate: "4"}); err != nil {
		t.Fatal(err)
	}

	if _, err := api.UpdateInvoice(InvoiceInput{ID: work.ID, Title: "Work again", Currency: "EUR", Amount: "20"}); err != nil {
		t.Fatal(err)
	}

	invoices, _ = api.Invoices("outgoing")
	if r := invoices.Invoices[0]; r.RateToBase != 2.5 || r.BaseCents != 5000 {
		t.Fatalf("an empty rate should keep the recorded one: %+v", r)
	}
}
