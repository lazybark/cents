package sqlite

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/lazybark/cents/flows/settings"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// Records link their currency by UID, like categories and accounts (see
// links.go). Currency is on nearly every record, so instead of each save
// and load doing it, two hooks do for every model with a CurrencyUID (and a
// rate record's BaseUID): loading fills the names in, saving turns them
// back into links. The base currency has a row too (IsBase), so it links
// like any other.

// currencyFields pairs each link with the name it stands for.
var currencyFields = [][2]string{{"CurrencyUID", "Currency"}, {"BaseUID", "Base"}}

func registerCurrencyLinks(db *gorm.DB) error {
	if err := db.Callback().Query().After("gorm:after_query").Register("cents:fill_currencies", fillCurrencies); err != nil {
		return err
	}

	if err := db.Callback().Create().Before("gorm:create").Register("cents:link_currencies", linkCurrencies); err != nil {
		return err
	}

	return db.Callback().Update().Before("gorm:update").Register("cents:link_currencies", linkCurrencies)
}

// linkedFields are the link and name fields of a model, if it has any.
func linkedFields(s *schema.Schema) [][2]*schema.Field {
	if s == nil {
		return nil
	}

	var pairs [][2]*schema.Field
	for _, f := range currencyFields {
		link, name := s.LookUpField(f[0]), s.LookUpField(f[1])
		if link != nil && name != nil {
			pairs = append(pairs, [2]*schema.Field{link, name})
		}
	}

	return pairs
}

// eachStruct calls fn with every struct the statement holds.
func eachStruct(tx *gorm.DB, fn func(reflect.Value)) {
	switch rv := reflect.Indirect(tx.Statement.ReflectValue); rv.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			if item := reflect.Indirect(rv.Index(i)); item.Kind() == reflect.Struct {
				fn(item)
			}
		}
	case reflect.Struct:
		fn(rv)
	}
}

type currencyRow struct {
	UID          string
	CurrencyName string
}

// currencyNames maps UIDs to names and name keys to UIDs.
func currencyNames(tx *gorm.DB) (map[string]string, map[string]string, error) {
	var rows []currencyRow
	if err := tx.Session(&gorm.Session{NewDB: true}).Table("setting_currencies").Select("uid, currency_name").Order("is_base desc, id asc").Scan(&rows).Error; err != nil {
		return nil, nil, err
	}

	names, uids := map[string]string{}, map[string]string{}
	for _, r := range rows {
		names[r.UID] = r.CurrencyName
		if _, ok := uids[nameKey(r.CurrencyName)]; !ok {
			uids[nameKey(r.CurrencyName)] = r.UID
		}
	}

	return names, uids, nil
}

func fillCurrencies(tx *gorm.DB) {
	pairs := linkedFields(tx.Statement.Schema)
	if len(pairs) == 0 || tx.Error != nil {
		return
	}

	names, _, err := currencyNames(tx)
	if err != nil {
		_ = tx.AddError(fmt.Errorf("failed to read currencies: %w", err))
		return
	}

	ctx := tx.Statement.Context
	eachStruct(tx, func(v reflect.Value) {
		for _, p := range pairs {
			uid, _ := p[0].ValueOf(ctx, v)
			_ = p[1].Set(ctx, v, names[uid.(string)])
		}
	})
}

func linkCurrencies(tx *gorm.DB) {
	pairs := linkedFields(tx.Statement.Schema)
	if len(pairs) == 0 || tx.Error != nil {
		return
	}

	_, uids, err := currencyNames(tx)
	if err != nil {
		_ = tx.AddError(fmt.Errorf("failed to read currencies: %w", err))
		return
	}

	ctx := tx.Statement.Context
	eachStruct(tx, func(v reflect.Value) {
		for _, p := range pairs {
			value, _ := p[1].ValueOf(ctx, v)
			name := strings.TrimSpace(value.(string))
			uid := ""
			if name != "" {
				var ok bool
				if uid, ok = uids[nameKey(name)]; !ok {
					_ = tx.AddError(fmt.Errorf("unknown currency %q: add it in settings first", name))
					return
				}
			}

			_ = p[0].Set(ctx, v, uid)
		}
	})
}

// --- the base currency's row -----------------------------------------------

var baseRecordIDs = map[string]bool{settings.BaseCurrencySettingID: true, settings.BaseCurrencyCodeID: true, settings.BaseCurrencyCodeForID: true}

// touchesBase reports whether records change the base currency.
func touchesBase(records []settings.SettingRecord) bool {
	for _, r := range records {
		if baseRecordIDs[r.SettingID] {
			return true
		}
	}

	return false
}

// currencyRefs are what link to currencies; rate records aren't counted as
// using one, they go with it.
var currencyRefs = []reference{
	{"accounts", "currency_uid"}, {"assets", "currency_uid"}, {"cashflow_entries", "currency_uid"},
	{"credits", "currency_uid"}, {"debts", "currency_uid"}, {"goals", "currency_uid"},
	{"invoices", "currency_uid"}, {"subscriptions", "currency_uid"}, {"taxes", "currency_uid"},
	{"setting_payment_methods", "currency_uid"},
}

// syncBaseCurrency makes the base currency's row match the settings records
// that name it (the label and the currency it's linked to). A new label for
// the same currency renames the row, so its records follow; a different
// currency, while records use the old one, leaves the old one as an
// ordinary currency (needing a rate) and makes the new one the base.
func syncBaseCurrency(tx *gorm.DB) error {
	var records []settings.SettingRecord
	if err := tx.Where("setting_id IN ?", []string{settings.BaseCurrencySettingID, settings.BaseCurrencyCodeID, settings.BaseCurrencyCodeForID}).Find(&records).Error; err != nil {
		return err
	}

	label, linked, linkedFor := "$", "", ""
	for _, r := range records {
		value := strings.TrimSpace(r.SettingValue)
		switch r.SettingID {
		case settings.BaseCurrencySettingID:
			if value != "" {
				label = value
			}
		case settings.BaseCurrencyCodeID:
			linked = value
		case settings.BaseCurrencyCodeForID:
			linkedFor = value
		}
	}

	code := settings.BaseCodeFor(label, linked, linkedFor)

	var all []settings.SettingCurrency
	if err := tx.Order("id asc").Find(&all).Error; err != nil {
		return err
	}

	var base, named *settings.SettingCurrency
	for i := range all {
		if all[i].IsBase && base == nil {
			base = &all[i]
		}

		if nameKey(all[i].CurrencyName) == nameKey(label) {
			named = &all[i]
		}
	}

	makeBase := func(c *settings.SettingCurrency) error {
		if c == nil {
			c = &settings.SettingCurrency{CurrencyName: label}
		}

		c.IsBase, c.RateToBase, c.Code = true, 1, code
		return tx.Save(c).Error
	}

	if base == nil {
		return makeBase(named)
	}

	if named == base {
		return makeBase(base)
	}

	// A new label for the same currency (or a base nothing uses yet): the
	// row is renamed and its records follow.
	if named == nil && !switching(tx, base, code) {
		base.CurrencyName = label
		return makeBase(base)
	}

	// Another currency is the base now: the old one stays, as an ordinary
	// currency that needs a rate.
	base.IsBase, base.RateToBase = false, 0
	if err := tx.Save(base).Error; err != nil {
		return err
	}

	return makeBase(named)
}

// switching reports whether code is a different currency from base, with
// records still using base: then base can't simply be renamed.
func switching(tx *gorm.DB, base *settings.SettingCurrency, code string) bool {
	old := base.Code
	if old == "" {
		old = base.LinkedCode()
	}

	if code == "" || old == "" || code == old {
		return false
	}

	n, err := usage(tx, currencyRefs, base.UID)
	return err == nil && n > 0
}

var errBaseCurrency = errors.New("that's the base currency; change it with the base currency instead")
