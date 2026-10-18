package service

import (
	"io"
	"sync"
)

// Switch is a Backend (and Files) that passes every call on to the one it's
// switched to: the desktop app switches between a database and a server
// while it runs. The methods below only pass calls on; TestSwitch checks
// each calls its own name.
type Switch struct {
	mu      sync.RWMutex
	backend Backend
	files   Files
}

var (
	_ Backend = (*Switch)(nil)
	_ Files   = (*Switch)(nil)
)

// NewSwitch is a switch to b, with files.
func NewSwitch(b Backend, files Files) *Switch {
	return &Switch{backend: b, files: files}
}

// To switches to b, with files. Calls already made finish on the one before.
func (s *Switch) To(b Backend, files Files) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.backend, s.files = b, files
}

func (s *Switch) current() Backend {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.backend
}

func (s *Switch) currentFiles() Files {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.files
}

func (s *Switch) Info() (Info, error) {
	return s.current().Info()
}

func (s *Switch) Overview() (Overview, error) {
	return s.current().Overview()
}

func (s *Switch) Analytics(months int) (Analytics, error) {
	return s.current().Analytics(months)
}

func (s *Switch) SetNetWorth(input NetWorthInput) error {
	return s.current().SetNetWorth(input)
}

func (s *Switch) DeleteNetWorth(month string) error {
	return s.current().DeleteNetWorth(month)
}

func (s *Switch) Accounts(sort int) (AccountsOverview, error) {
	return s.current().Accounts(sort)
}

func (s *Switch) CreateAccount(input NewAccountInput) error {
	return s.current().CreateAccount(input)
}

func (s *Switch) UpdateAccountAmount(input AmountUpdateInput) (AmountUpdateResult, error) {
	return s.current().UpdateAccountAmount(input)
}

func (s *Switch) DeleteAccount(id uint) error {
	return s.current().DeleteAccount(id)
}

func (s *Switch) AccountValueLogs(accountID uint) ([]ValueLog, error) {
	return s.current().AccountValueLogs(accountID)
}

func (s *Switch) SaveAccountValueLog(input ValueLogInput) error {
	return s.current().SaveAccountValueLog(input)
}

func (s *Switch) DeleteAccountValueLog(accountID uint, logID uint) error {
	return s.current().DeleteAccountValueLog(accountID, logID)
}

func (s *Switch) AccountMonthlyValues(accountID uint) ([]MonthPoint, error) {
	return s.current().AccountMonthlyValues(accountID)
}

func (s *Switch) Assets(kind string) (AssetsView, error) {
	return s.current().Assets(kind)
}

func (s *Switch) CreateAsset(input AssetInput) (AssetRow, error) {
	return s.current().CreateAsset(input)
}

func (s *Switch) UpdateAsset(input AssetInput) (AssetRow, error) {
	return s.current().UpdateAsset(input)
}

func (s *Switch) DeleteAsset(id uint) error {
	return s.current().DeleteAsset(id)
}

func (s *Switch) AssetValueLogs(assetID uint) ([]ValueLog, error) {
	return s.current().AssetValueLogs(assetID)
}

func (s *Switch) SaveAssetValueLog(input AssetValueLogInput) error {
	return s.current().SaveAssetValueLog(input)
}

func (s *Switch) DeleteAssetValueLog(assetID uint, logID uint) error {
	return s.current().DeleteAssetValueLog(assetID, logID)
}

func (s *Switch) CashflowMonth(month string) (CashflowMonth, error) {
	return s.current().CashflowMonth(month)
}

func (s *Switch) CashflowOverview() (CashflowOverview, error) {
	return s.current().CashflowOverview()
}

func (s *Switch) CashflowStats(includeArchived bool) (CashflowOverview, error) {
	return s.current().CashflowStats(includeArchived)
}

func (s *Switch) CreateCashflow(input NewCashflowInput) (NewCashflowResult, error) {
	return s.current().CreateCashflow(input)
}

func (s *Switch) UpdateCashflow(input CashflowUpdateInput) (NewCashflowResult, error) {
	return s.current().UpdateCashflow(input)
}

func (s *Switch) SetCashflowRate(input CashflowRateInput) (int, error) {
	return s.current().SetCashflowRate(input)
}

func (s *Switch) DeleteCashflow(id uint) error {
	return s.current().DeleteCashflow(id)
}

func (s *Switch) CashflowCategories(kind string, includeArchived bool) (CashflowCategories, error) {
	return s.current().CashflowCategories(kind, includeArchived)
}

func (s *Switch) CashflowSearch(input CashflowSearchInput) (CashflowSearchResult, error) {
	return s.current().CashflowSearch(input)
}

func (s *Switch) ChangeCashflows(input CashflowBulkInput) (int, error) {
	return s.current().ChangeCashflows(input)
}

func (s *Switch) CashflowTags() (TagsView, error) {
	return s.current().CashflowTags()
}

func (s *Switch) SaveTag(input TagInput) error {
	return s.current().SaveTag(input)
}

func (s *Switch) Budgets(month string) (BudgetsView, error) {
	return s.current().Budgets(month)
}

func (s *Switch) SaveBudget(input BudgetInput) error {
	return s.current().SaveBudget(input)
}

func (s *Switch) DeleteBudget(id uint) error {
	return s.current().DeleteBudget(id)
}

func (s *Switch) MonthNotes() (map[string]string, error) {
	return s.current().MonthNotes()
}

func (s *Switch) SaveMonthNote(input MonthNoteInput) error {
	return s.current().SaveMonthNote(input)
}

func (s *Switch) Credits(mode string) (CreditsView, error) {
	return s.current().Credits(mode)
}

func (s *Switch) CreateCredit(input CreditInput) (CreatedIn, error) {
	return s.current().CreateCredit(input)
}

func (s *Switch) UpdateCredit(input CreditInput) (CreatedIn, error) {
	return s.current().UpdateCredit(input)
}

func (s *Switch) DeleteCredit(id uint) error {
	return s.current().DeleteCredit(id)
}

func (s *Switch) CreditLogs(id uint) ([]CreditLogRow, error) {
	return s.current().CreditLogs(id)
}

func (s *Switch) AddCreditLog(input CreditLogInput) (CreditRow, error) {
	return s.current().AddCreditLog(input)
}

func (s *Switch) DeleteCreditLog(creditID uint, logID uint) (CreditRow, error) {
	return s.current().DeleteCreditLog(creditID, logID)
}

func (s *Switch) Debts(mode string) (DebtsView, error) {
	return s.current().Debts(mode)
}

func (s *Switch) CreateDebt(input NewDebtInput) (CreatedIn, error) {
	return s.current().CreateDebt(input)
}

func (s *Switch) UpdateDebt(input DebtUpdateInput) error {
	return s.current().UpdateDebt(input)
}

func (s *Switch) DebtLogs(id uint) ([]PaymentLog, error) {
	return s.current().DebtLogs(id)
}

func (s *Switch) AddDebtPayment(input PaymentInput) (DebtRow, error) {
	return s.current().AddDebtPayment(input)
}

func (s *Switch) DeleteDebtPayment(debtID uint, logID uint) (DebtRow, error) {
	return s.current().DeleteDebtPayment(debtID, logID)
}

func (s *Switch) DeleteDebt(id uint) error {
	return s.current().DeleteDebt(id)
}

func (s *Switch) Goals(mode string) (GoalsView, error) {
	return s.current().Goals(mode)
}

func (s *Switch) CreateGoal(input NewGoalInput) (CreatedIn, error) {
	return s.current().CreateGoal(input)
}

func (s *Switch) UpdateGoal(input GoalUpdateInput) error {
	return s.current().UpdateGoal(input)
}

func (s *Switch) GoalLogs(id uint) ([]PaymentLog, error) {
	return s.current().GoalLogs(id)
}

func (s *Switch) AddGoalChange(input PaymentInput) (GoalRow, error) {
	return s.current().AddGoalChange(input)
}

func (s *Switch) DeleteGoalChange(goalID uint, logID uint) (GoalRow, error) {
	return s.current().DeleteGoalChange(goalID, logID)
}

func (s *Switch) DeleteGoal(id uint) error {
	return s.current().DeleteGoal(id)
}

func (s *Switch) Taxes(mode string) (TaxesView, error) {
	return s.current().Taxes(mode)
}

func (s *Switch) CreateTax(input NewTaxInput) (CreatedIn, error) {
	return s.current().CreateTax(input)
}

func (s *Switch) UpdateTax(input TaxUpdateInput) error {
	return s.current().UpdateTax(input)
}

func (s *Switch) TaxLogs(id uint) ([]PaymentLog, error) {
	return s.current().TaxLogs(id)
}

func (s *Switch) AddTaxPayment(input PaymentInput) (TaxRow, error) {
	return s.current().AddTaxPayment(input)
}

func (s *Switch) DeleteTax(id uint) error {
	return s.current().DeleteTax(id)
}

func (s *Switch) Invoices(mode string) (InvoicesView, error) {
	return s.current().Invoices(mode)
}

func (s *Switch) CreateInvoice(input InvoiceInput) (CreatedIn, error) {
	return s.current().CreateInvoice(input)
}

func (s *Switch) UpdateInvoice(input InvoiceInput) (CreatedIn, error) {
	return s.current().UpdateInvoice(input)
}

func (s *Switch) DeleteInvoice(id uint) error {
	return s.current().DeleteInvoice(id)
}

func (s *Switch) InvoiceLink(id uint) (string, error) {
	return s.current().InvoiceLink(id)
}

func (s *Switch) Subscriptions(mode string, kind string) (SubscriptionsView, error) {
	return s.current().Subscriptions(mode, kind)
}

func (s *Switch) CreateSubscription(input SubscriptionInput) error {
	return s.current().CreateSubscription(input)
}

func (s *Switch) UpdateSubscription(input SubscriptionInput) error {
	return s.current().UpdateSubscription(input)
}

func (s *Switch) MarkSubscriptionPaid(id uint) (SubscriptionRow, error) {
	return s.current().MarkSubscriptionPaid(id)
}

func (s *Switch) SubscriptionPayments(id uint) ([]SubscriptionPaymentRow, error) {
	return s.current().SubscriptionPayments(id)
}

func (s *Switch) DeleteSubscriptionPayment(subscriptionID uint, paymentID uint) (SubscriptionRow, error) {
	return s.current().DeleteSubscriptionPayment(subscriptionID, paymentID)
}

func (s *Switch) DeleteSubscription(id uint) error {
	return s.current().DeleteSubscription(id)
}

func (s *Switch) Settings() (SettingsView, error) {
	return s.current().Settings()
}

func (s *Switch) SaveBaseCurrency(input BaseCurrencyInput) error {
	return s.current().SaveBaseCurrency(input)
}

func (s *Switch) SaveCurrency(input CurrencyInput) error {
	return s.current().SaveCurrency(input)
}

func (s *Switch) SavePaymentMethod(input PaymentMethodInput) error {
	return s.current().SavePaymentMethod(input)
}

func (s *Switch) SaveTaxType(input TaxTypeInput) error {
	return s.current().SaveTaxType(input)
}

func (s *Switch) SaveCategory(input CategoryInput) error {
	return s.current().SaveCategory(input)
}

func (s *Switch) DeleteSetting(kind string, id uint) error {
	return s.current().DeleteSetting(kind, id)
}

func (s *Switch) Merge(input MergeInput) (MergeResult, error) {
	return s.current().Merge(input)
}

func (s *Switch) CurrencyCatalog() ([]CatalogCurrency, error) {
	return s.current().CurrencyCatalog()
}

func (s *Switch) RatesStatus() (RatesStatus, error) {
	return s.current().RatesStatus()
}

func (s *Switch) RefreshRates() (RatesStatus, error) {
	return s.current().RefreshRates()
}

func (s *Switch) SetRatesAuto(on bool) (RatesStatus, error) {
	return s.current().SetRatesAuto(on)
}

func (s *Switch) SetupCurrencies(input CurrencySetupInput) (RatesStatus, error) {
	return s.current().SetupCurrencies(input)
}

func (s *Switch) SkipCurrencySetup() error {
	return s.current().SkipCurrencySetup()
}

func (s *Switch) ExportOptions() (ExportView, error) {
	return s.current().ExportOptions()
}

func (s *Switch) WriteBackup(w io.Writer, savedTo string) (BackupResult, error) {
	return s.currentFiles().WriteBackup(w, savedTo)
}

func (s *Switch) WriteCSV(w io.Writer, dataset, from, to string) (int, error) {
	return s.currentFiles().WriteCSV(w, dataset, from, to)
}
