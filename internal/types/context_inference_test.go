package types

import "testing"

// A return-only type parameter is inferred from the let-binding annotation, with
// no explicit type argument on the call. (FR-001, FR-002, FR-004a, SC-001)
func TestContextInferLetReturnOnly(t *testing.T) {
	expectOK(t, "fn empty_list[T]() -> T[] {\n let xs: T[] = []\n return xs\n}\n"+
		wrapMain("let xs: int[] = empty_list()\n print(to_string(length(xs)))"))
}

// A return-only param inside a composite return type (Optional[T]). (SC-001)
func TestContextInferLetOptional(t *testing.T) {
	expectOK(t, "fn nothing[T]() -> Optional[T] {\n return None\n}\n"+
		wrapMain("let x: Optional[int] = nothing()\n print(\"ok\")"))
}

// final binding pins the return-only param the same way. (FR-004a)
func TestContextInferFinal(t *testing.T) {
	expectOK(t, "fn empty_list[T]() -> T[] {\n let xs: T[] = []\n return xs\n}\n"+
		wrapMain("final ys: string[] = empty_list()\n print(to_string(length(ys)))"))
}

// return position: the enclosing function's declared (concrete) return type pins
// the return-only param. (FR-004b, SC-002)
func TestContextInferReturn(t *testing.T) {
	expectOK(t, "fn empty_list[T]() -> T[] {\n let xs: T[] = []\n return xs\n}\n"+
		"fn make() -> bool[] {\n return empty_list()\n}\n"+
		wrapMain("let b: bool[] = make()\n print(to_string(length(b)))"))
}

// call-argument position: a concretely-typed parameter pins the return-only param
// of a generic call passed to it. (FR-004c, SC-003)
func TestContextInferCallArg(t *testing.T) {
	expectOK(t, "fn empty_list[T]() -> T[] {\n let xs: T[] = []\n return xs\n}\n"+
		"fn takes_ints(xs: int[]) -> int {\n return length(xs)\n}\n"+
		wrapMain("let n: int = takes_ints(empty_list())\n print(to_string(n))"))
}

// assignment position: a plain-variable assignment target pins the param from the
// variable's declared type. (FR-004d, SC-004)
func TestContextInferAssign(t *testing.T) {
	expectOK(t, "fn empty_list[T]() -> T[] {\n let xs: T[] = []\n return xs\n}\n"+
		wrapMain("let xs: int[] = [1]\n xs = empty_list()\n print(to_string(length(xs)))"))
}

// array-literal element position: a known element type pins the param. (FR-004e, SC-004)
func TestContextInferArrayElem(t *testing.T) {
	expectOK(t, "fn empty_list[T]() -> T[] {\n let xs: T[] = []\n return xs\n}\n"+
		wrapMain("let grid: int[][] = [empty_list()]\n print(to_string(length(grid)))"))
}

// dict-literal VALUE element position: a known value type pins the param. (FR-004e, SC-004)
func TestContextInferDictValue(t *testing.T) {
	expectOK(t, "fn empty_list[T]() -> T[] {\n let xs: T[] = []\n return xs\n}\n"+
		wrapMain("let m: {string: int[]} = {\"a\": empty_list()}\n print(to_string(length(m[\"a\"])))"))
}

// Field-assignment RHS infers for free (uniform mechanism; see the Scope note in
// File Structure). Locking test so the behavior is not untested.
func TestContextInferFieldAssign(t *testing.T) {
	expectOK(t, "fn empty_list[T]() -> T[] {\n let xs: T[] = []\n return xs\n}\n"+
		"struct Holder { items: int[] }\n"+
		wrapMain("let h: Holder = Holder { items: [1] }\n h.items = empty_list()\n print(to_string(length(h.items)))"))
}

// Index-assignment RHS (array element target) infers for free. Locking test.
func TestContextInferIndexAssign(t *testing.T) {
	expectOK(t, "fn empty_list[T]() -> T[] {\n let xs: T[] = []\n return xs\n}\n"+
		wrapMain("let grid: int[][] = [[1]]\n grid[0] = empty_list()\n print(to_string(length(grid)))"))
}

// Value arguments win: identity's T is pinned by the argument, context agrees. (US5)
func TestContextInferArgWins(t *testing.T) {
	expectOK(t, "fn identity[T](x: T) -> T {\n return x\n}\n"+
		wrapMain("let x: int = identity(5)\n print(to_string(x))"))
}

// Value argument vs contradicting annotation: the existing initializer-mismatch
// error fires (context never overrides the arg-inferred binding). (FR-003, SC-009)
func TestContextInferArgVsAnnotationMismatch(t *testing.T) {
	expectErr(t, "fn identity[T](x: T) -> T {\n return x\n}\n"+
		wrapMain("let x: string = identity(5)\n print(x)"),
		"has type int, want string")
}

// Multi-param conflict: T is arg-pinned to int, context demands string -> the
// existing unify-conflict path fires (no new error kind). (FR-005, SC-007)
func TestContextInferMultiParamConflict(t *testing.T) {
	expectErr(t, "struct Pair[A, B] { first: A, second: B }\n"+
		"fn pair_with[T, U](x: T) -> Pair[T, U] {\n let u: U[] = []\n return Pair { first: x, second: u[0] }\n}\n"+
		wrapMain("let p: Pair[string, string] = pair_with(5)\n print(\"x\")"),
		"cannot infer type parameter T of pair_with: bound to int but also string")
}

// Multi-param success: T from arg, U from context. (FR-002, SC-007)
func TestContextInferMultiParamOK(t *testing.T) {
	expectOK(t, "struct Pair[A, B] { first: A, second: B }\n"+
		"fn pair_with[T, U](x: T) -> Pair[T, U] {\n let u: U[] = []\n return Pair { first: x, second: u[0] }\n}\n"+
		wrapMain("let p: Pair[int, string] = pair_with(5)\n print(\"x\")"))
}

// Nested unbound-parameter boundary: identity's param is an unbound type variable,
// so the inner empty_list() gets no concrete context -> still an error. (FR-010, SC-008)
func TestContextInferNestedUnboundErrors(t *testing.T) {
	expectErr(t, "fn empty_list[T]() -> T[] {\n let xs: T[] = []\n return xs\n}\n"+
		"fn identity[T](x: T) -> T {\n return x\n}\n"+
		wrapMain("let xs: int[] = identity(empty_list())\n print(to_string(length(xs)))"),
		"cannot infer type parameter T of empty_list")
}

// The same nested call compiles once the inner call is given explicit type args. (SC-008)
func TestContextInferNestedExplicitOK(t *testing.T) {
	expectOK(t, "fn empty_list[T]() -> T[] {\n let xs: T[] = []\n return xs\n}\n"+
		"fn identity[T](x: T) -> T {\n return x\n}\n"+
		wrapMain("let xs: int[] = identity(empty_list[int]())\n print(to_string(length(xs)))"))
}

// Explicit type args take precedence and are NOT overridden by context: when they
// disagree with the annotation, the existing binding-mismatch error fires (the
// call's type is int[] from the explicit [int], not string[] from context). (FR-007)
func TestContextInferExplicitArgsWinOverContext(t *testing.T) {
	expectErr(t, "fn empty_list[T]() -> T[] {\n let xs: T[] = []\n return xs\n}\n"+
		wrapMain("let xs: string[] = empty_list[int]()\n print(to_string(length(xs)))"),
		"has type int[], want string[]")
}

// Expected type carrying a type variable is not concrete: inside a generic body a
// return whose type is U[] does not drive inference; explicit args still required. (FR-004 concreteness)
func TestContextInferWantWithTypeVarErrors(t *testing.T) {
	expectErr(t, "fn empty_list[T]() -> T[] {\n let xs: T[] = []\n return xs\n}\n"+
		"fn wrap[U]() -> U[] {\n return empty_list()\n}\n"+
		wrapMain("return 0"),
		"cannot infer type parameter T of empty_list")
}

// The same generic body compiles when the inner call names the enclosing type param. (FR-004 concreteness)
func TestContextInferWantWithTypeVarExplicitOK(t *testing.T) {
	expectOK(t, "fn empty_list[T]() -> T[] {\n let xs: T[] = []\n return xs\n}\n"+
		"fn wrap[U]() -> U[] {\n return empty_list[U]()\n}\n"+
		wrapMain("let xs: int[] = wrap[int]()\n print(to_string(length(xs)))"))
}

// No context at all: a bare return-only call still cannot be inferred. (FR-006)
func TestContextInferNoContextErrors(t *testing.T) {
	expectErr(t, "fn empty_list[T]() -> T[] {\n let xs: T[] = []\n return xs\n}\n"+
		"fn identity[T](x: T) -> T {\n return x\n}\n"+
		wrapMain("let xs: int[] = identity(empty_list())\n print(to_string(length(xs)))"),
		"cannot infer type parameter")

	// A discarded bare call with no context also cannot infer.
	expectErr(t, "fn empty_list[T]() -> T[] {\n let xs: T[] = []\n return xs\n}\n"+
		wrapMain("empty_list()\n return 0"),
		"cannot infer type parameter T of empty_list")
}

// No let-type inference: an unannotated let is still a parse/type error, unchanged. (SC-010)
// (An annotation is required by the grammar; assert the feature did not relax that.)
func TestContextInferAnnotationStillRequired(t *testing.T) {
	info := check(t, "fn empty_list[T]() -> T[] {\n let xs: T[] = []\n return xs\n}\n"+
		wrapMain("let xs = empty_list()\n return 0"))
	if len(info.Errors) == 0 {
		t.Fatalf("expected an error for a let without a type annotation, got none")
	}
}
