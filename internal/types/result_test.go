package types

import "testing"

func TestResultTypeResolves(t *testing.T) {
	// Type appears only in annotation position; no constructor needed yet.
	for _, src := range []string{"Result[int]", "Result[int[]]", "Result[Optional[int]]", "Result[Result[int]]", "Result[{string:int}]"} {
		expectOK(t, "fn f() -> "+src+" { return f() }\nfn main() -> int { return 0 }\n")
	}
	// AC 1 also requires the Optional-of-Result nesting direction:
	expectOK(t, "fn g() -> Optional[Result[int]] { return g() }\nfn main() -> int { return 0 }\n")
}

func TestResultReservedNames(t *testing.T) {
	expectErr(t, wrapMain("let Result: int = 1\n"), "Result")
	expectErr(t, wrapMain("let Ok: int = 1\n"), "Ok")
	expectErr(t, wrapMain("let Err: int = 1\n"), "Err")
	// AC 15: Ok/Err rejected as FUNCTION names too.
	expectErr(t, "fn Ok() -> int { return 0 }\nfn main() -> int { return 0 }\n", "Ok")
	expectErr(t, "fn Err() -> int { return 0 }\nfn main() -> int { return 0 }\n", "Err")
	expectParseFail(t, "fn f[Result]() -> void {}\n")
}

func TestOkInference(t *testing.T) {
	expectOK(t, wrapMain("let r: Result[int] = Ok(42)"))
	expectOK(t, wrapMain("let r: Result[string] = Ok(\"x\")"))
	// Ok infers Result[typeof x] with NO blessed threading: in argument position it
	// type-checks against the parameter (unlike Err, which needs context). wisp has
	// no inferred let, so argument position is where context-free inference shows.
	expectOK(t, "fn f(r: Result[int]) -> int { return 0 }\nfn main() -> int { return f(Ok(1)) }\n")
}

func TestErrConcretizesAtBlessedSites(t *testing.T) {
	expectOK(t, wrapMain("let r: Result[int] = Err(error(\"boom\"))"))                                 // let-init
	expectOK(t, "fn p() -> Result[int] { return Err(error(\"e\")) }\nfn main() -> int { return 0 }\n") // return
	expectOK(t, wrapMain("let r: Result[int] = Ok(1)\nr = Err(error(\"e\"))"))                         // assignment
}

func TestErrNeedsContext(t *testing.T) {
	// A non-Result annotation gives Err no success type to concretize.
	expectErr(t, wrapMain("let r: int = Err(error(\"x\"))\n"), "context")
	// AC 4: Err in argument position (no blessed threading) is needs-context.
	expectErr(t, "fn f(r: Result[int]) -> int { return 0 }\nfn main() -> int { return f(Err(error(\"x\"))) }\n", "context")
}

func TestConstructorArgErrors(t *testing.T) {
	expectErr(t, wrapMain("let r: Result[int] = Err(5)\n"), "error")                          // non-error arg
	expectErr(t, wrapMain("let r: Result[Optional[int]] = Ok(None)\n"), "context")            // inner None no context (AC5)
	expectErr(t, wrapMain("let r: Result[Result[int]] = Ok(Err(error(\"x\")))\n"), "context") // inner Err no context (AC5)
}

func TestErrInCollectionDeferred(t *testing.T) {
	// AC 5a: Err in a collection literal is the deferred-position compile error.
	expectErr(t, wrapMain("let xs: Result[int][] = [Err(error(\"x\"))]\n"), "context")
}

func TestResultAccessorTypes(t *testing.T) {
	expectOK(t, wrapMain("let r: Result[int] = Ok(1)\nlet b: bool = is_ok(r)\nlet c: bool = is_err(r)\nlet v: int = unwrap(r)\nlet e: error = unwrap_err(r)\nlet w: int = unwrap_or(r, 9)"))
}

func TestAccessorFamilyMismatch(t *testing.T) {
	expectErr(t, wrapMain("let o: Optional[int] = Some(1)\nlet b: bool = is_ok(o)\n"), "Result")
	expectErr(t, wrapMain("let r: Result[int] = Ok(1)\nlet b: bool = is_some(r)\n"), "Optional")
	expectErr(t, wrapMain("let o: Optional[int] = Some(1)\nlet e: error = unwrap_err(o)\n"), "Result")
}

func TestUnwrapOverloadResult(t *testing.T) {
	expectErr(t, wrapMain("let r: Result[int] = Ok(1)\nlet v: string = unwrap(r)\n"), "string")     // unwrap(r) is int
	expectErr(t, wrapMain("let r: Result[int] = Ok(1)\nlet w: int = unwrap_or(r, \"x\")\n"), "int") // fallback must be int
}

func TestResultVoidRejected(t *testing.T) {
	// Rejected at parse time (parseTypeName uses allowVoid=false for the element),
	// mirroring Optional[void]. The checker's "result success type cannot be void"
	// guard is the defensive backstop for void arriving via a non-literal path.
	expectParseFail(t, wrapMain("let r: Result[void] = Ok(0)\n"))
}
