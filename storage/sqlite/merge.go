package sqlite

import (
	"errors"
	"fmt"

	"github.com/lazybark/cents/flows/account"
	"gorm.io/gorm"
)

// InUseError is returned for deleting something records still link to: it
// has to be archived instead, or merged into another one.
type InUseError struct {
	Count int64
}

func (e InUseError) Error() string {
	if e.Count == 1 {
		return "it's used by 1 record; archive it, or merge it into another one, instead"
	}

	return fmt.Sprintf("it's used by %d records; archive it, or merge it into another one, instead", e.Count)
}

// reference is a column that links to rows of a kind.
type reference struct{ table, column string }

// linkedKinds are the settings records link to: their table and what links
// to them.
var linkedKinds = map[string]struct {
	table string
	refs  []reference
}{
	"income_category":  {"setting_income_categories", []reference{{"cashflow_entries", "category_uid"}}},
	"expense_category": {"setting_expense_categories", []reference{{"cashflow_entries", "category_uid"}, {"budgets", "category_uid"}}},
	"payment_method":   {"setting_payment_methods", []reference{{"subscriptions", "payment_method_uid"}}},
	"tax_type":         {"setting_tax_types", []reference{{"taxes", "tax_type_uid"}}},
	"account":          {"accounts", []reference{{"cashflow_entries", "account_uid"}}},
	"currency":         {"setting_currencies", currencyRefs},
}

// isBaseCurrency reports whether the currency with id is the base one.
func isBaseCurrency(tx *gorm.DB, id uint) (bool, error) {
	var n int64
	err := tx.Table("setting_currencies").Where("id = ? AND is_base = ?", id, true).Count(&n).Error
	return n > 0, err
}

func uidOf(tx *gorm.DB, table string, id uint) (string, error) {
	var uids []string
	if err := tx.Table(table).Where("id = ?", id).Pluck("uid", &uids).Error; err != nil {
		return "", err
	}

	if len(uids) == 0 {
		return "", fmt.Errorf("%d not found", id)
	}

	return uids[0], nil
}

// usage counts the records linking to uid.
func usage(tx *gorm.DB, refs []reference, uid string) (int64, error) {
	var total int64
	for _, r := range refs {
		var n int64
		if err := tx.Table(r.table).Where(r.column+" = ?", uid).Count(&n).Error; err != nil {
			return 0, err
		}

		total += n
	}

	return total, nil
}

// checkUnused fails with InUseError when records link to the row of kind
// with id.
func checkUnused(tx *gorm.DB, kind string, id uint) error {
	k, ok := linkedKinds[kind]
	if !ok {
		return nil
	}

	uid, err := uidOf(tx, k.table, id)
	if err != nil || uid == "" {
		return err
	}

	n, err := usage(tx, k.refs, uid)
	if err != nil {
		return err
	}

	if n > 0 {
		return InUseError{Count: n}
	}

	return nil
}

// Merge moves every record linking to the row of kind with id from over to
// the one with id into, then deletes from; it reports how many records
// moved. An expense category's budget goes with it unless into has one,
// and an account's value history goes with the account.
func (s *SQLiteStorage) Merge(kind string, from, into uint) (int64, error) {
	k, ok := linkedKinds[kind]
	if !ok && kind != "tag" {
		return 0, fmt.Errorf("can't merge %s", kind)
	}

	if from == into {
		return 0, errors.New("pick another one to merge into")
	}

	if kind == "tag" {
		var moved int64
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var err error
			moved, err = mergeTags(tx, from, into)
			return err
		})

		return moved, err
	}

	var moved int64
	err := s.db.Transaction(func(tx *gorm.DB) error {
		fromUID, err := uidOf(tx, k.table, from)
		if err != nil {
			return err
		}

		if kind == "currency" {
			if base, err := isBaseCurrency(tx, from); err != nil || base {
				return errors.Join(err, errBaseCurrency)
			}

			// Its rates go with it, as a currency or as a base.
			if err := tx.Table("rate_records").Where("currency_uid = ? OR base_uid = ?", fromUID, fromUID).Delete(nil).Error; err != nil {
				return fmt.Errorf("failed to delete its rate history: %w", err)
			}
		}

		intoUID, err := uidOf(tx, k.table, into)
		if err != nil {
			return err
		}

		if kind == "expense_category" {
			var both int64
			if err := tx.Table("budgets").Where("category_uid = ?", intoUID).Count(&both).Error; err != nil {
				return err
			}

			if both > 0 {
				if err := tx.Table("budgets").Where("category_uid = ?", fromUID).Delete(nil).Error; err != nil {
					return fmt.Errorf("failed to drop the merged category's budget: %w", err)
				}
			}
		}

		for _, r := range k.refs {
			result := tx.Table(r.table).Where(r.column+" = ?", fromUID).Update(r.column, intoUID)
			if result.Error != nil {
				return fmt.Errorf("failed to move %s: %w", r.table, result.Error)
			}

			moved += result.RowsAffected
		}

		if kind == "account" {
			if err := tx.Where("account_id = ?", from).Delete(&account.AccountValueLog{}).Error; err != nil {
				return fmt.Errorf("failed to delete account value logs: %w", err)
			}
		}

		if err := tx.Table(k.table).Where("id = ?", from).Delete(nil).Error; err != nil {
			return fmt.Errorf("failed to delete the merged one: %w", err)
		}

		if kind == "payment_method" {
			return EnsurePaymentMethodDefaults(tx)
		}

		return nil
	})

	return moved, err
}
