package goal

import "github.com/charmbracelet/bubbles/textinput"

type EditGoalForm struct {
	TargetAmountInput      textinput.Model
	AccumulatedAmountInput textinput.Model
	DateStartedInput       textinput.Model
	TargetDateInput        textinput.Model
	DescriptionInput       textinput.Model
	LogDeltaInput          textinput.Model
	LogDateInput           textinput.Model
	LogCommentInput        textinput.Model
	ActiveField            int
	NameLabel              string
	CurrencyLabel          string
}

func NewEditGoalForm() EditGoalForm {
	targetAmountInput := textinput.New()
	targetAmountInput.Placeholder = "10000.00"
	targetAmountInput.CharLimit = 24
	targetAmountInput.Width = 20

	accumulatedAmountInput := textinput.New()
	accumulatedAmountInput.Placeholder = "0.00"
	accumulatedAmountInput.CharLimit = 24
	accumulatedAmountInput.Width = 20

	dateStartedInput := textinput.New()
	dateStartedInput.Placeholder = "02.01.2006"
	dateStartedInput.CharLimit = 24
	dateStartedInput.Width = 20

	targetDateInput := textinput.New()
	targetDateInput.Placeholder = "optional DD.MM.YYYY"
	targetDateInput.CharLimit = 24
	targetDateInput.Width = 24

	descriptionInput := textinput.New()
	descriptionInput.Placeholder = "description"
	descriptionInput.CharLimit = 120
	descriptionInput.Width = 36

	logDeltaInput := textinput.New()
	logDeltaInput.Placeholder = "+100.00 or -50.00"
	logDeltaInput.CharLimit = 24
	logDeltaInput.Width = 24

	logDateInput := textinput.New()
	logDateInput.Placeholder = "DD.MM.YYYY (optional)"
	logDateInput.CharLimit = 24
	logDateInput.Width = 24

	logCommentInput := textinput.New()
	logCommentInput.Placeholder = "transaction note (optional)"
	logCommentInput.CharLimit = 120
	logCommentInput.Width = 36

	form := EditGoalForm{
		TargetAmountInput:      targetAmountInput,
		AccumulatedAmountInput: accumulatedAmountInput,
		DateStartedInput:       dateStartedInput,
		TargetDateInput:        targetDateInput,
		DescriptionInput:       descriptionInput,
		LogDeltaInput:          logDeltaInput,
		LogDateInput:           logDateInput,
		LogCommentInput:        logCommentInput,
		ActiveField:            0,
	}

	return form.FocusActive()
}

func (f EditGoalForm) FocusActive() EditGoalForm {
	f.TargetAmountInput.Blur()
	f.AccumulatedAmountInput.Blur()
	f.DateStartedInput.Blur()
	f.TargetDateInput.Blur()
	f.DescriptionInput.Blur()
	f.LogDeltaInput.Blur()
	f.LogDateInput.Blur()
	f.LogCommentInput.Blur()

	switch f.ActiveField {
	case EditGoalFieldTargetAmount:
		f.TargetAmountInput.Focus()
	case EditGoalFieldAccumulated:
		f.AccumulatedAmountInput.Focus()
	case EditGoalFieldDateStarted:
		f.DateStartedInput.Focus()
	case EditGoalFieldTargetDate:
		f.TargetDateInput.Focus()
	case EditGoalFieldDescription:
		f.DescriptionInput.Focus()
	case EditGoalFieldLogDelta:
		f.LogDeltaInput.Focus()
	case EditGoalFieldLogDate:
		f.LogDateInput.Focus()
	case EditGoalFieldLogComment:
		f.LogCommentInput.Focus()
	}

	return f
}

func (f EditGoalForm) Next() EditGoalForm {
	if f.ActiveField < EditGoalFieldCount-1 {
		f.ActiveField++
	}

	return f.FocusActive()
}

func (f EditGoalForm) Prev() EditGoalForm {
	if f.ActiveField > 0 {
		f.ActiveField--
	}

	return f.FocusActive()
}
