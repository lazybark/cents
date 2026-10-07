package subscription

import "sort"

// PaymentDateLayout is how PaymentDateYearly is stored.
const PaymentDateLayout = "02.01.2006"

// SortByAmount sorts by raw amount, largest first, then newest first.
func SortByAmount(values []Subscription) []Subscription {
	if len(values) < 2 {
		return values
	}

	cloned := make([]Subscription, len(values))
	copy(cloned, values)
	sort.SliceStable(cloned, func(i, j int) bool {
		if cloned[i].AmountCents == cloned[j].AmountCents {
			return cloned[i].CreatedAt.After(cloned[j].CreatedAt)
		}

		return cloned[i].AmountCents > cloned[j].AmountCents
	})

	return cloned
}

func SplitByActivity(subs []Subscription) (active []Subscription, inactive []Subscription) {
	active = make([]Subscription, 0, len(subs))
	inactive = make([]Subscription, 0, len(subs))

	for _, sub := range subs {
		if sub.IsActive {
			active = append(active, sub)

			continue
		}

		inactive = append(inactive, sub)
	}

	return active, inactive
}

func contains(options []string, value string) bool {
	for _, option := range options {
		if option == value {
			return true
		}
	}

	return false
}
