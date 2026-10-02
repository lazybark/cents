package account

import (
	"sort"
	"strings"
)

// PickerNames are the account names new records can pick: every account
// with a name that isn't archived.
func PickerNames(accounts []Account) []string {
	names := make([]string, 0, len(accounts))
	for _, acct := range accounts {
		if name := strings.TrimSpace(acct.Name); name != "" && !acct.Archived {
			names = append(names, name)
		}
	}

	return names
}

// ArchivedLast moves archived accounts after the others, keeping each
// group's order.
func ArchivedLast(accounts []Account) []Account {
	sorted := append([]Account(nil), accounts...)
	sort.SliceStable(sorted, func(i, j int) bool { return !sorted[i].Archived && sorted[j].Archived })

	return sorted
}
