package app

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lazybark/cents/flows/cashflow"
)

func (m TheApplication) renderCashflowOverviewRow(width int, row cashflow.CashflowMonthlyOverviewRow, base string) string {
	monthWidth := 14
	incomeWidth := 14
	expenseWidth := 14
	netWidth := 14
	compareWidth := max(width-14-monthWidth-incomeWidth-expenseWidth-netWidth-12, 16)

	compareText := "-"

	if row.HasPrev {
		compareText = renderMoneyWithCurrency(base, row.DeltaFromPrev)
		if row.DeltaFromPrev > 0 {
			compareText = "+" + compareText
			compareText = positiveStyle.Render(compareText)
		} else if row.DeltaFromPrev < 0 {
			compareText = obligationStyle.Render(compareText)
		}
	}

	netText := renderSignedMoneyWithCurrency(base, row.NetBase)
	monthCell := lipgloss.NewStyle().Width(monthWidth).Align(lipgloss.Left).Render(row.Month.Format("2006-01"))
	incomeCell := lipgloss.NewStyle().Width(incomeWidth).Align(lipgloss.Left).Render(renderMoneyWithCurrency(base, row.IncomeBase))
	expenseCell := lipgloss.NewStyle().Width(expenseWidth).Align(lipgloss.Left).Render(renderMoneyWithCurrency(base, row.ExpenseBase))
	netCell := lipgloss.NewStyle().Width(netWidth).Align(lipgloss.Left).Render(netText)
	compareCell := lipgloss.NewStyle().Width(compareWidth).Align(lipgloss.Left).Render(compareText)
	line := strings.Join([]string{monthCell, incomeCell, expenseCell, netCell, compareCell}, " ")

	return rowStyle.Render(line)
}

func (m TheApplication) renderCashflowHistoryHeader(width int) string {
	dateWidth := 10
	typeWidth := 8
	currencyWidth := 8
	amountWidth := 12
	categoryWidth := 32
	accountWidth := 28

	commentWidth := width - 14 - dateWidth - typeWidth - currencyWidth - amountWidth - categoryWidth - accountWidth - 18
	if commentWidth < 12 {
		commentWidth = 12
	}

	header := fmt.Sprintf("%-2s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", "#", dateWidth, "Date", typeWidth, "Type", currencyWidth, "Curr", amountWidth, "Amount", categoryWidth, "Category", accountWidth, "Account", commentWidth, "Comment")

	return tableHeaderStyle.Render(header)
}

func (m TheApplication) renderCashflowHistoryRow(width int, index int, item cashflow.CashflowEntry) string {
	dateWidth := 10
	typeWidth := 8
	currencyWidth := 8
	amountWidth := 12
	categoryWidth := 32
	accountWidth := 28

	commentWidth := width - 14 - dateWidth - typeWidth - currencyWidth - amountWidth - categoryWidth - accountWidth - 18
	if commentWidth < 12 {
		commentWidth = 12
	}

	prefix := " "
	style := rowStyle

	if index == m.cashflowCursor {
		prefix = ">"
		style = selectedRowStyle
	}

	typeLabel := "expense"

	if item.IsIncome {
		typeLabel = "income"
	}

	accountLabel := strings.TrimSpace(item.AccountName)
	if accountLabel == "" {
		accountLabel = "-"
	}

	row := fmt.Sprintf("%s %-*s %-*s %-*s %-*s %-*s %-*s %-*s", prefix, dateWidth, item.EntryDate.Local().Format("2006-01-02"), typeWidth, typeLabel, currencyWidth, truncateText(item.Currency, currencyWidth), amountWidth, renderMoneyWithCurrency(item.Currency, item.AmountCents), categoryWidth, truncateText(item.Category, categoryWidth), accountWidth, truncateText(accountLabel, accountWidth), commentWidth, truncateText(item.Comment, commentWidth))

	return style.Render(row)
}

func (m TheApplication) renderCashflowNew(width int) string {
	title := "New expense"
	if m.addCashflowForm.IsIncome {
		title = "New income"
	}

	lines := []string{
		headlineStyle.Render(title),
		mutedStyle.Render("Currency, amount, date, category, optional account and comment. Date defaults to today."),
		"",
	}

	if len(m.addCashflowForm.CategoryOptions) == 0 {
		if m.addCashflowForm.IsIncome {
			lines = append(lines, mutedStyle.Render("No income categories configured. Add one in Settings first."))
		} else {
			lines = append(lines, mutedStyle.Render("No expense categories configured. Add one in Settings first."))
		}

		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	lines = append(lines,
		m.renderCashflowChoiceRow(cashflow.CashflowFieldCurrency, "Currency", m.addCashflowForm.CurrencyOptions, m.addCashflowForm.CurrencyIndex),
		m.renderCashflowTextRow(cashflow.CashflowFieldAmount, "Amount", m.addCashflowForm.Inputs[0].View()),
		m.renderCashflowTextRow(cashflow.CashflowFieldDate, "Date", m.addCashflowForm.Inputs[1].View()),
		m.renderCashflowChoiceRow(cashflow.CashflowFieldCategory, "Category", m.addCashflowForm.CategoryOptions, m.addCashflowForm.CategoryIndex),
		m.renderCashflowChoiceRow(cashflow.CashflowFieldAccount, "Account (optional)", cashflowAccountDisplayOptions(m.addCashflowForm.AccountOptions), m.addCashflowForm.AccountIndex),
		m.renderCashflowTextRow(cashflow.CashflowFieldComment, "Comment", m.addCashflowForm.Inputs[2].View()),
	)

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderCashflowTextRow(field int, label string, value string) string {
	prefix := "  "
	if m.addCashflowForm.Active == field {
		prefix = "> "
	}

	return prefix + fieldLabelStyle.Render(label) + "  " + value
}

func (m TheApplication) renderCashflowChoiceRow(field int, label string, options []string, selected int) string {
	prefix := "  "
	if m.addCashflowForm.Active == field {
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

func (m TheApplication) renderCashflowHistory(width int) string {
	monthLabel := m.cashflowHistoryMonth.Format("January 2006")
	lines := []string{
		lipgloss.JoinHorizontal(lipgloss.Center, sectionTitleStyle.Render("Income/Expense history"), "  ", modeBadgeStyle.Render("Monthly")),
		hintStyle.Render("Left/right changes month. Up/down scrolls rows. Delete/Backspace asks confirmation. Esc returns to menu."),
		"",
		fieldLabelStyle.Render("Month") + "  " + buttonStyle.Render("<") + " " + buttonActiveStyle.Render(monthLabel) + " " + buttonStyle.Render(">"),
		"",
	}

	items := m.filteredCashflowsForMonth()
	if len(items) == 0 {
		lines = append(lines, mutedStyle.Render("No records for this month."))

		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	var incomeTotal int64
	var expenseTotal int64

	missingRates := 0
	for _, item := range items {
		amountBase, ok := m.convertToBaseCents(item.Currency, item.AmountCents)
		if !ok {
			missingRates++

			continue
		}

		if item.IsIncome {
			incomeTotal += amountBase
		} else {
			expenseTotal += amountBase
		}
	}

	net := incomeTotal - expenseTotal
	base := m.baseCurrencyLabel()
	netLabel := renderSignedMoneyWithCurrency(base, net)
	lines = append(lines, fmt.Sprintf("Income: %s  Expense: %s  Net: %s", renderMoneyWithCurrency(base, incomeTotal), renderMoneyWithCurrency(base, expenseTotal), netLabel))

	if missingRates > 0 {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("%d record(s) excluded from summary due to missing conversion rate.", missingRates)))
	}

	lines = append(lines, "")
	lines = append(lines, m.renderCashflowHistoryHeader(width))

	for i, item := range items {
		lines = append(lines, m.renderCashflowHistoryRow(width, i, item))
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderCashflowOverview(width int) string {
	base := m.baseCurrencyLabel()
	lines := []string{
		lipgloss.JoinHorizontal(lipgloss.Center, sectionTitleStyle.Render("Income/Expense overview"), "  ", modeBadgeStyle.Render("Monthly")),
		hintStyle.Render("Month-by-month totals in base currency (newest first). Up/down or left/right changes page. Esc returns to menu."),
		"",
	}

	rows, missingRates := m.monthlyCashflowOverviewRows()
	if len(rows) == 0 {
		lines = append(lines, mutedStyle.Render("No records available for overview."))
		if missingRates > 0 {
			lines = append(lines, mutedStyle.Render(fmt.Sprintf("%d record(s) skipped due to missing conversion rate.", missingRates)))
		}

		return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
	}

	pageSize := m.cashflowOverviewPageSize()

	totalPages := (len(rows) + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}

	currentPage := clamp(m.cashflowOverviewPage, 0, totalPages-1)
	startFromEnd := len(rows) - currentPage*pageSize

	endFromEnd := startFromEnd - pageSize
	if endFromEnd < 0 {
		endFromEnd = 0
	}

	lines = append(lines, fmt.Sprintf("Page %d/%d", currentPage+1, totalPages), "")
	lines = append(lines, m.renderCashflowOverviewHeader(width, base))

	for i := startFromEnd - 1; i >= endFromEnd; i-- {
		lines = append(lines, m.renderCashflowOverviewRow(width, rows[i], base))
	}

	if missingRates > 0 {
		lines = append(lines, "", mutedStyle.Render(fmt.Sprintf("%d record(s) skipped due to missing conversion rate.", missingRates)))
	}

	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m TheApplication) renderCashflowOverviewHeader(width int, base string) string {
	monthWidth := 14
	incomeWidth := 14
	expenseWidth := 14
	netWidth := 14

	compareWidth := width - 14 - monthWidth - incomeWidth - expenseWidth - netWidth - 12
	if compareWidth < 16 {
		compareWidth = 16
	}

	header := fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s", monthWidth, "Month", incomeWidth, "Income ("+base+")", expenseWidth, "Expense ("+base+")", netWidth, "Net ("+base+")", compareWidth, "Net vs previous")

	return tableHeaderStyle.Render(header)
}

func (m TheApplication) updateCashflowNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

		return m, nil
	case "up", "shift+tab":
		m.addCashflowForm = m.addCashflowForm.Prev()

		return m, nil
	case "down", "tab":
		m.addCashflowForm = m.addCashflowForm.Next()

		return m, nil
	case "left":
		if m.addCashflowForm.Active == cashflow.CashflowFieldCurrency && m.addCashflowForm.CurrencyIndex > 0 {
			m.addCashflowForm.CurrencyIndex--
		}

		if m.addCashflowForm.Active == cashflow.CashflowFieldCategory && m.addCashflowForm.CategoryIndex > 0 {
			m.addCashflowForm.CategoryIndex--
		}

		if m.addCashflowForm.Active == cashflow.CashflowFieldAccount && m.addCashflowForm.AccountIndex > 0 {
			m.addCashflowForm.AccountIndex--
		}

		return m, nil
	case "right":
		if m.addCashflowForm.Active == cashflow.CashflowFieldCurrency && m.addCashflowForm.CurrencyIndex < len(m.addCashflowForm.CurrencyOptions)-1 {
			m.addCashflowForm.CurrencyIndex++
		}

		if m.addCashflowForm.Active == cashflow.CashflowFieldCategory && m.addCashflowForm.CategoryIndex < len(m.addCashflowForm.CategoryOptions)-1 {
			m.addCashflowForm.CategoryIndex++
		}

		if m.addCashflowForm.Active == cashflow.CashflowFieldAccount && m.addCashflowForm.AccountIndex < len(m.addCashflowForm.AccountOptions)-1 {
			m.addCashflowForm.AccountIndex++
		}

		return m, nil
	case "enter":
		if m.addCashflowForm.Active == cashflow.CashflowFieldCount-1 {
			return m.saveCashflowFromForm()
		}

		m.addCashflowForm = m.addCashflowForm.Next()

		return m, nil
	}

	if inputIndex := m.addCashflowForm.InputIndexForField(m.addCashflowForm.Active); inputIndex >= 0 {
		var cmd tea.Cmd

		m.addCashflowForm.Inputs[inputIndex], cmd = m.addCashflowForm.Inputs[inputIndex].Update(msg)

		return m, cmd
	}

	return m, nil
}

func (m TheApplication) saveCashflowFromForm() (tea.Model, tea.Cmd) {
	if len(m.addCashflowForm.CategoryOptions) == 0 {
		if m.addCashflowForm.IsIncome {
			m.status = "no income categories configured; add one in settings"
		} else {
			m.status = "no expense categories configured; add one in settings"
		}

		return m, nil
	}

	amountRaw := strings.TrimSpace(m.addCashflowForm.Inputs[0].Value())
	dateRaw := strings.TrimSpace(m.addCashflowForm.Inputs[1].Value())
	comment := strings.TrimSpace(m.addCashflowForm.Inputs[2].Value())

	amount, err := parseAmountCents(amountRaw)
	if err != nil {
		m.status = "amount error: " + err.Error()

		return m, nil
	}

	entryDate, err := parseRequiredDate(dateRaw)
	if err != nil {
		m.status = "date must use DD.MM.YYYY format"

		return m, nil
	}

	category := selectedStringOption(m.addCashflowForm.CategoryOptions, m.addCashflowForm.CategoryIndex)
	if category == "" {
		m.status = "category is required"

		return m, nil
	}

	now := time.Now()
	entry := cashflow.CashflowEntry{
		IsIncome:      m.addCashflowForm.IsIncome,
		Currency:      selectedCurrencyOption(m.addCashflowForm.CurrencyOptions, m.addCashflowForm.CurrencyIndex),
		AmountCents:   amount,
		EntryDate:     entryDate,
		Category:      category,
		AccountName:   selectedStringOption(m.addCashflowForm.AccountOptions, m.addCashflowForm.AccountIndex),
		Comment:       comment,
		LastUpdatedAt: now,
	}

	if err := m.storage.CreateCashflow(&entry); err != nil {
		m.status = "save failed: " + err.Error()

		return m, nil
	}

	m.cashflows = append([]cashflow.CashflowEntry{entry}, m.cashflows...)
	if entry.IsIncome {
		m.addCashflowForm = cashflow.NewAddCashflowForm(currencySelectionOptions(m.settings), incomeCategorySelectionOptions(m.settings), accountSelectionOptions(m.accounts), true)
		m.status = "saved income"
	} else {
		m.addCashflowForm = cashflow.NewAddCashflowForm(currencySelectionOptions(m.settings), expenseCategorySelectionOptions(m.settings), accountSelectionOptions(m.accounts), false)
		m.status = "saved expense"
	}

	m.screen = screenCashflowHistory
	m.cashflowHistoryMonth = beginningOfMonth(entry.EntryDate)
	m.cashflowCursor = 0

	return m, nil
}

func (m TheApplication) updateCashflowHistory(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if updatedModel, cmd, handled := m.handleDeleteConfirmation(msg); handled {
		return updatedModel, cmd
	}

	items := m.filteredCashflowsForMonth()

	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

		return m, nil
	case "left":
		m.cashflowHistoryMonth = monthShift(m.cashflowHistoryMonth, -1)
		m.cashflowCursor = 0

		return m, nil
	case "right":
		m.cashflowHistoryMonth = monthShift(m.cashflowHistoryMonth, 1)
		m.cashflowCursor = 0

		return m, nil
	case "up":
		if m.cashflowCursor > 0 {
			m.cashflowCursor--
		}

		return m, nil
	case "down":
		if m.cashflowCursor < len(items)-1 {
			m.cashflowCursor++
		}

		return m, nil
	case "backspace", "delete":
		if len(items) == 0 || m.cashflowCursor < 0 || m.cashflowCursor >= len(items) {
			m.status = "no entry selected"

			return m, nil
		}

		selected := items[m.cashflowCursor]
		targetName := selected.EntryDate.Local().Format("2006-01-02") + " " + selected.Category
		m = m.beginDeleteConfirmation("cashflow", selected.ID, targetName)

		return m, nil
	default:
		return m, nil
	}
}

func (m TheApplication) updateCashflowOverview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows, _ := m.monthlyCashflowOverviewRows()
	pageSize := m.cashflowOverviewPageSize()
	totalPages := 1

	if pageSize > 0 && len(rows) > 0 {
		totalPages = (len(rows) + pageSize - 1) / pageSize
	}

	m.cashflowOverviewPage = clamp(m.cashflowOverviewPage, 0, totalPages-1)

	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = databaseStatus(m.created, len(m.accounts), m.dbPath)

		return m, nil
	case "left", "h", "up", "k", "pgup":
		if m.cashflowOverviewPage > 0 {
			m.cashflowOverviewPage--
		}

		return m, nil
	case "right", "l", "down", "j", "pgdown":
		if m.cashflowOverviewPage < totalPages-1 {
			m.cashflowOverviewPage++
		}

		return m, nil
	case "home":
		m.cashflowOverviewPage = 0

		return m, nil
	case "end":
		m.cashflowOverviewPage = totalPages - 1

		return m, nil
	default:
		return m, nil
	}
}

func (m TheApplication) cashflowOverviewPageSize() int {
	if m.height <= 0 {
		return 18
	}

	size := m.height - 16
	if size < 6 {
		size = 6
	}

	return size
}

func (m TheApplication) filteredCashflowsForMonth() []cashflow.CashflowEntry {
	month := beginningOfMonth(m.cashflowHistoryMonth)
	filtered := make([]cashflow.CashflowEntry, 0)

	for _, entry := range m.cashflows {
		itemMonth := beginningOfMonth(entry.EntryDate)
		if itemMonth.Year() == month.Year() && itemMonth.Month() == month.Month() {
			filtered = append(filtered, entry)
		}
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].EntryDate.Equal(filtered[j].EntryDate) {
			return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
		}

		return filtered[i].EntryDate.After(filtered[j].EntryDate)
	})

	return filtered
}

func (m TheApplication) monthlyCashflowOverviewRows() ([]cashflow.CashflowMonthlyOverviewRow, int) {
	if len(m.cashflows) == 0 {
		return nil, 0
	}

	totalsByMonth := make(map[time.Time]cashflow.CashflowMonthlyOverviewRow)
	missingRates := 0

	for _, entry := range m.cashflows {
		amountBase, ok := m.convertToBaseCents(entry.Currency, entry.AmountCents)
		if !ok {
			missingRates++
			continue
		}

		month := beginningOfMonth(entry.EntryDate)
		row := totalsByMonth[month]
		row.Month = month

		if entry.IsIncome {
			row.IncomeBase += amountBase
		} else {
			row.ExpenseBase += amountBase
		}

		totalsByMonth[month] = row
	}

	if len(totalsByMonth) == 0 {
		return nil, missingRates
	}

	minMonthSet := false

	var minMonth time.Time
	var maxMonth time.Time

	for month := range totalsByMonth {
		if !minMonthSet {
			minMonth = month
			maxMonth = month
			minMonthSet = true

			continue
		}

		if month.Before(minMonth) {
			minMonth = month
		}

		if month.After(maxMonth) {
			maxMonth = month
		}
	}

	rows := make([]cashflow.CashflowMonthlyOverviewRow, 0)

	var prevNet int64

	hasPrev := false

	for month := minMonth; !month.After(maxMonth); month = monthShift(month, 1) {
		row := totalsByMonth[month]
		row.Month = month
		row.NetBase = row.IncomeBase - row.ExpenseBase
		if hasPrev {
			row.HasPrev = true
			row.DeltaFromPrev = row.NetBase - prevNet
		}
		rows = append(rows, row)
		prevNet = row.NetBase
		hasPrev = true
	}

	return rows, missingRates
}

func (m TheApplication) confirmDeleteCashflow() (tea.Model, tea.Cmd) {
	index := -1

	for i := range m.cashflows {
		if m.cashflows[i].ID == m.deleteConfirmID {
			index = i

			break
		}
	}
	if index < 0 {
		m = m.clearDeleteConfirmation("entry not found")

		return m, nil
	}

	selected := m.cashflows[index]
	if err := m.storage.DeleteCashflow(selected.ID); err != nil {
		m = m.clearDeleteConfirmation("delete failed: " + err.Error())

		return m, nil
	}

	m.cashflows = append(m.cashflows[:index], m.cashflows[index+1:]...)

	items := m.filteredCashflowsForMonth()
	if len(items) == 0 {
		m.cashflowCursor = 0
	} else if m.cashflowCursor >= len(items) {
		m.cashflowCursor = len(items) - 1
	}

	m = m.clearDeleteConfirmation("deleted cashflow entry")

	return m, nil
}
