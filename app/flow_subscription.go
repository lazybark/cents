package app

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lazybark/cents/flows/subscription"
)

func (m TheApplication) renderSubscriptionNew(width int) string {
	activeMarker := "[ ]"

	if m.addSubscriptionForm.IsActive {
		activeMarker = "[x]"
	}

	lines := []string{
		headlineStyle.Render("New subscription"),
		mutedStyle.Render("Use up/down to move fields. Left/right changes Currency/Method/Period/Type. Space toggles Is Active. Enter on last field saves."),
		"",
		m.renderSubscriptionTextFieldRow(subscription.SubFieldName, "Name", m.addSubscriptionForm.Inputs[0].View()),
		m.renderSubscriptionOptionRow(subscription.SubFieldCurrency, "Currency", m.addSubscriptionForm.CurrencyOptions, m.addSubscriptionForm.CurrencyIndex),
		m.renderSubscriptionTextFieldRow(subscription.SubFieldAmount, "Amount", m.addSubscriptionForm.Inputs[2].View()),
		m.renderSubscriptionOptionRow(subscription.SubFieldPeriod, "Period", m.addSubscriptionForm.PeriodOptions, m.addSubscriptionForm.PeriodIndex),
		m.renderSubscriptionOptionRow(subscription.SubFieldPaymentMethodChoice, "Payment method", m.addSubscriptionForm.PaymentMethodOptions, m.addSubscriptionForm.PaymentMethodIndex),
		m.renderSubscriptionTextFieldRow(subscription.SubFieldPaymentMethod, "Payment method (custom)", m.addSubscriptionForm.Inputs[3].View()),
		m.renderSubscriptionOptionRow(subscription.SubFieldType, "Type", m.addSubscriptionForm.TypeOptions, m.addSubscriptionForm.TypeIndex),
		m.renderSubscriptionTextFieldRow(subscription.SubFieldIsActive, "Is Active", activeMarker),
		m.renderSubscriptionTextFieldRow(subscription.SubFieldDayYearly, "Payment date (yearly)", m.addSubscriptionForm.Inputs[4].View()),
		m.renderSubscriptionTextFieldRow(subscription.SubFieldDayMonthly, "Payment day (monthly)", m.addSubscriptionForm.Inputs[5].View()),
		"",
		mutedStyle.Render("Esc goes back to menu. Day fields are optional."),
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderSubscriptionEdit(width int) string {
	activeMarker := "[ ]"

	if m.editSubscriptionForm.IsActive {
		activeMarker = "[x]"
	}

	lines := []string{
		headlineStyle.Render("Edit subscription"),
		mutedStyle.Render("Use up/down to move fields. Space toggles Is Active. Enter on Active saves. Esc cancels."),
		"",
		mutedStyle.Render("Name: " + m.editSubscriptionForm.NameLabel + " | Type: " + m.editSubscriptionForm.TypeLabel + " | Period: " + m.editSubscriptionForm.PeriodLabel),
		mutedStyle.Render("Currency: " + m.editSubscriptionForm.CurrencyLabel + " | Yearly date: " + m.editSubscriptionForm.PaymentDateYearly + " | Monthly day: " + m.editSubscriptionForm.PaymentDayMonthly),
		"",
		m.renderEditSubscriptionField(subscription.EditSubFieldAmount, "Amount", m.editSubscriptionForm.AmountInput.View()),
		m.renderEditSubscriptionField(subscription.EditSubFieldPaymentMethod, "Payment method", m.editSubscriptionForm.PaymentMethodInput.View()),
		m.renderEditSubscriptionField(subscription.EditSubFieldIsActive, "Is Active", activeMarker),
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderEditSubscriptionField(field int, label string, value string) string {
	prefix := "  "

	if m.editSubscriptionForm.ActiveField == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderSubscriptionTextFieldRow(field int, label string, value string) string {
	prefix := "  "

	if m.addSubscriptionForm.Active == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderSubscriptionOptionRow(field int, label string, options []string, selected int) string {
	prefix := "  "

	if m.addSubscriptionForm.Active == field {
		prefix = "> "
	}

	chips := make([]string, 0, len(options))

	for idx, option := range options {
		style := buttonStyle
		if idx == selected {
			style = buttonActiveStyle
		}

		chips = append(chips, style.Render(option))
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + strings.Join(chips, " ")
}

func (m TheApplication) renderSubscriptionList(width int) string {
	modeTitle := "All subscriptions"

	if m.subscriptionMode == subscriptionListActive {
		modeTitle = "Active subscriptions"
	}

	lines := []string{lipgloss.JoinHorizontal(lipgloss.Center, sectionTitleStyle.Render(modeTitle), "  ", modeBadgeStyle.Render("Subscriptions")), hintStyle.Render("Use up/down to browse. Enter edits the selected subscription. Delete/Backspace asks confirmation. Esc returns to menu."), ""}

	filtered := m.filteredSubscriptions()
	if len(filtered) == 0 {
		lines = append(lines, mutedStyle.Render("No subscriptions found."))

		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	if m.subscriptionMode == subscriptionListAll {
		activeSubs, inactiveSubs := splitSubscriptionsByActivity(filtered)
		activeMonthlyBase, activeYearlyProjectionBase := m.subscriptionTotalsInBaseCents(activeSubs)
		inactiveMonthlyBase, inactiveYearlyProjectionBase := m.subscriptionTotalsInBaseCents(inactiveSubs)
		baseLabel := m.baseCurrencyLabel()
		lines = append(lines,
			fieldLabelStyle.Render("Active totals in "+baseLabel+":"),
			"  monthly: "+renderMoneyWithCurrency(baseLabel, activeMonthlyBase),
			"  yearly:  "+renderMoneyWithCurrency(baseLabel, activeYearlyProjectionBase),
			"",
			fieldLabelStyle.Render("Inactive totals in "+baseLabel+":"),
			"  monthly: "+renderMoneyWithCurrency(baseLabel, inactiveMonthlyBase),
			"  yearly:  "+renderMoneyWithCurrency(baseLabel, inactiveYearlyProjectionBase),
			"",
		)
	} else {
		monthlyBase, yearlyProjectionBase := m.subscriptionTotalsInBaseCents(filtered)
		baseLabel := m.baseCurrencyLabel()
		lines = append(lines,
			fieldLabelStyle.Render("Active totals in "+baseLabel+":"),
			"  monthly: "+renderMoneyWithCurrency(baseLabel, monthlyBase),
			"  yearly:  "+renderMoneyWithCurrency(baseLabel, yearlyProjectionBase),
			"",
		)
	}

	lines = append(lines, m.renderSubscriptionTableHeader(width))

	for idx, sub := range filtered {
		lines = append(lines, m.renderSubscriptionRow(width, idx, sub))
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderSubscriptionTableHeader(width int) string {
	nameWidth := 14
	currencyWidth := 8
	amountWidth := 12
	baseAmountWidth := 12
	periodWidth := 7
	typeWidth := 12
	activeWidth := 7

	descWidth := width - 12 - nameWidth - currencyWidth - amountWidth - baseAmountWidth - periodWidth - typeWidth - activeWidth - 14
	if descWidth < 18 {
		descWidth = 18
	}

	baseCurrencyTitle := strings.TrimSpace(m.settings.BaseCurrency)
	if baseCurrencyTitle == "" {
		baseCurrencyTitle = "$"
	}

	header := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", "#", nameWidth, "Name", currencyWidth, "Curr", amountWidth, "Amount", baseAmountWidth, truncateText(baseCurrencyTitle, baseAmountWidth), periodWidth, "Period", typeWidth, "Type", activeWidth, "Active", descWidth, "Payment method")

	return tableHeaderStyle.Render(header)
}

func (m TheApplication) renderSubscriptionRow(width int, index int, sub subscription.Subscription) string {
	nameWidth := 14
	currencyWidth := 8
	amountWidth := 12
	baseAmountWidth := 12
	periodWidth := 7
	typeWidth := 12
	activeWidth := 7

	descWidth := width - 12 - nameWidth - currencyWidth - amountWidth - baseAmountWidth - periodWidth - typeWidth - activeWidth - 14
	if descWidth < 18 {
		descWidth = 18
	}

	prefix := " "

	style := rowStyle
	if index == m.subscriptionCursor {
		prefix = ">"
		style = selectedRowStyle
	}

	active := "no"

	if sub.IsActive {
		active = "yes"
	}

	row := fmt.Sprintf("%s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", prefix, nameWidth, truncateText(sub.Name, nameWidth), currencyWidth, truncateText(sub.Currency, currencyWidth), amountWidth, renderMoneyWithCurrency(sub.Currency, sub.AmountCents), baseAmountWidth, m.convertedAmountForBase(sub.Currency, sub.AmountCents), periodWidth, truncateText(sub.Period, periodWidth), typeWidth, truncateText(sub.Type, typeWidth), activeWidth, active, descWidth, truncateText(sub.PaymentMethod, descWidth))

	return style.Render(row)
}

func splitSubscriptionsByActivity(subs []subscription.Subscription) (active []subscription.Subscription, inactive []subscription.Subscription) {
	active = make([]subscription.Subscription, 0, len(subs))
	inactive = make([]subscription.Subscription, 0, len(subs))

	for _, sub := range subs {
		if sub.IsActive {
			active = append(active, sub)

			continue
		}

		inactive = append(inactive, sub)
	}

	return active, inactive
}

func sortSubscriptionsByAmount(values []subscription.Subscription) []subscription.Subscription {
	if len(values) < 2 {
		return values
	}

	cloned := make([]subscription.Subscription, len(values))

	copy(cloned, values)

	sort.SliceStable(cloned, func(i, j int) bool {
		if cloned[i].AmountCents == cloned[j].AmountCents {
			return cloned[i].CreatedAt.After(cloned[j].CreatedAt)
		}

		return cloned[i].AmountCents > cloned[j].AmountCents
	})

	return cloned
}

func (m TheApplication) updateSubscriptionNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		return m, nil
	case "up", "shift+tab":
		m.addSubscriptionForm = m.addSubscriptionForm.Prev()
		return m, nil
	case "down", "tab":
		m.addSubscriptionForm = m.addSubscriptionForm.Next()
		return m, nil
	case "left":
		switch m.addSubscriptionForm.Active {
		case subscription.SubFieldCurrency:
			if m.addSubscriptionForm.CurrencyIndex > 0 {
				m.addSubscriptionForm.CurrencyIndex--
			}
		case subscription.SubFieldPaymentMethodChoice:
			if m.addSubscriptionForm.PaymentMethodIndex > 0 {
				m.addSubscriptionForm.PaymentMethodIndex--
			}
		case subscription.SubFieldPeriod:
			if m.addSubscriptionForm.PeriodIndex > 0 {
				m.addSubscriptionForm.PeriodIndex--
			}
		case subscription.SubFieldType:
			if m.addSubscriptionForm.TypeIndex > 0 {
				m.addSubscriptionForm.TypeIndex--
			}
		}
		return m, nil
	case "right":
		switch m.addSubscriptionForm.Active {
		case subscription.SubFieldCurrency:
			if m.addSubscriptionForm.CurrencyIndex < len(m.addSubscriptionForm.CurrencyOptions)-1 {
				m.addSubscriptionForm.CurrencyIndex++
			}
		case subscription.SubFieldPaymentMethodChoice:
			if m.addSubscriptionForm.PaymentMethodIndex < len(m.addSubscriptionForm.PaymentMethodOptions)-1 {
				m.addSubscriptionForm.PaymentMethodIndex++
			}
		case subscription.SubFieldPeriod:
			if m.addSubscriptionForm.PeriodIndex < len(m.addSubscriptionForm.PeriodOptions)-1 {
				m.addSubscriptionForm.PeriodIndex++
			}
		case subscription.SubFieldType:
			if m.addSubscriptionForm.TypeIndex < len(m.addSubscriptionForm.TypeOptions)-1 {
				m.addSubscriptionForm.TypeIndex++
			}
		}
		return m, nil
	case " ":
		if m.addSubscriptionForm.Active == subscription.SubFieldIsActive {
			m.addSubscriptionForm.IsActive = !m.addSubscriptionForm.IsActive
			return m, nil
		}
		// Let text inputs receive spaces when a text field is active.
	case "enter":
		if m.addSubscriptionForm.Active == subscription.SubFieldCount-1 {
			return m.saveSubscriptionFromForm()
		}
		m.addSubscriptionForm = m.addSubscriptionForm.Next()
		return m, nil
	}

	if inputIndex := m.addSubscriptionForm.InputIndexForField(m.addSubscriptionForm.Active); inputIndex >= 0 {
		var cmd tea.Cmd
		m.addSubscriptionForm.Inputs[inputIndex], cmd = m.addSubscriptionForm.Inputs[inputIndex].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m TheApplication) updateSubscriptionList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if updatedModel, cmd, handled := m.handleDeleteConfirmation(msg); handled {
		return updatedModel, cmd
	}

	filtered := m.filteredSubscriptions()
	if len(filtered) == 0 {
		if msg.String() == "esc" {
			m.screen = screenMenu
			m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		return m, nil
	case "up":
		if m.subscriptionCursor > 0 {
			m.subscriptionCursor--
		}
		return m, nil
	case "down":
		if m.subscriptionCursor < len(filtered)-1 {
			m.subscriptionCursor++
		}
		return m, nil
	case "enter":
		m = m.openSubscriptionEditor(filtered[m.subscriptionCursor]).(TheApplication)
		return m, nil
	case "backspace", "delete":
		selected := filtered[m.subscriptionCursor]
		m = m.beginDeleteConfirmation("subscription", selected.ID, selected.Name)
		return m, nil
	default:
		return m, nil
	}
}

func (m TheApplication) openSubscriptionEditor(selected subscription.Subscription) tea.Model {
	m.screen = screenSubscriptionEdit
	m.editingSubscriptionID = selected.ID
	m.editingSubscriptionMode = m.subscriptionMode
	m.editSubscriptionForm = subscription.NewEditSubscriptionForm()
	m.editSubscriptionForm.AmountInput.SetValue(formatAmount(selected.AmountCents))
	m.editSubscriptionForm.PaymentMethodInput.SetValue(selected.PaymentMethod)
	m.editSubscriptionForm.IsActive = selected.IsActive
	m.editSubscriptionForm.PeriodLabel = selected.Period
	m.editSubscriptionForm.TypeLabel = selected.Type
	m.editSubscriptionForm.CurrencyLabel = selected.Currency
	m.editSubscriptionForm.NameLabel = selected.Name
	m.editSubscriptionForm.PaymentDateYearly = selected.PaymentDateYearly
	if selected.PaymentDayMonthly != nil {
		m.editSubscriptionForm.PaymentDayMonthly = strconv.Itoa(*selected.PaymentDayMonthly)
	}
	m.editSubscriptionForm = m.editSubscriptionForm.FocusActive()
	m.status = "editing subscription " + selected.Name
	return m
}

func (m TheApplication) updateSubscriptionEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenSubscriptionList
		m.status = "edit cancelled"

		return m, nil
	case "up", "shift+tab":
		m.editSubscriptionForm = m.editSubscriptionForm.Prev()

		return m, nil
	case "down", "tab":
		m.editSubscriptionForm = m.editSubscriptionForm.Next()

		return m, nil
	case " ":
		if m.editSubscriptionForm.ActiveField == subscription.EditSubFieldIsActive {
			m.editSubscriptionForm.IsActive = !m.editSubscriptionForm.IsActive

			return m, nil
		}
	case "enter":
		if m.editSubscriptionForm.ActiveField == subscription.EditSubFieldCount-1 {
			return m.saveSubscriptionEdit()
		}

		m.editSubscriptionForm = m.editSubscriptionForm.Next()

		return m, nil
	}

	if m.editSubscriptionForm.ActiveField == subscription.EditSubFieldAmount {
		var cmd tea.Cmd

		m.editSubscriptionForm.AmountInput, cmd = m.editSubscriptionForm.AmountInput.Update(msg)

		return m, cmd
	}

	if m.editSubscriptionForm.ActiveField == subscription.EditSubFieldPaymentMethod {
		var cmd tea.Cmd

		m.editSubscriptionForm.PaymentMethodInput, cmd = m.editSubscriptionForm.PaymentMethodInput.Update(msg)

		return m, cmd
	}

	return m, nil
}

func (m TheApplication) subscriptionTotalsInBaseCents(subs []subscription.Subscription) (monthlyTotal int64, yearlyProjection int64) {
	var yearlyOnly int64

	for _, sub := range subs {
		converted, ok := m.convertToBaseCents(sub.Currency, sub.AmountCents)
		if !ok {
			continue
		}

		if strings.EqualFold(strings.TrimSpace(sub.Period), "month") {
			monthlyTotal += converted

			continue
		}

		if strings.EqualFold(strings.TrimSpace(sub.Period), "year") {
			yearlyOnly += converted
		}
	}

	yearlyProjection = yearlyOnly + monthlyTotal*12

	return monthlyTotal, yearlyProjection
}

func (m TheApplication) filteredSubscriptions() []subscription.Subscription {
	if m.subscriptionMode == subscriptionListAll {
		return m.subscriptions
	}

	filtered := make([]subscription.Subscription, 0, len(m.subscriptions))

	for _, sub := range m.subscriptions {
		if sub.IsActive {
			filtered = append(filtered, sub)
		}
	}

	return filtered
}

func (m TheApplication) confirmDeleteSubscription() (tea.Model, tea.Cmd) {
	index := -1

	for i := range m.subscriptions {
		if m.subscriptions[i].ID == m.deleteConfirmID {
			index = i

			break
		}
	}

	if index < 0 {
		m = m.clearDeleteConfirmation("subscription not found")

		return m, nil
	}

	selected := m.subscriptions[index]
	if err := m.storage.DeleteSubscription(selected.ID); err != nil {
		m = m.clearDeleteConfirmation("delete failed: " + err.Error())

		return m, nil
	}

	m.subscriptions = append(m.subscriptions[:index], m.subscriptions[index+1:]...)

	filteredAfter := m.filteredSubscriptions()
	if len(filteredAfter) == 0 {
		m.subscriptionCursor = 0
	} else if m.subscriptionCursor >= len(filteredAfter) {
		m.subscriptionCursor = len(filteredAfter) - 1
	}

	m = m.clearDeleteConfirmation("deleted subscription " + selected.Name)

	return m, nil
}

func (m TheApplication) saveSubscriptionFromForm() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.addSubscriptionForm.Inputs[0].Value())
	currency := selectedCurrencyOption(m.addSubscriptionForm.CurrencyOptions, m.addSubscriptionForm.CurrencyIndex)
	amountRaw := strings.TrimSpace(m.addSubscriptionForm.Inputs[2].Value())
	paymentMethodSelection := selectedPaymentMethodOption(m.addSubscriptionForm.PaymentMethodOptions, m.addSubscriptionForm.PaymentMethodIndex)
	paymentMethodCustom := strings.TrimSpace(m.addSubscriptionForm.Inputs[3].Value())
	paymentMethod := paymentMethodSelection
	if paymentMethodCustom != "" {
		paymentMethod = paymentMethodCustom
	}
	dayYearRaw := strings.TrimSpace(m.addSubscriptionForm.Inputs[4].Value())
	dayMonthRaw := strings.TrimSpace(m.addSubscriptionForm.Inputs[5].Value())

	if name == "" {
		m.status = "subscription name is required"
		return m, nil
	}
	if currency == "" {
		m.status = "currency is required"
		return m, nil
	}
	if paymentMethod == "" {
		m.status = "payment method is required"
		return m, nil
	}

	amount, err := parseAmountCents(amountRaw)
	if err != nil {
		m.status = "amount error: " + err.Error()
		return m, nil
	}

	dateYearly, err := parseOptionalDate(dayYearRaw)
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	dayMonthly, err := parseOptionalDay(dayMonthRaw, 1, 31, "monthly")
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	now := time.Now()
	newSubscription := subscription.Subscription{
		Name:              name,
		Currency:          currency,
		AmountCents:       amount,
		Period:            m.addSubscriptionForm.PeriodOptions[m.addSubscriptionForm.PeriodIndex],
		PaymentMethod:     paymentMethod,
		Type:              m.addSubscriptionForm.TypeOptions[m.addSubscriptionForm.TypeIndex],
		IsActive:          m.addSubscriptionForm.IsActive,
		PaymentDateYearly: dateYearly,
		PaymentDayMonthly: dayMonthly,
		LastUpdatedAt:     now,
	}

	if err := m.storage.CreateSubscription(&newSubscription); err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.subscriptions = append([]subscription.Subscription{newSubscription}, m.subscriptions...)
	m.subscriptions = sortSubscriptionsByAmount(m.subscriptions)
	m.addSubscriptionForm = subscription.NewAddSubscriptionForm(currencySelectionOptions(m.settings), paymentMethodSelectionOptions(m.settings))
	m.screen = screenSubscriptionList
	m.subscriptionMode = subscriptionListAll
	m.subscriptionCursor = 0
	m.status = "saved subscription " + name
	return m, nil
}

func (m TheApplication) saveSubscriptionEdit() (tea.Model, tea.Cmd) {
	if m.editingSubscriptionID == 0 {
		m.status = "no subscription selected"
		return m, nil
	}

	amountRaw := strings.TrimSpace(m.editSubscriptionForm.AmountInput.Value())
	paymentMethod := strings.TrimSpace(m.editSubscriptionForm.PaymentMethodInput.Value())
	if paymentMethod == "" {
		m.status = "payment method is required"
		return m, nil
	}

	amount, err := parseAmountCents(amountRaw)
	if err != nil {
		m.status = "amount error: " + err.Error()
		return m, nil
	}

	selectedIndex := -1
	for i := range m.subscriptions {
		if m.subscriptions[i].ID == m.editingSubscriptionID {
			selectedIndex = i
			break
		}
	}
	if selectedIndex < 0 {
		m.status = "subscription not found"
		return m, nil
	}

	selected := m.subscriptions[selectedIndex]
	selected.AmountCents = amount
	selected.PaymentMethod = paymentMethod
	selected.IsActive = m.editSubscriptionForm.IsActive
	selected.LastUpdatedAt = time.Now()

	if err := m.storage.SaveSubscription(&selected); err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.subscriptions[selectedIndex] = selected
	if m.subscriptionMode == subscriptionListAll {
		m.subscriptions = sortSubscriptionsByAmount(m.subscriptions)
	}
	m.screen = screenSubscriptionList
	filtered := m.filteredSubscriptions()
	if len(filtered) == 0 {
		m.subscriptionCursor = 0
	} else if m.subscriptionCursor >= len(filtered) {
		m.subscriptionCursor = len(filtered) - 1
	}
	m.status = "updated subscription " + selected.Name
	return m, nil
}
