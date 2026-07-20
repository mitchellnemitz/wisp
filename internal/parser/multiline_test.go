package parser

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
)

// arrayLitFromLet parses `wrap("let xs: int[] = " + rhs)` and returns the ArrayLit.
func arrayLitFromLet(t *testing.T, rhs string) *ast.ArrayLit {
	t.Helper()
	prog := parseOK(t, wrap("let xs: int[] = "+rhs))
	let0 := prog.Funcs[0].Body[0].(*ast.LetStmt)
	al, ok := let0.Value.(*ast.ArrayLit)
	if !ok {
		t.Fatalf("expected ArrayLit, got %T", let0.Value)
	}
	return al
}

func TestParseArrayLitOneLineForms(t *testing.T) {
	cases := []struct {
		rhs   string
		nelem int
	}{
		{"[]", 0},
		{"[1]", 1},
		{"[1, 2, 3]", 3},
		{"[1, 2, 3,]", 3},
	}
	for _, c := range cases {
		al := arrayLitFromLet(t, c.rhs)
		if len(al.Elems) != c.nelem {
			t.Errorf("%s: elems = %d, want %d", c.rhs, len(al.Elems), c.nelem)
		}
		if al.Multiline {
			t.Errorf("%s: Multiline = true, want false", c.rhs)
		}
	}
}

func TestParseArrayLitMultilineForms(t *testing.T) {
	cases := []struct {
		name  string
		rhs   string
		nelem int
	}{
		{"newline-comma", "[\n 1,\n 2,\n 3,\n]", 3},
		{"newline-only", "[\n 1\n 2\n 3\n]", 3},
		{"mixed", "[1,\n 2]", 2},
	}
	for _, c := range cases {
		al := arrayLitFromLet(t, c.rhs)
		if len(al.Elems) != c.nelem {
			t.Errorf("%s: elems = %d, want %d", c.name, len(al.Elems), c.nelem)
		}
		if !al.Multiline {
			t.Errorf("%s: Multiline = false, want true", c.name)
		}
	}
}

func TestParseArrayLitCommentLineAllowed(t *testing.T) {
	al := arrayLitFromLet(t, "[\n 1,\n // a comment\n 2,\n]")
	if len(al.Elems) != 2 {
		t.Errorf("elems = %d, want 2", len(al.Elems))
	}
	if !al.Multiline {
		t.Error("Multiline = false, want true")
	}
}

func TestParseArrayLitMissingSeparatorErrors(t *testing.T) {
	err := parseErr(t, wrap("let xs: int[] = [1 2 3]"))
	errAt(t, err, "expected ',' or newline between array elements")
}

func TestParseArrayLitLeadingCommaErrors(t *testing.T) {
	parseErr(t, wrap("let xs: int[] = [,]"))
	parseErr(t, wrap("let xs: int[] = [,1]"))
}

func TestParseArrayLitDoubleCommaErrors(t *testing.T) {
	parseErr(t, wrap("let xs: int[] = [1,,2]"))
}

func TestParseArrayLitUnterminatedErrors(t *testing.T) {
	// No wrapper: the `[` is the last construct so the loop reaches real EOF.
	err := parseErr(t, "fn main() -> int {\n let xs: int[] = [\n 1,\n")
	errAt(t, err, "expected ']', got end of input")
}

func TestParseDictLitSemicolonNotMultiline(t *testing.T) {
	prog := parseOK(t, wrap("let m: {string: int} = { \"a\": 1; \"b\": 2 }"))
	let0 := prog.Funcs[0].Body[0].(*ast.LetStmt)
	dl, ok := let0.Value.(*ast.DictLit)
	if !ok || len(dl.Entries) != 2 {
		t.Fatalf("expected DictLit of 2, got %T", let0.Value)
	}
	if dl.Multiline {
		t.Error("Multiline = true for `;`-separated one-line dict, want false")
	}
}

func TestParseDictLitNewlineMultiline(t *testing.T) {
	prog := parseOK(t, wrap("let m: {string: int} = {\n \"a\": 1\n \"b\": 2\n }"))
	dl := prog.Funcs[0].Body[0].(*ast.LetStmt).Value.(*ast.DictLit)
	if !dl.Multiline {
		t.Error("Multiline = false for newline-separated dict, want true")
	}
}

func TestParseDictLitOneLineCommaNotMultiline(t *testing.T) {
	prog := parseOK(t, wrap("let m: {string: int} = { \"a\": 1, \"b\": 2 }"))
	dl := prog.Funcs[0].Body[0].(*ast.LetStmt).Value.(*ast.DictLit)
	if dl.Multiline {
		t.Error("Multiline = true for one-line comma dict, want false")
	}
}

func TestParseStructLitMultiline(t *testing.T) {
	prog := parseOK(t, "struct P { x: int, y: int }\nfn main() -> int { let p: P = P {\n x: 1\n y: 2\n }\n return 0 }")
	sl := prog.Funcs[0].Body[0].(*ast.LetStmt).Value.(*ast.StructLit)
	if !sl.Multiline {
		t.Error("Multiline = false for newline struct lit, want true")
	}
}

func TestParseStructLitOneLineNotMultiline(t *testing.T) {
	prog := parseOK(t, "struct P { x: int, y: int }\nfn main() -> int { let p: P = P { x: 1, y: 2 }\n return 0 }")
	sl := prog.Funcs[0].Body[0].(*ast.LetStmt).Value.(*ast.StructLit)
	if sl.Multiline {
		t.Error("Multiline = true for one-line struct lit, want false")
	}
}

func TestParseStructDeclMultiline(t *testing.T) {
	prog := parseOK(t, "struct P {\n x: int\n y: int\n}\nfn main() -> int { return 0 }")
	if !prog.Structs[0].Multiline {
		t.Error("Multiline = false for newline struct decl, want true")
	}
}

func TestParseStructDeclOneLineNotMultiline(t *testing.T) {
	prog := parseOK(t, "struct P { x: int, y: int }\nfn main() -> int { return 0 }")
	if prog.Structs[0].Multiline {
		t.Error("Multiline = true for one-line struct decl, want false")
	}
}
