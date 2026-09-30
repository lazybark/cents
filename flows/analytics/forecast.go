package analytics

import (
	"github.com/lazybark/cents/dates"
	"sort"
	"time"

	"github.com/lazybark/cents/summary"
)

// Kinds of things coming due.
const (
	DueSubscription = "subscription"
	DueObligation   = "obligation"
	DueDebt         = "debt"
	DueCredit       = "credit"
	DueTax          = "tax"
	DueInvoice      = "invoice"
)

// Due is something known to come due: money out, or in (Incoming) for
// debts and invoices owed to the user. Cents is in its currency, BaseCents
// in the base currency (HasRate is false when it can't be converted).
type Due struct {
	Date      time.Time
	Kind      string
	Name      string
	Incoming  bool
	Currency  string
	Cents     int64
	BaseCents int64
	HasRate   bool
	Overdue   bool
}

// Forecast lists everything due from now to days ahead, soonest first:
// every payment of active subscriptions and obligations, and unpaid debts,
// credits, taxes and invoices with a due date by then. Ones already past
// due (overdue) are included.
func Forecast(data summary.Data, now time.Time, days int) []Due {
	stts := data.Settings
	today := dates.Day(now)
	until := today.AddDate(0, 0, days)
	items := []Due{}

	add := func(d Due) {
		d.Overdue = d.Date.Before(today)
		items = append(items, d)
	}

	for _, sub := range data.Subscriptions {
		if !sub.IsActive {
			continue
		}

		kind := DueSubscription
		if sub.IsObligation {
			kind = DueObligation
		}

		base, ok := stts.ConvertToBaseCents(sub.Currency, sub.AmountCents)
		for _, at := range sub.PaymentsUntil(now, until) {
			add(Due{Date: at, Kind: kind, Name: sub.Name, Currency: sub.Currency, Cents: sub.AmountCents, BaseCents: base, HasRate: ok})
		}
	}

	dueBy := func(due *time.Time) bool { return due != nil && !dayOf(*due).After(until) }

	for _, d := range data.Debts {
		if d.IsPaid() || !dueBy(d.DueDate) {
			continue
		}

		base, ok := d.LeftBaseCents()
		add(Due{Date: dayOf(*d.DueDate), Kind: DueDebt, Name: d.Peer, Incoming: d.IsOwedToUser, Currency: d.Currency, Cents: d.LeftCents(), BaseCents: base, HasRate: ok})
	}

	for _, c := range data.Credits {
		if c.IsPaid() || !dueBy(c.DueDate) {
			continue
		}

		base, ok := c.LeftBaseCents()
		add(Due{Date: dayOf(*c.DueDate), Kind: DueCredit, Name: c.Name, Currency: c.Currency, Cents: c.LeftCents(), BaseCents: base, HasRate: ok})
	}

	for _, t := range data.Taxes {
		if t.IsPaid() || !dueBy(t.DueDate) {
			continue
		}

		add(Due{Date: dayOf(*t.DueDate), Kind: DueTax, Name: t.TaxCountry + " / " + t.TaxTypeName + " " + t.Period, Currency: t.Currency, Cents: t.LeftCents(), BaseCents: t.LeftBaseCents(), HasRate: true})
	}

	for _, inv := range data.Invoices {
		if inv.Paid || !dueBy(inv.DueDate) {
			continue
		}

		base, ok := inv.BaseCents()
		add(Due{Date: dayOf(*inv.DueDate), Kind: DueInvoice, Name: inv.Title, Incoming: !inv.IsIncoming, Currency: inv.Currency, Cents: inv.AmountCents, BaseCents: base, HasRate: ok})
	}

	sort.SliceStable(items, func(i, j int) bool { return items[i].Date.Before(items[j].Date) })

	return items
}

// dayOf is value's calendar day as a UTC midnight, like stored dates.
func dayOf(value time.Time) time.Time {
	return dates.Day(value)
}
