package sqlite

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/lazybark/cents/flows/account"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/debt"
	"github.com/lazybark/cents/flows/goal"
	"github.com/lazybark/cents/flows/invoice"
	"github.com/lazybark/cents/flows/settings"
	"github.com/lazybark/cents/flows/subscription"
	"github.com/lazybark/cents/flows/tax"
	"gorm.io/gorm"
)

type ExportFormat string

const (
	ExportFormatJSON ExportFormat = "json"
	ExportFormatCSV  ExportFormat = "csv"
)

type ExportDataset string

const (
	ExportDatasetAll               ExportDataset = "all"
	ExportDatasetAccounts          ExportDataset = "accounts"
	ExportDatasetAccountValueLogs  ExportDataset = "account_value_logs"
	ExportDatasetCashflows         ExportDataset = "cashflows"
	ExportDatasetSubscriptions     ExportDataset = "subscriptions"
	ExportDatasetInvoices          ExportDataset = "invoices"
	ExportDatasetDebts             ExportDataset = "debts"
	ExportDatasetDebtLogs          ExportDataset = "debt_logs"
	ExportDatasetGoals             ExportDataset = "goals"
	ExportDatasetGoalLogs          ExportDataset = "goal_logs"
	ExportDatasetTaxes             ExportDataset = "taxes"
	ExportDatasetTaxLogs           ExportDataset = "tax_logs"
	ExportDatasetSettingRecords    ExportDataset = "setting_records"
	ExportDatasetCurrencies        ExportDataset = "currencies"
	ExportDatasetPaymentMethods    ExportDataset = "payment_methods"
	ExportDatasetTaxTypes          ExportDataset = "tax_types"
	ExportDatasetIncomeCategories  ExportDataset = "income_categories"
	ExportDatasetExpenseCategories ExportDataset = "expense_categories"
)

type ExportRequest struct {
	Dataset ExportDataset
	Format  ExportFormat
	Path    string
}

type ExportResult struct {
	Path      string
	FileCount int
}

type exportTable struct {
	dataset  ExportDataset
	label    string
	filename string
	load     func(*gorm.DB) (any, error)
}

type exportBundle struct {
	ExportedAt time.Time      `json:"exported_at"`
	Tables     map[string]any `json:"tables"`
}

func ExportFormatOptions() []ExportFormat {
	return []ExportFormat{ExportFormatJSON, ExportFormatCSV}
}

func (format ExportFormat) Label() string {
	switch format {
	case ExportFormatCSV:
		return "CSV"
	default:
		return "JSON"
	}
}

func ExportDatasetOptions() []ExportDataset {
	return []ExportDataset{
		ExportDatasetAll,
		ExportDatasetAccounts,
		ExportDatasetAccountValueLogs,
		ExportDatasetCashflows,
		ExportDatasetSubscriptions,
		ExportDatasetInvoices,
		ExportDatasetDebts,
		ExportDatasetDebtLogs,
		ExportDatasetGoals,
		ExportDatasetGoalLogs,
		ExportDatasetTaxes,
		ExportDatasetTaxLogs,
		ExportDatasetSettingRecords,
		ExportDatasetCurrencies,
		ExportDatasetPaymentMethods,
		ExportDatasetTaxTypes,
		ExportDatasetIncomeCategories,
		ExportDatasetExpenseCategories,
	}
}

func (dataset ExportDataset) Label() string {
	for _, table := range exportTables() {
		if table.dataset == dataset {
			return table.label
		}
	}
	if dataset == ExportDatasetAll {
		return "All data"
	}
	return string(dataset)
}

func ExportData(db *gorm.DB, request ExportRequest) (ExportResult, error) {
	if db == nil {
		return ExportResult{}, errors.New("database is not open")
	}

	if request.Dataset == "" {
		request.Dataset = ExportDatasetAll
	}
	if request.Format == "" {
		request.Format = ExportFormatJSON
	}

	switch request.Format {
	case ExportFormatJSON, ExportFormatCSV:
	default:
		return ExportResult{}, fmt.Errorf("unsupported export format %q", request.Format)
	}

	destination := strings.TrimSpace(request.Path)
	if destination == "" {
		var err error
		destination, err = os.Getwd()
		if err != nil {
			return ExportResult{}, err
		}
	}

	if request.Dataset == ExportDatasetAll {
		return exportAllData(db, request.Format, destination)
	}

	table, ok := findExportTable(request.Dataset)
	if !ok {
		return ExportResult{}, fmt.Errorf("unsupported export data %q", request.Dataset)
	}

	records, err := table.load(db)
	if err != nil {
		return ExportResult{}, err
	}

	switch request.Format {
	case ExportFormatJSON:
		path, err := resolveExportFile(destination, table.filename+".json")
		if err != nil {
			return ExportResult{}, err
		}
		if err := writeJSONFile(path, records); err != nil {
			return ExportResult{}, err
		}
		return ExportResult{Path: path, FileCount: 1}, nil
	case ExportFormatCSV:
		path, err := resolveExportFile(destination, table.filename+".csv")
		if err != nil {
			return ExportResult{}, err
		}
		if err := writeCSVFile(path, records); err != nil {
			return ExportResult{}, err
		}
		return ExportResult{Path: path, FileCount: 1}, nil
	default:
		return ExportResult{}, fmt.Errorf("unsupported export format %q", request.Format)
	}
}

func exportAllData(db *gorm.DB, format ExportFormat, destination string) (ExportResult, error) {
	timestamp := time.Now().Format("20060102_150405")

	if format == ExportFormatJSON {
		path, err := resolveExportFile(destination, "cents_export_"+timestamp+".json")
		if err != nil {
			return ExportResult{}, err
		}

		bundle := exportBundle{ExportedAt: time.Now(), Tables: map[string]any{}}
		for _, table := range exportTables() {
			records, err := table.load(db)
			if err != nil {
				return ExportResult{}, fmt.Errorf("%s export failed: %w", table.filename, err)
			}
			bundle.Tables[table.filename] = records
		}

		if err := writeJSONFile(path, bundle); err != nil {
			return ExportResult{}, err
		}
		return ExportResult{Path: path, FileCount: 1}, nil
	}

	dir, err := resolveExportDir(destination, "cents_export_"+timestamp)
	if err != nil {
		return ExportResult{}, err
	}

	fileCount := 0
	for _, table := range exportTables() {
		records, err := table.load(db)
		if err != nil {
			return ExportResult{}, fmt.Errorf("%s export failed: %w", table.filename, err)
		}
		if err := writeCSVFile(filepath.Join(dir, table.filename+".csv"), records); err != nil {
			return ExportResult{}, err
		}
		fileCount++
	}

	return ExportResult{Path: dir, FileCount: fileCount}, nil
}

func exportTables() []exportTable {
	return []exportTable{
		{dataset: ExportDatasetAccounts, label: "Accounts", filename: "accounts", load: loadExportRows[account.Account]},
		{dataset: ExportDatasetAccountValueLogs, label: "Account logs", filename: "account_value_logs", load: loadExportRows[account.AccountValueLog]},
		{dataset: ExportDatasetCashflows, label: "Cashflows", filename: "cashflows", load: loadExportRows[cashflow.CashflowEntry]},
		{dataset: ExportDatasetSubscriptions, label: "Subscriptions", filename: "subscriptions", load: loadExportRows[subscription.Subscription]},
		{dataset: ExportDatasetInvoices, label: "Invoices", filename: "invoices", load: loadExportRows[invoice.Invoice]},
		{dataset: ExportDatasetDebts, label: "Debts", filename: "debts", load: loadExportRows[debt.Debt]},
		{dataset: ExportDatasetDebtLogs, label: "Debt logs", filename: "debt_logs", load: loadExportRows[debt.DebtLog]},
		{dataset: ExportDatasetGoals, label: "Goals", filename: "goals", load: loadExportRows[goal.Goal]},
		{dataset: ExportDatasetGoalLogs, label: "Goal logs", filename: "goal_logs", load: loadExportRows[goal.GoalLog]},
		{dataset: ExportDatasetTaxes, label: "Taxes", filename: "taxes", load: loadExportRows[tax.Tax]},
		{dataset: ExportDatasetTaxLogs, label: "Tax logs", filename: "tax_logs", load: loadExportRows[tax.TaxLog]},
		{dataset: ExportDatasetSettingRecords, label: "Setting records", filename: "setting_records", load: loadExportRows[settings.SettingRecord]},
		{dataset: ExportDatasetCurrencies, label: "Currencies", filename: "currencies", load: loadExportRows[settings.SettingCurrency]},
		{dataset: ExportDatasetPaymentMethods, label: "Payment methods", filename: "payment_methods", load: loadExportRows[settings.SettingPaymentMethod]},
		{dataset: ExportDatasetTaxTypes, label: "Tax types", filename: "tax_types", load: loadExportRows[settings.SettingTaxType]},
		{dataset: ExportDatasetIncomeCategories, label: "Income categories", filename: "income_categories", load: loadExportRows[settings.SettingIncomeCategory]},
		{dataset: ExportDatasetExpenseCategories, label: "Expense categories", filename: "expense_categories", load: loadExportRows[settings.SettingExpenseCategory]},
	}
}

func findExportTable(dataset ExportDataset) (exportTable, bool) {
	for _, table := range exportTables() {
		if table.dataset == dataset {
			return table, true
		}
	}
	return exportTable{}, false
}

func loadExportRows[T any](db *gorm.DB) (any, error) {
	rows := make([]T, 0)
	if err := db.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func resolveExportFile(destination string, defaultName string) (string, error) {
	destination = expandHome(destination)
	if isDirectoryPath(destination) {
		destination = filepath.Join(destination, defaultName)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return "", err
	}
	return filepath.Abs(destination)
}

func resolveExportDir(destination string, defaultName string) (string, error) {
	destination = expandHome(destination)

	info, err := os.Stat(destination)
	switch {
	case err == nil && info.IsDir():
		destination = filepath.Join(destination, defaultName)
	case err == nil && !info.IsDir():
		return "", fmt.Errorf("%s is a file; choose a folder for all-data CSV export", destination)
	case os.IsNotExist(err):
	default:
		return "", err
	}

	if err := os.MkdirAll(destination, 0o755); err != nil {
		return "", err
	}
	return filepath.Abs(destination)
}

func isDirectoryPath(path string) bool {
	if strings.HasSuffix(path, string(os.PathSeparator)) {
		return true
	}
	if filepath.Ext(path) == "" {
		return true
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func writeJSONFile(path string, value any) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeCSVFile(path string, records any) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	headers, rows, err := csvRows(records)
	if err != nil {
		return err
	}
	if err := writer.Write(headers); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	return writer.Error()
}

func csvRows(records any) ([]string, [][]string, error) {
	value := reflect.ValueOf(records)
	if value.Kind() != reflect.Slice {
		return nil, nil, errors.New("csv export requires a slice")
	}

	itemType := value.Type().Elem()
	for itemType.Kind() == reflect.Pointer {
		itemType = itemType.Elem()
	}
	if itemType.Kind() != reflect.Struct {
		return nil, nil, errors.New("csv export requires a slice of structs")
	}

	headers := make([]string, 0, itemType.NumField())
	fieldIndexes := make([]int, 0, itemType.NumField())
	for i := 0; i < itemType.NumField(); i++ {
		field := itemType.Field(i)
		if field.PkgPath != "" {
			continue
		}
		headers = append(headers, field.Name)
		fieldIndexes = append(fieldIndexes, i)
	}

	rows := make([][]string, 0, value.Len())
	for i := 0; i < value.Len(); i++ {
		item := value.Index(i)
		for item.Kind() == reflect.Pointer {
			if item.IsNil() {
				break
			}
			item = item.Elem()
		}

		row := make([]string, 0, len(fieldIndexes))
		for _, fieldIndex := range fieldIndexes {
			row = append(row, exportCellValue(item.Field(fieldIndex)))
		}
		rows = append(rows, row)
	}

	return headers, rows, nil
}

func exportCellValue(value reflect.Value) string {
	if !value.IsValid() {
		return ""
	}

	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return ""
		}
		value = value.Elem()
	}

	if value.CanInterface() {
		if timestamp, ok := value.Interface().(time.Time); ok {
			if timestamp.IsZero() {
				return ""
			}
			return timestamp.Format(time.RFC3339)
		}
	}

	switch value.Kind() {
	case reflect.String:
		return value.String()
	case reflect.Bool:
		return strconv.FormatBool(value.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(value.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(value.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(value.Float(), 'f', -1, 64)
	default:
		data, err := json.Marshal(value.Interface())
		if err != nil {
			return fmt.Sprint(value.Interface())
		}
		return string(data)
	}
}
