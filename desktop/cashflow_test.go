package desktop

import (
	"strings"
	"testing"
)

func TestCashflowLifecycle(t *testing.T) {
	api := newTestAPI(t)
	mustCreate(t, api, NewAccountInput{Name: "Checking", Description: "main", Currency: "$", Amount: "1"})

	result, err := api.CreateCashflow(NewCashflowInput{IsIncome: true, Currency: "$", Amount: "1000", Date: "2026-09-12", Category: "Salary", Account: "checking"})
	if err != nil || result.Month != "2026-09" {
		t.Fatalf("create income: %+v %v", result, err)
	}

	if _, err := api.CreateCashflow(NewCashflowInput{Currency: "eur", Amount: "100", Date: "2026-09-20", Category: "Rent", Comment: " flat "}); err != nil {
		t.Fatal(err)
	}

	if _, err := api.CreateCashflow(NewCashflowInput{Currency: "$", Amount: "50", Date: "2026-07-01", Category: "Rent"}); err != nil {
		t.Fatal(err)
	}

	month, err := api.CashflowMonth("2026-09")
	if err != nil {
		t.Fatal(err)
	}

	if month.IncomeCents != 100000 || month.ExpenseCents != 10800 || month.NetCents != 89200 || len(month.Entries) != 2 {
		t.Fatalf("unexpected month %+v", month)
	}

	if first := month.Entries[0]; first.Date != "2026-09-20" || first.IsIncome || first.Currency != "EUR" || first.Comment != "flat" {
		t.Fatalf("expected newest expense first with canonical currency, got %+v", first)
	}

	if second := month.Entries[1]; second.Account != "Checking" || second.Category != "Salary" {
		t.Fatalf("expected account matched to its real name, got %+v", second)
	}

	if strings.Join(month.Options.IncomeCategories, ",") != "Salary" || strings.Join(month.Options.Accounts, ",") != "Checking" {
		t.Fatalf("unexpected options %+v", month.Options)
	}

	overview, err := api.CashflowOverview()
	if err != nil {
		t.Fatal(err)
	}

	if len(overview.Rows) != 3 || overview.Rows[0].Month != "2026-09" || overview.Rows[1].Month != "2026-08" || overview.Rows[2].Month != "2026-07" {
		t.Fatalf("expected Sep, Aug, Jul newest first, got %+v", overview.Rows)
	}

	if sep := overview.Rows[0]; sep.NetCents != 89200 || !sep.HasPrev || sep.DeltaCents != 89200 {
		t.Fatalf("unexpected September row %+v", sep)
	}

	if err := api.DeleteCashflow(month.Entries[0].ID); err != nil {
		t.Fatal(err)
	}

	if err := api.DeleteCashflow(month.Entries[0].ID); err != errCashflowNotFound {
		t.Fatalf("expected not found, got %v", err)
	}

	month, err = api.CashflowMonth("2026-09")
	if err != nil || len(month.Entries) != 1 {
		t.Fatalf("expected one entry left, got %+v %v", month.Entries, err)
	}
}

func TestCreateCashflowValidates(t *testing.T) {
	api := newTestAPI(t)

	cases := map[string]NewCashflowInput{
		"amount error":       {Currency: "$", Amount: "-1", Date: "2026-09-01", Category: "Rent"},
		"date must use":      {Currency: "$", Amount: "1", Date: "01.09.2026", Category: "Rent"},
		"unknown currency":   {Currency: "BTC", Amount: "1", Date: "2026-09-01", Category: "Rent"},
		"unknown account":    {Currency: "$", Amount: "1", Date: "2026-09-01", Category: "Rent", Account: "Nope"},
		"unknown category":   {Currency: "$", Amount: "1", Date: "2026-09-01", Category: "Salary"},
		"category is requir": {IsIncome: true, Currency: "$", Amount: "1", Date: "2026-09-01"},
	}

	for want, input := range cases {
		if _, err := api.CreateCashflow(input); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("expected error containing %q, got %v", want, err)
		}
	}
}
