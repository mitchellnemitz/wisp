package types

import "testing"

// --- float type and literals ---

func TestFloat_LetLiteral_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let p: float = 3.14`))
}

func TestFloat_LiteralIntoIntFails(t *testing.T) {
	expectErr(t, wrapMain(`let n: int = 3.14`), "want int")
}

func TestFloat_LiteralIntoStringFails(t *testing.T) {
	expectErr(t, wrapMain(`let s: string = 3.14`), "want string")
}

func TestFloat_IntLiteralIntoFloatFails(t *testing.T) {
	// no implicit int->float; a bare int does not satisfy a float annotation.
	expectErr(t, wrapMain(`let f: float = 2`), "want float")
}

func TestFloat_UnaryMinus_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let n: float = -2.0`))
}

// --- arithmetic ---

func TestFloat_Add_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let f: float = 1.5 + 2.5`))
}

func TestFloat_Sub_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let f: float = 5.0 - 2.5`))
}

func TestFloat_Mul_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let f: float = 3.0 * 2.0`))
}

func TestFloat_Div_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let f: float = 7.0 / 2.0`))
}

func TestFloat_ModIsError(t *testing.T) {
	expectErr(t, wrapMain(`let f: float = 5.0 % 2.0`), "modulo is undefined for float")
}

func TestFloat_MixAddIsError(t *testing.T) {
	expectErr(t, wrapMain(`let f: float = 1 + 2.0`), "float+float")
}

func TestFloat_MixSubIsError(t *testing.T) {
	expectErr(t, wrapMain(`let f: float = 2.0 - 1`), "int+int or float+float")
}

func TestFloat_MixMulIsError(t *testing.T) {
	expectErr(t, wrapMain(`let n: int = 2 * 2.0`), "int+int or float+float")
}

// --- comparisons ---

func TestFloat_Comparisons_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let a: bool = 1.0 < 2.0
let b: bool = 1.0 <= 2.0
let c: bool = 3.0 > 2.0
let d: bool = 3.0 >= 3.0
let e: bool = 1.0 == 1.0
let f: bool = 1.0 != 2.0`))
}

func TestFloat_CompareMixIsError(t *testing.T) {
	expectErr(t, wrapMain(`let b: bool = 1 < 2.0`), "int+int or float+float operands")
}

func TestFloat_EqMixIsError(t *testing.T) {
	expectErr(t, wrapMain(`let b: bool = 1 == 2.0`), "same type")
}

// --- conversions / builtin signatures ---

func TestFloat_FloatOfInt_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let n: int = 2
let f: float = to_float(n)`))
}

func TestFloat_FloatOfString_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let f: float = to_float("3.14")`))
}

func TestFloat_FloatOfBoolIsError(t *testing.T) {
	expectErr(t, wrapMain(`let f: float = to_float(true)`), "float")
}

func TestFloat_IntOfFloat_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let f: float = 3.9
let n: int = to_int(f)`))
}

func TestFloat_StringOfFloat_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let f: float = 3.14
let s: string = to_string(f)`))
}

func TestFloat_BoolOfFloat_Positive(t *testing.T) {
	expectOK(t, wrapMain(`let f: float = 0.0
let b: bool = to_bool(f)`))
}

func TestFloat_FloatResultType(t *testing.T) {
	// to_float() result must be usable where a float is required.
	expectOK(t, wrapMain(`let f: float = to_float(2) + 1.0`))
}

func TestFloat_ParamAndReturn_Positive(t *testing.T) {
	expectOK(t, `fn dbl(x: float) -> float {
  return x * 2.0
}
fn main() -> int {
  let f: float = dbl(1.5)
  return 0
}`)
}

func TestFloat_DefaultArg_Positive(t *testing.T) {
	expectOK(t, `fn scale(x: float, by: float = 2.0) -> float {
  return x * by
}
fn main() -> int {
  let f: float = scale(1.5)
  return 0
}`)
}

func TestFloat_DefaultArgNegativeFloat_Positive(t *testing.T) {
	expectOK(t, `fn shift(x: float, by: float = -1.0) -> float {
  return x + by
}
fn main() -> int {
  let f: float = shift(1.5)
  return 0
}`)
}

// --- format_float builtin ---
//
// format_float is removable (string.format_float); the bare spelling is gone under
// PR C. The type/arity checks migrated to the namespaced form, which CheckLinked
// resolves via the string module (helpers wantNsOK/wantNsErr live in
// core_collections_neg_test.go). The bare reserved-name tests were removed:
// format_float is freed for reuse.

func TestFormatFloat_Checks(t *testing.T) {
	wantNsOK(t, "string", `fn main() -> int { let s: string = string.format_float(3.14, 2); return 0 }`)
	wantNsOK(t, "string", `fn main() -> int { let s: string = string.format_float(1.5, 1); print("after"); return 0 }`) // not a terminator

	// Arity.
	wantNsErr(t, "string", `fn main() -> int { let s: string = string.format_float(1.0); return 0 }`, "expects")
	wantNsErr(t, "string", `fn main() -> int { let s: string = string.format_float(1.0,2,3); return 0 }`, "expects")
	// Non-float arg 1 (NO implicit int->float -- float_test.go:20 -- so a bare int is a type error).
	wantNsErr(t, "string", `fn main() -> int { let s: string = string.format_float("x", 2); return 0 }`, "has type")
	wantNsErr(t, "string", `fn main() -> int { let s: string = string.format_float(true, 2); return 0 }`, "has type")
	wantNsErr(t, "string", `fn main() -> int { let s: string = string.format_float(3, 2); return 0 }`, "has type") // int not float
	// Non-int arg 2.
	wantNsErr(t, "string", `fn main() -> int { let s: string = string.format_float(1.0, "x"); return 0 }`, "has type")
	wantNsErr(t, "string", `fn main() -> int { let s: string = string.format_float(1.0, 1.5); return 0 }`, "has type")
}

func TestFormatFloat_ReferenceAllowed(t *testing.T) {
	// format_float is a located monomorphic builtin: (float, int) -> string. Under
	// universal funcrefs its namespaced member is referenceable and records that
	// funcref type in MemberFuncRefs.
	info := wantNsOK(t, "string", `fn main()->int{ let f: fn(float, int) -> string = string.format_float; return 0 }`)
	if len(info.MemberFuncRefs) == 0 {
		t.Fatal("expected a MemberFuncRef recorded for string.format_float")
	}
}
