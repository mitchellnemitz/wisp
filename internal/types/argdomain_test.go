package types

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/parser"
)

// exprOf parses `fn main() -> int { let _x = <expr>\n return 0 }` and returns the
// initializer expression of the let, so the probe can be exercised on real AST
// nodes the parser produces (IntLit, UnaryExpr, BinaryExpr).
func exprOf(t *testing.T, expr string) ast.Expr {
	t.Helper()
	src := "fn main() -> int {\n  let _x: int = " + expr + "\n  return 0\n}"
	prog, err := parser.Parse(src, "probe.wisp")
	if err != nil {
		t.Fatalf("parse %q: %v", expr, err)
	}
	fn := prog.Funcs[0]
	for _, st := range fn.Body {
		if ls, ok := st.(*ast.LetStmt); ok {
			return ls.Value
		}
	}
	t.Fatalf("no let initializer found in %q", expr)
	return nil
}

func TestConstIntProbe_Folds(t *testing.T) {
	cases := []struct {
		expr string
		want int64
	}{
		{"0", 0},
		{"42", 42},
		{"-1", -1},
		{"255", 255},
		{"256", 256},
		{"-9223372036854775808", wispIntMin}, // INT_MIN via unary minus over magnitude
		{"9223372036854775807", 9223372036854775807}, // INT_MAX
		{"64 + 1", 65}, // constant expression (AC2)
		{"2 * 200", 400},
		{"-(2 + 3)", -5}, // unary minus over a binary expr
		{"100 - 101", -1},
	}
	for _, tc := range cases {
		got, ok := constIntProbe(exprOf(t, tc.expr))
		if !ok || got != tc.want {
			t.Errorf("constIntProbe(%q) = (%d, %v), want (%d, true)", tc.expr, got, ok, tc.want)
		}
	}
}

func TestConstIntProbe_NotConstant(t *testing.T) {
	// Identifiers, calls, overflow, in-expression divide-by-zero, and string/bool
	// operands are NOT foldable: the probe returns ok==false (a sound
	// under-approximation; the runtime guard remains the enforcement).
	for _, expr := range []string{
		"x",                        // ident: not resolved by the probe
		"random(5)",                // call
		"9223372036854775807 + 1",  // overflow -> not folded
		"-9223372036854775808 - 1", // underflow
		"1 / 0",                    // in-expression div-by-zero -> not folded
		"1 % 0",
		"true", // bool
	} {
		if v, ok := constIntProbe(exprOf(t, expr)); ok {
			t.Errorf("constIntProbe(%q) = (%d, true), want ok=false", expr, v)
		}
	}
}

func TestArgDomain_DivModByZero(t *testing.T) {
	// Non-const context (E1 / R5): div or mod by a constant zero is a compile error.
	for _, expr := range []string{"5 / 0", "5 % 0", "5 / (1 - 1)", "5 % (2 - 2)"} {
		d := expectErr(t, wrapMain("let x: int = "+expr), "division by zero")
		if d.Pos.Line == 0 || d.Pos.Col == 0 {
			t.Errorf("%q: diagnostic not located (line=%d col=%d)", expr, d.Pos.Line, d.Pos.Col)
		}
	}
	// Non-zero constant divisors and non-constant divisors are accepted.
	expectOK(t, wrapMain("let x: int = 5 / 1"))
	expectOK(t, wrapMain("let x: int = 5 % 3"))
	expectOK(t, wrapMain("let d: int = 0\nlet x: int = 5 / d"))
}

func TestArgDomain_ConstDivByZeroStillFoldError(t *testing.T) {
	// The existing const-fold rejection is unchanged: `const C = 5 / 0` still
	// reports the const-fold message, and only one error is produced.
	info := check(t, wrapMain("const C: int = 5 / 0\nlet x: int = C"))
	got := 0
	for _, d := range info.Errors {
		if d.Msg == "constant expression: divide by zero" {
			got++
		}
	}
	if got != 1 {
		t.Errorf("const 5/0: want exactly 1 const-fold divide-by-zero error, got %d (all: %v)", got, info.Errors)
	}
}

func TestArgDomain_Builtins_Reject(t *testing.T) {
	// repeat/random/format_float/chr/remove_at/insert_at/abs/gcd/wait_any are now
	// removable; their arg-domain rejection is preserved namespaced in
	// core_strings/core_math/core_arrays/core_process/core_delegate_test.go. sleep
	// stays flat, so its const-domain rejection is asserted here.
	cases := []struct {
		src  string
		want string
	}{
		{`sleep(-1)`, "sleep: negative duration"},
	}
	for _, tc := range cases {
		d := expectErr(t, wrapMain(tc.src), tc.want)
		if d.Pos.Line == 0 || d.Pos.Col == 0 {
			t.Errorf("%q: diagnostic not located (line=%d col=%d)", tc.src, d.Pos.Line, d.Pos.Col)
		}
	}
}

func TestArgDomain_Builtins_Accept(t *testing.T) {
	// Accepted boundary for the one stays-flat builtin here (sleep). The removable
	// builtins' accept-boundary coverage lives in the core_* suites.
	for _, src := range []string{
		`sleep(0)`,
	} {
		expectOK(t, wrapMain(src))
	}
}

func TestArgDomain_Builtins_NonConstNotRejected(t *testing.T) {
	// R3/AC3: a non-constant argument is not rejected at compile time. Exercised via
	// the stays-flat sleep builtin (chr/repeat, previously used here, are removable).
	expectOK(t, wrapMain(`let n: int = -1
sleep(n)`))
}

// gcd is now removable (math.gcd); the gcd(INT_MIN,INT_MIN) single-diagnostic
// property moved to core_collections_neg_test.go
// (TestCoreMathNeg_GcdBothIntMinSingleDiagnostic).

func TestArgDomain_ArrayIndexNegative(t *testing.T) {
	d := expectErr(t, wrapMain(`let a: int[] = [1, 2, 3]
let x: int = a[-1]`), "array index out of bounds: negative constant index")
	if d.Pos.Line == 0 || d.Pos.Col == 0 {
		t.Errorf("diagnostic not located (line=%d col=%d)", d.Pos.Line, d.Pos.Col)
	}
	// Non-negative constant index, even one that exceeds the array length, is NOT
	// newly rejected (the dynamic upper bound is out of scope; AC8).
	expectOK(t, wrapMain(`let a: int[] = [1, 2, 3]
let x: int = a[0]`))
	expectOK(t, wrapMain(`let a: int[] = [1, 2, 3]
let x: int = a[99]`))
	// Non-constant index is not rejected.
	expectOK(t, wrapMain(`let a: int[] = [1, 2, 3]
let i: int = 0
let x: int = a[i]`))
}

// walkIntNodes collects every *ast.IntLit and *ast.UnaryExpr in the program, so
// the test can assert the probe left no fold record on any of them. It walks the
// node forms this program contains (let initializers, call args, binary/index
// expressions, array literals); it does not need to be exhaustive over the whole
// grammar, only over the constructs in the test source below.
func walkIntNodes(prog *ast.Program, visit func(ast.Expr)) {
	var ex func(e ast.Expr)
	ex = func(e ast.Expr) {
		switch n := e.(type) {
		case nil:
			return
		case *ast.IntLit:
			visit(n)
		case *ast.UnaryExpr:
			visit(n)
			ex(n.X)
		case *ast.BinaryExpr:
			ex(n.L)
			ex(n.R)
		case *ast.CallExpr:
			for _, a := range n.Args {
				ex(a)
			}
		case *ast.IndexExpr:
			ex(n.X)
			ex(n.Index)
		case *ast.ArrayLit:
			for _, el := range n.Elems {
				ex(el)
			}
		}
	}
	for _, fn := range prog.Funcs {
		for _, st := range fn.Body {
			if ls, ok := st.(*ast.LetStmt); ok {
				ex(ls.Value)
			}
			if es, ok := st.(*ast.ExprStmt); ok {
				ex(es.X)
			}
		}
	}
}

func TestConstIntProbe_NoInfoWrites(t *testing.T) {
	// AC6 PRIMARY (mechanical): the probe writes NOTHING to types.Info. This
	// program has NO const/final/default-arg/switch/enum context, so the const-fold
	// path (the ONLY writer of info.FoldedValues) never runs - FoldedValues MUST be
	// empty after the check. Every integer node here is an ordinary builtin
	// argument, operator divisor, or array index that the probe inspects. If the
	// probe wrote to FoldedValues (or recorded a Use for a literal), the assertions
	// below catch it mechanically.
	src := wrapMain(`let s: string = to_string(65)
let a: int[] = [1, 2, 3]
let y: int = a[1]
let q: int = 7 / 2`)
	prog, err := parser.Parse(src, "noinfo.wisp")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info := Check(prog)
	if len(info.Errors) != 0 {
		t.Fatalf("in-domain program produced errors: %v", info.Errors)
	}
	// MECHANICAL assertion 1: no const-fold ran, so FoldedValues is empty. A probe
	// that wrote a folded value would make this non-empty.
	if n := len(info.FoldedValues); n != 0 {
		t.Errorf("info.FoldedValues has %d entries, want 0 (the probe must not fold into Info)", n)
	}
	// MECHANICAL assertion 2: none of the integer argument/divisor/index nodes the
	// probe inspected appear as keys in FoldedValues. (info.Uses is keyed by
	// *ast.Ident, not ast.Expr, and literals are never Idents, so a Uses check on
	// these nodes neither type-checks nor means anything - the probe also never
	// resolves idents, so it cannot write Uses. FoldedValues is the live concern.)
	walkIntNodes(prog, func(e ast.Expr) {
		if _, ok := info.FoldedValues[e]; ok {
			t.Errorf("probe-inspected node %T at %v leaked into info.FoldedValues", e, e.Pos())
		}
	})
}
