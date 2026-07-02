package app

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/settings"
)

func (m TheApplication) loadAccountValueLogs(accountID uint) []account.AccountValueLog {
	if accountID == 0 {
		return nil
	}

	logs, err := m.storage.LoadAccountValueLogs(accountID)
	if err != nil {
		return nil
	}

	return logs
}

func (m TheApplication) upsertAccountValueLog(accountID uint, day time.Time, valueCents int64) error {
	return m.storage.UpsertAccountValueLog(accountID, day, valueCents)
}

func (m TheApplication) renderMoneyConditionalRed(base string, cents int64) string {
	formatted := renderMoneyWithCurrency(base, cents)
	if cents > 0 {
		return obligationStyle.Render(formatted)
	}

	return formatted
}

func (m TheApplication) renderAddAccount(width int) string {
	lines := []string{
		headlineStyle.Render("Add account"),
		mutedStyle.Render("Fill the fields, choose currency with left/right, and set whether this account should be ignored in summaries."),
		"",
	}

	for i := range m.addForm.Fields {
		if i == 2 {
			lines = append(lines, m.renderAccountCurrencyFieldRow(width))

			continue
		}

		label := fieldLabelStyle.Render(m.addForm.Labels[i])
		value := inputBoxStyle.Width(width - 18).Render(m.addForm.Fields[i].View())
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, label, "  ", value))
	}

	ignorePrefix := "  "
	if m.addForm.Active == 4 {
		ignorePrefix = "> "
	}
	ignoreMarker := "[ ]"
	if m.addForm.IgnoreInSummaries {
		ignoreMarker = "[x]"
	}
	lines = append(lines, ignorePrefix+fieldLabelStyle.Render("Ignore in summaries")+"  "+ignoreMarker)
	lines = append(lines, mutedStyle.Render("Use this for meta-accounts and historical records; ignored accounts stay out of totals."))

	lines = append(lines, "")
	lines = append(lines, mutedStyle.Render("Tab moves forward, Shift+Tab moves back, Space toggles the checkbox, Esc returns home."))

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderAccountCurrencyFieldRow(width int) string {
	options := make([]string, 0, len(m.addForm.CurrencyOptions))
	for index, option := range m.addForm.CurrencyOptions {
		style := buttonStyle
		if index == m.addForm.CurrencyIndex {
			style = buttonActiveStyle
		}

		options = append(options, style.Render(option))
	}

	prefix := " "
	if m.addForm.Active == 2 {
		prefix = ">"
	}

	label := fieldLabelStyle.Render(prefix + " Currency")
	value := inputBoxStyle.Width(width - 18).Render(strings.Join(options, " "))

	return lipgloss.JoinHorizontal(lipgloss.Top, label, "  ", value)
}

func (m TheApplication) renderAccountTable(width int) string {
	lines := []string{sectionTitleStyle.Render("Accounts")}
	lines = append(lines, hintStyle.Render("Use up/down to move, Enter to edit amount, S changes sorting, Delete/Backspace asks confirmation, Esc to go back."))
	lines = append(lines, "")

	if len(m.accounts) == 0 {
		lines = append(lines, mutedStyle.Render("No accounts yet."))
		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	accountBaseTotal := m.sumAccountsInBaseCents()
	baseLabel := m.baseCurrencyLabel()
	lines = append(lines, fieldLabelStyle.Render("Total in "+baseLabel+":"), "  "+renderMoneyWithCurrency(baseLabel, accountBaseTotal), "")
	ignoredCount := 0
	for _, acct := range m.accounts {
		if acct.IgnoreInSummaries {
			ignoredCount++
		}
	}
	if ignoredCount > 0 {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("%d account(s) ignored in summaries.", ignoredCount)), "")
	}
	lines = append(lines, fieldLabelStyle.Render("Sort")+"  "+accountSortLabel(m.accountSortField))
	if m.accountSortMenu {
		lines = append(lines, m.renderAccountSortMenu(width))
	}
	lines = append(lines, "")

	lines = append(lines, m.renderAccountTableHeader(width))

	for i, acct := range m.accounts {
		lines = append(lines, m.renderAccountRow(width, i, acct))
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderAccountTableHeader(width int) string {
	nameWidth := 16
	currencyWidth := 10
	amountWidth := 14
	baseAmountWidth := 14
	updatedWidth := 14

	descWidth := width - 14 - nameWidth - currencyWidth - amountWidth - baseAmountWidth - updatedWidth - 12
	if descWidth < 20 {
		descWidth = 20
	}

	baseCurrencyTitle := strings.TrimSpace(m.settings.BaseCurrency)
	if baseCurrencyTitle == "" {
		baseCurrencyTitle = "$"
	}

	header := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s", "#", nameWidth, "Name", currencyWidth, "Currency", amountWidth, "Amount", baseAmountWidth, truncateText(baseCurrencyTitle, baseAmountWidth), updatedWidth, "Updated", descWidth, "Description")

	return tableHeaderStyle.Render(header)
}

func (m TheApplication) renderAccountRow(width int, index int, acct account.Account) string {
	nameWidth := 16
	currencyWidth := 10
	amountWidth := 14
	baseAmountWidth := 14
	updatedWidth := 14
	descWidth := width - 14 - nameWidth - currencyWidth - amountWidth - baseAmountWidth - updatedWidth - 12
	if descWidth < 20 {
		descWidth = 20
	}

	prefix := " "
	style := rowStyle
	if index == m.cursor {
		prefix = ">"
		style = selectedRowStyle
	}

	row := fmt.Sprintf("%s %-*s %-*s %-*s %-*s %-*s %-*s", prefix, nameWidth, truncateText(acct.Name, nameWidth), currencyWidth, truncateText(acct.Currency, currencyWidth), amountWidth, renderMoneyWithCurrency(acct.Currency, acct.BalanceCents), baseAmountWidth, m.convertedAmountForBase(acct.Currency, acct.BalanceCents), updatedWidth, formatUpdatedAt(acct.LastUpdatedAt), descWidth, truncateText(acct.Description, descWidth))

	return style.Render(row)
}

func (m TheApplication) renderEditAmount(width int) string {
	if m.cursor < 0 || m.cursor >= len(m.accounts) {
		return panelStyle.Width(width).Render(mutedStyle.Render("No account selected."))
	}

	acct := m.accounts[m.cursor]

	currentPrefix := "  "
	if m.editAmountActiveField == editAmountFieldCurrent {
		currentPrefix = "> "
	}

	updateLogPrefix := "  "
	if m.editAmountActiveField == editAmountFieldUpdateLog {
		updateLogPrefix = "> "
	}

	ignorePrefix := "  "
	if m.editAmountActiveField == editAmountFieldIgnore {
		ignorePrefix = "> "
	}

	logDatePrefix := "  "
	if m.editAmountActiveField == editAmountFieldLogDate {
		logDatePrefix = "> "
	}

	logValuePrefix := "  "
	if m.editAmountActiveField == editAmountFieldLogValue {
		logValuePrefix = "> "
	}

	updateLogMarker := "[ ]"
	if m.editAmountUpdateLog {
		updateLogMarker = "[x]"
	}

	ignoreMarker := "[ ]"
	if m.editAmountIgnoreInSummaries {
		ignoreMarker = "[x]"
	}

	lines := []string{
		headlineStyle.Render("Edit account amount"),
		mutedStyle.Render("Account: " + acct.Name + " | " + acct.Currency + " | amount " + renderMoneyWithCurrency(acct.Currency, acct.BalanceCents)),
		"",
		currentPrefix + fieldLabelStyle.Render("Amount"),
		inputBoxStyle.Width(24).Render(m.editInput.View()),
		updateLogPrefix + fieldLabelStyle.Render("Update log on save") + "  " + updateLogMarker,
		ignorePrefix + fieldLabelStyle.Render("Ignore in summaries") + "  " + ignoreMarker,
		"",
		fieldLabelStyle.Render("Add/Update historical value"),
		logDatePrefix + fieldLabelStyle.Render("Date") + "  " + m.editAmountLogDateInput.View(),
		logValuePrefix + fieldLabelStyle.Render("Value") + "  " + m.editAmountLogValueInput.View(),
		"",
		mutedStyle.Render("Enter on Amount saves account amount. Enter on Value saves log for Date."),
		mutedStyle.Render("Use up/down or tab/shift+tab to move fields. Space toggles checkboxes. Esc cancels."),
	}

	lines = append(lines, "", fieldLabelStyle.Render("Value history"))
	if len(m.accountValueLogs) == 0 {
		lines = append(lines, mutedStyle.Render("No historical values yet."))
	} else {
		dateWidth := 12
		valueWidth := 14
		header := fmt.Sprintf("%-*s %-*s", dateWidth, "Date", valueWidth, "Value")
		lines = append(lines, tableHeaderStyle.Render(header))

		for _, entry := range m.accountValueLogs {
			row := fmt.Sprintf("%-*s %-*s", dateWidth, entry.LogDate.Local().Format("2006-01-02"), valueWidth, renderMoneyWithCurrency(acct.Currency, entry.ValueCents))
			lines = append(lines, rowStyle.Render(row))
		}
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderAccountSortMenu(width int) string {
	lines := []string{
		fieldLabelStyle.Render("Sorting options"),
		mutedStyle.Render("Use up/down to choose. Enter applies. Esc closes."),
	}

	for index, option := range accountSortOptions() {
		prefix := "  "
		if index == m.accountSortCursor {
			prefix = "> "
		}

		label := option
		if index == int(m.accountSortField) {
			label += " (current)"
		}

		lines = append(lines, prefix+label)
	}

	menuWidth := width - 4
	if menuWidth < 32 {
		menuWidth = 32
	}

	return inputBoxStyle.Width(menuWidth).Render(strings.Join(lines, "\n"))
}

func accountSortOptions() []string {
	return []string{
		"Balance in base currency",
		"Name",
		"Currency",
		"Last updated",
	}
}

func accountSortLabel(field accountSortField) string {
	options := accountSortOptions()
	index := int(field)
	if index < 0 || index >= len(options) {
		return options[0]
	}

	return options[index]
}

func comparableAccountBaseCents(acct account.Account, settings settings.AppSettings) int64 {
	base := strings.TrimSpace(settings.BaseCurrency)
	if base == "" {
		base = "$"
	}

	if strings.EqualFold(strings.TrimSpace(acct.Currency), base) {
		return acct.BalanceCents
	}

	for _, entry := range settings.Currencies {
		if strings.EqualFold(strings.TrimSpace(entry.CurrencyName), strings.TrimSpace(acct.Currency)) && entry.RateToBase > 0 {
			return int64(math.Round(float64(acct.BalanceCents) * entry.RateToBase))
		}
	}

	return acct.BalanceCents
}

func findAccountIndex(accounts []account.Account, id uint) int {
	for i := range accounts {
		if accounts[i].ID == id {
			return i
		}
	}

	return -1
}

func accountLogDay(value time.Time) time.Time {
	local := value.Local()

	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
}

func sortAccounts(accounts []account.Account, settings settings.AppSettings, field accountSortField) []account.Account {
	if len(accounts) < 2 {
		return accounts
	}

	cloned := make([]account.Account, len(accounts))
	copy(cloned, accounts)
	sort.SliceStable(cloned, func(i, j int) bool {
		left := cloned[i]
		right := cloned[j]

		switch field {
		case accountSortName:
			leftName := strings.ToLower(strings.TrimSpace(left.Name))
			rightName := strings.ToLower(strings.TrimSpace(right.Name))
			if leftName != rightName {
				return leftName < rightName
			}
		case accountSortCurrency:
			leftCurrency := strings.ToLower(strings.TrimSpace(left.Currency))
			rightCurrency := strings.ToLower(strings.TrimSpace(right.Currency))
			if leftCurrency != rightCurrency {
				return leftCurrency < rightCurrency
			}
		case accountSortUpdated:
			if !left.LastUpdatedAt.Equal(right.LastUpdatedAt) {
				return left.LastUpdatedAt.After(right.LastUpdatedAt)
			}
		default:
			leftComparable := comparableAccountBaseCents(left, settings)
			rightComparable := comparableAccountBaseCents(right, settings)
			if leftComparable != rightComparable {
				return leftComparable > rightComparable
			}
		}

		if left.CreatedAt.Equal(right.CreatedAt) {
			return left.ID > right.ID
		}

		return left.CreatedAt.After(right.CreatedAt)
	})

	return cloned
}

func (m TheApplication) updateAddAccount(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

		return m, nil
	case "up", "shift+tab":
		m.addForm = m.addForm.Prev()

		return m, nil
	case "down", "tab":
		m.addForm = m.addForm.Next()

		return m, nil
	case "left":
		if m.addForm.Active == 2 && m.addForm.CurrencyIndex > 0 {
			m.addForm.CurrencyIndex--
		}
		if m.addForm.Active == 4 {
			m.addForm.IgnoreInSummaries = false
		}

		return m, nil
	case "right":
		if m.addForm.Active == 2 && m.addForm.CurrencyIndex < len(m.addForm.CurrencyOptions)-1 {
			m.addForm.CurrencyIndex++
		}
		if m.addForm.Active == 4 {
			m.addForm.IgnoreInSummaries = true
		}

		return m, nil
	case " ":
		if m.addForm.Active == 4 {
			m.addForm.IgnoreInSummaries = !m.addForm.IgnoreInSummaries

			return m, nil
		}
	case "enter":
		if m.addForm.Active == len(m.addForm.Fields) {
			return m.saveAccountFromForm()
		}

		m.addForm = m.addForm.Next()

		return m, nil
	}

	if m.addForm.Active == 2 {
		return m, nil
	}

	if m.addForm.Active == 4 {
		return m, nil
	}

	var cmd tea.Cmd

	m.addForm.Fields[m.addForm.Active], cmd = m.addForm.Fields[m.addForm.Active].Update(msg)

	return m, cmd
}

func (m TheApplication) updateAccountTable(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if updatedModel, cmd, handled := m.handleDeleteConfirmation(msg); handled {
		return updatedModel, cmd
	}

	if len(m.accounts) == 0 {
		m.screen = screenMenu
		m.status = "no accounts available"

		return m, nil
	}

	if m.accountSortMenu {
		switch strings.ToLower(msg.String()) {
		case "esc", "s":
			m.accountSortMenu = false
			m.status = "account sort unchanged"

			return m, nil
		case "up", "k":
			if m.accountSortCursor > 0 {
				m.accountSortCursor--
			}

			return m, nil
		case "down", "j":
			if m.accountSortCursor < len(accountSortOptions())-1 {
				m.accountSortCursor++
			}

			return m, nil
		case "enter":
			selectedID := uint(0)
			if m.cursor >= 0 && m.cursor < len(m.accounts) {
				selectedID = m.accounts[m.cursor].ID
			}

			m.accountSortField = accountSortField(m.accountSortCursor)
			m.accounts = sortAccounts(m.accounts, m.settings, m.accountSortField)
			m.accountSortMenu = false
			if selectedID != 0 {
				m.cursor = findAccountIndex(m.accounts, selectedID)
			}
			if m.cursor < 0 {
				m.cursor = 0
			}
			m.status = "sorting accounts by " + accountSortLabel(m.accountSortField)

			return m, nil
		default:
			return m, nil
		}
	}

	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

		return m, nil
	case "up":
		if m.cursor > 0 {
			m.cursor--
		}

		return m, nil
	case "down":
		if m.cursor < len(m.accounts)-1 {
			m.cursor++
		}

		return m, nil
	case "s", "S":
		m.accountSortMenu = true
		m.accountSortCursor = int(m.accountSortField)
		m.status = "choose account sorting"

		return m, nil
	case "enter":
		return m.beginEditAmount(), nil
	case "backspace", "delete":
		selected := m.accounts[m.cursor]
		m = m.beginDeleteConfirmation("account", selected.ID, selected.Name)

		return m, nil
	default:
		return m, nil
	}
}

func (m TheApplication) updateEditAmount(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenAccountTable
		m.status = "amount edit cancelled"

		return m, nil
	case "up", "shift+tab":
		if m.editAmountActiveField > 0 {
			m.editAmountActiveField--
		}

		return m.focusEditAmountField(), nil
	case "down", "tab":
		if m.editAmountActiveField < editAmountFieldCount-1 {
			m.editAmountActiveField++
		}

		return m.focusEditAmountField(), nil
	case " ":
		if m.editAmountActiveField == editAmountFieldUpdateLog {
			m.editAmountUpdateLog = !m.editAmountUpdateLog

			return m, nil
		}
		if m.editAmountActiveField == editAmountFieldIgnore {
			m.editAmountIgnoreInSummaries = !m.editAmountIgnoreInSummaries

			return m, nil
		}
	case "enter":
		switch m.editAmountActiveField {
		case editAmountFieldCurrent:
			return m.saveAmount()
		case editAmountFieldIgnore:
			m.editAmountIgnoreInSummaries = !m.editAmountIgnoreInSummaries

			return m, nil
		case editAmountFieldLogValue:
			return m.applyAccountLogValue()
		default:
			return m, nil
		}
	}

	var cmd tea.Cmd

	switch m.editAmountActiveField {
	case editAmountFieldCurrent:
		m.editInput, cmd = m.editInput.Update(msg)
	case editAmountFieldLogDate:
		m.editAmountLogDateInput, cmd = m.editAmountLogDateInput.Update(msg)
	case editAmountFieldLogValue:
		m.editAmountLogValueInput, cmd = m.editAmountLogValueInput.Update(msg)
	default:
		return m, nil
	}

	return m, cmd
}

func (m TheApplication) focusEditAmountField() TheApplication {
	m.editInput.Blur()
	m.editAmountLogDateInput.Blur()
	m.editAmountLogValueInput.Blur()

	switch m.editAmountActiveField {
	case editAmountFieldCurrent:
		m.editInput.Focus()
	case editAmountFieldLogDate:
		m.editAmountLogDateInput.Focus()
	case editAmountFieldLogValue:
		m.editAmountLogValueInput.Focus()
	}

	return m
}

func (m TheApplication) sumAccountsInBaseCents() int64 {
	var total int64

	for _, acct := range m.accounts {
		if acct.IgnoreInSummaries {
			continue
		}

		converted, ok := m.convertToBaseCents(acct.Currency, acct.BalanceCents)
		if !ok {
			continue
		}

		total += converted
	}

	return total
}

func (m TheApplication) confirmDeleteAccount() (tea.Model, tea.Cmd) {
	index := findAccountIndex(m.accounts, m.deleteConfirmID)
	if index < 0 {
		m = m.clearDeleteConfirmation("account not found")

		return m, nil
	}

	selected := m.accounts[index]
	if err := m.storage.DeleteAccount(selected.ID); err != nil {
		m = m.clearDeleteConfirmation("delete failed: " + err.Error())

		return m, nil
	}

	m.accounts = append(m.accounts[:index], m.accounts[index+1:]...)
	if m.cursor >= len(m.accounts) && m.cursor > 0 {
		m.cursor--
	}

	if len(m.accounts) == 0 {
		m.screen = screenMenu
		m.cursor = 0
	}

	m = m.clearDeleteConfirmation("deleted account " + selected.Name)

	return m, nil
}

func (m TheApplication) applyAccountLogValue() (tea.Model, tea.Cmd) {
	if m.cursor < 0 || m.cursor >= len(m.accounts) {
		m.status = "no account selected"
		return m, nil
	}

	rawDay := strings.TrimSpace(m.editAmountLogDateInput.Value())
	if rawDay == "" {
		m.status = "log date is required"

		return m, nil
	}

	day, err := time.ParseInLocation("02.01.2006", rawDay, time.Now().Location())
	if err != nil {
		m.status = "log date must use DD.MM.YYYY format"

		return m, nil
	}

	value, err := parseAmountCents(strings.TrimSpace(m.editAmountLogValueInput.Value()))
	if err != nil {
		m.status = "log value error: " + err.Error()

		return m, nil
	}

	selected := m.accounts[m.cursor]
	if err := m.upsertAccountValueLog(selected.ID, day, value); err != nil {
		m.status = "log save failed: " + err.Error()

		return m, nil
	}

	m.accountValueLogs = m.loadAccountValueLogs(selected.ID)
	m.status = "saved log value for " + accountLogDay(day).Format("2006-01-02")

	return m, nil
}

func (m TheApplication) saveAccountFromForm() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.addForm.Fields[0].Value())
	description := strings.TrimSpace(m.addForm.Fields[1].Value())
	currency := selectedCurrencyOption(m.addForm.CurrencyOptions, m.addForm.CurrencyIndex)
	amountRaw := strings.TrimSpace(m.addForm.Fields[3].Value())

	if name == "" {
		m.status = "name is required"
		return m, nil
	}
	if description == "" {
		m.status = "description is required"
		return m, nil
	}
	if currency == "" {
		m.status = "currency is required"
		return m, nil
	}

	amount, err := parseAmountCents(amountRaw)
	if err != nil {
		m.status = "amount error: " + err.Error()
		return m, nil
	}

	now := time.Now()

	newAccount := account.Account{
		Name:              name,
		Description:       description,
		Currency:          currency,
		BalanceCents:      amount,
		LeftoverCents:     amount,
		IgnoreInSummaries: m.addForm.IgnoreInSummaries,
		LastUpdatedAt:     now,
	}

	if err := m.storage.CreateAccount(&newAccount); err != nil {
		m.status = "save failed: " + err.Error()
		return m, nil
	}

	m.accounts = append([]account.Account{newAccount}, m.accounts...)
	m.accounts = sortAccounts(m.accounts, m.settings, m.accountSortField)
	m.addForm = account.NewAddAccountForm(currencySelectionOptions(m.settings))
	m.screen = screenMenu
	m.status = "saved account " + name
	return m, nil
}

func (m TheApplication) beginEditAmount() tea.Model {
	if m.cursor < 0 || m.cursor >= len(m.accounts) {
		m.status = "no account selected"
		return m
	}

	current := m.accounts[m.cursor]
	m.editInput = textinput.New()
	m.editInput.Placeholder = "1234.56"
	m.editInput.CharLimit = 24
	m.editInput.Width = 20
	m.editInput.SetValue(formatAmount(current.BalanceCents))

	m.editAmountLogDateInput = textinput.New()
	m.editAmountLogDateInput.Placeholder = "DD.MM.YYYY"
	m.editAmountLogDateInput.CharLimit = 24
	m.editAmountLogDateInput.Width = 20
	m.editAmountLogDateInput.SetValue(time.Now().Format("02.01.2006"))

	m.editAmountLogValueInput = textinput.New()
	m.editAmountLogValueInput.Placeholder = "1234.56"
	m.editAmountLogValueInput.CharLimit = 24
	m.editAmountLogValueInput.Width = 20
	m.editAmountLogValueInput.SetValue(formatAmount(current.BalanceCents))

	m.editAmountActiveField = editAmountFieldCurrent
	m.editAmountUpdateLog = false
	m.editAmountIgnoreInSummaries = current.IgnoreInSummaries
	m.editInput.Focus()
	m.editAmountLogDateInput.Blur()
	m.editAmountLogValueInput.Blur()
	m.accountValueLogs = m.loadAccountValueLogs(current.ID)
	m.screen = screenEditAmount
	m.status = "editing account " + current.Name

	return m
}

func (m TheApplication) saveAmount() (tea.Model, tea.Cmd) {
	if m.cursor < 0 || m.cursor >= len(m.accounts) {
		m.status = "no account selected"

		return m, nil
	}

	amount, err := parseAmountCents(strings.TrimSpace(m.editInput.Value()))
	if err != nil {
		m.status = "amount error: " + err.Error()

		return m, nil
	}

	selected := m.accounts[m.cursor]
	now := time.Now()
	if err := m.storage.UpdateAccountAmount(selected.ID, amount, m.editAmountIgnoreInSummaries, now); err != nil {
		m.status = "update failed: " + err.Error()

		return m, nil
	}

	logWarn := ""

	if m.editAmountUpdateLog {
		if err := m.upsertAccountValueLog(selected.ID, now, amount); err != nil {
			logWarn = " (log update failed: " + err.Error() + ")"
		}
	}

	selected.BalanceCents = amount
	selected.LeftoverCents = amount
	selected.LastUpdatedAt = now
	selected.IgnoreInSummaries = m.editAmountIgnoreInSummaries
	m.accounts[m.cursor] = selected
	m.accounts = sortAccounts(m.accounts, m.settings, m.accountSortField)

	m.cursor = findAccountIndex(m.accounts, selected.ID)
	if m.cursor < 0 {
		m.cursor = 0
	}

	m.screen = screenAccountTable
	m.status = "updated amount for " + selected.Name + logWarn

	return m, nil
}
