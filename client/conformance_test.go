package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lazybark/cents/rates"
	"github.com/lazybark/cents/server"
	"github.com/lazybark/cents/service"
	storage "github.com/lazybark/cents/storage/sqlite"
)

// testRates quotes rates for EUR, offline.
type testRates struct{}

func (testRates) Fetch(_ context.Context, base string) (rates.Quote, error) {
	if !strings.EqualFold(base, "EUR") {
		return rates.Quote{}, errors.New("only EUR in tests")
	}

	return rates.Quote{Base: "EUR", Source: "test", Date: "2026-10-01", PerBase: map[string]float64{"USD": 1.25, "GBP": 0.8}}, nil
}

// pair is one database served two ways: by the service itself, and by a
// client of a server on the service.
func pair(t *testing.T) (*service.Service, *Client) {
	t.Helper()

	db, _, err := storage.OpenDatabase(filepath.Join(t.TempDir(), "cents.db"))
	if err != nil {
		t.Fatal(err)
	}

	store := storage.NewSQLiteStorage(db)
	t.Cleanup(func() { _ = store.Close() })
	local := service.On(store).WithRates(testRates{})

	token, hash, err := server.NewToken()
	if err != nil {
		t.Fatal(err)
	}

	h, err := server.Handler(server.Config{Tokens: []server.Token{{Name: "test", SHA256: hash}}}, local, local, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	remote, err := New(srv.URL+"/", token)
	if err != nil {
		t.Fatal(err)
	}

	return local, remote
}

// comparable is v as JSON, without when it was loaded (which differs from
// call to call).
func comparable(t *testing.T, v any) []byte {
	t.Helper()

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}

	var tree any
	if err := json.Unmarshal(data, &tree); err != nil {
		t.Fatal(err)
	}

	var strip func(any)
	strip = func(node any) {
		switch node := node.(type) {
		case map[string]any:
			delete(node, "loadedAt")
			for _, child := range node {
				strip(child)
			}
		case []any:
			for _, child := range node {
				strip(child)
			}
		}
	}
	strip(tree)

	data, _ = json.Marshal(tree)
	return data
}

// TestConformance runs every Backend method through the client and checks
// it does what the service does: changes go through the client, and every
// read and refusal is compared with the service's own, as JSON.
func TestConformance(t *testing.T) {
	local, remote := pair(t)
	covered := map[string]bool{}

	// same calls name on both and compares results and errors.
	same := func(name string, call func(b service.Backend) (any, error)) any {
		t.Helper()
		covered[name] = true

		want, wantErr := call(local)
		got, gotErr := call(remote)
		if (wantErr == nil) != (gotErr == nil) || wantErr != nil && wantErr.Error() != gotErr.Error() {
			t.Fatalf("%s: the service failed with %v, the client with %v", name, wantErr, gotErr)
		}

		for _, kind := range []error{service.ErrNotFound, service.ErrUpstream, service.ErrNoDatabase} {
			if errors.Is(wantErr, kind) != errors.Is(gotErr, kind) {
				t.Fatalf("%s: errors.Is(%v) differs: %v, %v", name, kind, wantErr, gotErr)
			}
		}

		if wantErr == nil {
			wantJSON, gotJSON := comparable(t, want), comparable(t, got)
			if !bytes.Equal(wantJSON, gotJSON) {
				t.Fatalf("%s differs:\nservice: %s\nclient:  %s", name, wantJSON, gotJSON)
			}
		}

		return got
	}

	// change calls name through the client, which must succeed.
	change := func(name string, call func(b service.Backend) (any, error)) any {
		t.Helper()
		covered[name] = true

		got, err := call(remote)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		return got
	}

	// refused calls name through both, which must refuse it alike.
	refused := func(name string, call func(b service.Backend) (any, error)) {
		t.Helper()
		if _, err := call(remote); err == nil {
			t.Fatalf("%s: expected a refusal", name)
		}

		same(name, call)
	}

	month := time.Now().Format("2006-01")
	today := time.Now().Format("2006-01-02")

	// A new database: currencies first.
	same("Info", func(b service.Backend) (any, error) { return b.Info() })
	same("CurrencyCatalog", func(b service.Backend) (any, error) { return b.CurrencyCatalog() })
	refused("SetupCurrencies", func(b service.Backend) (any, error) {
		return b.SetupCurrencies(service.CurrencySetupInput{Codes: []string{"EUR"}, Base: "USD"})
	})
	change("SetupCurrencies", func(b service.Backend) (any, error) {
		return b.SetupCurrencies(service.CurrencySetupInput{Codes: []string{"EUR", "USD"}, Base: "EUR"})
	})
	change("SkipCurrencySetup", func(b service.Backend) (any, error) { return nil, b.SkipCurrencySetup() })
	same("RatesStatus", func(b service.Backend) (any, error) { return b.RatesStatus() })
	change("RefreshRates", func(b service.Backend) (any, error) { return b.RefreshRates() })
	change("SetRatesAuto", func(b service.Backend) (any, error) { return b.SetRatesAuto(false) })

	// Settings.
	change("SaveCurrency", func(b service.Backend) (any, error) {
		return nil, b.SaveCurrency(service.CurrencyInput{Name: "Lari", Rate: "0.33"})
	})
	change("SavePaymentMethod", func(b service.Backend) (any, error) {
		return nil, b.SavePaymentMethod(service.PaymentMethodInput{Name: "Visa", Type: "Card", IsDefault: true})
	})
	change("SaveTaxType", func(b service.Backend) (any, error) {
		return nil, b.SaveTaxType(service.TaxTypeInput{Country: "NL", Name: "VAT"})
	})
	for _, c := range []service.CategoryInput{{IsIncome: true, Name: "Salary"}, {Name: "Rent"}, {Name: "Food"}, {Name: "Eating out"}} {
		change("SaveCategory", func(b service.Backend) (any, error) { return nil, b.SaveCategory(c) })
	}
	change("SaveTag", func(b service.Backend) (any, error) { return nil, b.SaveTag(service.TagInput{Name: "work"}) })
	view := same("Settings", func(b service.Backend) (any, error) { return b.Settings() }).(service.SettingsView)
	refused("SaveBaseCurrency", func(b service.Backend) (any, error) {
		return nil, b.SaveBaseCurrency(service.BaseCurrencyInput{Value: " "})
	})
	change("SaveBaseCurrency", func(b service.Backend) (any, error) {
		return nil, b.SaveBaseCurrency(service.BaseCurrencyInput{Value: "EUR", Code: "EUR"})
	})

	var lari service.CurrencySetting
	for _, c := range view.Currencies {
		if c.Name == "Lari" {
			lari = c
		}
	}
	change("SaveCurrency", func(b service.Backend) (any, error) {
		return nil, b.SaveCurrency(service.CurrencyInput{ID: lari.ID, Name: "GEL", Rate: "0.34"})
	})
	refused("SaveCategory", func(b service.Backend) (any, error) {
		return nil, b.SaveCategory(service.CategoryInput{Name: "rent"})
	})

	// Accounts.
	change("CreateAccount", func(b service.Backend) (any, error) {
		return nil, b.CreateAccount(service.NewAccountInput{Name: "Bank", Description: "main", Currency: "EUR", Amount: "1000"})
	})
	change("CreateAccount", func(b service.Backend) (any, error) {
		return nil, b.CreateAccount(service.NewAccountInput{Name: "Wallet", Description: "cash", Currency: "USD", Amount: "50"})
	})
	refused("CreateAccount", func(b service.Backend) (any, error) {
		return nil, b.CreateAccount(service.NewAccountInput{Name: "Bank"})
	})
	accounts := same("Accounts", func(b service.Backend) (any, error) { return b.Accounts(1) }).(service.AccountsOverview)
	bank, wallet := accounts.Accounts[0].ID, accounts.Accounts[1].ID
	change("UpdateAccountAmount", func(b service.Backend) (any, error) {
		return b.UpdateAccountAmount(service.AmountUpdateInput{ID: bank, Amount: "1200", UpdateLog: true})
	})
	change("SaveAccountValueLog", func(b service.Backend) (any, error) {
		return nil, b.SaveAccountValueLog(service.ValueLogInput{AccountID: bank, Date: "2026-01-31", Value: "900"})
	})
	logs := same("AccountValueLogs", func(b service.Backend) (any, error) { return b.AccountValueLogs(bank) }).([]service.ValueLog)
	same("AccountMonthlyValues", func(b service.Backend) (any, error) { return b.AccountMonthlyValues(bank) })
	change("DeleteAccountValueLog", func(b service.Backend) (any, error) {
		return nil, b.DeleteAccountValueLog(bank, logs[len(logs)-1].ID)
	})
	refused("DeleteAccountValueLog", func(b service.Backend) (any, error) { return nil, b.DeleteAccountValueLog(bank, 999) })

	// Incomes and expenses.
	change("CreateCashflow", func(b service.Backend) (any, error) {
		return b.CreateCashflow(service.NewCashflowInput{IsIncome: true, Currency: "EUR", Amount: "3000", Date: today, Category: "Salary", Account: "Bank"})
	})
	change("CreateCashflow", func(b service.Backend) (any, error) {
		return b.CreateCashflow(service.NewCashflowInput{Currency: "USD", Amount: "25", Date: today, Category: "Food", Comment: "lunch", Tags: []string{"work"}})
	})
	change("CreateCashflow", func(b service.Backend) (any, error) {
		return b.CreateCashflow(service.NewCashflowInput{Currency: "EUR", Amount: "900", Date: today, Category: "Rent"})
	})
	refused("CreateCashflow", func(b service.Backend) (any, error) {
		return b.CreateCashflow(service.NewCashflowInput{Currency: "EUR", Amount: "1", Date: "yesterday", Category: "Rent"})
	})
	monthView := same("CashflowMonth", func(b service.Backend) (any, error) { return b.CashflowMonth(month) }).(service.CashflowMonth)
	var lunch, rent uint
	for _, e := range monthView.Entries {
		switch e.Comment {
		case "lunch":
			lunch = e.ID
		case "":
			if !e.IsIncome {
				rent = e.ID
			}
		}
	}

	change("UpdateCashflow", func(b service.Backend) (any, error) {
		return b.UpdateCashflow(service.CashflowUpdateInput{ID: lunch, NewCashflowInput: service.NewCashflowInput{Currency: "USD", Amount: "30", Date: today, Category: "Eating out", Comment: "lunch", Tags: []string{"work"}}})
	})
	change("SetCashflowRate", func(b service.Backend) (any, error) {
		return b.SetCashflowRate(service.CashflowRateInput{ID: lunch, Rate: "0.9"})
	})
	change("ChangeCashflows", func(b service.Backend) (any, error) {
		return b.ChangeCashflows(service.CashflowBulkInput{IDs: []uint{lunch, rent}, AddTag: "monthly"})
	})
	same("CashflowSearch", func(b service.Backend) (any, error) {
		return b.CashflowSearch(service.CashflowSearchInput{Text: "lunch", Tag: "work", Kind: "expense"})
	})
	same("CashflowOverview", func(b service.Backend) (any, error) { return b.CashflowOverview() })
	same("CashflowStats", func(b service.Backend) (any, error) { return b.CashflowStats(false) })
	same("CashflowCategories", func(b service.Backend) (any, error) { return b.CashflowCategories("expense", true) })
	same("CashflowTags", func(b service.Backend) (any, error) { return b.CashflowTags() })
	same("CashflowMonth", func(b service.Backend) (any, error) { return b.CashflowMonth(month) })
	refused("CashflowMonth", func(b service.Backend) (any, error) { return b.CashflowMonth("September") })
	// No month is the running one; where a month is needed, it's refused
	// alike.
	same("CashflowMonth", func(b service.Backend) (any, error) { return b.CashflowMonth("") })
	refused("SaveMonthNote", func(b service.Backend) (any, error) {
		return nil, b.SaveMonthNote(service.MonthNoteInput{Text: "no month"})
	})
	refused("SetNetWorth", func(b service.Backend) (any, error) {
		return nil, b.SetNetWorth(service.NetWorthInput{Amount: "1"})
	})
	refused("DeleteNetWorth", func(b service.Backend) (any, error) { return nil, b.DeleteNetWorth(" ") })

	// Budgets and notes.
	change("SaveBudget", func(b service.Backend) (any, error) {
		return nil, b.SaveBudget(service.BudgetInput{Category: "Food", Limit: "200"})
	})
	budgets := same("Budgets", func(b service.Backend) (any, error) { return b.Budgets(month) }).(service.BudgetsView)
	var food uint
	for _, row := range budgets.Rows {
		if row.Category == "Food" {
			food = row.ID
		}
	}

	change("SaveBudget", func(b service.Backend) (any, error) {
		return nil, b.SaveBudget(service.BudgetInput{ID: food, Category: "Food", Limit: "250"})
	})
	change("SaveMonthNote", func(b service.Backend) (any, error) {
		return nil, b.SaveMonthNote(service.MonthNoteInput{Month: month, Text: "a trip"})
	})
	same("MonthNotes", func(b service.Backend) (any, error) { return b.MonthNotes() })

	// Property and investments.
	created := change("CreateAsset", func(b service.Backend) (any, error) {
		return b.CreateAsset(service.AssetInput{Kind: "investment", Currency: "USD", Name: "Index fund", Type: "Fund", Value: "1000", Cost: "800"})
	}).(service.AssetRow)
	change("UpdateAsset", func(b service.Backend) (any, error) {
		in := service.AssetInput{ID: created.ID, Kind: "investment", Currency: "USD", Name: "Index fund", Type: "Fund", Value: "1100", Cost: "800"}
		return b.UpdateAsset(in)
	})
	change("SaveAssetValueLog", func(b service.Backend) (any, error) {
		return nil, b.SaveAssetValueLog(service.AssetValueLogInput{AssetID: created.ID, Date: "2026-01-01", Value: "950"})
	})
	assetLogs := same("AssetValueLogs", func(b service.Backend) (any, error) { return b.AssetValueLogs(created.ID) }).([]service.ValueLog)
	change("DeleteAssetValueLog", func(b service.Backend) (any, error) {
		return nil, b.DeleteAssetValueLog(created.ID, assetLogs[len(assetLogs)-1].ID)
	})
	same("Assets", func(b service.Backend) (any, error) { return b.Assets("investment") })

	// Debts.
	change("CreateDebt", func(b service.Backend) (any, error) {
		return b.CreateDebt(service.NewDebtInput{Peer: "Ann", Currency: "EUR", Amount: "100", AmountPaid: "0", CreatedAt: "2026-03-01"})
	})
	debts := same("Debts", func(b service.Backend) (any, error) { return b.Debts("outgoing") }).(service.DebtsView)
	debtID := debts.Debts[0].ID
	change("UpdateDebt", func(b service.Backend) (any, error) {
		return nil, b.UpdateDebt(service.DebtUpdateInput{ID: debtID, Amount: "120", AmountPaid: "0", CreatedAt: "2026-03-01", Comment: "dinner"})
	})
	change("AddDebtPayment", func(b service.Backend) (any, error) {
		return b.AddDebtPayment(service.PaymentInput{ID: debtID, Delta: "20", Date: "2026-04-01"})
	})
	debtLogs := same("DebtLogs", func(b service.Backend) (any, error) { return b.DebtLogs(debtID) }).([]service.PaymentLog)
	change("DeleteDebtPayment", func(b service.Backend) (any, error) { return b.DeleteDebtPayment(debtID, debtLogs[0].ID) })
	change("DeleteDebt", func(b service.Backend) (any, error) { return nil, b.DeleteDebt(debtID) })
	refused("DeleteDebt", func(b service.Backend) (any, error) { return nil, b.DeleteDebt(debtID) })

	// Credits.
	change("CreateCredit", func(b service.Backend) (any, error) {
		return b.CreateCredit(service.CreditInput{Currency: "EUR", Name: "Car loan", Purpose: "Car", Issuer: "Bank", Total: "10000", Paid: "0", StartDate: "2026-01-01"})
	})
	credits := same("Credits", func(b service.Backend) (any, error) { return b.Credits("") }).(service.CreditsView)
	creditID := credits.Credits[0].ID
	change("UpdateCredit", func(b service.Backend) (any, error) {
		return b.UpdateCredit(service.CreditInput{ID: creditID, Currency: "EUR", Name: "Car loan", Purpose: "Car", Issuer: "Bank", Total: "11000", Paid: "0", StartDate: "2026-01-01"})
	})
	change("AddCreditLog", func(b service.Backend) (any, error) {
		return b.AddCreditLog(service.CreditLogInput{ID: creditID, Kind: "payment", Amount: "500", Date: "2026-02-01"})
	})
	creditLogs := same("CreditLogs", func(b service.Backend) (any, error) { return b.CreditLogs(creditID) }).([]service.CreditLogRow)
	change("DeleteCreditLog", func(b service.Backend) (any, error) { return b.DeleteCreditLog(creditID, creditLogs[0].ID) })
	change("DeleteCredit", func(b service.Backend) (any, error) { return nil, b.DeleteCredit(creditID) })

	// Goals.
	change("CreateGoal", func(b service.Backend) (any, error) {
		return b.CreateGoal(service.NewGoalInput{Name: "Trip", Currency: "EUR", Target: "2000", Accumulated: "100", StartedAt: "2026-01-01"})
	})
	goals := same("Goals", func(b service.Backend) (any, error) { return b.Goals("active") }).(service.GoalsView)
	goalID := goals.Goals[0].ID
	change("UpdateGoal", func(b service.Backend) (any, error) {
		return nil, b.UpdateGoal(service.GoalUpdateInput{ID: goalID, Target: "2500", Accumulated: "100", StartedAt: "2026-01-01"})
	})
	change("AddGoalChange", func(b service.Backend) (any, error) {
		return b.AddGoalChange(service.PaymentInput{ID: goalID, Delta: "50", Date: "2026-02-01"})
	})
	goalLogs := same("GoalLogs", func(b service.Backend) (any, error) { return b.GoalLogs(goalID) }).([]service.PaymentLog)
	change("DeleteGoalChange", func(b service.Backend) (any, error) { return b.DeleteGoalChange(goalID, goalLogs[0].ID) })
	change("DeleteGoal", func(b service.Backend) (any, error) { return nil, b.DeleteGoal(goalID) })

	// Taxes.
	taxType := view.TaxTypes
	if len(taxType) == 0 {
		view = same("Settings", func(b service.Backend) (any, error) { return b.Settings() }).(service.SettingsView)
		taxType = view.TaxTypes
	}
	change("CreateTax", func(b service.Backend) (any, error) {
		return b.CreateTax(service.NewTaxInput{TaxTypeID: taxType[0].ID, Currency: "EUR", AmountDue: "300", AmountPaid: "0", Period: "Q1 2026", DueDate: "2026-04-30"})
	})
	taxes := same("Taxes", func(b service.Backend) (any, error) { return b.Taxes("") }).(service.TaxesView)
	taxID := taxes.Taxes[0].ID
	change("UpdateTax", func(b service.Backend) (any, error) {
		return nil, b.UpdateTax(service.TaxUpdateInput{ID: taxID, AmountDue: "320", AmountPaid: "0", Period: "Q1 2026", DueDate: "2026-04-30"})
	})
	change("AddTaxPayment", func(b service.Backend) (any, error) {
		return b.AddTaxPayment(service.PaymentInput{ID: taxID, Delta: "100", Date: "2026-04-01"})
	})
	same("TaxLogs", func(b service.Backend) (any, error) { return b.TaxLogs(taxID) })
	change("DeleteTax", func(b service.Backend) (any, error) { return nil, b.DeleteTax(taxID) })

	// Invoices.
	change("CreateInvoice", func(b service.Backend) (any, error) {
		return b.CreateInvoice(service.InvoiceInput{Title: "Design", Currency: "EUR", Amount: "500", Peer: "Acme", InvoiceDate: "2026-03-01", URL: "https://example.com/i/1"})
	})
	invoices := same("Invoices", func(b service.Backend) (any, error) { return b.Invoices("outgoing") }).(service.InvoicesView)
	invoiceID := invoices.Invoices[0].ID
	same("InvoiceLink", func(b service.Backend) (any, error) { return b.InvoiceLink(invoiceID) })
	change("UpdateInvoice", func(b service.Backend) (any, error) {
		return b.UpdateInvoice(service.InvoiceInput{ID: invoiceID, Title: "Design work", Currency: "EUR", Amount: "500", Peer: "Acme", InvoiceDate: "2026-03-01"})
	})
	refused("InvoiceLink", func(b service.Backend) (any, error) { return b.InvoiceLink(invoiceID) })
	change("DeleteInvoice", func(b service.Backend) (any, error) { return nil, b.DeleteInvoice(invoiceID) })

	// Subscriptions.
	change("CreateSubscription", func(b service.Backend) (any, error) {
		return nil, b.CreateSubscription(service.SubscriptionInput{Name: "Music", Type: "Multimedia", Currency: "EUR", Amount: "10", Period: "month", PaymentMethod: "Visa", NextPayment: "2026-01-05", IsActive: true, PaidManually: true})
	})
	subs := same("Subscriptions", func(b service.Backend) (any, error) { return b.Subscriptions("all", "") }).(service.SubscriptionsView)
	subID := subs.Subscriptions[0].ID
	change("UpdateSubscription", func(b service.Backend) (any, error) {
		return nil, b.UpdateSubscription(service.SubscriptionInput{ID: subID, Name: "Music", Type: "Multimedia", Currency: "EUR", Amount: "12", Period: "month", PaymentMethod: "Visa", NextPayment: "2026-01-05", IsActive: true, PaidManually: true})
	})
	change("MarkSubscriptionPaid", func(b service.Backend) (any, error) { return b.MarkSubscriptionPaid(subID) })
	payments := same("SubscriptionPayments", func(b service.Backend) (any, error) { return b.SubscriptionPayments(subID) }).([]service.SubscriptionPaymentRow)
	change("DeleteSubscriptionPayment", func(b service.Backend) (any, error) {
		return b.DeleteSubscriptionPayment(subID, payments[0].ID)
	})
	change("DeleteSubscription", func(b service.Backend) (any, error) { return nil, b.DeleteSubscription(subID) })

	// Overview and analytics.
	same("Overview", func(b service.Backend) (any, error) { return b.Overview() })
	change("SetNetWorth", func(b service.Backend) (any, error) {
		return nil, b.SetNetWorth(service.NetWorthInput{Month: "2025-12", Amount: "5000"})
	})
	same("Analytics", func(b service.Backend) (any, error) { return b.Analytics(12) })
	change("DeleteNetWorth", func(b service.Backend) (any, error) { return nil, b.DeleteNetWorth("2025-12") })
	refused("DeleteNetWorth", func(b service.Backend) (any, error) { return nil, b.DeleteNetWorth("2025-12") })
	same("ExportOptions", func(b service.Backend) (any, error) { return b.ExportOptions() })

	// Merging and deleting settings, and accounts.
	view = same("Settings", func(b service.Backend) (any, error) { return b.Settings() }).(service.SettingsView)
	ids := map[string]uint{}
	for _, c := range view.ExpenseCategories {
		ids[c.Name] = c.ID
	}

	refused("DeleteSetting", func(b service.Backend) (any, error) {
		return nil, b.DeleteSetting("expense_category", ids["Eating out"])
	})
	change("Merge", func(b service.Backend) (any, error) {
		return b.Merge(service.MergeInput{Kind: "expense_category", FromID: ids["Eating out"], IntoID: ids["Food"]})
	})
	refused("Merge", func(b service.Backend) (any, error) {
		return b.Merge(service.MergeInput{Kind: "colour", FromID: 1, IntoID: 2})
	})
	change("Merge", func(b service.Backend) (any, error) {
		return b.Merge(service.MergeInput{Kind: "account", FromID: wallet, IntoID: bank})
	})
	change("DeleteSetting", func(b service.Backend) (any, error) { return nil, b.DeleteSetting("tax_type", taxType[0].ID) })
	change("DeleteBudget", func(b service.Backend) (any, error) { return nil, b.DeleteBudget(food) })
	change("DeleteCashflow", func(b service.Backend) (any, error) { return nil, b.DeleteCashflow(lunch) })
	refused("DeleteCashflow", func(b service.Backend) (any, error) { return nil, b.DeleteCashflow(lunch) })
	change("DeleteAsset", func(b service.Backend) (any, error) { return nil, b.DeleteAsset(created.ID) })
	refused("DeleteAccount", func(b service.Backend) (any, error) { return nil, b.DeleteAccount(bank) })
	change("CreateAccount", func(b service.Backend) (any, error) {
		return nil, b.CreateAccount(service.NewAccountInput{Name: "Old", Description: "closed", Currency: "EUR", Amount: "0"})
	})
	accounts = same("Accounts", func(b service.Backend) (any, error) { return b.Accounts(1) }).(service.AccountsOverview)
	change("DeleteAccount", func(b service.Backend) (any, error) {
		return nil, b.DeleteAccount(accounts.Accounts[len(accounts.Accounts)-1].ID)
	})
	same("Settings", func(b service.Backend) (any, error) { return b.Settings() })

	// Every Backend method is covered.
	backend := reflect.TypeFor[service.Backend]()
	var missing []string
	for i := 0; i < backend.NumMethod(); i++ {
		if name := backend.Method(i).Name; !covered[name] {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		t.Fatalf("not covered: %s", strings.Join(missing, ", "))
	}
}

func TestFiles(t *testing.T) {
	local, remote := pair(t)
	if _, err := remote.CreateDebt(service.NewDebtInput{Peer: "Ann", Currency: "$", Amount: "10", AmountPaid: "0", CreatedAt: "2026-03-01"}); err != nil {
		t.Fatal(err)
	}

	var want, got bytes.Buffer
	wantRows, wantErr := local.WriteCSV(&want, "debts", "", "")
	gotRows, gotErr := remote.WriteCSV(&got, "debts", "", "")
	if wantErr != nil || gotErr != nil || gotRows != wantRows || gotRows != 1 || got.String() != want.String() {
		t.Fatalf("expected the same CSV: %d %d %v %v\n%s\n%s", wantRows, gotRows, wantErr, gotErr, want.String(), got.String())
	}

	if _, err := remote.WriteCSV(&got, "nope", "", ""); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("expected an unknown dataset not found, got %v", err)
	}

	// The savers work on either.
	dir := t.TempDir()
	result, err := service.SaveBackup(remote, filepath.Join(dir, "backup"))
	if err != nil || result.Path != filepath.Join(dir, "backup.db") || result.At == "" {
		t.Fatalf("unexpected %+v %v", result, err)
	}

	if err := storage.CheckDatabase(result.Path); err != nil {
		t.Fatalf("expected a database: %v", err)
	}

	if export, _ := remote.ExportOptions(); export.LastBackupPath != result.Path {
		t.Fatalf("expected the backup remembered: %+v", export)
	}

	exported, err := service.ExportCSV(remote, dir, service.ExportInput{Datasets: []string{"debts", "accounts"}})
	if err != nil || len(exported.Files) != 2 || exported.Files[1].Name != "debts.csv" || exported.Files[1].Rows != 1 {
		t.Fatalf("unexpected %+v %v", exported, err)
	}
}

func TestClientErrors(t *testing.T) {
	if _, err := New("cents.lan:8740", "x"); err == nil {
		t.Fatal("expected an address without a scheme refused")
	}

	if _, err := New("https://cents.lan", " "); err == nil {
		t.Fatal("expected a token asked for")
	}

	_, remote := pair(t)
	wrong := *remote
	wrong.token = "cents_wrong"
	_, err := wrong.Info()
	var answer *Error
	if !errors.As(err, &answer) || answer.Status != 401 || answer.Code != "unauthorized" {
		t.Fatalf("expected unauthorized, got %v", err)
	}

	gone, _ := New("http://127.0.0.1:1", "cents_x")
	if _, err := gone.Info(); err == nil || !strings.Contains(err.Error(), "couldn't reach the server") {
		t.Fatalf("expected the server unreachable, got %v", err)
	}

	if !slices.Contains([]string{"currencies", "tags"}, settingLists["currency"]) {
		t.Fatal("unexpected setting lists")
	}
}
