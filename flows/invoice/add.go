package invoice

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
)

func NewAddInvoiceForm(currencyOptions []string, accountOptions []string) AddInvoiceForm {
	currencyOptions = invoiceCurrencySelectionOptions(currencyOptions)
	today := time.Now().Format("02.01.2006")
	inputs := make([]textinput.Model, 8)
	placeholders := []string{"Invoice title", "1000.00", "Peer", today, "optional DD.MM.YYYY", "optional account override", "optional https://...", "optional description"}

	for i := range inputs {
		field := textinput.New()
		field.Placeholder = placeholders[i]
		field.CharLimit = 180
		field.Width = 36
		inputs[i] = field
	}

	form := AddInvoiceForm{
		Inputs:          inputs,
		Active:          0,
		CurrencyOptions: append([]string(nil), currencyOptions...),
		CurrencyIndex:   0,
		AccountOptions:  append([]string(nil), accountOptions...),
		AccountIndex:    0,
		IsIncoming:      true,
		Paid:            false,
	}

	return form.FocusActive()
}

func (f AddInvoiceForm) InputIndexForField(field int) int {
	switch field {
	case InvoiceFieldTitle:
		return 0
	case InvoiceFieldType:
		return -1
	case InvoiceFieldCurrency:
		return -1
	case InvoiceFieldAmount:
		return 1
	case InvoiceFieldPaid:
		return -1
	case InvoiceFieldPeer:
		return 2
	case InvoiceFieldInvoiceDate:
		return 3
	case InvoiceFieldDueDate:
		return 4
	case InvoiceFieldTargetAccountChoice:
		return -1
	case InvoiceFieldTargetAccountName:
		return 5
	case InvoiceFieldURL:
		return 6
	case InvoiceFieldDescription:
		return 7
	default:
		return -1
	}
}

func (f AddInvoiceForm) FocusActive() AddInvoiceForm {
	for i := range f.Inputs {
		f.Inputs[i].Blur()
	}

	if inputIndex := f.InputIndexForField(f.Active); inputIndex >= 0 {
		f.Inputs[inputIndex].Focus()
	}

	return f
}

func (f AddInvoiceForm) Next() AddInvoiceForm {
	if f.Active < InvoiceFieldCount-1 {
		f.Active++
	}

	return f.FocusActive()
}

func (f AddInvoiceForm) Prev() AddInvoiceForm {
	if f.Active > 0 {
		f.Active--
	}

	return f.FocusActive()
}
