package subscription

import "github.com/charmbracelet/bubbles/textinput"

func NewEditSubscriptionForm() EditSubscriptionForm {
	amountInput := textinput.New()
	amountInput.Placeholder = "9.99"
	amountInput.CharLimit = 24
	amountInput.Width = 20

	paymentMethodInput := textinput.New()
	paymentMethodInput.Placeholder = "Card **** 1234"
	paymentMethodInput.CharLimit = 80
	paymentMethodInput.Width = 28

	return EditSubscriptionForm{
		AmountInput:        amountInput,
		PaymentMethodInput: paymentMethodInput,
		IsActive:           true,
		ActiveField:        0,
	}
}

func (f EditSubscriptionForm) FocusActive() EditSubscriptionForm {
	f.AmountInput.Blur()
	f.PaymentMethodInput.Blur()

	if f.ActiveField == EditSubFieldAmount {
		f.AmountInput.Focus()
	}

	if f.ActiveField == EditSubFieldPaymentMethod {
		f.PaymentMethodInput.Focus()
	}

	return f
}

func (f EditSubscriptionForm) Next() EditSubscriptionForm {
	if f.ActiveField < EditSubFieldCount-1 {
		f.ActiveField++
	}

	return f.FocusActive()
}

func (f EditSubscriptionForm) Prev() EditSubscriptionForm {
	if f.ActiveField > 0 {
		f.ActiveField--
	}

	return f.FocusActive()
}
