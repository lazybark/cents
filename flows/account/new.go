package account

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lazybark/cents/money"
)

// New validates the fields of a new account and builds it. Every interface
// creates accounts through here so they enforce the same rules.
func New(name, description, currency, amount string, ignoreInSummaries bool, now time.Time) (Account, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	currency = strings.TrimSpace(currency)

	if name == "" {
		return Account{}, errors.New("name is required")
	}

	if description == "" {
		return Account{}, errors.New("description is required")
	}

	if currency == "" {
		return Account{}, errors.New("currency is required")
	}

	amountCents, err := money.ParseAmountCents(amount)
	if err != nil {
		return Account{}, fmt.Errorf("amount error: %w", err)
	}

	return Account{
		Name:              name,
		Description:       description,
		Currency:          currency,
		BalanceCents:      amountCents,
		LeftoverCents:     amountCents,
		IgnoreInSummaries: ignoreInSummaries,
		LastUpdatedAt:     now,
	}, nil
}
