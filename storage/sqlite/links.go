package sqlite

import (
	"fmt"
	"strings"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/budget"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/subscription"
	"github.com/lazybark/cents/flows/tax"
	"gorm.io/gorm"
)

// Records link categories, accounts, payment methods and tax types by UID.
// The rest of the app works with names: loading fills them in from the
// links, and saving turns them back into links, so a rename shows
// everywhere without touching a single record.

// links maps names (matched ignoring case and spaces) to UIDs and back.
type links struct {
	income, expense, methods, accounts map[string]string
	// ambiguousAccounts are names more than one active account has.
	ambiguousAccounts map[string]bool
	names             map[string]string
	taxByID           map[uint]settings.SettingTaxType
	taxByUID          map[string]settings.SettingTaxType
}

func nameKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func loadLinks(db *gorm.DB) (*links, error) {
	l := &links{
		income: map[string]string{}, expense: map[string]string{}, methods: map[string]string{}, accounts: map[string]string{},
		ambiguousAccounts: map[string]bool{}, names: map[string]string{},
		taxByID: map[uint]settings.SettingTaxType{}, taxByUID: map[string]settings.SettingTaxType{},
	}

	var income []settings.SettingIncomeCategory
	var expense []settings.SettingExpenseCategory
	var methods []settings.SettingPaymentMethod
	var taxTypes []settings.SettingTaxType
	var accounts []account.Account
	for _, q := range []struct {
		dest  any
		order string
	}{{&income, "archived asc, id asc"}, {&expense, "archived asc, id asc"}, {&methods, "archived asc, id asc"}, {&taxTypes, "id asc"}, {&accounts, "archived asc, id asc"}} {
		if err := db.Order(q.order).Find(q.dest).Error; err != nil {
			return nil, fmt.Errorf("failed to load what records link to: %w", err)
		}
	}

	add := func(m map[string]string, name, uid string) {
		l.names[uid] = name
		if _, ok := m[nameKey(name)]; !ok {
			m[nameKey(name)] = uid
		}
	}

	for _, c := range income {
		add(l.income, c.CategoryName, c.UID)
	}

	for _, c := range expense {
		add(l.expense, c.CategoryName, c.UID)
	}

	for _, m := range methods {
		add(l.methods, m.PaymentMethodName, m.UID)
	}

	active := map[string]int{}
	for _, a := range accounts {
		add(l.accounts, a.Name, a.UID)
		if !a.Archived {
			active[nameKey(a.Name)]++
		}
	}

	for key, n := range active {
		l.ambiguousAccounts[key] = n > 1
	}

	for _, t := range taxTypes {
		l.taxByID[t.ID] = t
		l.taxByUID[t.UID] = t
	}

	return l, nil
}

// --- loading: links to names ---------------------------------------------

func (l *links) fillCashflows(entries []cashflow.CashflowEntry) {
	for i := range entries {
		entries[i].Category = l.names[entries[i].CategoryUID]
		entries[i].AccountName = l.names[entries[i].AccountUID]
	}
}

func (l *links) fillBudgets(items []budget.Budget) {
	for i := range items {
		items[i].Category = l.names[items[i].CategoryUID]
	}
}

func (l *links) fillSubscriptions(items []subscription.Subscription) {
	for i := range items {
		items[i].PaymentMethod = l.names[items[i].PaymentMethodUID]
	}
}

func (l *links) fillTaxes(items []tax.Tax) {
	for i := range items {
		t := l.taxByUID[items[i].TaxTypeUID]
		items[i].TaxTypeID, items[i].TaxCountry, items[i].TaxTypeName = t.ID, t.Country, t.TaxTypeName
	}
}

// --- saving: names to links ------------------------------------------------

func (l *links) linkCashflow(e *cashflow.CashflowEntry) error {
	e.CategoryUID = ""
	if name := strings.TrimSpace(e.Category); name != "" {
		kind, list := "expense", l.expense
		if e.IsIncome {
			kind, list = "income", l.income
		}

		uid, ok := list[nameKey(name)]
		if !ok {
			return fmt.Errorf("unknown %s category %q", kind, name)
		}

		e.CategoryUID = uid
	}

	e.AccountUID = ""
	if name := strings.TrimSpace(e.AccountName); name != "" {
		if l.ambiguousAccounts[nameKey(name)] {
			return fmt.Errorf("more than one account is called %q; rename one of them first", name)
		}

		uid, ok := l.accounts[nameKey(name)]
		if !ok {
			return fmt.Errorf("unknown account %q", name)
		}

		e.AccountUID = uid
	}

	return nil
}

func (l *links) linkBudget(b *budget.Budget) error {
	b.CategoryUID = ""
	if name := strings.TrimSpace(b.Category); name != "" {
		uid, ok := l.expense[nameKey(name)]
		if !ok {
			return fmt.Errorf("unknown expense category %q", name)
		}

		b.CategoryUID = uid
	}

	return nil
}

// linkSubscription links s's payment method, adding it to settings when
// it's a new one: payment methods can be typed in freely (the TUI's
// "custom" one).
func (l *links) linkSubscription(tx *gorm.DB, s *subscription.Subscription) error {
	s.PaymentMethodUID = ""
	name := strings.TrimSpace(s.PaymentMethod)
	if name == "" {
		return nil
	}

	if uid, ok := l.methods[nameKey(name)]; ok {
		s.PaymentMethodUID = uid
		return nil
	}

	method := settings.SettingPaymentMethod{PaymentMethodName: name, PaymentMethodType: "Other", CreatedAt: s.LastUpdatedAt, LastUpdatedAt: s.LastUpdatedAt}
	if err := tx.Create(&method).Error; err != nil {
		return fmt.Errorf("failed to add payment method %q: %w", name, err)
	}

	l.methods[nameKey(name)] = method.UID
	l.names[method.UID] = name
	s.PaymentMethodUID = method.UID

	return nil
}

// linkTax links t's tax type: the one with its TaxTypeID, else the one with
// its country and name.
func (l *links) linkTax(t *tax.Tax) error {
	if typ, ok := l.taxByID[t.TaxTypeID]; ok && t.TaxTypeID != 0 {
		t.TaxTypeUID, t.TaxCountry, t.TaxTypeName = typ.UID, typ.Country, typ.TaxTypeName
		return nil
	}

	for _, typ := range l.taxByID {
		if nameKey(typ.Country) == nameKey(t.TaxCountry) && nameKey(typ.TaxTypeName) == nameKey(t.TaxTypeName) {
			t.TaxTypeUID, t.TaxTypeID = typ.UID, typ.ID
			return nil
		}
	}

	return fmt.Errorf("unknown tax type %s / %s", t.TaxCountry, t.TaxTypeName)
}

// createCashflow links e and creates it.
func createCashflow(tx *gorm.DB, e *cashflow.CashflowEntry) error {
	l, err := loadLinks(tx)
	if err != nil {
		return err
	}

	if err := l.linkCashflow(e); err != nil {
		return err
	}

	if err := tx.Create(e).Error; err != nil {
		return fmt.Errorf("failed to create cashflow entry: %w", err)
	}

	return syncTags(tx, e)
}
