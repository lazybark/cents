package analytics

import (
	"testing"
	"time"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/asset"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/credit"
	"github.com/lazybark/cents/flows/debt"
	"github.com/lazybark/cents/flows/goal"
	"github.com/lazybark/cents/flows/invoice"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/subscription"
	"github.com/lazybark/cents/summary"
)

func TestRateHistory(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.Local)
	rates := RateHistory(RateSources{
		Settings:  stts, // $ at 0.5 now
		Cashflows: []cashflow.CashflowEntry{{Currency: "$", RateToBase: 0.8, EntryDate: d(2026, 1, 10)}, {Currency: "$", RateToBase: 0.9, EntryDate: d(2026, 2, 20)}},
		Records: []settings.RateRecord{
			{Day: d(2026, 2, 20), Currency: "$", Base: "€", RateToBase: 0.7}, // wins over the entry that day
			{Day: d(2026, 3, 1), Currency: "$", Base: "£", RateToBase: 9},    // another base: ignored
		},
	}, now)

	for _, c := range []struct {
		day  time.Time
		want float64
		ok   bool
	}{{d(2026, 1, 9), 0, false}, {d(2026, 1, 10), 0.8, true}, {d(2026, 2, 19), 0.8, true}, {d(2026, 3, 31), 0.7, true}, {now, 0.5, true}} {
		if got, ok := rates.At("$", c.day); got != c.want || ok != c.ok {
			t.Errorf("%v: want %v %v, got %v %v", c.day, c.want, c.ok, got, ok)
		}
	}

	if got, ok := rates.At(" € ", d(2000, 1, 1)); got != 1 || !ok {
		t.Fatal("the base currency is always 1")
	}
}

func TestExchangeEffect(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.Local)
	h := History{
		Settings:    stts,
		Accounts:    []account.Account{{ID: 1, Currency: "$", BalanceCents: 1000}, {ID: 2, Currency: "€", BalanceCents: 5000}, {ID: 3, Currency: "₾", BalanceCents: 100}},
		AccountLogs: []account.AccountValueLog{{AccountID: 1, LogDate: d(2026, 1, 5), ValueCents: 1000}, {AccountID: 2, LogDate: d(2026, 1, 5), ValueCents: 5000}, {AccountID: 3, LogDate: d(2026, 1, 5), ValueCents: 100}},
	}
	rates := RateHistory(RateSources{Settings: stts, Records: []settings.RateRecord{
		{Day: d(2026, 1, 31), Currency: "$", Base: "€", RateToBase: 0.8},
		{Day: d(2026, 2, 28), Currency: "$", Base: "€", RateToBase: 0.9},
	}}, now)

	fx := ExchangeEffect(h, rates, d(2026, 2, 1), now)
	// February: $10 from 0.8 to 0.9 = +€1. March: 0.9 to today's 0.5 = -€4.
	if len(fx.Months) != 2 || fx.Months[0].Cents != 100 || fx.Months[1].Cents != -400 || fx.TotalCents != -300 {
		t.Fatalf("unexpected %+v", fx)
	}

	if len(fx.ByCurrency) != 1 || fx.ByCurrency[0].Currency != "$" || len(fx.Missing) != 1 || fx.Missing[0] != "₾" {
		t.Fatalf("expected $ counted and ₾ without a rate: %+v", fx)
	}
}

func TestCurrencyExposure(t *testing.T) {
	data := summary.Data{
		Settings: stts,
		Accounts: []account.Account{{Currency: "€", BalanceCents: 2000}, {Currency: "$", BalanceCents: 2000}, {Currency: "$", BalanceCents: 99999, IgnoreInSummaries: true}},
		Assets:   []asset.Asset{{Currency: "$", ValueCents: 4000}, {Currency: "₾", ValueCents: 700}},
		Debts:    []debt.Debt{{Currency: "$", AmountCents: 1000, IsOwedToUser: true}, {Currency: "€", AmountCents: 500}},
		Credits:  []credit.Credit{{Currency: "$", TotalCents: 3000, PaidCents: 1000}},
		Invoices: []invoice.Invoice{{Currency: "€", AmountCents: 200, Paid: true}},
	}

	got := CurrencyExposure(data)
	if len(got) != 3 || got[0].Currency != "$" || got[1].Currency != "€" || got[2].Currency != "₾" {
		t.Fatalf("unexpected order %+v", got)
	}

	usd := got[0]
	if usd.OwnedCents != 6000 || usd.OwedToMeCents != 1000 || usd.OwedByMeCents != 2000 || usd.NetCents() != 5000 || usd.OwnedBaseCents != 3000 || usd.NetBaseCents != 2500 || usd.Share != 60 {
		t.Fatalf("unexpected $ %+v", usd)
	}

	if got[1].NetBaseCents != 1500 || got[1].Share != 40 || got[2].HasRate || got[2].Share != 0 {
		t.Fatalf("unexpected %+v", got[1:])
	}
}

func TestAssetGrowth(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.Local)
	assets := []asset.Asset{
		{ID: 1, Kind: "investment", Currency: "$", ValueCents: 3000, CostCents: 2000},
		{ID: 2, Kind: "investment", Currency: "€", ValueCents: 500},
		{ID: 3, Kind: "property", Currency: "€", ValueCents: 9999},
	}
	logs := []asset.AssetValueLog{{AssetID: 1, LogDate: d(2026, 1, 10), ValueCents: 2000}, {AssetID: 2, LogDate: d(2026, 2, 3), ValueCents: 400}}

	got := AssetGrowth(assets, logs, stts, asset.KindInvestment, d(2026, 1, 1), now)
	want := []AssetMonth{
		{Month: d(2026, 1, 1), ValueCents: 1000, CostCents: 1000, GainCents: 0, HasCost: true},
		{Month: d(2026, 2, 1), ValueCents: 1400, CostCents: 1000, GainCents: 0, HasCost: true},
		{Month: d(2026, 3, 1), ValueCents: 2000, CostCents: 1000, GainCents: 500, HasCost: true},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("month %d: want %+v, got %+v", i, want[i], got[i])
		}
	}
}

func TestGoalPace(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.Local)
	goals := []goal.Goal{
		{ID: 1, Name: "Car", TargetAmountCents: 100000, AmountAccumulatedCents: 40000, DateStartedAt: d(2025, 1, 1), TargetDate: ptr(d(2026, 7, 1))},
		{ID: 2, Name: "Trip", TargetAmountCents: 10000, AmountAccumulatedCents: 2000, DateStartedAt: d(2026, 1, 15)},
		{ID: 3, Name: "Done", TargetAmountCents: 100, AmountAccumulatedCents: 100},
		{ID: 4, Name: "Idle", TargetAmountCents: 100, AmountAccumulatedCents: 10, DateStartedAt: d(2025, 1, 1)},
		{ID: 5, Name: "Fast", TargetAmountCents: 10000, AmountAccumulatedCents: 7000, DateStartedAt: d(2025, 1, 1), TargetDate: ptr(d(2027, 1, 1))},
	}
	logs := []goal.GoalLog{
		{GoalID: 1, CreatedAt: d(2025, 6, 1), DeltaAccumulatedCents: 10000}, // older than six months
		{GoalID: 1, CreatedAt: d(2026, 1, 1), DeltaAccumulatedCents: 6000},
		{GoalID: 1, CreatedAt: d(2026, 3, 1), DeltaAccumulatedCents: 6000},
		{GoalID: 4, CreatedAt: d(2025, 2, 1), DeltaAccumulatedCents: 10},
		{GoalID: 5, CreatedAt: d(2026, 4, 1), DeltaAccumulatedCents: 6000},
	}

	got := map[string]GoalPace{}
	for _, p := range Pace(goals, logs, now) {
		got[p.Name] = p
	}

	if _, ok := got["Done"]; ok || len(got) != 4 {
		t.Fatalf("done goals are left out: %+v", got)
	}

	// 120.00 over six months is about 20.00 a month (182 days are a bit
	// short of six average months); 600.00 left takes about 30 months.
	car := got["Car"]
	if car.PerMonthCents < 1990 || car.PerMonthCents > 2010 || car.Status != PaceBehind || car.Projected == nil || car.MonthsLate < 25 || car.NeededPerMonth < 20000 {
		t.Fatalf("unexpected car %+v", car)
	}

	// No log: 20.00 since mid-January, three months.
	if trip := got["Trip"]; trip.Status != PaceNoTarget || trip.PerMonthCents < 600 || trip.PerMonthCents > 700 || trip.Projected == nil {
		t.Fatalf("unexpected trip %+v", trip)
	}

	if idle := got["Idle"]; idle.Status != PaceStalled || idle.Projected != nil {
		t.Fatalf("unexpected idle %+v", idle)
	}

	if fast := got["Fast"]; fast.Status != PaceOnTrack || fast.MonthsLate >= 0 {
		t.Fatalf("unexpected fast %+v", fast)
	}
}

func TestCommitments(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.Local)
	subs := []subscription.Subscription{
		{Currency: "€", AmountCents: 60000, Period: "month", IsActive: true, IsObligation: true},
		{Currency: "€", AmountCents: 1000, Period: "month", IsActive: true}, // a subscription, not counted
		{Currency: "€", AmountCents: 9000, Period: "month", IsObligation: true},
	}
	credits := []credit.Credit{
		{ID: 1, Currency: "$", TotalCents: 100000, PaidCents: 10000, RateToBase: 0.5, StartDate: d(2025, 1, 1)},
		{ID: 2, Currency: "€", TotalCents: 50000, PaidCents: 2000, RateToBase: 1, StartDate: d(2026, 2, 10)},
		{ID: 3, Currency: "€", TotalCents: 100, PaidCents: 100, RateToBase: 1, StartDate: d(2025, 1, 1)},
	}
	at := func(m time.Month) time.Time { return time.Date(2026, m, 5, 10, 0, 0, 0, time.Local) }
	logs := []credit.CreditLog{
		{CreditID: 1, CreatedAt: at(1), DeltaPaidCents: 6000},
		{CreditID: 1, CreatedAt: at(3), DeltaPaidCents: 6000},
		{CreditID: 1, CreatedAt: at(4), DeltaPaidCents: 6000},  // this month: not yet
		{CreditID: 1, CreatedAt: at(2), DeltaTotalCents: 5000}, // an addition
		{CreditID: 2, CreatedAt: at(2), DeltaPaidCents: 1000},  // two months since it started
		{CreditID: 2, CreatedAt: at(3), DeltaPaidCents: 1000},
		{CreditID: 3, CreatedAt: at(3), DeltaPaidCents: 100}, // paid off
	}

	c := CommitmentsPerMonth(subs, credits, logs, stts, now, 200000)
	// Obligations 600.00; credit 1: $120 over six months at 0.5 = 10.00;
	// credit 2: 20.00 over two months = 10.00.
	if c.ObligationsCents != 60000 || c.CreditPaymentsCents != 2000 || c.TotalCents != 62000 || !c.HasShare || c.Share != 31 {
		t.Fatalf("unexpected %+v", c)
	}

	if CommitmentsPerMonth(subs, nil, nil, stts, now, 0).HasShare {
		t.Fatal("no income, no share")
	}
}

func TestVersusUsual(t *testing.T) {
	entry := func(cents int64, category string, m time.Month) cashflow.CashflowEntry {
		return cashflow.CashflowEntry{AmountCents: cents, RateToBase: 1, AmountBaseCents: cents, Category: category, EntryDate: time.Date(2026, m, 5, 0, 0, 0, 0, time.Local)}
	}

	entries := []cashflow.CashflowEntry{
		entry(10000, "Food", 1), entry(10000, "food", 2), entry(50000, "Rent", 1), entry(50000, "Rent", 2),
		entry(15000, "Food", 3), entry(50000, "Rent", 3), entry(5000, "Car", 3), entry(100, "Gift", 3),
		{IsIncome: true, AmountCents: 99999, RateToBase: 1, AmountBaseCents: 99999, Category: "Salary", EntryDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)},
	}

	c := VersusUsual(entries, time.Date(2026, 3, 20, 0, 0, 0, 0, time.Local))
	if c.UsualMonths != 2 || c.TotalCents != 70100 || c.UsualTotalCents != 60000 || len(c.Items) != 4 {
		t.Fatalf("unexpected %+v", c)
	}

	items := map[string]CategoryChange{}
	for _, item := range c.Items {
		items[item.Category] = item
	}

	if food := items["Food"]; food.UsualCents != 10000 || food.DiffCents != 5000 || food.DiffPercent != 50 || !food.Unusual || c.Items[0].Category != "Food" && c.Items[0].Category != "Car" {
		t.Fatalf("unexpected food %+v", c.Items)
	}

	if !items["Car"].Unusual || items["Rent"].Unusual || items["Gift"].Unusual {
		t.Fatalf("new car spending stands out, rent and a small gift don't: %+v", c.Items)
	}

	if first := VersusUsual(entries, time.Date(2026, 1, 20, 0, 0, 0, 0, time.Local)); first.UsualMonths != 0 || first.Items[0].HasUsual || first.Items[0].Unusual {
		t.Fatalf("nothing to compare the first month with: %+v", first)
	}
}
