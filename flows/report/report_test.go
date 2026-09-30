package report

import (
	"github.com/lazybark/cents/flows/note"
	"strings"
	"testing"
	"time"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/analytics"
	"github.com/lazybark/cents/flows/asset"
	"github.com/lazybark/cents/flows/budget"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/credit"
	"github.com/lazybark/cents/flows/debt"
	"github.com/lazybark/cents/flows/goal"
	"github.com/lazybark/cents/flows/invoice"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/subscription"
	"github.com/lazybark/cents/flows/tax"
	"github.com/lazybark/cents/summary"
)

func d(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, time.UTC) }
func ptr(t time.Time) *time.Time               { return &t }

var stts = settings.AppSettings{BaseCurrency: "€", Currencies: []settings.SettingCurrency{{CurrencyName: "$", RateToBase: 0.5}}}

func input() Input {
	return Input{
		Now: time.Date(2026, 4, 15, 12, 0, 0, 0, time.Local),
		Data: summary.Data{
			Settings: stts,
			Cashflows: []cashflow.CashflowEntry{
				{ID: 2, IsIncome: true, Currency: "€", AmountCents: 300000, RateToBase: 1, AmountBaseCents: 300000, EntryDate: d(2026, 3, 1), Category: "Salary"},
				{ID: 1, Currency: "$", AmountCents: -1, RateToBase: 0.5, AmountBaseCents: 0, EntryDate: d(2026, 2, 28), Category: "Before"},
				{ID: 3, Currency: "$", AmountCents: 12345, RateToBase: 0.9, AmountBaseCents: 11111, EntryDate: d(2026, 3, 31), Category: "Food", AccountName: "Card", Comment: `say "hi", ok`, Tags: []string{"Trip", "gifts"}},
				{ID: 4, Currency: "₾", AmountCents: 500, EntryDate: d(2026, 3, 10), Category: "Taxi"},
			},
			Accounts:      []account.Account{{ID: 1, Name: "Bank", Currency: "$", BalanceCents: 1000}, {ID: 2, Name: "Old", Currency: "€", Archived: true, IgnoreInSummaries: true}},
			Assets:        []asset.Asset{{ID: 1, Kind: "investment", Name: "AAPL", Currency: "$", ValueCents: 3000, CostCents: 2000}, {ID: 2, Kind: "property", Name: "Flat", Currency: "€", ValueCents: 100}},
			Debts:         []debt.Debt{{ID: 1, Peer: "Ann", Currency: "€", AmountCents: 1000, AmountPaidCents: 250, RateToBase: 1, IsOwedToUser: true, DebtCreatedAt: d(2026, 1, 2)}},
			Credits:       []credit.Credit{{ID: 1, Name: "Car", Currency: "€", TotalCents: 5000, PaidCents: 1000, RateToBase: 1, InterestPercent: 7.5, StartDate: d(2025, 5, 1)}},
			Taxes:         []tax.Tax{{ID: 1, TaxCountry: "NL", TaxTypeName: "VAT", Period: "Q1", Currency: "€", AmountDueCents: 900, AmountPaidCents: 300, RateToBase: 1, AmountDueBaseCents: 900, AmountPaidBaseCents: 300}},
			Invoices:      []invoice.Invoice{{Title: "Design", Currency: "$", AmountCents: 1000, RateToBase: 0.5, IsIncoming: true, InvoiceDate: ptr(d(2026, 3, 3))}},
			Goals:         []goal.Goal{{ID: 1, Name: "Trip", Currency: "€", TargetAmountCents: 1000, AmountAccumulatedCents: 1000, DateStartedAt: d(2026, 1, 1)}},
			Subscriptions: []subscription.Subscription{{Name: "Rent", Currency: "$", AmountCents: 10000, Period: "month", IsActive: true, IsObligation: true, NextPaymentDate: ptr(d(2026, 4, 20))}},
		},
		AccountLogs: []account.AccountValueLog{{AccountID: 1, LogDate: d(2026, 3, 5), ValueCents: 900}, {AccountID: 1, LogDate: d(2025, 12, 5), ValueCents: 800}, {AccountID: 99, LogDate: d(2026, 3, 5)}},
		AssetLogs:   []asset.AssetValueLog{{AssetID: 1, LogDate: d(2026, 3, 1), ValueCents: 2500}},
		DebtLogs:    []debt.DebtLog{{DebtID: 1, CreatedAt: time.Date(2026, 3, 2, 15, 0, 0, 0, time.Local), DeltaPaidCents: 250, Note: "cash"}},
		CreditLogs:  []credit.CreditLog{{CreditID: 1, CreatedAt: d(2026, 3, 9), DeltaTotalCents: 500}, {CreditID: 1, CreatedAt: d(2026, 1, 9), DeltaPaidCents: 1000}},
		GoalLogs:    []goal.GoalLog{{GoalID: 1, CreatedAt: d(2026, 3, 20), DeltaAccumulatedCents: 1000}},
		NetWorth:    []analytics.MonthValue{{Month: d(2026, 2, 1), Cents: 10, Estimated: true}, {Month: d(2026, 3, 1), Cents: 20, Manual: true}, {Month: d(2026, 4, 1), Cents: 30}},
		Period:      Period{From: d(2026, 3, 1), To: d(2026, 3, 31)},
	}
}

func TestTimedTablesKeepToThePeriod(t *testing.T) {
	in := input()

	ie := IncomesExpenses(in)
	if len(ie.Rows) != 3 || ie.Header[7] != "Amount (€)" {
		t.Fatalf("expected March's three entries: %+v", ie)
	}

	want := []string{"2026-03-31", "Expense", "Food", "Card", "$", "123.45", "0.9", "111.11", `say "hi", ok`, "Trip, gifts"}
	if strings.Join(ie.Rows[2], "|") != strings.Join(want, "|") {
		t.Fatalf("unexpected row %q", ie.Rows[2])
	}

	if taxi := ie.Rows[1]; taxi[6] != "" || taxi[7] != "" {
		t.Fatalf("no rate, no base amount: %q", taxi)
	}

	in.Notes = []note.MonthNote{{Month: d(2026, 3, 1), Text: "Trip"}, {Month: d(2026, 5, 1), Text: "Later"}}
	if notes := Notes(in); len(notes.Rows) != 1 || notes.Rows[0][1] != "Trip" {
		t.Fatalf("unexpected notes %q", notes.Rows)
	}

	if ms := MonthlySummary(in); len(ms.Rows) != 1 || strings.Join(ms.Rows[0], "|") != "2026-03|3000.00|111.11|2888.89|96.3|Trip" {
		t.Fatalf("unexpected summary %q", ms.Rows)
	}

	if ah := AccountHistory(in); len(ah.Rows) != 1 || strings.Join(ah.Rows[0], "|") != "Bank|2026-03-05|$|9.00|4.50" {
		t.Fatalf("unexpected account history %q", ah.Rows)
	}

	if nw := NetWorth(in); len(nw.Rows) != 1 || nw.Rows[0][2] != "Entered" {
		t.Fatalf("unexpected net worth %q", nw.Rows)
	}

	p := Payments(in)
	if len(p.Rows) != 3 || p.Rows[0][1] != "Debt" || p.Rows[0][0] != "2026-03-02" || p.Rows[1][3] != "Added to the total" || p.Rows[2][1] != "Goal" {
		t.Fatalf("unexpected payments %q", p.Rows)
	}

	in.Period = Period{}
	if len(IncomesExpenses(in).Rows) != 4 || len(AccountHistory(in).Rows) != 2 || len(NetWorth(in).Rows) != 3 {
		t.Fatal("an open period has everything")
	}
}

func TestListTables(t *testing.T) {
	in := input()
	check := func(table Table, row int, want string) {
		t.Helper()
		if got := strings.Join(table.Rows[row], "|"); got != want {
			t.Errorf("%s: want %q, got %q", table.Name, want, got)
		}
	}

	check(Accounts(in), 1, "Old||€|0.00|0.00|no|yes")
	check(Assets(in), 0, "Investment|AAPL||$|30.00|20.00|10.00|15.00||yes|")
	check(Assets(in), 1, "Property|Flat||€|1.00|||1.00||yes|")
	check(Debts(in), 0, "Owed to me|Ann|€|10.00|2.50|7.50|1|7.50|2026-01-02||")
	check(Credits(in), 0, "Car|||€|50.00|10.00|40.00|7.5|1|40.00|2025-05-01||")
	check(Taxes(in), 0, "NL|VAT|Q1|€|9.00|3.00|6.00|1|6.00||")
	check(Invoices(in), 0, "Design|I pay||2026-03-03||no|$|10.00|0.5|5.00|||")
	check(Goals(in), 0, "Trip|€|10.00|10.00|0.00|2026-01-01||yes|")
	check(Subscriptions(in), 0, "Obligation|Rent||$|100.00|month|600.00||2026-04-20||yes|no")
}

func TestBudgetsTable(t *testing.T) {
	in := input()
	in.Data.Budgets = []budget.Budget{{Category: "food", LimitCents: 10000}, {LimitCents: 50000}}

	got := Budgets(in)
	if len(got.Rows) != 2 || strings.Join(got.Rows[0], "|") != "2026-03|All spending|500.00|111.11|388.89|22.2" || strings.Join(got.Rows[1], "|") != "2026-03|food|100.00|111.11|-11.11|111.1" {
		t.Fatalf("unexpected %q", got.Rows)
	}

	in.Data.Budgets = nil
	if len(Budgets(in).Rows) != 0 {
		t.Fatal("no budgets, no rows")
	}
}

func TestCSV(t *testing.T) {
	data, err := CSV(Table{Header: []string{"A", "B"}, Rows: [][]string{{"1", `x, "y"`}}})
	if err != nil {
		t.Fatal(err)
	}

	if got := string(data); got != "\uFEFFA,B\n1,\"x, \"\"y\"\"\"\n" {
		t.Fatalf("unexpected CSV %q", got)
	}

	keys := map[string]bool{}
	for _, ds := range Datasets() {
		if keys[ds.Key] || ds.Label == "" || ds.Build == nil {
			t.Fatalf("bad dataset %+v", ds)
		}
		keys[ds.Key] = true
		if table := ds.Build(input()); table.Name != ds.Key || len(table.Header) == 0 {
			t.Fatalf("%s builds %q", ds.Key, table.Name)
		}
	}
}

func TestPeriod(t *testing.T) {
	p := Period{From: d(2026, 3, 15), To: d(2026, 4, 2)}
	if !p.ContainsMonth(d(2026, 3, 1)) || !p.ContainsMonth(d(2026, 4, 1)) || p.ContainsMonth(d(2026, 2, 1)) || p.ContainsMonth(d(2026, 5, 1)) {
		t.Fatal("months overlapping the period count")
	}

	// A local evening west of UTC is still that day.
	west := time.FixedZone("west", -8*3600)
	if !p.Contains(time.Date(2026, 4, 2, 23, 0, 0, 0, west)) || p.Contains(time.Date(2026, 3, 14, 23, 0, 0, 0, west)) {
		t.Fatal("days are read as saved")
	}
}
