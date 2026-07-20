package types

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/parser"
)

// --- error type / error() builtin / .message ---

func TestErrorConstructAndMessage(t *testing.T) {
	expectOK(t, wrapMain(`let e: error = error("boom")
print(e.message)`))
}

func TestErrorConstructWrongArg(t *testing.T) {
	expectErr(t, wrapMain(`let e: error = error(5)`), "argument 1")
}

func TestErrorConstructArity(t *testing.T) {
	expectErr(t, wrapMain(`let e: error = error("a", "b")`), "error")
}

func TestErrorMessageIsString(t *testing.T) {
	expectOK(t, wrapMain(`let e: error = error("x")
let s: string = e.message`))
}

func TestErrorNoSuchField(t *testing.T) {
	expectErr(t, wrapMain(`let e: error = error("x")
let c: int = e.details`), "field")
}

func TestErrorCodeIsInt(t *testing.T) {
	expectOK(t, wrapMain(`let e: error = error("x")
let c: int = e.code`))
}

func TestErrorWithOK(t *testing.T) {
	expectOK(t, wrapMain(`let e: error = error_with(42, "boom")
print(e.message)
print(to_string(e.code))`))
}

func TestErrorWithCodeIsInt(t *testing.T) {
	expectOK(t, wrapMain(`let e: error = error_with(1, "x")
let c: int = e.code`))
}

func TestErrorWithMessageIsString(t *testing.T) {
	expectOK(t, wrapMain(`let e: error = error_with(0, "x")
let s: string = e.message`))
}

func TestErrorWithWrongArgOrder(t *testing.T) {
	expectErr(t, wrapMain(`let e: error = error_with("msg", 42)`), "argument 1")
}

func TestErrorWithArity(t *testing.T) {
	expectErr(t, wrapMain(`let e: error = error_with(1, "x", "extra")`), "error_with")
}

func TestErrorWithThrowCatch(t *testing.T) {
	expectOK(t, wrapMain(`try {
  throw error_with(5, "oops")
} catch (e) {
  print(e.message)
  print(to_string(e.code))
}`))
}

// --- error opacity (handle: no int/arith/compare) ---

func TestErrorOpaqueNoArith(t *testing.T) {
	expectErr(t, wrapMain(`let e: error = error("x")
let n: int = e + 1`), "requires")
}

func TestErrorOpaqueNoCompare(t *testing.T) {
	expectErr(t, wrapMain(`let a: error = error("x")
let b: error = error("y")
let c: bool = a == b`), "opaque")
}

func TestErrorOpaqueNoInterpolate(t *testing.T) {
	// e.message is renderable; the error handle itself is not.
	expectErr(t, wrapMain(`let e: error = error("x")
print("${e}")`), "")
}

// --- error reserved name ---
//
// `error` was reserved as a keyword in M1, so using it as a struct/func/var
// name is a PARSE error (the lexer never yields an Ident named "error"). These
// assert the reservation still holds: parsing must fail.

func TestErrorReservedAsStructName(t *testing.T) {
	expectParseFail(t, "struct error { x: int }\nfn main() -> int { return 0 }")
}

func TestErrorReservedAsFuncName(t *testing.T) {
	expectParseFail(t, "fn error(s: string) -> int { return 0 }\nfn main() -> int { return 0 }")
}

func TestErrorReservedAsVarName(t *testing.T) {
	expectParseFail(t, wrapMain(`let error: int = 5`))
}

// expectParseFail asserts that src fails to parse (the reserved keyword cannot
// be used as a name).
func expectParseFail(t *testing.T, src string) {
	t.Helper()
	_, err := parser.Parse(src, "test.wisp")
	if err == nil {
		t.Fatalf("expected a parse error, got none\nsrc:\n%s", src)
	}
}

// --- throw ---

func TestThrowError(t *testing.T) {
	expectOK(t, wrapMain(`try {
  throw error("boom")
} catch (e) {
  print(e.message)
}`))
}

func TestThrowNonError(t *testing.T) {
	expectErr(t, wrapMain(`throw "boom"`), "error")
}

func TestThrowIntNonError(t *testing.T) {
	expectErr(t, wrapMain(`throw 5`), "error")
}

func TestThrowTerminatesPath(t *testing.T) {
	// A function whose every path ends in return or throw satisfies
	// all-paths-return even with no trailing return.
	expectOK(t, `fn f(x: int) -> int {
  if (x > 0) {
    return x
  } else {
    throw error("neg")
  }
}
fn main() -> int { return 0 }`)
}

// --- try/catch/finally typing ---

func TestTryCatchOK(t *testing.T) {
	expectOK(t, wrapMain(`try {
  let n: int = to_int("5")
  print(to_string(n))
} catch (e) {
  print(e.message)
}`))
}

func TestTryCatchFinallyOK(t *testing.T) {
	expectOK(t, wrapMain(`try {
  print("body")
} catch (e) {
  print("handler")
} finally {
  print("cleanup")
}`))
}

func TestCatchBindsErrorTyped(t *testing.T) {
	// e is typed error in the handler: e.message is a string.
	expectOK(t, wrapMain(`try {
  print("b")
} catch (e) {
  let s: string = e.message
  print(s)
}`))
}

func TestCatchVarNotVisibleOutside(t *testing.T) {
	expectErr(t, wrapMain(`try {
  print("b")
} catch (e) {
  print("c")
}
print(e.message)`), "undeclared")
}

// --- control-flow restrictions (compile errors) ---

func TestReturnInTryBodyError(t *testing.T) {
	expectErr(t, `fn main() -> int {
  try {
    return 1
  } catch (e) {
    print("c")
  }
  return 0
}`, "return")
}

func TestReturnInCatchError(t *testing.T) {
	expectErr(t, `fn main() -> int {
  try {
    print("b")
  } catch (e) {
    return 1
  }
  return 0
}`, "return")
}

func TestReturnInFinallyError(t *testing.T) {
	expectErr(t, `fn main() -> int {
  try {
    print("b")
  } catch (e) {
    print("c")
  } finally {
    return 1
  }
  return 0
}`, "return")
}

func TestBreakInTryBodyError(t *testing.T) {
	expectErr(t, wrapMain(`for (let i: int = 0; i < 3; i = i + 1) {
  try {
    break
  } catch (e) {
    print("c")
  }
}`), "break")
}

func TestContinueInTryBodyError(t *testing.T) {
	expectErr(t, wrapMain(`for (let i: int = 0; i < 3; i = i + 1) {
  try {
    continue
  } catch (e) {
    print("c")
  }
}`), "continue")
}

func TestBreakInCatchError(t *testing.T) {
	expectErr(t, wrapMain(`while (true) {
  try {
    print("b")
  } catch (e) {
    break
  }
}`), "break")
}

// break/continue OUTSIDE any try (but inside a try elsewhere in the program)
// still work normally.
func TestBreakOutsideTryOK(t *testing.T) {
	expectOK(t, wrapMain(`for (let i: int = 0; i < 3; i = i + 1) {
  if (i == 1) { break }
  try {
    print("b")
  } catch (e) {
    print("c")
  }
}`))
}

// --- error value-flow (param / return) ---

func TestErrorAsParam(t *testing.T) {
	expectOK(t, `fn report(e: error) -> string {
  return e.message
}
fn main() -> int { return 0 }`)
}

func TestErrorAsReturn(t *testing.T) {
	expectOK(t, `fn make() -> error {
  return error("x")
}
fn main() -> int { return 0 }`)
}

// A try is NOT a terminating path: a function whose only statement is a try does
// not satisfy all-paths-return.
func TestTryNotTerminating(t *testing.T) {
	expectErr(t, `fn f() -> int {
  try {
    print("b")
  } catch (e) {
    print("c")
  }
}
fn main() -> int { return 0 }`, "every path")
}

// --- wrap / cause builtins ---

// wrap(error, string) -> error: type-checks correctly.
func TestWrapOK(t *testing.T) {
	expectOK(t, wrapMain(`let e: error = error("inner")
let w: error = wrap(e, "outer")`))
}

// cause(error) -> Optional[error]: type-checks correctly.
func TestCauseOK(t *testing.T) {
	expectOK(t, wrapMain(`let e: error = error("x")
let o: Optional[error] = cause(e)`))
}

// cause(wrap(e, msg)) is Optional[error] (composable).
func TestCauseOfWrapOK(t *testing.T) {
	expectOK(t, wrapMain(`let e: error = error("inner")
let w: error = wrap(e, "outer")
let o: Optional[error] = cause(w)`))
}

// AC1a: wrap is reserved -- let wrap = ... is a located "reserved" error.
func TestWrapReservedAsVar(t *testing.T) {
	expectErr(t, wrapMain(`let wrap: int = 5`), "reserved")
}

// AC1a: cause is reserved -- let cause = ... is a located "reserved" error.
func TestCauseReservedAsVar(t *testing.T) {
	expectErr(t, wrapMain(`let cause: int = 5`), "reserved")
}

// AC1a: wrap reserved as function name.
func TestWrapReservedAsFunc(t *testing.T) {
	expectErr(t, `fn wrap(e: error, m: string) -> error { return e }
fn main() -> int { return 0 }`, "reserved")
}

// AC1a: cause reserved as function name.
func TestCauseReservedAsFunc(t *testing.T) {
	expectErr(t, `fn cause(e: error) -> bool { return true }
fn main() -> int { return 0 }`, "reserved")
}

// wrap wrong arg 1 type: wrap("s", "m") -> checker error.
func TestWrapWrongArg1Type(t *testing.T) {
	expectErr(t, wrapMain(`let w: error = wrap("s", "m")`), "argument 1")
}

// wrap wrong arg 2 type: wrap(e, 5) -> checker error.
func TestWrapWrongArg2Type(t *testing.T) {
	expectErr(t, wrapMain(`let e: error = error("x")
let w: error = wrap(e, 5)`), "argument 2")
}

// cause wrong arg type: cause("s") -> checker error.
func TestCauseWrongArgType(t *testing.T) {
	expectErr(t, wrapMain(`let o: Optional[error] = cause("s")`), "argument 1")
}

// wrap with one arg (missing msg) -> arity error.
func TestWrapArityOneArg(t *testing.T) {
	expectErr(t, wrapMain(`let e: error = error("x")
let w: error = wrap(e)`), "wrap")
}

// cause with extra arg -> arity error.
func TestCauseArityExtraArg(t *testing.T) {
	expectErr(t, wrapMain(`let e: error = error("x")
let x: error = error("y")
let o: Optional[error] = cause(e, x)`), "cause")
}
