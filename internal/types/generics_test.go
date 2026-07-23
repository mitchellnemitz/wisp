package types

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/parser"
)

// containsErr reports whether any error message in info contains substr.
func containsErr(info *Info, substr string) bool {
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, substr) {
			return true
		}
	}
	return false
}

// --- Task 2: type-parameter scope ---

// type parameters resolve where annotations appear, including nested.
func TestGenericSignatureResolves(t *testing.T) {
	expectOK(t, "fn id[T](x: T) -> T { return x }\n"+wrapMain("return 0"))
	expectOK(t, "fn head[T](xs: T[]) -> Optional[T] { return None }\n"+wrapMain("return 0"))
	expectOK(t, "fn kv[T](d: {string: T}) -> T { return d[\"k\"] }\n"+wrapMain("return 0"))
	expectOK(t, "fn apply[T, U](x: T, f: fn(T) -> U) -> U { return f(x) }\n"+wrapMain("return 0"))
}

// a type parameter used in a let annotation inside the body resolves.
func TestGenericLetAnnotation(t *testing.T) {
	expectOK(t, "fn id[T](x: T) -> T { let y: T = x\n return y }\n"+wrapMain("return 0"))
}

// out of scope: T referenced outside its declaring function is unknown-type.
func TestGenericTypeParamOutOfScope(t *testing.T) {
	expectErr(t, "fn id[T](x: T) -> T { return x }\nfn g(y: T) -> T { return y }\n"+wrapMain("return 0"), "unknown type")
}

// struct-name collision (decided in the checker).
func TestGenericTypeParamShadowsStruct(t *testing.T) {
	expectErr(t, "struct T { x: int }\nfn f[T](v: T) -> T { return v }\n"+wrapMain("return 0"), "collides")
}

// COLLISION-SAFETY (spec 8 guarantee a): a struct named T (in a DIFFERENT
// function with no type-param T) resolves to a Type string DISTINCT from the
// type variable $T.
func TestGenericTypeVarDistinctFromStruct(t *testing.T) {
	if typeVarType("T") == internalStructName("T", 0) {
		t.Fatalf("type variable %q collides with struct token %q", typeVarType("T"), internalStructName("T", 0))
	}
	if isTypeVar(internalStructName("T", 0)) {
		t.Fatalf("struct token %q misidentified as a type variable", internalStructName("T", 0))
	}
	expectOK(t, "struct T { x: int }\nfn id[U](v: U) -> U { return v }\n"+
		wrapMain("let p: T = T { x: 1 }\n return p.x"))
}

// --- Task 3: unification at call sites ---

// inference: identity instantiates to the argument type.
func TestGenericInferIdentity(t *testing.T) {
	info := expectOK(t, "fn id[T](x: T) -> T { return x }\n"+
		wrapMain("let a: int = id(5)\n let s: string = id(\"hi\")"))
	_ = info
}

// inference through composites and funcref.
func TestGenericInferStructural(t *testing.T) {
	expectOK(t, "fn head[T](xs: T[]) -> Optional[T] { return None }\n"+
		wrapMain("let o: Optional[int] = head([1, 2, 3])"))
	expectOK(t, "fn apply[T, U](x: T, f: fn(T) -> U) -> U { return f(x) }\n"+
		"fn to_s(n: int) -> string { return to_string(n) }\n"+
		wrapMain("let r: string = apply(3, to_s)"))
}

// CALL-RESULT substitution (spec 8): the recorded result type of an
// instantiated call is concrete, never a type variable.
func TestGenericCallResultConcrete(t *testing.T) {
	src := "fn id[T](x: T) -> T { return x }\n" +
		"fn head[T](xs: T[]) -> Optional[T] { return None }\n" +
		wrapMain("let a: int = id(5)\n let o: Optional[string] = head([\"hi\"])")
	info := check(t, src)
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %s", diagList(info.Errors))
	}
	for n, ci := range info.Calls {
		if ci.Kind == CallUser && (n.CalleeName == "id" || n.CalleeName == "head") {
			if isTypeVar(ci.Result) {
				t.Errorf("call %s result is a type variable %s, want concrete", n.CalleeName, ci.Result)
			}
			if isTypeVar(info.Types[n]) {
				t.Errorf("call %s recorded type is a type variable %s", n.CalleeName, info.Types[n])
			}
		}
	}
}

// conflict: T bound to int then string -> ONE diagnostic naming the param AND
// both concrete types.
func TestGenericConflict(t *testing.T) {
	d := expectErr(t, "fn pair[T](a: T, b: T) -> T { return a }\n"+
		wrapMain("let x: int = pair(1, \"two\")"), "T")
	if d.Pos.Line == 0 {
		t.Errorf("conflict diagnostic missing position")
	}
	if !strings.Contains(d.Msg, "int") || !strings.Contains(d.Msg, "string") {
		t.Errorf("conflict diagnostic %q must name both int and string", d.Msg)
	}
}

// structural mismatch: T[] param against a non-array actual.
func TestGenericStructuralMismatch(t *testing.T) {
	expectErr(t, "fn head[T](xs: T[]) -> T { return xs[0] }\n"+
		wrapMain("let x: int = head(5)"), "")
}

// ROLLBACK: arg 1 partially binds then fails; the partial binding must not leak.
func TestGenericUnifyRollback(t *testing.T) {
	src := "fn g[T, U](f: fn(T, U) -> T, x: T) -> T { return x }\n" +
		"fn bad(a: int, b: string) -> bool { return true }\n" +
		wrapMain("let r: string = g(bad, \"s\")")
	info := check(t, src)
	if len(info.Errors) != 1 {
		t.Fatalf("expected exactly 1 error (arg 1 mismatch), got %d:\n%s", len(info.Errors), diagList(info.Errors))
	}
}

// cannot infer: return-only parameter (single diagnostic).
func TestGenericCannotInferReturnOnly(t *testing.T) {
	info := check(t, "fn make[T]() -> T[] { return [] }\n"+
		wrapMain("let xs: int[] = make()"))
	if !containsErr(info, "cannot infer") {
		t.Fatalf("want 'cannot infer', got:\n%s", diagList(info.Errors))
	}
	if len(info.Errors) != 1 {
		t.Fatalf("expected exactly 1 error, got %d:\n%s", len(info.Errors), diagList(info.Errors))
	}
}

// cannot infer + NO cascade: a [] argument already errored in checkArrayLit.
func TestGenericCannotInferEmptyLiteralSingleDiag(t *testing.T) {
	info := check(t, "fn head[T](xs: T[]) -> Optional[T] { return None }\n"+
		wrapMain("let o: Optional[int] = head([])"))
	if len(info.Errors) != 1 {
		t.Fatalf("expected exactly 1 error (the empty-literal error), got %d:\n%s", len(info.Errors), diagList(info.Errors))
	}
}

// cannot infer + NO cascade: bare None already errored via noneNeedsContext.
func TestGenericNoneSingleDiag(t *testing.T) {
	info := check(t, "fn unwrap_first[T](o: Optional[T]) -> T { return unwrap(o) }\n"+
		wrapMain("let x: int = unwrap_first(None)"))
	if len(info.Errors) != 1 {
		t.Fatalf("expected exactly 1 error (the None error), got %d:\n%s", len(info.Errors), diagList(info.Errors))
	}
}

// non-cascading: a conflict yields exactly one error.
func TestGenericConflictNonCascading(t *testing.T) {
	info := check(t, "fn pair[T](a: T, b: T) -> T { return a }\n"+
		wrapMain("let x: int = pair(1, \"two\")"))
	if len(info.Errors) != 1 {
		t.Fatalf("expected exactly 1 error, got %d:\n%s", len(info.Errors), diagList(info.Errors))
	}
}

// NO literal '$' in any diagnostic.
func TestGenericDiagnosticsHaveNoDollar(t *testing.T) {
	for _, src := range []string{
		"fn head[T](xs: T[]) -> T { return xs[0] }\n" + wrapMain("let x: int = head(5)"),
		"fn pair[T](a: T, b: T) -> T { return a }\n" + wrapMain("let x: int = pair(1, \"two\")"),
	} {
		info := check(t, src)
		for _, d := range info.Errors {
			if strings.Contains(d.Msg, "$") {
				t.Errorf("diagnostic leaks a type-variable encoding: %q", d.Msg)
			}
		}
	}
}

// --- Task 4: bare-T guards ---

func TestGenericBareTGuards(t *testing.T) {
	pre := "fn f[T](x: T, y: T) -> T {\n"
	post := "\n return x\n}\n" + wrapMain("return 0")
	cases := []struct{ body, want string }{
		{"let z: T = x + y", "type parameter T"},
		{"let z: T = x - y", "type parameter T"},
		{"let z: T = x * y", "type parameter T"},
		{"let z: T = x / y", "type parameter T"},
		{"let z: T = x % y", "type parameter T"},
		{"let b: bool = x < y", "type parameter T"},
		{"let b: bool = x <= y", "type parameter T"},
		{"let b: bool = x > y", "type parameter T"},
		{"let b: bool = x >= y", "type parameter T"},
		{"let b: bool = x == y", "type parameter T"},
		{"let b: bool = x != y", "type parameter T"},
		{"let s: string = \"v=${x}\"", "type parameter T"},
		{"let n: int = x.field", "type parameter T"},
		{"switch (x) { case 1 { } default { } }", "type parameter T"},
	}
	for _, tc := range cases {
		expectErr(t, pre+tc.body+post, tc.want)
	}
}

func TestGenericBareTUnary(t *testing.T) {
	expectErr(t, "fn f[T](x: T) -> T { let z: T = -x\n return z }\n"+wrapMain("return 0"), "type parameter T")
}

// sort/sum are removable (array.sort/array.sum); the bare-T guard is asserted on
// the namespaced form, which CheckLinked resolves via the array module (helper
// wantNsErr lives in core_collections_neg_test.go).
func TestGenericBareTSortSum(t *testing.T) {
	wantNsErr(t, "array", "fn f[T](xs: T[]) -> int { let s: T[] = array.sort(xs)\n return 0 }\n"+wrapMain("return 0"), "type parameter T")
	wantNsErr(t, "array", "fn f[T](xs: T[]) -> int { let s: T = array.sum(xs)\n return 0 }\n"+wrapMain("return 0"), "type parameter T")
}

func TestGenericFuncrefValue(t *testing.T) {
	expectErr(t, "fn id[T](x: T) -> T { return x }\n"+
		wrapMain("let f: fn(int) -> int = id\n return 0"), "generic")
}

func TestGenericErasureSafe(t *testing.T) {
	expectOK(t, "fn id[T](x: T) -> T {\n"+
		"  let y: T = x\n"+
		"  return y\n"+
		"}\n"+wrapMain("return 0"))
	// push is removable (array.push); a T[] literal exercises the same erasure-safe
	// generic array construction without the removed bare builtin.
	expectOK(t, "fn box[T](x: T) -> T[] {\n"+
		"  let xs: T[] = [x]\n"+
		"  return xs\n"+
		"}\n"+wrapMain("return 0"))
	expectOK(t, "fn idx[T](xs: T[]) -> T { return xs[0] }\n"+wrapMain("return 0"))
	expectOK(t, "fn wrap_it[T](x: T) -> Optional[T] { return Some(x) }\n"+wrapMain("return 0"))
	expectOK(t, "fn head[T](o: Optional[T]) -> T { return unwrap(o) }\n"+wrapMain("return 0"))
	expectOK(t, "fn lng[T](xs: T[]) -> int { return length(xs) }\n"+wrapMain("return 0"))
	expectOK(t, "fn callit[T, U](x: T, f: fn(T) -> U) -> U { return f(x) }\n"+wrapMain("return 0"))
}

func TestGenericBuiltinsRejectBareT(t *testing.T) {
	// to_string stays flat; min/contains are removable (math.min/string.contains) and
	// are asserted namespaced so the bare-T rejection is exercised (not masked by the
	// moved-to-module note the bare spelling would now emit).
	expectErr(t, "fn f[T](x: T) -> string { return to_string(x) }\n"+wrapMain("return 0"), "")
	wantNsErr(t, "math", "fn f[T](x: T) -> T { return math.min(x, x) }\n"+wrapMain("return 0"), "")
	wantNsErr(t, "string", "fn f[T](xs: T[], x: T) -> bool { return string.contains(xs, x) }\n"+wrapMain("return 0"), "")
}

// --- Copilot-review regressions ---

// A type parameter in the reserved "__" namespace is rejected (consistent with
// structs/functions/params/vars).
func TestGenericTypeParamReservedNamespace(t *testing.T) {
	expectErr(t, "fn f[__T](x: __T) -> __T { return x }\n"+wrapMain("return 0"), "reserved")
}

// An arity mismatch on a GENERIC call returns Invalid (not the un-substituted
// return type), so no type variable surfaces in a diagnostic and there is no
// "$T"-typed cascade. (Body nodes legitimately carry $T; the concern is the
// call-site result and the user-visible messages, not the generic body.)
func TestGenericArityMismatchNoTypeVarLeak(t *testing.T) {
	info := checkSrc(t, "fn id[T](x: T) -> T { return x }\n"+wrapMain("let a: int = id(1, 2)"))
	if !containsErr(info, "expects") {
		t.Fatalf("want an arity error, got: %v", info.Errors)
	}
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "$") {
			t.Fatalf("diagnostic leaks a type variable: %q", d.Msg)
		}
	}
	// The call result is Invalid, so the `let a: int = id(1,2)` assignment does
	// NOT add a second "$T does not match int" cascade error: exactly one error.
	if len(info.Errors) != 1 {
		t.Fatalf("want exactly one (arity) error, got %d: %v", len(info.Errors), info.Errors)
	}
}

// The bare-T guard message reads grammatically (no dangling "of").
func TestGenericBareTMessageGrammar(t *testing.T) {
	info := checkSrc(t, "fn f[T](x: T) -> string { return \"${x}\" }\n"+wrapMain("return 0"))
	found := false
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "string interpolation is not allowed on type parameter T") {
			found = true
		}
		if strings.Contains(d.Msg, "of is not") {
			t.Fatalf("message has a dangling 'of': %q", d.Msg)
		}
	}
	if !found {
		t.Fatalf("want a grammatical interpolation guard message, got: %v", info.Errors)
	}
}

func TestRejectTypeVarUnboundedMessageDropsMilestoneLanguage(t *testing.T) {
	info := checkSrc(t, "fn f[T](x: T) -> string { return \"${x}\" }\n"+wrapMain("return 0"))
	found := false
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "later slice") {
			t.Fatalf("message still leaks milestone language: %q", d.Msg)
		}
		if strings.Contains(d.Msg, "is not allowed on type parameter") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want an unbounded type-parameter rejection message, got: %v", info.Errors)
	}
}

func TestRejectTypeVarBoundedMessageDropsMilestoneLanguage(t *testing.T) {
	info := checkSrc(t, "fn f[T: comparable](a: T, b: T) -> bool { return a < b }\n"+wrapMain("return 0"))
	found := false
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "monomorphization milestone") {
			t.Fatalf("message still leaks milestone language: %q", d.Msg)
		}
		if strings.Contains(d.Msg, "is not yet supported for a bounded type parameter") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want a bounded type-parameter rejection message, got: %v", info.Errors)
	}
}

// --- Slice 2: comparable bound ---

func TestComparableUnlocksEquality(t *testing.T) {
	expectOK(t, "fn eq[T: comparable](a: T, b: T) -> bool { return a == b }\n"+wrapMain("return 0"))
	expectOK(t, "fn ne[T: comparable](a: T, b: T) -> bool { return a != b }\n"+wrapMain("return 0"))
}

func TestUnboundedStillRejectsEquality(t *testing.T) {
	expectErr(t, "fn eq[T](a: T, b: T) -> bool { return a == b }\n"+wrapMain("return 0"), "type parameter")
}

func TestComparableStillBarsOtherOps(t *testing.T) {
	expectErr(t, "fn f[T: comparable](a: T, b: T) -> bool { return a < b }\n"+wrapMain("return 0"), "type parameter")
	expectErr(t, "fn f[T: comparable](a: T, b: T) -> T { return a + b }\n"+wrapMain("return 0"), "type parameter")
	expectErr(t, "fn f[T: comparable](a: T) -> string { return \"${a}\" }\n"+wrapMain("return 0"), "type parameter")
	expectErr(t, "struct P { x: int }\nfn f[T: comparable](a: T) -> int { return a.x }\n"+wrapMain("return 0"), "type parameter")
	expectErr(t, "fn f[T: comparable](a: T) -> int { switch (a) { default { } }\n return 0 }\n"+wrapMain("return 0"), "type parameter")
	// sort/sum are removable (array.sort/array.sum); the bounded bare-T guard is
	// asserted on the namespaced form.
	wantNsErr(t, "array", "fn f[T: comparable](xs: T[]) -> T[] { return array.sort(xs) }\n"+wrapMain("return 0"), "type parameter")
	wantNsErr(t, "array", "fn f[T: comparable](xs: T[]) -> T { return array.sum(xs) }\n"+wrapMain("return 0"), "type parameter")
}

func TestBoundedOpMessageDiffers(t *testing.T) {
	info := checkSrc(t, "fn f[T: comparable](a: T, b: T) -> bool { return a < b }\n"+wrapMain("return 0"))
	if !containsErr(info, "not yet supported for a bounded type parameter") {
		t.Fatalf("want bounded-specific ordered message, got %v", info.Errors)
	}
}

// --- Slice 2: bound satisfaction at call sites ---

func TestComparableBoundSatisfaction(t *testing.T) {
	base := "fn eq[T: comparable](a: T, b: T) -> bool { return a == b }\n"
	expectOK(t, base+wrapMain("let r: bool = eq(1, 2)\nreturn 0"))
	expectOK(t, base+wrapMain("let r: bool = eq(true, false)\nreturn 0"))
	expectOK(t, base+wrapMain("let r: bool = eq(\"x\", \"y\")\nreturn 0"))
}

func TestComparableSatisfactionRejectsNonComparable(t *testing.T) {
	eqf := "fn eq[T: comparable](a: T, b: T) -> bool { return a == b }\n"
	// float now satisfies comparable (uniform scalar comparability); the still-non-
	// comparable handle types below stay rejected.
	expectErr(t, "struct P { x: int }\n"+eqf+wrapMain("let p: P = P { x: 1 }\nlet r: bool = eq(p, p)\nreturn 0"), "does not satisfy comparable")
	expectErr(t, eqf+wrapMain("let xs: int[] = [1]\nlet r: bool = eq(xs, xs)\nreturn 0"), "does not satisfy comparable")
	expectErr(t, eqf+wrapMain("let d: {string: int} = {\"a\": 1}\nlet r: bool = eq(d, d)\nreturn 0"), "does not satisfy comparable")
	expectErr(t, eqf+wrapMain("let o: Optional[int] = Some(1)\nlet r: bool = eq(o, o)\nreturn 0"), "does not satisfy comparable")
	expectErr(t, eqf+wrapMain("let e: error = error(\"x\")\nlet r: bool = eq(e, e)\nreturn 0"), "does not satisfy comparable")
	// funcref: a function reference is not comparable.
	expectErr(t, "fn inc(x: int) -> int { return x + 1 }\n"+eqf+
		wrapMain("let f: fn(int) -> int = inc\nlet r: bool = eq(f, f)\nreturn 0"), "does not satisfy comparable")
}

// A comparable-bounded generic called from INSIDE another generic: the caller's
// bounded type variable satisfies the callee's comparable bound (bound
// propagation), so composition type-checks; an UNBOUNDED caller does not.
func TestComparableBoundPropagation(t *testing.T) {
	eqf := "fn eq[T: comparable](a: T, b: T) -> bool { return a == b }\n"
	// caller U is comparable -> eq(a,a) with a: U is accepted.
	expectOK(t, eqf+"fn outer[U: comparable](a: U) -> bool { return eq(a, a) }\n"+wrapMain("return 0"))
	// caller U is unbounded -> U does not satisfy comparable, rejected; the message
	// shows "type parameter U", never the "$U" encoding.
	d := expectErr(t, eqf+"fn outer[U](a: U) -> bool { return eq(a, a) }\n"+wrapMain("return 0"),
		"does not satisfy comparable")
	if strings.Contains(d.Msg, "$") {
		t.Fatalf("message leaked the $U encoding: %q", d.Msg)
	}
}

func TestComparableSatisfactionBlamesBinder(t *testing.T) {
	// float now satisfies comparable, so blame a non-comparable struct binder.
	d := expectErr(t, "struct P { x: int }\nfn eq[T: comparable](a: T, b: T) -> bool { return a == b }\n"+
		wrapMain("let p: P = P { x: 1 }\nlet r: bool = eq(p, p)\nreturn 0"), "does not satisfy comparable")
	// The diagnostic is positioned at the first argument (p), not the callee.
	// With the struct decl (line 1) and eq (line 2), main opens at line 3, `let p`
	// is line 4, and `let r: bool = eq(p, p)` is line 5; the callee `eq` starts at
	// col 15, so the first argument `p` starts at col 18.
	if d.Pos.Line != 5 || d.Pos.Col != 18 {
		t.Fatalf("want blame at first arg (line 5 col 18), got %s", d.Pos)
	}
}

func TestComparableSatisfactionNoCascade(t *testing.T) {
	info := checkSrc(t, "fn eq[T: comparable](a: T, b: T) -> bool { return a == b }\n"+
		wrapMain("let r: bool = eq(nope, nope)\nreturn 0"))
	if containsErr(info, "does not satisfy comparable") {
		t.Fatalf("bound error cascaded over an already-invalid argument: %v", info.Errors)
	}
}

// When one argument binds a comparable T to a non-comparable type AND another
// argument independently errors, the call is already Invalid, so the bound
// sweep is skipped: no second "does not satisfy comparable" piles on the primary
// error.
func TestComparableSatisfactionNoCascadeWithBadBinding(t *testing.T) {
	info := checkSrc(t, "fn eq[T: comparable](a: T, b: T) -> bool { return a == b }\n"+
		wrapMain("let r: bool = eq(1.0, nope)\nreturn 0"))
	// `nope` is undefined -> primary error; the float binding's bound violation
	// must NOT add a second diagnostic.
	if containsErr(info, "does not satisfy comparable") {
		t.Fatalf("bound error cascaded on top of an errored argument: %v", info.Errors)
	}
}

// Deeper cascade case: an errored argument that does NOT mention the bounded
// param (so suppressed[T] is not set and ret is not Invalid) must still suppress
// the bound sweep, because the call is fundamentally broken.
func TestComparableSatisfactionNoCascadeUnrelatedErroredArg(t *testing.T) {
	info := checkSrc(t, "fn f[T: comparable, U](a: T, b: U) -> bool { return true }\n"+
		wrapMain("let r: bool = f(1.0, nope)\nreturn 0"))
	if containsErr(info, "does not satisfy comparable") {
		t.Fatalf("bound error cascaded though an unrelated argument errored: %v", info.Errors)
	}
}

// --- Slice 3: numeric bound ---

func TestNumericBoundAccepted(t *testing.T) {
	expectOK(t, "fn add[T: numeric](a: T, b: T) -> T { return a + b }\n"+wrapMain("return 0"))
	expectOK(t, "fn scale[T: numeric](a: T, b: T) -> T { return a * b }\n"+wrapMain("return 0"))
	expectOK(t, "fn neg[T: numeric](a: T) -> T { return -a }\n"+wrapMain("return 0"))
}

func TestNumericBoundArithmeticOps(t *testing.T) {
	pre := "fn f[T: numeric](a: T, b: T) -> T {\n"
	post := "\n return a\n}\n" + wrapMain("return 0")
	for _, body := range []string{
		"let z: T = a + b",
		"let z: T = a - b",
		"let z: T = a * b",
		"let z: T = a / b",
	} {
		expectOK(t, pre+body+post)
	}
}

func TestNumericBoundModuloRejected(t *testing.T) {
	info := checkSrc(t, "fn f[T: numeric](a: T, b: T) -> T { let z: T = a % b\n return a }\n"+wrapMain("return 0"))
	if !containsErr(info, "not allowed on numeric type parameter") {
		t.Fatalf("want modulo-on-numeric error, got: %v", info.Errors)
	}
}

func TestNumericBoundComparisonOps(t *testing.T) {
	pre := "fn f[T: numeric](a: T, b: T) -> bool {\n"
	post := "\n}\n" + wrapMain("return 0")
	for _, body := range []string{
		"return a < b",
		"return a <= b",
		"return a > b",
		"return a >= b",
	} {
		expectOK(t, pre+body+post)
	}
}

func TestNumericBoundEqNeqAllowed(t *testing.T) {
	expectOK(t, "fn eq[T: numeric](a: T, b: T) -> bool { return a == b }\n"+wrapMain("return 0"))
	expectOK(t, "fn ne[T: numeric](a: T, b: T) -> bool { return a != b }\n"+wrapMain("return 0"))
}

func TestNumericBoundUnaryMinus(t *testing.T) {
	expectOK(t, "fn neg[T: numeric](a: T) -> T { return -a }\n"+wrapMain("return 0"))
}

func TestNumericBoundStillBarsOtherOps(t *testing.T) {
	expectErr(t, "fn f[T: numeric](a: T) -> string { return \"${a}\" }\n"+wrapMain("return 0"), "type parameter")
	expectErr(t, "struct P { x: int }\nfn f[T: numeric](a: T) -> int { return a.x }\n"+wrapMain("return 0"), "type parameter")
	// sort/sum are removable (array.sort/array.sum); the bounded bare-T guard is
	// asserted on the namespaced form.
	wantNsErr(t, "array", "fn f[T: numeric](xs: T[]) -> T[] { return array.sort(xs) }\n"+wrapMain("return 0"), "type parameter")
	wantNsErr(t, "array", "fn f[T: numeric](xs: T[]) -> T { return array.sum(xs) }\n"+wrapMain("return 0"), "type parameter")
}

func TestNumericBoundSatisfactionInt(t *testing.T) {
	base := "fn add[T: numeric](a: T, b: T) -> T { return a + b }\n"
	expectOK(t, base+wrapMain("let r: int = add(1, 2)\nreturn 0"))
}

func TestNumericBoundSatisfactionFloat(t *testing.T) {
	base := "fn add[T: numeric](a: T, b: T) -> T { return a + b }\n"
	expectOK(t, base+wrapMain("let r: float = add(1.0, 2.0)\nreturn 0"))
}

func TestNumericBoundSatisfactionRejected(t *testing.T) {
	base := "fn add[T: numeric](a: T, b: T) -> T { return a + b }\n"
	expectErr(t, base+wrapMain("let r: bool = add(true, false)\nreturn 0"), "does not satisfy numeric")
	expectErr(t, base+wrapMain("let r: string = add(\"x\", \"y\")\nreturn 0"), "does not satisfy numeric")
}

// A numeric-bounded generic called from inside another numeric-bounded generic:
// the caller's T satisfies the callee's numeric bound (bound propagation).
// An unbounded caller does NOT propagate and is rejected.
func TestNumericBoundPropagation(t *testing.T) {
	addf := "fn add[T: numeric](a: T, b: T) -> T { return a + b }\n"
	expectOK(t, addf+"fn double[U: numeric](a: U) -> U { return add(a, a) }\n"+wrapMain("return 0"))
	d := expectErr(t, addf+"fn double[U](a: U) -> U { return add(a, a) }\n"+wrapMain("return 0"),
		"does not satisfy numeric")
	if strings.Contains(d.Msg, "$") {
		t.Fatalf("message leaked the $U encoding: %q", d.Msg)
	}
}

// TypeSubst is populated with the concrete binding for a numeric-bounded call.
func TestNumericBoundTypeSubstPopulated(t *testing.T) {
	src := "fn add[T: numeric](a: T, b: T) -> T { return a + b }\n" +
		wrapMain("let i: int = add(1, 2)\nlet f: float = add(1.0, 2.0)\nreturn 0")
	info := expectOK(t, src)
	var saw int
	for n, ci := range info.Calls {
		if ci.Kind != CallUser || n.CalleeName != "add" {
			continue
		}
		if ci.TypeSubst == nil {
			t.Fatalf("TypeSubst is nil for call to add")
		}
		ct, ok := ci.TypeSubst["T"]
		if !ok {
			t.Fatalf("TypeSubst missing key 'T' for call to add")
		}
		if ct != Int && ct != Float {
			t.Fatalf("TypeSubst[T] = %q, want int or float", ct)
		}
		saw++
	}
	if saw != 2 {
		t.Fatalf("expected 2 add calls with TypeSubst, got %d", saw)
	}
}

// expectParseErr asserts that src fails to parse and the error message contains want.
func expectParseErr(t *testing.T, src, want string) {
	t.Helper()
	_, err := parser.Parse(src, "test.wisp")
	if err == nil {
		t.Fatalf("expected parse error containing %q, but parse succeeded\nsrc:\n%s", want, src)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected parse error containing %q, got %q\nsrc:\n%s", want, err.Error(), src)
	}
}

// --- Slice 4: generic structs ---

func TestGenericStructDeclOK(t *testing.T) {
	expectOK(t, "struct Box[T] { value: T }\n"+wrapMain("return 0"))
	expectOK(t, "struct Pair[A, B] { first: A, second: B }\n"+wrapMain("return 0"))
}

func TestGenericStructLitAnnotated(t *testing.T) {
	expectOK(t, "struct Box[T] { value: T }\n"+
		wrapMain("let b: Box[int] = Box { value: 5 }\n return b.value"))
}

func TestGenericStructFieldAccess(t *testing.T) {
	info := expectOK(t, "struct Box[T] { value: T }\n"+
		wrapMain("let b: Box[int] = Box { value: 5 }\n let v: int = b.value\n return v"))
	_ = info
}

func TestGenericStructFieldAssign(t *testing.T) {
	expectOK(t, "struct Box[T] { value: T }\n"+
		wrapMain("let b: Box[int] = Box { value: 5 }\n b.value = 10\n return b.value"))
}

func TestGenericStructMultiParam(t *testing.T) {
	expectOK(t, "struct Pair[A, B] { first: A, second: B }\n"+
		wrapMain("let p: Pair[int, string] = Pair { first: 1, second: \"hi\" }\n return 0"))
}

func TestGenericStructStringElem(t *testing.T) {
	expectOK(t, "struct Box[T] { value: T }\n"+
		wrapMain("let b: Box[string] = Box { value: \"hello\" }\n return 0"))
}

func TestGenericStructArrayElem(t *testing.T) {
	expectOK(t, "struct Box[T] { value: T }\n"+
		wrapMain("let b: Box[int[]] = Box { value: [1, 2, 3] }\n return 0"))
}

func TestGenericStructConcreteInstRegistered(t *testing.T) {
	// Two distinct instantiations must each appear in info.Structs.
	src := "struct Box[T] { value: T }\n" +
		wrapMain("let i: Box[int] = Box { value: 1 }\n let s: Box[string] = Box { value: \"x\" }\n return 0")
	info := expectOK(t, src)
	if _, ok := info.Structs["Box[int]@0"]; !ok {
		t.Fatalf("info.Structs missing Box[int]@0")
	}
	if _, ok := info.Structs["Box[string]@0"]; !ok {
		t.Fatalf("info.Structs missing Box[string]@0")
	}
}

func TestGenericStructMissingAnnotationErrors(t *testing.T) {
	// T is unused in field types, so inference cannot determine T.
	expectErr(t,
		"struct Phantom[T] { count: int }\n"+
			wrapMain("let b: Phantom = Phantom { count: 5 }\n return 0"),
		"cannot infer")
}

func TestGenericStructWrongFieldTypeErrors(t *testing.T) {
	expectErr(t,
		"struct Box[T] { value: T }\n"+
			wrapMain("let b: Box[int] = Box { value: \"oops\" }\n return 0"),
		"want int")
}

func TestGenericStructMissingFieldErrors(t *testing.T) {
	expectErr(t,
		"struct Box[T] { value: T }\n"+
			wrapMain("let b: Box[int] = Box { }\n return 0"),
		"missing field")
}

func TestGenericStructUnknownFieldErrors(t *testing.T) {
	expectErr(t,
		"struct Box[T] { value: T }\n"+
			wrapMain("let b: Box[int] = Box { value: 5, extra: 6 }\n return 0"),
		"no field")
}

func TestGenericStructNonGenericUsedAsGenericErrors(t *testing.T) {
	expectErr(t,
		"struct Point { x: int, y: int }\n"+
			wrapMain("let p: Point[int] = Point { x: 1, y: 2 }\n return 0"),
		"not generic")
}

func TestGenericStructWrongArityErrors(t *testing.T) {
	expectErr(t,
		"struct Box[T] { value: T }\n"+
			wrapMain("let b: Box[int, string] = Box { value: 5 }\n return 0"),
		"requires 1 type argument")
}

func TestGenericStructEmptyTypeParamErrors(t *testing.T) {
	expectParseErr(t, "struct Box[] { value: int }\n"+wrapMain("return 0"), "cannot be empty")
}

func TestGenericStructDuplicateTypeParamErrors(t *testing.T) {
	expectParseErr(t, "struct Box[T, T] { value: T }\n"+wrapMain("return 0"), "more than once")
}

// TestMonoNameCollision_Negative pins that a user function whose name matches a
// monomorphization the compiler would generate for a numeric generic is rejected
// (regression: add[T: numeric] + a user add__int silently collided in shell).
func TestMonoNameCollision_Negative(t *testing.T) {
	expectErr(t, "fn add[T: numeric](a: T, b: T) -> T { return a + b }\n"+
		"fn add__int(a: int, b: int) -> int { return a * b }\n"+
		wrapMain("return 0"), "collides with a monomorphization")
}

// TestMonoNameNonCollision_OK: a __int-suffixed name is fine when no matching
// numeric generic exists.
func TestMonoNameNonCollision_OK(t *testing.T) {
	expectOK(t, "fn parse__int(s: string) -> int { return to_int(s) }\n"+
		"fn main() -> int { return parse__int(\"5\") }")
}

// Result unifies against a generic Result[T] parameter (TS2).
func TestGenericResultUnifies(t *testing.T) {
	src := "fn get_or_default[T](r: Result[T], d: T) -> T {\n" +
		"  match (r) {\n" +
		"    case Ok(v) { return v }\n" +
		"    case Err(e) { return d }\n" +
		"  }\n" +
		"}\n" +
		wrapMain("let r: Result[int] = Ok(5)\n print(to_string(get_or_default(r, 0)))")
	info := check(t, src)
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %s", diagList(info.Errors))
	}
}

// A generic function returning Result[T] in statement position (return value
// discarded) must not leak an unsubstituted "$T" into info.Types/info.Calls
// (TS2's silent-leak claim).
func TestGenericResultTypeVarNoLeak(t *testing.T) {
	src := "fn wrap_ok[T](x: T) -> Result[T] { return Ok(x) }\n" +
		wrapMain("wrap_ok(41)")
	info := check(t, src)
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %s", diagList(info.Errors))
	}
	found := false
	for n, ci := range info.Calls {
		if ci.Kind == CallUser && n.CalleeName == "wrap_ok" {
			found = true
			if strings.Contains(string(ci.Result), "$") {
				t.Errorf("call result leaks a type variable: %s", ci.Result)
			}
			if strings.Contains(string(info.Types[n]), "$") {
				t.Errorf("recorded call type leaks a type variable: %s", info.Types[n])
			}
		}
	}
	if !found {
		t.Fatalf("no CallUser entry found for wrap_ok in info.Calls")
	}
}

// typeVarsIn must see T inside Result[T] so a wrong-shape first argument does
// not ALSO produce a spurious "cannot infer type parameter T" cascade.
func TestGenericResultCannotInferSuppressed(t *testing.T) {
	src := "fn r_first[T](a: Result[T], b: Result[T]) -> T { return unwrap(a) }\n" +
		wrapMain("let b: Result[int] = Ok(1)\n let x: int = r_first(5, b)\n print(to_string(x))")
	info := checkSrc(t, src)
	if len(info.Errors) != 1 {
		t.Fatalf("want exactly one (shape mismatch) error, got %d: %v", len(info.Errors), info.Errors)
	}
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "cannot infer") {
			t.Fatalf("spurious cannot-infer cascade: %q", d.Msg)
		}
	}
}

// Single-argument isolation of the Result[T] suppression path: the lone
// argument has the wrong shape (a plain int where Result[T] is expected), so
// nothing else can bind T. Without typeVarsIn seeing T inside Result[T], the
// end-of-call sweep would ALSO fire a spurious "cannot infer" (2 errors); with
// the fix, only the shape-mismatch error fires.
func TestGenericResultUnifyFailureSuppressesInfer(t *testing.T) {
	src := "fn only_result[T](r: Result[T]) -> T { return unwrap(r) }\n" +
		wrapMain("only_result(5)")
	info := checkSrc(t, src)
	if len(info.Errors) != 1 {
		t.Fatalf("want exactly one (shape mismatch) error, got %d: %v", len(info.Errors), info.Errors)
	}
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "cannot infer") {
			t.Fatalf("spurious cannot-infer cascade: %q", d.Msg)
		}
	}
}

// Tuple unifies against a generic (T, U) parameter (TS2).
func TestGenericTupleUnifies(t *testing.T) {
	src := "fn first[T, U](p: (T, U)) -> T { return p[0] }\n" +
		wrapMain("let p: (int, string) = (1, \"x\")\n print(to_string(first(p)))")
	info := check(t, src)
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %s", diagList(info.Errors))
	}
}

// A generic function returning (T, U) in statement position must not leak
// unsubstituted type variables into info.Types/info.Calls.
func TestGenericTupleTypeVarNoLeak(t *testing.T) {
	src := "fn wrap_pair[T, U](x: T, y: U) -> (T, U) { return (x, y) }\n" +
		wrapMain("wrap_pair(1, \"s\")")
	info := check(t, src)
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %s", diagList(info.Errors))
	}
	found := false
	for n, ci := range info.Calls {
		if ci.Kind == CallUser && n.CalleeName == "wrap_pair" {
			found = true
			if strings.Contains(string(ci.Result), "$") {
				t.Errorf("call result leaks a type variable: %s", ci.Result)
			}
			if strings.Contains(string(info.Types[n]), "$") {
				t.Errorf("recorded call type leaks a type variable: %s", info.Types[n])
			}
		}
	}
	if !found {
		t.Fatalf("no CallUser entry found for wrap_pair in info.Calls")
	}
}

// A tuple with the wrong arity does not unify -- hard structural mismatch.
func TestGenericTupleArityMismatch(t *testing.T) {
	expectErr(t, "fn first[T, U](p: (T, U)) -> T { return p[0] }\n"+
		wrapMain("let q: (int, string, bool) = (1, \"x\", true)\n print(to_string(first(q)))"), "does not match")
}

// typeVarsIn must see T/U inside (T, U) so a wrong-shape first argument does
// not ALSO produce a spurious "cannot infer" cascade.
func TestGenericTupleCannotInferSuppressed(t *testing.T) {
	src := "fn pair_first[T, U](p: (T, U), q: (T, U)) -> T { return p[0] }\n" +
		wrapMain("let q: (int, string) = (1, \"x\")\n let x: int = pair_first(5, q)\n print(to_string(x))")
	info := checkSrc(t, src)
	if len(info.Errors) != 1 {
		t.Fatalf("want exactly one (shape mismatch) error, got %d: %v", len(info.Errors), info.Errors)
	}
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "cannot infer") {
			t.Fatalf("spurious cannot-infer cascade: %q", d.Msg)
		}
	}
}

// Single-argument isolation of the tuple suppression path: the lone argument
// has the wrong shape (a plain int where (T, U) is expected), so nothing else
// can bind T/U. Without typeVarsIn seeing T/U inside (T, U), the end-of-call
// sweep would ALSO fire a spurious "cannot infer" (2 errors); with the fix,
// only the shape-mismatch error fires.
func TestGenericTupleUnifyFailureSuppressesInfer(t *testing.T) {
	src := "fn only_tuple[T, U](p: (T, U)) -> T { return p[0] }\n" +
		wrapMain("only_tuple(5)")
	info := checkSrc(t, src)
	if len(info.Errors) != 1 {
		t.Fatalf("want exactly one (shape mismatch) error, got %d: %v", len(info.Errors), info.Errors)
	}
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "cannot infer") {
			t.Fatalf("spurious cannot-infer cascade: %q", d.Msg)
		}
	}
}

// Box[int] unifies against a generic Box[T] parameter (TS2).
func TestGenericStructInstUnifies(t *testing.T) {
	src := "struct Box[T] { value: T }\n" +
		"fn unwrap_box[T](b: Box[T]) -> T { return b.value }\n" +
		wrapMain("let b: Box[int] = Box { value: 7 }\n print(to_string(unwrap_box(b)))")
	info := check(t, src)
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %s", diagList(info.Errors))
	}
}

// A struct instantiation of a DIFFERENT base struct is a hard mismatch, not a
// partial/best-effort match.
func TestGenericStructInstWrongBaseMismatch(t *testing.T) {
	expectErr(t, "struct Box[T] { value: T }\n"+
		"struct Wrap[T] { value: T }\n"+
		"fn unwrap_box[T](b: Box[T]) -> T { return b.value }\n"+
		wrapMain("let w: Wrap[int] = Wrap { value: 7 }\n print(to_string(unwrap_box(w)))"), "does not match")
}

// Nested generic-struct instantiations (Box[Box[T]]) recurse correctly.
func TestGenericStructInstNested(t *testing.T) {
	src := "struct Box[T] { value: T }\n" +
		"fn unwrap2[T](b: Box[Box[T]]) -> T { return b.value.value }\n" +
		wrapMain("let b: Box[Box[int]] = Box { value: Box { value: 9 } }\n print(to_string(unwrap2(b)))")
	info := check(t, src)
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %s", diagList(info.Errors))
	}
}

// applySubst must substitute inside a RETURNED generic-struct-instantiation
// type, not just accept it as a parameter (unify alone would not catch this).
func TestGenericStructInstSubstituted(t *testing.T) {
	src := "struct Box[T] { value: T }\n" +
		"fn make_box[T](x: T) -> Box[T] { return Box { value: x } }\n" +
		wrapMain("make_box(7)")
	info := check(t, src)
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %s", diagList(info.Errors))
	}
	found := false
	for n, ci := range info.Calls {
		if ci.Kind == CallUser && n.CalleeName == "make_box" {
			found = true
			if strings.Contains(string(ci.Result), "$") {
				t.Errorf("call result leaks a type variable: %s", ci.Result)
			}
			if strings.Contains(string(info.Types[n]), "$") {
				t.Errorf("recorded call type leaks a type variable: %s", info.Types[n])
			}
		}
	}
	if !found {
		t.Fatalf("no CallUser entry found for make_box in info.Calls")
	}
}

// A funcref whose return type is itself a generic-struct instantiation
// (fn(T)->Box[T]) must be unified/substituted through the FUNCREF case, never
// misclassified as a bare generic-struct-instantiation token. make_int_box is
// deliberately NON-generic: passing a generic function by name as a funcref
// value is a separate, pre-existing compile error (see TestGenericFuncrefValue
// above), so a generic make_box[T] here would fail for the wrong reason and
// never reach the code path this test exists to exercise.
func TestGenericFuncrefReturningStructInstNotMisclassified(t *testing.T) {
	src := "struct Box[T] { value: T }\n" +
		"fn make_int_box(x: int) -> Box[int] { return Box { value: x } }\n" +
		"fn use[T](f: fn(T) -> Box[T], x: T) -> Box[T] { return f(x) }\n" +
		wrapMain("use(make_int_box, 5)")
	info := check(t, src)
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %s", diagList(info.Errors))
	}
	found := false
	for n, ci := range info.Calls {
		if ci.Kind == CallUser && n.CalleeName == "use" {
			found = true
			if strings.Contains(string(ci.Result), "$") {
				t.Errorf("call result leaks a type variable: %s", ci.Result)
			}
			if strings.Contains(string(info.Types[n]), "$") {
				t.Errorf("recorded call type leaks a type variable: %s", info.Types[n])
			}
		}
	}
	if !found {
		t.Fatalf("no CallUser entry found for use in info.Calls")
	}
}

// typeVarsIn must see T inside Box[T] so a wrong-shape first argument does not
// ALSO produce a spurious "cannot infer" cascade.
func TestGenericStructInstCannotInferSuppressed(t *testing.T) {
	src := "struct Box[T] { value: T }\n" +
		"fn b_first[T](a: Box[T], b: Box[T]) -> T { return a.value }\n" +
		wrapMain("let b: Box[int] = Box { value: 1 }\n let x: int = b_first(5, b)\n print(to_string(x))")
	info := checkSrc(t, src)
	if len(info.Errors) != 1 {
		t.Fatalf("want exactly one (shape mismatch) error, got %d: %v", len(info.Errors), info.Errors)
	}
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "cannot infer") {
			t.Fatalf("spurious cannot-infer cascade: %q", d.Msg)
		}
	}
}

// Single-argument isolation of the generic-struct-instantiation suppression
// path: the lone argument has the wrong shape (a plain int where Box[T] is
// expected), so nothing else can bind T. Without typeVarsIn seeing T inside
// Box[T], the end-of-call sweep would ALSO fire a spurious "cannot infer" (2
// errors); with the fix, only the shape-mismatch error fires.
func TestGenericStructInstUnifyFailureSuppressesInfer(t *testing.T) {
	src := "struct Box[T] { value: T }\n" +
		"fn only_box[T](b: Box[T]) -> T { return b.value }\n" +
		wrapMain("only_box(5)")
	info := checkSrc(t, src)
	if len(info.Errors) != 1 {
		t.Fatalf("want exactly one (shape mismatch) error, got %d: %v", len(info.Errors), info.Errors)
	}
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "cannot infer") {
			t.Fatalf("spurious cannot-infer cascade: %q", d.Msg)
		}
	}
}
