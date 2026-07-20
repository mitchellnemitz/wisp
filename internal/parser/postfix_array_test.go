package parser

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
)

// typeOf parses `let x: <ts> = 0` and returns the parsed annotation. The value
// need not typecheck; Parse is parse-only.
func typeOf(t *testing.T, ts string) ast.TypeName {
	t.Helper()
	prog := parseOK(t, wrap("let x: "+ts+" = 0"))
	return prog.Funcs[0].Body[0].(*ast.LetStmt).Type
}

// TestParsePostfixArray asserts postfix `T[]` produces the expected internal
// `"[T]"` encoding (the surface-only invariant).
func TestParsePostfixArray(t *testing.T) {
	cases := []struct {
		src  string
		want ast.TypeName
	}{
		{"string[]", ast.ArrayType(ast.TypeString)},
		{"int[][]", ast.ArrayType(ast.ArrayType(ast.TypeInt))},
		{"int[][][]", ast.ArrayType(ast.ArrayType(ast.ArrayType(ast.TypeInt)))},
		{"(int, string)[]", ast.ArrayType(ast.TupleType([]ast.TypeName{ast.TypeInt, ast.TypeString}))},
		{"{string: int[]}", ast.DictType(ast.TypeString, ast.ArrayType(ast.TypeInt))},
		{"Box[string[]]", ast.TypeName("Box[" + string(ast.ArrayType(ast.TypeString)) + "]")},
		{"Box[int][]", ast.ArrayType(ast.TypeName("Box[int]"))},
		{"Optional[int[]]", ast.OptionalType(ast.ArrayType(ast.TypeInt))},
	}
	for _, c := range cases {
		if got := typeOf(t, c.src); got != c.want {
			t.Errorf("typeOf(%q) = %q, want %q", c.src, got, c.want)
		}
	}
}

// TestParsePrefixArrayRejected: after the migration, a leading `[` in type
// position is a parse error (arrays are postfix only; no dual-accept remains).
func TestParsePrefixArrayRejected(t *testing.T) {
	for _, ann := range []string{"[int]", "[[int]]", "[string]", "[Point]"} {
		if _, err := Parse(wrap("let x: "+ann+" = 0"), "test.wisp"); err == nil {
			t.Errorf("prefix %q: expected parse error, got none", ann)
		}
	}
	// Nested prefix inside an otherwise-valid generic is also rejected.
	if _, err := Parse(wrap("let x: Optional[[int]] = 0"), "test.wisp"); err == nil {
		t.Errorf("Optional[[int]]: expected parse error, got none")
	}
}

// TestParseParenGrouping: single-element `(T)` unwraps to T (not a 1-tuple).
func TestParseParenGrouping(t *testing.T) {
	if got := typeOf(t, "(int)"); got != ast.TypeInt {
		t.Errorf("(int) = %q, want int", got)
	}
	if got := typeOf(t, "((int))"); got != ast.TypeInt {
		t.Errorf("((int)) = %q, want int", got)
	}
	if got := typeOf(t, "(int)[]"); got != ast.ArrayType(ast.TypeInt) {
		t.Errorf("(int)[] = %q, want [int]", got)
	}
}

// TestParseArrayOfFuncrefPrecedence pins the precedence rule: grouping forces
// an array-of-funcref, distinct from a funcref returning an array.
func TestParseArrayOfFuncrefPrecedence(t *testing.T) {
	arrOfFn := typeOf(t, "(fn(int) -> string)[]")
	fnRetArr := typeOf(t, "fn(int) -> string[]")
	wantArrOfFn := ast.ArrayType(ast.FuncType([]ast.TypeName{ast.TypeInt}, ast.TypeString))
	wantFnRetArr := ast.FuncType([]ast.TypeName{ast.TypeInt}, ast.ArrayType(ast.TypeString))
	if arrOfFn != wantArrOfFn {
		t.Errorf("(fn(int) -> string)[] = %q, want %q", arrOfFn, wantArrOfFn)
	}
	if fnRetArr != wantFnRetArr {
		t.Errorf("fn(int) -> string[] = %q, want %q", fnRetArr, wantFnRetArr)
	}
	if arrOfFn == fnRetArr {
		t.Errorf("array-of-funcref and funcref-returning-array must differ, both = %q", arrOfFn)
	}
}

// TestParsePostfixArrayErrors: malformed postfix forms are parse errors.
func TestParsePostfixArrayErrors(t *testing.T) {
	// Unterminated `int[` (EOF before ]).
	if _, err := Parse(wrap("let x: int[ = 0"), "test.wisp"); err == nil {
		t.Errorf("int[ : expected parse error, got none")
	}
	// Non-empty bracket in postfix position after a non-generic atom.
	if _, err := Parse(wrap("let x: int[5] = 0"), "test.wisp"); err == nil {
		t.Errorf("int[5] : expected parse error, got none")
	}
}
