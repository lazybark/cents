package subscription

import "github.com/charmbracelet/bubbles/textinput"

func NewAddSubscriptionForm(currencyOptions []string, paymentMethodOptions []string) AddSubscriptionForm {
	inputs := make([]textinput.Model, 6)
	placeholders := []string{"GitHub", "", "9.99", "Card **** 1234", "12.12.2012", "18"}

	for i := range inputs {
		field := textinput.New()
		field.Placeholder = placeholders[i]
		field.CharLimit = 80
		field.Width = 28
		inputs[i] = field
	}

	form := AddSubscriptionForm{
		Inputs:               inputs,
		Active:               0,
		CurrencyOptions:      append([]string(nil), currencyOptions...),
		CurrencyIndex:        0,
		PaymentMethodOptions: append([]string(nil), paymentMethodOptions...),
		PaymentMethodIndex:   0,
		PeriodOptions:        []string{"month", "year"},
		PeriodIndex:          0,
		TypeOptions:          []string{"Software", "Domain", "Service", "Multimedia", "Other"},
		TypeIndex:            0,
		IsActive:             true,
	}

	return form.FocusActive()
}

func (f AddSubscriptionForm) InputIndexForField(field int) int {
	switch field {
	case SubFieldName:
		return 0
	case SubFieldCurrency:
		return -1
	case SubFieldAmount:
		return 2
	case SubFieldPaymentMethodChoice:
		return -1
	case SubFieldPaymentMethod:
		return 3
	case SubFieldDayYearly:
		return 4
	case SubFieldDayMonthly:
		return 5
	default:
		return -1
	}
}

func (f AddSubscriptionForm) FocusActive() AddSubscriptionForm {
	for i := range f.Inputs {
		f.Inputs[i].Blur()
	}

	if inputIndex := f.InputIndexForField(f.Active); inputIndex >= 0 {
		f.Inputs[inputIndex].Focus()
	}

	return f
}

func (f AddSubscriptionForm) Next() AddSubscriptionForm {
	if f.Active < SubFieldCount-1 {
		f.Active++
	}

	return f.FocusActive()
}

func (f AddSubscriptionForm) Prev() AddSubscriptionForm {
	if f.Active > 0 {
		f.Active--
	}

	return f.FocusActive()
}
