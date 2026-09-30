package tag

import (
	"strings"
	"testing"
)

func TestClean(t *testing.T) {
	got, err := Clean([]string{" Trip  to Japan ", "gifts", "trip to japan", "", "Gifts"})
	if err != nil || strings.Join(got, "|") != "gifts|Trip to Japan" {
		t.Fatalf("unexpected %v %v", got, err)
	}

	for _, bad := range [][]string{{"a,b"}, {strings.Repeat("x", MaxLength+1)}} {
		if _, err := Clean(bad); err == nil {
			t.Errorf("expected %v to fail", bad)
		}
	}

	if _, err := CleanName("   "); err == nil {
		t.Fatal("expected an empty name to fail")
	}

	if !Has([]string{"Trip to Japan"}, " trip TO japan") || Has(nil, "x") {
		t.Fatal("Has ignores case")
	}
}
