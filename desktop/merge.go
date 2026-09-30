package desktop

import (
	"errors"
	"fmt"
)

// MergeInput merges the record of Kind with FromID into the one with
// IntoID: everything linking to it moves over, and it's deleted. Kind is a
// setting kind (currency, payment_method, tax_type, income_category,
// expense_category) or "account". Merging currencies converts no amounts:
// it's for two names of one currency.
type MergeInput struct {
	Kind   string `json:"kind"`
	FromID uint   `json:"fromId"`
	IntoID uint   `json:"intoId"`
}

// MergeResult says how many records moved.
type MergeResult struct {
	Moved int64 `json:"moved"`
}

var mergeable = map[string]bool{settingTag: true, settingCurrency: true, settingPaymentMethod: true, settingTaxType: true, settingIncomeCategory: true, settingExpenseCategory: true, "account": true}

func (a *API) Merge(input MergeInput) (MergeResult, error) {
	if !mergeable[input.Kind] {
		return MergeResult{}, fmt.Errorf("%s can't be merged", input.Kind)
	}

	if input.FromID == 0 || input.IntoID == 0 {
		return MergeResult{}, errors.New("pick one to merge into")
	}

	storage, err := a.currentStorage()
	if err != nil {
		return MergeResult{}, err
	}

	moved, err := storage.Merge(input.Kind, input.FromID, input.IntoID)
	if err != nil {
		return MergeResult{}, fmt.Errorf("merge failed: %w", err)
	}

	return MergeResult{Moved: moved}, nil
}
