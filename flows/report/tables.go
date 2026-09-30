package report

import (
	"github.com/lazybark/cents/flows/note"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/analytics"
	"github.com/lazybark/cents/flows/asset"
	"github.com/lazybark/cents/flows/budget"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/credit"
	"github.com/lazybark/cents/flows/debt"
	"github.com/lazybark/cents/flows/goal"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/tax"
	"github.com/lazybark/cents/summary"
)

// Input is everything the tables are made from.
type Input struct {
	Data        summary.Data
	AccountLogs []account.AccountValueLog
	AssetLogs   []asset.AssetValueLog
	DebtLogs    []debt.DebtLog
	CreditLogs  []credit.CreditLog
	TaxLogs     []tax.TaxLog
	GoalLogs    []goal.GoalLog
	NetWorth    []analytics.MonthValue
	Notes       []note.MonthNote
	Period      Period
	Now         time.Time
}

// Dataset is a table that can be exported. Timed ones are limited to the
// period.
type Dataset struct {
	Key   string
	Label string
	Timed bool
	Build func(Input) Table
}

// Datasets lists every table, things over time first.
func Datasets() []Dataset {
	return []Dataset{
		{Key: "incomes_expenses", Label: "Incomes & expenses", Timed: true, Build: IncomesExpenses},
		{Key: "monthly_summary", Label: "Monthly summary", Timed: true, Build: MonthlySummary},
		{Key: "account_history", Label: "Account history", Timed: true, Build: AccountHistory},
		{Key: "asset_history", Label: "Property & investment history", Timed: true, Build: AssetHistory},
		{Key: "net_worth", Label: "Net worth by month", Timed: true, Build: NetWorth},
		{Key: "payments", Label: "Payments on debts, credits, taxes and goals", Timed: true, Build: Payments},
		{Key: "budgets", Label: "Budgets by month", Timed: true, Build: Budgets},
		{Key: "month_notes", Label: "Month notes", Timed: true, Build: Notes},
		{Key: "accounts", Label: "Accounts", Build: Accounts},
		{Key: "property_investments", Label: "Property & investments", Build: Assets},
		{Key: "subscriptions_obligations", Label: "Subscriptions & obligations", Build: Subscriptions},
		{Key: "invoices", Label: "Invoices", Build: Invoices},
		{Key: "debts", Label: "Debts", Build: Debts},
		{Key: "credits", Label: "Credits", Build: Credits},
		{Key: "taxes", Label: "Taxes", Build: Taxes},
		{Key: "goals", Label: "Goals", Build: Goals},
	}
}

func base(in Input) string {
	return in.Data.Settings.BaseCurrencyLabel()
}

func IncomesExpenses(in Input) Table {
	entries := make([]cashflow.CashflowEntry, 0)
	for _, e := range in.Data.Cashflows {
		if in.Period.Contains(e.EntryDate) {
			entries = append(entries, e)
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if !a.EntryDate.Equal(b.EntryDate) {
			return a.EntryDate.Before(b.EntryDate)
		}

		return a.ID < b.ID
	})

	t := Table{Name: "incomes_expenses", Header: []string{"Date", "Type", "Category", "Account", "Currency", "Amount", "Rate to base", inBase("Amount", base(in)), "Comment", "Tags"}}
	for _, e := range entries {
		kind := "Expense"
		if e.IsIncome {
			kind = "Income"
		}

		baseCents, ok := e.BaseCents()
		t.Rows = append(t.Rows, []string{day(e.EntryDate), kind, e.Category, e.AccountName, e.Currency, amount(e.AmountCents), rate(e.RateToBase), baseAmount(baseCents, ok), e.Comment, strings.Join(e.Tags, ", ")})
	}

	return t
}

func MonthlySummary(in Input) Table {
	rows, _ := cashflow.MonthlyOverview(in.Data.Cashflows)
	notes := notesByMonth(in)
	t := Table{Name: "monthly_summary", Header: []string{"Month", inBase("Income", base(in)), inBase("Expenses", base(in)), inBase("Net", base(in)), "Savings rate %", "Note"}}
	for _, row := range rows {
		if !in.Period.ContainsMonth(row.Month) {
			continue
		}

		savings := ""
		if r, ok := analytics.SavingsRate(row.IncomeBase, row.ExpenseBase); ok {
			savings = strconv.FormatFloat(r, 'f', 1, 64)
		}

		month := row.Month.Format("2006-01")
		t.Rows = append(t.Rows, []string{month, amount(row.IncomeBase), amount(row.ExpenseBase), amount(row.NetBase), savings, notes[month]})
	}

	return t
}

func AccountHistory(in Input) Table {
	stts := in.Data.Settings
	accounts := map[uint]account.Account{}
	for _, a := range in.Data.Accounts {
		accounts[a.ID] = a
	}

	logs := make([]account.AccountValueLog, 0)
	for _, l := range in.AccountLogs {
		if _, ok := accounts[l.AccountID]; ok && in.Period.Contains(l.LogDate) {
			logs = append(logs, l)
		}
	}

	sort.SliceStable(logs, func(i, j int) bool {
		a, b := accounts[logs[i].AccountID].Name, accounts[logs[j].AccountID].Name
		if a != b {
			return a < b
		}

		return logs[i].LogDate.Before(logs[j].LogDate)
	})

	t := Table{Name: "account_history", Header: []string{"Account", "Date", "Currency", "Value", inBase("Value at today's rate", base(in))}}
	for _, l := range logs {
		acct := accounts[l.AccountID]
		baseCents, ok := stts.ConvertToBaseCents(acct.Currency, l.ValueCents)
		t.Rows = append(t.Rows, []string{acct.Name, day(l.LogDate), acct.Currency, amount(l.ValueCents), baseAmount(baseCents, ok)})
	}

	return t
}

func assetKind(kind string) string {
	if kind == string(asset.KindProperty) {
		return "Property"
	}

	return "Investment"
}

func AssetHistory(in Input) Table {
	stts := in.Data.Settings
	assets := map[uint]asset.Asset{}
	for _, a := range in.Data.Assets {
		assets[a.ID] = a
	}

	logs := make([]asset.AssetValueLog, 0)
	for _, l := range in.AssetLogs {
		if _, ok := assets[l.AssetID]; ok && in.Period.Contains(l.LogDate) {
			logs = append(logs, l)
		}
	}

	sort.SliceStable(logs, func(i, j int) bool {
		a, b := assets[logs[i].AssetID], assets[logs[j].AssetID]
		if a.Kind != b.Kind {
			return a.Kind > b.Kind
		}

		if a.Name != b.Name {
			return a.Name < b.Name
		}

		return logs[i].LogDate.Before(logs[j].LogDate)
	})

	t := Table{Name: "asset_history", Header: []string{"Kind", "Name", "Date", "Currency", "Value", inBase("Value at today's rate", base(in))}}
	for _, l := range logs {
		item := assets[l.AssetID]
		baseCents, ok := stts.ConvertToBaseCents(item.Currency, l.ValueCents)
		t.Rows = append(t.Rows, []string{assetKind(item.Kind), item.Name, day(l.LogDate), item.Currency, amount(l.ValueCents), baseAmount(baseCents, ok)})
	}

	return t
}

func NetWorth(in Input) Table {
	t := Table{Name: "net_worth", Header: []string{"Month", inBase("Net worth", base(in)), "Value"}}
	for _, m := range in.NetWorth {
		if !in.Period.ContainsMonth(m.Month) {
			continue
		}

		source := "Kept by the app"
		switch {
		case m.Estimated:
			source = "Estimated from accounts, property and investments"
		case m.Manual:
			source = "Entered"
		}

		t.Rows = append(t.Rows, []string{m.Month.Format("2006-01"), amount(m.Cents), source})
	}

	return t
}

// Payments lists what was paid or added on debts, credits, taxes and goals.
func Payments(in Input) Table {
	type row struct {
		at   time.Time
		cols []string
	}

	var rows []row
	add := func(at time.Time, kind, name, change, currency string, cents int64, note string) {
		if in.Period.Contains(at) {
			rows = append(rows, row{at: at, cols: []string{day(at), kind, name, change, currency, amount(cents), note}})
		}
	}

	debts := map[uint]debt.Debt{}
	for _, d := range in.Data.Debts {
		debts[d.ID] = d
	}

	for _, l := range in.DebtLogs {
		if d, ok := debts[l.DebtID]; ok {
			add(l.CreatedAt, "Debt", d.Peer, "Paid", d.Currency, l.DeltaPaidCents, l.Note)
		}
	}

	credits := map[uint]credit.Credit{}
	for _, c := range in.Data.Credits {
		credits[c.ID] = c
	}

	for _, l := range in.CreditLogs {
		if c, ok := credits[l.CreditID]; ok {
			if l.Kind() == credit.LogAddition {
				add(l.CreatedAt, "Credit", c.Name, "Added to the total", c.Currency, l.DeltaTotalCents, l.Note)
			} else {
				add(l.CreatedAt, "Credit", c.Name, "Paid", c.Currency, l.DeltaPaidCents, l.Note)
			}
		}
	}

	taxes := map[uint]tax.Tax{}
	for _, t := range in.Data.Taxes {
		taxes[t.ID] = t
	}

	for _, l := range in.TaxLogs {
		if t, ok := taxes[l.TaxID]; ok {
			add(l.CreatedAt, "Tax", strings.TrimSpace(t.TaxCountry+" "+t.TaxTypeName+" "+t.Period), "Paid", t.Currency, l.DeltaPaidCents, l.Note)
		}
	}

	goals := map[uint]goal.Goal{}
	for _, g := range in.Data.Goals {
		goals[g.ID] = g
	}

	for _, l := range in.GoalLogs {
		if g, ok := goals[l.GoalID]; ok {
			add(l.CreatedAt, "Goal", g.Name, "Put aside", g.Currency, l.DeltaAccumulatedCents, l.Note)
		}
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i].at.Before(rows[j].at) })

	t := Table{Name: "payments", Header: []string{"Date", "Kind", "Name", "Change", "Currency", "Amount", "Note"}}
	for _, r := range rows {
		t.Rows = append(t.Rows, r.cols)
	}

	return t
}

func Accounts(in Input) Table {
	stts := in.Data.Settings
	t := Table{Name: "accounts", Header: []string{"Name", "Description", "Currency", "Balance", inBase("Balance at today's rate", base(in)), "Counted in summaries", "Archived"}}
	for _, a := range in.Data.Accounts {
		baseCents, ok := stts.ConvertToBaseCents(a.Currency, a.BalanceCents)
		t.Rows = append(t.Rows, []string{a.Name, a.Description, a.Currency, amount(a.BalanceCents), baseAmount(baseCents, ok), yesNo(!a.IgnoreInSummaries), yesNo(a.Archived)})
	}

	return t
}

func Assets(in Input) Table {
	stts := in.Data.Settings
	t := Table{Name: "property_investments", Header: []string{"Kind", "Name", "Type", "Currency", "Value", "Cost", "Gain", inBase("Value at today's rate", base(in)), "Acquired", "Counted in net worth", "Description"}}
	for _, a := range in.Data.Assets {
		cost, gain := "", ""
		if a.CostCents > 0 {
			cost, gain = amount(a.CostCents), amount(a.ValueCents-a.CostCents)
		}

		baseCents, ok := stts.ConvertToBaseCents(a.Currency, a.ValueCents)
		t.Rows = append(t.Rows, []string{assetKind(a.Kind), a.Name, a.Type, a.Currency, amount(a.ValueCents), cost, gain, baseAmount(baseCents, ok), optionalDay(a.AcquiredAt), yesNo(!a.IgnoreInNetWorth), a.Description})
	}

	return t
}

func Subscriptions(in Input) Table {
	stts := in.Data.Settings
	t := Table{Name: "subscriptions_obligations", Header: []string{"Kind", "Name", "Type", "Currency", "Amount", "Period", inBase("A year at today's rate", base(in)), "Payment method", "Next payment", "Last paid", "Active", "Paid by hand"}}
	for _, s := range in.Data.Subscriptions {
		kind := "Subscription"
		if s.IsObligation {
			kind = "Obligation"
		}

		yearly := ""
		if converted, ok := stts.ConvertToBaseCents(s.Currency, s.AmountCents); ok {
			_, perYear := s.PerMonthAndYear(converted)
			yearly = amount(perYear)
		}

		next := ""
		if at, ok := s.NextPayment(in.Now); ok && s.IsActive {
			next = day(at)
		}

		t.Rows = append(t.Rows, []string{kind, s.Name, s.Type, s.Currency, amount(s.AmountCents), s.Period, yearly, s.PaymentMethod, next, optionalDay(s.LastPaidDate), yesNo(s.IsActive), yesNo(s.PaidManually)})
	}

	return t
}

func Invoices(in Input) Table {
	t := Table{Name: "invoices", Header: []string{"Title", "Direction", "Peer", "Issued", "Due", "Paid", "Currency", "Amount", "Rate to base", inBase("Amount", base(in)), "Target account", "URL", "Description"}}
	for _, inv := range in.Data.Invoices {
		direction := "They pay me"
		if inv.IsIncoming {
			direction = "I pay"
		}

		baseCents, ok := settings.BaseCents(inv.AmountCents, inv.RateToBase)
		t.Rows = append(t.Rows, []string{inv.Title, direction, inv.Peer, optionalDay(inv.InvoiceDate), optionalDay(inv.DueDate), yesNo(inv.Paid), inv.Currency, amount(inv.AmountCents), rate(inv.RateToBase), baseAmount(baseCents, ok), inv.TargetAccount, inv.URL, inv.Description})
	}

	return t
}

func Debts(in Input) Table {
	t := Table{Name: "debts", Header: []string{"Direction", "Peer", "Currency", "Amount", "Paid", "Left", "Rate to base", inBase("Left", base(in)), "Created", "Due", "Comment"}}
	for _, d := range in.Data.Debts {
		direction := "I owe"
		if d.IsOwedToUser {
			direction = "Owed to me"
		}

		baseCents, ok := settings.BaseCents(d.LeftCents(), d.RateToBase)
		t.Rows = append(t.Rows, []string{direction, d.Peer, d.Currency, amount(d.AmountCents), amount(d.AmountPaidCents), amount(d.LeftCents()), rate(d.RateToBase), baseAmount(baseCents, ok), day(d.DebtCreatedAt), optionalDay(d.DueDate), d.Comment})
	}

	return t
}

func Credits(in Input) Table {
	t := Table{Name: "credits", Header: []string{"Name", "Purpose", "Issuer", "Currency", "Total", "Paid", "Left", "Interest % a year", "Rate to base", inBase("Left", base(in)), "Start", "Due", "Comment"}}
	for _, c := range in.Data.Credits {
		interest := ""
		if c.InterestPercent > 0 {
			interest = strconv.FormatFloat(c.InterestPercent, 'f', -1, 64)
		}

		baseCents, ok := settings.BaseCents(c.LeftCents(), c.RateToBase)
		t.Rows = append(t.Rows, []string{c.Name, c.Purpose, c.Issuer, c.Currency, amount(c.TotalCents), amount(c.PaidCents), amount(c.LeftCents()), interest, rate(c.RateToBase), baseAmount(baseCents, ok), day(c.StartDate), optionalDay(c.DueDate), c.Comment})
	}

	return t
}

func Taxes(in Input) Table {
	t := Table{Name: "taxes", Header: []string{"Country", "Type", "Period", "Currency", "Due", "Paid", "Left", "Rate to base", inBase("Left", base(in)), "Due date", "Comment"}}
	for _, x := range in.Data.Taxes {
		t.Rows = append(t.Rows, []string{x.TaxCountry, x.TaxTypeName, x.Period, x.Currency, amount(x.AmountDueCents), amount(x.AmountPaidCents), amount(x.LeftCents()), rate(x.RateToBase), amount(x.LeftBaseCents()), optionalDay(x.DueDate), x.Comment})
	}

	return t
}

func Goals(in Input) Table {
	t := Table{Name: "goals", Header: []string{"Name", "Currency", "Target", "Saved", "Left", "Started", "Target date", "Reached", "Description"}}
	for _, g := range in.Data.Goals {
		t.Rows = append(t.Rows, []string{g.Name, g.Currency, amount(g.TargetAmountCents), amount(g.AmountAccumulatedCents), amount(g.LeftCents()), day(g.DateStartedAt), optionalDay(g.TargetDate), yesNo(g.IsDone()), g.Description})
	}

	return t
}

// Budgets is each budget against each month's spending, from the period's
// first month (or the first expense) to its last (or now), at the limits
// set now.
func Budgets(in Input) Table {
	t := Table{Name: "budgets", Header: []string{"Month", "Budget", inBase("Limit", base(in)), inBase("Spent", base(in)), inBase("Left", base(in)), "Used %"}}
	if len(in.Data.Budgets) == 0 {
		return t
	}

	last := cashflow.MonthStart(in.Now)
	if !in.Period.To.IsZero() && cashflow.MonthStart(in.Period.To).Before(last) {
		last = cashflow.MonthStart(in.Period.To)
	}

	first := last
	if !in.Period.From.IsZero() {
		first = cashflow.MonthStart(in.Period.From)
	} else {
		for _, e := range in.Data.Cashflows {
			if m := cashflow.MonthStart(e.EntryDate); !e.IsIncome && m.Before(first) {
				first = m
			}
		}
	}

	for month := first; !month.After(last); month = month.AddDate(0, 1, 0) {
		progress, _ := budget.Month(in.Data.Budgets, in.Data.Cashflows, month)
		for _, p := range progress {
			name := p.Budget.Category
			if p.Budget.IsTotal() {
				name = "All spending"
			}

			t.Rows = append(t.Rows, []string{month.Format("2006-01"), name, amount(p.Budget.LimitCents), amount(p.SpentCents), amount(p.LeftCents), strconv.FormatFloat(p.Percent, 'f', 1, 64)})
		}
	}

	return t
}

func notesByMonth(in Input) map[string]string {
	notes := make(map[string]string, len(in.Notes))
	for _, n := range in.Notes {
		notes[n.Month.UTC().Format(note.MonthLayout)] = n.Text
	}

	return notes
}

// Notes lists the notes on months in the period.
func Notes(in Input) Table {
	t := Table{Name: "month_notes", Header: []string{"Month", "Note"}}
	for _, n := range in.Notes {
		if in.Period.ContainsMonth(n.Month.UTC()) {
			t.Rows = append(t.Rows, []string{n.Month.UTC().Format(note.MonthLayout), n.Text})
		}
	}

	return t
}
