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
