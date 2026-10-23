package sqlite

import (
	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/analytics"
	"github.com/lazybark/cents/flows/asset"
	"github.com/lazybark/cents/flows/budget"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/credit"
	"github.com/lazybark/cents/flows/debt"
	"github.com/lazybark/cents/flows/goal"
	"github.com/lazybark/cents/flows/invoice"
	"github.com/lazybark/cents/flows/note"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/subscription"
	"github.com/lazybark/cents/flows/tag"
	"github.com/lazybark/cents/flows/tax"
)

type importRef struct {
	column, table string
	uid           bool
}
type importTable struct {
	model   any
	name    string
	columns []string
	natural []string
	refs    []importRef
	uid     bool
}

// This explicit model list is the schema allowlist and dependency order.
// Uploaded SQL, schema objects and arbitrary tables are never executed/copied.
func importTables() []importTable {
	currency := importRef{"currency_uid", "setting_currencies", true}
	return []importTable{
		{model: &settings.SettingCurrency{}, natural: []string{"currency_name"}, uid: true},
		{model: &settings.SettingIncomeCategory{}, natural: []string{"category_name"}, uid: true},
		{model: &settings.SettingExpenseCategory{}, natural: []string{"category_name"}, uid: true},
		{model: &settings.SettingPaymentMethod{}, natural: []string{"payment_method_name"}, uid: true, refs: []importRef{currency}},
		{model: &settings.SettingTaxType{}, natural: []string{"country", "tax_type_name"}, uid: true},
		{model: &tag.Tag{}, natural: []string{"name"}, uid: true},
		{model: &account.Account{}, uid: true, refs: []importRef{currency}},
		{model: &cashflow.CashflowEntry{}, refs: []importRef{currency, {"account_uid", "accounts", true}}},
		{model: &asset.Asset{}, refs: []importRef{currency}},
		{model: &debt.Debt{}, refs: []importRef{currency}},
		{model: &credit.Credit{}, refs: []importRef{currency}},
		{model: &goal.Goal{}, refs: []importRef{currency}},
		{model: &tax.Tax{}, refs: []importRef{currency, {"tax_type_uid", "setting_tax_types", true}}},
		{model: &subscription.Subscription{}, refs: []importRef{currency, {"payment_method_uid", "setting_payment_methods", true}}},
		{model: &invoice.Invoice{}, refs: []importRef{currency, {"cashflow_entry_id", "cashflow_entries", false}}},
		{model: &budget.Budget{}, natural: []string{"category_uid"}, refs: []importRef{{"category_uid", "setting_expense_categories", true}}},
		{model: &account.AccountValueLog{}, natural: []string{"account_id", "log_date"}, refs: []importRef{{"account_id", "accounts", false}}},
		{model: &asset.AssetValueLog{}, natural: []string{"asset_id", "log_date"}, refs: []importRef{{"asset_id", "assets", false}}},
		{model: &debt.DebtLog{}, refs: []importRef{{"debt_id", "debts", false}, {"cashflow_entry_id", "cashflow_entries", false}}},
		{model: &credit.CreditLog{}, refs: []importRef{{"credit_id", "credits", false}, {"cashflow_entry_id", "cashflow_entries", false}}},
		{model: &goal.GoalLog{}, refs: []importRef{{"goal_id", "goals", false}}},
		{model: &tax.TaxLog{}, refs: []importRef{{"tax_id", "taxes", false}}},
		{model: &subscription.SubscriptionPayment{}, refs: []importRef{{"subscription_id", "subscriptions", false}}},
		{model: &analytics.NetWorthSnapshot{}, natural: []string{"month"}},
		{model: &note.MonthNote{}, natural: []string{"month"}},
		{model: &settings.RateRecord{}, natural: []string{"currency_uid", "base_uid", "day"}, refs: []importRef{currency, {"base_uid", "setting_currencies", true}}},
		{model: &tag.EntryTag{}, natural: []string{"entry_id", "tag_uid"}, refs: []importRef{{"entry_id", "cashflow_entries", false}, {"tag_uid", "tags", true}}},
	}
}

// ImportRecord remembers identities from earlier imports, including records
// edited in the destination after import. It is part of database backups.
type ImportRecord struct {
	Source    string `gorm:"primaryKey"`
	TableName string `gorm:"primaryKey"`
	Record    string `gorm:"primaryKey"`
	TargetID  int64
}
