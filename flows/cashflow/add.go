package cashflow

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
)

func NewAddCashflowForm(currencyOptions []string, categoryOptions []string, accountOptions []string, isIncome bool) AddCashflowForm {
	today := time.Now().Format("02.01.2006")
	inputs := make([]textinput.Model, 3)
	placeholders := []string{"1000.00", today, "optional comment"}

	for i := range inputs {
		field := textinput.New()
		field.Placeholder = placeholders[i]
		field.CharLimit = 140
		field.Width = 34

		if i == 1 {
			field.SetValue(today)
		}

		inputs[i] = field
	}

	form := AddCashflowForm{
		Inputs:          inputs,
		Active:          0,
		CurrencyOptions: append([]string(nil), currencyOptions...),
		CurrencyIndex:   0,
		IsIncome:        isIncome,
		CategoryOptions: append([]string(nil), categoryOptions...),
		CategoryIndex:   0,
		AccountOptions:  append([]string(nil), accountOptions...),
		AccountIndex:    0,
	}

	return form.FocusActive()
}

func (f AddCashflowForm) InputIndexForField(field int) int {
	switch field {
	case CashflowFieldCurrency:
		return -1
	case CashflowFieldAmount:
		return 0
	case CashflowFieldDate:
		return 1
	case CashflowFieldCategory:
		return -1
	case CashflowFieldAccount:
		return -1
	case CashflowFieldComment:
		return 2
	default:
		return -1
	}
}

func (f AddCashflowForm) FocusActive() AddCashflowForm {
	for i := range f.Inputs {
		f.Inputs[i].Blur()
	}

	if inputIndex := f.InputIndexForField(f.Active); inputIndex >= 0 {
		f.Inputs[inputIndex].Focus()
	}

	return f
}

func (f AddCashflowForm) Next() AddCashflowForm {
	if f.Active < CashflowFieldCount-1 {
		f.Active++
	}

	return f.FocusActive()
}

func (f AddCashflowForm) Prev() AddCashflowForm {
	if f.Active > 0 {
		f.Active--
	}

	return f.FocusActive()
}
