package desktop

import (
	"strings"
	"testing"
	"time"
)

func TestDebtLifecycle(t *testing.T) {
	api := newTestAPI(t)

	created, err := api.CreateDebt(NewDebtInput{Peer: "Bank", Currency: "eur", Amount: "1000", AmountPaid: "0", CreatedAt: "2026-01-10", DueDate: "2020-01-01"})
	if err != nil || created.Mode != "outgoing" {
		t.Fatalf("create outgoing: %+v %v", created, err)
	}

	created, err = api.CreateDebt(NewDebtInput{IsOwedToUser: true, Peer: "Alex", Currency: "$", Amount: "50", AmountPaid: "50", CreatedAt: "2026-02-01"})
	if err != nil || created.Mode != "paid" {
		t.Fatalf("a fully paid debt should land in paid: %+v %v", created, err)
	}

	view, err := api.Debts("outgoing")
	if err != nil {
		t.Fatal(err)
	}

	if len(view.Debts) != 1 || view.Counts["outgoing"] != 1 || view.Counts["incoming"] != 0 || view.Counts["paid"] != 1 {
		t.Fatalf("unexpected lists %+v %+v", view.Debts, view.Counts)
	}

	bank := view.Debts[0]
	if bank.Currency != "EUR" || bank.LeftCents != 100000 || !bank.Overdue || bank.CreatedAt != "2026-01-10" || bank.DueDate != "2020-01-01" {
		t.Fatalf("unexpected row %+v", bank)
	}

	if view.Progress.TotalCents != 108000 || view.Progress.PaidCents != 0 {
		t.Fatalf("expected progress in base, got %+v", view.Progress)
	}

	if _, err := api.AddDebtPayment(PaymentInput{ID: bank.ID, Delta: "1001"}); err == nil || err.Error() != "delta makes amount paid out of range" {
		t.Fatalf("expected range error, got %v", err)
	}

	row, err := api.AddDebtPayment(PaymentInput{ID: bank.ID, Delta: "250.50", Date: "2026-03-01", Note: "first"})
	if err != nil || row.PaidCents != 25050 || row.LeftCents != 74950 {
		t.Fatalf("payment: %+v %v", row, err)
	}

	logs, err := api.DebtLogs(bank.ID)
	if err != nil || len(logs) != 1 || logs[0].DeltaCents != 25050 || logs[0].Note != "first" || !strings.HasPrefix(logs[0].When, "2026-03-01 ") {
		t.Fatalf("logs: %+v %v", logs, err)
	}

	if err := api.UpdateDebt(DebtUpdateInput{ID: bank.ID, Amount: "100", AmountPaid: "250.50", CreatedAt: "2026-01-10"}); err == nil || err.Error() != "amount paid cannot be more than amount" {
		t.Fatalf("expected paid > amount error, got %v", err)
	}

	if err := api.UpdateDebt(DebtUpdateInput{ID: bank.ID, Amount: "1000", AmountPaid: "1000", CreatedAt: "2026-01-10", Comment: " settled "}); err != nil {
		t.Fatal(err)
	}

	view, _ = api.Debts("paid")
	if len(view.Debts) != 2 || view.Counts["outgoing"] != 0 {
		t.Fatalf("expected both debts paid, got %+v", view)
	}

	for _, d := range view.Debts {
		if d.Peer == "Bank" && (d.Overdue || d.Comment != "settled" || d.DueDate != "") {
			t.Fatalf("expected paid debt not overdue and due date cleared, got %+v", d)
		}
	}

	if err := api.DeleteDebt(bank.ID); err != nil {
		t.Fatal(err)
	}

	if err := api.DeleteDebt(bank.ID); err != errDebtNotFound {
		t.Fatalf("expected not found, got %v", err)
	}

	if logs, _ := api.DebtLogs(bank.ID); len(logs) != 0 {
		t.Fatalf("expected logs deleted with the debt, got %+v", logs)
	}
}

func TestCreateDebtValidates(t *testing.T) {
	api := newTestAPI(t)
	today := time.Now().Format("2006-01-02")

	for want, input := range map[string]NewDebtInput{
		"peer is required":                        {Currency: "$", Amount: "1", AmountPaid: "0", CreatedAt: today},
		"created date is required":                {Peer: "x", Currency: "$", Amount: "1", AmountPaid: "0"},
		"created date must use YYYY-MM-DD format": {Peer: "x", Currency: "$", Amount: "1", AmountPaid: "0", CreatedAt: "01.01.2026"},
		`unknown currency "BTC"`:                  {Peer: "x", Currency: "BTC", Amount: "1", AmountPaid: "0", CreatedAt: today},
	} {
		if _, err := api.CreateDebt(input); err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("expected %q, got %v", want, err)
		}
	}
}
