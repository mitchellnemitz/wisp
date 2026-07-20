package types

import "testing"

// Extended-math builtins (math.exp/math.ln/math.log10/math.log2 float->float;
// math.pi nullary ->float). Float-only args, matching sqrt: an int argument is
// REJECTED (no int->float coercion). All five are removable builtins now
// namespaced under math, so every test below checks through the linked
// module set with the math namespace bound.

func TestExtMath_UnaryFloatToFloat_OK(t *testing.T) {
	expectOKNS(t, wrapMain(`let a: float = math.exp(1.0)
let b: float = math.ln(2.0)
let c: float = math.log10(1000.0)
let d: float = math.log2(8.0)`), "math")
}

func TestExtMath_Pi_NullaryFloat_OK(t *testing.T) {
	expectOKNS(t, wrapMain(`let p: float = math.pi()`), "math")
}

func TestExtMath_ResultUsableAsFloat_OK(t *testing.T) {
	expectOKNS(t, wrapMain(`let f: float = math.exp(1.0) + math.ln(2.0) + math.pi()`), "math")
}

func TestExtMath_IntArgRejected(t *testing.T) {
	// No int->float coercion (like sqrt). A bare int is a type error.
	expectErrNS(t, wrapMain(`let a: float = math.exp(1)`), "argument 1 of math.exp has type int, want float", "math")
	expectErrNS(t, wrapMain(`let b: float = math.ln(2)`), "argument 1 of math.ln has type int, want float", "math")
	expectErrNS(t, wrapMain(`let c: float = math.log10(1000)`), "argument 1 of math.log10 has type int, want float", "math")
	expectErrNS(t, wrapMain(`let d: float = math.log2(8)`), "argument 1 of math.log2 has type int, want float", "math")
}

func TestExtMath_ResultNotInt(t *testing.T) {
	expectErrNS(t, wrapMain(`let n: int = math.exp(1.0)`), "want int", "math")
	expectErrNS(t, wrapMain(`let n: int = math.pi()`), "want int", "math")
}

func TestExtMath_WrongArity(t *testing.T) {
	expectErrNS(t, wrapMain(`let a: float = math.exp()`), "math.exp expects 1 argument, got 0", "math")
	expectErrNS(t, wrapMain(`let a: float = math.ln(1.0, 2.0)`), "math.ln expects 1 argument, got 2", "math")
	expectErrNS(t, wrapMain(`let a: float = math.pi(1.0)`), "math.pi expects 0 arguments, got 1", "math")
}

// exp/ln/log10/log2/pi are removable builtins: their flat names were freed by
// the modules-only migration (isReservedName excludes the removable set), so
// a user may now define a function with one of these names -- unlike the
// pre-removal original, which reserved them as bare builtin names.
func TestExtMath_NamesFreed(t *testing.T) {
	for _, name := range []string{"exp", "ln", "log10", "log2", "pi"} {
		src := "fn " + name + "() -> int {\n  return 0\n}\nfn main() -> int {\n  return 0\n}"
		expectOKNS(t, src, "math")
	}
}
