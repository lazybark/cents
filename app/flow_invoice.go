package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/invoice"
)

func (m TheApplication) confirmDeleteInvoice() (tea.Model, tea.Cmd) {
	index := m.findInvoiceIndex(m.deleteConfirmID)
	if index < 0 {
		m = m.clearDeleteConfirmation("invoice not found")
		return m, nil
	}

	selected := m.invoices[index]

	if err := m.storage.DeleteInvoice(selected.ID); err != nil {
		m = m.clearDeleteConfirmation("delete failed: " + err.Error())

		return m, nil
	}

	m.invoices = append(m.invoices[:index], m.invoices[index+1:]...)
	filteredAfter := m.filteredInvoices()

	if len(filteredAfter) == 0 {
		m.invoiceCursor = 0
	} else if m.invoiceCursor >= len(filteredAfter) {
		m.invoiceCursor = len(filteredAfter) - 1
	}

	m = m.clearDeleteConfirmation("deleted invoice " + selected.Title)

	return m, nil
}

func (m TheApplication) renderInvoiceRowText(field int, label string, value string) string {
	prefix := "  "
	if m.addInvoiceForm.Active == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderInvoiceChoiceRow(field int, label string, options []string, selected int) string {
	prefix := "  "
	if m.addInvoiceForm.Active == field {
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

func (m TheApplication) renderEditInvoiceField(field int, label string, value string) string {
	prefix := "  "
	if m.editInvoiceForm.ActiveField == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderEditInvoiceChoiceRow(field int, label string, options []string, selected int) string {
	prefix := "  "
	if m.editInvoiceForm.ActiveField == field {
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

func (m TheApplication) renderInvoiceTableHeader(width int) string {
	titleWidth := 14
	typeWidth := 8
	currencyWidth := 7
	amountWidth := 12
	paidWidth := 6
	peerWidth := 12
	invDateWidth := 10
	dueDateWidth := 10
	accountWidth := 16

	descWidth := width - 14 - titleWidth - typeWidth - currencyWidth - amountWidth - paidWidth - peerWidth - invDateWidth - dueDateWidth - accountWidth - 20
	if descWidth < 16 {
		descWidth = 16
	}

	header := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", "#", titleWidth, "Title", typeWidth, "Type", currencyWidth, "Curr", amountWidth, "Amount", paidWidth, "Paid", peerWidth, "Peer", invDateWidth, "Issued", dueDateWidth, "Due", accountWidth, "Account", descWidth, "Description")

	return tableHeaderStyle.Render(header)
}

func (m TheApplication) renderInvoiceTableRow(width int, index int, item invoice.Invoice) string {
	titleWidth := 14
	typeWidth := 8
	currencyWidth := 7
	amountWidth := 12
	paidWidth := 6
	peerWidth := 12
	invDateWidth := 10
	dueDateWidth := 10
	accountWidth := 16

	descWidth := width - 14 - titleWidth - typeWidth - currencyWidth - amountWidth - paidWidth - peerWidth - invDateWidth - dueDateWidth - accountWidth - 20
	if descWidth < 16 {
		descWidth = 16
	}

	prefix := " "
	style := rowStyle
	if index == m.invoiceCursor {
		prefix = ">"
		style = selectedRowStyle
	}

	typeLabel := "in"
	if !item.IsIncoming {
		typeLabel = "out"
	}

	paidLabel := "no"
	if item.Paid {
		paidLabel = "yes"
	}

	issued := "-"
	if item.InvoiceDate != nil {
		issued = item.InvoiceDate.Local().Format("2006-01-02")
	}

	due := "-"
	if item.DueDate != nil {
		due = item.DueDate.Local().Format("2006-01-02")
	}

	row := fmt.Sprintf("%s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", prefix, titleWidth, truncateText(item.Title, titleWidth), typeWidth, typeLabel, currencyWidth, truncateText(item.Currency, currencyWidth), amountWidth, renderMoneyWithCurrency(item.Currency, item.AmountCents), paidWidth, paidLabel, peerWidth, truncateText(item.Peer, peerWidth), invDateWidth, issued, dueDateWidth, due, accountWidth, truncateText(item.TargetAccount, accountWidth), descWidth, truncateText(item.Description, descWidth))

	return style.Render(row)
}

func (m TheApplication) renderInvoiceNew(width int) string {
	typeIndex := 0
	if !m.addInvoiceForm.IsIncoming {
		typeIndex = 1
	}

	paidMarker := "[ ]"
	if m.addInvoiceForm.Paid {
		paidMarker = "[x]"
	}

	lines := []string{
		headlineStyle.Render("New invoice"),
		mutedStyle.Render("Only title and type are required. Pick target account or type custom override."),
		"",
		m.renderInvoiceRowText(invoice.InvoiceFieldTitle, "Title", m.addInvoiceForm.Inputs[0].View()),
		m.renderInvoiceChoiceRow(invoice.InvoiceFieldType, "Type", []string{"incoming (i must pay)", "outgoing (they pay me)"}, typeIndex),
		m.renderInvoiceChoiceRow(invoice.InvoiceFieldCurrency, "Currency", m.addInvoiceForm.CurrencyOptions, m.addInvoiceForm.CurrencyIndex),
		m.renderInvoiceRowText(invoice.InvoiceFieldAmount, "Amount", m.addInvoiceForm.Inputs[1].View()),
		m.renderInvoiceRowText(invoice.InvoiceFieldPaid, "Paid", paidMarker),
		m.renderInvoiceRowText(invoice.InvoiceFieldPeer, "Peer", m.addInvoiceForm.Inputs[2].View()),
		m.renderInvoiceRowText(invoice.InvoiceFieldInvoiceDate, "Invoice date", m.addInvoiceForm.Inputs[3].View()),
		m.renderInvoiceRowText(invoice.InvoiceFieldDueDate, "Due date", m.addInvoiceForm.Inputs[4].View()),
		m.renderInvoiceChoiceRow(invoice.InvoiceFieldTargetAccountChoice, "Target account (pick)", cashflowAccountDisplayOptions(m.addInvoiceForm.AccountOptions), m.addInvoiceForm.AccountIndex),
		m.renderInvoiceRowText(invoice.InvoiceFieldTargetAccountName, "Target account (type)", m.addInvoiceForm.Inputs[5].View()),
		m.renderInvoiceRowText(invoice.InvoiceFieldURL, "URL", m.addInvoiceForm.Inputs[6].View()),
		m.renderInvoiceRowText(invoice.InvoiceFieldDescription, "Description", m.addInvoiceForm.Inputs[7].View()),
		"",
		mutedStyle.Render("Space toggles Paid when active."),
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderInvoiceList(width int) string {
	title := "Outgoing unpaid invoices"
	if m.invoiceMode == invoiceListIncomingUnpaid {
		title = "Incoming unpaid invoices"
	}

	if m.invoiceMode == invoiceListHistoryPaid {
		title = "Invoice history (paid)"
	}

	lines := []string{lipgloss.JoinHorizontal(lipgloss.Center, sectionTitleStyle.Render(title), "  ", modeBadgeStyle.Render("Invoices")), hintStyle.Render("Up/down selects row. Left/right or PgUp/PgDn changes page. Enter edits invoice. Delete/Backspace asks confirmation. Esc returns to menu."), ""}

	filtered := m.filteredInvoices()
	if len(filtered) == 0 {
		lines = append(lines, mutedStyle.Render("No invoices found."))
		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	pageSize := m.invoiceListPageSize()

	totalPages := (len(filtered) + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}

	currentPage := clamp(m.invoicePage, 0, totalPages-1)

	start := currentPage * pageSize
	if start < 0 {
		start = 0
	}

	if start > len(filtered) {
		start = len(filtered)
	}

	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}

	lines = append(lines, fmt.Sprintf("Page %d/%d", currentPage+1, totalPages), "")
	lines = append(lines, m.renderInvoiceTableHeader(width))

	for i := start; i < end; i++ {
		lines = append(lines, m.renderInvoiceTableRow(width, i, filtered[i]))
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderInvoiceEdit(width int) string {
	typeIndex := 0
	if !m.editInvoiceForm.IsIncoming {
		typeIndex = 1
	}

	paidMarker := "[ ]"
	if m.editInvoiceForm.Paid {
		paidMarker = "[x]"
	}

	lines := []string{
		headlineStyle.Render("Edit invoice"),
		mutedStyle.Render("Edit any field and press Enter on Description to save."),
		"",
		m.renderEditInvoiceField(invoice.InvoiceFieldTitle, "Title", m.editInvoiceForm.TitleInput.View()),
		m.renderEditInvoiceChoiceRow(invoice.InvoiceFieldType, "Type", []string{"incoming (i must pay)", "outgoing (they pay me)"}, typeIndex),
		m.renderEditInvoiceChoiceRow(invoice.InvoiceFieldCurrency, "Currency", m.editInvoiceForm.CurrencyOptions, m.editInvoiceForm.CurrencyIndex),
		m.renderEditInvoiceField(invoice.InvoiceFieldAmount, "Amount", m.editInvoiceForm.AmountInput.View()),
		m.renderEditInvoiceField(invoice.InvoiceFieldPaid, "Paid", paidMarker),
		m.renderEditInvoiceField(invoice.InvoiceFieldPeer, "Peer", m.editInvoiceForm.PeerInput.View()),
		m.renderEditInvoiceField(invoice.InvoiceFieldInvoiceDate, "Invoice date", m.editInvoiceForm.InvoiceDateInput.View()),
		m.renderEditInvoiceField(invoice.InvoiceFieldDueDate, "Due date", m.editInvoiceForm.DueDateInput.View()),
		m.renderEditInvoiceChoiceRow(invoice.InvoiceFieldTargetAccountChoice, "Target account (pick)", cashflowAccountDisplayOptions(m.editInvoiceForm.AccountOptions), m.editInvoiceForm.AccountIndex),
		m.renderEditInvoiceField(invoice.InvoiceFieldTargetAccountName, "Target account (type)", m.editInvoiceForm.TargetAccountInput.View()),
		m.renderEditInvoiceField(invoice.InvoiceFieldURL, "URL", m.editInvoiceForm.URLInput.View()),
		m.renderEditInvoiceField(invoice.InvoiceFieldDescription, "Description", m.editInvoiceForm.DescriptionInput.View()),
		"",
		mutedStyle.Render("Space toggles Paid when active."),
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) saveInvoiceFromForm() (tea.Model, tea.Cmd) {
	form := m.addInvoiceForm
	inputs := form.Inputs

	targetAccount := strings.TrimSpace(inputs[5].Value())
	if targetAccount == "" {
		targetAccount = selectedStringOption(form.AccountOptions, form.AccountIndex)
	}

	item, err := invoice.New(invoice.Fields{
		Title:         inputs[0].Value(),
		IsIncoming:    form.IsIncoming,
		Currency:      selectedCurrencyOption(form.CurrencyOptions, form.CurrencyIndex),
		Amount:        inputs[1].Value(),
		Paid:          form.Paid,
		Peer:          inputs[2].Value(),
		InvoiceDate:   inputs[3].Value(),
		DueDate:       inputs[4].Value(),
		TargetAccount: targetAccount,
		URL:           inputs[6].Value(),
		Description:   inputs[7].Value(),
	}, m.settings, dates.TUI, time.Now())
	if err != nil {
		m.status = err.Error()

		return m, nil
	}

	title := item.Title

	if err := m.storage.CreateInvoice(&item); err != nil {
		m.status = "save failed: " + err.Error()

		return m, nil
	}

	m.invoices = append([]invoice.Invoice{item}, m.invoices...)
	m.addInvoiceForm = invoice.NewAddInvoiceForm(currencySelectionOptions(m.settings), accountSelectionOptions(m.accounts))
	m.screen = screenInvoiceList

	switch item.Mode() {
	case invoice.ListPaid:
		m.invoiceMode = invoiceListHistoryPaid
	case invoice.ListIncoming:
		m.invoiceMode = invoiceListIncomingUnpaid
	default:
		m.invoiceMode = invoiceListOutgoingUnpaid
	}

	m.invoiceCursor = 0
	m.invoicePage = 0
	m.status = "saved invoice " + title

	return m, nil
}

func (m TheApplication) openInvoiceEditor(item invoice.Invoice) tea.Model {
	m.screen = screenInvoiceEdit
	m.editingInvoiceID = item.ID
	m.editInvoiceForm = invoice.NewEditInvoiceForm(currencySelectionOptions(m.settings), accountSelectionOptions(m.accounts))
	m.editInvoiceForm.TitleInput.SetValue(item.Title)

	if item.AmountCents != 0 {
		m.editInvoiceForm.AmountInput.SetValue(formatAmount(item.AmountCents))
	} else {
		m.editInvoiceForm.AmountInput.SetValue("")
	}

	m.editInvoiceForm.PeerInput.SetValue(item.Peer)
	if item.InvoiceDate != nil {
		m.editInvoiceForm.InvoiceDateInput.SetValue(item.InvoiceDate.Local().Format("02.01.2006"))
	}

	if item.DueDate != nil {
		m.editInvoiceForm.DueDateInput.SetValue(item.DueDate.Local().Format("02.01.2006"))
	}

	m.editInvoiceForm.TargetAccountInput.SetValue(item.TargetAccount)

	m.editInvoiceForm.URLInput.SetValue(item.URL)
	m.editInvoiceForm.DescriptionInput.SetValue(item.Description)
	m.editInvoiceForm.CurrencyIndex = 0
	m.editInvoiceForm.AccountIndex = 0

	for i := range m.editInvoiceForm.CurrencyOptions {
		if strings.EqualFold(strings.TrimSpace(m.editInvoiceForm.CurrencyOptions[i]), strings.TrimSpace(item.Currency)) {
			m.editInvoiceForm.CurrencyIndex = i

			break
		}
	}

	for i := range m.editInvoiceForm.AccountOptions {
		if strings.EqualFold(strings.TrimSpace(m.editInvoiceForm.AccountOptions[i]), strings.TrimSpace(item.TargetAccount)) {
			m.editInvoiceForm.AccountIndex = i

			break
		}
	}
	m.editInvoiceForm.IsIncoming = item.IsIncoming
	m.editInvoiceForm.Paid = item.Paid
	m.editInvoiceForm = m.editInvoiceForm.FocusActive()
	m.status = "editing invoice " + item.Title

	return m
}

func (m TheApplication) saveInvoiceEdit() (tea.Model, tea.Cmd) {
	index := m.findInvoiceIndex(m.editingInvoiceID)
	if index < 0 {
		m.status = "invoice not found"

		return m, nil
	}

	form := m.editInvoiceForm

	targetAccount := strings.TrimSpace(form.TargetAccountInput.Value())
	if targetAccount == "" {
		targetAccount = selectedStringOption(form.AccountOptions, form.AccountIndex)
	}

	selected, err := m.invoices[index].Edit(invoice.Fields{
		Title:         form.TitleInput.Value(),
		IsIncoming:    form.IsIncoming,
		Currency:      selectedCurrencyOption(form.CurrencyOptions, form.CurrencyIndex),
		Amount:        form.AmountInput.Value(),
		Paid:          form.Paid,
		Peer:          form.PeerInput.Value(),
		InvoiceDate:   form.InvoiceDateInput.Value(),
		DueDate:       form.DueDateInput.Value(),
		TargetAccount: targetAccount,
		URL:           form.URLInput.Value(),
		Description:   form.DescriptionInput.Value(),
	}, m.settings, dates.TUI, time.Now())
	if err != nil {
		m.status = err.Error()

		return m, nil
	}

	if err := m.storage.SaveInvoice(&selected); err != nil {
		m.status = "save failed: " + err.Error()

		return m, nil
	}

	m.invoices[index] = selected
	m.screen = screenInvoiceList
	m.status = "updated invoice " + selected.Title

	return m, nil
}

func (m TheApplication) filteredInvoices() []invoice.Invoice {
	switch m.invoiceMode {
	case invoiceListIncomingUnpaid:
		return invoice.Filter(m.invoices, invoice.ListIncoming)
	case invoiceListHistoryPaid:
		return invoice.Filter(m.invoices, invoice.ListPaid)
	default:
		return invoice.Filter(m.invoices, invoice.ListOutgoing)
	}
}

func (m TheApplication) invoiceListPageSize() int {
	if m.height <= 0 {
		return 12
	}

	size := m.height - 18
	if size < 5 {
		size = 5
	}

	return size
}

func (m TheApplication) findInvoiceIndex(id uint) int {
	for i := range m.invoices {
		if m.invoices[i].ID == id {
			return i
		}
	}

	return -1
}

func (m TheApplication) updateInvoiceNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

		return m, nil
	case "up", "shift+tab":
		m.addInvoiceForm = m.addInvoiceForm.Prev()

		return m, nil
	case "down", "tab":
		m.addInvoiceForm = m.addInvoiceForm.Next()

		return m, nil
	case "left":
		if m.addInvoiceForm.Active == invoice.InvoiceFieldType {
			m.addInvoiceForm.IsIncoming = true
		}

		if m.addInvoiceForm.Active == invoice.InvoiceFieldCurrency && m.addInvoiceForm.CurrencyIndex > 0 {
			m.addInvoiceForm.CurrencyIndex--
		}

		if m.addInvoiceForm.Active == invoice.InvoiceFieldTargetAccountChoice && m.addInvoiceForm.AccountIndex > 0 {
			m.addInvoiceForm.AccountIndex--
		}

		return m, nil
	case "right":
		if m.addInvoiceForm.Active == invoice.InvoiceFieldType {
			m.addInvoiceForm.IsIncoming = false
		}

		if m.addInvoiceForm.Active == invoice.InvoiceFieldCurrency && m.addInvoiceForm.CurrencyIndex < len(m.addInvoiceForm.CurrencyOptions)-1 {
			m.addInvoiceForm.CurrencyIndex++
		}

		if m.addInvoiceForm.Active == invoice.InvoiceFieldTargetAccountChoice && m.addInvoiceForm.AccountIndex < len(m.addInvoiceForm.AccountOptions)-1 {
			m.addInvoiceForm.AccountIndex++
		}

		return m, nil
	case " ":
		if m.addInvoiceForm.Active == invoice.InvoiceFieldPaid {
			m.addInvoiceForm.Paid = !m.addInvoiceForm.Paid

			return m, nil
		}
	case "enter":
		if m.addInvoiceForm.Active == invoice.InvoiceFieldCount-1 {
			return m.saveInvoiceFromForm()
		}

		m.addInvoiceForm = m.addInvoiceForm.Next()

		return m, nil
	}

	if inputIndex := m.addInvoiceForm.InputIndexForField(m.addInvoiceForm.Active); inputIndex >= 0 {
		var cmd tea.Cmd

		m.addInvoiceForm.Inputs[inputIndex], cmd = m.addInvoiceForm.Inputs[inputIndex].Update(msg)

		return m, cmd
	}

	return m, nil
}

func (m TheApplication) updateInvoiceList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if updatedModel, cmd, handled := m.handleDeleteConfirmation(msg); handled {
		return updatedModel, cmd
	}

	filtered := m.filteredInvoices()
	pageSize := m.invoiceListPageSize()
	totalPages := 1
	if pageSize > 0 && len(filtered) > 0 {
		totalPages = (len(filtered) + pageSize - 1) / pageSize
	}
	m.invoicePage = clamp(m.invoicePage, 0, totalPages-1)

	pageStart := m.invoicePage * pageSize
	if pageStart < 0 {
		pageStart = 0
	}
	if pageStart > len(filtered) {
		pageStart = len(filtered)
	}
	pageEnd := pageStart + pageSize
	if pageEnd > len(filtered) {
		pageEnd = len(filtered)
	}

	if len(filtered) > 0 {
		if m.invoiceCursor < pageStart {
			m.invoiceCursor = pageStart
		}
		if m.invoiceCursor >= pageEnd {
			m.invoiceCursor = pageEnd - 1
		}
	}

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
		if m.invoiceCursor > pageStart {
			m.invoiceCursor--
		}

		return m, nil
	case "down":
		if m.invoiceCursor < pageEnd-1 {
			m.invoiceCursor++
		}

		return m, nil
	case "left", "h", "pgup":
		if m.invoicePage > 0 {
			m.invoicePage--
			pageStart = m.invoicePage * pageSize
			if pageStart >= len(filtered) {
				pageStart = len(filtered) - 1
			}
			if pageStart < 0 {
				pageStart = 0
			}
			m.invoiceCursor = pageStart
		}

		return m, nil
	case "right", "l", "pgdown":
		if m.invoicePage < totalPages-1 {
			m.invoicePage++
			pageStart = m.invoicePage * pageSize
			if pageStart >= len(filtered) {
				pageStart = len(filtered) - 1
			}
			if pageStart < 0 {
				pageStart = 0
			}
			m.invoiceCursor = pageStart
		}

		return m, nil
	case "home":
		m.invoicePage = 0
		m.invoiceCursor = 0

		return m, nil
	case "end":
		m.invoicePage = totalPages - 1
		m.invoiceCursor = m.invoicePage * pageSize
		if m.invoiceCursor >= len(filtered) {
			m.invoiceCursor = len(filtered) - 1
		}

		return m, nil
	case "enter":
		m = m.openInvoiceEditor(filtered[m.invoiceCursor]).(TheApplication)

		return m, nil
	case "backspace", "delete":
		selected := filtered[m.invoiceCursor]
		m = m.beginDeleteConfirmation("invoice", selected.ID, selected.Title)

		return m, nil
	default:
		return m, nil
	}
}

func (m TheApplication) updateInvoiceEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenInvoiceList
		m.status = "invoice edit cancelled"

		return m, nil
	case "up", "shift+tab":
		m.editInvoiceForm = m.editInvoiceForm.Prev()

		return m, nil
	case "down", "tab":
		m.editInvoiceForm = m.editInvoiceForm.Next()

		return m, nil
	case "left":
		if m.editInvoiceForm.ActiveField == invoice.InvoiceFieldType {
			m.editInvoiceForm.IsIncoming = true
		}

		if m.editInvoiceForm.ActiveField == invoice.InvoiceFieldCurrency && m.editInvoiceForm.CurrencyIndex > 0 {
			m.editInvoiceForm.CurrencyIndex--
		}

		if m.editInvoiceForm.ActiveField == invoice.InvoiceFieldTargetAccountChoice && m.editInvoiceForm.AccountIndex > 0 {
			m.editInvoiceForm.AccountIndex--
		}

		return m, nil
	case "right":
		if m.editInvoiceForm.ActiveField == invoice.InvoiceFieldType {
			m.editInvoiceForm.IsIncoming = false
		}

		if m.editInvoiceForm.ActiveField == invoice.InvoiceFieldCurrency && m.editInvoiceForm.CurrencyIndex < len(m.editInvoiceForm.CurrencyOptions)-1 {
			m.editInvoiceForm.CurrencyIndex++
		}

		if m.editInvoiceForm.ActiveField == invoice.InvoiceFieldTargetAccountChoice && m.editInvoiceForm.AccountIndex < len(m.editInvoiceForm.AccountOptions)-1 {
			m.editInvoiceForm.AccountIndex++
		}

		return m, nil
	case " ":
		if m.editInvoiceForm.ActiveField == invoice.InvoiceFieldPaid {
			m.editInvoiceForm.Paid = !m.editInvoiceForm.Paid

			return m, nil
		}
	case "enter":
		if m.editInvoiceForm.ActiveField == invoice.InvoiceFieldCount-1 {
			return m.saveInvoiceEdit()
		}

		m.editInvoiceForm = m.editInvoiceForm.Next()

		return m, nil
	}

	var cmd tea.Cmd
	switch m.editInvoiceForm.ActiveField {
	case invoice.InvoiceFieldTitle:
		m.editInvoiceForm.TitleInput, cmd = m.editInvoiceForm.TitleInput.Update(msg)
	case invoice.InvoiceFieldAmount:
		m.editInvoiceForm.AmountInput, cmd = m.editInvoiceForm.AmountInput.Update(msg)
	case invoice.InvoiceFieldPeer:
		m.editInvoiceForm.PeerInput, cmd = m.editInvoiceForm.PeerInput.Update(msg)
	case invoice.InvoiceFieldInvoiceDate:
		m.editInvoiceForm.InvoiceDateInput, cmd = m.editInvoiceForm.InvoiceDateInput.Update(msg)
	case invoice.InvoiceFieldDueDate:
		m.editInvoiceForm.DueDateInput, cmd = m.editInvoiceForm.DueDateInput.Update(msg)
	case invoice.InvoiceFieldTargetAccountName:
		m.editInvoiceForm.TargetAccountInput, cmd = m.editInvoiceForm.TargetAccountInput.Update(msg)
	case invoice.InvoiceFieldURL:
		m.editInvoiceForm.URLInput, cmd = m.editInvoiceForm.URLInput.Update(msg)
	case invoice.InvoiceFieldDescription:
		m.editInvoiceForm.DescriptionInput, cmd = m.editInvoiceForm.DescriptionInput.Update(msg)
	}

	return m, cmd
}
