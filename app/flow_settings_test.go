package app

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	storage "github.com/lazybark/cents/storage/sqlite"
)

func newTestApp(t *testing.T) TheApplication {
	t.Helper()

	db, _, err := storage.OpenDatabase(filepath.Join(t.TempDir(), "cents.db"))
	if err != nil {
		t.Fatal(err)
	}

	stts, err := storage.LoadAppSettings(db)
	if err != nil {
		t.Fatal(err)
	}

	m := NewApp(storage.NewSQLiteStorage(db), "cents.db", true, nil, nil, nil, nil, nil, nil, nil, stts)
	m.screen = screenSettings

	return m
}

// press sends keys to the model: "enter", "esc" or text to type.
func press(t *testing.T, m TheApplication, keys ...string) TheApplication {
	t.Helper()

	for _, key := range keys {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		switch key {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		}

		updated, _ := m.Update(msg)
		m = updated.(TheApplication)
	}

	return m
}

func TestSettingsCurrencyFormUsesSharedRules(t *testing.T) {
	m := newTestApp(t)
	m.settingsCursor = len(m.settings.Currencies) + 1

	m = press(t, m, "enter", "GBP", "enter", "0", "enter")
	if m.status != "rate must be greater than zero" {
		t.Fatalf("unexpected status %q", m.status)
	}

	m.settingsCurrencyRateInput.SetValue("1.27")
	m = press(t, m, "enter")
	if m.status != "saved currency GBP" || len(m.settings.Currencies) != 1 || m.settings.Currencies[0].RateToBase != 1.27 {
		t.Fatalf("currency not saved: %q %+v", m.status, m.settings.Currencies)
	}
}

func TestSettingsListFormsUseSharedRules(t *testing.T) {
	m := newTestApp(t)

	m.settingsCursor = settingsExpenseCategoryAddCursor(m.settings)
	m = press(t, m, "enter", "enter")
	if m.status != "expense category name is required" {
		t.Fatalf("unexpected status %q", m.status)
	}

	m = press(t, m, " Rent ", "enter")
	if m.status != "saved expense category Rent" || len(m.settings.ExpenseCategories) != 1 {
		t.Fatalf("category not saved: %q", m.status)
	}

	m.settingsCursor = settingsTaxTypeAddCursor(m.settings)
	m = press(t, m, "enter", "enter", "VAT", "enter", "enter", "enter")
	if m.status != "country is required" {
		t.Fatalf("unexpected status %q", m.status)
	}

	m = press(t, m, "esc")
	m.settingsCursor = settingsPaymentMethodAddCursor(m.settings)
	m = press(t, m, "enter", "Visa", "enter", "enter", "enter")
	if m.status != "saved payment method Visa" || len(m.settings.PaymentMethods) != 2 {
		t.Fatalf("payment method not saved: %q %+v", m.status, m.settings.PaymentMethods)
	}

	for _, method := range m.settings.PaymentMethods {
		if method.PaymentMethodName == "Visa" && method.PaymentMethodType != "Card" {
			t.Fatalf("expected the first type option, got %+v", method)
		}
	}
}
