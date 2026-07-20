package types

import (
	"strings"
	"testing"
)

// --- AC 17: reserved names ---

func TestCombinatorNamesReserved(t *testing.T) {
	// Each name must be a redefinition error when used as a user function name.
	expectErr(t, "fn and_then(x: int) -> int { return x }\nfn main() -> int { return 0 }\n", "and_then")
	expectErr(t, "fn or_else(x: int) -> int { return x }\nfn main() -> int { return 0 }\n", "or_else")
	expectErr(t, "fn map_err(x: int) -> int { return x }\nfn main() -> int { return 0 }\n", "map_err")
}

// --- AC 12: arity errors ---

// map/filter are removable (array.map/array.filter); their overload-branch arity
// and non-collection-arg0 guards migrated to the namespaced form. and_then/or_else/
// map_err stay flat, so their arity guards remain bare here.
func TestCombinatorArityErrors(t *testing.T) {
	// and_then: one arg
	expectErr(t, wrapMain("let o: Optional[int] = Some(1)\nlet r: Optional[int] = and_then(o)"), "2")
	// and_then: three args (checker reports arity before funcref check)
	expectErr(t, "fn f(x: int) -> Optional[int] { return Some(x) }\nfn main() -> int {\nlet o: Optional[int] = Some(1)\nlet r: Optional[int] = and_then(o, f, o)\nreturn 0\n}\n", "2")
	// or_else: one arg
	expectErr(t, wrapMain("let o: Optional[int] = Some(1)\nlet r: Optional[int] = or_else(o)"), "2")
	// map_err: one arg
	expectErr(t, wrapMain("let r: Result[int] = Ok(1)\nlet r2: Result[int] = map_err(r)"), "2")
	// map: one arg (overload branch must also guard arity), namespaced
	wantNsErr(t, "array", "fn main() -> int {\nlet o: Optional[int] = Some(1)\nlet r: Optional[int] = array.map(o)\nreturn 0\n}\n", "2")
}

// --- AC 11c: map/filter over non-collection arg0 still errors via the array path ---

func TestMapFilterNonCollectionArgErrors(t *testing.T) {
	// map(int, f) -- arg0 is neither array nor Optional nor Result
	wantNsErr(t, "array", "fn f(x: int) -> int { return x }\nfn main() -> int { let r: int = array.map(1, f)\nreturn 0 }\n", "array")
	// filter(int, f) -- same
	wantNsErr(t, "array", "fn f(x: int) -> bool { return true }\nfn main() -> int { let r: bool = array.filter(1, f)\nreturn 0 }\n", "array")
}

// --- AC 11b: diagnostic message content ---

func TestCombinatorDiagnosticMessages(t *testing.T) {
	// and_then over Optional[int] with fn(int)->int (not Optional return): the
	// message must name the combinator AND the expected funcref shape (AC 11b).
	src := "fn f(x: int) -> int { return x }\nfn main() -> int {\nlet o: Optional[int] = Some(1)\nlet r: Optional[int] = and_then(o, f)\nreturn 0\n}\n"
	d := expectErr(t, src, "and_then")
	if !strings.Contains(d.Msg, "fn(int)->Optional") {
		t.Errorf("and_then mismatch: expected 'fn(int)->Optional' shape in diagnostic, got %q", d.Msg)
	}
	// or_else over Result[int] with a function taking the wrong param type
	// (fn(int)->Result instead of fn(error)->Result): the message must name or_else
	// and mention the expected error parameter.
	src2 := "fn g(x: int) -> Result[int] { return Ok(x) }\nfn main() -> int {\nlet r: Result[int] = Ok(1)\nlet r2: Result[int] = or_else(r, g)\nreturn 0\n}\n"
	d2 := expectErr(t, src2, "or_else")
	if !strings.Contains(d2.Msg, "error") {
		t.Errorf("or_else mismatch: expected 'error' (the expected param type) in diagnostic, got %q", d2.Msg)
	}
}

// --- AC 6: and_then return-type constraint ---

func TestAndThenReturnTypeMismatch(t *testing.T) {
	// Optional: f must return Optional[U], not a plain int
	expectErr(t, "fn f(x: int) -> int { return x }\nfn main() -> int {\nlet o: Optional[int] = Some(1)\nlet r: Optional[int] = and_then(o, f)\nreturn 0\n}\n", "and_then")
	// Optional: f must return Optional, not Result
	expectErr(t, "fn f(x: int) -> Result[int] { return Ok(x) }\nfn main() -> int {\nlet o: Optional[int] = Some(1)\nlet r: Optional[int] = and_then(o, f)\nreturn 0\n}\n", "and_then")
	// Result: f must return Result[U], not int
	expectErr(t, "fn f(x: int) -> int { return x }\nfn main() -> int {\nlet r: Result[int] = Ok(1)\nlet r2: Result[int] = and_then(r, f)\nreturn 0\n}\n", "and_then")
}

// --- AC 7: filter over Result is a compile error ---

// ERGO-6: this test's error source shifted from the "was moved to a
// module" hint to checkFilterCall's "use and_then" diagnostic (both
// contain "filter", so the substring assertion below still passes
// unmodified); see TestBareFilterResultUsesAndThenGuidance for the
// dedicated assertion on the new message.
func TestFilterOverResultError(t *testing.T) {
	expectErr(t, "fn f(x: int) -> bool { return true }\nfn main() -> int {\nlet r: Result[int] = Ok(1)\nlet r2: Result[int] = filter(r, f)\nreturn 0\n}\n", "filter")
}

// --- AC 10: map_err over Optional is a compile error ---

func TestMapErrOverOptionalError(t *testing.T) {
	expectErr(t, "fn f(e: error) -> error { return e }\nfn main() -> int {\nlet o: Optional[int] = Some(1)\nlet r: Optional[int] = map_err(o, f)\nreturn 0\n}\n", "map_err")
}

// --- AC 11: type mismatches in map ---

// map is removable (array.map); the Optional/Result overload mismatches migrated
// to the namespaced form.
func TestMapTypeMismatches(t *testing.T) {
	// map Optional: param type mismatch (fn(string)->int over Optional[int])
	wantNsErr(t, "array", "fn f(s: string) -> int { return 0 }\nfn main() -> int {\nlet o: Optional[int] = Some(1)\nlet r: Optional[int] = array.map(o, f)\nreturn 0\n}\n", "map")
	// map Optional: void return rejected
	wantNsErr(t, "array", "fn f(x: int) -> void {}\nfn main() -> int {\nlet o: Optional[int] = Some(1)\nlet r: Optional[int] = array.map(o, f)\nreturn 0\n}\n", "void")
	// map Result: param type mismatch
	wantNsErr(t, "array", "fn f(s: string) -> int { return 0 }\nfn main() -> int {\nlet r: Result[int] = Ok(1)\nlet r2: Result[int] = array.map(r, f)\nreturn 0\n}\n", "map")
	// map Result: void return rejected
	wantNsErr(t, "array", "fn f(x: int) -> void {}\nfn main() -> int {\nlet r: Result[int] = Ok(1)\nlet r2: Result[int] = array.map(r, f)\nreturn 0\n}\n", "void")
}

// --- AC 11: type mismatches in filter ---

// filter is removable (array.filter); the Optional overload mismatches migrated to
// the namespaced form.
func TestFilterTypeMismatches(t *testing.T) {
	// filter Optional: param type mismatch
	wantNsErr(t, "array", "fn f(s: string) -> bool { return true }\nfn main() -> int {\nlet o: Optional[int] = Some(1)\nlet r: Optional[int] = array.filter(o, f)\nreturn 0\n}\n", "filter")
	// filter Optional: non-bool return
	wantNsErr(t, "array", "fn f(x: int) -> int { return x }\nfn main() -> int {\nlet o: Optional[int] = Some(1)\nlet r: Optional[int] = array.filter(o, f)\nreturn 0\n}\n", "bool")
}

// --- AC 11: type mismatches in or_else ---

func TestOrElseTypeMismatches(t *testing.T) {
	// Optional: function must take ZERO params
	expectErr(t, "fn f(x: int) -> Optional[int] { return Some(x) }\nfn main() -> int {\nlet o: Optional[int] = Some(1)\nlet r: Optional[int] = or_else(o, f)\nreturn 0\n}\n", "or_else")
	// Optional: function must return Optional[T] with same T
	expectErr(t, "fn f() -> Optional[string] { return Some(\"x\") }\nfn main() -> int {\nlet o: Optional[int] = Some(1)\nlet r: Optional[int] = or_else(o, f)\nreturn 0\n}\n", "or_else")
	// Optional: function must return Optional (not int)
	expectErr(t, "fn f() -> int { return 0 }\nfn main() -> int {\nlet o: Optional[int] = Some(1)\nlet r: Optional[int] = or_else(o, f)\nreturn 0\n}\n", "or_else")
}

// --- AC 11a: Result-form mismatches in or_else and map_err ---

func TestOrElseResultTypeMismatches(t *testing.T) {
	// or_else Result: param must be error type (not int)
	expectErr(t, "fn f(x: int) -> Result[int] { return Ok(x) }\nfn main() -> int {\nlet r: Result[int] = Ok(1)\nlet r2: Result[int] = or_else(r, f)\nreturn 0\n}\n", "or_else")
	// or_else Result: return must be Result[T] (not Result[string] for Result[int])
	expectErr(t, "fn f(e: error) -> Result[string] { return Ok(\"x\") }\nfn main() -> int {\nlet r: Result[int] = Ok(1)\nlet r2: Result[int] = or_else(r, f)\nreturn 0\n}\n", "or_else")
	// or_else Result: return must be Result (not error)
	expectErr(t, "fn f(e: error) -> error { return e }\nfn main() -> int {\nlet r: Result[int] = Ok(1)\nlet r2: Result[int] = or_else(r, f)\nreturn 0\n}\n", "or_else")
}

func TestMapErrTypeMismatches(t *testing.T) {
	// map_err: param must be error (not int)
	expectErr(t, "fn f(x: int) -> error { return error(\"e\") }\nfn main() -> int {\nlet r: Result[int] = Ok(1)\nlet r2: Result[int] = map_err(r, f)\nreturn 0\n}\n", "map_err")
	// map_err: return must be error (not int)
	expectErr(t, "fn f(e: error) -> int { return 0 }\nfn main() -> int {\nlet r: Result[int] = Ok(1)\nlet r2: Result[int] = map_err(r, f)\nreturn 0\n}\n", "map_err")
}

// --- AC 20: every error carries a source position ---

func TestCombinatorErrorsPositioned(t *testing.T) {
	// One positioned diagnostic from EACH new error surface (AC 20). The stays-flat
	// combinators (and_then/or_else/map_err) remain bare.
	cases := []struct {
		name, src, want string
	}{
		{"and_then return-type", "fn f(x: int) -> int { return x }\nfn main() -> int {\nlet o: Optional[int] = Some(1)\nlet r: Optional[int] = and_then(o, f)\nreturn 0\n}\n", "and_then"},
		{"or_else Optional mismatch", "fn f() -> int { return 0 }\nfn main() -> int {\nlet o: Optional[int] = Some(1)\nlet r: Optional[int] = or_else(o, f)\nreturn 0\n}\n", "or_else"},
		{"or_else Result mismatch", "fn f(x: int) -> Result[int] { return Ok(x) }\nfn main() -> int {\nlet r: Result[int] = Ok(1)\nlet r2: Result[int] = or_else(r, f)\nreturn 0\n}\n", "or_else"},
		{"map_err mismatch", "fn f(e: error) -> int { return 0 }\nfn main() -> int {\nlet r: Result[int] = Ok(1)\nlet r2: Result[int] = map_err(r, f)\nreturn 0\n}\n", "map_err"},
	}
	for _, tc := range cases {
		d := expectErr(t, tc.src, tc.want)
		if d.Pos.Line == 0 {
			t.Errorf("%s: expected positioned diagnostic (non-zero line), got line 0 (msg %q)", tc.name, d.Msg)
		}
	}

	// map/filter are removable; their positioned diagnostics are asserted namespaced
	// (filter over Result, and the map/filter Optional element-type mismatches).
	nsCases := []struct {
		name, src, want string
	}{
		{"map arity", "fn main() -> int {\nlet o: Optional[int] = Some(1)\nlet r: Optional[int] = array.map(o)\nreturn 0\n}\n", "2"},
		{"filter over Result", "fn f(x: int) -> bool { return true }\nfn main() -> int {\nlet r: Result[int] = Ok(1)\nlet r2: Result[int] = array.filter(r, f)\nreturn 0\n}\n", "filter"},
		{"map Optional element-type mismatch", "fn f(s: string) -> int { return 0 }\nfn main() -> int {\nlet o: Optional[int] = Some(1)\nlet r: Optional[int] = array.map(o, f)\nreturn 0\n}\n", "map"},
		{"filter Optional element-type mismatch", "fn f(s: string) -> bool { return true }\nfn main() -> int {\nlet o: Optional[int] = Some(1)\nlet r: Optional[int] = array.filter(o, f)\nreturn 0\n}\n", "filter"},
	}
	for _, tc := range nsCases {
		d := firstNsErr(t, "array", tc.src, tc.want)
		if d.Pos.Line == 0 {
			t.Errorf("%s: expected positioned diagnostic (non-zero line), got line 0 (msg %q)", tc.name, d.Msg)
		}
	}
}

// --- AC 9: or_else Optional type-checks OK on the success path ---

func TestOrElseOptionalTypeChecks(t *testing.T) {
	expectOK(t, "fn fallback() -> Optional[int] { return Some(0) }\nfn main() -> int {\nlet o: Optional[int] = Some(1)\nlet r: Optional[int] = or_else(o, fallback)\nreturn 0\n}\n")
}

// --- AC 5: and_then type-checks OK ---

func TestAndThenTypeChecks(t *testing.T) {
	expectOK(t, "fn safe_div(x: int) -> Optional[int] {\nif (x == 0) { return None }\nreturn Some(100 / x)\n}\nfn main() -> int {\nlet o: Optional[int] = Some(5)\nlet r: Optional[int] = and_then(o, safe_div)\nreturn 0\n}\n")
	// Result form
	expectOK(t, "fn step(x: int) -> Result[int] { return Ok(x * 2) }\nfn main() -> int {\nlet r: Result[int] = Ok(3)\nlet r2: Result[int] = and_then(r, step)\nreturn 0\n}\n")
}

// --- map over Optional/Result type-checks OK ---

func TestMapOptResultTypeChecks(t *testing.T) {
	wantNsOK(t, "array", "fn dbl(x: int) -> int { return x * 2 }\nfn main() -> int {\nlet o: Optional[int] = Some(5)\nlet r: Optional[int] = array.map(o, dbl)\nreturn 0\n}\n")
	wantNsOK(t, "array", "fn dbl(x: int) -> int { return x * 2 }\nfn main() -> int {\nlet r: Result[int] = Ok(5)\nlet r2: Result[int] = array.map(r, dbl)\nreturn 0\n}\n")
	// map changes element type
	wantNsOK(t, "array", "fn tostr(x: int) -> string { return to_string(x) }\nfn main() -> int {\nlet o: Optional[int] = Some(5)\nlet r: Optional[string] = array.map(o, tostr)\nreturn 0\n}\n")
}

// --- filter over Optional type-checks OK ---

func TestFilterOptionalTypeChecks(t *testing.T) {
	wantNsOK(t, "array", "fn pos(x: int) -> bool { return x > 0 }\nfn main() -> int {\nlet o: Optional[int] = Some(5)\nlet r: Optional[int] = array.filter(o, pos)\nreturn 0\n}\n")
}

// --- map_err type-checks OK ---

func TestMapErrTypeChecks(t *testing.T) {
	expectOK(t, "fn prefix(e: error) -> error { return error(\"wrapped\") }\nfn main() -> int {\nlet r: Result[int] = Ok(1)\nlet r2: Result[int] = map_err(r, prefix)\nreturn 0\n}\n")
}

// --- ERGO-6: bare map/filter over Optional/Result ---

func TestBareMapOptionalTypeChecks(t *testing.T) {
	expectOK(t, "fn tostr(x: int) -> string { return to_string(x) }\nfn main() -> int {\nlet o: Optional[int] = Some(5)\nlet r: Optional[string] = map(o, tostr)\nreturn 0\n}\n")
	expectErr(t, "fn tostr(x: int) -> string { return to_string(x) }\nfn main() -> int {\nlet o: Optional[int] = Some(5)\nlet r: Optional[int] = map(o, tostr)\nreturn 0\n}\n", "")
}

func TestBareMapResultTypeChecks(t *testing.T) {
	expectOK(t, "fn tostr(x: int) -> string { return to_string(x) }\nfn main() -> int {\nlet r: Result[int] = Ok(5)\nlet r2: Result[string] = map(r, tostr)\nreturn 0\n}\n")
}

func TestBareFilterOptionalTypeChecks(t *testing.T) {
	expectOK(t, "fn pos(x: int) -> bool { return x > 0 }\nfn main() -> int {\nlet o: Optional[int] = Some(5)\nlet r: Optional[int] = filter(o, pos)\nreturn 0\n}\n")
}

func TestBareFilterResultUsesAndThenGuidance(t *testing.T) {
	d := expectErr(t, "fn f(x: int) -> bool { return true }\nfn main() -> int {\nlet r: Result[int] = Ok(1)\nlet r2: Result[int] = filter(r, f)\nreturn 0\n}\n", "filter")
	if !strings.Contains(d.Msg, "use and_then") {
		t.Errorf("expected 'use and_then' guidance, got %q", d.Msg)
	}
	if strings.Contains(d.Msg, "was moved to a module") {
		t.Errorf("expected the and_then diagnostic, not the module hint, got %q", d.Msg)
	}
}

func TestBareMapArrayStillHintsModule(t *testing.T) {
	d := expectErr(t, "fn f(x: int) -> int { return x }\nfn main() -> int {\nlet xs: int[] = [1, 2, 3]\nlet ys: int[] = map(xs, f)\nreturn 0\n}\n", "map")
	if !strings.Contains(d.Msg, "was moved to a module") {
		t.Errorf("expected the module-hint diagnostic for array arg0, got %q", d.Msg)
	}
}

func TestBareFilterArrayStillHintsModule(t *testing.T) {
	d := expectErr(t, "fn p(x: int) -> bool { return true }\nfn main() -> int {\nlet xs: int[] = [1, 2, 3]\nlet ys: int[] = filter(xs, p)\nreturn 0\n}\n", "filter")
	if !strings.Contains(d.Msg, "was moved to a module") {
		t.Errorf("expected the module-hint diagnostic for array arg0, got %q", d.Msg)
	}
}

func TestBareMapFilterWrongArityStillHintsModule(t *testing.T) {
	cases := []struct{ src, name string }{
		{wrapMain("let r: int[] = map()"), "map"},
		{wrapMain("let r: int[] = map(5)"), "map"},
		{wrapMain("let r: bool[] = filter()"), "filter"},
	}
	for _, tc := range cases {
		d := expectErr(t, tc.src, tc.name)
		if !strings.Contains(d.Msg, "was moved to a module") {
			t.Errorf("expected the module-hint diagnostic for wrong-arity bare call, got %q", d.Msg)
		}
	}
}
