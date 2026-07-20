package types

import "testing"

// Task 2: type-checker coverage for the assertion + skip builtins. Positive
// cases assert the call type-checks (result void); negative cases assert the
// located compile error.

// --- assert(cond: bool, msg: string = "") ---

func TestAssert_OK(t *testing.T) {
	expectOK(t, wrapMain(`assert(true)
assert(1 == 1, "one is one")`))
}

func TestAssert_NonBoolCond_Error(t *testing.T) {
	expectErr(t, wrapMain(`assert(1)`), "assert")
}

func TestAssert_NonStringMsg_Error(t *testing.T) {
	expectErr(t, wrapMain(`assert(true, 1)`), "assert")
}

func TestAssert_TooManyArgs_Error(t *testing.T) {
	expectErr(t, wrapMain(`assert(true, "a", "b")`), "assert")
}

// --- assert_eq / assert_ne [T: comparable] ---

func TestAssertEq_OK(t *testing.T) {
	expectOK(t, wrapMain(`assert_eq(1, 1)
assert_eq("a", "a")
assert_eq(true, false)
assert_ne(1, 2)`))
}

func TestAssertEq_ComparableOptional_OK(t *testing.T) {
	expectOK(t, wrapMain(`let a: Optional[int] = Some(1)
let b: Optional[int] = Some(1)
assert_eq(a, b)`))
}

func TestAssertEq_NonComparable_Error(t *testing.T) {
	// float is deliberately not comparable.
	expectErr(t, wrapMain(`assert_eq(1.0, 1.0)`), "comparable")
}

func TestAssertEq_MismatchedTypes_Error(t *testing.T) {
	expectErr(t, wrapMain(`assert_eq(1, "a")`), "assert_eq")
}

func TestAssertNe_NonComparable_Error(t *testing.T) {
	expectErr(t, wrapMain(`assert_ne(1.0, 2.0)`), "comparable")
}

// --- assert_some / assert_none over Optional[T] ---

func TestAssertSome_OK(t *testing.T) {
	expectOK(t, wrapMain(`let o: Optional[int] = Some(1)
assert_some(o)
assert_none(o)`))
}

func TestAssertSome_NonOptional_Error(t *testing.T) {
	expectErr(t, wrapMain(`assert_some(1)`), "assert_some")
}

func TestAssertNone_NonOptional_Error(t *testing.T) {
	expectErr(t, wrapMain(`assert_none(1)`), "assert_none")
}

// --- assert_ok / assert_err over Result[T] ---

func TestAssertOk_OK(t *testing.T) {
	expectOK(t, wrapMain(`let r: Result[int] = Ok(1)
assert_ok(r)
assert_err(r)`))
}

func TestAssertOk_NonResult_Error(t *testing.T) {
	expectErr(t, wrapMain(`assert_ok(1)`), "assert_ok")
}

func TestAssertErr_NonResult_Error(t *testing.T) {
	expectErr(t, wrapMain(`assert_err(1)`), "assert_err")
}

// --- assert_contains: overloaded on arg-0 type like contains ---

func TestAssertContains_String_OK(t *testing.T) {
	expectOK(t, wrapMain(`assert_contains("hello", "ell")`))
}

func TestAssertContains_Array_OK(t *testing.T) {
	expectOK(t, wrapMain(`let xs: int[] = [1, 2, 3]
assert_contains(xs, 2)`))
}

func TestAssertContains_StringNonStringNeedle_Error(t *testing.T) {
	expectErr(t, wrapMain(`assert_contains("hello", 1)`), "assert_contains")
}

func TestAssertContains_ArrayFloat_Error(t *testing.T) {
	expectErr(t, wrapMain(`let xs: float[] = [1.0, 2.0]
assert_contains(xs, 1.0)`), "comparable element types int/bool/string")
}

func TestAssertContains_ArrayNeedleMismatch_Error(t *testing.T) {
	expectErr(t, wrapMain(`let xs: int[] = [1, 2]
assert_contains(xs, "x")`), "the array element type")
}

func TestAssertContains_BadArg0_Error(t *testing.T) {
	expectErr(t, wrapMain(`assert_contains(1, 2)`), "assert_contains")
}

// --- skip(reason: string) -> void ---

func TestSkip_OK(t *testing.T) {
	expectOK(t, wrapMain(`skip("not ready")`))
}

func TestSkip_NonString_Error(t *testing.T) {
	expectErr(t, wrapMain(`skip(1)`), "skip")
}

func TestSkip_NoArg_Error(t *testing.T) {
	expectErr(t, wrapMain(`skip()`), "skip")
}

// --- the assert builtins return void (cannot be assigned to a value) ---

func TestAssert_ReturnsVoid_Error(t *testing.T) {
	expectErr(t, wrapMain(`let x: bool = assert(true)`), "void")
}
