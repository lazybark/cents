package invoice

import "strings"

func invoiceCurrencySelectionOptions(currencyOptions []string) []string {
	options := []string{""}
	seen := map[string]struct{}{"": {}}

	for _, item := range currencyOptions {
		value := strings.TrimSpace(item)

		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}

		seen[key] = struct{}{}
		options = append(options, value)
	}

	return options
}
