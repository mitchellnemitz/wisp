package types

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/parser"
)

// TestTupleTypeResolves exercises resolveType on tuple annotations in return/param
// position (no constructor needed yet).
func TestTupleTypeResolves(t *testing.T) {
	cases := []string{
		"(int,string)",
		"(int,bool,int)",
		"(int[],{string:int})",
		"(int,(bool,int))",
		"Optional[(int,int)]",
		"(int,string)[]",
	}
	for _, tc := range cases {
		expectOK(t, "fn f() -> "+tc+" { return f() }\nfn main() -> int { return 0 }\n")
	}
}

// TestTupleVoidElemRejected: void element in tuple annotation is rejected. The
// parser's parseTypeName(false) rejects void before the checker; assert the
// parse-layer rejection (the live path), mirroring TestOptionalElementVoidRejected.
func TestTupleVoidElemRejected(t *testing.T) {
	_, err := parser.Parse("fn f() -> (int,void) { return f() }\nfn main() -> int { return 0 }\n", "test.wisp")
	if err == nil {
		t.Fatal("expected (int,void) type to be rejected, got no error")
	}
	if !strings.Contains(err.Error(), "void") {
		t.Errorf("error = %q, want mention of void", err.Error())
	}
}

// TestTupleUnknownElemRejected: unknown type in element is a checker error.
func TestTupleUnknownElemRejected(t *testing.T) {
	expectErr(t, "fn f() -> (int,Bogus) { return f() }\nfn main() -> int { return 0 }\n", "Bogus")
}

// TestTupleTrailingCommaTypeAccepted: (int, string,) is valid and identical to (int, string).
func TestTupleTrailingCommaTypeAccepted(t *testing.T) {
	expectOK(t, "fn f() -> (int,string,) { return f() }\nfn main() -> int { return 0 }\n")
}

// TestTupleLiteralTypeChecks: basic literal type-checking.
func TestTupleLiteralTypeChecks(t *testing.T) {
	// AC 1: annotated let round-trips
	expectOK(t, wrapMain("let t: (int,string) = (1, \"a\")"))
	// AC 11: evaluation order observable via side effects is a codegen concern;
	// checker just needs this to type-check
	expectOK(t, wrapMain("let t: (int,bool) = (42, true)"))
	// AC 3: nested
	expectOK(t, wrapMain("let t: (int,(bool,int)) = (1, (true, 2))"))
	// AC 21: trailing comma in literal
	expectOK(t, wrapMain("let t: (int,string) = (1, \"x\",)"))
}

// TestTupleLiteralVoidElemRejected: a void-typed element is a compile error.
func TestTupleLiteralVoidElemRejected(t *testing.T) {
	expectErr(t, "fn noop() -> void {}\nfn main() -> int { let t: (int, int) = (1, noop())\nreturn 0 }\n", "void")
}

// TestTupleIndexAccess: constant-index access type-checks and yields the element type.
func TestTupleIndexAccess(t *testing.T) {
	// AC 4: correct element type at index 0 and 1
	expectOK(t, wrapMain("let t: (int,string) = (1, \"a\")\nlet x: int = t[0]\nlet y: string = t[1]"))
	// AC 5: out-of-range index is a compile error that NAMES the arity. The
	// checker message is "tuple index k out of range for <type> (arity M)";
	// assert both the "range" wording and the arity number.
	expectErr(t, wrapMain("let t: (int,string) = (1, \"a\")\nlet z: int = t[2]"), "range")
	expectErr(t, wrapMain("let t: (int,string) = (1, \"a\")\nlet z: int = t[2]"), "arity 2")
	// AC 6: non-literal index is a compile error
	expectErr(t, wrapMain("let t: (int,string) = (1, \"a\")\nlet i: int = 0\nlet z: int = t[i]"), "constant integer literal")
	// AC 6: arithmetic index is also non-literal
	expectErr(t, wrapMain("let t: (int,string) = (1, \"a\")\nlet z: int = t[1+0]"), "constant integer literal")
	// An overflowing index literal must be a clean out-of-range compile error,
	// not a checker panic from a wrapped-negative int (strconv.Atoi errors on
	// overflow; the lexer accepts arbitrarily long integer literals).
	expectErr(t, wrapMain("let t: (int,string) = (1, \"a\")\nlet z: int = t[99999999999999999999]"), "range")
}

// TestTupleImmutable: t[0] = x is rejected by the existing index-assign guard.
func TestTupleImmutable(t *testing.T) {
	// AC 9: the existing guard rejects non-array, non-dict base
	expectErr(t, wrapMain("let t: (int,string) = (1, \"a\")\nt[0] = 99"), "cannot index-assign")
}

// TestTupleOpaque: ==, !=, to_string(), switch (t) are each compile errors.
func TestTupleOpaque(t *testing.T) {
	// AC 8: == is rejected (isHandle membership -> "aggregate handles are opaque")
	expectErr(t, wrapMain("let t: (int,string) = (1, \"a\")\nlet u: (int,string) = (2, \"b\")\nlet eq: bool = t == u"), "")
	// AC 8: != is also rejected (same handle-opacity guard)
	expectErr(t, wrapMain("let t: (int,string) = (1, \"a\")\nlet u: (int,string) = (2, \"b\")\nlet ne: bool = t != u"), "")
	// AC 8: to_string(t) is rejected by the to_string() builtin signature
	expectErr(t, wrapMain("let t: (int,string) = (1, \"a\")\nlet s: string = to_string(t)"), "")
	// AC 8: switch (t) is rejected by checkSwitch subject restriction
	expectErr(t, wrapMain("let t: (int,string) = (1, \"a\")\nswitch (t) { default { } }"), "")
}

// TestTupleNoneErrNeedsContext: None/Err in a tuple element is a needs-context error.
func TestTupleNoneErrNeedsContext(t *testing.T) {
	// AC 10: tuple element is not a blessed expected-type site -- even with a
	// tuple annotation, None/Err inside an element get no expected type, because
	// checkTupleLit checks elements bottom-up without threading the annotation in.
	expectErr(t, wrapMain("let t: (int, Optional[int]) = (1, None)"), "context")
	expectErr(t, wrapMain("let t: (int, Result[int]) = (1, Err(error(\"x\")))"), "context")
}

// TestTupleInParamReturnPosition: tuple types work as parameter and return types.
func TestTupleInParamReturnPosition(t *testing.T) {
	// AC 12, return half: a function returns a tuple, the caller indexes it.
	expectOK(t, "fn pair(a: int, b: string) -> (int,string) { return (a, b) }\nfn main() -> int {\nlet r: (int,string) = pair(1, \"x\")\nlet x: int = r[0]\nreturn 0 }\n")
	// AC 12, parameter half: a tuple VALUE is passed into a tuple-typed parameter
	// and an element is read inside the callee.
	expectOK(t, "fn f(t: (int, string)) -> int { return t[0] }\nfn main() -> int {\nlet p: (int,string) = (1, \"a\")\nreturn f(p) }\n")
}

// TestIndexExprArrayDictUnchanged: array and dict indexing with a RUNTIME (variable)
// index still type-checks after the checkIndexExpr tuple branch is added (AC 7).
func TestIndexExprArrayDictUnchanged(t *testing.T) {
	expectOK(t, wrapMain("let a: int[] = [1, 2]\nlet i: int = 0\nlet x: int = a[i]"))
	expectOK(t, wrapMain("let d: {string:int} = {\"k\": 1}\nlet v: int = d[\"k\"]"))
}

// TestTupleUnknownElemCheckerError: Bogus element in annotation is a checker error.
// The void case is rejected by the parser (parseTypeName(false) disallows void);
// Bogus is rejected by the checker (unknown type name).
func TestTupleUnknownElemCheckerError(t *testing.T) {
	// AC 22: unknown struct name rejected by checker
	expectErr(t, wrapMain("let t: (int,Bogus) = (1, 1)"), "")
	// AC 22: void rejected at parse level (mirrors TestTupleVoidElemRejected)
	_, err := parser.Parse("fn main() -> int {\nlet t: (int,void) = (1, 1)\nreturn 0\n}", "test.wisp")
	if err == nil {
		t.Fatal("expected (int,void) to be rejected, got no error")
	}
	if !strings.Contains(err.Error(), "void") {
		t.Errorf("error = %q, want mention of void", err.Error())
	}
}

// TestTupleErrorsPositioned: every diagnostic carries a source position (AC 23).
// The expectErr helper already asserts a non-empty positioned error; these tests
// ensure specific tuple errors are positioned.
func TestTupleErrorsPositioned(t *testing.T) {
	// non-literal index error is positioned
	expectErr(t, wrapMain("let t: (int,string) = (1, \"a\")\nlet i: int = 0\nlet z: int = t[i]"), "constant integer literal")
	// out-of-range error is positioned
	expectErr(t, wrapMain("let t: (int,string) = (1, \"a\")\nlet z: int = t[5]"), "range")
}

// zip is now removable (array.zip); its type-check, non-array rejection, arity,
// and reserved-name coverage lives in core_arrays_test.go (TestCoreArraysZip).
