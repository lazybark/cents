package app

import (
	"sort"

	"github.com/lazybark/cents/flows/subscription"
)

func splitSubscriptionsByActivity(subs []subscription.Subscription) (active []subscription.Subscription, inactive []subscription.Subscription) {
	active = make([]subscription.Subscription, 0, len(subs))
	inactive = make([]subscription.Subscription, 0, len(subs))

	for _, sub := range subs {
		if sub.IsActive {
			active = append(active, sub)

			continue
		}

		inactive = append(inactive, sub)
	}

	return active, inactive
}

func sortSubscriptionsByAmount(values []subscription.Subscription) []subscription.Subscription {
	if len(values) < 2 {
		return values
	}

	cloned := make([]subscription.Subscription, len(values))

	copy(cloned, values)

	sort.SliceStable(cloned, func(i, j int) bool {
		if cloned[i].AmountCents == cloned[j].AmountCents {
			return cloned[i].CreatedAt.After(cloned[j].CreatedAt)
		}

		return cloned[i].AmountCents > cloned[j].AmountCents
	})

	return cloned
}
