package desktop

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/lazybark/cents/dates"
	"github.com/lazybark/cents/flows/invoice"
	"github.com/lazybark/cents/flows/settings"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var errInvoiceNotFound = errors.New("invoice not found")

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
}

// InvoiceInput is an invoice as typed into the form; ID is zero for a new
// one. Every field can change, as in the TUI. Rate is the currency's rate
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
}

// Invoices lists one of the TUI's invoice lists: "outgoing" (unpaid, paid
// to me), "incoming" (unpaid, I pay) or "paid" (both directions).
func (a *API) Invoices(mode string) (InvoicesView, error) {
	storage, stts, err := a.storageAndSettings()
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

func (a *API) CreateInvoice(input InvoiceInput) (CreatedIn, error) {
	storage, stts, err := a.storageAndSettings()
	if err != nil {
		return CreatedIn{}, err
	}

	fields, err := invoiceFields(input, stts, "")
	if err != nil {
		return CreatedIn{}, err
	}

	entry, err := invoice.New(fields, stts, dates.ISO, time.Now())
	if err != nil {
		return CreatedIn{}, err
	}

	if err := storage.CreateInvoice(&entry); err != nil {
		return CreatedIn{}, fmt.Errorf("save failed: %w", err)
	}

	return CreatedIn{Mode: string(entry.Mode())}, nil
}

// UpdateInvoice replaces every field of an invoice. It returns the list the
// invoice is in now, which changes when it's marked paid or flipped.
func (a *API) UpdateInvoice(input InvoiceInput) (CreatedIn, error) {
	storage, stts, err := a.storageAndSettings()
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

	updated, err := current.Edit(fields, stts, dates.ISO, time.Now())
	if err != nil {
		return CreatedIn{}, err
	}

	if err := storage.SaveInvoice(&updated); err != nil {
		return CreatedIn{}, fmt.Errorf("save failed: %w", err)
	}

	return CreatedIn{Mode: string(updated.Mode())}, nil
}

func (a *API) DeleteInvoice(id uint) error {
	storage, err := a.currentStorage()
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

// OpenInvoiceURL opens an invoice's link in the system browser. Only web
// links are opened.
func (a *API) OpenInvoiceURL(id uint) error {
	storage, err := a.currentStorage()
	if err != nil {
		return err
	}

	item, err := findInvoice(storage, id)
	if err != nil {
		return err
	}

	link, err := url.Parse(item.URL)
	if err != nil || (link.Scheme != "http" && link.Scheme != "https") || link.Host == "" {
		return errors.New("only http and https links can be opened")
	}

	runtime.BrowserOpenURL(a.ctx, link.String())

	return nil
}

// invoiceFields checks what the TUI's pickers guarantee: the currency is
// empty or a configured one. An edit may keep the invoice's current
// currency even if it was removed from settings since. The target account
// stays free text, like the TUI's override field.
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

func findInvoice(storage StorageWorker, id uint) (invoice.Invoice, error) {
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
	}
}
