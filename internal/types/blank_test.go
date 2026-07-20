package types

import "testing"

func TestBlankAsValueIsError(t *testing.T) {
	// Every rvalue position rejects a bare `_`.
	expectErr(t, wrapMain("let x: int = _"), "cannot use _ as a value")
	expectErr(t, wrapMain("print(to_string(_))"), "cannot use _ as a value")
	expectErr(t, wrapMain("let x: int = _ + 1"), "cannot use _ as a value")
	expectErr(t, "fn f() -> int { return _ }\nfn main() -> int { return 0 }\n", "cannot use _ as a value")
	expectErr(t, wrapMain("if (_) { print(\"x\") }"), "cannot use _ as a value")
	expectErr(t, wrapMain("let a: int = 1\nlet b: bool = _ == a"), "cannot use _ as a value")
}

func TestBlankLetAndAssign(t *testing.T) {
	// Blank let: RHS still type-checked, no binding, no unused warning.
	expectOK(t, wrapMain("let _: int = 5"))
	// Two blank lets in one block are legal (a named redeclaration would error).
	expectOK(t, wrapMain("let _: int = 1\nlet _: int = 2"))
	// RHS type mismatch is still reported through a blank let.
	expectErr(t, wrapMain("let _: int = \"str\""), "want int")
	// Blank assignment: previously "undeclared variable", now legal.
	expectOK(t, wrapMain("_ = 5"))
	// RHS type errors still surface through a blank assignment.
	expectErr(t, wrapMain("_ = nope"), "nope")
	// Spec §4.2: `_` as the base of a field/index assignment is a value read of
	// `_`, so it errors via checkIdent. (`checkFieldAssign`/`checkIndexAssign`
	// resolve the target through `checkExpr`, routing `_` into checkIdent.)
	expectErr(t, wrapMain("_.field = 1"), "cannot use _ as a value")
	expectErr(t, wrapMain("_[0] = 1"), "cannot use _ as a value")
}

func TestBlankLetNoUnusedWarning(t *testing.T) {
	info := check(t, wrapMain("let _: int = 5"))
	for _, w := range info.Warnings {
		if w.Msg == "unused variable \"_\"" {
			t.Fatalf("blank let must not warn unused: %v", info.Warnings)
		}
	}
}

func TestBlankForIn(t *testing.T) {
	// Array for-in with blank loop var.
	expectOK(t, wrapMain("let xs: int[] = [1, 2, 3]\nfor (_ in xs) { print(\"x\") }"))
	// Dict for-in with blank loop var.
	expectOK(t, wrapMain("let d: {string: int} = {\"a\": 1}\nfor (_ in d) { print(\"x\") }"))
	// Reading `_` inside the body is still an error.
	expectErr(t, wrapMain("let xs: int[] = [1]\nfor (_ in xs) { print(to_string(_)) }"), "cannot use _ as a value")
	// C-style for with a blank init (delegates to blank let).
	expectOK(t, wrapMain("for (let _: int = 0; false; _ = 1) { print(\"x\") }"))
}

func TestBlankParams(t *testing.T) {
	expectOK(t, "fn f(_: int) -> int { return 0 }\nfn main() -> int { return f(1) }\n")
	// Two blank params are legal (no "declared more than once").
	expectOK(t, "fn f(_: int, _: string) -> int { return 0 }\nfn main() -> int { return f(1, \"a\") }\n")
	// A blank param is not in the function body scope.
	expectErr(t, "fn f(_: int) -> int { return _ }\nfn main() -> int { return 0 }\n", "cannot use _ as a value")
	// A non-blank duplicate still errors.
	expectErr(t, "fn f(x: int, x: int) -> int { return 0 }\nfn main() -> int { return 0 }\n", "declared more than once")
}

func TestBlankMatchBinder(t *testing.T) {
	// Blank binder in a match arm is legal (no binding created).
	expectOK(t, wrapMain("let o: Optional[int] = Some(1)\nmatch (o) { case Some(_) { print(\"y\") } case None { print(\"n\") } }"))
	expectOK(t, wrapMain("let r: Result[int] = Ok(1)\nmatch (r) { case Ok(_) { print(\"y\") } case Err(_) { print(\"n\") } }"))
	expectOK(t, wrapMain("let r: Result[int] = Ok(1)\nmatch (r) { case Err(_) { print(\"e\") } case Ok(_) { print(\"y\") } }"))
	// Reading `_` in an arm-body is still an error.
	expectErr(t, wrapMain("let o: Optional[int] = Some(1)\nmatch (o) { case Some(_) { print(to_string(_)) } case None { print(\"n\") } }"), "cannot use _ as a value")
}

func TestBlankCatch(t *testing.T) {
	expectOK(t, wrapMain("try { throw error(\"e\") } catch (_) { print(\"caught\") }"))
	// Reading `_` in the handler is still an error.
	expectErr(t, wrapMain("try { throw error(\"e\") } catch (_) { print(to_string(_)) }"), "cannot use _ as a value")
}
