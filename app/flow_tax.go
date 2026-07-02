package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lazybark/cents/flows/tax"
)

func (m TheApplication) confirmDeleteTax() (tea.Model, tea.Cmd) {
	index := m.findTaxIndex(m.deleteConfirmID)
	if index < 0 {
		m = m.clearDeleteConfirmation("tax not found")
		return m, nil
	}

	selected := m.taxes[index]
	if err := m.storage.DeleteTax(selected.ID); err != nil {
		m = m.clearDeleteConfirmation("delete failed: " + err.Error())
		return m, nil
	}

	m.taxes = append(m.taxes[:index], m.taxes[index+1:]...)
	filteredAfter := m.filteredTaxes()
	if len(filteredAfter) == 0 {
		m.taxCursor = 0
	} else if m.taxCursor >= len(filteredAfter) {
		m.taxCursor = len(filteredAfter) - 1
	}

	m = m.clearDeleteConfirmation("deleted tax " + selected.TaxCountry + " / " + selected.TaxTypeName)
	return m, nil
}

func (m TheApplication) focusTaxTypeFormField() TheApplication {
	m.settingsTaxTypeCountryInput.Blur()
	m.settingsTaxTypeNameInput.Blur()
	m.settingsTaxTypeDescriptionInput.Blur()
	m.settingsTaxTypeURLInput.Blur()

	switch m.settingsTaxTypeField {
	case 0:
		m.settingsTaxTypeCountryInput.Focus()
	case 1:
		m.settingsTaxTypeNameInput.Focus()
	case 2:
		m.settingsTaxTypeDescriptionInput.Focus()
	case 3:
		m.settingsTaxTypeURLInput.Focus()
	}

	return m
}

func (m TheApplication) renderEditTaxField(field int, label string, value string) string {
	prefix := "  "

	if m.editTaxForm.ActiveField == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderTaxNew(width int) string {
	lines := []string{
		headlineStyle.Render("New tax"),
		mutedStyle.Render("Use up/down to move fields. Left/right changes tax type. Enter on last field saves."),
		"",
	}

	if len(m.addTaxForm.TaxTypeOptions) == 0 {
		lines = append(lines, mutedStyle.Render("No tax types configured. Add one in Settings first."))
		lines = append(lines, mutedStyle.Render("Press Esc to go back."))

		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	lines = append(lines,
		m.renderTaxChoiceRow(tax.TaxFieldTaxType, "Tax type", m.addTaxForm.TaxDisplayNames, m.addTaxForm.TaxTypeIndex),
		m.renderTaxRowText(tax.TaxFieldAmountDue, "Amount due", m.addTaxForm.Inputs[0].View()),
		m.renderTaxRowText(tax.TaxFieldAmountPaid, "Amount paid", m.addTaxForm.Inputs[1].View()),
		m.renderTaxRowText(tax.TaxFieldPeriod, "Period", m.addTaxForm.Inputs[2].View()),
		m.renderTaxRowText(tax.TaxFieldDueDate, "Due date", m.addTaxForm.Inputs[3].View()),
		m.renderTaxRowText(tax.TaxFieldComment, "Comment", m.addTaxForm.Inputs[4].View()),
	)

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderTaxRowText(field int, label string, value string) string {
	prefix := "  "
	if m.addTaxForm.Active == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderTaxChoiceRow(field int, label string, options []string, selected int) string {
	prefix := "  "
	if m.addTaxForm.Active == field {
		prefix = "> "
	}

	chips := make([]string, 0, len(options))

	for i, option := range options {
		style := buttonStyle
		if i == selected {
			style = buttonActiveStyle
		}

		chips = append(chips, style.Render(option))
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + strings.Join(chips, " ")
}

func (m TheApplication) renderTaxList(width int) string {
	title := "Unpaid taxes"
	if m.taxMode == taxListHistory {
		title = "Tax history (paid)"
	}

	lines := []string{lipgloss.JoinHorizontal(lipgloss.Center, sectionTitleStyle.Render(title), "  ", modeBadgeStyle.Render("Taxes")), hintStyle.Render("Use up/down to browse. Enter edits tax. Delete/Backspace asks confirmation. Esc returns to menu."), ""}

	filtered := m.filteredTaxes()
	if len(filtered) == 0 {
		lines = append(lines, mutedStyle.Render("No taxes found."))

		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	paid, total := taxProgressTotals(filtered)
	lines = append(lines, fieldLabelStyle.Render("Overall paid progress ("+m.baseCurrencyLabel()+")"))
	lines = append(lines, "  "+m.renderProgressBar(paid, total, 28))
	lines = append(lines, "")

	lines = append(lines, m.renderTaxTableHeader(width))

	for i, item := range filtered {
		lines = append(lines, m.renderTaxTableRow(width, i, item))
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderTaxTableHeader(width int) string {
	countryWidth := 10
	typeWidth := 12
	dueWidth := 10
	paidWidth := 12
	leftWidth := 12
	periodWidth := 10
	dueDateWidth := 10
	progressWidth := 8

	commentWidth := width - 14 - countryWidth - typeWidth - dueWidth - paidWidth - leftWidth - periodWidth - dueDateWidth - progressWidth - 18
	if commentWidth < 18 {
		commentWidth = 18
	}

	header := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", "#", countryWidth, "Country", typeWidth, "Tax type", dueWidth, "Due", paidWidth, "Paid", leftWidth, "Left", periodWidth, "Period", dueDateWidth, "Due date", progressWidth, "Done", commentWidth, "Comment")

	return tableHeaderStyle.Render(header)
}

func (m TheApplication) renderTaxTableRow(width int, index int, item tax.Tax) string {
	countryWidth := 10
	typeWidth := 12
	dueWidth := 10
	paidWidth := 12
	leftWidth := 12
	periodWidth := 10
	dueDateWidth := 10
	progressWidth := 8

	commentWidth := width - 14 - countryWidth - typeWidth - dueWidth - paidWidth - leftWidth - periodWidth - dueDateWidth - progressWidth - 18
	if commentWidth < 18 {
		commentWidth = 18
	}

	prefix := " "
	style := rowStyle

	if index == m.taxCursor {
		prefix = ">"
		style = selectedRowStyle
	}

	left := item.AmountDueCents - item.AmountPaidCents
	if left < 0 {
		left = 0
	}

	progressValue := 0.0

	if item.AmountDueCents > 0 {
		progressValue = (float64(item.AmountPaidCents) / float64(item.AmountDueCents)) * 100
		if progressValue < 0 {
			progressValue = 0
		}
		if progressValue > 100 {
			progressValue = 100
		}
	}

	base := m.baseCurrencyLabel()
	dueDateLabel := "-"

	if item.DueDate != nil {
		dueDateLabel = item.DueDate.Local().Format("2006-01-02")
	}

	row := fmt.Sprintf("%s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", prefix, countryWidth, truncateText(item.TaxCountry, countryWidth), typeWidth, truncateText(item.TaxTypeName, typeWidth), dueWidth, renderMoneyWithCurrency(base, item.AmountDueCents), paidWidth, renderMoneyWithCurrency(base, item.AmountPaidCents), leftWidth, renderMoneyWithCurrency(base, left), periodWidth, truncateText(item.Period, periodWidth), dueDateWidth, dueDateLabel, progressWidth, fmt.Sprintf("%5.1f%%", progressValue), commentWidth, truncateText(item.Comment, commentWidth))

	return style.Render(row)
}

func (m TheApplication) renderTaxEdit(width int) string {
	lines := []string{
		headlineStyle.Render("Edit tax"),
		mutedStyle.Render("Edit fields and press Enter on Comment to save."),
		"",
		mutedStyle.Render("Tax: " + m.editTaxForm.CountryLabel + " / " + m.editTaxForm.TaxTypeLabel),
	}

	if idx := m.findTaxIndex(m.editingTaxID); idx >= 0 {
		item := m.taxes[idx]
		lines = append(lines, fieldLabelStyle.Render("Paid progress"))
		lines = append(lines, "  "+m.renderProgressBar(item.AmountPaidCents, item.AmountDueCents, 28))
	}

	lines = append(lines,
		"",
		m.renderEditTaxField(tax.EditTaxFieldAmountDue, "Amount due", m.editTaxForm.AmountDueInput.View()),
		m.renderEditTaxField(tax.EditTaxFieldAmountPaid, "Amount paid", m.editTaxForm.AmountPaidInput.View()),
		m.renderEditTaxField(tax.EditTaxFieldPeriod, "Period", m.editTaxForm.PeriodInput.View()),
		m.renderEditTaxField(tax.EditTaxFieldDueDate, "Due date", m.editTaxForm.DueDateInput.View()),
		m.renderEditTaxField(tax.EditTaxFieldComment, "Comment", m.editTaxForm.CommentInput.View()),
		"",
		fieldLabelStyle.Render("Add transaction"),
		mutedStyle.Render("Set delta/date/comment, then press Enter on Transaction comment to apply."),
		m.renderEditTaxField(tax.EditTaxFieldLogDelta, "Transaction delta", m.editTaxForm.LogDeltaInput.View()),
		m.renderEditTaxField(tax.EditTaxFieldLogDate, "Transaction date", m.editTaxForm.LogDateInput.View()),
		m.renderEditTaxField(tax.EditTaxFieldLogComment, "Transaction comment", m.editTaxForm.LogCommentInput.View()),
		"",
		fieldLabelStyle.Render("Logs"),
	)

	if len(m.taxLogs) == 0 {
		lines = append(lines, mutedStyle.Render("No log entries yet."))
	} else {
		for _, entry := range m.taxLogs {
			sign := "+"
			if entry.DeltaPaidCents < 0 {
				sign = ""
			}

			note := strings.TrimSpace(entry.Note)
			if note != "" {
				lines = append(lines, fmt.Sprintf("%s%s at %s | %s", sign, formatAmount(entry.DeltaPaidCents), entry.CreatedAt.Local().Format("2006-01-02 15:04"), note))
			} else {
				lines = append(lines, fmt.Sprintf("%s%s at %s", sign, formatAmount(entry.DeltaPaidCents), entry.CreatedAt.Local().Format("2006-01-02 15:04")))
			}
		}
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) updateTaxNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.addTaxForm.TaxTypeOptions) == 0 {
		if msg.String() == "esc" {
			m.screen = screenMenu
			m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		return m, nil
	case "up", "shift+tab":
		m.addTaxForm = m.addTaxForm.Prev()
		return m, nil
	case "down", "tab":
		m.addTaxForm = m.addTaxForm.Next()
		return m, nil
	case "left":
		if m.addTaxForm.Active == tax.TaxFieldTaxType && m.addTaxForm.TaxTypeIndex > 0 {
			m.addTaxForm.TaxTypeIndex--
		}
		return m, nil
	case "right":
		if m.addTaxForm.Active == tax.TaxFieldTaxType && m.addTaxForm.TaxTypeIndex < len(m.addTaxForm.TaxTypeOptions)-1 {
			m.addTaxForm.TaxTypeIndex++
		}
		return m, nil
	case "enter":
		if m.addTaxForm.Active == tax.TaxFieldCount-1 {
			return m.saveTaxFromForm()
		}
		m.addTaxForm = m.addTaxForm.Next()
		return m, nil
	}

	if inputIndex := m.addTaxForm.InputIndexForField(m.addTaxForm.Active); inputIndex >= 0 {
		var cmd tea.Cmd
		m.addTaxForm.Inputs[inputIndex], cmd = m.addTaxForm.Inputs[inputIndex].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m TheApplication) updateTaxList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if updatedModel, cmd, handled := m.handleDeleteConfirmation(msg); handled {
		return updatedModel, cmd
	}

	filtered := m.filteredTaxes()
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
		if m.taxCursor > 0 {
			m.taxCursor--
		}
		return m, nil
	case "down":
		if m.taxCursor < len(filtered)-1 {
			m.taxCursor++
		}
		return m, nil
	case "enter":
		m = m.openTaxEditor(filtered[m.taxCursor]).(TheApplication)
		return m, nil
	case "backspace", "delete":
		selected := filtered[m.taxCursor]
		m = m.beginDeleteConfirmation("tax", selected.ID, selected.TaxCountry+" / "+selected.TaxTypeName)
		return m, nil
	default:
		return m, nil
	}
}

func (m TheApplication) openTaxEditor(selected tax.Tax) tea.Model {
	m.screen = screenTaxEdit
	m.editingTaxID = selected.ID
	m.editTaxForm = tax.NewEditTaxForm()
	m.editTaxForm.AmountDueInput.SetValue(formatAmount(selected.AmountDueCents))
	m.editTaxForm.AmountPaidInput.SetValue(formatAmount(selected.AmountPaidCents))
	m.editTaxForm.PeriodInput.SetValue(selected.Period)
	if selected.DueDate != nil {
		m.editTaxForm.DueDateInput.SetValue(selected.DueDate.Local().Format("02.01.2006"))
	} else {
		m.editTaxForm.DueDateInput.SetValue("")
	}
	m.editTaxForm.CommentInput.SetValue(selected.Comment)
	m.editTaxForm.LogDateInput.SetValue(time.Now().Format("02.01.2006"))
	m.editTaxForm.LogCommentInput.SetValue("")
	m.editTaxForm.TaxTypeLabel = selected.TaxTypeName
	m.editTaxForm.CountryLabel = selected.TaxCountry
	m.editTaxForm = m.editTaxForm.FocusActive()

	if logs, err := m.storage.LoadTaxLogs(selected.ID); err == nil {
		m.taxLogs = logs
	} else {
		m.taxLogs = nil
	}

	m.status = "editing tax " + selected.TaxCountry + " / " + selected.TaxTypeName
	return m
}

func (m TheApplication) updateTaxEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenTaxList
		m.status = "tax edit cancelled"
		return m, nil
	case "up", "shift+tab":
		m.editTaxForm = m.editTaxForm.Prev()
		return m, nil
	case "down", "tab":
		m.editTaxForm = m.editTaxForm.Next()
		return m, nil
	case "enter":
		if m.editTaxForm.ActiveField == tax.EditTaxFieldLogComment {
			return m.applyTaxLogDelta()
		}
		if m.editTaxForm.ActiveField == tax.EditTaxFieldComment {
			return m.saveTaxEdit()
		}
		m.editTaxForm = m.editTaxForm.Next()
		return m, nil
	}

	var cmd tea.Cmd
	switch m.editTaxForm.ActiveField {
	case tax.EditTaxFieldAmountDue:
		m.editTaxForm.AmountDueInput, cmd = m.editTaxForm.AmountDueInput.Update(msg)
	case tax.EditTaxFieldAmountPaid:
		m.editTaxForm.AmountPaidInput, cmd = m.editTaxForm.AmountPaidInput.Update(msg)
	case tax.EditTaxFieldPeriod:
		m.editTaxForm.PeriodInput, cmd = m.editTaxForm.PeriodInput.Update(msg)
	case tax.EditTaxFieldDueDate:
		m.editTaxForm.DueDateInput, cmd = m.editTaxForm.DueDateInput.Update(msg)
	case tax.EditTaxFieldComment:
		m.editTaxForm.CommentInput, cmd = m.editTaxForm.CommentInput.Update(msg)
	case tax.EditTaxFieldLogDelta:
		m.editTaxForm.LogDeltaInput, cmd = m.editTaxForm.LogDeltaInput.Update(msg)
	case tax.EditTaxFieldLogDate:
		m.editTaxForm.LogDateInput, cmd = m.editTaxForm.LogDateInput.Update(msg)
	case tax.EditTaxFieldLogComment:
		m.editTaxForm.LogCommentInput, cmd = m.editTaxForm.LogCommentInput.Update(msg)
	}
	return m, cmd
}

func (m TheApplication) saveTaxFromForm() (tea.Model, tea.Cmd) {
	if len(m.addTaxForm.TaxTypeOptions) == 0 {
		m.status = "no tax types configured; add one in settings"
		return m, nil
	}

	taxTypeIndex := m.addTaxForm.TaxTypeIndex
	if taxTypeIndex < 0 || taxTypeIndex >= len(m.addTaxForm.TaxTypeOptions) {
		taxTypeIndex = 0
	}
	selectedType := m.addTaxForm.TaxTypeOptions[taxTypeIndex]
	amountDueRaw := strings.TrimSpace(m.addTaxForm.Inputs[0].Value())
	amountPaidRaw := strings.TrimSpace(m.addTaxForm.Inputs[1].Value())
	period := strings.TrimSpace(m.addTaxForm.Inputs[2].Value())
	dueDateRaw := strings.TrimSpace(m.addTaxForm.Inputs[3].Value())
	comment := strings.TrimSpace(m.addTaxForm.Inputs[4].Value())

	if period == "" {
		m.status = "period is required"
		return m, nil
	}

	amountDue, err := parseAmountCents(amountDueRaw)
	if err != nil {
		m.status = "amount due error: " + err.Error()
		return m, nil
	}
	if amountDue <= 0 {
		m.status = "amount due must be greater than zero"
		return m, nil
	}

	amountPaid, err := parseAmountCents(amountPaidRaw)
	if err != nil {
		m.status = "amount paid error: " + err.Error()
		return m, nil
	}

	dueDate, err := parseOptionalDatePointer(dueDateRaw)
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	now := time.Now()
	newTax := tax.Tax{
		TaxTypeID:       selectedType.ID,
		TaxCountry:      strings.TrimSpace(selectedType.Country),
		TaxTypeName:     strings.TrimSpace(selectedType.TaxTypeName),
		AmountDueCents:  amountDue,
		AmountPaidCents: amountPaid,
		Period:          period,
		DueDate:         dueDate,
		Comment:         comment,
		LastUpdatedAt:   now,
	}

	if err := m.storage.CreateTax(&newTax); err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.taxes = append([]tax.Tax{newTax}, m.taxes...)
	m.addTaxForm = tax.NewAddTaxForm(m.settings.TaxTypes)
	m.screen = screenTaxList
	if newTax.AmountPaidCents >= newTax.AmountDueCents {
		m.taxMode = taxListHistory
	} else {
		m.taxMode = taxListUnpaid
	}
	m.taxCursor = 0
	m.status = "saved tax " + newTax.TaxCountry + " / " + newTax.TaxTypeName
	return m, nil
}

func (m TheApplication) saveTaxEdit() (tea.Model, tea.Cmd) {
	index := m.findTaxIndex(m.editingTaxID)
	if index < 0 {
		m.status = "tax not found"
		return m, nil
	}

	amountDue, err := parseAmountCents(strings.TrimSpace(m.editTaxForm.AmountDueInput.Value()))
	if err != nil {
		m.status = "amount due error: " + err.Error()
		return m, nil
	}
	if amountDue <= 0 {
		m.status = "amount due must be greater than zero"
		return m, nil
	}

	amountPaid, err := parseAmountCents(strings.TrimSpace(m.editTaxForm.AmountPaidInput.Value()))
	if err != nil {
		m.status = "amount paid error: " + err.Error()
		return m, nil
	}

	period := strings.TrimSpace(m.editTaxForm.PeriodInput.Value())
	if period == "" {
		m.status = "period is required"
		return m, nil
	}

	dueDate, err := parseOptionalDatePointer(strings.TrimSpace(m.editTaxForm.DueDateInput.Value()))
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	selected := m.taxes[index]
	selected.AmountDueCents = amountDue
	selected.AmountPaidCents = amountPaid
	selected.Period = period
	selected.DueDate = dueDate
	selected.Comment = strings.TrimSpace(m.editTaxForm.CommentInput.Value())
	selected.LastUpdatedAt = time.Now()

	if err := m.storage.SaveTax(&selected); err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.taxes[index] = selected
	m.screen = screenTaxList
	m.status = "updated tax " + selected.TaxCountry + " / " + selected.TaxTypeName
	return m, nil
}

func (m TheApplication) applyTaxLogDelta() (tea.Model, tea.Cmd) {
	index := m.findTaxIndex(m.editingTaxID)
	if index < 0 {
		m.status = "tax not found"
		return m, nil
	}

	delta, err := parseSignedAmountCents(strings.TrimSpace(m.editTaxForm.LogDeltaInput.Value()))
	if err != nil {
		m.status = "log delta error: " + err.Error()
		return m, nil
	}
	if delta == 0 {
		m.status = "delta cannot be zero"
		return m, nil
	}

	selected := m.taxes[index]
	nextPaid := selected.AmountPaidCents + delta
	if nextPaid < 0 {
		m.status = "delta makes amount paid negative"
		return m, nil
	}

	entryTime, err := parseLogDateOrToday(strings.TrimSpace(m.editTaxForm.LogDateInput.Value()))
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	now := time.Now()
	selected.AmountPaidCents = nextPaid
	selected.LastUpdatedAt = now

	if err := m.storage.SaveTax(&selected); err != nil {
		m.status = "tax update failed: " + err.Error()

		return m, nil
	}

	note := strings.TrimSpace(m.editTaxForm.LogCommentInput.Value())
	if note == "" {
		note = "manual tax paid adjustment"
	}

	entry := tax.TaxLog{
		TaxID:          selected.ID,
		DeltaPaidCents: delta,
		Note:           note,
		CreatedAt:      entryTime,
	}

	if err := m.storage.CreateTaxLog(&entry); err != nil {
		m.status = "log save failed: " + err.Error()

		return m, nil
	}

	m.taxes[index] = selected
	m.taxLogs = append([]tax.TaxLog{entry}, m.taxLogs...)
	m.editTaxForm.AmountPaidInput.SetValue(formatAmount(selected.AmountPaidCents))
	m.editTaxForm.LogDeltaInput.SetValue("")
	m.editTaxForm.LogDateInput.SetValue(time.Now().Format("02.01.2006"))
	m.editTaxForm.LogCommentInput.SetValue("")
	m.status = "applied tax log delta"

	return m, nil
}

func (m TheApplication) findTaxIndex(id uint) int {
	for i := range m.taxes {
		if m.taxes[i].ID == id {
			return i
		}
	}

	return -1
}

func (m TheApplication) filteredTaxes() []tax.Tax {
	filtered := make([]tax.Tax, 0, len(m.taxes))

	for _, item := range m.taxes {
		paid := item.AmountPaidCents >= item.AmountDueCents
		switch m.taxMode {
		case taxListUnpaid:
			if !paid {
				filtered = append(filtered, item)
			}
		case taxListHistory:
			if paid {
				filtered = append(filtered, item)
			}
		}
	}

	return filtered
}
