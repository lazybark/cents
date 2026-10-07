package service

// Backend is every call a client can make, with JSON-friendly inputs and
// results. Service implements it on a database and client.Client on a
// cents server (whose REST API maps onto it), so a client written against
// Backend works with either.
type Backend interface {
	Info() (Info, error)
	Overview() (Overview, error)
	Analytics(months int) (Analytics, error)
	SetNetWorth(input NetWorthInput) error
	DeleteNetWorth(month string) error

	Accounts(sort int) (AccountsOverview, error)
	CreateAccount(input NewAccountInput) error
	UpdateAccountAmount(input AmountUpdateInput) (AmountUpdateResult, error)
	DeleteAccount(id uint) error
	AccountValueLogs(accountID uint) ([]ValueLog, error)
	SaveAccountValueLog(input ValueLogInput) error
	DeleteAccountValueLog(accountID uint, logID uint) error
	AccountMonthlyValues(accountID uint) ([]MonthPoint, error)

	Assets(kind string) (AssetsView, error)
	CreateAsset(input AssetInput) (AssetRow, error)
	UpdateAsset(input AssetInput) (AssetRow, error)
	DeleteAsset(id uint) error
	AssetValueLogs(assetID uint) ([]ValueLog, error)
	SaveAssetValueLog(input AssetValueLogInput) error
	DeleteAssetValueLog(assetID uint, logID uint) error

	CashflowMonth(month string) (CashflowMonth, error)
	CashflowOverview() (CashflowOverview, error)
	CashflowStats(includeArchived bool) (CashflowOverview, error)
	CreateCashflow(input NewCashflowInput) (NewCashflowResult, error)
	UpdateCashflow(input CashflowUpdateInput) (NewCashflowResult, error)
	SetCashflowRate(input CashflowRateInput) (int, error)
	DeleteCashflow(id uint) error
	CashflowCategories(kind string, includeArchived bool) (CashflowCategories, error)
	CashflowSearch(input CashflowSearchInput) (CashflowSearchResult, error)
	ChangeCashflows(input CashflowBulkInput) (int, error)
	CashflowTags() (TagsView, error)
	SaveTag(input TagInput) error

	Budgets(month string) (BudgetsView, error)
	SaveBudget(input BudgetInput) error
	DeleteBudget(id uint) error

	MonthNotes() (map[string]string, error)
	SaveMonthNote(input MonthNoteInput) error

	Credits(mode string) (CreditsView, error)
	CreateCredit(input CreditInput) (CreatedIn, error)
	UpdateCredit(input CreditInput) (CreatedIn, error)
	DeleteCredit(id uint) error
	CreditLogs(id uint) ([]CreditLogRow, error)
	AddCreditLog(input CreditLogInput) (CreditRow, error)
	DeleteCreditLog(creditID uint, logID uint) (CreditRow, error)

	Debts(mode string) (DebtsView, error)
	CreateDebt(input NewDebtInput) (CreatedIn, error)
	UpdateDebt(input DebtUpdateInput) error
	DebtLogs(id uint) ([]PaymentLog, error)
	AddDebtPayment(input PaymentInput) (DebtRow, error)
	DeleteDebtPayment(debtID uint, logID uint) (DebtRow, error)
	DeleteDebt(id uint) error

	Goals(mode string) (GoalsView, error)
	CreateGoal(input NewGoalInput) (CreatedIn, error)
	UpdateGoal(input GoalUpdateInput) error
	GoalLogs(id uint) ([]PaymentLog, error)
	AddGoalChange(input PaymentInput) (GoalRow, error)
	DeleteGoalChange(goalID uint, logID uint) (GoalRow, error)
	DeleteGoal(id uint) error

	Taxes(mode string) (TaxesView, error)
	CreateTax(input NewTaxInput) (CreatedIn, error)
	UpdateTax(input TaxUpdateInput) error
	TaxLogs(id uint) ([]PaymentLog, error)
	AddTaxPayment(input PaymentInput) (TaxRow, error)
	DeleteTax(id uint) error

	Invoices(mode string) (InvoicesView, error)
	CreateInvoice(input InvoiceInput) (CreatedIn, error)
	UpdateInvoice(input InvoiceInput) (CreatedIn, error)
	DeleteInvoice(id uint) error
	InvoiceLink(id uint) (string, error)

	Subscriptions(mode string, kind string) (SubscriptionsView, error)
	CreateSubscription(input SubscriptionInput) error
	UpdateSubscription(input SubscriptionInput) error
	MarkSubscriptionPaid(id uint) (SubscriptionRow, error)
	SubscriptionPayments(id uint) ([]SubscriptionPaymentRow, error)
	DeleteSubscriptionPayment(subscriptionID uint, paymentID uint) (SubscriptionRow, error)
	DeleteSubscription(id uint) error

	Settings() (SettingsView, error)
	SaveBaseCurrency(input BaseCurrencyInput) error
	SaveCurrency(input CurrencyInput) error
	SavePaymentMethod(input PaymentMethodInput) error
	SaveTaxType(input TaxTypeInput) error
	SaveCategory(input CategoryInput) error
	DeleteSetting(kind string, id uint) error
	Merge(input MergeInput) (MergeResult, error)

	CurrencyCatalog() ([]CatalogCurrency, error)
	RatesStatus() (RatesStatus, error)
	RefreshRates() (RatesStatus, error)
	SetRatesAuto(on bool) (RatesStatus, error)
	SetupCurrencies(input CurrencySetupInput) (RatesStatus, error)
	SkipCurrencySetup() error

	ExportOptions() (ExportView, error)
}
