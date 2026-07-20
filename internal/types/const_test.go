package types

// Task-2: const-expression evaluator -- operators, folding, error classes.
//
// These tests verify that checkConstExpr (via checkConstExprWithValue) folds
// operator expressions at compile time and reports the right errors.  They do
// NOT test const-reference resolution (Task 3) or the const declaration
// statement (Task 4).

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/token"
)

// evalConst builds a minimal checker, calls checkConstExpr on e, and returns
// the resolved type and the folded value from info.FoldedValues.
func evalConst(e ast.Expr) (Type, interface{}) {
	c := &checker{info: newInfo()}
	t := c.checkConstExpr(e)
	return t, c.info.FoldedValues[e]
}

// mkChecker creates a minimal checker suitable for const-expr evaluation.
func mkChecker() *checker { return &checker{info: newInfo()} }

// intLit creates an IntLit from a decimal string.
func intLit(raw string) *ast.IntLit { return &ast.IntLit{Raw: raw} }

// boolLit creates a BoolLit.
func boolLit(v bool) *ast.BoolLit { return &ast.BoolLit{Value: v} }

// strLit creates an interpolation-free StringLit.
func strLit(s string) *ast.StringLit {
	return &ast.StringLit{Parts: []ast.StringPart{{Text: s}}}
}

// unary builds a UnaryExpr.
func unary(op token.Kind, x ast.Expr) *ast.UnaryExpr {
	return &ast.UnaryExpr{Op: op, X: x}
}

// binary builds a BinaryExpr.
func binary(op token.Kind, l, r ast.Expr) *ast.BinaryExpr {
	return &ast.BinaryExpr{Op: op, L: l, R: r}
}

// constHasErr checks that c has at least one error whose message contains sub.
func constHasErr(t *testing.T, c *checker, sub string) {
	t.Helper()
	for _, d := range c.info.Errors {
		if strings.Contains(d.Msg, sub) {
			return
		}
	}
	var msgs []string
	for _, d := range c.info.Errors {
		msgs = append(msgs, d.Msg)
	}
	t.Fatalf("expected error containing %q, got: %v", sub, msgs)
}

// --- Existing literal cases (must still pass) ---

func TestConstExpr_IntLit(t *testing.T) {
	ty, v := evalConst(intLit("42"))
	if ty != Int {
		t.Fatalf("type = %s, want int", ty)
	}
	if v.(int64) != 42 {
		t.Fatalf("value = %v, want 42", v)
	}
}

func TestConstExpr_BoolLit(t *testing.T) {
	ty, v := evalConst(boolLit(true))
	if ty != Bool {
		t.Fatalf("type = %s, want bool", ty)
	}
	if v.(bool) != true {
		t.Fatalf("value = %v, want true", v)
	}
}

func TestConstExpr_StringLit(t *testing.T) {
	ty, v := evalConst(strLit("hello"))
	if ty != String {
		t.Fatalf("type = %s, want string", ty)
	}
	if v.(string) != "hello" {
		t.Fatalf("value = %v, want hello", v)
	}
}

func TestConstExpr_FloatLit(t *testing.T) {
	ty, _ := evalConst(&ast.FloatLit{Raw: "3.14"})
	if ty != Float {
		t.Fatalf("type = %s, want float", ty)
	}
}

func TestConstExpr_UnaryMinusIntLit(t *testing.T) {
	ty, v := evalConst(unary(token.Minus, intLit("5")))
	if ty != Int {
		t.Fatalf("type = %s, want int", ty)
	}
	if v.(int64) != -5 {
		t.Fatalf("value = %v, want -5", v)
	}
}

func TestConstExpr_StdoutFoldsToInt(t *testing.T) {
	ty, v := evalConst(&ast.Ident{Name: "stdout"})
	if ty != Int {
		t.Fatalf("type = %s, want int", ty)
	}
	if v.(int64) != 1 {
		t.Fatalf("value = %v, want 1", v)
	}
}

func TestConstExpr_StderrFoldsToInt(t *testing.T) {
	ty, v := evalConst(&ast.Ident{Name: "stderr"})
	if ty != Int {
		t.Fatalf("type = %s, want int", ty)
	}
	if v.(int64) != 2 {
		t.Fatalf("value = %v, want 2", v)
	}
}

// --- Int arithmetic folding ---

func TestConstExpr_IntMul(t *testing.T) {
	// 60 * 60 = 3600
	e := binary(token.Star, intLit("60"), intLit("60"))
	ty, v := evalConst(e)
	if ty != Int {
		t.Fatalf("type = %s, want int", ty)
	}
	if v.(int64) != 3600 {
		t.Fatalf("value = %v, want 3600", v)
	}
}

func TestConstExpr_IntAdd(t *testing.T) {
	e := binary(token.Plus, intLit("10"), intLit("3"))
	ty, v := evalConst(e)
	if ty != Int {
		t.Fatalf("type = %s, want int", ty)
	}
	if v.(int64) != 13 {
		t.Fatalf("value = %v, want 13", v)
	}
}

func TestConstExpr_IntSub(t *testing.T) {
	e := binary(token.Minus, intLit("10"), intLit("3"))
	ty, v := evalConst(e)
	if ty != Int {
		t.Fatalf("type = %s, want int", ty)
	}
	if v.(int64) != 7 {
		t.Fatalf("value = %v, want 7", v)
	}
}

func TestConstExpr_IntDiv(t *testing.T) {
	e := binary(token.Slash, intLit("10"), intLit("3"))
	ty, v := evalConst(e)
	if ty != Int {
		t.Fatalf("type = %s, want int", ty)
	}
	if v.(int64) != 3 { // truncating
		t.Fatalf("value = %v, want 3", v)
	}
}

func TestConstExpr_IntMod(t *testing.T) {
	e := binary(token.Percent, intLit("10"), intLit("3"))
	ty, v := evalConst(e)
	if ty != Int {
		t.Fatalf("type = %s, want int", ty)
	}
	if v.(int64) != 1 {
		t.Fatalf("value = %v, want 1", v)
	}
}

func TestConstExpr_UnaryMinusOverExpr(t *testing.T) {
	// unary minus over a BinaryExpr result: -(3 + 4) = -7
	inner := binary(token.Plus, intLit("3"), intLit("4"))
	e := unary(token.Minus, inner)
	ty, v := evalConst(e)
	if ty != Int {
		t.Fatalf("type = %s, want int", ty)
	}
	if v.(int64) != -7 {
		t.Fatalf("value = %v, want -7", v)
	}
}

// --- String concat ---

func TestConstExpr_StringConcat(t *testing.T) {
	e := binary(token.Plus, strLit("foo"), strLit("bar"))
	ty, v := evalConst(e)
	if ty != String {
		t.Fatalf("type = %s, want string", ty)
	}
	if v.(string) != "foobar" {
		t.Fatalf("value = %v, want foobar", v)
	}
}

// --- Bool operators ---

func TestConstExpr_BoolAnd(t *testing.T) {
	e := binary(token.AndAnd, boolLit(true), boolLit(false))
	ty, v := evalConst(e)
	if ty != Bool {
		t.Fatalf("type = %s, want bool", ty)
	}
	if v.(bool) != false {
		t.Fatalf("value = %v, want false", v)
	}
}

func TestConstExpr_BoolOr(t *testing.T) {
	e := binary(token.OrOr, boolLit(false), boolLit(true))
	ty, v := evalConst(e)
	if ty != Bool {
		t.Fatalf("type = %s, want bool", ty)
	}
	if v.(bool) != true {
		t.Fatalf("value = %v, want true", v)
	}
}

func TestConstExpr_BoolNot(t *testing.T) {
	e := unary(token.Bang, boolLit(true))
	ty, v := evalConst(e)
	if ty != Bool {
		t.Fatalf("type = %s, want bool", ty)
	}
	if v.(bool) != false {
		t.Fatalf("value = %v, want false", v)
	}
}

// --- Comparison operators -> bool ---

func TestConstExpr_IntLt(t *testing.T) {
	e := binary(token.Lt, intLit("3"), intLit("5"))
	ty, v := evalConst(e)
	if ty != Bool {
		t.Fatalf("type = %s, want bool", ty)
	}
	if v.(bool) != true {
		t.Fatalf("value = %v, want true", v)
	}
}

func TestConstExpr_IntGt(t *testing.T) {
	e := binary(token.Gt, intLit("5"), intLit("3"))
	ty, v := evalConst(e)
	if ty != Bool {
		t.Fatalf("type = %s, want bool", ty)
	}
	if v.(bool) != true {
		t.Fatalf("value = %v, want true", v)
	}
}

func TestConstExpr_IntEq(t *testing.T) {
	e := binary(token.Eq, intLit("7"), intLit("7"))
	ty, v := evalConst(e)
	if ty != Bool {
		t.Fatalf("type = %s, want bool", ty)
	}
	if v.(bool) != true {
		t.Fatalf("value = %v, want true", v)
	}
}

func TestConstExpr_IntNeq(t *testing.T) {
	e := binary(token.Neq, intLit("7"), intLit("8"))
	ty, v := evalConst(e)
	if ty != Bool {
		t.Fatalf("type = %s, want bool", ty)
	}
	if v.(bool) != true {
		t.Fatalf("value = %v, want true", v)
	}
}

func TestConstExpr_IntLte(t *testing.T) {
	e := binary(token.Lte, intLit("5"), intLit("5"))
	ty, v := evalConst(e)
	if ty != Bool {
		t.Fatalf("type = %s, want bool", ty)
	}
	if v.(bool) != true {
		t.Fatalf("value = %v, want true", v)
	}
}

func TestConstExpr_IntGte(t *testing.T) {
	e := binary(token.Gte, intLit("5"), intLit("3"))
	ty, v := evalConst(e)
	if ty != Bool {
		t.Fatalf("type = %s, want bool", ty)
	}
	if v.(bool) != true {
		t.Fatalf("value = %v, want true", v)
	}
}

func TestConstExpr_StringEq(t *testing.T) {
	e := binary(token.Eq, strLit("a"), strLit("a"))
	ty, v := evalConst(e)
	if ty != Bool {
		t.Fatalf("type = %s, want bool", ty)
	}
	if v.(bool) != true {
		t.Fatalf("value = %v, want true", v)
	}
}

func TestConstExpr_BoolEq(t *testing.T) {
	e := binary(token.Eq, boolLit(true), boolLit(false))
	ty, v := evalConst(e)
	if ty != Bool {
		t.Fatalf("type = %s, want bool", ty)
	}
	if v.(bool) != false {
		t.Fatalf("value = %v, want false", v)
	}
}

// --- Error cases ---

func TestConstExpr_DivideByZero(t *testing.T) {
	c := mkChecker()
	e := binary(token.Slash, intLit("10"), intLit("0"))
	ty := c.checkConstExpr(e)
	if ty != Invalid {
		t.Fatalf("type = %s, want invalid", ty)
	}
	constHasErr(t, c, "zero")
}

func TestConstExpr_ModuloByZero(t *testing.T) {
	c := mkChecker()
	e := binary(token.Percent, intLit("10"), intLit("0"))
	ty := c.checkConstExpr(e)
	if ty != Invalid {
		t.Fatalf("type = %s, want invalid", ty)
	}
	constHasErr(t, c, "zero")
}

func TestConstExpr_IntOverflow(t *testing.T) {
	// max int64 + 1 overflows: 9223372036854775807 + 1
	c := mkChecker()
	e := binary(token.Plus, intLit("9223372036854775807"), intLit("1"))
	ty := c.checkConstExpr(e)
	if ty != Invalid {
		t.Fatalf("type = %s, want invalid", ty)
	}
	if len(c.info.Errors) == 0 {
		t.Fatal("expected an overflow/out-of-range error")
	}
}

func TestConstExpr_MinInt64ViaUnaryMinus(t *testing.T) {
	// -9223372036854775808 is the most-negative int64. It is parsed as unary
	// minus over IntLit raw "9223372036854775808" (a magnitude that overflows on
	// its own), so the negation must be applied to the literal's magnitude.
	c := mkChecker()
	e := unary(token.Minus, intLit("9223372036854775808"))
	ty := c.checkConstExpr(e)
	if ty != Int {
		t.Fatalf("type = %s, want int (errors: %v)", ty, c.info.Errors)
	}
	if len(c.info.Errors) != 0 {
		t.Fatalf("unexpected errors folding min int64: %v", c.info.Errors)
	}
	if got := c.info.FoldedValues[e]; got != wispIntMin {
		t.Fatalf("folded value = %v, want %d", got, wispIntMin)
	}
}

func TestConstExpr_PositiveMinMagnitudeRejected(t *testing.T) {
	// Without the unary minus, 9223372036854775808 is out of range and rejected.
	c := mkChecker()
	ty := c.checkConstExpr(intLit("9223372036854775808"))
	if ty != Invalid {
		t.Fatalf("type = %s, want invalid", ty)
	}
	constHasErr(t, c, "out of range")
}

func TestConstExpr_BelowMinInt64Rejected(t *testing.T) {
	// -9223372036854775809 is one below the min and must still be rejected.
	c := mkChecker()
	e := unary(token.Minus, intLit("9223372036854775809"))
	ty := c.checkConstExpr(e)
	if ty != Invalid {
		t.Fatalf("type = %s, want invalid", ty)
	}
	constHasErr(t, c, "out of range")
}

func TestConstExpr_FloatArithRejected(t *testing.T) {
	// float + float must be rejected
	c := mkChecker()
	e := binary(token.Plus, &ast.FloatLit{Raw: "1.5"}, &ast.FloatLit{Raw: "2.5"})
	ty := c.checkConstExpr(e)
	if ty != Invalid {
		t.Fatalf("type = %s, want invalid", ty)
	}
	constHasErr(t, c, "float")
}

func TestConstExpr_FloatLitAccepted(t *testing.T) {
	// A float literal by itself is still valid (type Float, no error).
	c := mkChecker()
	ty := c.checkConstExpr(&ast.FloatLit{Raw: "3.14"})
	if ty != Float {
		t.Fatalf("type = %s, want float", ty)
	}
	if len(c.info.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", c.info.Errors)
	}
}

func TestConstExpr_UnaryMinusFloatLitAccepted(t *testing.T) {
	// -3.14 (unary minus over FloatLit) remains valid.
	ty, _ := evalConst(unary(token.Minus, &ast.FloatLit{Raw: "3.14"}))
	if ty != Float {
		t.Fatalf("type = %s, want float", ty)
	}
}

func TestConstExpr_NonConstIdent(t *testing.T) {
	// An unknown ident that is not stdout/stderr should error -- the const-ref
	// hook (Task 3) is nil, so it stays "not a constant expression".
	c := mkChecker()
	ty := c.checkConstExpr(&ast.Ident{Name: "foo"})
	if ty != Invalid {
		t.Fatalf("type = %s, want invalid", ty)
	}
	if len(c.info.Errors) == 0 {
		t.Fatal("expected an error for non-const ident")
	}
}
