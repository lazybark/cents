package app

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/settings"
	"gorm.io/gorm"
)

func (m TheApplication) loadAccountValueLogs(accountID uint) []account.AccountValueLog {
	if accountID == 0 {
		return nil
	}

	logs := make([]account.AccountValueLog, 0)
	if err := m.db.Where("account_id = ?", accountID).Order("log_date desc, id desc").Find(&logs).Error; err != nil {
		return nil
	}

	return logs
}

func (m TheApplication) upsertAccountValueLog(accountID uint, day time.Time, valueCents int64) error {
	if accountID == 0 {
		return errors.New("account is required")
	}

	normalizedDay := accountLogDay(day)
	now := time.Now()
	entry := account.AccountValueLog{}

	err := m.db.Where("account_id = ? AND log_date = ?", accountID, normalizedDay).First(&entry).Error
	if err == nil {
		entry.ValueCents = valueCents
		entry.UpdatedAt = now

		return m.db.Save(&entry).Error
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	entry = account.AccountValueLog{
		AccountID:  accountID,
		LogDate:    normalizedDay,
		ValueCents: valueCents,
		UpdatedAt:  now,
	}

	return m.db.Create(&entry).Error
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
