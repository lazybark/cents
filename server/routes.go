package server

import (
	"net/http"

	"github.com/lazybark/cents/service"
)

// Kinds of setting list, by their path: /settings/{list}.
var settingLists = map[string]string{
	"currencies":         "currency",
	"payment-methods":    "payment_method",
	"tax-types":          "tax_type",
	"income-categories":  "income_category",
	"expense-categories": "expense_category",
	"tags":               "tag",
}

// Changed is how many records a call changed.
type Changed struct {
	Changed int `json:"changed"`
}

// Link is a web link.
type Link struct {
	URL string `json:"url"`
}

// NetWorthAmount is a net worth entered for a month.
type NetWorthAmount struct {
	Amount string `json:"amount"`
}

// NoteText is a month note.
type NoteText struct {
	Text string `json:"text"`
}

// MergeInto names the record to merge into.
type MergeInto struct {
	IntoID uint `json:"intoId"`
}

// RatesAuto turns daily rate updates on or off.
type RatesAuto struct {
	On bool `json:"on"`
}

// BackupRequest says where the client keeps the backup it downloads, to be
// remembered as the last backup; empty remembers nothing.
type BackupRequest struct {
	SavedTo string `json:"savedTo"`
}

// apiRoutes is the REST API, under /api/v1. Each route calls one Backend
// method; client.Client calls the same routes, so the two stay
// interchangeable.
func apiRoutes(b service.Backend, files service.Files) []route {
	return []route{
		// Info and analytics.
		get("/info", "What a client needs to know first.", nil, func(_ *http.Request, _ *params) (service.Info, error) {
			return b.Info()
		}).named("info"),
		get("/overview", "The home screen: summary, obligations, net worth and budgets.", nil, func(_ *http.Request, _ *params) (service.Overview, error) {
			return b.Overview()
		}).named("overview"),
		get("/analytics", "Analytics over the last months (default 12).", []string{"months:int"}, func(r *http.Request, p *params) (service.Analytics, error) {
			return b.Analytics(p.queryInt(r, "months", 12))
		}).named("analytics"),
		send("PUT", "/net-worth/{month}", "Keeps a net worth entered for a month (YYYY-MM).", func(_ *http.Request, p *params, in NetWorthAmount) (none, error) {
			return none{}, b.SetNetWorth(service.NetWorthInput{Month: p.path("month"), Amount: in.Amount})
		}).named("setNetWorth"),
		act("DELETE", "/net-worth/{month}", "Deletes a net worth entered for a month.", func(_ *http.Request, p *params) error {
			return b.DeleteNetWorth(p.path("month"))
		}).named("deleteNetWorth"),

		// Accounts.
		get("/accounts", "Accounts with totals; sort is an index into sortOptions.", []string{"sort:int"}, func(r *http.Request, p *params) (service.AccountsOverview, error) {
			return b.Accounts(p.queryInt(r, "sort", 0))
		}).named("accounts"),
		send("POST", "/accounts", "Adds an account.", func(_ *http.Request, _ *params, in service.NewAccountInput) (none, error) {
			return none{}, b.CreateAccount(in)
		}).created().named("createAccount"),
		send("PUT", "/accounts/{id}", "Updates an account's amount and flags.", func(_ *http.Request, p *params, in service.AmountUpdateInput) (service.AmountUpdateResult, error) {
			in.ID = p.id("id")
			return b.UpdateAccountAmount(in)
		}).named("updateAccountAmount"),
		act("DELETE", "/accounts/{id}", "Deletes an account.", func(_ *http.Request, p *params) error {
			return b.DeleteAccount(p.id("id"))
		}).named("deleteAccount"),
		get("/accounts/{id}/values", "An account's value history.", nil, func(_ *http.Request, p *params) ([]service.ValueLog, error) {
			return b.AccountValueLogs(p.id("id"))
		}).named("accountValueLogs"),
		send("POST", "/accounts/{id}/values", "Keeps an account's value on a day, replacing that day's.", func(_ *http.Request, p *params, in service.ValueLogInput) (none, error) {
			in.AccountID = p.id("id")
			return none{}, b.SaveAccountValueLog(in)
		}).created().named("saveAccountValueLog"),
		act("DELETE", "/accounts/{id}/values/{logId}", "Deletes a value from an account's history.", func(_ *http.Request, p *params) error {
			return b.DeleteAccountValueLog(p.id("id"), p.id("logId"))
		}).named("deleteAccountValueLog"),
		get("/accounts/{id}/months", "An account's value at the end of each month.", nil, func(_ *http.Request, p *params) ([]service.MonthPoint, error) {
			return b.AccountMonthlyValues(p.id("id"))
		}).named("accountMonthlyValues"),
		send("POST", "/accounts/{id}/merge", "Merges the account into another one.", func(_ *http.Request, p *params, in MergeInto) (service.MergeResult, error) {
			return b.Merge(service.MergeInput{Kind: "account", FromID: p.id("id"), IntoID: in.IntoID})
		}).named("mergeAccount"),

		// Property and investments.
		get("/assets", "Assets of a kind (property or investment).", []string{"kind"}, func(r *http.Request, p *params) (service.AssetsView, error) {
			return b.Assets(p.query(r, "kind"))
		}).named("assets"),
		send("POST", "/assets", "Adds an asset.", func(_ *http.Request, _ *params, in service.AssetInput) (service.AssetRow, error) {
			in.ID = 0
			return b.CreateAsset(in)
		}).created().named("createAsset"),
		send("PUT", "/assets/{id}", "Updates an asset.", func(_ *http.Request, p *params, in service.AssetInput) (service.AssetRow, error) {
			in.ID = p.id("id")
			return b.UpdateAsset(in)
		}).named("updateAsset"),
		act("DELETE", "/assets/{id}", "Deletes an asset.", func(_ *http.Request, p *params) error {
			return b.DeleteAsset(p.id("id"))
		}).named("deleteAsset"),
		get("/assets/{id}/values", "An asset's value history.", nil, func(_ *http.Request, p *params) ([]service.ValueLog, error) {
			return b.AssetValueLogs(p.id("id"))
		}).named("assetValueLogs"),
		send("POST", "/assets/{id}/values", "Keeps an asset's value on a day, replacing that day's.", func(_ *http.Request, p *params, in service.AssetValueLogInput) (none, error) {
			in.AssetID = p.id("id")
			return none{}, b.SaveAssetValueLog(in)
		}).created().named("saveAssetValueLog"),
		act("DELETE", "/assets/{id}/values/{logId}", "Deletes a value from an asset's history.", func(_ *http.Request, p *params) error {
			return b.DeleteAssetValueLog(p.id("id"), p.id("logId"))
		}).named("deleteAssetValueLog"),

		// Incomes and expenses.
		get("/cashflow/months", "Every month's totals (archived categories too, unless includeArchived=false).", []string{"includeArchived:bool"}, func(r *http.Request, p *params) (service.CashflowOverview, error) {
			return b.CashflowStats(p.queryBool(r, "includeArchived", true))
		}).named("cashflowStats"),
		get("/cashflow/months/{month}", "A month's entries (YYYY-MM, or current) with totals.", nil, func(_ *http.Request, p *params) (service.CashflowMonth, error) {
			return b.CashflowMonth(p.path("month"))
		}).named("cashflowMonth"),
		get("/cashflow/categories", "Categories of a kind (income or expense) with their totals.", []string{"kind", "includeArchived:bool"}, func(r *http.Request, p *params) (service.CashflowCategories, error) {
			return b.CashflowCategories(p.query(r, "kind"), p.queryBool(r, "includeArchived", false))
		}).named("cashflowCategories"),
		get("/cashflow/entries", "Entries matching every filter given.", queryFields(service.CashflowSearchInput{}), func(r *http.Request, p *params) (service.CashflowSearchResult, error) {
			var in service.CashflowSearchInput
			p.queryInto(r, &in)
			return b.CashflowSearch(in)
		}).named("cashflowSearch"),
		send("POST", "/cashflow/entries", "Adds an income or expense.", func(_ *http.Request, _ *params, in service.NewCashflowInput) (service.NewCashflowResult, error) {
			return b.CreateCashflow(in)
		}).created().named("createCashflow"),
		send("PATCH", "/cashflow/entries", "Changes several entries at once.", func(_ *http.Request, _ *params, in service.CashflowBulkInput) (Changed, error) {
			n, err := b.ChangeCashflows(in)
			return Changed{n}, err
		}).named("changeCashflows"),
		send("PUT", "/cashflow/entries/{id}", "Updates an entry.", func(_ *http.Request, p *params, in service.NewCashflowInput) (service.NewCashflowResult, error) {
			return b.UpdateCashflow(service.CashflowUpdateInput{ID: p.id("id"), NewCashflowInput: in})
		}).named("updateCashflow"),
		act("DELETE", "/cashflow/entries/{id}", "Deletes an entry.", func(_ *http.Request, p *params) error {
			return b.DeleteCashflow(p.id("id"))
		}).named("deleteCashflow"),
		send("PUT", "/cashflow/entries/{id}/rate", "Corrects an entry's rate (or its month's, in its currency).", func(_ *http.Request, p *params, in service.CashflowRateInput) (Changed, error) {
			in.ID = p.id("id")
			n, err := b.SetCashflowRate(in)
			return Changed{n}, err
		}).named("setCashflowRate"),
		get("/tags", "Every tag with its use.", nil, func(_ *http.Request, _ *params) (service.TagsView, error) {
			return b.CashflowTags()
		}).named("cashflowTags"),

		// Budgets and month notes.
		get("/budgets", "Budgets for a month (YYYY-MM, default this one).", []string{"month"}, func(r *http.Request, p *params) (service.BudgetsView, error) {
			return b.Budgets(p.query(r, "month"))
		}).named("budgets"),
		send("POST", "/budgets", "Adds a budget.", func(_ *http.Request, _ *params, in service.BudgetInput) (none, error) {
			in.ID = 0
			return none{}, b.SaveBudget(in)
		}).created().named("createBudget"),
		send("PUT", "/budgets/{id}", "Updates a budget.", func(_ *http.Request, p *params, in service.BudgetInput) (none, error) {
			in.ID = p.id("id")
			return none{}, b.SaveBudget(in)
		}).named("updateBudget"),
		act("DELETE", "/budgets/{id}", "Deletes a budget.", func(_ *http.Request, p *params) error {
			return b.DeleteBudget(p.id("id"))
		}).named("deleteBudget"),
		get("/notes", "Every month note, by month (YYYY-MM).", nil, func(_ *http.Request, _ *params) (map[string]string, error) {
			return b.MonthNotes()
		}).named("monthNotes"),
		send("PUT", "/notes/{month}", "Keeps a month's note; empty text deletes it.", func(_ *http.Request, p *params, in NoteText) (none, error) {
			return none{}, b.SaveMonthNote(service.MonthNoteInput{Month: p.path("month"), Text: in.Text})
		}).named("saveMonthNote"),

		// Credits.
		get("/credits", "Credits: active or paid off.", []string{"mode"}, func(r *http.Request, p *params) (service.CreditsView, error) {
			return b.Credits(p.query(r, "mode"))
		}).named("credits"),
		send("POST", "/credits", "Adds a credit.", func(_ *http.Request, _ *params, in service.CreditInput) (service.CreatedIn, error) {
			in.ID = 0
			return b.CreateCredit(in)
		}).created().named("createCredit"),
		send("PUT", "/credits/{id}", "Updates a credit.", func(_ *http.Request, p *params, in service.CreditInput) (service.CreatedIn, error) {
			in.ID = p.id("id")
			return b.UpdateCredit(in)
		}).named("updateCredit"),
		act("DELETE", "/credits/{id}", "Deletes a credit.", func(_ *http.Request, p *params) error {
			return b.DeleteCredit(p.id("id"))
		}).named("deleteCredit"),
		get("/credits/{id}/logs", "A credit's payments and changes.", nil, func(_ *http.Request, p *params) ([]service.CreditLogRow, error) {
			return b.CreditLogs(p.id("id"))
		}).named("creditLogs"),
		send("POST", "/credits/{id}/logs", "Logs a payment or change on a credit.", func(_ *http.Request, p *params, in service.CreditLogInput) (service.CreditRow, error) {
			in.ID = p.id("id")
			return b.AddCreditLog(in)
		}).created().named("addCreditLog"),
		send("DELETE", "/credits/{id}/logs/{logId}", "Deletes a credit's logged payment or change.", func(_ *http.Request, p *params, _ none) (service.CreditRow, error) {
			return b.DeleteCreditLog(p.id("id"), p.id("logId"))
		}).noBody().named("deleteCreditLog"),

		// Debts.
		get("/debts", "Debts: outgoing, incoming or paid.", []string{"mode"}, func(r *http.Request, p *params) (service.DebtsView, error) {
			return b.Debts(p.query(r, "mode"))
		}).named("debts"),
		send("POST", "/debts", "Adds a debt.", func(_ *http.Request, _ *params, in service.NewDebtInput) (service.CreatedIn, error) {
			return b.CreateDebt(in)
		}).created().named("createDebt"),
		send("PUT", "/debts/{id}", "Updates a debt.", func(_ *http.Request, p *params, in service.DebtUpdateInput) (none, error) {
			in.ID = p.id("id")
			return none{}, b.UpdateDebt(in)
		}).named("updateDebt"),
		act("DELETE", "/debts/{id}", "Deletes a debt.", func(_ *http.Request, p *params) error {
			return b.DeleteDebt(p.id("id"))
		}).named("deleteDebt"),
		get("/debts/{id}/payments", "A debt's payments.", nil, func(_ *http.Request, p *params) ([]service.PaymentLog, error) {
			return b.DebtLogs(p.id("id"))
		}).named("debtLogs"),
		send("POST", "/debts/{id}/payments", "Logs a payment on a debt.", func(_ *http.Request, p *params, in service.PaymentInput) (service.DebtRow, error) {
			in.ID = p.id("id")
			return b.AddDebtPayment(in)
		}).created().named("addDebtPayment"),
		send("DELETE", "/debts/{id}/payments/{logId}", "Deletes a debt's payment.", func(_ *http.Request, p *params, _ none) (service.DebtRow, error) {
			return b.DeleteDebtPayment(p.id("id"), p.id("logId"))
		}).noBody().named("deleteDebtPayment"),

		// Goals.
		get("/goals", "Goals: active or done.", []string{"mode"}, func(r *http.Request, p *params) (service.GoalsView, error) {
			return b.Goals(p.query(r, "mode"))
		}).named("goals"),
		send("POST", "/goals", "Adds a goal.", func(_ *http.Request, _ *params, in service.NewGoalInput) (service.CreatedIn, error) {
			return b.CreateGoal(in)
		}).created().named("createGoal"),
		send("PUT", "/goals/{id}", "Updates a goal.", func(_ *http.Request, p *params, in service.GoalUpdateInput) (none, error) {
			in.ID = p.id("id")
			return none{}, b.UpdateGoal(in)
		}).named("updateGoal"),
		act("DELETE", "/goals/{id}", "Deletes a goal.", func(_ *http.Request, p *params) error {
			return b.DeleteGoal(p.id("id"))
		}).named("deleteGoal"),
		get("/goals/{id}/changes", "A goal's logged changes.", nil, func(_ *http.Request, p *params) ([]service.PaymentLog, error) {
			return b.GoalLogs(p.id("id"))
		}).named("goalLogs"),
		send("POST", "/goals/{id}/changes", "Logs a change on a goal.", func(_ *http.Request, p *params, in service.PaymentInput) (service.GoalRow, error) {
			in.ID = p.id("id")
			return b.AddGoalChange(in)
		}).created().named("addGoalChange"),
		send("DELETE", "/goals/{id}/changes/{logId}", "Deletes a goal's logged change.", func(_ *http.Request, p *params, _ none) (service.GoalRow, error) {
			return b.DeleteGoalChange(p.id("id"), p.id("logId"))
		}).noBody().named("deleteGoalChange"),

		// Taxes.
		get("/taxes", "Taxes: unpaid or paid.", []string{"mode"}, func(r *http.Request, p *params) (service.TaxesView, error) {
			return b.Taxes(p.query(r, "mode"))
		}).named("taxes"),
		send("POST", "/taxes", "Adds a tax.", func(_ *http.Request, _ *params, in service.NewTaxInput) (service.CreatedIn, error) {
			return b.CreateTax(in)
		}).created().named("createTax"),
		send("PUT", "/taxes/{id}", "Updates a tax.", func(_ *http.Request, p *params, in service.TaxUpdateInput) (none, error) {
			in.ID = p.id("id")
			return none{}, b.UpdateTax(in)
		}).named("updateTax"),
		act("DELETE", "/taxes/{id}", "Deletes a tax.", func(_ *http.Request, p *params) error {
			return b.DeleteTax(p.id("id"))
		}).named("deleteTax"),
		get("/taxes/{id}/payments", "A tax's payments.", nil, func(_ *http.Request, p *params) ([]service.PaymentLog, error) {
			return b.TaxLogs(p.id("id"))
		}).named("taxLogs"),
		send("POST", "/taxes/{id}/payments", "Logs a payment on a tax.", func(_ *http.Request, p *params, in service.PaymentInput) (service.TaxRow, error) {
			in.ID = p.id("id")
			return b.AddTaxPayment(in)
		}).created().named("addTaxPayment"),

		// Invoices.
		get("/invoices", "Invoices: outgoing, incoming or paid.", []string{"mode"}, func(r *http.Request, p *params) (service.InvoicesView, error) {
			return b.Invoices(p.query(r, "mode"))
		}).named("invoices"),
		send("POST", "/invoices", "Adds an invoice.", func(_ *http.Request, _ *params, in service.InvoiceInput) (service.CreatedIn, error) {
			in.ID = 0
			return b.CreateInvoice(in)
		}).created().named("createInvoice"),
		send("PUT", "/invoices/{id}", "Updates an invoice.", func(_ *http.Request, p *params, in service.InvoiceInput) (service.CreatedIn, error) {
			in.ID = p.id("id")
			return b.UpdateInvoice(in)
		}).named("updateInvoice"),
		act("DELETE", "/invoices/{id}", "Deletes an invoice.", func(_ *http.Request, p *params) error {
			return b.DeleteInvoice(p.id("id"))
		}).named("deleteInvoice"),
		get("/invoices/{id}/link", "An invoice's web link.", nil, func(_ *http.Request, p *params) (Link, error) {
			url, err := b.InvoiceLink(p.id("id"))
			return Link{url}, err
		}).named("invoiceLink"),

		// Subscriptions and obligations.
		get("/subscriptions", "Regular payments: mode active or all; kind subscription or obligation.", []string{"mode", "kind"}, func(r *http.Request, p *params) (service.SubscriptionsView, error) {
			return b.Subscriptions(p.query(r, "mode"), p.query(r, "kind"))
		}).named("subscriptions"),
		send("POST", "/subscriptions", "Adds a regular payment.", func(_ *http.Request, _ *params, in service.SubscriptionInput) (none, error) {
			in.ID = 0
			return none{}, b.CreateSubscription(in)
		}).created().named("createSubscription"),
		send("PUT", "/subscriptions/{id}", "Updates a regular payment.", func(_ *http.Request, p *params, in service.SubscriptionInput) (none, error) {
			in.ID = p.id("id")
			return none{}, b.UpdateSubscription(in)
		}).named("updateSubscription"),
		act("DELETE", "/subscriptions/{id}", "Deletes a regular payment.", func(_ *http.Request, p *params) error {
			return b.DeleteSubscription(p.id("id"))
		}).named("deleteSubscription"),
		get("/subscriptions/{id}/payments", "Payments marked paid.", nil, func(_ *http.Request, p *params) ([]service.SubscriptionPaymentRow, error) {
			return b.SubscriptionPayments(p.id("id"))
		}).named("subscriptionPayments"),
		send("POST", "/subscriptions/{id}/payments", "Marks the next payment paid.", func(_ *http.Request, p *params, _ none) (service.SubscriptionRow, error) {
			return b.MarkSubscriptionPaid(p.id("id"))
		}).noBody().created().named("markSubscriptionPaid"),
		send("DELETE", "/subscriptions/{id}/payments/{paymentId}", "Unmarks a payment.", func(_ *http.Request, p *params, _ none) (service.SubscriptionRow, error) {
			return b.DeleteSubscriptionPayment(p.id("id"), p.id("paymentId"))
		}).noBody().named("deleteSubscriptionPayment"),

		// Settings.
		get("/settings", "Every setting.", nil, func(_ *http.Request, _ *params) (service.SettingsView, error) {
			return b.Settings()
		}).named("settings"),
		send("PUT", "/settings/base-currency", "Changes the base currency.", func(_ *http.Request, _ *params, in service.BaseCurrencyInput) (none, error) {
			return none{}, b.SaveBaseCurrency(in)
		}).named("saveBaseCurrency"),
		send("POST", "/settings/currencies", "Adds a currency.", func(_ *http.Request, _ *params, in service.CurrencyInput) (none, error) {
			in.ID = 0
			return none{}, b.SaveCurrency(in)
		}).created().named("createCurrency"),
		send("PUT", "/settings/currencies/{id}", "Updates a currency.", func(_ *http.Request, p *params, in service.CurrencyInput) (none, error) {
			in.ID = p.id("id")
			return none{}, b.SaveCurrency(in)
		}).named("updateCurrency"),
		send("POST", "/settings/payment-methods", "Adds a payment method.", func(_ *http.Request, _ *params, in service.PaymentMethodInput) (none, error) {
			in.ID = 0
			return none{}, b.SavePaymentMethod(in)
		}).created().named("createPaymentMethod"),
		send("PUT", "/settings/payment-methods/{id}", "Updates a payment method.", func(_ *http.Request, p *params, in service.PaymentMethodInput) (none, error) {
			in.ID = p.id("id")
			return none{}, b.SavePaymentMethod(in)
		}).named("updatePaymentMethod"),
		send("POST", "/settings/tax-types", "Adds a tax type.", func(_ *http.Request, _ *params, in service.TaxTypeInput) (none, error) {
			in.ID = 0
			return none{}, b.SaveTaxType(in)
		}).created().named("createTaxType"),
		send("PUT", "/settings/tax-types/{id}", "Updates a tax type.", func(_ *http.Request, p *params, in service.TaxTypeInput) (none, error) {
			in.ID = p.id("id")
			return none{}, b.SaveTaxType(in)
		}).named("updateTaxType"),
		send("POST", "/settings/income-categories", "Adds an income category.", category(b, true, false)).created().named("createIncomeCategory"),
		send("PUT", "/settings/income-categories/{id}", "Updates an income category.", category(b, true, true)).named("updateIncomeCategory"),
		send("POST", "/settings/expense-categories", "Adds an expense category.", category(b, false, false)).created().named("createExpenseCategory"),
		send("PUT", "/settings/expense-categories/{id}", "Updates an expense category.", category(b, false, true)).named("updateExpenseCategory"),
		send("POST", "/settings/tags", "Adds a tag.", func(_ *http.Request, _ *params, in service.TagInput) (none, error) {
			in.ID = 0
			return none{}, b.SaveTag(in)
		}).created().named("createTag"),
		send("PUT", "/settings/tags/{id}", "Renames or archives a tag.", func(_ *http.Request, p *params, in service.TagInput) (none, error) {
			in.ID = p.id("id")
			return none{}, b.SaveTag(in)
		}).named("updateTag"),
		act("DELETE", "/settings/{list}/{id}", "Deletes a setting (refused while records use it).", func(_ *http.Request, p *params) error {
			return b.DeleteSetting(p.list(), p.id("id"))
		}).named("deleteSetting"),
		send("POST", "/settings/{list}/{id}/merge", "Merges a setting into another of its list.", func(_ *http.Request, p *params, in MergeInto) (service.MergeResult, error) {
			return b.Merge(service.MergeInput{Kind: p.list(), FromID: p.id("id"), IntoID: in.IntoID})
		}).named("mergeSetting"),

		// Currencies and rates.
		get("/currency-catalog", "Every currency that can be picked.", nil, func(_ *http.Request, _ *params) ([]service.CatalogCurrency, error) {
			return b.CurrencyCatalog()
		}).named("currencyCatalog"),
		send("POST", "/currency-setup", "Sets up a new database's currencies.", func(_ *http.Request, _ *params, in service.CurrencySetupInput) (service.RatesStatus, error) {
			return b.SetupCurrencies(in)
		}).named("setupCurrencies"),
		act("DELETE", "/currency-setup", "Skips setting up currencies.", func(_ *http.Request, _ *params) error {
			return b.SkipCurrencySetup()
		}).named("skipCurrencySetup"),
		get("/rates", "How exchange rates are kept up to date.", nil, func(_ *http.Request, _ *params) (service.RatesStatus, error) {
			return b.RatesStatus()
		}).named("ratesStatus"),
		send("POST", "/rates/refresh", "Fetches exchange rates now.", func(_ *http.Request, _ *params, _ none) (service.RatesStatus, error) {
			return b.RefreshRates()
		}).noBody().named("refreshRates"),
		send("PUT", "/rates/auto", "Turns daily rate updates on or off.", func(_ *http.Request, _ *params, in RatesAuto) (service.RatesStatus, error) {
			return b.SetRatesAuto(in.On)
		}).named("setRatesAuto"),

		// Backups and exports.
		get("/export", "What can be exported, and the last backup.", nil, func(_ *http.Request, _ *params) (service.ExportView, error) {
			return b.ExportOptions()
		}).named("exportOptions"),
		download("GET", "/export/{dataset}", "A dataset as CSV, for a period (YYYY-MM-DD, either may be left out).", []string{"from", "to"}, "text/csv; charset=utf-8", func(w *payload, r *http.Request, p *params) error {
			rows, err := files.WriteCSV(w, p.path("dataset"), p.query(r, "from"), p.query(r, "to"))
			w.header("Cents-Rows", rows)
			w.attachment(p.path("dataset") + ".csv")
			return err
		}).named("exportCSV"),
		downloadWith("POST", "/backup", "A copy of the whole database, encrypted like it.", "application/vnd.sqlite3", func(w *payload, _ *http.Request, _ *params, in BackupRequest) error {
			result, err := files.WriteBackup(w, in.SavedTo)
			w.header("Cents-Backup-At", result.At)
			w.attachment("cents-backup.db")
			return err
		}).named("backup"),
	}
}

func category(b service.Backend, income, existing bool) func(*http.Request, *params, service.CategoryInput) (none, error) {
	return func(_ *http.Request, p *params, in service.CategoryInput) (none, error) {
		in.IsIncome, in.ID = income, 0
		if existing {
			in.ID = p.id("id")
		}

		return none{}, b.SaveCategory(in)
	}
}
