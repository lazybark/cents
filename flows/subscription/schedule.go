package subscription

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/money"
)

// Periods a subscription (any regular payment: software, rent, insurance…)
// can repeat at.
const (
	PeriodWeek    = "week"
	PeriodMonth   = "month"
	PeriodQuarter = "quarter"
	PeriodYear    = "year"
)

// AllPeriods lists every period, shortest first. (PeriodOptions are the
// ones the TUI form offers.)
func AllPeriods() []string {
	return []string{PeriodWeek, PeriodMonth, PeriodQuarter, PeriodYear}
}

// SubscriptionTypes are common kinds of minor subscriptions, offered when
// picking a type; any other can be typed in. The TUI's TypeOptions are
// among them.
func SubscriptionTypes() []string {
	return []string{"Software", "Multimedia", "Domain", "Service", "Membership", "Fitness & sports", "Education", "Donation", "Other"}
}

// ObligationTypes are common kinds of serious regular payments.
func ObligationTypes() []string {
	return []string{"Rent", "Utilities", "Internet & phone", "Insurance", "Transport", "Healthcare", "Childcare", "Education", "Loan payment", "Other"}
}

// ObligationTypesByDefault are the types that make a payment an
// obligation when sorting older ones out.
func ObligationTypesByDefault() []string {
	return []string{"Rent", "Utilities", "Internet & phone", "Insurance", "Transport", "Healthcare", "Childcare", "Loan payment"}
}

// SplitByKind separates minor subscriptions from obligations.
func SplitByKind(subs []Subscription) (subscriptions []Subscription, obligations []Subscription) {
	for _, sub := range subs {
		if sub.IsObligation {
			obligations = append(obligations, sub)
		} else {
			subscriptions = append(subscriptions, sub)
		}
	}

	return subscriptions, obligations
}

// Fields are a subscription's values as typed into a form; NextPayment is a
// date in format, or empty for no schedule.
type Fields struct {
	Name          string
	Type          string
	Currency      string
	Amount        string
	Period        string
	PaymentMethod string
	NextPayment   string
	IsActive      bool
	IsObligation  bool
	PaidManually  bool
}

// Update replaces every field of the subscription with f (a zero value
// makes a new one). The old yearly date and monthly day are kept in step
// with the next payment, for the TUI.
func (s Subscription) Update(f Fields, format dates.Format, now time.Time) (Subscription, error) {
	name := strings.TrimSpace(f.Name)
	if name == "" {
		return Subscription{}, errors.New("subscription name is required")
	}

	subType := strings.TrimSpace(f.Type)
	if subType == "" {
		return Subscription{}, errors.New("type is required")
	}

	currency := strings.TrimSpace(f.Currency)
	if currency == "" {
		return Subscription{}, errors.New("currency is required")
	}

	paymentMethod := strings.TrimSpace(f.PaymentMethod)
	if paymentMethod == "" {
		return Subscription{}, errors.New("payment method is required")
	}

	amount, err := money.ParseAmountCents(f.Amount)
	if err != nil {
		return Subscription{}, fmt.Errorf("amount error: %w", err)
	}

	period := strings.ToLower(strings.TrimSpace(f.Period))
	if !contains(AllPeriods(), period) {
		return Subscription{}, fmt.Errorf("period must be one of %s", strings.Join(AllPeriods(), ", "))
	}

	next, err := format.Optional(f.NextPayment, "next payment")
	if err != nil {
		return Subscription{}, err
	}

	s.Name = name
	s.Type = subType
	s.Currency = currency
	s.AmountCents = amount
	s.PaymentMethod = paymentMethod
	s.IsActive = f.IsActive
	s.IsObligation = f.IsObligation
	s.PaidManually = f.PaidManually
	s.Period = period
	s.NextPaymentDate = next
	s.LastUpdatedAt = now
	s.syncTUIDays()

	return s, nil
}

// syncTUIDays sets the yearly date and monthly day the TUI shows from the
// next payment.
func (s *Subscription) syncTUIDays() {
	s.PaymentDateYearly = ""
	s.PaymentDayMonthly = nil
	if s.NextPaymentDate == nil {
		return
	}

	switch s.Period {
	case PeriodYear:
		s.PaymentDateYearly = s.NextPaymentDate.Format(PaymentDateLayout)
	case PeriodMonth:
		day := s.NextPaymentDate.Day()
		s.PaymentDayMonthly = &day
	}
}

// Anchor is a payment date the schedule repeats from: the next payment
// date, or else the TUI's yearly date or monthly day (that day this month).
// It reports false when the subscription has no schedule.
func (s Subscription) Anchor(today time.Time) (time.Time, bool) {
	if s.NextPaymentDate != nil {
		return day(*s.NextPaymentDate), true
	}

	if s.PaymentDateYearly != "" {
		if parsed, err := time.Parse(PaymentDateLayout, s.PaymentDateYearly); err == nil {
			return parsed, true
		}
	}

	if s.PaymentDayMonthly != nil {
		today = day(today)
		return time.Date(today.Year(), today.Month(), clampDay(today.Year(), today.Month(), *s.PaymentDayMonthly), 0, 0, 0, 0, time.UTC), true
	}

	return time.Time{}, false
}

// NextPayment is the next payment not marked paid. A payment made by hand
// (PaidManually) stays due once its day has passed, so it can be in the
// past (overdue). Otherwise a passed payment counts as paid and the next one
// is on or after today. Either way it comes after the latest payment marked
// paid. Payments repeat every period from the date set, so one on the 31st
// lands on the last day of shorter months and comes back after.
func (s Subscription) NextPayment(today time.Time) (time.Time, bool) {
	anchor, ok := s.Anchor(today)
	if !ok || !contains(AllPeriods(), s.Period) {
		return time.Time{}, false
	}

	var from time.Time
	if s.LastPaidDate != nil {
		from = day(*s.LastPaidDate).AddDate(0, 0, 1)
	}

	if !s.PaidManually && from.Before(day(today)) {
		from = day(today)
	}

	n := 0
	for occurrence(anchor, s.Period, n).Before(from) {
		n++
	}

	return occurrence(anchor, s.Period, n), true
}

// PaymentsUntil lists the payments from the next one (which may be
// overdue) up to and including until.
func (s Subscription) PaymentsUntil(today time.Time, until time.Time) []time.Time {
	next, ok := s.NextPayment(today)
	if !ok {
		return nil
	}

	anchor, _ := s.Anchor(today)
	n := 0
	for occurrence(anchor, s.Period, n).Before(next) {
		n++
	}

	payments := []time.Time{}
	for at := occurrence(anchor, s.Period, n); !at.After(day(until)); at = occurrence(anchor, s.Period, n) {
		payments = append(payments, at)
		n++
	}

	return payments
}

// MarkPaid marks the next payment paid, returning the subscription with it
// as the latest paid and the day it was for, to record.
func (s Subscription) MarkPaid(today time.Time, now time.Time) (Subscription, time.Time, error) {
	next, ok := s.NextPayment(today)
	if !ok {
		return Subscription{}, time.Time{}, errors.New("this subscription has no next payment date")
	}

	s.LastPaidDate = &next
	s.LastUpdatedAt = now

	return s, next, nil
}

// PaidUpTo lists the payments from the date set up to and including the
// latest paid, newest last, at most limit of them: what a LastPaidDate set
// before payments were recorded stands for.
func (s Subscription) PaidUpTo(today time.Time, limit int) []time.Time {
	anchor, ok := s.Anchor(today)
	if !ok || s.LastPaidDate == nil || !contains(AllPeriods(), s.Period) {
		return nil
	}

	last := day(*s.LastPaidDate)
	paid := []time.Time{}
	for n := 0; !occurrence(anchor, s.Period, n).After(last); n++ {
		paid = append(paid, occurrence(anchor, s.Period, n))
	}

	if len(paid) > limit {
		paid = paid[len(paid)-limit:]
	}

	return paid
}

// SoonDays is how many days before a payment it counts as coming up: two
// for weekly ones, five for monthly, two weeks for quarterly and a month
// for yearly ones.
func SoonDays(period string) int {
	switch period {
	case PeriodWeek:
		return 2
	case PeriodMonth:
		return 5
	case PeriodQuarter:
		return 14
	default:
		return 30
	}
}

// DueIn is how many days until the next payment (negative once it's
// overdue) and whether that's soon (overdue counts as soon).
func (s Subscription) DueIn(today time.Time) (next time.Time, days int, soon bool, ok bool) {
	next, ok = s.NextPayment(today)
	if !ok {
		return time.Time{}, 0, false, false
	}

	days = int(next.Sub(day(today)).Hours() / 24)

	return next, days, s.IsActive && days <= SoonDays(s.Period), true
}

// Upcoming lists the active subscriptions paid soon, soonest first.
func Upcoming(subs []Subscription, today time.Time) []Subscription {
	type due struct {
		sub  Subscription
		next time.Time
	}

	list := []due{}
	for _, sub := range subs {
		if next, _, soon, ok := sub.DueIn(today); ok && soon {
			list = append(list, due{sub, next})
		}
	}

	sort.SliceStable(list, func(i, j int) bool { return list[i].next.Before(list[j].next) })

	result := make([]Subscription, 0, len(list))
	for _, d := range list {
		result = append(result, d.sub)
	}

	return result
}

// occurrence is the anchor moved by n periods. Months count from the
// anchor itself, so the 31st lands on the last day of shorter months and
// comes back to the 31st after them.
func occurrence(anchor time.Time, period string, n int) time.Time {
	months := 0
	switch period {
	case PeriodWeek:
		return anchor.AddDate(0, 0, 7*n)
	case PeriodMonth:
		months = n
	case PeriodQuarter:
		months = 3 * n
	case PeriodYear:
		months = 12 * n
	}

	total := int(anchor.Month()) - 1 + months
	year := anchor.Year() + floorDiv(total, 12)
	month := time.Month(total - floorDiv(total, 12)*12 + 1)

	return time.Date(year, month, clampDay(year, month, anchor.Day()), 0, 0, 0, 0, time.UTC)
}

func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}

	return q
}

func clampDay(year int, month time.Month, wanted int) int {
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	return min(max(wanted, 1), last)
}

// day is the calendar day of value, as a UTC midnight like stored dates.
func day(value time.Time) time.Time {
	if value.Location() != time.UTC {
		local := value.Local()
		return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	}

	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

// PerMonthAndYear is the subscription's amount per month (counted only
// for ones paid monthly or weekly) and per year (every period).
func (s Subscription) PerMonthAndYear(cents int64) (perMonth int64, perYear int64) {
	switch s.Period {
	case PeriodWeek:
		return cents * 52 / 12, cents * 52
	case PeriodMonth:
		return cents, cents * 12
	case PeriodQuarter:
		return 0, cents * 4
	case PeriodYear:
		return 0, cents
	}

	return 0, 0
}
