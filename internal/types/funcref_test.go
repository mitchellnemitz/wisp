package types

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/parser"
)

// --- M4: function references + higher-order builtins type checking ---

func TestFuncref_BindPassReturnStore_OK(t *testing.T) {
	expectOK(t, `struct Op { f: fn(int, int) -> int }
fn add(a: int, b: int) -> int { return a + b }
fn apply(g: fn(int, int) -> int, a: int, b: int) -> int { return g(a, b) }
fn getOp() -> fn(int, int) -> int { return add }
fn main() -> int {
  let f: fn(int, int) -> int = add
  let r: int = f(1, 2)
  let o: Op = Op { f: add }
  let fns: (fn(int, int) -> int)[] = [add]
  let m: {string: fn(int, int) -> int} = { "a": add }
  let _z: int = apply(add, 1, 2) + o.f(1, 1) + fns[0](1, 1) + m["a"](1, 1) + getOp()(1, 1) + r
  return 0
}`)
}

func TestFuncref_ExactTypeMatch_OK(t *testing.T) {
	expectOK(t, `fn add(a: int, b: int) -> int { return a + b }
fn main() -> int {
  let f: fn(int, int) -> int = add
  return f(1, 2)
}`)
}

func TestFuncref_TypeMismatchAssign_Negative(t *testing.T) {
	expectErr(t, `fn add(a: int) -> int { return a }
fn main() -> int {
  let f: fn(string) -> int = add
  return 0
}`, "want fn(string)->int")
}

func TestFuncref_FullArityRule_OK(t *testing.T) {
	// A defaulted function's reference type uses its FULL declared arity.
	expectOK(t, `fn f(a: int, b: int = 0) -> int { return a + b }
fn main() -> int {
  let g: fn(int, int) -> int = f
  return g(1, 2)
}`)
}

func TestFuncref_DefaultedShortType_Negative(t *testing.T) {
	expectErr(t, `fn f(a: int, b: int = 0) -> int { return a + b }
fn main() -> int {
  let g: fn(int) -> int = f
  return 0
}`, "want fn(int)->int")
}

func TestFuncref_IndirectCallShortArity_Negative(t *testing.T) {
	expectErr(t, `fn f(a: int, b: int = 0) -> int { return a + b }
fn main() -> int {
  let g: fn(int, int) -> int = f
  return g(1)
}`, "expects 2 arguments, got 1")
}

func TestFuncref_IndirectCallSingularArity_Negative(t *testing.T) {
	// A one-parameter funcref uses the singular "1 argument" wording.
	expectErr(t, `fn f(a: int) -> int { return a }
fn main() -> int {
  let g: fn(int) -> int = f
  return g()
}`, "function reference expects 1 argument, got 0")
}

func TestFuncref_IndirectCallArgTypeMismatch_Negative(t *testing.T) {
	expectErr(t, `fn f(a: int) -> int { return a }
fn main() -> int {
  let g: fn(int) -> int = f
  return g("x")
}`, "want int")
}

// --- opacity (one negative per form) ---

func TestFuncref_Conversion_Negative(t *testing.T) {
	expectErr(t, `fn add(a: int) -> int { return a }
fn main() -> int {
  let s: string = to_string(add)
  return 0
}`, "want int|float|bool|string")
}

func TestFuncref_StringToFuncrefAssign_Negative(t *testing.T) {
	expectErr(t, `fn main() -> int {
  let f: fn(int) -> int = "x"
  return 0
}`, "want fn(int)->int")
}

func TestFuncref_CompareEq_Negative(t *testing.T) {
	expectErr(t, `fn add(a: int) -> int { return a }
fn main() -> int {
  let b: bool = (add == add)
  return 0
}`, "function references")
}

func TestFuncref_CompareNeq_Negative(t *testing.T) {
	expectErr(t, `fn add(a: int) -> int { return a }
fn main() -> int {
  let b: bool = (add != add)
  return 0
}`, "function references")
}

func TestFuncref_CompareLt_Negative(t *testing.T) {
	expectErr(t, `fn add(a: int) -> int { return a }
fn main() -> int {
  let b: bool = (add < add)
  return 0
}`, "requires int+int or float+float")
}

func TestFuncref_Arithmetic_Negative(t *testing.T) {
	expectErr(t, `fn add(a: int) -> int { return a }
fn main() -> int {
  let f: fn(int) -> int = add
  let g: int = f + 1
  return 0
}`, "requires int+int")
}

func TestFuncref_Interpolation_Negative(t *testing.T) {
	expectErr(t, `fn add(a: int) -> int { return a }
fn main() -> int {
  let f: fn(int) -> int = add
  print("${f}")
  return 0
}`, "cannot interpolate a function reference")
}

func TestFuncref_BuiltinAsValue_Negative(t *testing.T) {
	expectErr(t, `fn main() -> int {
  let f: fn(string) -> int = length
  return 0
}`, "overloaded")
}

// --- no-shadow function name ---

func TestFuncref_LetShadowsFunction_Negative(t *testing.T) {
	expectErr(t, `fn add(a: int) -> int { return a }
fn main() -> int {
  let add: int = 3
  return 0
}`, "declared function and cannot be shadowed")
}

func TestFuncref_ParamShadowsFunction_Negative(t *testing.T) {
	expectErr(t, `fn add(a: int) -> int { return a }
fn g(add: int) -> int { return add }
fn main() -> int { return 0 }`, "declared function and cannot be shadowed")
}

// --- non-callable callee ---

func TestFuncref_CallNonFunctionValue_Negative(t *testing.T) {
	expectErr(t, `fn main() -> int {
  let x: int = 3
  return x(1)
}`, "not callable")
}

// --- map / filter / each typing ---
//
// map/filter/each are now removable (array.map/array.filter/array.each); the bare
// spellings are gone under PR C. These tests migrated to the namespaced form,
// which CheckLinked resolves via the array module (helpers wantNsOK/wantNsErr live
// in core_collections_neg_test.go). The bare reserved-name redefinition tests
// (TestHigherOrder_RedefineMap/Filter/Each) were removed: those names are freed
// for reuse under PR C, so redefining them is no longer an error.

func TestHigherOrder_MapFilterEach_OK(t *testing.T) {
	wantNsOK(t, "array", `fn dbl(x: int) -> int { return x * 2 }
fn even(x: int) -> bool { return x % 2 == 0 }
fn show(x: int) -> void { print(to_string(x)) }
fn main() -> int {
  let xs: int[] = [1, 2, 3]
  let ys: int[] = array.map(xs, dbl)
  let zs: int[] = array.filter(xs, even)
  array.each(xs, show)
  return length(ys) + length(zs)
}`)
}

func TestHigherOrder_MapElemMismatch_Negative(t *testing.T) {
	wantNsErr(t, "array", `fn f(s: string) -> int { return 1 }
fn main() -> int {
  let xs: int[] = [1]
  let ys: int[] = array.map(xs, f)
  return 0
}`, "the array element type is int")
}

func TestHigherOrder_FilterNonBool_Negative(t *testing.T) {
	wantNsErr(t, "array", `fn id(x: int) -> int { return x }
fn main() -> int {
  let xs: int[] = [1]
  let ys: int[] = array.filter(xs, id)
  return 0
}`, "filter must return bool")
}

func TestHigherOrder_EachNonVoid_Negative(t *testing.T) {
	wantNsErr(t, "array", `fn id(x: int) -> int { return x }
fn main() -> int {
  let xs: int[] = [1]
  array.each(xs, id)
  return 0
}`, "each must return void")
}

func TestHigherOrder_MapResultType(t *testing.T) {
	// map over int[] with fn(int)->string yields string[].
	wantNsOK(t, "array", `fn s(x: int) -> string { return to_string(x) }
fn main() -> int {
  let xs: int[] = [1]
  let ys: string[] = array.map(xs, s)
  return length(ys)
}`)
}

// map/filter/each ARE referenceable given a matching container-axis annotation
// (see TestGenericFuncref_MapFilterAxes / TestGenericFuncref_ArrayOnlyBuiltins);
// these pin that a MISMATCHED annotation (wrong result shape) is still rejected
// as ambiguous, not silently coerced.
func TestHigherOrder_MapInValuePosition_Negative(t *testing.T) {
	wantNsErr(t, "array", `fn main() -> int {
  let f: fn(int[], fn(int) -> int) -> int = array.map
  return 0
}`, `"map" has no function-reference form matching fn([int],fn(int)->int)->int; supported containers: array, optional, result`)
}

func TestHigherOrder_FilterInValuePosition_Negative(t *testing.T) {
	wantNsErr(t, "array", `fn main() -> int {
  let f: fn(int[], fn(int) -> bool) -> int = array.filter
  return 0
}`, `"filter" has no function-reference form matching fn([int],fn(int)->bool)->int; supported containers: array, optional`)
}

func TestHigherOrder_EachInValuePosition_Negative(t *testing.T) {
	wantNsErr(t, "array", `fn main() -> int {
  let f: fn(int[], fn(int) -> void) -> bool = array.each
  return 0
}`, `"each" has no function-reference form matching fn([int],fn(int)->void)->bool; supported containers: array`)
}

// --- fn type with void parameter rejected ---

func TestFuncType_VoidParam_Negative(t *testing.T) {
	// A void parameter type is rejected at parse time (void is only valid as a
	// return type); the program does not compile.
	_, err := parser.Parse(`fn main() -> int {
  let f: fn(void) -> int = main
  return 0
}`, "test.wisp")
	if err == nil {
		t.Fatal("expected a parse error for a void parameter type")
	}
}
