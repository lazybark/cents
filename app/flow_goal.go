package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/goal"
)

func (m TheApplication) renderGoalTableRow(width int, index int, item goal.Goal) string {
	nameWidth := 12
	currencyWidth := 8
	targetWidth := 12
	accumWidth := 12
	leftWidth := 12
	startedWidth := 10
	targetDateWidth := 10
	progressWidth := 8

	descWidth := width - 14 - nameWidth - currencyWidth - targetWidth - accumWidth - leftWidth - startedWidth - targetDateWidth - progressWidth - 18
	descWidth = max(descWidth, 18)

	prefix := " "
	style := rowStyle
	if index == m.goalCursor {
		prefix = ">"
		style = selectedRowStyle
	}

	left := item.LeftCents()

	targetDate := "-"
	if item.TargetDate != nil {
		targetDate = item.TargetDate.Local().Format("2006-01-02")
	}

	progressValue := item.Percent()

	row := fmt.Sprintf("%s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", prefix, nameWidth, truncateText(item.Name, nameWidth), currencyWidth, truncateText(item.Currency, currencyWidth), targetWidth, renderMoneyWithCurrency(item.Currency, item.TargetAmountCents), accumWidth, renderMoneyWithCurrency(item.Currency, item.AmountAccumulatedCents), leftWidth, renderMoneyWithCurrency(item.Currency, left), startedWidth, item.DateStartedAt.Local().Format("2006-01-02"), targetDateWidth, targetDate, progressWidth, fmt.Sprintf("%5.1f%%", progressValue), descWidth, truncateText(item.Description, descWidth))

	return style.Render(row)
}

func (m TheApplication) renderGoalEdit(width int) string {
	lines := []string{
		headlineStyle.Render("Edit goal"),
		mutedStyle.Render("Edit fields and press Enter on Description to save."),
		"",
		mutedStyle.Render("Goal: " + m.editGoalForm.NameLabel + " | Currency: " + m.editGoalForm.CurrencyLabel),
	}

	if idx := m.findGoalIndex(m.editingGoalID); idx >= 0 {
		item := m.goals[idx]
		lines = append(lines, fieldLabelStyle.Render("Accumulation progress"))
		lines = append(lines, "  "+m.renderProgressBar(item.AmountAccumulatedCents, item.TargetAmountCents, 28))
	}

	lines = append(lines,
		"",
		m.renderEditGoalField(goal.EditGoalFieldTargetAmount, "Target amount", m.editGoalForm.TargetAmountInput.View()),
		m.renderEditGoalField(goal.EditGoalFieldAccumulated, "Accumulated", m.editGoalForm.AccumulatedAmountInput.View()),
		m.renderEditGoalField(goal.EditGoalFieldDateStarted, "Date started", m.editGoalForm.DateStartedInput.View()),
		m.renderEditGoalField(goal.EditGoalFieldTargetDate, "Target date", m.editGoalForm.TargetDateInput.View()),
		m.renderEditGoalField(goal.EditGoalFieldDescription, "Description", m.editGoalForm.DescriptionInput.View()),
		"",
		fieldLabelStyle.Render("Add transaction"),
		mutedStyle.Render("Set delta/date/comment, then press Enter on Transaction comment to apply."),
		m.renderEditGoalField(goal.EditGoalFieldLogDelta, "Transaction delta", m.editGoalForm.LogDeltaInput.View()),
		m.renderEditGoalField(goal.EditGoalFieldLogDate, "Transaction date", m.editGoalForm.LogDateInput.View()),
		m.renderEditGoalField(goal.EditGoalFieldLogComment, "Transaction comment", m.editGoalForm.LogCommentInput.View()),
		"",
		fieldLabelStyle.Render("Logs"),
	)

	if len(m.goalLogs) == 0 {
		lines = append(lines, mutedStyle.Render("No log entries yet."))
	} else {
		for _, entry := range m.goalLogs {
			sign := "+"
			if entry.DeltaAccumulatedCents < 0 {
				sign = ""
			}

			note := strings.TrimSpace(entry.Note)
			if note != "" {
				lines = append(lines, fmt.Sprintf("%s%s at %s | %s", sign, formatAmount(entry.DeltaAccumulatedCents), entry.CreatedAt.Local().Format("2006-01-02 15:04"), note))
			} else {
				lines = append(lines, fmt.Sprintf("%s%s at %s", sign, formatAmount(entry.DeltaAccumulatedCents), entry.CreatedAt.Local().Format("2006-01-02 15:04")))
			}
		}
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderEditGoalField(field int, label string, value string) string {
	prefix := "  "
	if m.editGoalForm.ActiveField == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderGoalNew(width int) string {
	lines := []string{
		headlineStyle.Render("New goal"),
		mutedStyle.Render("Use up/down to move fields. Left/right changes currency. Enter on last field saves."),
		"",
		m.renderGoalRowText(goal.GoalFieldName, "Goal", m.addGoalForm.Inputs[0].View()),
		m.renderGoalChoiceRow(goal.GoalFieldCurrency, "Currency", m.addGoalForm.CurrencyOptions, m.addGoalForm.CurrencyIndex),
		m.renderGoalRowText(goal.GoalFieldTargetAmount, "Target amount", m.addGoalForm.Inputs[1].View()),
		m.renderGoalRowText(goal.GoalFieldAccumulated, "Accumulated", m.addGoalForm.Inputs[2].View()),
		m.renderGoalRowText(goal.GoalFieldDescription, "Description", m.addGoalForm.Inputs[3].View()),
		m.renderGoalRowText(goal.GoalFieldDateStarted, "Date started", m.addGoalForm.Inputs[4].View()),
		m.renderGoalRowText(goal.GoalFieldTargetDate, "Target date", m.addGoalForm.Inputs[5].View()),
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderGoalRowText(field int, label string, value string) string {
	prefix := "  "
	if m.addGoalForm.Active == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderGoalChoiceRow(field int, label string, options []string, selected int) string {
	prefix := "  "
	if m.addGoalForm.Active == field {
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

func (m TheApplication) renderGoalList(width int) string {
	title := "Active goals"

	if m.goalMode == goalListHistory {
		title = "Goal history (completed)"
	}

	lines := []string{lipgloss.JoinHorizontal(lipgloss.Center, sectionTitleStyle.Render(title), "  ", modeBadgeStyle.Render("Goals")), hintStyle.Render("Use up/down to browse. Enter edits goal. Delete/Backspace asks confirmation. Esc returns to menu."), ""}

	filtered := m.filteredGoals()
	if len(filtered) == 0 {
		lines = append(lines, mutedStyle.Render("No goals found."))

		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	accumulatedBase, targetBase := m.goalProgressTotalsBase(filtered)
	lines = append(lines, fieldLabelStyle.Render("Overall progress ("+m.baseCurrencyLabel()+")"))
	lines = append(lines, "  "+m.renderProgressBar(accumulatedBase, targetBase, 28))
	lines = append(lines, "")

	lines = append(lines, m.renderGoalTableHeader(width))
	for i, item := range filtered {
		lines = append(lines, m.renderGoalTableRow(width, i, item))
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderGoalTableHeader(width int) string {
	nameWidth := 12
	currencyWidth := 8
	targetWidth := 12
	accumWidth := 12
	leftWidth := 12
	startedWidth := 10
	targetDateWidth := 10
	progressWidth := 8

	descWidth := width - 14 - nameWidth - currencyWidth - targetWidth - accumWidth - leftWidth - startedWidth - targetDateWidth - progressWidth - 18
	if descWidth < 18 {
		descWidth = 18
	}

	header := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", "#", nameWidth, "Goal", currencyWidth, "Curr", targetWidth, "Target", accumWidth, "Saved", leftWidth, "Left", startedWidth, "Started", targetDateWidth, "Target dt", progressWidth, "Done", descWidth, "Description")

	return tableHeaderStyle.Render(header)
}

func (m TheApplication) updateGoalNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

		return m, nil
	case "up", "shift+tab":
		m.addGoalForm = m.addGoalForm.Prev()

		return m, nil
	case "down", "tab":
		m.addGoalForm = m.addGoalForm.Next()

		return m, nil
	case "left":
		if m.addGoalForm.Active == goal.GoalFieldCurrency && m.addGoalForm.CurrencyIndex > 0 {
			m.addGoalForm.CurrencyIndex--
		}

		return m, nil
	case "right":
		if m.addGoalForm.Active == goal.GoalFieldCurrency && m.addGoalForm.CurrencyIndex < len(m.addGoalForm.CurrencyOptions)-1 {
			m.addGoalForm.CurrencyIndex++
		}

		return m, nil
	case "enter":
		if m.addGoalForm.Active == goal.GoalFieldCount-1 {
			return m.saveGoalFromForm()
		}
		m.addGoalForm = m.addGoalForm.Next()

		return m, nil
	}

	if inputIndex := m.addGoalForm.InputIndexForField(m.addGoalForm.Active); inputIndex >= 0 {
		var cmd tea.Cmd
		m.addGoalForm.Inputs[inputIndex], cmd = m.addGoalForm.Inputs[inputIndex].Update(msg)

		return m, cmd
	}

	return m, nil
}

func (m TheApplication) updateGoalList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if updatedModel, cmd, handled := m.handleDeleteConfirmation(msg); handled {
		return updatedModel, cmd
	}

	filtered := m.filteredGoals()
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
		if m.goalCursor > 0 {
			m.goalCursor--
		}

		return m, nil
	case "down":
		if m.goalCursor < len(filtered)-1 {
			m.goalCursor++
		}

		return m, nil
	case "enter":
		m = m.openGoalEditor(filtered[m.goalCursor]).(TheApplication)

		return m, nil
	case "backspace", "delete":
		selected := filtered[m.goalCursor]
		m = m.beginDeleteConfirmation("goal", selected.ID, selected.Name)

		return m, nil
	default:
		return m, nil
	}
}

func (m TheApplication) openGoalEditor(selected goal.Goal) tea.Model {
	m.screen = screenGoalEdit
	m.editingGoalID = selected.ID
	m.editGoalForm = goal.NewEditGoalForm()
	m.editGoalForm.TargetAmountInput.SetValue(formatAmount(selected.TargetAmountCents))
	m.editGoalForm.AccumulatedAmountInput.SetValue(formatAmount(selected.AmountAccumulatedCents))
	m.editGoalForm.DateStartedInput.SetValue(selected.DateStartedAt.Local().Format("02.01.2006"))

	if selected.TargetDate != nil {
		m.editGoalForm.TargetDateInput.SetValue(selected.TargetDate.Local().Format("02.01.2006"))
	}

	m.editGoalForm.DescriptionInput.SetValue(selected.Description)
	m.editGoalForm.LogDateInput.SetValue(time.Now().Format("02.01.2006"))
	m.editGoalForm.LogCommentInput.SetValue("")
	m.editGoalForm.NameLabel = selected.Name
	m.editGoalForm.CurrencyLabel = selected.Currency
	m.editGoalForm = m.editGoalForm.FocusActive()

	if logs, err := m.storage.LoadGoalLogs(selected.ID); err == nil {
		m.goalLogs = logs
	} else {
		m.goalLogs = nil
	}

	m.status = "editing goal " + selected.Name

	return m
}

func (m TheApplication) updateGoalEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenGoalList
		m.status = "goal edit cancelled"

		return m, nil
	case "up", "shift+tab":
		m.editGoalForm = m.editGoalForm.Prev()

		return m, nil
	case "down", "tab":
		m.editGoalForm = m.editGoalForm.Next()

		return m, nil
	case "enter":
		if m.editGoalForm.ActiveField == goal.EditGoalFieldLogComment {
			return m.applyGoalLogDelta()
		}

		if m.editGoalForm.ActiveField == goal.EditGoalFieldDescription {
			return m.saveGoalEdit()
		}

		m.editGoalForm = m.editGoalForm.Next()

		return m, nil
	}

	var cmd tea.Cmd
	switch m.editGoalForm.ActiveField {
	case goal.EditGoalFieldTargetAmount:
		m.editGoalForm.TargetAmountInput, cmd = m.editGoalForm.TargetAmountInput.Update(msg)
	case goal.EditGoalFieldAccumulated:
		m.editGoalForm.AccumulatedAmountInput, cmd = m.editGoalForm.AccumulatedAmountInput.Update(msg)
	case goal.EditGoalFieldDateStarted:
		m.editGoalForm.DateStartedInput, cmd = m.editGoalForm.DateStartedInput.Update(msg)
	case goal.EditGoalFieldTargetDate:
		m.editGoalForm.TargetDateInput, cmd = m.editGoalForm.TargetDateInput.Update(msg)
	case goal.EditGoalFieldDescription:
		m.editGoalForm.DescriptionInput, cmd = m.editGoalForm.DescriptionInput.Update(msg)
	case goal.EditGoalFieldLogDelta:
		m.editGoalForm.LogDeltaInput, cmd = m.editGoalForm.LogDeltaInput.Update(msg)
	case goal.EditGoalFieldLogDate:
		m.editGoalForm.LogDateInput, cmd = m.editGoalForm.LogDateInput.Update(msg)
	case goal.EditGoalFieldLogComment:
		m.editGoalForm.LogCommentInput, cmd = m.editGoalForm.LogCommentInput.Update(msg)
	}

	return m, cmd
}

func (m TheApplication) saveGoalFromForm() (tea.Model, tea.Cmd) {
	currency := selectedCurrencyOption(m.addGoalForm.CurrencyOptions, m.addGoalForm.CurrencyIndex)
	inputs := m.addGoalForm.Inputs

	newGoal, err := goal.New(inputs[0].Value(), currency, inputs[1].Value(), inputs[2].Value(), inputs[3].Value(), inputs[4].Value(), inputs[5].Value(), dates.TUI, time.Now())
	if err != nil {
		m.status = err.Error()

		return m, nil
	}

	name := newGoal.Name

	if err := m.storage.CreateGoal(&newGoal); err != nil {
		m.status = "save failed: " + err.Error()

		return m, nil
	}

	m.goals = append([]goal.Goal{newGoal}, m.goals...)
	m.addGoalForm = goal.NewAddGoalForm(currencySelectionOptions(m.settings))
	m.screen = screenGoalList

	if newGoal.IsDone() {
		m.goalMode = goalListHistory
	} else {
		m.goalMode = goalListActive
	}

	m.goalCursor = 0
	m.status = "saved goal " + name

	return m, nil
}

func (m TheApplication) saveGoalEdit() (tea.Model, tea.Cmd) {
	index := m.findGoalIndex(m.editingGoalID)
	if index < 0 {
		m.status = "goal not found"

		return m, nil
	}

	form := m.editGoalForm
	selected, err := m.goals[index].Edit(form.TargetAmountInput.Value(), form.AccumulatedAmountInput.Value(), form.DateStartedInput.Value(), form.TargetDateInput.Value(), form.DescriptionInput.Value(), dates.TUI, time.Now())
	if err != nil {
		m.status = err.Error()

		return m, nil
	}

	if err := m.storage.SaveGoal(&selected); err != nil {
		m.status = "save failed: " + err.Error()

		return m, nil
	}

	m.goals[index] = selected
	m.screen = screenGoalList
	m.status = "updated goal " + selected.Name

	return m, nil
}

func (m TheApplication) applyGoalLogDelta() (tea.Model, tea.Cmd) {
	index := m.findGoalIndex(m.editingGoalID)
	if index < 0 {
		m.status = "goal not found"
		return m, nil
	}

	form := m.editGoalForm
	selected, entry, err := m.goals[index].ApplyDelta(form.LogDeltaInput.Value(), form.LogDateInput.Value(), form.LogCommentInput.Value(), dates.TUI, time.Now())
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	if err := m.storage.SaveGoal(&selected); err != nil {
		m.status = "goal update failed: " + err.Error()
		return m, nil
	}

	if err := m.storage.CreateGoalLog(&entry); err != nil {
		m.status = "log save failed: " + err.Error()

		return m, nil
	}

	m.goals[index] = selected
	m.goalLogs = append([]goal.GoalLog{entry}, m.goalLogs...)
	m.editGoalForm.AccumulatedAmountInput.SetValue(formatAmount(selected.AmountAccumulatedCents))
	m.editGoalForm.LogDeltaInput.SetValue("")
	m.editGoalForm.LogDateInput.SetValue(time.Now().Format("02.01.2006"))
	m.editGoalForm.LogCommentInput.SetValue("")
	m.status = "applied goal log delta"

	return m, nil
}

func (m TheApplication) findGoalIndex(id uint) int {
	for i := range m.goals {
		if m.goals[i].ID == id {
			return i
		}
	}

	return -1
}

func (m TheApplication) filteredGoals() []goal.Goal {
	if m.goalMode == goalListHistory {
		return goal.Filter(m.goals, goal.ListDone)
	}

	return goal.Filter(m.goals, goal.ListActive)
}

func (m TheApplication) goalProgressTotalsBase(items []goal.Goal) (accumulated int64, target int64) {
	return goal.ProgressInBaseCents(items, m.settings)
}
