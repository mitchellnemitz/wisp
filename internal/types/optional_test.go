package types

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/parser"
)

// TestOptionalElementVoidRejected: Optional[void] is rejected. The parser's
// element parse uses parseTypeName(false), so void is rejected at the parse
// layer ("void is only valid as a return type") before the checker's
// "optional element type cannot be void" guard is reachable. Either way the
// void element is rejected; assert the parse-layer rejection (the live path).
func TestOptionalElementVoidRejected(t *testing.T) {
	_, err := parser.Parse(`fn f(a: Optional[void]) -> int { return 0 } fn main() -> int { return 0 }`, "test.wisp")
	if err == nil {
		t.Fatal("expected Optional[void] to be rejected, got no error")
	}
	if !strings.Contains(err.Error(), "void") {
		t.Errorf("error = %q, want mention of void", err.Error())
	}
}

func TestOptionalReservedAsFuncName(t *testing.T) {
	expectErr(t, `fn Optional() -> int { return 0 } fn main() -> int { return 0 }`, "reserved")
}

func TestOptionalReservedAsVarName(t *testing.T) {
	expectErr(t, wrapMain(`let Optional: int = 0`), "reserved")
}

func TestOptionalReservedAsStructName(t *testing.T) {
	expectErr(t, `struct Optional { x: int } fn main() -> int { return 0 }`, "reserved")
}

// --- Task 2: Some / None typing ---

func TestSomeAndNoneTyping(t *testing.T) {
	expectOK(t, wrapMain(`let a: Optional[int] = Some(1)`))
	expectErr(t, wrapMain(`let a: Optional[string] = Some(1)`), "want Optional[string]")
	expectOK(t, wrapMain(`let a: Optional[int] = None`))
	expectOK(t, `fn f() -> Optional[int] { return None } fn main() -> int { return 0 }`)
	expectOK(t, wrapMain(`let a: Optional[int] = Some(1)`+"\n"+`a = None`))
}

func TestSomeOfNoneIsError(t *testing.T) {
	expectErr(t, wrapMain(`let a: Optional[int] = Some(None)`), "none requires an expected Optional type")
}

func TestSomeWrongTargetType(t *testing.T) {
	expectErr(t, wrapMain(`let a: int = Some(1)`), "want int")
}

func TestSomeOfVoidIsError(t *testing.T) {
	expectErr(t, "fn v() -> void {} fn main() -> int {\nlet a: Optional[int] = Some(v())\nreturn 0\n}", "Some requires a value, got void")
}

func TestNoneDeferredSitesAreErrors(t *testing.T) {
	expectErr(t, `fn g(o: Optional[int]) -> int { return 0 } fn main() -> int { return g(None) }`, "none requires an expected Optional type")
	expectErr(t, wrapMain(`let xs: Optional[int][] = [None]`), "none requires an expected Optional type")
	expectErr(t, wrapMain(`None`), "none requires an expected Optional type")
}

func TestSomeNoneReservedRedefinition(t *testing.T) {
	expectErr(t, `fn Some(x: int) -> int { return x } fn main() -> int { return 0 }`, "reserved")
	expectErr(t, wrapMain(`let None: int = 0`), "reserved")
}

// TestNoSentinelLeak (acceptance 6): no diagnostic, no recorded type, and no
// declared binding/param/return type ever contains "Optional[?]" or a standalone
// "?". Scans a program with None at all three blessed sites plus the deferred
// error sites.
func TestNoSentinelLeak(t *testing.T) {
	srcs := []string{
		wrapMain(`let a: Optional[int] = None`),
		`fn f() -> Optional[int] { return None } fn main() -> int { return 0 }`,
		wrapMain(`let a: Optional[int] = Some(1)` + "\n" + `a = None`),
		wrapMain(`let a: Optional[int] = Some(None)`),
		wrapMain(`let xs: Optional[int][] = [None]`),
		wrapMain(`None`),
	}
	for _, src := range srcs {
		info := check(t, src)
		for _, d := range info.Errors {
			if strings.Contains(d.Msg, "Optional[?]") || strings.Contains(d.Msg, "[?]") {
				t.Errorf("diagnostic leaks sentinel: %q\nsrc:\n%s", d.Msg, src)
			}
		}
		for _, d := range info.Warnings {
			if strings.Contains(d.Msg, "Optional[?]") || strings.Contains(d.Msg, "[?]") {
				t.Errorf("warning leaks sentinel: %q\nsrc:\n%s", d.Msg, src)
			}
		}
		for _, ty := range info.Types {
			if strings.Contains(string(ty), "?") {
				t.Errorf("info.Types contains sentinel %q\nsrc:\n%s", ty, src)
			}
		}
	}
}

// --- Task 3: access builtins + get ---

func TestIsSomeIsNoneTyping(t *testing.T) {
	expectOK(t, wrapMain(`let o: Optional[int] = Some(1)`+"\n"+`let b: bool = is_some(o)`))
	expectOK(t, wrapMain(`let o: Optional[int] = Some(1)`+"\n"+`let b: bool = is_none(o)`))
}

func TestUnwrapTyping(t *testing.T) {
	expectOK(t, wrapMain(`let o: Optional[int] = Some(1)`+"\n"+`let x: int = unwrap(o)`))
	expectErr(t, wrapMain(`let x: string = unwrap(Some(1))`), "want string")
	expectErr(t, wrapMain(`let x: int = unwrap(5)`), "unwrap requires an Optional")
}

func TestUnwrapOrTyping(t *testing.T) {
	expectOK(t, wrapMain(`let o: Optional[int] = Some(1)`+"\n"+`let x: int = unwrap_or(o, -1)`))
	expectErr(t, wrapMain(`let o: Optional[int] = Some(1)`+"\n"+`let x: int = unwrap_or(o, "z")`), "want int")
}

func TestAccessBuiltinsOnBareNoneAreErrors(t *testing.T) {
	expectErr(t, wrapMain(`let b: bool = is_some(None)`), "none requires an expected Optional type")
	expectErr(t, wrapMain(`let x: int = unwrap(None)`), "none requires an expected Optional type")
	expectErr(t, wrapMain(`let o: Optional[int] = Some(1)`+"\n"+`let x: int = unwrap_or(o, None)`), "none requires an expected Optional type")
}

func TestUnwrapOrBoundNoneFallback(t *testing.T) {
	expectOK(t, wrapMain(`let inner: Optional[int] = None`+"\n"+`let oo: Optional[Optional[int]] = Some(inner)`+"\n"+`let r: Optional[int] = unwrap_or(oo, inner)`))
}

// get is removable (dict.get); its typing checks migrated to the namespaced form,
// which CheckLinked resolves via the dict module (helpers wantNsOK/wantNsErr live
// in core_collections_neg_test.go).
func TestGetTyping(t *testing.T) {
	wantNsOK(t, "dict", `fn main() -> int { let d: {string:int} = {"a": 1}
let o: Optional[int] = dict.get(d, "a")
return 0 }`)
	wantNsErr(t, "dict", `fn main() -> int { let d: {string:int} = {"a": 1}
let o: Optional[int] = dict.get(d, 1)
return 0 }`, "dict key type")
	wantNsErr(t, "dict", `fn main() -> int { let o: Optional[int] = dict.get(5, 1)
return 0 }`, "must be a dict")
}

// --- Task 5: opacity / rejection ---

// assertNoSentinel fails if any diagnostic mentions a "?"-bearing sentinel.
func assertNoSentinel(t *testing.T, d Diagnostic) {
	t.Helper()
	if strings.Contains(d.Msg, "Optional[?]") || strings.Contains(d.Msg, "[?]") {
		t.Errorf("diagnostic leaks sentinel: %q", d.Msg)
	}
}

func TestOptionalFloatEqNeqRejected(t *testing.T) {
	// float inner is non-comparable: == / != must still be rejected.
	d := expectErr(t, wrapMain(`let o: Optional[float] = Some(1.0)`+"\n"+`let b: bool = o == o`), "use is_some/is_none and unwrap")
	assertNoSentinel(t, d)
	d = expectErr(t, wrapMain(`let o: Optional[float] = Some(1.0)`+"\n"+`let p: Optional[float] = Some(2.0)`+"\n"+`let b: bool = o != p`), "use is_some/is_none and unwrap")
	assertNoSentinel(t, d)
}

func TestOptionalStringRejected(t *testing.T) {
	d := expectErr(t, wrapMain(`let o: Optional[int] = Some(1)`+"\n"+`print(to_string(o))`), "Optional[int]")
	assertNoSentinel(t, d)
	if !strings.Contains(d.Msg, "want int|float|bool|string") {
		t.Errorf("to_string(Optional) message = %q, want generic arg-type text", d.Msg)
	}
}

func TestOptionalInterpolationRejected(t *testing.T) {
	d := expectErr(t, wrapMain(`let o: Optional[int] = Some(1)`+"\n"+`print("${o}")`), "Optional")
	assertNoSentinel(t, d)
}

func TestOptionalSwitchSubjectRejected(t *testing.T) {
	d := expectErr(t, wrapMain(`let o: Optional[int] = Some(1)`+"\n"+`switch (o) { default { } }`), "switch subject must be int, string, or bool")
	assertNoSentinel(t, d)
}

func TestOptionalNumericRejected(t *testing.T) {
	d := expectErr(t, wrapMain(`let o: Optional[int] = Some(1)`+"\n"+`let x: int = o + 1`), "requires int+int")
	assertNoSentinel(t, d)
	if !strings.Contains(d.Msg, "Optional[int]") {
		t.Errorf("numeric message = %q, want concrete Optional[int]", d.Msg)
	}
}

func TestOptionalIndexRejected(t *testing.T) {
	d := expectErr(t, wrapMain(`let o: Optional[int] = Some(1)`+"\n"+`let x: int = o[0]`), "cannot index")
	assertNoSentinel(t, d)
}

func TestOptionalCalleeRejected(t *testing.T) {
	d := expectErr(t, wrapMain(`let o: Optional[int] = Some(1)`+"\n"+`o()`), "not callable")
	assertNoSentinel(t, d)
}

// Some/None are reserved constants, but print's `to` must still be only
// stdout/stderr -- they must not slip through the stream-target check.
func TestPrintToRejectsSomeNone(t *testing.T) {
	expectErr(t, wrapMain(`print("x", Some)`), "must be the constant stdout or stderr")
	expectErr(t, wrapMain(`print("x", None)`), "must be the constant stdout or stderr")
}

// When the annotation is itself invalid, None must not pile a noneNeedsContext
// cascade on top of the real (unknown-type) error.
func TestNoneSuppressesCascadeOnInvalidWant(t *testing.T) {
	info := checkSrc(t, wrapMain(`let x: Bogus = None`))
	for _, e := range info.Errors {
		if strings.Contains(e.Msg, "none requires an expected Optional type") {
			t.Fatalf("None added a cascade error on an already-invalid annotation: %v", info.Errors)
		}
	}
}

// --- Optional comparable-equality ---

func TestOptionalComparableEqAccepted(t *testing.T) {
	// int inner
	expectOK(t, wrapMain(`let a: Optional[int] = Some(1)`+"\n"+`let b: Optional[int] = Some(2)`+"\n"+`let r: bool = a == b`))
	// bool inner, with None
	expectOK(t, wrapMain(`let a: Optional[bool] = Some(true)`+"\n"+`let n: Optional[bool] = None`+"\n"+`let r: bool = a == n`))
	// string inner
	expectOK(t, wrapMain(`let a: Optional[string] = Some("x")`+"\n"+`let b: Optional[string] = Some("y")`+"\n"+`let r: bool = a != b`))
	// nested comparable Optional
	expectOK(t, wrapMain(`let a: Optional[Optional[int]] = Some(Some(1))`+"\n"+`let inner: Optional[int] = None`+"\n"+`let b: Optional[Optional[int]] = Some(inner)`+"\n"+`let r: bool = a == b`))
}

func TestOptionalNonComparableEqRejected(t *testing.T) {
	// float inner -> Optional message
	expectErr(t, wrapMain(`let a: Optional[float] = Some(1.0)`+"\n"+`let b: Optional[float] = Some(2.0)`+"\n"+`let r: bool = a == b`), "is not defined for Optional values")
	// error inner -> Optional message
	expectErr(t, wrapMain(`let a: Optional[error] = Some(error("x", 1))`+"\n"+`let b: Optional[error] = Some(error("y", 2))`+"\n"+`let r: bool = a == b`), "is not defined for Optional values")
	// array inner -> Optional message
	expectErr(t, wrapMain(`let a: Optional[int[]] = Some([1, 2])`+"\n"+`let b: Optional[int[]] = Some([3, 4])`+"\n"+`let r: bool = a == b`), "is not defined for Optional values")
}

func TestOptionalEqTypeMismatch(t *testing.T) {
	// mismatched Optional types -> Optional message (NOT same-type message)
	expectErr(t, wrapMain(`let a: Optional[int] = Some(1)`+"\n"+`let b: Optional[string] = Some("x")`+"\n"+`let r: bool = a == b`), "is not defined for Optional values")
	// Optional vs scalar -> Optional message
	expectErr(t, wrapMain(`let a: Optional[int] = Some(1)`+"\n"+`let r: bool = a == 1`), "is not defined for Optional values")
}

func TestResultEqStillRejected(t *testing.T) {
	// Result stays non-comparable (handled by isHandle guard, not Optional guard)
	expectErr(t, wrapMain(`let a: Result[int] = Ok(1)`+"\n"+`let b: Result[int] = Ok(2)`+"\n"+`let r: bool = a == b`), "aggregate handles are opaque")
}

func TestBareErrorEqStillRejected(t *testing.T) {
	expectErr(t, wrapMain(`let a: error = error("x", 1)`+"\n"+`let b: error = error("y", 2)`+"\n"+`let r: bool = a == b`), "aggregate handles are opaque")
}

func TestComparableBoundNotSatisfiedByOptional(t *testing.T) {
	expectErr(t,
		`fn eq[T: comparable](a: T, b: T) -> bool { return a == b }`+"\n"+
			`fn main() -> int {`+"\n"+
			`  let a: Optional[int] = Some(1)`+"\n"+
			`  let b: Optional[int] = Some(2)`+"\n"+
			`  let r: bool = eq(a, b)`+"\n"+
			`  return 0`+"\n"+
			`}`,
		"does not satisfy comparable")
}
