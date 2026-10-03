package desktop

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/subscription"
)

const (
	subscriptionsActive = "active"
	subscriptionsAll    = "all"

	// Kinds of regular payments, each listed apart.
	kindSubscription = "subscription"
	kindObligation   = "obligation"
)

var errSubscriptionNotFound = errors.New("subscription not found")

// SubscriptionRow is a subscription: any regular payment, from software to
// rent. NextPayment is the next payment day (YYYY-MM-DD, "" without a
// schedule), DueInDays how far off it is, and DueSoon whether that's
// within the period's warning window (see subscription.SoonDays).
type SubscriptionRow struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	Currency      string `json:"currency"`
	IsBase        bool   `json:"isBase"`
	AmountCents   int64  `json:"amountCents"`
	BaseCents     int64  `json:"baseCents"`
	HasRate       bool   `json:"hasRate"`
	Period        string `json:"period"`
	Type          string `json:"type"`
	PaymentMethod string `json:"paymentMethod"`
	IsActive      bool   `json:"isActive"`
	IsObligation  bool   `json:"isObligation"`
	PaidManually  bool   `json:"paidManually"`
	// Anchor is the payment date set for the schedule, as YYYY-MM-DD.
	Anchor      string `json:"anchor"`
	NextPayment string `json:"nextPayment"`
	DueInDays   int    `json:"dueInDays"`
	DueSoon     bool   `json:"dueSoon"`
}

type SubscriptionTotals struct {
	MonthlyCents int64 `json:"monthlyCents"`
	YearlyCents  int64 `json:"yearlyCents"`
}

type SubscriptionOptions struct {
	Currencies     []string `json:"currencies"`
	PaymentMethods []string `json:"paymentMethods"`
	// PaymentMethodCurrencies maps a payment method to the currency it pays
	// in, for the ones that have one.
	PaymentMethodCurrencies map[string]string `json:"paymentMethodCurrencies"`
	Periods                 []string          `json:"periods"`
	Types                   []string          `json:"types"`
}

type SubscriptionsView struct {
	BaseCurrency  string              `json:"baseCurrency"`
	Mode          string              `json:"mode"`
	Kind          string              `json:"kind"`
	Subscriptions []SubscriptionRow   `json:"subscriptions"`
	Active        SubscriptionTotals  `json:"active"`
	Inactive      SubscriptionTotals  `json:"inactive"`
	Options       SubscriptionOptions `json:"options"`
}

// SubscriptionInput is a subscription as typed into the form; ID is zero
// for a new one. Every field can be changed. NextPayment is YYYY-MM-DD, as
// <input type="date"> sends it, or empty for no schedule.
type SubscriptionInput struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Currency      string `json:"currency"`
	Amount        string `json:"amount"`
	Period        string `json:"period"`
	PaymentMethod string `json:"paymentMethod"`
	NextPayment   string `json:"nextPayment"`
	IsActive      bool   `json:"isActive"`
	// IsObligation lists it under obligations (rent, insurance) rather than
	// subscriptions.
	IsObligation bool `json:"isObligation"`
	// PaidManually keeps a passed payment due until it's marked paid.
	PaidManually bool `json:"paidManually"`
}

// SubscriptionPaymentRow is a payment marked paid: PaidFor is its day,
// MarkedAt when it was marked.
type SubscriptionPaymentRow struct {
	ID       uint   `json:"id"`
	PaidFor  string `json:"paidFor"`
	MarkedAt string `json:"markedAt"`
}

// Subscriptions lists one kind of regular payment, "subscription" (minor
// ones: streaming, apps) or "obligation" (rent, insurance, bills): active
// ones, or all of them when mode is "all", next payment first (ones
// without a schedule last, largest first). Totals cover active and
// inactive ones separately, in the base currency.
func (a *API) Subscriptions(mode string, kind string) (SubscriptionsView, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return SubscriptionsView{}, err
	}

	subs, err := storage.LoadSubscriptions()
	if err != nil {
		return SubscriptionsView{}, fmt.Errorf("failed to load subscriptions: %w", err)
	}

	if mode != subscriptionsAll {
		mode = subscriptionsActive
	}

	if kind != kindObligation {
		kind = kindSubscription
	}

	subscriptions, obligations := subscription.SplitByKind(subs)
	types := subscription.SubscriptionTypes()
	subs = subscriptions
	if kind == kindObligation {
		types = subscription.ObligationTypes()
		subs = obligations
	}

	active, inactive := subscription.SplitByActivity(subs)
	listed := subs
	if mode == subscriptionsActive {
		listed = active
	}

	for _, sub := range subs {
		if sub.Type != "" && !containsFold(types, sub.Type) {
			types = append(types, sub.Type)
		}
	}

	result := SubscriptionsView{
		BaseCurrency:  stts.BaseCurrencyLabel(),
		Mode:          mode,
		Kind:          kind,
		Subscriptions: make([]SubscriptionRow, 0, len(listed)),
		Options: SubscriptionOptions{
			Currencies:              stts.CurrencyOptions(),
			PaymentMethods:          stts.PaymentMethodOptions(),
			PaymentMethodCurrencies: paymentMethodCurrencies(stts),
			Periods:                 subscription.AllPeriods(),
			Types:                   types,
		},
	}

	result.Active.MonthlyCents, result.Active.YearlyCents = subscription.TotalsInBaseCents(active, stts)
	result.Inactive.MonthlyCents, result.Inactive.YearlyCents = subscription.TotalsInBaseCents(inactive, stts)

	today := time.Now()
	for _, sub := range subscription.SortByAmount(listed) {
		result.Subscriptions = append(result.Subscriptions, subscriptionRow(sub, stts, today))
	}

	sort.SliceStable(result.Subscriptions, func(i, j int) bool {
		left, right := result.Subscriptions[i], result.Subscriptions[j]
		if (left.NextPayment == "") != (right.NextPayment == "") {
			return left.NextPayment != ""
		}

		return left.NextPayment < right.NextPayment
	})

	return result, nil
}

func (a *API) CreateSubscription(input SubscriptionInput) error {
	return a.saveSubscription(input, nil)
}

// UpdateSubscription changes every field of a subscription. A new next
// payment date (or period) starts its schedule over.
func (a *API) UpdateSubscription(input SubscriptionInput) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	current, err := findSubscription(storage, input.ID)
	if err != nil {
		return err
	}

	return a.saveSubscription(input, &current)
}

// MarkSubscriptionPaid marks the next payment paid and records it, so the
// one after it shows as next. It returns the subscription as it is now.
func (a *API) MarkSubscriptionPaid(id uint) (SubscriptionRow, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return SubscriptionRow{}, err
	}

	current, err := findSubscription(storage, id)
	if err != nil {
		return SubscriptionRow{}, err
	}

	now := time.Now()
	paid, paidFor, err := current.MarkPaid(now, now)
	if err != nil {
		return SubscriptionRow{}, err
	}

	if err := storage.AddSubscriptionPayment(&paid, &subscription.SubscriptionPayment{SubscriptionID: paid.ID, PaidFor: paidFor}); err != nil {
		return SubscriptionRow{}, err
	}

	return subscriptionRow(paid, stts, now), nil
}

// SubscriptionPayments lists a subscription's payments marked paid, latest
// first.
func (a *API) SubscriptionPayments(id uint) ([]SubscriptionPaymentRow, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return nil, err
	}

	payments, err := storage.LoadSubscriptionPayments(id)
	if err != nil {
		return nil, err
	}

	rows := make([]SubscriptionPaymentRow, 0, len(payments))
	for _, p := range payments {
		rows = append(rows, SubscriptionPaymentRow{ID: p.ID, PaidFor: p.PaidFor.Format(logDateLayout), MarkedAt: p.CreatedAt.Local().Format("2006-01-02 15:04")})
	}

	return rows, nil
}

// DeleteSubscriptionPayment deletes a payment marked paid, by mistake say:
// the latest one left becomes the latest paid, so the schedule rolls back.
// It returns the subscription as it is now.
func (a *API) DeleteSubscriptionPayment(subscriptionID uint, paymentID uint) (SubscriptionRow, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return SubscriptionRow{}, err
	}

	current, err := findSubscription(storage, subscriptionID)
	if err != nil {
		return SubscriptionRow{}, err
	}

	if err := storage.DeleteSubscriptionPayment(&current, paymentID); err != nil {
		return SubscriptionRow{}, err
	}

	return subscriptionRow(current, stts, time.Now()), nil
}

func (a *API) saveSubscription(input SubscriptionInput, current *subscription.Subscription) error {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return err
	}

	// An edited subscription may keep a currency no longer in settings.
	currencies := stts.CurrencyOptions()
	record := subscription.Subscription{}
	if current != nil {
		record = *current
		currencies = append(currencies, current.Currency)
	}

	currency := strings.TrimSpace(input.Currency)
	if currency != "" {
		var ok bool
		if currency, ok = matchOption(currencies, currency); !ok {
			return fmt.Errorf("unknown currency %q: add it in settings first", input.Currency)
		}
	}

	updated, err := record.Update(subscription.Fields{
		Name:          input.Name,
		Type:          input.Type,
		Currency:      currency,
		Amount:        input.Amount,
		Period:        input.Period,
		PaymentMethod: input.PaymentMethod,
		NextPayment:   input.NextPayment,
		IsActive:      input.IsActive,
		IsObligation:  input.IsObligation,
		PaidManually:  input.PaidManually,
	}, dates.ISO, time.Now())
	if err != nil {
		return err
	}

	if current == nil {
		if err := storage.CreateSubscription(&updated); err != nil {
			return fmt.Errorf("save failed: %w", err)
		}

		return nil
	}

	if err := storage.SaveSubscription(&updated); err != nil {
		return fmt.Errorf("save failed: %w", err)
	}

	return nil
}

func subscriptionRow(sub subscription.Subscription, stts settings.AppSettings, today time.Time) SubscriptionRow {
	baseCents, ok := stts.ConvertToBaseCents(sub.Currency, sub.AmountCents)
	row := SubscriptionRow{
		ID:            sub.ID,
		Name:          sub.Name,
		Currency:      sub.Currency,
		IsBase:        stts.IsBase(sub.Currency),
		AmountCents:   sub.AmountCents,
		BaseCents:     baseCents,
		HasRate:       ok,
		Period:        sub.Period,
		Type:          sub.Type,
		PaymentMethod: sub.PaymentMethod,
		IsActive:      sub.IsActive,
		IsObligation:  sub.IsObligation,
		PaidManually:  sub.PaidManually,
	}

	if anchor, ok := sub.Anchor(today); ok {
		row.Anchor = anchor.Format(logDateLayout)
	}

	if next, days, soon, ok := sub.DueIn(today); ok {
		row.NextPayment = next.Format(logDateLayout)
		row.DueInDays = days
		row.DueSoon = soon
	}

	return row
}

func paymentMethodCurrencies(stts settings.AppSettings) map[string]string {
	result := map[string]string{}
	for _, method := range stts.PaymentMethods {
		if method.Currency != "" {
			result[method.PaymentMethodName] = method.Currency
		}
	}

	return result
}

func containsFold(values []string, value string) bool {
	for _, v := range values {
		if strings.EqualFold(strings.TrimSpace(v), strings.TrimSpace(value)) {
			return true
		}
	}

	return false
}

func (a *API) DeleteSubscription(id uint) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	if _, err := findSubscription(storage, id); err != nil {
		return err
	}

	if err := storage.DeleteSubscription(id); err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	return nil
}

func findSubscription(storage StorageWorker, id uint) (subscription.Subscription, error) {
	subs, err := storage.LoadSubscriptions()
	if err != nil {
		return subscription.Subscription{}, fmt.Errorf("failed to load subscriptions: %w", err)
	}

	for _, sub := range subs {
		if sub.ID == id {
			return sub, nil
		}
	}

	return subscription.Subscription{}, errSubscriptionNotFound
}
