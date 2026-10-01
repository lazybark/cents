package app

import "testing"

func TestGoalFormsUseSharedRules(t *testing.T) {
	m := newTestApp(t)

	inputs := m.addGoalForm.Inputs
	inputs[0].SetValue("Car")
	inputs[1].SetValue("100")
	inputs[2].SetValue("150")

	updated, _ := m.saveGoalFromForm()
	m = updated.(TheApplication)
	if m.status != "accumulated amount cannot be more than target" {
		t.Fatalf("unexpected status %q", m.status)
	}

	m.addGoalForm.Inputs[2].SetValue("20")
	updated, _ = m.saveGoalFromForm()
	m = updated.(TheApplication)
	if m.status != "saved goal Car" || len(m.goals) != 1 || m.goals[0].AmountAccumulatedCents != 2000 || m.goalMode != goalListActive {
		t.Fatalf("goal not saved: %q %+v", m.status, m.goals)
	}

	m = m.openGoalEditor(m.goals[0]).(TheApplication)
	m.editGoalForm.LogDeltaInput.SetValue("+81")
	updated, _ = m.applyGoalLogDelta()
	m = updated.(TheApplication)
	if m.status != "delta makes accumulated amount out of range" {
		t.Fatalf("unexpected status %q", m.status)
	}

	m.editGoalForm.LogDeltaInput.SetValue("+80")
	updated, _ = m.applyGoalLogDelta()
	m = updated.(TheApplication)
	if m.status != "applied goal log delta" || !m.goals[0].IsDone() || len(m.goalLogs) != 1 || m.goalLogs[0].Note != "manual accumulated adjustment" {
		t.Fatalf("change not applied: %q %+v %+v", m.status, m.goals[0], m.goalLogs)
	}

	if len(m.filteredGoals()) != 0 {
		t.Fatal("a reached goal should leave the active list")
	}

	m.editGoalForm.TargetAmountInput.SetValue("0")
	updated, _ = m.saveGoalEdit()
	m = updated.(TheApplication)
	if m.status != "target amount must be greater than zero" {
		t.Fatalf("unexpected status %q", m.status)
	}
}

func TestInvoiceFormsUseSharedRules(t *testing.T) {
	m := newTestApp(t)

	updated, _ := m.saveInvoiceFromForm()
	m = updated.(TheApplication)
	if m.status != "invoice title is required" {
		t.Fatalf("unexpected status %q", m.status)
	}

	m.addInvoiceForm.Inputs[0].SetValue("Hosting")
	m.addInvoiceForm.Inputs[4].SetValue("2026-01-01")
	updated, _ = m.saveInvoiceFromForm()
	m = updated.(TheApplication)
	if m.status != "due date must use DD.MM.YYYY format" {
		t.Fatalf("unexpected status %q", m.status)
	}

	m.addInvoiceForm.Inputs[4].SetValue("01.02.2026")
	m.addInvoiceForm.Inputs[5].SetValue("Card")
	updated, _ = m.saveInvoiceFromForm()
	m = updated.(TheApplication)
	if m.status != "saved invoice Hosting" || len(m.invoices) != 1 || m.invoices[0].TargetAccount != "Card" || m.invoiceMode != invoiceListIncomingUnpaid {
		t.Fatalf("invoice not saved: %q %+v", m.status, m.invoices)
	}

	m = m.openInvoiceEditor(m.invoices[0]).(TheApplication)
	m.editInvoiceForm.Paid = true
	updated, _ = m.saveInvoiceEdit()
	m = updated.(TheApplication)
	if m.status != "updated invoice Hosting" || !m.invoices[0].Paid || m.invoices[0].DueDate == nil || len(m.filteredInvoices()) != 0 {
		t.Fatalf("invoice not updated: %q %+v", m.status, m.invoices)
	}
}
