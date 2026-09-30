package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lazybark/cents/flows/debt"
)

func (m TheApplication) renderDebtList(width int) string {
	title := "Outgoing debts"

	if m.debtMode == debtListIncoming {
		title = "Incoming debts"
	}

	if m.debtMode == debtListHistory {
		title = "Debt history (paid)"
	}

	lines := []string{lipgloss.JoinHorizontal(lipgloss.Center, sectionTitleStyle.Render(title), "  ", modeBadgeStyle.Render("Debts")), hintStyle.Render("Use up/down to browse. Enter edits debt. Delete/Backspace asks confirmation. Esc returns to menu."), ""}
	filtered := m.filteredDebts()

	if len(filtered) == 0 {
		lines = append(lines, mutedStyle.Render("No debts found."))

		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	paidBase, totalBase := m.debtProgressTotalsBase(filtered)
	lines = append(lines, fieldLabelStyle.Render("Overall paid progress ("+m.baseCurrencyLabel()+")"))
	lines = append(lines, "  "+m.renderProgressBar(paidBase, totalBase, 28))
	lines = append(lines, "")

	lines = append(lines, m.renderDebtTableHeader(width))

	for i, item := range filtered {
		lines = append(lines, m.renderDebtTableRow(width, i, item))
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderDebtTableHeader(width int) string {
	peerWidth := 12
	currencyWidth := 8
	amountWidth := 12
	paidWidth := 12
	leftWidth := 12
	dueWidth := 10
	dirWidth := 6

	commentWidth := width - 14 - peerWidth - currencyWidth - amountWidth - paidWidth - leftWidth - dueWidth - dirWidth - 18
	if commentWidth < 18 {
		commentWidth = 18
	}

	header := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", "#", peerWidth, "Peer", currencyWidth, "Curr", amountWidth, "Amount", paidWidth, "Paid", leftWidth, "Left", dueWidth, "Due", dirWidth, "Dir", commentWidth, "Comment")

	return tableHeaderStyle.Render(header)
}

func (m TheApplication) renderDebtTableRow(width int, index int, item debt.Debt) string {
	peerWidth := 12
	currencyWidth := 8
	amountWidth := 12
	paidWidth := 12
	leftWidth := 12
	dueWidth := 10
	dirWidth := 6

	commentWidth := width - 14 - peerWidth - currencyWidth - amountWidth - paidWidth - leftWidth - dueWidth - dirWidth - 18
	if commentWidth < 18 {
		commentWidth = 18
	}

	prefix := " "
	style := rowStyle

	if index == m.debtCursor {
		prefix = ">"
		style = selectedRowStyle
	}

	left := item.AmountCents - item.AmountPaidCents
	if left < 0 {
		left = 0
	}

	due := "-"
	if item.DueDate != nil {
		due = item.DueDate.Local().Format("2006-01-02")
	}

	dir := "out"
	if item.IsOwedToUser {
		dir = "in"
	}

	row := fmt.Sprintf("%s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", prefix, peerWidth, truncateText(item.Peer, peerWidth), currencyWidth, truncateText(item.Currency, currencyWidth), amountWidth, renderMoneyWithCurrency(item.Currency, item.AmountCents), paidWidth, renderMoneyWithCurrency(item.Currency, item.AmountPaidCents), leftWidth, renderMoneyWithCurrency(item.Currency, left), dueWidth, due, dirWidth, dir, commentWidth, truncateText(item.Comment, commentWidth))

	return style.Render(row)
}

func (m TheApplication) renderDebtEdit(width int) string {
	lines := []string{
		headlineStyle.Render("Edit debt"),
		mutedStyle.Render("Edit fields and press Enter on Comment to save."),
		"",
		mutedStyle.Render("Peer: " + m.editDebtForm.PeerLabel + " | Direction: " + m.editDebtForm.DirectionLabel + " | Currency: " + m.editDebtForm.CurrencyLabel),
	}

	if idx := m.findDebtIndex(m.editingDebtID); idx >= 0 {
		item := m.debts[idx]
		lines = append(lines, fieldLabelStyle.Render("Paid progress"))
		lines = append(lines, "  "+m.renderProgressBar(item.AmountPaidCents, item.AmountCents, 28))
	}

	lines = append(lines,
		"",
		m.renderEditDebtField(debt.EditDebtFieldAmount, "Amount", m.editDebtForm.AmountInput.View()),
		m.renderEditDebtField(debt.EditDebtFieldAmountPaid, "Amount paid", m.editDebtForm.AmountPaidInput.View()),
		m.renderEditDebtField(debt.EditDebtFieldDebtCreated, "Debt created", m.editDebtForm.DebtCreatedInput.View()),
		m.renderEditDebtField(debt.EditDebtFieldDueDate, "Due date", m.editDebtForm.DueDateInput.View()),
		m.renderEditDebtField(debt.EditDebtFieldComment, "Comment", m.editDebtForm.CommentInput.View()),
		"",
		fieldLabelStyle.Render("Add transaction"),
		mutedStyle.Render("Set delta/date/comment, then press Enter on Transaction comment to apply."),
		m.renderEditDebtField(debt.EditDebtFieldLogDelta, "Transaction delta", m.editDebtForm.LogDeltaInput.View()),
		m.renderEditDebtField(debt.EditDebtFieldLogDate, "Transaction date", m.editDebtForm.LogDateInput.View()),
		m.renderEditDebtField(debt.EditDebtFieldLogComment, "Transaction comment", m.editDebtForm.LogCommentInput.View()),
		"",
		fieldLabelStyle.Render("Logs"),
	)

	if len(m.debtLogs) == 0 {
		lines = append(lines, mutedStyle.Render("No log entries yet."))
	} else {
		for _, entry := range m.debtLogs {
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

func (m TheApplication) renderEditDebtField(field int, label string, value string) string {
	prefix := "  "
	if m.editDebtForm.ActiveField == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderDebtNew(width int) string {
	directionIndex := 0

	if m.addDebtForm.IsOwedToUser {
		directionIndex = 1
	}

	lines := []string{
		headlineStyle.Render("New debt"),
		mutedStyle.Render("Use up/down to move fields. Left/right changes direction and currency. Space also toggles direction. Enter on last field saves."),
		"",
		m.renderDebtChoiceRow(debt.DebtFieldDirection, "Direction", []string{"outgoing (i owe)", "incoming (owed to me)"}, directionIndex),
		m.renderDebtRowText(debt.DebtFieldPeer, "Peer", m.addDebtForm.Inputs[0].View()),
		m.renderDebtChoiceRow(debt.DebtFieldCurrency, "Currency", m.addDebtForm.CurrencyOptions, m.addDebtForm.CurrencyIndex),
		m.renderDebtRowText(debt.DebtFieldAmount, "Amount", m.addDebtForm.Inputs[1].View()),
		m.renderDebtRowText(debt.DebtFieldAmountPaid, "Amount paid", m.addDebtForm.Inputs[2].View()),
		m.renderDebtRowText(debt.DebtFieldDebtCreated, "Debt created", m.addDebtForm.Inputs[3].View()),
		m.renderDebtRowText(debt.DebtFieldDueDate, "Due date", m.addDebtForm.Inputs[4].View()),
		m.renderDebtRowText(debt.DebtFieldComment, "Comment", m.addDebtForm.Inputs[5].View()),
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderDebtRowText(field int, label string, value string) string {
	prefix := "  "

	if m.addDebtForm.Active == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderDebtChoiceRow(field int, label string, options []string, selected int) string {
	prefix := "  "

	if m.addDebtForm.Active == field {
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

func (m TheApplication) debtProgressTotalsBase(items []debt.Debt) (paid int64, total int64) {
	for _, item := range items {
		itemTotal := item.AmountCents
		itemPaid := item.AmountPaidCents

		if itemTotal < 0 {
			itemTotal = 0
		}

		if itemPaid < 0 {
			itemPaid = 0
		}

		if itemPaid > itemTotal {
			itemPaid = itemTotal
		}

		totalBase, totalOK := m.convertToBaseCents(item.Currency, itemTotal)
		paidBase, paidOK := m.convertToBaseCents(item.Currency, itemPaid)

		if totalOK && paidOK {
			total += totalBase
			paid += paidBase

			continue
		}

		total += itemTotal
		paid += itemPaid
	}

	if paid > total {
		paid = total
	}

	if paid < 0 {
		paid = 0
	}

	if total < 0 {
		total = 0
	}

	return paid, total
}

func (m TheApplication) confirmDeleteDebt() (tea.Model, tea.Cmd) {
	index := m.findDebtIndex(m.deleteConfirmID)
	if index < 0 {
		m = m.clearDeleteConfirmation("debt not found")

		return m, nil
	}

	selected := m.debts[index]
	if err := m.storage.DeleteDebt(selected.ID); err != nil {
		m = m.clearDeleteConfirmation("delete failed: " + err.Error())

		return m, nil
	}

	m.debts = append(m.debts[:index], m.debts[index+1:]...)

	filteredAfter := m.filteredDebts()
	if len(filteredAfter) == 0 {
		m.debtCursor = 0
	} else if m.debtCursor >= len(filteredAfter) {
		m.debtCursor = len(filteredAfter) - 1
	}

	m = m.clearDeleteConfirmation("deleted debt " + selected.Peer)

	return m, nil
}

func (m TheApplication) updateDebtNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)
		return m, nil
	case "up", "shift+tab":
		m.addDebtForm = m.addDebtForm.Prev()
		return m, nil
	case "down", "tab":
		m.addDebtForm = m.addDebtForm.Next()
		return m, nil
	case "left":
		if m.addDebtForm.Active == debt.DebtFieldDirection {
			m.addDebtForm.IsOwedToUser = false
		}
		if m.addDebtForm.Active == debt.DebtFieldCurrency && m.addDebtForm.CurrencyIndex > 0 {
			m.addDebtForm.CurrencyIndex--
		}
		return m, nil
	case "right":
		if m.addDebtForm.Active == debt.DebtFieldDirection {
			m.addDebtForm.IsOwedToUser = true
		}
		if m.addDebtForm.Active == debt.DebtFieldCurrency && m.addDebtForm.CurrencyIndex < len(m.addDebtForm.CurrencyOptions)-1 {
			m.addDebtForm.CurrencyIndex++
		}
		return m, nil
	case " ":
		if m.addDebtForm.Active == debt.DebtFieldDirection {
			m.addDebtForm.IsOwedToUser = !m.addDebtForm.IsOwedToUser
			return m, nil
		}
	case "enter":
		if m.addDebtForm.Active == debt.DebtFieldCount-1 {
			return m.saveDebtFromForm()
		}
		m.addDebtForm = m.addDebtForm.Next()
		return m, nil
	}

	if inputIndex := m.addDebtForm.InputIndexForField(m.addDebtForm.Active); inputIndex >= 0 {
		var cmd tea.Cmd
		m.addDebtForm.Inputs[inputIndex], cmd = m.addDebtForm.Inputs[inputIndex].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m TheApplication) updateDebtList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if updatedModel, cmd, handled := m.handleDeleteConfirmation(msg); handled {
		return updatedModel, cmd
	}

	filtered := m.filteredDebts()
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
		if m.debtCursor > 0 {
			m.debtCursor--
		}
		return m, nil
	case "down":
		if m.debtCursor < len(filtered)-1 {
			m.debtCursor++
		}
		return m, nil
	case "enter":
		m = m.openDebtEditor(filtered[m.debtCursor]).(TheApplication)
		return m, nil
	case "backspace", "delete":
		selected := filtered[m.debtCursor]
		m = m.beginDeleteConfirmation("debt", selected.ID, selected.Peer)
		return m, nil
	default:
		return m, nil
	}
}

func (m TheApplication) openDebtEditor(selected debt.Debt) tea.Model {
	m.screen = screenDebtEdit
	m.editingDebtID = selected.ID
	m.editDebtForm = debt.NewEditDebtForm()
	m.editDebtForm.AmountInput.SetValue(formatAmount(selected.AmountCents))
	m.editDebtForm.AmountPaidInput.SetValue(formatAmount(selected.AmountPaidCents))
	m.editDebtForm.DebtCreatedInput.SetValue(selected.DebtCreatedAt.Local().Format("02.01.2006"))
	if selected.DueDate != nil {
		m.editDebtForm.DueDateInput.SetValue(selected.DueDate.Local().Format("02.01.2006"))
	}
	m.editDebtForm.CommentInput.SetValue(selected.Comment)
	m.editDebtForm.LogDateInput.SetValue(time.Now().Format("02.01.2006"))
	m.editDebtForm.LogCommentInput.SetValue("")
	m.editDebtForm.PeerLabel = selected.Peer
	m.editDebtForm.CurrencyLabel = selected.Currency
	m.editDebtForm.DirectionLabel = debtDirectionLabel(selected.IsOwedToUser)
	m.editDebtForm = m.editDebtForm.FocusActive()

	if logs, err := m.storage.LoadDebtLogs(selected.ID); err == nil {
		m.debtLogs = logs
	} else {
		m.debtLogs = nil
	}

	m.status = "editing debt " + selected.Peer
	return m
}

func (m TheApplication) updateDebtEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenDebtList
		m.status = "debt edit cancelled"
		return m, nil
	case "up", "shift+tab":
		m.editDebtForm = m.editDebtForm.Prev()
		return m, nil
	case "down", "tab":
		m.editDebtForm = m.editDebtForm.Next()
		return m, nil
	case "enter":
		if m.editDebtForm.ActiveField == debt.EditDebtFieldLogComment {
			return m.applyDebtLogDelta()
		}
		if m.editDebtForm.ActiveField == debt.EditDebtFieldComment {
			return m.saveDebtEdit()
		}
		m.editDebtForm = m.editDebtForm.Next()
		return m, nil
	}

	var cmd tea.Cmd
	switch m.editDebtForm.ActiveField {
	case debt.EditDebtFieldAmount:
		m.editDebtForm.AmountInput, cmd = m.editDebtForm.AmountInput.Update(msg)
	case debt.EditDebtFieldAmountPaid:
		m.editDebtForm.AmountPaidInput, cmd = m.editDebtForm.AmountPaidInput.Update(msg)
	case debt.EditDebtFieldDebtCreated:
		m.editDebtForm.DebtCreatedInput, cmd = m.editDebtForm.DebtCreatedInput.Update(msg)
	case debt.EditDebtFieldDueDate:
		m.editDebtForm.DueDateInput, cmd = m.editDebtForm.DueDateInput.Update(msg)
	case debt.EditDebtFieldComment:
		m.editDebtForm.CommentInput, cmd = m.editDebtForm.CommentInput.Update(msg)
	case debt.EditDebtFieldLogDelta:
		m.editDebtForm.LogDeltaInput, cmd = m.editDebtForm.LogDeltaInput.Update(msg)
	case debt.EditDebtFieldLogDate:
		m.editDebtForm.LogDateInput, cmd = m.editDebtForm.LogDateInput.Update(msg)
	case debt.EditDebtFieldLogComment:
		m.editDebtForm.LogCommentInput, cmd = m.editDebtForm.LogCommentInput.Update(msg)
	}
	return m, cmd
}

func (m TheApplication) saveDebtFromForm() (tea.Model, tea.Cmd) {
	peer := strings.TrimSpace(m.addDebtForm.Inputs[0].Value())
	currency := selectedCurrencyOption(m.addDebtForm.CurrencyOptions, m.addDebtForm.CurrencyIndex)
	amountRaw := strings.TrimSpace(m.addDebtForm.Inputs[1].Value())
	amountPaidRaw := strings.TrimSpace(m.addDebtForm.Inputs[2].Value())
	debtCreatedRaw := strings.TrimSpace(m.addDebtForm.Inputs[3].Value())
	dueRaw := strings.TrimSpace(m.addDebtForm.Inputs[4].Value())
	comment := strings.TrimSpace(m.addDebtForm.Inputs[5].Value())

	if peer == "" {
		m.status = "peer is required"
		return m, nil
	}

	amount, err := parseAmountCents(amountRaw)
	if err != nil {
		m.status = "amount error: " + err.Error()
		return m, nil
	}

	amountPaid, err := parseAmountCents(amountPaidRaw)
	if err != nil {
		m.status = "amount paid error: " + err.Error()
		return m, nil
	}
	if amountPaid > amount {
		m.status = "amount paid cannot be more than amount"
		return m, nil
	}

	debtCreatedAt, err := parseRequiredDate(debtCreatedRaw)
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	dueDate, err := parseOptionalDatePointer(dueRaw)
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	now := time.Now()
	newDebt := debt.Debt{
		Peer:            peer,
		Currency:        currency,
		AmountCents:     amount,
		AmountPaidCents: amountPaid,
		IsOwedToUser:    m.addDebtForm.IsOwedToUser,
		DebtCreatedAt:   debtCreatedAt,
		DueDate:         dueDate,
		Comment:         comment,
		LastUpdatedAt:   now,
	}

	if err := m.storage.CreateDebt(&newDebt); err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.debts = append([]debt.Debt{newDebt}, m.debts...)
	m.addDebtForm = debt.NewAddDebtForm(currencySelectionOptions(m.settings))
	m.screen = screenDebtList
	if newDebt.IsOwedToUser {
		m.debtMode = debtListIncoming
	} else {
		m.debtMode = debtListOutgoing
	}
	m.debtCursor = 0
	m.status = "saved debt for " + peer
	return m, nil
}

func (m TheApplication) saveDebtEdit() (tea.Model, tea.Cmd) {
	index := m.findDebtIndex(m.editingDebtID)
	if index < 0 {
		m.status = "debt not found"
		return m, nil
	}

	amount, err := parseAmountCents(strings.TrimSpace(m.editDebtForm.AmountInput.Value()))
	if err != nil {
		m.status = "amount error: " + err.Error()
		return m, nil
	}
	amountPaid, err := parseAmountCents(strings.TrimSpace(m.editDebtForm.AmountPaidInput.Value()))
	if err != nil {
		m.status = "amount paid error: " + err.Error()
		return m, nil
	}
	if amountPaid > amount {
		m.status = "amount paid cannot be more than amount"
		return m, nil
	}

	debtCreatedAt, err := parseRequiredDate(strings.TrimSpace(m.editDebtForm.DebtCreatedInput.Value()))
	if err != nil {
		m.status = err.Error()
		return m, nil
	}
	dueDate, err := parseOptionalDatePointer(strings.TrimSpace(m.editDebtForm.DueDateInput.Value()))
	if err != nil {
		m.status = err.Error()
		return m, nil
	}

	selected := m.debts[index]
	selected.AmountCents = amount
	selected.AmountPaidCents = amountPaid
	selected.DebtCreatedAt = debtCreatedAt
	selected.DueDate = dueDate
	selected.Comment = strings.TrimSpace(m.editDebtForm.CommentInput.Value())
	selected.LastUpdatedAt = time.Now()

	if err := m.storage.SaveDebt(&selected); err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.debts[index] = selected
	m.screen = screenDebtList
	m.status = "updated debt for " + selected.Peer
	return m, nil
}

func (m TheApplication) applyDebtLogDelta() (tea.Model, tea.Cmd) {
	index := m.findDebtIndex(m.editingDebtID)
	if index < 0 {
		m.status = "debt not found"
		return m, nil
	}

	delta, err := parseSignedAmountCents(strings.TrimSpace(m.editDebtForm.LogDeltaInput.Value()))
	if err != nil {
		m.status = "log delta error: " + err.Error()
		return m, nil
	}
	if delta == 0 {
		m.status = "delta cannot be zero"
		return m, nil
	}

	selected := m.debts[index]
	nextPaid := selected.AmountPaidCents + delta
	if nextPaid < 0 || nextPaid > selected.AmountCents {
		m.status = "delta makes amount paid out of range"
		return m, nil
	}

	entryTime, err := parseLogDateOrToday(strings.TrimSpace(m.editDebtForm.LogDateInput.Value()))
	if err != nil {
		m.status = err.Error()
		return m, nil
	}
	now := time.Now()
	selected.AmountPaidCents = nextPaid
	selected.LastUpdatedAt = now
	if err := m.storage.SaveDebt(&selected); err != nil {
		m.status = "debt update failed: " + err.Error()
		return m, nil
	}

	note := strings.TrimSpace(m.editDebtForm.LogCommentInput.Value())
	if note == "" {
		note = "manual paid adjustment"
	}
	entry := debt.DebtLog{
		DebtID:         selected.ID,
		DeltaPaidCents: delta,
		Note:           note,
		CreatedAt:      entryTime,
	}
	if err := m.storage.CreateDebtLog(&entry); err != nil {
		m.status = "log save failed: " + err.Error()
		return m, nil
	}

	m.debts[index] = selected
	m.debtLogs = append([]debt.DebtLog{entry}, m.debtLogs...)
	m.editDebtForm.AmountPaidInput.SetValue(formatAmount(selected.AmountPaidCents))
	m.editDebtForm.LogDeltaInput.SetValue("")
	m.editDebtForm.LogDateInput.SetValue(time.Now().Format("02.01.2006"))
	m.editDebtForm.LogCommentInput.SetValue("")
	m.status = "applied log delta"
	return m, nil
}

func (m TheApplication) findDebtIndex(id uint) int {
	for i := range m.debts {
		if m.debts[i].ID == id {
			return i
		}
	}
	return -1
}

func (m TheApplication) filteredDebts() []debt.Debt {
	filtered := make([]debt.Debt, 0, len(m.debts))
	for _, item := range m.debts {
		paid := item.AmountPaidCents >= item.AmountCents
		switch m.debtMode {
		case debtListOutgoing:
			if !item.IsOwedToUser && !paid {
				filtered = append(filtered, item)
			}
		case debtListIncoming:
			if item.IsOwedToUser && !paid {
				filtered = append(filtered, item)
			}
		case debtListHistory:
			if paid {
				filtered = append(filtered, item)
			}
		}
	}
	return filtered
}
