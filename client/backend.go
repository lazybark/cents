package client

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/lazybark/cents/service"
)

// read gets path's JSON result.
func read[T any](c *Client, path string, query url.Values) (T, error) {
	var out T
	err := c.get(path, query, &out)
	return out, err
}

// write sends in (nil for no body) and reads the JSON result.
func write[T any](c *Client, method, path string, in any) (T, error) {
	var out T
	err := c.call(method, path, nil, in, &out)
	return out, err
}

// month is a month for a path: an empty one is the running month, which
// the server takes as "current" (and the service as "").
func month(m string) string {
	if strings.TrimSpace(m) == "" {
		return "current"
	}

	return m
}

// do sends in (nil for no body), expecting no result.
func (c *Client) do(method, path string, in any) error {
	return c.call(method, path, nil, in, nil)
}

func (c *Client) Info() (service.Info, error) {
	return read[service.Info](c, "/info", nil)
}

func (c *Client) Overview() (service.Overview, error) {
	return read[service.Overview](c, "/overview", nil)
}

func (c *Client) Analytics(months int) (service.Analytics, error) {
	return read[service.Analytics](c, "/analytics", values("months", months))
}

func (c *Client) SetNetWorth(input service.NetWorthInput) error {
	return c.do(http.MethodPut, at("/net-worth/{month}", month(input.Month)), map[string]string{"amount": input.Amount})
}

func (c *Client) DeleteNetWorth(m string) error {
	return c.do(http.MethodDelete, at("/net-worth/{month}", month(m)), nil)
}

func (c *Client) Accounts(sort int) (service.AccountsOverview, error) {
	return read[service.AccountsOverview](c, "/accounts", values("sort", sort))
}

func (c *Client) CreateAccount(input service.NewAccountInput) error {
	return c.do(http.MethodPost, "/accounts", input)
}

func (c *Client) UpdateAccountAmount(input service.AmountUpdateInput) (service.AmountUpdateResult, error) {
	return write[service.AmountUpdateResult](c, http.MethodPut, at("/accounts/{id}", input.ID), input)
}

func (c *Client) DeleteAccount(id uint) error {
	return c.do(http.MethodDelete, at("/accounts/{id}", id), nil)
}

func (c *Client) AccountValueLogs(accountID uint) ([]service.ValueLog, error) {
	return read[[]service.ValueLog](c, at("/accounts/{id}/values", accountID), nil)
}

func (c *Client) SaveAccountValueLog(input service.ValueLogInput) error {
	return c.do(http.MethodPost, at("/accounts/{id}/values", input.AccountID), input)
}

func (c *Client) DeleteAccountValueLog(accountID uint, logID uint) error {
	return c.do(http.MethodDelete, at("/accounts/{id}/values/{logId}", accountID, logID), nil)
}

func (c *Client) AccountMonthlyValues(accountID uint) ([]service.MonthPoint, error) {
	return read[[]service.MonthPoint](c, at("/accounts/{id}/months", accountID), nil)
}

func (c *Client) Assets(kind string) (service.AssetsView, error) {
	return read[service.AssetsView](c, "/assets", values("kind", kind))
}

func (c *Client) CreateAsset(input service.AssetInput) (service.AssetRow, error) {
	return write[service.AssetRow](c, http.MethodPost, "/assets", input)
}

func (c *Client) UpdateAsset(input service.AssetInput) (service.AssetRow, error) {
	return write[service.AssetRow](c, http.MethodPut, at("/assets/{id}", input.ID), input)
}

func (c *Client) DeleteAsset(id uint) error {
	return c.do(http.MethodDelete, at("/assets/{id}", id), nil)
}

func (c *Client) AssetValueLogs(assetID uint) ([]service.ValueLog, error) {
	return read[[]service.ValueLog](c, at("/assets/{id}/values", assetID), nil)
}

func (c *Client) SaveAssetValueLog(input service.AssetValueLogInput) error {
	return c.do(http.MethodPost, at("/assets/{id}/values", input.AssetID), input)
}

func (c *Client) DeleteAssetValueLog(assetID uint, logID uint) error {
	return c.do(http.MethodDelete, at("/assets/{id}/values/{logId}", assetID, logID), nil)
}

func (c *Client) CashflowMonth(m string) (service.CashflowMonth, error) {
	return read[service.CashflowMonth](c, at("/cashflow/months/{month}", month(m)), nil)
}

func (c *Client) CashflowOverview() (service.CashflowOverview, error) {
	return read[service.CashflowOverview](c, "/cashflow/months", nil)
}

func (c *Client) CashflowStats(includeArchived bool) (service.CashflowOverview, error) {
	return read[service.CashflowOverview](c, "/cashflow/months", values("includeArchived", includeArchived))
}

func (c *Client) CreateCashflow(input service.NewCashflowInput) (service.NewCashflowResult, error) {
	return write[service.NewCashflowResult](c, http.MethodPost, "/cashflow/entries", input)
}

func (c *Client) UpdateCashflow(input service.CashflowUpdateInput) (service.NewCashflowResult, error) {
	return write[service.NewCashflowResult](c, http.MethodPut, at("/cashflow/entries/{id}", input.ID), input.NewCashflowInput)
}

func (c *Client) SetCashflowRate(input service.CashflowRateInput) (int, error) {
	result, err := write[changed](c, http.MethodPut, at("/cashflow/entries/{id}/rate", input.ID), input)
	return result.Changed, err
}

func (c *Client) DeleteCashflow(id uint) error {
	return c.do(http.MethodDelete, at("/cashflow/entries/{id}", id), nil)
}

func (c *Client) CashflowCategories(kind string, includeArchived bool) (service.CashflowCategories, error) {
	return read[service.CashflowCategories](c, "/cashflow/categories", values("kind", kind, "includeArchived", includeArchived))
}

func (c *Client) CashflowSearch(input service.CashflowSearchInput) (service.CashflowSearchResult, error) {
	return read[service.CashflowSearchResult](c, "/cashflow/entries", fields(input))
}

func (c *Client) ChangeCashflows(input service.CashflowBulkInput) (int, error) {
	result, err := write[changed](c, http.MethodPatch, "/cashflow/entries", input)
	return result.Changed, err
}

func (c *Client) CashflowTags() (service.TagsView, error) {
	return read[service.TagsView](c, "/tags", nil)
}

func (c *Client) SaveTag(input service.TagInput) error {
	if input.ID == 0 {
		return c.do(http.MethodPost, "/settings/tags", input)
	}

	return c.do(http.MethodPut, at("/settings/tags/{id}", input.ID), input)
}

func (c *Client) Budgets(month string) (service.BudgetsView, error) {
	return read[service.BudgetsView](c, "/budgets", values("month", month))
}

func (c *Client) SaveBudget(input service.BudgetInput) error {
	if input.ID == 0 {
		return c.do(http.MethodPost, "/budgets", input)
	}

	return c.do(http.MethodPut, at("/budgets/{id}", input.ID), input)
}

func (c *Client) DeleteBudget(id uint) error {
	return c.do(http.MethodDelete, at("/budgets/{id}", id), nil)
}

func (c *Client) MonthNotes() (map[string]string, error) {
	return read[map[string]string](c, "/notes", nil)
}

func (c *Client) SaveMonthNote(input service.MonthNoteInput) error {
	return c.do(http.MethodPut, at("/notes/{month}", month(input.Month)), map[string]string{"text": input.Text})
}

func (c *Client) Credits(mode string) (service.CreditsView, error) {
	return read[service.CreditsView](c, "/credits", values("mode", mode))
}

func (c *Client) CreateCredit(input service.CreditInput) (service.CreatedIn, error) {
	return write[service.CreatedIn](c, http.MethodPost, "/credits", input)
}

func (c *Client) UpdateCredit(input service.CreditInput) (service.CreatedIn, error) {
	return write[service.CreatedIn](c, http.MethodPut, at("/credits/{id}", input.ID), input)
}

func (c *Client) DeleteCredit(id uint) error {
	return c.do(http.MethodDelete, at("/credits/{id}", id), nil)
}

func (c *Client) CreditLogs(id uint) ([]service.CreditLogRow, error) {
	return read[[]service.CreditLogRow](c, at("/credits/{id}/logs", id), nil)
}

func (c *Client) AddCreditLog(input service.CreditLogInput) (service.CreditRow, error) {
	return write[service.CreditRow](c, http.MethodPost, at("/credits/{id}/logs", input.ID), input)
}

func (c *Client) DeleteCreditLog(creditID uint, logID uint) (service.CreditRow, error) {
	return write[service.CreditRow](c, http.MethodDelete, at("/credits/{id}/logs/{logId}", creditID, logID), nil)
}

func (c *Client) Debts(mode string) (service.DebtsView, error) {
	return read[service.DebtsView](c, "/debts", values("mode", mode))
}

func (c *Client) CreateDebt(input service.NewDebtInput) (service.CreatedIn, error) {
	return write[service.CreatedIn](c, http.MethodPost, "/debts", input)
}

func (c *Client) UpdateDebt(input service.DebtUpdateInput) error {
	return c.do(http.MethodPut, at("/debts/{id}", input.ID), input)
}

func (c *Client) DebtLogs(id uint) ([]service.PaymentLog, error) {
	return read[[]service.PaymentLog](c, at("/debts/{id}/payments", id), nil)
}

func (c *Client) AddDebtPayment(input service.PaymentInput) (service.DebtRow, error) {
	return write[service.DebtRow](c, http.MethodPost, at("/debts/{id}/payments", input.ID), input)
}

func (c *Client) DeleteDebtPayment(debtID uint, logID uint) (service.DebtRow, error) {
	return write[service.DebtRow](c, http.MethodDelete, at("/debts/{id}/payments/{logId}", debtID, logID), nil)
}

func (c *Client) DeleteDebt(id uint) error {
	return c.do(http.MethodDelete, at("/debts/{id}", id), nil)
}

func (c *Client) Goals(mode string) (service.GoalsView, error) {
	return read[service.GoalsView](c, "/goals", values("mode", mode))
}

func (c *Client) CreateGoal(input service.NewGoalInput) (service.CreatedIn, error) {
	return write[service.CreatedIn](c, http.MethodPost, "/goals", input)
}

func (c *Client) UpdateGoal(input service.GoalUpdateInput) error {
	return c.do(http.MethodPut, at("/goals/{id}", input.ID), input)
}

func (c *Client) GoalLogs(id uint) ([]service.PaymentLog, error) {
	return read[[]service.PaymentLog](c, at("/goals/{id}/changes", id), nil)
}

func (c *Client) AddGoalChange(input service.PaymentInput) (service.GoalRow, error) {
	return write[service.GoalRow](c, http.MethodPost, at("/goals/{id}/changes", input.ID), input)
}

func (c *Client) DeleteGoalChange(goalID uint, logID uint) (service.GoalRow, error) {
	return write[service.GoalRow](c, http.MethodDelete, at("/goals/{id}/changes/{logId}", goalID, logID), nil)
}

func (c *Client) DeleteGoal(id uint) error {
	return c.do(http.MethodDelete, at("/goals/{id}", id), nil)
}

func (c *Client) Taxes(mode string) (service.TaxesView, error) {
	return read[service.TaxesView](c, "/taxes", values("mode", mode))
}

func (c *Client) CreateTax(input service.NewTaxInput) (service.CreatedIn, error) {
	return write[service.CreatedIn](c, http.MethodPost, "/taxes", input)
}

func (c *Client) UpdateTax(input service.TaxUpdateInput) error {
	return c.do(http.MethodPut, at("/taxes/{id}", input.ID), input)
}

func (c *Client) TaxLogs(id uint) ([]service.PaymentLog, error) {
	return read[[]service.PaymentLog](c, at("/taxes/{id}/payments", id), nil)
}

func (c *Client) AddTaxPayment(input service.PaymentInput) (service.TaxRow, error) {
	return write[service.TaxRow](c, http.MethodPost, at("/taxes/{id}/payments", input.ID), input)
}

func (c *Client) DeleteTax(id uint) error {
	return c.do(http.MethodDelete, at("/taxes/{id}", id), nil)
}

func (c *Client) Invoices(mode string) (service.InvoicesView, error) {
	return read[service.InvoicesView](c, "/invoices", values("mode", mode))
}

func (c *Client) CreateInvoice(input service.InvoiceInput) (service.CreatedIn, error) {
	return write[service.CreatedIn](c, http.MethodPost, "/invoices", input)
}

func (c *Client) UpdateInvoice(input service.InvoiceInput) (service.CreatedIn, error) {
	return write[service.CreatedIn](c, http.MethodPut, at("/invoices/{id}", input.ID), input)
}

func (c *Client) DeleteInvoice(id uint) error {
	return c.do(http.MethodDelete, at("/invoices/{id}", id), nil)
}

func (c *Client) InvoiceLink(id uint) (string, error) {
	link, err := read[struct {
		URL string `json:"url"`
	}](c, at("/invoices/{id}/link", id), nil)

	return link.URL, err
}

func (c *Client) Subscriptions(mode string, kind string) (service.SubscriptionsView, error) {
	return read[service.SubscriptionsView](c, "/subscriptions", values("mode", mode, "kind", kind))
}

func (c *Client) CreateSubscription(input service.SubscriptionInput) error {
	return c.do(http.MethodPost, "/subscriptions", input)
}

func (c *Client) UpdateSubscription(input service.SubscriptionInput) error {
	return c.do(http.MethodPut, at("/subscriptions/{id}", input.ID), input)
}

func (c *Client) MarkSubscriptionPaid(id uint) (service.SubscriptionRow, error) {
	return write[service.SubscriptionRow](c, http.MethodPost, at("/subscriptions/{id}/payments", id), nil)
}

func (c *Client) SubscriptionPayments(id uint) ([]service.SubscriptionPaymentRow, error) {
	return read[[]service.SubscriptionPaymentRow](c, at("/subscriptions/{id}/payments", id), nil)
}

func (c *Client) DeleteSubscriptionPayment(subscriptionID uint, paymentID uint) (service.SubscriptionRow, error) {
	return write[service.SubscriptionRow](c, http.MethodDelete, at("/subscriptions/{id}/payments/{paymentId}", subscriptionID, paymentID), nil)
}

func (c *Client) DeleteSubscription(id uint) error {
	return c.do(http.MethodDelete, at("/subscriptions/{id}", id), nil)
}

func (c *Client) Settings() (service.SettingsView, error) {
	return read[service.SettingsView](c, "/settings", nil)
}

func (c *Client) SaveBaseCurrency(input service.BaseCurrencyInput) error {
	return c.do(http.MethodPut, "/settings/base-currency", input)
}

// saveSetting adds (id 0) or updates a setting of the list.
func (c *Client) saveSetting(list string, id uint, input any) error {
	if id == 0 {
		return c.do(http.MethodPost, "/settings/"+list, input)
	}

	return c.do(http.MethodPut, at("/settings/"+list+"/{id}", id), input)
}

func (c *Client) SaveCurrency(input service.CurrencyInput) error {
	return c.saveSetting("currencies", input.ID, input)
}

func (c *Client) SavePaymentMethod(input service.PaymentMethodInput) error {
	return c.saveSetting("payment-methods", input.ID, input)
}

func (c *Client) SaveTaxType(input service.TaxTypeInput) error {
	return c.saveSetting("tax-types", input.ID, input)
}

func (c *Client) SaveCategory(input service.CategoryInput) error {
	if input.IsIncome {
		return c.saveSetting("income-categories", input.ID, input)
	}

	return c.saveSetting("expense-categories", input.ID, input)
}

func (c *Client) DeleteSetting(kind string, id uint) error {
	list, ok := settingLists[kind]
	if !ok {
		list = kind
	}

	return c.do(http.MethodDelete, at("/settings/"+list+"/{id}", id), nil)
}

func (c *Client) Merge(input service.MergeInput) (service.MergeResult, error) {
	into := mergeInto{IntoID: input.IntoID}
	if input.Kind == "account" {
		return write[service.MergeResult](c, http.MethodPost, at("/accounts/{id}/merge", input.FromID), into)
	}

	list, ok := settingLists[input.Kind]
	if !ok {
		return service.MergeResult{}, fmt.Errorf("%s can't be merged", input.Kind)
	}

	return write[service.MergeResult](c, http.MethodPost, at("/settings/"+list+"/{id}/merge", input.FromID), into)
}

func (c *Client) CurrencyCatalog() ([]service.CatalogCurrency, error) {
	return read[[]service.CatalogCurrency](c, "/currency-catalog", nil)
}

func (c *Client) RatesStatus() (service.RatesStatus, error) {
	return read[service.RatesStatus](c, "/rates", nil)
}

func (c *Client) RefreshRates() (service.RatesStatus, error) {
	return write[service.RatesStatus](c, http.MethodPost, "/rates/refresh", nil)
}

func (c *Client) SetRatesAuto(on bool) (service.RatesStatus, error) {
	return write[service.RatesStatus](c, http.MethodPut, "/rates/auto", map[string]bool{"on": on})
}

func (c *Client) SetupCurrencies(input service.CurrencySetupInput) (service.RatesStatus, error) {
	return write[service.RatesStatus](c, http.MethodPost, "/currency-setup", input)
}

func (c *Client) SkipCurrencySetup() error {
	return c.do(http.MethodDelete, "/currency-setup", nil)
}

func (c *Client) ExportOptions() (service.ExportView, error) {
	return read[service.ExportView](c, "/export", nil)
}
