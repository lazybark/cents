package desktop

import (
	"io/fs"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// Wails binds the exported methods of *API, the embedded Backend's
// included: every one the frontend calls must be there, and nothing that
// can't cross to JS.
func TestFrontendCallsAreBound(t *testing.T) {
	bound := reflect.TypeFor[*API]()
	call := regexp.MustCompile(`\bapi\.([A-Z]\w*)\(`)

	called := map[string]bool{}
	err := fs.WalkDir(assets, "frontend/js", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".js") {
			return err
		}

		data, err := fs.ReadFile(assets, path)
		for _, m := range call.FindAllStringSubmatch(string(data), -1) {
			called[m[1]] = true
		}

		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(called) < 50 {
		t.Fatalf("expected the frontend's calls, found %d", len(called))
	}

	for name := range called {
		if _, ok := bound.MethodByName(name); !ok {
			t.Errorf("the frontend calls %s, which isn't bound", name)
		}
	}

	for _, name := range []string{"WriteBackup", "WriteCSV", "KeepRatesFresh", "WithRates"} {
		if _, ok := bound.MethodByName(name); ok {
			t.Errorf("%s shouldn't be bound", name)
		}
	}
}
