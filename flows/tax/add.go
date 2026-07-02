package tax

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/lazybark/cents/flows/settings"
)

func NewAddTaxForm(taxTypeOptions []settings.SettingTaxType) AddTaxForm {
	today := time.Now().Format("02.01.2006")
	inputs := make([]textinput.Model, 5)
	placeholders := []string{"1000.00", "0.00", "Q1 2026", today, "Optional comment"}

	for i := range inputs {
		field := textinput.New()
		field.Placeholder = placeholders[i]
		field.CharLimit = 120
		field.Width = 34

		if i == 3 {
			field.SetValue(today)
		}

		inputs[i] = field
	}

	display := make([]string, 0, len(taxTypeOptions))

	for _, item := range taxTypeOptions {
		label := strings.TrimSpace(item.Country)
		name := strings.TrimSpace(item.TaxTypeName)

		if label == "" {
			label = "Unknown"
		}

		if name == "" {
			name = "Tax"
		}

		display = append(display, label+" / "+name)
	}

	form := AddTaxForm{
		Inputs:          inputs,
		Active:          0,
		TaxTypeOptions:  append([]settings.SettingTaxType(nil), taxTypeOptions...),
		TaxTypeIndex:    0,
		TaxDisplayNames: display,
	}

	return form.FocusActive()
}

func (f AddTaxForm) InputIndexForField(field int) int {
	switch field {
	case TaxFieldTaxType:
		return -1
	case TaxFieldAmountDue:
		return 0
	case TaxFieldAmountPaid:
		return 1
	case TaxFieldPeriod:
		return 2
	case TaxFieldDueDate:
		return 3
	case TaxFieldComment:
		return 4
	default:
		return -1
	}
}

func (f AddTaxForm) FocusActive() AddTaxForm {
	for i := range f.Inputs {
		f.Inputs[i].Blur()
	}

	if inputIndex := f.InputIndexForField(f.Active); inputIndex >= 0 {
		f.Inputs[inputIndex].Focus()
	}

	return f
}

func (f AddTaxForm) Next() AddTaxForm {
	if f.Active < TaxFieldCount-1 {
		f.Active++
	}

	return f.FocusActive()
}

func (f AddTaxForm) Prev() AddTaxForm {
	if f.Active > 0 {
		f.Active--
	}

	return f.FocusActive()
}
