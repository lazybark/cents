package debt

import "github.com/charmbracelet/bubbles/textinput"

func NewEditDebtForm() EditDebtForm {
	amountInput := textinput.New()
	amountInput.Placeholder = "1000.00"
	amountInput.CharLimit = 24
	amountInput.Width = 20

	amountPaidInput := textinput.New()
	amountPaidInput.Placeholder = "0.00"
	amountPaidInput.CharLimit = 24
	amountPaidInput.Width = 20

	debtCreatedInput := textinput.New()
	debtCreatedInput.Placeholder = "02.01.2006"
	debtCreatedInput.CharLimit = 24
	debtCreatedInput.Width = 20

	dueDateInput := textinput.New()
	dueDateInput.Placeholder = "optional DD.MM.YYYY"
	dueDateInput.CharLimit = 24
	dueDateInput.Width = 20

	commentInput := textinput.New()
	commentInput.Placeholder = "comment"
	commentInput.CharLimit = 120
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

	form := EditDebtForm{
		AmountInput:      amountInput,
		AmountPaidInput:  amountPaidInput,
		DebtCreatedInput: debtCreatedInput,
		DueDateInput:     dueDateInput,
		CommentInput:     commentInput,
		LogDeltaInput:    logDeltaInput,
		LogDateInput:     logDateInput,
		LogCommentInput:  logCommentInput,
		ActiveField:      0,
	}

	return form.FocusActive()
}

func (f EditDebtForm) FocusActive() EditDebtForm {
	f.AmountInput.Blur()
	f.AmountPaidInput.Blur()
	f.DebtCreatedInput.Blur()
	f.DueDateInput.Blur()
	f.CommentInput.Blur()
	f.LogDeltaInput.Blur()
	f.LogDateInput.Blur()
	f.LogCommentInput.Blur()

	switch f.ActiveField {
	case EditDebtFieldAmount:
		f.AmountInput.Focus()
	case EditDebtFieldAmountPaid:
		f.AmountPaidInput.Focus()
	case EditDebtFieldDebtCreated:
		f.DebtCreatedInput.Focus()
	case EditDebtFieldDueDate:
		f.DueDateInput.Focus()
	case EditDebtFieldComment:
		f.CommentInput.Focus()
	case EditDebtFieldLogDelta:
		f.LogDeltaInput.Focus()
	case EditDebtFieldLogDate:
		f.LogDateInput.Focus()
	case EditDebtFieldLogComment:
		f.LogCommentInput.Focus()
	}

	return f
}

func (f EditDebtForm) Next() EditDebtForm {
	if f.ActiveField < EditDebtFieldCount-1 {
		f.ActiveField++
	}

	return f.FocusActive()
}

func (f EditDebtForm) Prev() EditDebtForm {
	if f.ActiveField > 0 {
		f.ActiveField--
	}

	return f.FocusActive()
}
