package app

import (
	"testing"

	"github.com/lazybark/cents/flows/settings"
)

func TestDebtFormsUseSharedRules(t *testing.T) {
	m := newTestApp(t)

	inputs := m.addDebtForm.Inputs
	inputs[0].SetValue("Bank")
	inputs[1].SetValue("100")
	inputs[2].SetValue("150")
	inputs[3].SetValue("10.01.2026")

	updated, _ := m.saveDebtFromForm()
	m = updated.(TheApplication)
	if m.status != "amount paid cannot be more than amount" {
		t.Fatalf("unexpected status %q", m.status)
	}

	m.addDebtForm.Inputs[2].SetValue("20")
	updated, _ = m.saveDebtFromForm()
	m = updated.(TheApplication)
	if m.status != "saved debt for Bank" || len(m.debts) != 1 || m.debts[0].AmountPaidCents != 2000 || m.debtMode != debtListOutgoing {
		t.Fatalf("debt not saved: %q %+v", m.status, m.debts)
	}

	m = m.openDebtEditor(m.debts[0]).(TheApplication)
	m.editDebtForm.LogDeltaInput.SetValue("+90")
	updated, _ = m.applyDebtLogDelta()
	m = updated.(TheApplication)
	if m.status != "delta makes amount paid out of range" {
		t.Fatalf("unexpected status %q", m.status)
	}

	m.editDebtForm.LogDeltaInput.SetValue("+80")
	updated, _ = m.applyDebtLogDelta()
	m = updated.(TheApplication)
	if m.status != "applied log delta" || !m.debts[0].IsPaid() || len(m.debtLogs) != 1 || m.debtLogs[0].Note != "manual paid adjustment" {
		t.Fatalf("payment not applied: %q %+v %+v", m.status, m.debts[0], m.debtLogs)
	}

	if len(m.filteredDebts()) != 0 {
		t.Fatal("a paid debt should leave the outgoing list")
	}
}

func TestTaxFormsUseSharedRules(t *testing.T) {
	m := newTestApp(t)

	updated, _ := m.saveTaxFromForm()
	m = updated.(TheApplication)
	if m.status != "no tax types configured; add one in settings" {
		t.Fatalf("unexpected status %q", m.status)
	}

	m.addTaxForm.TaxTypeOptions = []settings.SettingTaxType{{ID: 1, Country: "NL", TaxTypeName: "VAT"}}
	m.addTaxForm.Inputs[0].SetValue("0")
	m.addTaxForm.Inputs[2].SetValue("Q1")
	updated, _ = m.saveTaxFromForm()
	m = updated.(TheApplication)
	if m.status != "amount due must be greater than zero" {
		t.Fatalf("unexpected status %q", m.status)
	}

	m.addTaxForm.Inputs[0].SetValue("100")
	m.addTaxForm.Inputs[1].SetValue("0")
	updated, _ = m.saveTaxFromForm()
	m = updated.(TheApplication)
	if m.status != "saved tax NL / VAT" || len(m.taxes) != 1 || m.taxes[0].Period != "Q1" || m.taxes[0].Currency != "$" || m.taxes[0].RateToBase != 1 || m.taxes[0].AmountDueBaseCents != 10000 {
		t.Fatalf("tax not saved: %q %+v", m.status, m.taxes)
	}

	m = m.openTaxEditor(m.taxes[0]).(TheApplication)
	m.editTaxForm.LogDeltaInput.SetValue("150")
	updated, _ = m.applyTaxLogDelta()
	m = updated.(TheApplication)
	if m.status != "applied tax log delta" || m.taxes[0].AmountPaidCents != 15000 {
		t.Fatalf("overpayment should apply: %q %+v", m.status, m.taxes[0])
	}
}
