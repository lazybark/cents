package bot

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lazybark/cents/flows/budget"
	"github.com/lazybark/cents/money"
	"github.com/lazybark/cents/telegram"
)

const helpText = `Send an amount to add an expense; start with + for an income:

<code>12.50 food lunch</code>
<code>+3000 salary</code>
<code>15 usd taxi #work</code>
<code>40€ eat dinner @bank yesterday</code>

After the amount: a currency (the base one if left out), the category (or the start of its name), a comment, #tags, @account, and a day (yesterday, 2026-10-05 or 5.10). Without a category, I ask. Each entry has an Undo button.

/month — this month's incomes and expenses (/month 2026-09 for another)
/last — the latest entries (/last 20 for more)
/budgets — budgets this month
/accounts — accounts and their balances
/overview — net worth, debts and payments coming up
/due — what's due in the next 7 days, or overdue`

// formatMoney formats cents in currency the way the app does: "-€ 12.34".
func formatMoney(currency string, cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}

	amount := fmt.Sprintf("%d.%02d", cents/100, cents%100)
	if currency == "" {
		return sign + amount
	}

	return sign + telegram.Escape(currency) + " " + amount
}

// describe is an entry in one line: "Expense € 12.50 · Food · lunch #work".
// The day shows unless it's today.
func describe(e entry, today time.Time) string {
	kind := "Expense"
	if e.IsIncome {
		kind = "Income"
	}

	cents, _ := money.ParseAmountCents(e.Amount)
	parts := []string{"<b>" + kind + "</b> " + formatMoney(e.Currency, cents)}

	if e.Category != "" {
		parts = append(parts, telegram.Escape(e.Category))
	}

	if e.Comment != "" {
		parts = append(parts, telegram.Escape(e.Comment))
	}

	line := strings.Join(parts, " · ")
	for _, tag := range e.Tags {
		line += " #" + telegram.Escape(tag)
	}

	if e.Account != "" {
		line += " → " + telegram.Escape(e.Account)
	}

	if day, err := time.Parse("2006-01-02", e.Date); err == nil && e.Date != today.Format("2006-01-02") {
		line += " · " + day.Format("2 Jan")
	}

	return line
}

// budgetNote says how the entry's budget stands in its month, when it has
// one, or how all spending does once that's close or over.
func (bt *Bot) budgetNote(e entry, month string) string {
	if e.IsIncome {
		return ""
	}

	view, err := bt.b.Budgets(month)
	if err != nil {
		return ""
	}

	for _, row := range view.Rows {
		if !row.IsTotal && strings.EqualFold(row.Category, e.Category) {
			return budgetLine(row.State, row.Name, row.SpentCents, row.LimitCents, row.Percent, view.BaseCurrency)
		}
	}

	for _, row := range view.Rows {
		if row.IsTotal && row.State != budget.StateOK {
			return budgetLine(row.State, row.Name, row.SpentCents, row.LimitCents, row.Percent, view.BaseCurrency)
		}
	}

	return ""
}

func budgetLine(state, name string, spent, limit int64, percent float64, base string) string {
	mark := map[string]string{budget.StateOK: "🟢", budget.StateClose: "🟡", budget.StateOver: "🔴"}[state]
	return fmt.Sprintf("%s %s: %s of %s (%.0f%%)", mark, telegram.Escape(name), formatMoney(base, spent), formatMoney(base, limit), percent)
}

func monthTitle(month string) string {
	if t, err := time.Parse("2006-01", month); err == nil {
		return t.Format("January 2006")
	}

	return month
}

func (bt *Bot) month(month string) (string, error) {
	view, err := bt.b.CashflowMonth(month)
	if err != nil {
		return "", err
	}

	base := view.BaseCurrency
	lines := []string{
		"<b>" + monthTitle(view.Month) + "</b>",
		"Income: " + formatMoney(base, view.IncomeCents),
		"Expenses: " + formatMoney(base, view.ExpenseCents),
		"Net: " + formatMoney(base, view.NetCents),
	}

	byCategory := map[string]int64{}
	for _, row := range view.Entries {
		if !row.IsIncome && row.HasRate {
			byCategory[row.Category] += row.BaseCents
		}
	}

	type spent struct {
		name  string
		cents int64
	}

	var top []spent
	for name, cents := range byCategory {
		top = append(top, spent{name, cents})
	}

	sort.Slice(top, func(i, j int) bool {
		return top[i].cents > top[j].cents || top[i].cents == top[j].cents && top[i].name < top[j].name
	})
	if len(top) > 0 {
		lines = append(lines, "", "<b>Spent most on</b>")
		for _, s := range top[:min(len(top), 5)] {
			lines = append(lines, telegram.Escape(s.name)+": "+formatMoney(base, s.cents))
		}
	}

	if view.MissingRates > 0 {
		lines = append(lines, "", fmt.Sprintf("<i>%d entries have no rate to %s, so they aren't counted.</i>", view.MissingRates, telegram.Escape(base)))
	}

	if len(view.Entries) == 0 {
		lines = append(lines, "", "Nothing yet.")
	}

	return strings.Join(lines, "\n"), nil
}

func (bt *Bot) last(n int) (string, error) {
	view, err := bt.b.CashflowMonth("")
	if err != nil {
		return "", err
	}

	entries := view.Entries
	// The month before too, early in a month.
	if len(entries) < n {
		start, _ := time.Parse("2006-01", view.Month)
		if before, err := bt.b.CashflowMonth(start.AddDate(0, -1, 0).Format("2006-01")); err == nil {
			entries = append(entries, before.Entries...)
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Date != entries[j].Date {
			return entries[i].Date > entries[j].Date
		}

		return entries[i].ID > entries[j].ID
	})

	if len(entries) == 0 {
		return "No entries this month or last.", nil
	}

	lines := []string{"<b>Latest entries</b>"}
	for _, row := range entries[:min(len(entries), n)] {
		amount := formatMoney(row.Currency, row.AmountCents)
		if row.IsIncome {
			amount = "+" + amount
		}

		line := day(row.Date) + "  " + amount + " · " + telegram.Escape(row.Category)
		if row.Comment != "" {
			line += " · " + telegram.Escape(row.Comment)
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n"), nil
}

func day(date string) string {
	if t, err := time.Parse("2006-01-02", date); err == nil {
		return t.Format("02 Jan")
	}

	return date
}

func (bt *Bot) budgets() (string, error) {
	view, err := bt.b.Budgets("")
	if err != nil {
		return "", err
	}

	if len(view.Rows) == 0 {
		return "No budgets yet: set them in the app, under Budgets.", nil
	}

	lines := []string{"<b>Budgets, " + monthTitle(view.Month) + "</b>"}
	for _, row := range view.Rows {
		lines = append(lines, budgetLine(row.State, row.Name, row.SpentCents, row.LimitCents, row.Percent, view.BaseCurrency))
	}

	return strings.Join(lines, "\n"), nil
}

func (bt *Bot) accounts() (string, error) {
	view, err := bt.b.Accounts(0)
	if err != nil {
		return "", err
	}

	lines := []string{"<b>Accounts</b>: " + formatMoney(view.BaseCurrency, view.TotalCents)}
	for _, row := range view.Accounts {
		if row.Archived {
			continue
		}

		line := telegram.Escape(row.Name) + ": " + formatMoney(row.Currency, row.BalanceCents)
		if row.Currency != view.BaseCurrency && row.HasRate {
			line += " (" + formatMoney(view.BaseCurrency, row.BaseCents) + ")"
		}

		if row.IgnoreInSummaries {
			line += " <i>not in totals</i>"
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n"), nil
}

func (bt *Bot) overview() (string, error) {
	view, err := bt.b.Overview()
	if err != nil {
		return "", err
	}

	base := view.BaseCurrency
	lines := []string{
		"<b>Net worth</b>: " + formatMoney(base, view.NetWorth),
		"Accounts: " + formatMoney(base, view.Accounts),
	}

	optional := []struct {
		label string
		cents int64
	}{
		{"Property", view.Property}, {"Investments", view.Investments},
		{"Owed to me", view.DebtsToMe}, {"I owe", view.DebtsByMe}, {"Credits left", view.Credits},
		{"Unpaid taxes", view.UnpaidTaxes}, {"Invoices to be paid to me", view.InvoicesToMe}, {"Invoices I pay", view.InvoicesByMe},
	}
	for _, item := range optional {
		if item.cents != 0 {
			lines = append(lines, item.label+": "+formatMoney(base, item.cents))
		}
	}

	lines = append(lines, "", "This month so far: "+formatMoney(base, view.MonthlyNet))
	if len(view.Upcoming) > 0 {
		lines = append(lines, "", "<b>Coming up</b>")
		for _, sub := range view.Upcoming[:min(len(view.Upcoming), 5)] {
			when := fmt.Sprintf("in %d days", sub.DueInDays)
			switch {
			case sub.DueInDays == 0:
				when = "today"
			case sub.DueInDays == 1:
				when = "tomorrow"
			case sub.DueInDays < 0:
				when = fmt.Sprintf("%d days overdue", -sub.DueInDays)
			}

			lines = append(lines, telegram.Escape(sub.Name)+": "+formatMoney(sub.Currency, sub.AmountCents)+", "+when)
		}
	}

	return strings.Join(lines, "\n"), nil
}
