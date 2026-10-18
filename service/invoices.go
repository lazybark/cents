package service

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/cashflow"
	"github.com/lazybark/cents/flows/invoice"
	"github.com/lazybark/cents/flows/settings"
)

var errInvoiceNotFound = notFoundError("invoice")

// InvoiceRow is an invoice; BaseCents is its amount at the rate recorded
// with it (HasRate is false when it has none).
type InvoiceRow struct {
	ID            uint    `json:"id"`
	Title         string  `json:"title"`
	IsIncoming    bool    `json:"isIncoming"`
	Currency      string  `json:"currency"`
	IsBase        bool    `json:"isBase"`
	RateToBase    float64 `json:"rateToBase"`
	HasRate       bool    `json:"hasRate"`
	AmountCents   int64   `json:"amountCents"`
	BaseCents     int64   `json:"baseCents"`
	Paid          bool    `json:"paid"`
	Peer          string  `json:"peer"`
	InvoiceDate   string  `json:"invoiceDate"`
	DueDate       string  `json:"dueDate"`
	Overdue       bool    `json:"overdue"`
	TargetAccount string  `json:"targetAccount"`
	URL           string  `json:"url"`
	Description   string  `json:"description"`
	// CashflowID is the expense or income added when it was marked paid.
	CashflowID uint `json:"cashflowId"`
}

// InvoicesView is one invoice list. The unpaid totals cover every unpaid
// invoice in the base currency, whichever list is shown; NotCounted is how
// many of them have no conversion rate (or no currency) and are left out.
type InvoicesView struct {
	BaseCurrency    string         `json:"baseCurrency"`
	Mode            string         `json:"mode"`
	Invoices        []InvoiceRow   `json:"invoices"`
	Counts          map[string]int `json:"counts"`
	UnpaidToMeCents int64          `json:"unpaidToMeCents"`
	UnpaidByMeCents int64          `json:"unpaidByMeCents"`
	NotCounted      int            `json:"notCounted"`
	Currencies      []CurrencyRate `json:"currencies"`
	Accounts        []string       `json:"accounts"`
	// Cashflow is what an entry made for a paid invoice can use.
	Cashflow CashflowOptions `json:"cashflow"`
}

// InvoiceInput is an invoice as typed into the form; ID is zero for a new
// one. Every field can change. Rate is the currency's rate
// to the base currency; empty keeps the recorded one while the currency
// stays the same, and otherwise takes the one in settings now.
type InvoiceInput struct {
	ID            uint   `json:"id"`
	Title         string `json:"title"`
	IsIncoming    bool   `json:"isIncoming"`
	Currency      string `json:"currency"`
	Rate          string `json:"rate"`
	Amount        string `json:"amount"`
	Paid          bool   `json:"paid"`
	Peer          string `json:"peer"`
	InvoiceDate   string `json:"invoiceDate"`
	DueDate       string `json:"dueDate"`
	TargetAccount string `json:"targetAccount"`
	URL           string `json:"url"`
	Description   string `json:"description"`
	// Cashflow adds an invoice being marked paid to incomes and expenses,
	// paid on PaidOn (YYYY-MM-DD, empty for today): as an expense when I
	// pay it, an income when it's paid to me.
	Cashflow PaymentCashflow `json:"cashflow"`
	PaidOn   string          `json:"paidOn"`
}

// Invoices lists one of the invoice lists: "outgoing" (unpaid, paid
// to me), "incoming" (unpaid, I pay) or "paid" (both directions).
func (s *Service) Invoices(mode string) (InvoicesView, error) {
	storage, stts, err := s.storageAndSettings()
	if err != nil {
		return InvoicesView{}, err
	}

	items, err := storage.LoadInvoices()
	if err != nil {
		return InvoicesView{}, fmt.Errorf("failed to load invoices: %w", err)
	}

	accounts, err := storage.LoadAccounts()
	if err != nil {
		return InvoicesView{}, fmt.Errorf("failed to load accounts: %w", err)
	}

	listMode := invoice.ListMode(mode)
	if listMode != invoice.ListIncoming && listMode != invoice.ListPaid {
		listMode = invoice.ListOutgoing
	}

	listed := invoice.Filter(items, listMode)
	toMe, byMe := invoice.UnpaidInBaseCents(items)
	now := time.Now()

	view := InvoicesView{
		BaseCurrency:    stts.BaseCurrencyLabel(),
		Mode:            string(listMode),
		Invoices:        make([]InvoiceRow, 0, len(listed)),
		Counts:          map[string]int{},
		UnpaidToMeCents: toMe,
		UnpaidByMeCents: byMe,
		Currencies:      currencyRates(stts),
		Accounts:        accountNames(accounts),
		Cashflow:        cashflowOptions(stts, accounts),
	}

	for _, m := range []invoice.ListMode{invoice.ListOutgoing, invoice.ListIncoming, invoice.ListPaid} {
		view.Counts[string(m)] = len(invoice.Filter(items, m))
	}

	for _, item := range items {
		if _, ok := item.BaseCents(); !ok && !item.Paid && item.AmountCents != 0 {
			view.NotCounted++
		}
	}

	for _, item := range listed {
		view.Invoices = append(view.Invoices, invoiceRow(item, stts, now))
	}

	return view, nil
}

func (s *Service) CreateInvoice(input InvoiceInput) (CreatedIn, error) {
	storage, stts, err := s.storageAndSettings()
	if err != nil {
		return CreatedIn{}, err
	}

	fields, err := invoiceFields(input, stts, "")
	if err != nil {
		return CreatedIn{}, err
	}

	now := time.Now()
	entry, err := invoice.New(fields, stts, dates.ISO, now)
	if err != nil {
		return CreatedIn{}, err
	}

	cash, unlink, err := s.invoiceCashflow(storage, stts, nil, &entry, input, now)
	if err != nil {
		return CreatedIn{}, err
	}

	if err := storage.SaveInvoicePaid(&entry, cash, unlink); err != nil {
		return CreatedIn{}, err
	}

	return CreatedIn{Mode: string(entry.Mode())}, nil
}

// UpdateInvoice replaces every field of an invoice. It returns the list the
// invoice is in now, which changes when it's marked paid or flipped.
func (s *Service) UpdateInvoice(input InvoiceInput) (CreatedIn, error) {
	storage, stts, err := s.storageAndSettings()
	if err != nil {
		return CreatedIn{}, err
	}

	current, err := findInvoice(storage, input.ID)
	if err != nil {
		return CreatedIn{}, err
	}

	fields, err := invoiceFields(input, stts, current.Currency)
	if err != nil {
		return CreatedIn{}, err
	}

	now := time.Now()
	updated, err := current.Edit(fields, stts, dates.ISO, now)
	if err != nil {
		return CreatedIn{}, err
	}

	cash, unlink, err := s.invoiceCashflow(storage, stts, &current, &updated, input, now)
	if err != nil {
		return CreatedIn{}, err
	}

	if err := storage.SaveInvoicePaid(&updated, cash, unlink); err != nil {
		return CreatedIn{}, err
	}

	return CreatedIn{Mode: string(updated.Mode())}, nil
}

func (s *Service) DeleteInvoice(id uint) error {
	storage, err := s.currentStorage()
	if err != nil {
		return err
	}

	if _, err := findInvoice(storage, id); err != nil {
		return err
	}

	if err := storage.DeleteInvoice(id); err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	return nil
}

// InvoiceLink is an invoice's link, to open in a browser. Only web links
// are given.
func (s *Service) InvoiceLink(id uint) (string, error) {
	storage, err := s.currentStorage()
	if err != nil {
		return "", err
	}

	item, err := findInvoice(storage, id)
	if err != nil {
		return "", err
	}

	link, err := url.Parse(item.URL)
	if err != nil || (link.Scheme != "http" && link.Scheme != "https") || link.Host == "" {
		return "", errors.New("only http and https links can be opened")
	}

	return link.String(), nil
}

// invoiceFields checks the currency is empty or a configured one. An edit
// may keep the invoice's current currency even if it was removed from
// settings since. The target account stays free text.
func invoiceFields(input InvoiceInput, stts settings.AppSettings, current string) (invoice.Fields, error) {
	currency := strings.TrimSpace(input.Currency)
	if currency != "" && !strings.EqualFold(currency, strings.TrimSpace(current)) {
		matched, ok := matchOption(stts.CurrencyOptions(), currency)
		if !ok {
			return invoice.Fields{}, fmt.Errorf("unknown currency %q: add it in settings first", input.Currency)
		}

		currency = matched
	}

	return invoice.Fields{
		Title:         input.Title,
		IsIncoming:    input.IsIncoming,
		Currency:      currency,
		Rate:          input.Rate,
		Amount:        input.Amount,
		Paid:          input.Paid,
		Peer:          input.Peer,
		InvoiceDate:   input.InvoiceDate,
		DueDate:       input.DueDate,
		TargetAccount: input.TargetAccount,
		URL:           input.URL,
		Description:   input.Description,
	}, nil
}

// invoiceCashflow works out what saving an invoice does to incomes and
// expenses: cash is the entry to add for one just marked paid (when asked
// for), unlink the entry to delete for one no longer paid. current is nil
// for a new invoice.
func (s *Service) invoiceCashflow(storage Store, stts settings.AppSettings, current, updated *invoice.Invoice, input InvoiceInput, now time.Time) (*cashflow.CashflowEntry, uint, error) {
	var unlink uint
	if current != nil && current.CashflowEntryID != 0 && !updated.Paid {
		unlink = current.CashflowEntryID
		updated.CashflowEntryID = 0
	}

	req := input.Cashflow
	if !req.Add {
		return nil, unlink, nil
	}

	switch {
	case !updated.Paid:
		return nil, 0, errors.New("only a paid invoice can be added to incomes and expenses")
	case updated.CashflowEntryID != 0:
		return nil, 0, errors.New("this invoice is already in incomes and expenses")
	case updated.Currency == "":
		return nil, 0, errors.New("an invoice without a currency can't be added to incomes and expenses: pick one")
	case updated.AmountCents <= 0:
		return nil, 0, errors.New("an invoice without an amount can't be added to incomes and expenses")
	}

	accounts, err := storage.LoadAccounts()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to load accounts: %w", err)
	}

	comment := fmt.Sprintf("Invoice %q", updated.Title)
	if updated.Peer != "" {
		comment += " (" + updated.Peer + ")"
	}

	// An invoice I pay is an expense; one paid to me, an income.
	cash, err := paymentEntry(req, stts, accounts, !updated.IsIncoming, updated.Currency, updated.AmountCents, input.PaidOn, comment, now)
	if err != nil {
		return nil, 0, err
	}

	return cash, unlink, nil
}

func findInvoice(storage Store, id uint) (invoice.Invoice, error) {
	items, err := storage.LoadInvoices()
	if err != nil {
		return invoice.Invoice{}, fmt.Errorf("failed to load invoices: %w", err)
	}

	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}

	return invoice.Invoice{}, errInvoiceNotFound
}

func invoiceRow(item invoice.Invoice, stts settings.AppSettings, now time.Time) InvoiceRow {
	baseCents, hasRate := item.BaseCents()

	return InvoiceRow{
		ID:            item.ID,
		Title:         item.Title,
		IsIncoming:    item.IsIncoming,
		Currency:      item.Currency,
		IsBase:        stts.IsBase(item.Currency),
		RateToBase:    item.RateToBase,
		HasRate:       hasRate,
		AmountCents:   item.AmountCents,
		BaseCents:     baseCents,
		Paid:          item.Paid,
		Peer:          item.Peer,
		InvoiceDate:   formatOptionalDay(item.InvoiceDate),
		DueDate:       formatOptionalDay(item.DueDate),
		Overdue:       overdue(item.DueDate, item.Paid, now),
		TargetAccount: item.TargetAccount,
		URL:           item.URL,
		Description:   item.Description,
		CashflowID:    item.CashflowEntryID,
	}
}
