package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestAdminTokenConfiguration(t *testing.T) {
	cmd := root()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"token", "new", "--name", "admin", "--admin", "--read-only"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"name: admin", "sha256:", "admin: true", "read_only: true"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("generated config missing %q", want)
		}
	}
}
