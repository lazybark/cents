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

// IncomeCategoryOptions are the income categories new entries can use:
// every one that isn't archived.
func (s AppSettings) IncomeCategoryOptions() []string {
	items := make([]string, 0, len(s.IncomeCategories))
	for _, category := range s.IncomeCategories {
		if name := strings.TrimSpace(category.CategoryName); name != "" && !category.Archived {
			items = append(items, name)
		}
	}

	return items
}

// ExpenseCategoryOptions are the expense categories new entries can use:
// every one that isn't archived.
func (s AppSettings) ExpenseCategoryOptions() []string {
	items := make([]string, 0, len(s.ExpenseCategories))
	for _, category := range s.ExpenseCategories {
		if name := strings.TrimSpace(category.CategoryName); name != "" && !category.Archived {
			items = append(items, name)
		}
	}

	return items
}

// IsArchivedCategory reports whether an income (or expense) entry's
// category is archived. Names match ignoring case and spaces, like the
// statistics group them.
func (s AppSettings) IsArchivedCategory(isIncome bool, name string) bool {
	key := strings.ToLower(strings.TrimSpace(name))

	if isIncome {
		for _, category := range s.IncomeCategories {
			if category.Archived && strings.ToLower(strings.TrimSpace(category.CategoryName)) == key {
				return true
			}
		}

		return false
	}

	for _, category := range s.ExpenseCategories {
		if category.Archived && strings.ToLower(strings.TrimSpace(category.CategoryName)) == key {
			return true
		}
	}

	return false
}
