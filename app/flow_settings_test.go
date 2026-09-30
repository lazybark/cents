package app

import (
	"github.com/lazybark/cents/flows/settings"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lazybark/cents/flows/account"
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

func TestEditingACategoryKeepsItArchived(t *testing.T) {
	m := newTestApp(t)

	m.settingsCursor = settingsExpenseCategoryAddCursor(m.settings)
	m = press(t, m, "enter", "Car", "enter")

	// Archived in the desktop app.
	car := m.settings.ExpenseCategories[0]
	car.Archived = true
	if err := m.storage.SaveSettingExpenseCategory(&car); err != nil {
		t.Fatal(err)
	}

	m.settings.ExpenseCategories[0] = car
	m.settingsCursor = settingsExpenseCategoryStartCursor(m.settings)
	m = press(t, m, "enter", "s", "enter")
	if m.status != "saved expense category Cars" {
		t.Fatalf("unexpected status %q", m.status)
	}

	stts, err := m.storage.LoadAppSettings()
	if err != nil || len(stts.ExpenseCategories) != 1 || stts.ExpenseCategories[0].CategoryName != "Cars" || !stts.ExpenseCategories[0].Archived || !stts.ExpenseCategories[0].CreatedAt.Equal(car.CreatedAt) {
		t.Fatalf("renaming should keep the category archived and its creation time: %+v %v", stts.ExpenseCategories, err)
	}
}

func TestArchivedAccountsLeaveTUIPickers(t *testing.T) {
	got := accountSelectionOptions([]account.Account{{Name: "Checking"}, {Name: "Old", Archived: true}})
	if len(got) != 2 || got[0] != "" || got[1] != "Checking" {
		t.Fatalf("unexpected options %v", got)
	}
}

func TestEditingAPaymentMethodKeepsItsCurrency(t *testing.T) {
	m := newTestApp(t)

	// A method's currency links one in settings.
	if err := m.storage.SaveSettingCurrency(&settings.SettingCurrency{CurrencyName: "EUR", RateToBase: 1.1}); err != nil {
		t.Fatal(err)
	}

	method := m.settings.PaymentMethods[0]
	method.Currency = "EUR"
	if err := m.storage.SaveSettingPaymentMethod(&method); err != nil {
		t.Fatal(err)
	}

	m.settings.PaymentMethods[0] = method
	if got := storedPaymentMethod(m.settings, method.ID); got.Currency != "EUR" {
		t.Fatalf("editing should start from the stored method: %+v", got)
	}

	if got := storedPaymentMethod(m.settings, 0); got.Currency != "" || got.ID != 0 {
		t.Fatalf("a new method starts empty: %+v", got)
	}
}
