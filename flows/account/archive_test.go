package account

import "testing"

func TestArchivedAccountsLeavePickers(t *testing.T) {
	accounts := []Account{{Name: "Old card", Archived: true}, {Name: " Checking "}, {Name: ""}, {Name: "Savings"}}

	if got := PickerNames(accounts); len(got) != 2 || got[0] != "Checking" || got[1] != "Savings" {
		t.Fatalf("unexpected picker names %v", got)
	}

	sorted := ArchivedLast(accounts)
	if sorted[3].Name != "Old card" || sorted[0].Name != " Checking " || accounts[0].Name != "Old card" {
		t.Fatalf("archived should go last without changing the input: %+v", sorted)
	}
}
