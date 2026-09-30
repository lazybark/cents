package debt

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
)

func NewAddDebtForm(currencyOptions []string) AddDebtForm {
	inputs := make([]textinput.Model, 6)
	placeholders := []string{"John Doe", "1000.00", "0.00", time.Now().Format("02.01.2006"), "", "Optional comment"}

	for i := range inputs {
		field := textinput.New()
		field.Placeholder = placeholders[i]
		field.CharLimit = 120
		field.Width = 34
		inputs[i] = field
	}

	form := AddDebtForm{
		Inputs:          inputs,
		Active:          0,
		CurrencyOptions: append([]string(nil), currencyOptions...),
		CurrencyIndex:   0,
		IsOwedToUser:    false,
	}

	return form.FocusActive()
}

func (f AddDebtForm) InputIndexForField(field int) int {
	switch field {
	case DebtFieldDirection:
		return -1
	case DebtFieldPeer:
		return 0
	case DebtFieldCurrency:
		return -1
	case DebtFieldAmount:
		return 1
	case DebtFieldAmountPaid:
		return 2
	case DebtFieldDebtCreated:
		return 3
	case DebtFieldDueDate:
		return 4
	case DebtFieldComment:
		return 5
	default:
		return -1
	}
}

func (f AddDebtForm) FocusActive() AddDebtForm {
	for i := range f.Inputs {
		f.Inputs[i].Blur()
	}

	if inputIndex := f.InputIndexForField(f.Active); inputIndex >= 0 {
		f.Inputs[inputIndex].Focus()
	}

	return f
}

func (f AddDebtForm) Next() AddDebtForm {
	if f.Active < DebtFieldCount-1 {
		f.Active++
	}

	return f.FocusActive()
}

func (f AddDebtForm) Prev() AddDebtForm {
	if f.Active > 0 {
		f.Active--
	}

	return f.FocusActive()
}
