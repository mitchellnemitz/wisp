// internal/fuzz/print_test.go
package fuzz

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/driver"
)

func TestPrintProgramCompiles(t *testing.T) {
	p := &Program{
		Body: []Stmt{
			&LetStmt{Name: "v0", T: Type{Kind: KInt}, Init: &Binary{Op: "+", L: &IntLit{V: "7"}, R: &IntLit{V: "3"}, T: Type{Kind: KInt}}},
			&PrintStmt{Arg: &Var{Name: "v0", T: Type{Kind: KInt}}},
		},
	}
	src := Print(p)
	if !strings.Contains(src, "fn main()") {
		t.Fatalf("expected fn main, got:\n%s", src)
	}
	script, _, diags := driver.Compile("fuzz.wisp", src)
	for _, d := range diags {
		if d.Severity == driver.Error {
			t.Fatalf("did not compile: %s\n%s", d.String(), src)
		}
	}
	if len(script) == 0 {
		t.Fatalf("no script for:\n%s", src)
	}
}

func TestPrintTypeAnnotations(t *testing.T) {
	cases := []struct {
		typ  Type
		want string
	}{
		{Type{Kind: KInt}, "int"},
		{Type{Kind: KFloat}, "float"},
		{Type{Kind: KBool}, "bool"},
		{Type{Kind: KString}, "string"},
		{Type{Kind: KArray, Elem: &Type{Kind: KInt}}, "int[]"},
		{Type{Kind: KDict, Elem: &Type{Kind: KInt}}, "{string: int}"},
	}
	for _, c := range cases {
		if got := c.typ.String(); got != c.want {
			t.Errorf("Type.String() = %q, want %q", got, c.want)
		}
	}
}
