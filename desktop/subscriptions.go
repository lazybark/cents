package desktop

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lazybark/cents/flows/subscription"
)

const (
	subscriptionsActive = "active"
	subscriptionsAll    = "all"
)

var errSubscriptionNotFound = errors.New("subscription not found")

type SubscriptionRow struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	Currency      string `json:"currency"`
	AmountCents   int64  `json:"amountCents"`
	BaseCents     int64  `json:"baseCents"`
	HasRate       bool   `json:"hasRate"`
	Period        string `json:"period"`
	Type          string `json:"type"`
	PaymentMethod string `json:"paymentMethod"`
	IsActive      bool   `json:"isActive"`
	// PaymentDate is the yearly payment date as YYYY-MM-DD, or "".
	PaymentDate string `json:"paymentDate"`
	// PaymentDay is the monthly payment day, or "".
	PaymentDay string `json:"paymentDay"`
}

type SubscriptionTotals struct {
	MonthlyCents int64 `json:"monthlyCents"`
	YearlyCents  int64 `json:"yearlyCents"`
}

type SubscriptionOptions struct {
	Currencies     []string `json:"currencies"`
	PaymentMethods []string `json:"paymentMethods"`
	Periods        []string `json:"periods"`
	Types          []string `json:"types"`
}

type SubscriptionsView struct {
	BaseCurrency  string              `json:"baseCurrency"`
	Mode          string              `json:"mode"`
	Subscriptions []SubscriptionRow   `json:"subscriptions"`
	Active        SubscriptionTotals  `json:"active"`
	Inactive      SubscriptionTotals  `json:"inactive"`
	Options       SubscriptionOptions `json:"options"`
}

type NewSubscriptionInput struct {
	Name          string `json:"name"`
	Currency      string `json:"currency"`
	Amount        string `json:"amount"`
	Period        string `json:"period"`
	PaymentMethod string `json:"paymentMethod"`
	Type          string `json:"type"`
	IsActive      bool   `json:"isActive"`
	// PaymentDate is YYYY-MM-DD, as <input type="date"> sends it.
	PaymentDate string `json:"paymentDate"`
	PaymentDay  string `json:"paymentDay"`
}

type SubscriptionUpdateInput struct {
	ID            uint   `json:"id"`
	Amount        string `json:"amount"`
	PaymentMethod string `json:"paymentMethod"`
	IsActive      bool   `json:"isActive"`
}

// Subscriptions lists active subscriptions, or all of them when mode is
// "all", largest amount first. Totals cover active and inactive ones
// separately, in the base currency.
func (a *API) Subscriptions(mode string) (SubscriptionsView, error) {
	storage, err := a.currentStorage()
	if err != nil {
		return SubscriptionsView{}, err
	}

	stts, err := storage.LoadAppSettings()
	if err != nil {
		return SubscriptionsView{}, fmt.Errorf("failed to load settings: %w", err)
	}

	subs, err := storage.LoadSubscriptions()
	if err != nil {
		return SubscriptionsView{}, fmt.Errorf("failed to load subscriptions: %w", err)
	}

	if mode != subscriptionsAll {
		mode = subscriptionsActive
	}

	subs = subscription.SortByAmount(subs)
	active, inactive := subscription.SplitByActivity(subs)
	listed := subs
	if mode == subscriptionsActive {
		listed = active
	}

	result := SubscriptionsView{
		BaseCurrency:  stts.BaseCurrencyLabel(),
		Mode:          mode,
		Subscriptions: make([]SubscriptionRow, 0, len(listed)),
		Options: SubscriptionOptions{
			Currencies:     stts.CurrencyOptions(),
			PaymentMethods: stts.PaymentMethodOptions(),
			Periods:        subscription.PeriodOptions(),
			Types:          subscription.TypeOptions(),
		},
	}

	result.Active.MonthlyCents, result.Active.YearlyCents = subscription.TotalsInBaseCents(active, stts)
	result.Inactive.MonthlyCents, result.Inactive.YearlyCents = subscription.TotalsInBaseCents(inactive, stts)

	for _, sub := range listed {
		baseCents, ok := stts.ConvertToBaseCents(sub.Currency, sub.AmountCents)
		row := SubscriptionRow{
			ID:            sub.ID,
			Name:          sub.Name,
			Currency:      sub.Currency,
			AmountCents:   sub.AmountCents,
			BaseCents:     baseCents,
			HasRate:       ok,
			Period:        sub.Period,
			Type:          sub.Type,
			PaymentMethod: sub.PaymentMethod,
			IsActive:      sub.IsActive,
			PaymentDate:   sub.PaymentDateYearly,
		}

		if day, err := time.Parse(subscription.PaymentDateLayout, sub.PaymentDateYearly); err == nil {
			row.PaymentDate = day.Format(logDateLayout)
		}

		if sub.PaymentDayMonthly != nil {
			row.PaymentDay = strconv.Itoa(*sub.PaymentDayMonthly)
		}

		result.Subscriptions = append(result.Subscriptions, row)
	}

	return result, nil
}

func (a *API) CreateSubscription(input NewSubscriptionInput) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	stts, err := storage.LoadAppSettings()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	currency := strings.TrimSpace(input.Currency)
	if currency != "" {
		var ok bool
		if currency, ok = matchOption(stts.CurrencyOptions(), currency); !ok {
			return fmt.Errorf("unknown currency %q: add it in settings first", input.Currency)
		}
	}

	// Stored the way the TUI stores it.
	paymentDate := strings.TrimSpace(input.PaymentDate)
	if paymentDate != "" {
		day, err := time.Parse(logDateLayout, paymentDate)
		if err != nil {
			return errors.New("yearly date must use YYYY-MM-DD format")
		}

		paymentDate = day.Format(subscription.PaymentDateLayout)
	}

	entry, err := subscription.New(input.Name, currency, input.Amount, input.Period, input.PaymentMethod, input.Type, input.IsActive, paymentDate, input.PaymentDay, time.Now())
	if err != nil {
		return err
	}

	if err := storage.CreateSubscription(&entry); err != nil {
		return fmt.Errorf("save failed: %w", err)
	}

	return nil
}

// UpdateSubscription changes what the TUI lets you edit: amount, payment
// method and whether the subscription is active.
func (a *API) UpdateSubscription(input SubscriptionUpdateInput) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	current, err := findSubscription(storage, input.ID)
	if err != nil {
		return err
	}

	updated, err := current.Edit(input.Amount, input.PaymentMethod, input.IsActive, time.Now())
	if err != nil {
		return err
	}

	if err := storage.SaveSubscription(&updated); err != nil {
		return fmt.Errorf("save failed: %w", err)
	}

	return nil
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
