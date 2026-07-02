package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lazybark/cents/flows/export"
	"github.com/lazybark/cents/flows/settings"
	sqliteStorage "github.com/lazybark/cents/storage/sqlite"
)

func (m TheApplication) renderSettings(width int) string {
	header := []string{sectionTitleStyle.Render("Settings"), hintStyle.Render("Use up/down to select rows. Enter edits selected row. Esc returns to menu."), ""}
	header = append(header, sectionTitleStyle.Render("General"))

	basePrefix := " "
	if m.settingsCursor == 0 {
		basePrefix = ">"
	}

	baseCurrencyValue := m.settings.BaseCurrency

	if m.settingsEditMode == settingsEditBaseCurrency {
		baseCurrencyValue = m.settingsEditInput.View()
	}

	rows := []string{fmt.Sprintf("%s %-18s %s", basePrefix, "base_currency", baseCurrencyValue), "", sectionTitleStyle.Render("Currencies (relation to base currency)")}

	for index, currency := range m.settings.Currencies {
		prefix := " "
		if m.settingsCursor == index+1 {
			prefix = ">"
		}

		rows = append(rows, fmt.Sprintf("%s %-16s rate=%s", prefix, truncateText(currency.CurrencyName, 16), formatRate(currency.RateToBase)))
	}

	addPrefix := " "
	if m.settingsCursor == len(m.settings.Currencies)+1 {
		addPrefix = ">"
	}

	rows = append(rows, fmt.Sprintf("%s + add currency", addPrefix))
	rows = append(rows, "", sectionTitleStyle.Render("Payment methods"))
	paymentStart := settingsPaymentMethodStartCursor(m.settings)

	for index, method := range m.settings.PaymentMethods {
		prefix := " "
		cursor := paymentStart + index

		if m.settingsCursor == cursor {
			prefix = ">"
		}

		defaultTag := ""

		if method.IsDefault {
			defaultTag = " [default]"
		}

		rows = append(rows, fmt.Sprintf("%s %-16s %-10s%s", prefix, truncateText(method.PaymentMethodName, 16), truncateText(method.PaymentMethodType, 10), defaultTag))
	}

	methodAddPrefix := " "

	if m.settingsCursor == settingsPaymentMethodAddCursor(m.settings) {
		methodAddPrefix = ">"
	}

	rows = append(rows, fmt.Sprintf("%s + add payment method", methodAddPrefix))
	rows = append(rows, "", sectionTitleStyle.Render("Tax types"))
	taxStart := settingsTaxTypeStartCursor(m.settings)

	for index, taxType := range m.settings.TaxTypes {
		prefix := " "
		cursor := taxStart + index

		if m.settingsCursor == cursor {
			prefix = ">"
		}

		description := strings.TrimSpace(taxType.Description)
		if description == "" {
			description = "-"
		}

		rows = append(rows, fmt.Sprintf("%s %-14s %-16s %s", prefix, truncateText(taxType.Country, 14), truncateText(taxType.TaxTypeName, 16), truncateText(description, 26)))
	}

	taxAddPrefix := " "
	if m.settingsCursor == settingsTaxTypeAddCursor(m.settings) {
		taxAddPrefix = ">"
	}

	rows = append(rows, fmt.Sprintf("%s + add tax type", taxAddPrefix))
	rows = append(rows, "", sectionTitleStyle.Render("Income categories"))
	incomeStart := settingsIncomeCategoryStartCursor(m.settings)

	for index, category := range m.settings.IncomeCategories {
		prefix := " "
		cursor := incomeStart + index

		if m.settingsCursor == cursor {
			prefix = ">"
		}

		rows = append(rows, fmt.Sprintf("%s %-24s", prefix, truncateText(category.CategoryName, 24)))
	}

	incomeAddPrefix := " "

	if m.settingsCursor == settingsIncomeCategoryAddCursor(m.settings) {
		incomeAddPrefix = ">"
	}

	rows = append(rows, fmt.Sprintf("%s + add income category", incomeAddPrefix))
	rows = append(rows, "", sectionTitleStyle.Render("Expense categories"))
	expenseStart := settingsExpenseCategoryStartCursor(m.settings)

	for index, category := range m.settings.ExpenseCategories {
		prefix := " "
		cursor := expenseStart + index

		if m.settingsCursor == cursor {
			prefix = ">"
		}

		rows = append(rows, fmt.Sprintf("%s %-24s", prefix, truncateText(category.CategoryName, 24)))
	}

	expenseAddPrefix := " "

	if m.settingsCursor == settingsExpenseCategoryAddCursor(m.settings) {
		expenseAddPrefix = ">"
	}

	rows = append(rows, fmt.Sprintf("%s + add expense category", expenseAddPrefix))

	if m.settingsEditMode == settingsEditBaseCurrency {
		rows = append(rows, "", mutedStyle.Render("Editing base currency: Enter saves, Esc exits settings."))
	}

	if m.settingsEditMode == settingsEditCurrency {
		namePrefix := " "
		ratePrefix := " "

		if m.settingsCurrencyField == 0 {
			namePrefix = ">"
		} else {
			ratePrefix = ">"
		}

		rows = append(rows, "")
		rows = append(rows, fieldLabelStyle.Render("Currency form"))
		rows = append(rows, fmt.Sprintf("%s Name  %s", namePrefix, m.settingsCurrencyNameInput.View()))
		rows = append(rows, fmt.Sprintf("%s Rate  %s", ratePrefix, m.settingsCurrencyRateInput.View()))
		rows = append(rows, mutedStyle.Render("Enter on Name moves to Rate. Enter on Rate saves. Esc exits settings."))
	}

	if m.settingsEditMode == settingsEditPaymentMethod {
		namePrefix := " "
		typePrefix := " "
		defaultPrefix := " "

		if m.settingsPaymentMethodField == 0 {
			namePrefix = ">"
		}

		if m.settingsPaymentMethodField == 1 {
			typePrefix = ">"
		}

		if m.settingsPaymentMethodField == 2 {
			defaultPrefix = ">"
		}

		typeOptions := make([]string, 0, len(m.settingsPaymentMethodTypeOptions))
		for i, option := range m.settingsPaymentMethodTypeOptions {
			style := buttonStyle

			if i == m.settingsPaymentMethodTypeIndex {
				style = buttonActiveStyle
			}

			typeOptions = append(typeOptions, style.Render(option))
		}

		defaultMarker := "[ ]"

		if m.settingsPaymentMethodIsDefault {
			defaultMarker = "[x]"
		}

		rows = append(rows, "")
		rows = append(rows, fieldLabelStyle.Render("Payment method form"))
		rows = append(rows, fmt.Sprintf("%s Name     %s", namePrefix, m.settingsPaymentMethodNameInput.View()))
		rows = append(rows, fmt.Sprintf("%s Type     %s", typePrefix, strings.Join(typeOptions, " ")))
		rows = append(rows, fmt.Sprintf("%s Default  %s", defaultPrefix, defaultMarker))
		rows = append(rows, mutedStyle.Render("Enter/Tab moves fields. Left/right changes type. Space toggles default. Enter on Default saves. Esc exits settings."))
	}

	if m.settingsEditMode == settingsEditTaxType {
		countryPrefix := " "
		namePrefix := " "
		descPrefix := " "
		urlPrefix := " "

		if m.settingsTaxTypeField == 0 {
			countryPrefix = ">"
		}

		if m.settingsTaxTypeField == 1 {
			namePrefix = ">"
		}

		if m.settingsTaxTypeField == 2 {
			descPrefix = ">"
		}

		if m.settingsTaxTypeField == 3 {
			urlPrefix = ">"
		}

		rows = append(rows, "")
		rows = append(rows, fieldLabelStyle.Render("Tax type form"))
		rows = append(rows, fmt.Sprintf("%s Country      %s", countryPrefix, m.settingsTaxTypeCountryInput.View()))
		rows = append(rows, fmt.Sprintf("%s Tax type     %s", namePrefix, m.settingsTaxTypeNameInput.View()))
		rows = append(rows, fmt.Sprintf("%s Description  %s", descPrefix, m.settingsTaxTypeDescriptionInput.View()))
		rows = append(rows, fmt.Sprintf("%s URL          %s", urlPrefix, m.settingsTaxTypeURLInput.View()))
		rows = append(rows, mutedStyle.Render("Enter/Tab moves fields. Enter on URL saves. Esc exits settings."))
	}

	if m.settingsEditMode == settingsEditIncomeCategory {
		rows = append(rows, "")
		rows = append(rows, fieldLabelStyle.Render("Income category form"))
		rows = append(rows, "> Name  "+m.settingsIncomeCategoryNameInput.View())
		rows = append(rows, mutedStyle.Render("Enter saves. Esc exits edit."))
	}

	if m.settingsEditMode == settingsEditExpenseCategory {
		rows = append(rows, "")
		rows = append(rows, fieldLabelStyle.Render("Expense category form"))
		rows = append(rows, "> Name  "+m.settingsExpenseCategoryNameInput.View())
		rows = append(rows, mutedStyle.Render("Enter saves. Esc exits edit."))
	}

	if m.settingsDeleteConfirm {
		yesStyle := buttonStyle
		noStyle := buttonStyle

		if m.settingsDeleteChoice == 0 {
			yesStyle = buttonActiveStyle
		} else {
			noStyle = buttonActiveStyle
		}

		rows = append(rows, "")
		rows = append(rows, fieldLabelStyle.Render("Confirm delete"))
		rows = append(rows, fmt.Sprintf("Are you sure you want to delete %s?", m.settingsDeleteTargetName))
		rows = append(rows, yesStyle.Render("Yes")+" "+noStyle.Render("No"))
		rows = append(rows, mutedStyle.Render("Left/Right selects. Enter confirms. Esc cancels."))
	}

	content := append(header, rows...)

	return panelStyle.Width(width).Render(strings.Join(content, "\n"))
}

func (m TheApplication) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.settingsDeleteConfirm {
		switch msg.String() {
		case "esc":
			m.settingsDeleteConfirm = false
			m.status = "delete cancelled"

			return m, nil
		case "left", "h", "shift+tab":
			m.settingsDeleteChoice = 0

			return m, nil
		case "right", "l", "tab":
			m.settingsDeleteChoice = 1

			return m, nil
		case "enter":
			if m.settingsDeleteChoice == 1 {
				m.settingsDeleteConfirm = false
				m.status = "delete cancelled"

				return m, nil
			}

			if err := m.storage.DeleteSetting(m.settingsDeleteTargetType, m.settingsDeleteTargetID); err != nil {
				m.status = "settings delete failed: " + err.Error()

				return m, nil
			}

			updated, err := m.storage.LoadAppSettings()
			if err != nil {
				m.status = "settings reload failed: " + err.Error()

				return m, nil
			}

			m.settings = updated
			m.accounts = sortAccounts(m.accounts, m.settings, m.accountSortField)
			deletedName := m.settingsDeleteTargetName
			deletedType := m.settingsDeleteTargetType
			m.settingsDeleteConfirm = false
			m.settingsDeleteTargetType = ""
			m.settingsDeleteTargetID = 0
			m.settingsDeleteTargetName = ""
			m.settingsDeleteChoice = 1
			m.settingsCursor = 0
			m.status = "deleted " + deletedType + " " + deletedName

			return m, nil
		}

		return m, nil
	}

	if m.settingsEditMode == settingsEditBaseCurrency {
		switch msg.String() {
		case "esc":
			m.settingsEditMode = settingsEditNone
			m.settingsEditInput.Blur()
			m.screen = screenMenu
			m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

			return m, nil
		case "enter":
			value := strings.TrimSpace(m.settingsEditInput.Value())
			if value == "" {
				m.status = "base currency cannot be empty"

				return m, nil
			}

			record := settings.SettingRecord{SettingID: "base_currency", SettingValue: value}
			if err := m.storage.SaveSettingRecord(&record); err != nil {
				m.status = "settings save failed: " + err.Error()

				return m, nil
			}

			updated, err := m.storage.LoadAppSettings()
			if err != nil {
				m.status = "settings reload failed: " + err.Error()

				return m, nil
			}

			m.settings = updated
			m.accounts = sortAccounts(m.accounts, m.settings, m.accountSortField)
			m.settingsEditMode = settingsEditNone
			m.settingsEditInput.Blur()
			m.status = "saved setting base_currency"

			return m, nil
		}

		var cmd tea.Cmd

		m.settingsEditInput, cmd = m.settingsEditInput.Update(msg)

		return m, cmd
	}

	if m.settingsEditMode == settingsEditCurrency {
		switch msg.String() {
		case "esc":
			m.settingsEditMode = settingsEditNone
			m.settingsCurrencyNameInput.Blur()
			m.settingsCurrencyRateInput.Blur()
			m.screen = screenMenu
			m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

			return m, nil
		case "tab", "down":
			m.settingsCurrencyField = 1
			m = m.focusCurrencyFormField()

			return m, nil
		case "shift+tab", "up":
			m.settingsCurrencyField = 0
			m = m.focusCurrencyFormField()

			return m, nil
		case "enter":
			if m.settingsCurrencyField == 0 {
				m.settingsCurrencyField = 1
				m = m.focusCurrencyFormField()

				return m, nil
			}

			name := strings.TrimSpace(m.settingsCurrencyNameInput.Value())
			rateRaw := strings.TrimSpace(m.settingsCurrencyRateInput.Value())
			if name == "" {
				m.status = "currency name is required"

				return m, nil
			}

			rate, err := strconv.ParseFloat(rateRaw, 64)
			if err != nil {
				m.status = "rate must be a number"

				return m, nil
			}
			if rate <= 0 {
				m.status = "rate must be greater than zero"

				return m, nil
			}

			now := time.Now()
			record := settings.SettingCurrency{
				ID:            m.settingsCurrencyEditingID,
				CurrencyName:  name,
				RateToBase:    rate,
				LastUpdatedAt: now,
			}
			if record.ID == 0 {
				record.CreatedAt = now
			}

			if err := m.storage.SaveSettingCurrency(&record); err != nil {
				m.status = "currency save failed: " + err.Error()

				return m, nil
			}

			updated, err := m.storage.LoadAppSettings()
			if err != nil {
				m.status = "settings reload failed: " + err.Error()

				return m, nil
			}

			m.settings = updated
			m.accounts = sortAccounts(m.accounts, m.settings, m.accountSortField)
			m.settingsEditMode = settingsEditNone
			m.settingsCurrencyNameInput.Blur()
			m.settingsCurrencyRateInput.Blur()
			m.settingsCurrencyEditingID = 0
			m.settingsCurrencyField = 0
			m.settingsCursor = settingsCursorByName(m.settings.Currencies, name)
			m.status = "saved currency " + name

			return m, nil
		}

		var cmd tea.Cmd

		if m.settingsCurrencyField == 0 {
			m.settingsCurrencyNameInput, cmd = m.settingsCurrencyNameInput.Update(msg)

			return m, cmd
		}

		m.settingsCurrencyRateInput, cmd = m.settingsCurrencyRateInput.Update(msg)

		return m, cmd
	}

	if m.settingsEditMode == settingsEditPaymentMethod {
		switch msg.String() {
		case "esc":
			m.settingsEditMode = settingsEditNone
			m.settingsPaymentMethodNameInput.Blur()
			m.screen = screenMenu
			m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

			return m, nil
		case "tab", "down":
			if m.settingsPaymentMethodField < 2 {
				m.settingsPaymentMethodField++
			}
			m = m.focusPaymentMethodFormField()

			return m, nil
		case "shift+tab", "up":
			if m.settingsPaymentMethodField > 0 {
				m.settingsPaymentMethodField--
			}
			m = m.focusPaymentMethodFormField()

			return m, nil
		case "left":
			if m.settingsPaymentMethodField == 1 && m.settingsPaymentMethodTypeIndex > 0 {
				m.settingsPaymentMethodTypeIndex--
			}

			return m, nil
		case "right":
			if m.settingsPaymentMethodField == 1 && m.settingsPaymentMethodTypeIndex < len(m.settingsPaymentMethodTypeOptions)-1 {
				m.settingsPaymentMethodTypeIndex++
			}

			return m, nil
		case " ":
			if m.settingsPaymentMethodField == 2 {
				m.settingsPaymentMethodIsDefault = !m.settingsPaymentMethodIsDefault

				return m, nil
			}
		case "enter":
			if m.settingsPaymentMethodField < 2 {
				m.settingsPaymentMethodField++
				m = m.focusPaymentMethodFormField()

				return m, nil
			}

			name := strings.TrimSpace(m.settingsPaymentMethodNameInput.Value())
			if name == "" {
				m.status = "payment method name is required"

				return m, nil
			}

			methodType := selectedPaymentMethodType(m.settingsPaymentMethodTypeOptions, m.settingsPaymentMethodTypeIndex)
			isDefault := m.settingsPaymentMethodIsDefault
			if len(m.settings.PaymentMethods) == 0 && m.settingsPaymentMethodEditingID == 0 {
				isDefault = true
			}

			now := time.Now()

			record := settings.SettingPaymentMethod{
				ID:                m.settingsPaymentMethodEditingID,
				PaymentMethodName: name,
				PaymentMethodType: methodType,
				IsDefault:         isDefault,
				LastUpdatedAt:     now,
			}
			if record.ID == 0 {
				record.CreatedAt = now
			}

			if err := m.storage.SaveSettingPaymentMethod(&record); err != nil {
				m.status = "payment method save failed: " + err.Error()

				return m, nil
			}

			updated, err := m.storage.LoadAppSettings()
			if err != nil {
				m.status = "settings reload failed: " + err.Error()

				return m, nil
			}

			m.settings = updated
			m.settingsEditMode = settingsEditNone
			m.settingsPaymentMethodNameInput.Blur()
			m.settingsPaymentMethodEditingID = 0
			m.settingsPaymentMethodField = 0
			m.settingsCursor = settingsCursorByPaymentMethodName(m.settings, name)
			m.status = "saved payment method " + name

			return m, nil
		}

		var cmd tea.Cmd

		if m.settingsPaymentMethodField == 0 {
			m.settingsPaymentMethodNameInput, cmd = m.settingsPaymentMethodNameInput.Update(msg)

			return m, cmd
		}

		return m, nil
	}

	if m.settingsEditMode == settingsEditTaxType {
		switch msg.String() {
		case "esc":
			m.settingsEditMode = settingsEditNone
			m.settingsTaxTypeCountryInput.Blur()
			m.settingsTaxTypeNameInput.Blur()
			m.settingsTaxTypeDescriptionInput.Blur()
			m.settingsTaxTypeURLInput.Blur()
			m.settingsTaxTypeEditingID = 0
			m.settingsTaxTypeField = 0
			m.status = "tax type edit cancelled"

			return m, nil
		case "down", "tab":
			if m.settingsTaxTypeField < 3 {
				m.settingsTaxTypeField++
			}
			m = m.focusTaxTypeFormField()

			return m, nil
		case "up", "shift+tab":
			if m.settingsTaxTypeField > 0 {
				m.settingsTaxTypeField--
			}
			m = m.focusTaxTypeFormField()

			return m, nil
		case "enter":
			if m.settingsTaxTypeField < 3 {
				m.settingsTaxTypeField++
				m = m.focusTaxTypeFormField()

				return m, nil
			}

			country := strings.TrimSpace(m.settingsTaxTypeCountryInput.Value())
			name := strings.TrimSpace(m.settingsTaxTypeNameInput.Value())
			description := strings.TrimSpace(m.settingsTaxTypeDescriptionInput.Value())
			url := strings.TrimSpace(m.settingsTaxTypeURLInput.Value())
			if country == "" {
				m.status = "country is required"

				return m, nil
			}

			if name == "" {
				m.status = "tax type name is required"

				return m, nil
			}

			now := time.Now()

			record := settings.SettingTaxType{
				ID:            m.settingsTaxTypeEditingID,
				Country:       country,
				TaxTypeName:   name,
				Description:   description,
				URL:           url,
				LastUpdatedAt: now,
			}
			if record.ID == 0 {
				record.CreatedAt = now
			}

			if err := m.storage.SaveSettingTaxType(&record); err != nil {
				m.status = "tax type save failed: " + err.Error()

				return m, nil
			}

			updated, err := m.storage.LoadAppSettings()
			if err != nil {
				m.status = "settings reload failed: " + err.Error()

				return m, nil
			}

			m.settings = updated
			m.settingsEditMode = settingsEditNone
			m.settingsTaxTypeCountryInput.Blur()
			m.settingsTaxTypeNameInput.Blur()
			m.settingsTaxTypeDescriptionInput.Blur()
			m.settingsTaxTypeURLInput.Blur()
			m.settingsTaxTypeEditingID = 0
			m.settingsTaxTypeField = 0
			m.settingsCursor = settingsCursorByTaxTypeID(m.settings, record.ID, country, name)
			m.status = "saved tax type " + country + " / " + name

			return m, nil
		}

		var cmd tea.Cmd
		switch m.settingsTaxTypeField {
		case 0:
			m.settingsTaxTypeCountryInput, cmd = m.settingsTaxTypeCountryInput.Update(msg)
		case 1:
			m.settingsTaxTypeNameInput, cmd = m.settingsTaxTypeNameInput.Update(msg)
		case 2:
			m.settingsTaxTypeDescriptionInput, cmd = m.settingsTaxTypeDescriptionInput.Update(msg)
		case 3:
			m.settingsTaxTypeURLInput, cmd = m.settingsTaxTypeURLInput.Update(msg)
		}
		return m, cmd
	}

	if m.settingsEditMode == settingsEditIncomeCategory {
		switch msg.String() {
		case "esc":
			m.settingsEditMode = settingsEditNone
			m.settingsIncomeCategoryNameInput.Blur()
			m.settingsIncomeCategoryEditingID = 0
			m.status = "income category edit cancelled"
			return m, nil
		case "enter":
			name := strings.TrimSpace(m.settingsIncomeCategoryNameInput.Value())
			if name == "" {
				m.status = "income category name is required"
				return m, nil
			}
			now := time.Now()
			record := settings.SettingIncomeCategory{ID: m.settingsIncomeCategoryEditingID, CategoryName: name, LastUpdatedAt: now}
			if record.ID == 0 {
				record.CreatedAt = now
			}
			if err := m.storage.SaveSettingIncomeCategory(&record); err != nil {
				m.status = "income category save failed: " + err.Error()
				return m, nil
			}
			updated, err := m.storage.LoadAppSettings()
			if err != nil {
				m.status = "settings reload failed: " + err.Error()
				return m, nil
			}
			m.settings = updated
			m.settingsEditMode = settingsEditNone
			m.settingsIncomeCategoryNameInput.Blur()
			m.settingsIncomeCategoryEditingID = 0
			m.settingsCursor = settingsCursorByIncomeCategoryName(m.settings, name)
			m.status = "saved income category " + name
			return m, nil
		}

		var cmd tea.Cmd
		m.settingsIncomeCategoryNameInput, cmd = m.settingsIncomeCategoryNameInput.Update(msg)
		return m, cmd
	}

	if m.settingsEditMode == settingsEditExpenseCategory {
		switch msg.String() {
		case "esc":
			m.settingsEditMode = settingsEditNone
			m.settingsExpenseCategoryNameInput.Blur()
			m.settingsExpenseCategoryEditingID = 0
			m.status = "expense category edit cancelled"
			return m, nil
		case "enter":
			name := strings.TrimSpace(m.settingsExpenseCategoryNameInput.Value())
			if name == "" {
				m.status = "expense category name is required"
				return m, nil
			}
			now := time.Now()
			record := settings.SettingExpenseCategory{ID: m.settingsExpenseCategoryEditingID, CategoryName: name, LastUpdatedAt: now}
			if record.ID == 0 {
				record.CreatedAt = now
			}
			if err := m.storage.SaveSettingExpenseCategory(&record); err != nil {
				m.status = "expense category save failed: " + err.Error()
				return m, nil
			}
			updated, err := m.storage.LoadAppSettings()
			if err != nil {
				m.status = "settings reload failed: " + err.Error()
				return m, nil
			}
			m.settings = updated
			m.settingsEditMode = settingsEditNone
			m.settingsExpenseCategoryNameInput.Blur()
			m.settingsExpenseCategoryEditingID = 0
			m.settingsCursor = settingsCursorByExpenseCategoryName(m.settings, name)
			m.status = "saved expense category " + name
			return m, nil
		}

		var cmd tea.Cmd
		m.settingsExpenseCategoryNameInput, cmd = m.settingsExpenseCategoryNameInput.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		return m, nil
	case "backspace", "delete":
		paymentStart := settingsPaymentMethodStartCursor(m.settings)
		paymentAdd := settingsPaymentMethodAddCursor(m.settings)
		taxStart := settingsTaxTypeStartCursor(m.settings)
		taxAdd := settingsTaxTypeAddCursor(m.settings)
		incomeStart := settingsIncomeCategoryStartCursor(m.settings)
		incomeAdd := settingsIncomeCategoryAddCursor(m.settings)
		expenseStart := settingsExpenseCategoryStartCursor(m.settings)
		expenseAdd := settingsExpenseCategoryAddCursor(m.settings)
		if m.settingsCursor > 0 && m.settingsCursor <= len(m.settings.Currencies) {
			selected := m.settings.Currencies[m.settingsCursor-1]
			m.settingsDeleteConfirm = true
			m.settingsDeleteTargetType = "currency"
			m.settingsDeleteTargetID = selected.ID
			m.settingsDeleteTargetName = selected.CurrencyName
			m.settingsDeleteChoice = 1
			m.status = "confirm currency delete"
			return m, nil
		}
		if m.settingsCursor >= paymentStart && m.settingsCursor < paymentAdd {
			selected := m.settings.PaymentMethods[m.settingsCursor-paymentStart]
			m.settingsDeleteConfirm = true
			m.settingsDeleteTargetType = "payment_method"
			m.settingsDeleteTargetID = selected.ID
			m.settingsDeleteTargetName = selected.PaymentMethodName
			m.settingsDeleteChoice = 1
			m.status = "confirm payment method delete"
			return m, nil
		}
		if m.settingsCursor >= taxStart && m.settingsCursor < taxAdd {
			selected := m.settings.TaxTypes[m.settingsCursor-taxStart]
			m.settingsDeleteConfirm = true
			m.settingsDeleteTargetType = "tax_type"
			m.settingsDeleteTargetID = selected.ID
			m.settingsDeleteTargetName = strings.TrimSpace(selected.Country + " / " + selected.TaxTypeName)
			m.settingsDeleteChoice = 1
			m.status = "confirm tax type delete"
			return m, nil
		}
		if m.settingsCursor >= incomeStart && m.settingsCursor < incomeAdd {
			selected := m.settings.IncomeCategories[m.settingsCursor-incomeStart]
			m.settingsDeleteConfirm = true
			m.settingsDeleteTargetType = "income_category"
			m.settingsDeleteTargetID = selected.ID
			m.settingsDeleteTargetName = selected.CategoryName
			m.settingsDeleteChoice = 1
			m.status = "confirm income category delete"
			return m, nil
		}
		if m.settingsCursor >= expenseStart && m.settingsCursor < expenseAdd {
			selected := m.settings.ExpenseCategories[m.settingsCursor-expenseStart]
			m.settingsDeleteConfirm = true
			m.settingsDeleteTargetType = "expense_category"
			m.settingsDeleteTargetID = selected.ID
			m.settingsDeleteTargetName = selected.CategoryName
			m.settingsDeleteChoice = 1
			m.status = "confirm expense category delete"
			return m, nil
		}
		return m, nil
	case "up":
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
		return m, nil
	case "down":
		maxCursor := settingsExpenseCategoryAddCursor(m.settings)
		if m.settingsCursor < maxCursor {
			m.settingsCursor++
		}
		return m, nil
	case "enter":
		paymentStart := settingsPaymentMethodStartCursor(m.settings)
		paymentAdd := settingsPaymentMethodAddCursor(m.settings)
		taxStart := settingsTaxTypeStartCursor(m.settings)
		taxAdd := settingsTaxTypeAddCursor(m.settings)
		incomeStart := settingsIncomeCategoryStartCursor(m.settings)
		incomeAdd := settingsIncomeCategoryAddCursor(m.settings)
		expenseStart := settingsExpenseCategoryStartCursor(m.settings)
		expenseAdd := settingsExpenseCategoryAddCursor(m.settings)

		if m.settingsCursor == 0 {
			m.settingsEditMode = settingsEditBaseCurrency
			m.settingsEditInput.SetValue(m.settings.BaseCurrency)
			m.settingsEditInput.Focus()
			m.status = "editing setting base_currency"
			return m, nil
		}

		if m.settingsCursor == len(m.settings.Currencies)+1 {
			m.settingsEditMode = settingsEditCurrency
			m.settingsCurrencyEditingID = 0
			m.settingsCurrencyField = 0
			m.settingsCurrencyNameInput.SetValue("")
			m.settingsCurrencyRateInput.SetValue("")
			m = m.focusCurrencyFormField()
			m.status = "adding new currency"
			return m, nil
		}

		if m.settingsCursor == paymentAdd {
			m.settingsEditMode = settingsEditPaymentMethod
			m.settingsPaymentMethodEditingID = 0
			m.settingsPaymentMethodField = 0
			m.settingsPaymentMethodTypeIndex = 0
			m.settingsPaymentMethodIsDefault = len(m.settings.PaymentMethods) == 0
			m.settingsPaymentMethodNameInput.SetValue("")
			m = m.focusPaymentMethodFormField()
			m.status = "adding new payment method"
			return m, nil
		}

		if m.settingsCursor == taxAdd {
			m.settingsEditMode = settingsEditTaxType
			m.settingsTaxTypeEditingID = 0
			m.settingsTaxTypeField = 0
			m.settingsTaxTypeCountryInput.SetValue("")
			m.settingsTaxTypeNameInput.SetValue("")
			m.settingsTaxTypeDescriptionInput.SetValue("")
			m.settingsTaxTypeURLInput.SetValue("")
			m = m.focusTaxTypeFormField()
			m.status = "adding new tax type"
			return m, nil
		}

		if m.settingsCursor == incomeAdd {
			m.settingsEditMode = settingsEditIncomeCategory
			m.settingsIncomeCategoryEditingID = 0
			m.settingsIncomeCategoryNameInput.SetValue("")
			m.settingsIncomeCategoryNameInput.Focus()
			m.status = "adding new income category"
			return m, nil
		}

		if m.settingsCursor == expenseAdd {
			m.settingsEditMode = settingsEditExpenseCategory
			m.settingsExpenseCategoryEditingID = 0
			m.settingsExpenseCategoryNameInput.SetValue("")
			m.settingsExpenseCategoryNameInput.Focus()
			m.status = "adding new expense category"
			return m, nil
		}

		index := m.settingsCursor - 1
		if index >= 0 && index < len(m.settings.Currencies) {
			selected := m.settings.Currencies[index]
			m.settingsEditMode = settingsEditCurrency
			m.settingsCurrencyEditingID = selected.ID
			m.settingsCurrencyField = 1
			m.settingsCurrencyNameInput.SetValue(selected.CurrencyName)
			m.settingsCurrencyRateInput.SetValue(formatRate(selected.RateToBase))
			m = m.focusCurrencyFormField()
			m.status = "editing currency " + selected.CurrencyName
			return m, nil
		}

		if m.settingsCursor >= paymentStart && m.settingsCursor < paymentAdd {
			methodIndex := m.settingsCursor - paymentStart
			selected := m.settings.PaymentMethods[methodIndex]
			m.settingsEditMode = settingsEditPaymentMethod
			m.settingsPaymentMethodEditingID = selected.ID
			m.settingsPaymentMethodField = 0
			m.settingsPaymentMethodNameInput.SetValue(selected.PaymentMethodName)
			m.settingsPaymentMethodTypeIndex = paymentMethodTypeIndex(m.settingsPaymentMethodTypeOptions, selected.PaymentMethodType)
			m.settingsPaymentMethodIsDefault = selected.IsDefault
			m = m.focusPaymentMethodFormField()
			m.status = "editing payment method " + selected.PaymentMethodName
			return m, nil
		}

		if m.settingsCursor >= taxStart && m.settingsCursor < taxAdd {
			taxTypeIndex := m.settingsCursor - taxStart
			selected := m.settings.TaxTypes[taxTypeIndex]
			m.settingsEditMode = settingsEditTaxType
			m.settingsTaxTypeEditingID = selected.ID
			m.settingsTaxTypeField = 0
			m.settingsTaxTypeCountryInput.SetValue(selected.Country)
			m.settingsTaxTypeNameInput.SetValue(selected.TaxTypeName)
			m.settingsTaxTypeDescriptionInput.SetValue(selected.Description)
			m.settingsTaxTypeURLInput.SetValue(selected.URL)
			m = m.focusTaxTypeFormField()
			m.status = "editing tax type " + selected.Country + " / " + selected.TaxTypeName

			return m, nil
		}

		if m.settingsCursor >= incomeStart && m.settingsCursor < incomeAdd {
			idx := m.settingsCursor - incomeStart
			selected := m.settings.IncomeCategories[idx]
			m.settingsEditMode = settingsEditIncomeCategory
			m.settingsIncomeCategoryEditingID = selected.ID
			m.settingsIncomeCategoryNameInput.SetValue(selected.CategoryName)
			m.settingsIncomeCategoryNameInput.Focus()
			m.status = "editing income category " + selected.CategoryName

			return m, nil
		}

		if m.settingsCursor >= expenseStart && m.settingsCursor < expenseAdd {
			idx := m.settingsCursor - expenseStart
			selected := m.settings.ExpenseCategories[idx]
			m.settingsEditMode = settingsEditExpenseCategory
			m.settingsExpenseCategoryEditingID = selected.ID
			m.settingsExpenseCategoryNameInput.SetValue(selected.CategoryName)
			m.settingsExpenseCategoryNameInput.Focus()
			m.status = "editing expense category " + selected.CategoryName

			return m, nil
		}

		return m, nil
	default:
		return m, nil
	}
}

func (m TheApplication) updateDataExport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

		return m, nil
	case "tab", "down":
		m.exportForm = m.exportForm.Next()

		return m, nil
	case "shift+tab", "up":
		m.exportForm = m.exportForm.Prev()

		return m, nil
	case "left", "h":
		if m.exportForm.Active == export.ExportFieldDataset && m.exportForm.DatasetIndex > 0 {
			m.exportForm.DatasetIndex--
		}

		if m.exportForm.Active == export.ExportFieldFormat && m.exportForm.FormatIndex > 0 {
			m.exportForm.FormatIndex--
		}

		return m, nil
	case "right", "l":
		if m.exportForm.Active == export.ExportFieldDataset && m.exportForm.DatasetIndex < len(m.exportForm.DatasetOptions)-1 {
			m.exportForm.DatasetIndex++
		}

		if m.exportForm.Active == export.ExportFieldFormat && m.exportForm.FormatIndex < len(m.exportForm.FormatOptions)-1 {
			m.exportForm.FormatIndex++
		}

		return m, nil
	case "enter":
		if m.exportForm.Active < export.ExportFieldRun {
			m.exportForm = m.exportForm.Next()

			return m, nil
		}

		result, err := m.storage.ExportData(sqliteStorage.ExportRequest{
			Dataset: selectedExportDataset(m.exportForm),
			Format:  selectedExportFormat(m.exportForm),
			Path:    strings.TrimSpace(m.exportForm.PathInput.Value()),
		})
		if err != nil {
			m.status = "export failed: " + err.Error()

			return m, nil
		}

		m.status = fmt.Sprintf("exported %d file(s) to %s", result.FileCount, result.Path)

		return m, nil
	}

	var cmd tea.Cmd

	if m.exportForm.Active == export.ExportFieldPath {
		m.exportForm.PathInput, cmd = m.exportForm.PathInput.Update(msg)

		return m, cmd
	}

	return m, nil
}

func (m TheApplication) focusCurrencyFormField() TheApplication {
	m.settingsCurrencyNameInput.Blur()
	m.settingsCurrencyRateInput.Blur()

	if m.settingsCurrencyField == 0 {
		m.settingsCurrencyNameInput.Focus()
	} else {
		m.settingsCurrencyRateInput.Focus()
	}

	return m
}

func (m TheApplication) focusPaymentMethodFormField() TheApplication {
	m.settingsPaymentMethodNameInput.Blur()
	if m.settingsPaymentMethodField == 0 {
		m.settingsPaymentMethodNameInput.Focus()
	}

	return m
}
