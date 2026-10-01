package settings

import "strings"

// PaymentMethodOptions lists payment method names with the default first,
// falling back to "Other" when none are configured.
func (s AppSettings) PaymentMethodOptions() []string {
	options := make([]string, 0, len(s.PaymentMethods))
	for _, method := range s.PaymentMethods {
		name := strings.TrimSpace(method.PaymentMethodName)
		if name == "" {
			continue
		}

		if method.IsDefault {
			options = append([]string{name}, options...)

			continue
		}

		options = append(options, name)
	}

	if len(options) == 0 {
		return []string{"Other"}
	}

	return options
}

func (s AppSettings) IncomeCategoryOptions() []string {
	items := make([]string, 0, len(s.IncomeCategories))
	for _, category := range s.IncomeCategories {
		if name := strings.TrimSpace(category.CategoryName); name != "" {
			items = append(items, name)
		}
	}

	return items
}

func (s AppSettings) ExpenseCategoryOptions() []string {
	items := make([]string, 0, len(s.ExpenseCategories))
	for _, category := range s.ExpenseCategories {
		if name := strings.TrimSpace(category.CategoryName); name != "" {
			items = append(items, name)
		}
	}

	return items
}
