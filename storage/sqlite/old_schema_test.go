package sqlite

import (
	"testing"

	"gorm.io/gorm"
)

// asOldDatabase gives db the shape it had before records linked their
// currency: a currency column on each record, holding the name.
func asOldDatabase(t *testing.T, db *gorm.DB) {
	t.Helper()

	for _, table := range currencyTables {
		for _, stmt := range []string{
			"ALTER TABLE " + table + " ADD COLUMN currency text",
			"UPDATE " + table + " SET currency = (SELECT currency_name FROM setting_currencies c WHERE c.uid = " + table + ".currency_uid), currency_uid = ''",
		} {
			if err := db.Exec(stmt).Error; err != nil {
				t.Fatalf("%s: %v", stmt, err)
			}
		}
	}
}
