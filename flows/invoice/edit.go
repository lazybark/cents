package invoice

import "github.com/charmbracelet/bubbles/textinput"

func NewEditInvoiceForm(currencyOptions []string, accountOptions []string) EditInvoiceForm {
	currencyOptions = invoiceCurrencySelectionOptions(currencyOptions)
	titleInput := textinput.New()
	titleInput.Placeholder = "Invoice title"
	titleInput.CharLimit = 120
	titleInput.Width = 36

	amountInput := textinput.New()
	amountInput.Placeholder = "optional 1000.00"
	amountInput.CharLimit = 24
	amountInput.Width = 20

	peerInput := textinput.New()
	peerInput.Placeholder = "peer"
	peerInput.CharLimit = 120
	peerInput.Width = 30

	invoiceDateInput := textinput.New()
	invoiceDateInput.Placeholder = "optional DD.MM.YYYY"
	invoiceDateInput.CharLimit = 24
	invoiceDateInput.Width = 24

	dueDateInput := textinput.New()
	dueDateInput.Placeholder = "optional DD.MM.YYYY"
	dueDateInput.CharLimit = 24
	dueDateInput.Width = 24

	targetAccountInput := textinput.New()
	targetAccountInput.Placeholder = "optional account override"
	targetAccountInput.CharLimit = 120
	targetAccountInput.Width = 30

	urlInput := textinput.New()
	urlInput.Placeholder = "optional https://..."
	urlInput.CharLimit = 180
	urlInput.Width = 42

	descriptionInput := textinput.New()
	descriptionInput.Placeholder = "optional description"
	descriptionInput.CharLimit = 180
	descriptionInput.Width = 42

	form := EditInvoiceForm{
		TitleInput:         titleInput,
		AmountInput:        amountInput,
		PeerInput:          peerInput,
		InvoiceDateInput:   invoiceDateInput,
		DueDateInput:       dueDateInput,
		TargetAccountInput: targetAccountInput,
		URLInput:           urlInput,
		DescriptionInput:   descriptionInput,
		ActiveField:        0,
		CurrencyOptions:    append([]string(nil), currencyOptions...),
		CurrencyIndex:      0,
		AccountOptions:     append([]string(nil), accountOptions...),
		AccountIndex:       0,
		IsIncoming:         true,
		Paid:               false,
	}

	return form.FocusActive()
}

func (f EditInvoiceForm) FocusActive() EditInvoiceForm {
	f.TitleInput.Blur()
	f.AmountInput.Blur()
	f.PeerInput.Blur()
	f.InvoiceDateInput.Blur()
	f.DueDateInput.Blur()
	f.TargetAccountInput.Blur()
	f.URLInput.Blur()
	f.DescriptionInput.Blur()

	switch f.ActiveField {
	case InvoiceFieldTitle:
		f.TitleInput.Focus()
	case InvoiceFieldAmount:
		f.AmountInput.Focus()
	case InvoiceFieldPeer:
		f.PeerInput.Focus()
	case InvoiceFieldInvoiceDate:
		f.InvoiceDateInput.Focus()
	case InvoiceFieldDueDate:
		f.DueDateInput.Focus()
	case InvoiceFieldTargetAccountName:
		f.TargetAccountInput.Focus()
	case InvoiceFieldURL:
		f.URLInput.Focus()
	case InvoiceFieldDescription:
		f.DescriptionInput.Focus()
	}
	return f
}

func (f EditInvoiceForm) Next() EditInvoiceForm {
	if f.ActiveField < InvoiceFieldCount-1 {
		f.ActiveField++
	}

	return f.FocusActive()
}

func (f EditInvoiceForm) Prev() EditInvoiceForm {
	if f.ActiveField > 0 {
		f.ActiveField--
	}

	return f.FocusActive()
}
