package goal

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
)

func NewAddGoalForm(currencyOptions []string) AddGoalForm {
	today := time.Now().Format("02.01.2006")
	inputs := make([]textinput.Model, 6)
	placeholders := []string{"Emergency Fund", "10000.00", "0.00", "Optional description", today, "optional DD.MM.YYYY"}

	for i := range inputs {
		field := textinput.New()
		field.Placeholder = placeholders[i]
		field.CharLimit = 120
		field.Width = 34

		if i == 4 {
			field.SetValue(today)
		}

		inputs[i] = field
	}

	form := AddGoalForm{
		Inputs:          inputs,
		Active:          0,
		CurrencyOptions: append([]string(nil), currencyOptions...),
		CurrencyIndex:   0,
	}

	return form.FocusActive()
}

func (f AddGoalForm) InputIndexForField(field int) int {
	switch field {
	case GoalFieldName:
		return 0
	case GoalFieldCurrency:
		return -1
	case GoalFieldTargetAmount:
		return 1
	case GoalFieldAccumulated:
		return 2
	case GoalFieldDescription:
		return 3
	case GoalFieldDateStarted:
		return 4
	case GoalFieldTargetDate:
		return 5
	default:
		return -1
	}
}

func (f AddGoalForm) FocusActive() AddGoalForm {
	for i := range f.Inputs {
		f.Inputs[i].Blur()
	}

	if inputIndex := f.InputIndexForField(f.Active); inputIndex >= 0 {
		f.Inputs[inputIndex].Focus()
	}

	return f
}

func (f AddGoalForm) Next() AddGoalForm {
	if f.Active < GoalFieldCount-1 {
		f.Active++
	}

	return f.FocusActive()
}

func (f AddGoalForm) Prev() AddGoalForm {
	if f.Active > 0 {
		f.Active--
	}

	return f.FocusActive()
}
