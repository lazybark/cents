package sqlite

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/settings"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Databases from before links named categories, accounts, payment methods
// and tax types on each record. These are the columns that did; once a
// database is linked they're gone.
var nameColumns = []struct{ table, column string }{
	{"cashflow_entries", "category"},
	{"cashflow_entries", "account_name"},
	{"budgets", "category"},
	{"subscriptions", "payment_method"},
	{"taxes", "tax_type_id"},
	{"taxes", "tax_country"},
	{"taxes", "tax_type_name"},
}

// currencyTables named their currency in a "currency" column.
var currencyTables = []string{"accounts", "assets", "cashflow_entries", "credits", "debts", "goals", "invoices", "subscriptions", "taxes", "setting_payment_methods"}

// linkedTables have a UID that records link to.
var linkedTables = []string{"setting_income_categories", "setting_expense_categories", "setting_payment_methods", "setting_tax_types", "accounts", "setting_currencies", "tags"}

func needsLinking(db *gorm.DB) bool {
	has := func(table, column string) bool {
		return db.Migrator().HasTable(table) && db.Migrator().HasColumn(table, column)
	}

	for _, c := range nameColumns {
		if has(c.table, c.column) {
			return true
		}
	}

	for _, table := range currencyTables {
		if has(table, "currency") {
			return true
		}
	}

	return has("rate_records", "currency")
}

// backupBeforeLinking saves a copy of a database about to be linked next to
// it, since the change can't be undone: older versions of the app can't
// read a linked database.
func backupBeforeLinking(db *gorm.DB, dbPath string, now time.Time) (string, error) {
	ext := filepath.Ext(dbPath)
	path := strings.TrimSuffix(dbPath, ext) + "-before-links-" + now.Format("2006-01-02-150405") + ext
	if err := db.Exec("VACUUM INTO ?", path).Error; err != nil {
		return "", fmt.Errorf("failed to back up the database before linking records: %w", err)
	}

	return path, nil
}

// assignMissingUIDs gives every row records can link to a UID.
func assignMissingUIDs(tx *gorm.DB) error {
	for _, table := range linkedTables {
		var ids []uint
		if err := tx.Table(table).Where("uid = '' OR uid IS NULL").Pluck("id", &ids).Error; err != nil {
			return fmt.Errorf("failed to find %s without a UID: %w", table, err)
		}

		for _, id := range ids {
			if err := tx.Table(table).Where("id = ?", id).Update("uid", uuid.NewString()).Error; err != nil {
				return fmt.Errorf("failed to give %s %d a UID: %w", table, id, err)
			}
		}
	}

	return nil
}

type namedRow struct {
	ID       uint
	IsIncome bool
	Name     string
	Currency string
	Country  string
}

// linkByUID turns the names records use into links, in one go: rows get
// UIDs, names that match nothing get an archived stand-in (so no record
// loses what it said), every record is linked, and the name columns are
// dropped. Names match ignoring case and surrounding spaces.
func linkByUID(db *gorm.DB, now time.Time) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := assignMissingUIDs(tx); err != nil {
			return err
		}

		has := func(table, column string) bool {
			return tx.Migrator().HasColumn(table, column)
		}

		// Currencies first: what the steps below add may need one.
		if err := linkCurrencyColumns(tx); err != nil {
			return err
		}

		if has("cashflow_entries", "category") || has("budgets", "category") {
			if err := linkCategories(tx, has("budgets", "category")); err != nil {
				return err
			}
		}

		if has("cashflow_entries", "account_name") {
			if err := linkAccounts(tx, now); err != nil {
				return err
			}
		}

		if has("subscriptions", "payment_method") {
			if err := linkPaymentMethods(tx, now); err != nil {
				return err
			}
		}

		if has("taxes", "tax_type_name") {
			if err := linkTaxTypes(tx, now); err != nil {
				return err
			}
		}

		for _, c := range nameColumns {
			if has(c.table, c.column) {
				if err := tx.Exec(fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`", c.table, c.column)).Error; err != nil {
					return fmt.Errorf("failed to drop %s.%s: %w", c.table, c.column, err)
				}
			}
		}

		return nil
	})
}

// setLinks writes uid into column for the ids, in batches.
func setLinks(tx *gorm.DB, table, column string, byUID map[string][]uint) error {
	for uid, ids := range byUID {
		for start := 0; start < len(ids); start += 500 {
			end := min(start+500, len(ids))
			if err := tx.Table(table).Where("id IN ?", ids[start:end]).Update(column, uid).Error; err != nil {
				return fmt.Errorf("failed to link %s: %w", table, err)
			}
		}
	}

	return nil
}

func linkCategories(tx *gorm.DB, budgets bool) error {
	var income []settings.SettingIncomeCategory
	var expense []settings.SettingExpenseCategory
	if err := tx.Order("archived asc, id asc").Find(&income).Error; err != nil {
		return err
	}
	if err := tx.Order("archived asc, id asc").Find(&expense).Error; err != nil {
		return err
	}

	incomeUIDs, expenseUIDs := map[string]string{}, map[string]string{}
	for _, c := range income {
		if _, ok := incomeUIDs[nameKey(c.CategoryName)]; !ok {
			incomeUIDs[nameKey(c.CategoryName)] = c.UID
		}
	}
	for _, c := range expense {
		if _, ok := expenseUIDs[nameKey(c.CategoryName)]; !ok {
			expenseUIDs[nameKey(c.CategoryName)] = c.UID
		}
	}

	// uidFor finds the category, adding an archived stand-in for a name no
	// category has.
	uidFor := func(isIncome bool, name string) (string, error) {
		uids := expenseUIDs
		if isIncome {
			uids = incomeUIDs
		}

		if uid, ok := uids[nameKey(name)]; ok {
			return uid, nil
		}

		var uid string
		if isIncome {
			c := settings.SettingIncomeCategory{CategoryName: strings.TrimSpace(name), Archived: true}
			if err := tx.Create(&c).Error; err != nil {
				return "", fmt.Errorf("failed to add income category %q: %w", name, err)
			}
			uid = c.UID
		} else {
			c := settings.SettingExpenseCategory{CategoryName: strings.TrimSpace(name), Archived: true}
			if err := tx.Create(&c).Error; err != nil {
				return "", fmt.Errorf("failed to add expense category %q: %w", name, err)
			}
			uid = c.UID
		}

		uids[nameKey(name)] = uid
		return uid, nil
	}

	if tx.Migrator().HasColumn("cashflow_entries", "category") {
		var rows []namedRow
		if err := tx.Table("cashflow_entries").Select("id, is_income, category AS name").Scan(&rows).Error; err != nil {
			return err
		}

		byUID := map[string][]uint{}
		for _, r := range rows {
			if strings.TrimSpace(r.Name) == "" {
				continue
			}

			uid, err := uidFor(r.IsIncome, r.Name)
			if err != nil {
				return err
			}

			byUID[uid] = append(byUID[uid], r.ID)
		}

		if err := setLinks(tx, "cashflow_entries", "category_uid", byUID); err != nil {
			return err
		}
	}

	if budgets {
		var rows []namedRow
		if err := tx.Table("budgets").Select("id, category AS name").Scan(&rows).Error; err != nil {
			return err
		}

		byUID := map[string][]uint{}
		for _, r := range rows {
			if strings.TrimSpace(r.Name) == "" {
				continue
			}

			uid, err := uidFor(false, r.Name)
			if err != nil {
				return err
			}

			byUID[uid] = append(byUID[uid], r.ID)
		}

		if err := setLinks(tx, "budgets", "category_uid", byUID); err != nil {
			return err
		}
	}

	return nil
}

func linkAccounts(tx *gorm.DB, now time.Time) error {
	var accounts []account.Account
	if err := tx.Order("archived asc, id asc").Find(&accounts).Error; err != nil {
		return err
	}

	uids := map[string]string{}
	for _, a := range accounts {
		if _, ok := uids[nameKey(a.Name)]; !ok {
			uids[nameKey(a.Name)] = a.UID
		}
	}

	var rows []namedRow
	if err := tx.Table("cashflow_entries").Select("id, account_name AS name, currency_uid AS currency").Scan(&rows).Error; err != nil {
		return err
	}

	// Entries hold their currency's link by now; stand-ins take its name.
	currencyNamesByUID, _, err := currencyNames(tx)
	if err != nil {
		return err
	}

	byUID := map[string][]uint{}
	for _, r := range rows {
		if strings.TrimSpace(r.Name) == "" {
			continue
		}

		uid, ok := uids[nameKey(r.Name)]
		if !ok {
			// An account entries named that doesn't exist (deleted, say):
			// kept as an archived account, out of totals and pickers.
			a := account.Account{
				Name:              strings.TrimSpace(r.Name),
				Description:       "Added when records were linked: entries named it, but no such account existed.",
				Currency:          currencyNamesByUID[r.Currency],
				IgnoreInSummaries: true,
				Archived:          true,
				CreatedAt:         now,
				LastUpdatedAt:     now,
			}
			if err := tx.Create(&a).Error; err != nil {
				return fmt.Errorf("failed to add account %q: %w", r.Name, err)
			}

			uid = a.UID
			uids[nameKey(r.Name)] = uid
		}

		byUID[uid] = append(byUID[uid], r.ID)
	}

	return setLinks(tx, "cashflow_entries", "account_uid", byUID)
}

func linkPaymentMethods(tx *gorm.DB, now time.Time) error {
	var methods []settings.SettingPaymentMethod
	if err := tx.Order("id asc").Find(&methods).Error; err != nil {
		return err
	}

	uids := map[string]string{}
	for _, m := range methods {
		if _, ok := uids[nameKey(m.PaymentMethodName)]; !ok {
			uids[nameKey(m.PaymentMethodName)] = m.UID
		}
	}

	var rows []namedRow
	if err := tx.Table("subscriptions").Select("id, payment_method AS name").Scan(&rows).Error; err != nil {
		return err
	}

	byUID := map[string][]uint{}
	for _, r := range rows {
		if strings.TrimSpace(r.Name) == "" {
			continue
		}

		uid, ok := uids[nameKey(r.Name)]
		if !ok {
			m := settings.SettingPaymentMethod{PaymentMethodName: strings.TrimSpace(r.Name), PaymentMethodType: "Other", Archived: true, CreatedAt: now, LastUpdatedAt: now}
			if err := tx.Create(&m).Error; err != nil {
				return fmt.Errorf("failed to add payment method %q: %w", r.Name, err)
			}

			uid = m.UID
			uids[nameKey(r.Name)] = uid
		}

		byUID[uid] = append(byUID[uid], r.ID)
	}

	return setLinks(tx, "subscriptions", "payment_method_uid", byUID)
}

func linkTaxTypes(tx *gorm.DB, now time.Time) error {
	var types []settings.SettingTaxType
	if err := tx.Order("id asc").Find(&types).Error; err != nil {
		return err
	}

	byID := map[uint]string{}
	byName := map[string]string{}
	for _, t := range types {
		byID[t.ID] = t.UID
		key := nameKey(t.Country) + "\x00" + nameKey(t.TaxTypeName)
		if _, ok := byName[key]; !ok {
			byName[key] = t.UID
		}
	}

	var rows []struct {
		ID          uint
		TaxTypeID   uint
		TaxCountry  string
		TaxTypeName string
	}
	columns := "id, tax_type_name, tax_country"
	if tx.Migrator().HasColumn("taxes", "tax_type_id") {
		columns += ", tax_type_id"
	}
	if err := tx.Table("taxes").Select(columns).Scan(&rows).Error; err != nil {
		return err
	}

	byUID := map[string][]uint{}
	for _, r := range rows {
		uid, ok := byID[r.TaxTypeID]
		if !ok || r.TaxTypeID == 0 {
			key := nameKey(r.TaxCountry) + "\x00" + nameKey(r.TaxTypeName)
			if uid, ok = byName[key]; !ok {
				t := settings.SettingTaxType{Country: strings.TrimSpace(r.TaxCountry), TaxTypeName: strings.TrimSpace(r.TaxTypeName), Description: "Added when taxes were linked to their types.", CreatedAt: now, LastUpdatedAt: now}
				if err := tx.Create(&t).Error; err != nil {
					return fmt.Errorf("failed to add tax type %s / %s: %w", r.TaxCountry, r.TaxTypeName, err)
				}

				uid = t.UID
				byName[key] = uid
			}
		}

		byUID[uid] = append(byUID[uid], r.ID)
	}

	return setLinks(tx, "taxes", "tax_type_uid", byUID)
}

// linkCurrencyColumns links each record's currency by UID, adding a
// currency (without a rate, as it had none) for any name settings don't
// have, and drops the currency name columns.
func linkCurrencyColumns(tx *gorm.DB) error {
	if err := syncBaseCurrency(tx); err != nil {
		return err
	}

	if err := assignMissingUIDs(tx); err != nil {
		return err
	}

	_, uids, err := currencyNames(tx)
	if err != nil {
		return err
	}

	for _, table := range currencyTables {
		if !tx.Migrator().HasColumn(table, "currency") {
			continue
		}

		var rows []namedRow
		if err := tx.Table(table).Select("id, currency AS name").Scan(&rows).Error; err != nil {
			return err
		}

		byUID := map[string][]uint{}
		for _, r := range rows {
			name := strings.TrimSpace(r.Name)
			if name == "" {
				continue
			}

			uid, ok := uids[nameKey(name)]
			if !ok {
				c := settings.SettingCurrency{CurrencyName: name}
				if err := tx.Create(&c).Error; err != nil {
					return fmt.Errorf("failed to add currency %q: %w", name, err)
				}

				uid = c.UID
				uids[nameKey(name)] = uid
			}

			byUID[uid] = append(byUID[uid], r.ID)
		}

		if err := setLinks(tx, table, "currency_uid", byUID); err != nil {
			return err
		}

		if err := tx.Exec(fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `currency`", table)).Error; err != nil {
			return fmt.Errorf("failed to drop %s.currency: %w", table, err)
		}
	}

	return nil
}

// oldRateRecord is the rate history as kept before it was linked.
type oldRateRecord struct {
	Day        time.Time
	Currency   string
	Base       string
	RateToBase float64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// takeOldRateRecords reads a rate history that names currencies and drops
// its table, so it can be made again linked.
func takeOldRateRecords(db *gorm.DB) ([]oldRateRecord, error) {
	if !db.Migrator().HasTable("rate_records") || !db.Migrator().HasColumn("rate_records", "currency") {
		return nil, nil
	}

	var rows []oldRateRecord
	if err := db.Table("rate_records").Select("day, currency, base, rate_to_base, created_at, updated_at").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to read the rate history: %w", err)
	}

	if err := db.Migrator().DropTable("rate_records"); err != nil {
		return nil, fmt.Errorf("failed to set the rate history aside: %w", err)
	}

	return rows, nil
}

// putBackRateRecords keeps the old rate history, linked; rates of
// currencies (or against bases) that are gone are left out.
func putBackRateRecords(db *gorm.DB, rows []oldRateRecord) error {
	if len(rows) == 0 {
		return nil
	}

	_, uids, err := currencyNames(db)
	if err != nil {
		return err
	}

	records := make([]settings.RateRecord, 0, len(rows))
	for _, r := range rows {
		_, currency := uids[nameKey(r.Currency)]
		_, base := uids[nameKey(r.Base)]
		if currency && base {
			records = append(records, settings.RateRecord{Day: r.Day, Currency: r.Currency, Base: r.Base, RateToBase: r.RateToBase, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt})
		}
	}

	if len(records) == 0 {
		return nil
	}

	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&records).Error
}
