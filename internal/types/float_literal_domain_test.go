package types

import (
	"strings"
	"testing"
)

// AC1: in-domain literals are accepted in expression position.
func TestFloatLiteralDomain_AC1_InDomain_Accepted(t *testing.T) {
	cases := []string{
		`let x: float = 0.0`,
		`let x: float = 0.5`,
		`let x: float = 0.0005`,
		`let x: float = 3.14`,
		`let x: float = 42.0`,
		`let x: float = 99999999999999.0`,
	}
	for _, body := range cases {
		expectOK(t, wrapMain(body))
	}
}

// AC2 expr: 0.000001 renders in exponent form and is rejected in expression position.
func TestFloatLiteralDomain_AC2_SmallExponent_ExprRejected(t *testing.T) {
	expectErr(t, wrapMain(`let x: float = 0.000001`), "float literal out of domain")
}

// AC2 const: 0.000001 is rejected in a const initializer.
func TestFloatLiteralDomain_AC2_SmallExponent_ConstRejected(t *testing.T) {
	expectErr(t, `const c: float = 0.000001
fn main() -> int { return 0 }`, "float literal out of domain")
}

// AC3 expr: 100000000000000000.0 renders in exponent form and is rejected in expression position.
func TestFloatLiteralDomain_AC3_LargeExponent_ExprRejected(t *testing.T) {
	expectErr(t, wrapMain(`let x: float = 100000000000000000.0`), "float literal out of domain")
}

// AC3 const: 100000000000000000.0 is rejected in a const initializer.
func TestFloatLiteralDomain_AC3_LargeExponent_ConstRejected(t *testing.T) {
	expectErr(t, `const c: float = 100000000000000000.0
fn main() -> int { return 0 }`, "float literal out of domain")
}

// AC4 expr: -0.000001 is rejected in expression position (the inner FloatLit is checked
// via the expr path when checkUnary recurses into checkExpr for the non-IntLit case).
func TestFloatLiteralDomain_AC4_NegativeSmall_ExprRejected(t *testing.T) {
	expectErr(t, wrapMain(`let x: float = -0.000001`), "float literal out of domain")
}

// AC4 const: -0.000001 is rejected in a const initializer (foldUnary recurses into
// foldConst, which hits the FloatLit case).
func TestFloatLiteralDomain_AC4_NegativeSmall_ConstRejected(t *testing.T) {
	expectErr(t, `const c: float = -0.000001
fn main() -> int { return 0 }`, "float literal out of domain")
}

// AC6 in-domain const: an in-domain float const must compile without error.
func TestFloatLiteralDomain_AC6_InDomainConst_Accepted(t *testing.T) {
	expectOK(t, `const c: float = 0.5
fn main() -> int { return 0 }`)
}

// AC6 out-of-domain const: covered by AC2 and AC3 const cases above.

// AC7: the error for 0.000001 must be located (Pos.Line != 0) and name the literal.
func TestFloatLiteralDomain_AC7_Diagnostic_Located(t *testing.T) {
	d := expectErr(t, wrapMain(`let x: float = 0.000001`), "float literal out of domain")
	if d.Pos.Line == 0 {
		t.Errorf("diagnostic missing position: %+v", d)
	}
	if !strings.Contains(d.Msg, "0.000001") {
		t.Errorf("diagnostic message does not name the literal: %q", d.Msg)
	}
}

// AC7: the error for -0.000001 must be located and name the inner literal.
func TestFloatLiteralDomain_AC7_Negative_Diagnostic_Located(t *testing.T) {
	d := expectErr(t, wrapMain(`let x: float = -0.000001`), "float literal out of domain")
	if d.Pos.Line == 0 {
		t.Errorf("diagnostic missing position: %+v", d)
	}
	if !strings.Contains(d.Msg, "0.000001") {
		t.Errorf("diagnostic message does not name the literal: %q", d.Msg)
	}
}
