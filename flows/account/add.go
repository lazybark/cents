package account

import "github.com/charmbracelet/bubbles/textinput"

func NewAddAccountForm(currencyOptions []string) AddAccountForm {
	labels := []string{"Name", "Description", "Currency", "Amount"}
	placeholders := []string{"Emergency Fund", "Rainy day savings", "", "2500.00"}
	fields := make([]textinput.Model, len(labels))

	for i := range fields {
		field := textinput.New()
		field.Placeholder = placeholders[i]
		field.CharLimit = 80
		field.Width = 34
		fields[i] = field
	}

	form := AddAccountForm{Fields: fields, Labels: labels, CurrencyOptions: append([]string(nil), currencyOptions...), CurrencyIndex: 0, IgnoreInSummaries: false}

	return form.FocusActive()
}

func (f AddAccountForm) FocusActive() AddAccountForm {
	for i := range f.Fields {
		if i == f.Active {
			f.Fields[i].Focus()
		} else {
			f.Fields[i].Blur()
		}
	}

	return f
}

func (f AddAccountForm) Next() AddAccountForm {
	if f.Active < len(f.Fields) {
		f.Active++
	}

	return f.FocusActive()
}

func (f AddAccountForm) Prev() AddAccountForm {
	if f.Active > 0 {
		f.Active--
	}

	return f.FocusActive()
}
