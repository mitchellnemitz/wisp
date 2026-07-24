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
