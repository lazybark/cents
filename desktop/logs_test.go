package desktop

import (
	"strings"
	"testing"
)

func TestDeleteDebtPaymentUndoesIt(t *testing.T) {
	api := newTestAPI(t)

	if _, err := api.CreateDebt(NewDebtInput{Peer: "Bank", Currency: "$", Amount: "100", AmountPaid: "0", CreatedAt: "2026-01-10"}); err != nil {
		t.Fatal(err)
	}

	if _, err := api.CreateDebt(NewDebtInput{Peer: "Alex", Currency: "$", Amount: "50", AmountPaid: "0", CreatedAt: "2026-01-10"}); err != nil {
		t.Fatal(err)
	}

	view, _ := api.Debts("outgoing")
	bank, alex := view.Debts[0], view.Debts[1]

	for _, delta := range []string{"30", "60"} {
		if _, err := api.AddDebtPayment(PaymentInput{ID: bank.ID, Delta: delta}); err != nil {
			t.Fatal(err)
		}
	}

	logs, _ := api.DebtLogs(bank.ID)
	thirty := logs[1]

	if _, err := api.DeleteDebtPayment(alex.ID, thirty.ID); err != errLogNotFound {
		t.Fatalf("expected another debt's payment to be refused, got %v", err)
	}

	row, err := api.DeleteDebtPayment(bank.ID, thirty.ID)
	if err != nil || row.PaidCents != 6000 {
		t.Fatalf("expected paid to drop by 30, got %+v %v", row, err)
	}

	logs, _ = api.DebtLogs(bank.ID)
	if len(logs) != 1 || logs[0].DeltaCents != 6000 {
		t.Fatalf("expected only the 60 payment left, got %+v", logs)
	}

	// Paid edited down by hand: undoing the 60 payment would go below zero.
	if err := api.UpdateDebt(DebtUpdateInput{ID: bank.ID, Amount: "100", AmountPaid: "10", CreatedAt: "2026-01-10"}); err != nil {
		t.Fatal(err)
	}

	if _, err := api.DeleteDebtPayment(bank.ID, logs[0].ID); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("expected range refusal, got %v", err)
	}

	if logs, _ := api.DebtLogs(bank.ID); len(logs) != 1 {
		t.Fatal("a refused delete must keep the log")
	}
}

func TestDeleteAccountValueLog(t *testing.T) {
	api := newTestAPI(t)
	checking := mustCreate(t, api, NewAccountInput{Name: "Checking", Description: "main", Currency: "$", Amount: "100"})
	savings := mustCreate(t, api, NewAccountInput{Name: "Savings", Description: "rainy", Currency: "$", Amount: "5"})

	for _, date := range []string{"2026-01-01", "2026-02-01"} {
		if err := api.SaveAccountValueLog(ValueLogInput{AccountID: checking.ID, Date: date, Value: "90"}); err != nil {
			t.Fatal(err)
		}
	}

	logs, _ := api.AccountValueLogs(checking.ID)

	if err := api.DeleteAccountValueLog(savings.ID, logs[0].ID); err == nil || err.Error() != "log entry not found" {
		t.Fatalf("expected another account's entry to be refused, got %v", err)
	}

	if err := api.DeleteAccountValueLog(checking.ID, logs[0].ID); err != nil {
		t.Fatal(err)
	}

	remaining, _ := api.AccountValueLogs(checking.ID)
	if len(remaining) != 1 || remaining[0].Date != "2026-01-01" {
		t.Fatalf("expected only January left, got %+v", remaining)
	}

	overview, _ := api.Accounts(int(1))
	for _, acct := range overview.Accounts {
		if acct.ID == checking.ID && acct.BalanceCents != 10000 {
			t.Fatalf("deleting history must not change the balance, got %+v", acct)
		}
	}
}
