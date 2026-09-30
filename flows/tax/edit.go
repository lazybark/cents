package tax

import "github.com/charmbracelet/bubbles/textinput"

func NewEditTaxForm() EditTaxForm {
	amountDueInput := textinput.New()
	amountDueInput.Placeholder = "1000.00"
	amountDueInput.CharLimit = 24
	amountDueInput.Width = 20

	amountPaidInput := textinput.New()
	amountPaidInput.Placeholder = "0.00"
	amountPaidInput.CharLimit = 24
	amountPaidInput.Width = 20

	periodInput := textinput.New()
	periodInput.Placeholder = "Q1 2026"
	periodInput.CharLimit = 60
	periodInput.Width = 24

	dueDateInput := textinput.New()
	dueDateInput.Placeholder = "optional DD.MM.YYYY"
	dueDateInput.CharLimit = 24
	dueDateInput.Width = 20

	commentInput := textinput.New()
	commentInput.Placeholder = "comment"
	commentInput.CharLimit = 140
	commentInput.Width = 36

	logDeltaInput := textinput.New()
	logDeltaInput.Placeholder = "+10.00 or -5.00"
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

	form := EditTaxForm{
		AmountDueInput:  amountDueInput,
		AmountPaidInput: amountPaidInput,
		PeriodInput:     periodInput,
		DueDateInput:    dueDateInput,
		CommentInput:    commentInput,
		LogDeltaInput:   logDeltaInput,
		LogDateInput:    logDateInput,
		LogCommentInput: logCommentInput,
		ActiveField:     0,
	}

	return form.FocusActive()
}

func (f EditTaxForm) FocusActive() EditTaxForm {
	f.AmountDueInput.Blur()
	f.AmountPaidInput.Blur()
	f.PeriodInput.Blur()
	f.DueDateInput.Blur()
	f.CommentInput.Blur()
	f.LogDeltaInput.Blur()
	f.LogDateInput.Blur()
	f.LogCommentInput.Blur()

	switch f.ActiveField {
	case EditTaxFieldAmountDue:
		f.AmountDueInput.Focus()
	case EditTaxFieldAmountPaid:
		f.AmountPaidInput.Focus()
	case EditTaxFieldPeriod:
		f.PeriodInput.Focus()
	case EditTaxFieldDueDate:
		f.DueDateInput.Focus()
	case EditTaxFieldComment:
		f.CommentInput.Focus()
	case EditTaxFieldLogDelta:
		f.LogDeltaInput.Focus()
	case EditTaxFieldLogDate:
		f.LogDateInput.Focus()
	case EditTaxFieldLogComment:
		f.LogCommentInput.Focus()
	}
	return f
}

func (f EditTaxForm) Next() EditTaxForm {
	if f.ActiveField < EditTaxFieldCount-1 {
		f.ActiveField++
	}

	return f.FocusActive()
}

func (f EditTaxForm) Prev() EditTaxForm {
	if f.ActiveField > 0 {
		f.ActiveField--
	}

	return f.FocusActive()
}
