package service

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// Each of the switch's methods passes the call on to the method of its own
// name, with its arguments in order.
func TestSwitch(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "switch.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	passed := 0
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || !fn.Name.IsExported() || fn.Name.Name == "To" {
			continue
		}

		ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
		if !ok || len(fn.Body.List) != 1 {
			t.Fatalf("%s should only pass the call on", fn.Name.Name)
		}

		call := ret.Results[0].(*ast.CallExpr)
		if name := call.Fun.(*ast.SelectorExpr).Sel.Name; name != fn.Name.Name {
			t.Fatalf("%s passes the call to %s", fn.Name.Name, name)
		}

		var params []string
		for _, field := range fn.Type.Params.List {
			for _, name := range field.Names {
				params = append(params, name.Name)
			}
		}

		if len(call.Args) != len(params) {
			t.Fatalf("%s passes %d arguments of %d", fn.Name.Name, len(call.Args), len(params))
		}

		for i, arg := range call.Args {
			if arg.(*ast.Ident).Name != params[i] {
				t.Fatalf("%s passes its arguments out of order", fn.Name.Name)
			}
		}

		passed++
	}

	if passed < 90 {
		t.Fatalf("expected every method checked, got %d", passed)
	}

	// Switching takes the next call elsewhere.
	first, second := newTestAPI(t), newTestAPI(t)
	if err := second.CreateAccount(NewAccountInput{Name: "Only here", Description: "x", Currency: "$", Amount: "1"}); err != nil {
		t.Fatal(err)
	}

	s := NewSwitch(first, first)
	if view, _ := s.Accounts(0); len(view.Accounts) != 0 {
		t.Fatalf("expected the first: %+v", view)
	}

	s.To(second, second)
	if view, _ := s.Accounts(0); len(view.Accounts) != 1 {
		t.Fatalf("expected the second: %+v", view)
	}

	var csv bytes.Buffer
	if rows, err := s.WriteCSV(&csv, "accounts", "", ""); err != nil || rows != 1 {
		t.Fatalf("expected the second's files: %d %v", rows, err)
	}
}
