package parser

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
)

// callOf parses `<expr>` as the sole statement of main and returns the CallExpr.
func callOf(t *testing.T, expr string) *ast.CallExpr {
	t.Helper()
	prog := parseOK(t, wrap(expr))
	es, ok := mainBody(t, prog)[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("stmt is %T, want *ast.ExprStmt", mainBody(t, prog)[0])
	}
	call, ok := es.X.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expr is %T, want *ast.CallExpr", es.X)
	}
	return call
}

func typeArgNames(call *ast.CallExpr) []ast.TypeName {
	out := make([]ast.TypeName, len(call.TypeArgs))
	for i, ta := range call.TypeArgs {
		out[i] = ta.Name
	}
	return out
}

func eqNames(a []ast.TypeName, b ...ast.TypeName) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestParseCallTypeArgs_Bare(t *testing.T) {
	call := callOf(t, "f[int](x)")
	if call.CalleeName != "f" {
		t.Errorf("CalleeName = %q, want f", call.CalleeName)
	}
	if !eqNames(typeArgNames(call), ast.TypeInt) {
		t.Errorf("TypeArgs = %v, want [int]", typeArgNames(call))
	}
	if len(call.Args) != 1 {
		t.Errorf("Args len = %d, want 1", len(call.Args))
	}
}

func TestParseCallTypeArgs_Qualified(t *testing.T) {
	call := callOf(t, "ns.decode[int](x)")
	if call.CalleeName != "" {
		t.Errorf("CalleeName = %q, want empty for qualified call", call.CalleeName)
	}
	if _, ok := call.Callee.(*ast.FieldAccess); !ok {
		t.Errorf("Callee = %T, want *ast.FieldAccess", call.Callee)
	}
	if !eqNames(typeArgNames(call), ast.TypeInt) {
		t.Errorf("TypeArgs = %v, want [int]", typeArgNames(call))
	}
}

func TestParseCallTypeArgs_MultiAndTrailingComma(t *testing.T) {
	for _, src := range []string{"f[int, string](x)", "f[int, string,](x)"} {
		call := callOf(t, src)
		if !eqNames(typeArgNames(call), ast.TypeInt, ast.TypeString) {
			t.Errorf("%s: TypeArgs = %v, want [int string]", src, typeArgNames(call))
		}
	}
}

func TestParseCallTypeArgs_Composite(t *testing.T) {
	cases := []struct {
		src  string
		want ast.TypeName
	}{
		{"f[int[]](x)", ast.ArrayType(ast.TypeInt)},
		{"f[{string: int}](x)", ast.DictType(ast.TypeString, ast.TypeInt)},
		{"f[fn(int) -> bool](x)", ast.FuncType([]ast.TypeName{ast.TypeInt}, ast.TypeBool)},
		{"f[(int)](x)", ast.TypeInt},
		{"f[Box[int]](x)", ast.TypeName("Box[int]")},
	}
	for _, c := range cases {
		call := callOf(t, c.src)
		if !eqNames(typeArgNames(call), c.want) {
			t.Errorf("%s: TypeArgs = %v, want [%s]", c.src, typeArgNames(call), c.want)
		}
	}
}

// TestParseCallTypeArgs_IndexNotConfused: a bare `a[i]` with no following call is
// a value index, and index-then-call with a LITERAL index (the funcref_compose.wisp
// shape) must remain index-then-call, reproduced inside interpolation.
func TestParseCallTypeArgs_IndexNotConfused(t *testing.T) {
	// a[i] with no call -> IndexExpr.
	prog := parseOK(t, wrap("let y: int = a[i]"))
	rhs := prog.Funcs[0].Body[0].(*ast.LetStmt).Value
	if _, ok := rhs.(*ast.IndexExpr); !ok {
		t.Errorf("a[i] parsed as %T, want *ast.IndexExpr", rhs)
	}

	// Index-then-call with a literal index (interpolated, matching the fixture).
	for _, src := range []string{`print("${fns[1](6, 7)}")`, `print("${m["x"](2, 8)}")`} {
		call := callOf(t, src) // outer print(...)
		inner := call.Args[0]  // the interpolated string
		is, ok := inner.(*ast.StringLit)
		if !ok {
			t.Fatalf("%s: arg is %T, want *ast.StringLit", src, inner)
		}
		var found *ast.CallExpr
		for _, part := range is.Parts {
			if part.Expr != nil {
				if ce, ok := part.Expr.(*ast.CallExpr); ok {
					found = ce
				}
			}
		}
		if found == nil {
			t.Fatalf("%s: no CallExpr in interpolation", src)
		}
		if len(found.TypeArgs) != 0 {
			t.Errorf("%s: inner call has TypeArgs %v, want none (index-then-call)", src, typeArgNames(found))
		}
		if _, ok := found.Callee.(*ast.IndexExpr); !ok {
			t.Errorf("%s: inner call callee = %T, want *ast.IndexExpr", src, found.Callee)
		}
	}
}

// TestParseCallTypeArgs_BareIndexNoCall: `f[int]` with no following `(` is a value
// index, not a bare type application.
func TestParseCallTypeArgs_BareIndexNoCall(t *testing.T) {
	prog := parseOK(t, wrap("let y: int = f[i]"))
	rhs := prog.Funcs[0].Body[0].(*ast.LetStmt).Value
	if _, ok := rhs.(*ast.IndexExpr); !ok {
		t.Errorf("f[i] parsed as %T, want *ast.IndexExpr", rhs)
	}
}

// TestParseCallTypeArgs_ResidualAmbiguity: `fns[f](x)` reads as a type-arg call;
// the documented workaround `(fns[f])(x)` reads as index-then-call.
func TestParseCallTypeArgs_ResidualAmbiguity(t *testing.T) {
	call := callOf(t, "fns[f](x)")
	if len(call.TypeArgs) != 1 {
		t.Errorf("fns[f](x): TypeArgs = %v, want 1 (type-arg call)", typeArgNames(call))
	}

	grouped := callOf(t, "(fns[f])(x)")
	if len(grouped.TypeArgs) != 0 {
		t.Errorf("(fns[f])(x): TypeArgs = %v, want none", typeArgNames(grouped))
	}
	if _, ok := grouped.Callee.(*ast.IndexExpr); !ok {
		t.Errorf("(fns[f])(x): callee = %T, want *ast.IndexExpr", grouped.Callee)
	}
}

// TestParseCallTypeArgs_Chaining: postfix chains after a type-arg call.
func TestParseCallTypeArgs_Chaining(t *testing.T) {
	// f[T](x).field -> FieldAccess whose X is the type-arg call.
	prog := parseOK(t, wrap("let y: int = f[int](x).field"))
	fa, ok := prog.Funcs[0].Body[0].(*ast.LetStmt).Value.(*ast.FieldAccess)
	if !ok {
		t.Fatalf("f[int](x).field = %T, want *ast.FieldAccess", prog.Funcs[0].Body[0].(*ast.LetStmt).Value)
	}
	inner, ok := fa.X.(*ast.CallExpr)
	if !ok || len(inner.TypeArgs) != 1 {
		t.Errorf("f[int](x).field: inner = %T with TypeArgs %v", fa.X, func() []ast.TypeName {
			if inner != nil {
				return typeArgNames(inner)
			}
			return nil
		}())
	}
}

// TestParseCallTypeArgs_EmptyBrackets: `f[]()` is not a type-arg call; the empty
// bracket falls to the existing index parse, which errors.
func TestParseCallTypeArgs_EmptyBrackets(t *testing.T) {
	parseErr(t, wrap("let y: int = f[]()"))
}

// TestParseCallTypeArgs_AssignmentTarget: `f[int](x) = y` parses the LHS as a
// CallExpr, so the existing invalid-assignment-target error fires (no mis-parse).
func TestParseCallTypeArgs_AssignmentTarget(t *testing.T) {
	parseErr(t, wrap("f[int](x) = y"))
}

// TestParseCallTypeArgs_ConversionKeywordRejected: a type-keyword conversion
// builtin (int/string/bool/float/error) is only a valid callee when directly
// followed by `(`, so `int[int](x)` is a parse error (the type keyword before `[`
// is not a value). Type args on a conversion are thus rejected at parse time.
func TestParseCallTypeArgs_ConversionKeywordRejected(t *testing.T) {
	for _, src := range []string{`int[int]("5")`, `string[int](5)`} {
		parseErr(t, wrap("let z: int = "+src))
	}
}
