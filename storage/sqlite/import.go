package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/lazybark/cents/dbimport"
	"github.com/lazybark/cents/flows/settings"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type importRow = map[string]any

var errImportPreview = errors.New("import preview rollback")

// ImportDatabase adds a snapshot's missing records in one transaction. Preview
// runs the same checks/inserts, then rolls them all back (including identities).
func (s *SQLiteStorage) ImportDatabase(ctx context.Context, path string, preview bool) (dbimport.Result, error) {
	var result dbimport.Result
	if s.vault != nil {
		return result, errors.New("import into an unencrypted database or server; encrypted destinations are not supported yet")
	}
	tables := importTables()
	for i := range tables {
		stmt := &gorm.Statement{DB: s.db}
		if err := stmt.Parse(tables[i].model); err != nil {
			return result, err
		}
		tables[i].name, tables[i].columns = stmt.Schema.Table, stmt.Schema.DBNames
	}
	snapshot, err := readImport(ctx, path, tables)
	if err != nil {
		return result, err
	}
	result.Preview = preview
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(&ImportRecord{}); err != nil {
			return err
		}
		m := &databaseMerge{tx: tx, ids: map[string]map[int64]int64{}, uids: map[string]map[string]string{}, snapshot: snapshot}
		if err := m.checkBase(); err != nil {
			return err
		}
		var remembered []ImportRecord
		if err := tx.Where("source = ?", m.source).Find(&remembered).Error; err != nil {
			return err
		}
		m.remembered = map[string]int64{}
		for _, r := range remembered {
			m.remembered[r.TableName+"/"+r.Record] = r.TargetID
		}
		for _, table := range tables {
			stats, err := m.mergeTable(table)
			if err != nil {
				return err
			}
			result.Tables = append(result.Tables, stats)
			result.Added += stats.Added
			result.Skipped += stats.Skipped
			result.Conflicts += stats.Conflicts
		}
		if err := EnsurePaymentMethodDefaults(tx); err != nil {
			return err
		}
		if preview {
			return errImportPreview
		}
		return nil
	})
	if errors.Is(err, errImportPreview) {
		err = nil
	}
	return result, err
}

// readImport opens only a read-only SQLite snapshot. No migrations, triggers,
// views, uploaded SQL or uploaded table/column names run against either DB.
func readImport(ctx context.Context, path string, tables []importTable) (map[string][]importRow, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: abs, RawQuery: "mode=ro&immutable=1&_query_only=1"}
	db, err := sql.Open("sqlite3", u.String())
	if err != nil {
		return nil, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var integrity string
	if err := db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&integrity); err != nil || integrity != "ok" {
		return nil, errors.New("choose an unencrypted, complete cents .db backup")
	}
	var objects int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema WHERE type IN ('trigger', 'view')").Scan(&objects); err != nil || objects != 0 {
		return nil, errors.New("choose a cents backup without custom triggers or views")
	}
	all := append(append([]importTable{}, tables...), importTable{name: "setting_records", columns: []string{"setting_id", "setting_value"}})
	snapshot := map[string][]importRow{}
	total := 0
	for _, spec := range all {
		rows, err := db.QueryContext(ctx, "SELECT "+quoteColumns(spec.columns)+" FROM `"+spec.name+"` LIMIT ?", dbimport.MaxRows-total+1)
		if err != nil {
			return nil, errors.New("the database uses a different schema; open it in the current desktop app and make a new backup first")
		}
		items, err := scanImportRows(rows)
		if err != nil {
			return nil, errors.New("could not read the database records")
		}
		total += len(items)
		if total > dbimport.MaxRows {
			return nil, fmt.Errorf("import at most %d records at a time", dbimport.MaxRows)
		}
		snapshot[spec.name] = items
	}
	return snapshot, nil
}

func quoteColumns(columns []string) string { return "`" + strings.Join(columns, "`,`") + "`" }

func scanImportRows(rows *sql.Rows) ([]importRow, error) {
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var result []importRow
	for rows.Next() {
		values, pointers := make([]any, len(columns)), make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		row := importRow{}
		for i, column := range columns {
			value := values[i]
			if bytes, ok := value.([]byte); ok {
				value = string(bytes)
			}
			row[column] = value
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

type databaseMerge struct {
	tx         *gorm.DB
	snapshot   map[string][]importRow
	source     string
	baseID     int64
	ids        map[string]map[int64]int64
	uids       map[string]map[string]string
	remembered map[string]int64
}

func integer(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case bool:
		if n {
			return 1
		}
	}
	return 0
}

func text(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func currencyCode(row importRow) string {
	return settings.BaseCodeFor(text(row["currency_name"]), text(row["code"]), text(row["currency_name"]))
}

func (m *databaseMerge) checkBase() error {
	var source importRow
	for _, row := range m.snapshot["setting_currencies"] {
		if integer(row["is_base"]) != 0 {
			if source != nil {
				return errors.New("the source database has more than one base currency")
			}
			source = row
		}
	}
	if source == nil || text(source["uid"]) == "" {
		return errors.New("the source database has no base currency; make a backup with the current desktop app")
	}
	m.source = text(source["uid"])
	current, err := LoadAppSettings(m.tx)
	if err != nil {
		return err
	}
	m.baseID = int64(current.BaseCurrencyID)
	same := strings.EqualFold(current.BaseCurrency, text(source["currency_name"]))
	if code := currencyCode(source); code != "" && current.BaseCurrencyCode != "" {
		same = strings.EqualFold(code, current.BaseCurrencyCode)
	}
	if same {
		if !current.CurrenciesSetUp {
			for _, row := range m.snapshot["setting_records"] {
				if text(row["setting_id"]) == settings.CurrenciesSetUpID && text(row["setting_value"]) == "1" {
					return m.tx.Save(&settings.SettingRecord{SettingID: settings.CurrenciesSetUpID, SettingValue: "1"}).Error
				}
			}
		}
		return nil
	}
	if current.CurrenciesSetUp || len(current.Currencies) > 0 {
		return errors.New("both databases must use the same base currency; importing does not convert amounts")
	}
	for _, table := range []string{"accounts", "cashflow_entries", "assets", "debts", "credits", "goals", "taxes", "subscriptions", "invoices", "budgets", "net_worth_snapshots"} {
		var count int64
		if err := m.tx.Table(table).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return errors.New("both databases must use the same base currency; importing does not convert amounts")
		}
	}
	if err := m.tx.Table("setting_currencies").Where("id = ?", m.baseID).Updates(map[string]any{"currency_name": source["currency_name"], "code": source["code"], "rate_to_base": 1}).Error; err != nil {
		return err
	}
	for key, value := range map[string]string{
		settings.BaseCurrencySettingID: text(source["currency_name"]), settings.BaseCurrencyCodeID: currencyCode(source), settings.BaseCurrencyCodeForID: text(source["currency_name"]), settings.CurrenciesSetUpID: "1",
	} {
		if err := m.tx.Save(&settings.SettingRecord{SettingID: key, SettingValue: value}).Error; err != nil {
			return err
		}
	}
	return nil
}

// rowKey ignores DB-local primary keys. Timestamps retain the distinction
// between independent transactions that happen to have the same amount/date.
func rowKey(row importRow, meaningful bool) string {
	copy := importRow{}
	for key, value := range row {
		if key == "id" || meaningful && (key == "uid" || key == "created_at" || key == "last_updated_at" || key == "updated_at" || key == "is_default") {
			continue
		}
		if t, ok := value.(time.Time); ok {
			value = t.UTC().Format(time.RFC3339Nano)
		}
		copy[key] = value
	}
	encoded, _ := json.Marshal(copy)
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:])
}

func naturalKey(row importRow, fields []string) string {
	parts := make([]string, len(fields))
	for i, field := range fields {
		if t, ok := row[field].(time.Time); ok {
			parts[i] = t.UTC().Format(time.RFC3339Nano)
		} else {
			parts[i] = nameKey(text(row[field]))
		}
	}
	return strings.Join(parts, "\x00")
}

func (m *databaseMerge) remap(row importRow, spec importTable) error {
	refs := append([]importRef{}, spec.refs...)
	if spec.name == "cashflow_entries" {
		table := "setting_expense_categories"
		if integer(row["is_income"]) != 0 {
			table = "setting_income_categories"
		}
		refs = append(refs, importRef{"category_uid", table, true})
	}
	for _, ref := range refs {
		if ref.uid {
			old := text(row[ref.column])
			if old == "" {
				continue
			}
			mapped, ok := m.uids[ref.table][old]
			if !ok {
				return fmt.Errorf("%s contains a missing %s link; nothing was imported", spec.name, ref.table)
			}
			row[ref.column] = mapped
		} else {
			old := integer(row[ref.column])
			if old == 0 {
				if ref.column != "cashflow_entry_id" {
					return fmt.Errorf("%s contains an empty %s link; nothing was imported", spec.name, ref.table)
				}
				continue
			}
			mapped, ok := m.ids[ref.table][old]
			if !ok {
				return fmt.Errorf("%s contains a missing %s record; nothing was imported", spec.name, ref.table)
			}
			row[ref.column] = mapped
		}
	}
	return nil
}

func (m *databaseMerge) mergeTable(spec importTable) (dbimport.Table, error) {
	stats := dbimport.Table{Name: spec.name}
	rows, err := m.tx.Raw("SELECT " + quoteColumns(spec.columns) + " FROM `" + spec.name + "` ORDER BY " + quoteColumns(spec.columns[:1])).Rows()
	if err != nil {
		return stats, err
	}
	existing, err := scanImportRows(rows)
	if err != nil {
		return stats, err
	}
	byID, byUID, byNatural, exact := map[int64]importRow{}, map[string]importRow{}, map[string]importRow{}, map[string][]importRow{}
	for _, row := range existing {
		byID[integer(row["id"])] = row
		if uid := text(row["uid"]); uid != "" {
			byUID[uid] = row
		}
		if len(spec.natural) > 0 {
			byNatural[naturalKey(row, spec.natural)] = row
		}
		key := rowKey(row, false)
		exact[key] = append(exact[key], row)
	}
	m.ids[spec.name], m.uids[spec.name] = map[int64]int64{}, map[string]string{}
	used := map[int64]bool{}
	for _, original := range m.snapshot[spec.name] {
		row := importRow{}
		for k, v := range original {
			row[k] = v
		}
		oldID, oldUID := integer(row["id"]), text(row["uid"])
		if spec.name != "cashflow_entry_tags" && oldID <= 0 {
			return stats, fmt.Errorf("%s has an invalid record ID", spec.name)
		}
		if spec.uid {
			if _, duplicate := m.uids[spec.name][oldUID]; duplicate {
				return stats, fmt.Errorf("%s has duplicate record identities", spec.name)
			}
		}
		identity := fmt.Sprint(oldID) + "/" + text(row["created_at"])
		if err := m.remap(row, spec); err != nil {
			return stats, err
		}
		var match importRow
		if spec.uid {
			if oldUID == "" {
				return stats, fmt.Errorf("%s has a record without an identity", spec.name)
			}
			match = byUID[oldUID]
			if spec.name == "setting_currencies" && integer(row["is_base"]) != 0 {
				match = byID[m.baseID]
			}
		}
		if match == nil {
			if rememberedID, ok := m.remembered[spec.name+"/"+identity]; ok && rememberedID > 0 {
				match = byID[rememberedID]
			}
		}
		if match == nil && len(spec.natural) > 0 {
			match = byNatural[naturalKey(row, spec.natural)]
		}
		if match == nil && spec.name == "setting_currencies" && currencyCode(row) != "" {
			for _, candidate := range existing {
				if currencyCode(candidate) == currencyCode(row) {
					match = candidate
					break
				}
			}
		}
		if match == nil {
			for _, candidate := range exact[rowKey(row, false)] {
				if !used[integer(candidate["id"])] {
					match = candidate
					break
				}
			}
		}
		if match != nil {
			if spec.name == "setting_currencies" && currencyCode(row) != "" && currencyCode(match) != "" && currencyCode(row) != currencyCode(match) {
				return stats, errors.New("a currency name refers to different currencies in these databases; rename it before importing")
			}
			stats.Skipped++
			if rowKey(row, true) != rowKey(match, true) {
				stats.Conflicts++
			}
			row = match
		} else {
			delete(row, "id")
			if spec.name == "setting_payment_methods" {
				row["is_default"] = false
			}
			if err := m.insert(spec, row); err != nil {
				return stats, err
			}
			stats.Added++
			if uid := text(row["uid"]); uid != "" {
				byUID[uid] = row
			}
			if len(spec.natural) > 0 {
				byNatural[naturalKey(row, spec.natural)] = row
			}
		}
		id := integer(row["id"])
		used[id] = true
		m.ids[spec.name][oldID] = id
		if spec.uid {
			m.uids[spec.name][oldUID] = text(row["uid"])
		}
		if oldID != 0 {
			record := ImportRecord{Source: m.source, TableName: spec.name, Record: identity, TargetID: id}
			if err := m.tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&record).Error; err != nil {
				return stats, err
			}
		}
	}
	return stats, nil
}

func (m *databaseMerge) insert(spec importTable, row importRow) error {
	var columns []string
	var values []any
	for _, column := range spec.columns {
		if column == "id" {
			continue
		}
		columns, values = append(columns, column), append(values, row[column])
	}
	query := "INSERT INTO `" + spec.name + "` (" + quoteColumns(columns) + ") VALUES (" + strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",") + ")"
	if spec.name == "cashflow_entry_tags" {
		return m.tx.Exec(query, values...).Error
	}
	var id int64
	if err := m.tx.Raw(query+" RETURNING id", values...).Scan(&id).Error; err != nil {
		return err
	}
	row["id"] = id
	return nil
}
