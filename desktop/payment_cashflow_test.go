package desktop

import (
	"strings"
	"testing"
)

func TestPaymentsAddedToCashflow(t *testing.T) {
	api := newTestAPI(t)
	mustCreate(t, api, NewAccountInput{Name: "Checking", Description: "main", Currency: "$", Amount: "0"})

	if _, err := api.CreateDebt(NewDebtInput{Peer: "Bank", Currency: "EUR", Rate: "1.2", Amount: "1000", AmountPaid: "0", CreatedAt: "2026-01-01"}); err != nil {
		t.Fatal(err)
	}

	if _, err := api.CreateDebt(NewDebtInput{IsOwedToUser: true, Peer: "Alex", Currency: "$", Amount: "50", AmountPaid: "0", CreatedAt: "2026-01-01"}); err != nil {
		t.Fatal(err)
	}

	debts, _ := api.Debts("outgoing")
	bank := debts.Debts[0]
	if len(debts.Cashflow.ExpenseCategories) == 0 || len(debts.Cashflow.Accounts) != 1 {
		t.Fatalf("debts should offer cashflow options: %+v", debts.Cashflow)
	}

	month := func(m string) []CashflowRow {
		t.Helper()
		data, err := api.CashflowMonth(m)
		if err != nil {
			t.Fatal(err)
		}
		return data.Entries
	}

	for want, req := range map[string]PaymentCashflow{
		"can't add it as an expense: category is required":      {Add: true},
		`can't add it as an expense: unknown category "Salary"`: {Add: true, Category: "Salary"},
		`unknown account "Savings"`:                             {Add: true, Category: "Rent", Account: "Savings"},
	} {
		if _, err := api.AddDebtPayment(PaymentInput{ID: bank.ID, Delta: "100", Date: "2026-03-05", Cashflow: req}); err == nil || err.Error() != want {
			t.Errorf("want %q, got %v", want, err)
		}
	}

	// A refused entry leaves the payment unsaved too.
	if logs, _ := api.DebtLogs(bank.ID); len(logs) != 0 || len(month("2026-03")) != 0 {
		t.Fatalf("nothing should be saved yet: %+v", logs)
	}

	row, err := api.AddDebtPayment(PaymentInput{ID: bank.ID, Delta: "100", Date: "2026-03-05", Note: "march", Cashflow: PaymentCashflow{Add: true, Category: "Rent", Account: "checking"}})
	if err != nil || row.PaidCents != 10000 {
		t.Fatalf("payment: %+v %v", row, err)
	}

	entries := month("2026-03")
	if len(entries) != 1 || entries[0].IsIncome || entries[0].AmountCents != 10000 || entries[0].Currency != "EUR" || entries[0].RateToBase != 1.08 || entries[0].Category != "Rent" || entries[0].Account != "Checking" || entries[0].Comment != "Payment to Bank: march" || entries[0].Date != "2026-03-05" {
		t.Fatalf("unexpected expense %+v", entries)
	}

	logs, _ := api.DebtLogs(bank.ID)
	if len(logs) != 1 || logs[0].CashflowID != entries[0].ID {
		t.Fatalf("the payment should know its expense: %+v", logs)
	}

	if _, err := api.AddDebtPayment(PaymentInput{ID: bank.ID, Delta: "-5", Cashflow: PaymentCashflow{Add: true, Category: "Rent"}}); err == nil || !strings.Contains(err.Error(), "not one taken back") {
		t.Fatalf("expected a taken-back payment to be refused, got %v", err)
	}

	// Without the box ticked nothing is added.
	if _, err := api.AddDebtPayment(PaymentInput{ID: bank.ID, Delta: "50", Date: "2026-03-06"}); err != nil {
		t.Fatal(err)
	}

	if len(month("2026-03")) != 1 {
		t.Fatal("an unticked payment added an entry")
	}

	// Deleting the payment deletes its expense.
	if _, err := api.DeleteDebtPayment(bank.ID, logs[0].ID); err != nil {
		t.Fatal(err)
	}

	if len(month("2026-03")) != 0 {
		t.Fatal("the payment's expense should go with it")
	}

	// A debt owed to me makes an income.
	incoming, _ := api.Debts("incoming")
	alex := incoming.Debts[0]
	if _, err := api.AddDebtPayment(PaymentInput{ID: alex.ID, Delta: "20", Date: "2026-04-01", Cashflow: PaymentCashflow{Add: true, Category: "Salary"}}); err != nil {
		t.Fatal(err)
	}

	if e := month("2026-04"); len(e) != 1 || !e[0].IsIncome || e[0].Comment != "Repayment from Alex" || e[0].Category != "Salary" {
		t.Fatalf("unexpected income %+v", e)
	}
}

func TestCreditPaymentAddedAsExpense(t *testing.T) {
	api := newTestAPI(t)

	if _, err := api.CreateCredit(CreditInput{Currency: "$", Name: "Car loan", Total: "1000", StartDate: "2026-01-01"}); err != nil {
		t.Fatal(err)
	}

	credits, _ := api.Credits("active")
	car := credits.Credits[0]
	if len(credits.Cashflow.ExpenseCategories) == 0 {
		t.Fatalf("credits should offer expense categories: %+v", credits.Cashflow)
	}

	if _, err := api.AddCreditLog(CreditLogInput{ID: car.ID, Kind: "addition", Amount: "10", Cashflow: PaymentCashflow{Add: true, Category: "Rent"}}); err == nil || err.Error() != "only a payment can be added as an expense" {
		t.Fatalf("expected an addition to be refused, got %v", err)
	}

	if _, err := api.AddCreditLog(CreditLogInput{ID: car.ID, Kind: "payment", Amount: "200", Date: "2026-05-10", Cashflow: PaymentCashflow{Add: true, Category: "Rent"}}); err != nil {
		t.Fatal(err)
	}

	data, _ := api.CashflowMonth("2026-05")
	if len(data.Entries) != 1 || data.Entries[0].AmountCents != 20000 || data.Entries[0].Comment != "Payment on credit Car loan" || data.Entries[0].IsIncome {
		t.Fatalf("unexpected expense %+v", data.Entries)
	}

	logs, _ := api.CreditLogs(car.ID)
	if len(logs) != 1 || logs[0].CashflowID == 0 {
		t.Fatalf("the payment should know its expense: %+v", logs)
	}

	if _, err := api.DeleteCreditLog(car.ID, logs[0].ID); err != nil {
		t.Fatal(err)
	}

	if data, _ := api.CashflowMonth("2026-05"); len(data.Entries) != 0 {
		t.Fatal("the payment's expense should go with it")
	}
}
