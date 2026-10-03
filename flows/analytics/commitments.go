package analytics

import (
	"time"

	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/credit"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/subscription"
)

// creditMonths is how many full months credit payments are averaged over.
const creditMonths = 6

// Commitments is what's committed to be paid each month, in the base
// currency: active obligations (a year's worth over twelve) and credit
// payments, as the average of payments logged on unpaid credits over the
// last full months (only months since a credit started count for it).
// Share is the total as a part of the average income, in percent.
type Commitments struct {
	ObligationsCents    int64
	CreditPaymentsCents int64
	TotalCents          int64
	Share               float64
	HasShare            bool
}

func CommitmentsPerMonth(subs []subscription.Subscription, credits []credit.Credit, logs []credit.CreditLog, stts settings.AppSettings, now time.Time, averageIncome int64) Commitments {
	var obligations []subscription.Subscription
	for _, sub := range subs {
		if sub.IsActive && sub.IsObligation {
			obligations = append(obligations, sub)
		}
	}

	_, yearly := subscription.TotalsInBaseCents(obligations, stts)
	c := Commitments{ObligationsCents: yearly / 12}

	current := cashflow.MonthStart(now)
	from := current.AddDate(0, -creditMonths, 0)
	byCredit := map[uint][]credit.CreditLog{}
	for _, l := range logs {
		byCredit[l.CreditID] = append(byCredit[l.CreditID], l)
	}

	for _, cr := range credits {
		if cr.IsPaid() {
			continue
		}

		start := from
		if started := cashflow.MonthStart(cr.StartDate); started.After(start) {
			start = started
		}

		months := 0
		for m := start; m.Before(current); m = m.AddDate(0, 1, 0) {
			months++
		}

		var paid int64
		for _, l := range byCredit[cr.ID] {
			month := cashflow.MonthStart(l.CreatedAt)
			if l.Kind() == credit.LogPayment && l.DeltaPaidCents > 0 && !month.Before(start) && month.Before(current) {
				paid += l.DeltaPaidCents
			}
		}

		if months == 0 || paid == 0 {
			continue
		}

		base, ok := settings.BaseCents(paid, cr.RateToBase)
		if !ok {
			if base, ok = stts.ConvertToBaseCents(cr.Currency, paid); !ok {
				continue
			}
		}

		c.CreditPaymentsCents += base / int64(months)
	}

	c.TotalCents = c.ObligationsCents + c.CreditPaymentsCents
	if averageIncome > 0 {
		c.Share = float64(c.TotalCents) / float64(averageIncome) * 100
		c.HasShare = true
	}

	return c
}
