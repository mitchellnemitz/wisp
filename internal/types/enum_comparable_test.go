package types

import (
	"strings"
	"testing"
)

// Task #19 (F6/CI-7): an enum type is admitted, on equal footing with
// int/bool/string, to the comparable bound and the equality-derived
// membership/assert builtins. Each of the 6 acceptance sites gets an ACCEPT
// test (an enum program compiles clean) and a negative-control REJECT test
// (a still-excluded type, e.g. float or struct, is still rejected with the
// reworded diagnostic) proving the enum arm did not widen the set further.

const enumColorDecl = "enum Color { Red, Green, Blue }\n"

// --- 1. comparable bound (call.go:960) ---

func TestEnumComparableBound_Accept(t *testing.T) {
	expectOK(t, enumColorDecl+`fn eq2[T: comparable](a: T, b: T) -> bool { return a == b }
`+wrapMain(`let b: bool = eq2(Color.Red, Color.Blue)`))
}

func TestEnumComparableBound_Reject_Float(t *testing.T) {
	d := expectErr(t, `fn eq2[T: comparable](a: T, b: T) -> bool { return a == b }
`+wrapMain(`let b: bool = eq2(1.0, 2.0)`), "does not satisfy comparable")
	if d.Pos.Line == 0 {
		t.Errorf("diagnostic missing position: %+v", d)
	}
	if got := d.Msg; !strings.Contains(got, "an enum type") {
		t.Errorf("diagnostic %q missing reworded type list", got)
	}
}

// --- 2. contains (stdlib.go:337) ---
//
// contains/index_of/unique are array-module members (checkNS links a
// synthetic "array" core module), so these three sites use checkNS instead of
// the bare check()/expectOK() used elsewhere in this file.

func TestEnumContains_Accept(t *testing.T) {
	info := checkNS(t, enumColorDecl+wrapMain(`let cs: Color[] = [Color.Red, Color.Green]
let b: bool = array.contains(cs, Color.Red)`), "array")
	if len(info.Errors) != 0 {
		t.Fatalf("expected no errors, got:\n%s", diagList(info.Errors))
	}
}

func TestEnumContains_Reject_Float(t *testing.T) {
	info := checkNS(t, wrapMain(`let xs: float[] = [1.0, 2.0]
let b: bool = array.contains(xs, 1.0)`), "array")
	d := findErrContains(t, info, "comparable element types int/bool/string/enum")
	if d.Pos.Line == 0 {
		t.Errorf("diagnostic missing position: %+v", d)
	}
}

// --- 3. index_of (collections.go:396) ---

func TestEnumIndexOf_Accept(t *testing.T) {
	info := checkNS(t, enumColorDecl+wrapMain(`let cs: Color[] = [Color.Red, Color.Green]
let i: Optional[int] = array.index_of(cs, Color.Red)`), "array")
	if len(info.Errors) != 0 {
		t.Fatalf("expected no errors, got:\n%s", diagList(info.Errors))
	}
}

func TestEnumIndexOf_Reject_Struct(t *testing.T) {
	info := checkNS(t, `struct P { x: int }
`+wrapMain(`let xs: P[] = []
let i: Optional[int] = array.index_of(xs, P{x: 1})`), "array")
	d := findErrContains(t, info, "comparable element types int/bool/string/enum")
	if d.Pos.Line == 0 {
		t.Errorf("diagnostic missing position: %+v", d)
	}
}

// --- 4. unique (collections.go:472) ---

func TestEnumUnique_Accept(t *testing.T) {
	info := checkNS(t, enumColorDecl+wrapMain(`let cs: Color[] = [Color.Red, Color.Red, Color.Green]
let u: Color[] = array.unique(cs)`), "array")
	if len(info.Errors) != 0 {
		t.Fatalf("expected no errors, got:\n%s", diagList(info.Errors))
	}
}

func TestEnumUnique_Reject_Float(t *testing.T) {
	info := checkNS(t, wrapMain(`let xs: float[] = [1.0, 2.0]
let u: float[] = array.unique(xs)`), "array")
	d := findErrContains(t, info, "comparable element types int/bool/string/enum")
	if d.Pos.Line == 0 {
		t.Errorf("diagnostic missing position: %+v", d)
	}
}

// findErrContains asserts info has an error whose message contains want, and
// returns the matching diagnostic (the checkNS analogue of expectErr, which
// re-runs check() on unlinked source and cannot be used here).
func findErrContains(t *testing.T, info *Info, want string) Diagnostic {
	t.Helper()
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, want) {
			return d
		}
	}
	t.Fatalf("expected an error containing %q, got:\n%s", want, diagList(info.Errors))
	return Diagnostic{}
}

// --- 5. assert_eq/assert_ne (stdlib.go:380) ---

func TestEnumAssertEq_Accept(t *testing.T) {
	expectOK(t, enumColorDecl+wrapMain(`assert_eq(Color.Red, Color.Red)`))
}

func TestEnumAssertNe_Accept(t *testing.T) {
	expectOK(t, enumColorDecl+wrapMain(`assert_ne(Color.Red, Color.Blue)`))
}

func TestEnumAssertEq_Reject_Struct(t *testing.T) {
	d := expectErr(t, `struct P { x: int }
`+wrapMain(`assert_eq(P{x: 1}, P{x: 1})`), "int, bool, string, an enum type, or a nested comparable Optional")
	if d.Pos.Line == 0 {
		t.Errorf("diagnostic missing position: %+v", d)
	}
}

// --- 6. assert_contains (stdlib.go:441) ---

func TestEnumAssertContains_Accept(t *testing.T) {
	expectOK(t, enumColorDecl+wrapMain(`let cs: Color[] = [Color.Red, Color.Green]
assert_contains(cs, Color.Red)`))
}

func TestEnumAssertContains_Reject_Float(t *testing.T) {
	d := expectErr(t, wrapMain(`let xs: float[] = [1.0, 2.0]
assert_contains(xs, 1.0)`), "comparable element types int/bool/string/enum")
	if d.Pos.Line == 0 {
		t.Errorf("diagnostic missing position: %+v", d)
	}
}
