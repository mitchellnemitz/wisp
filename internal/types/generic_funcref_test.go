package types

import "testing"

// map/filter/each/reduce/sort_by/find/any/all/count_where are removable
// (array module); the funcref-axis references migrated to the namespaced form,
// where CheckLinked resolves the member and records it in MemberFuncRefs (helpers
// wantNsOK/wantNsErr live in core_collections_neg_test.go). and_then/or_else/
// map_err stay flat, so their references remain bare (recorded in FuncRefs).

// TestGenericFuncref_MapFilterAxes pins map/filter's container-overload axes
// (array, Optional, Result for map; array, Optional for filter): a
// correctly-shaped annotation resolves and records a FuncRef of exactly that
// axis's type; a mismatched or absent annotation is rejected as ambiguous.
func TestGenericFuncref_MapFilterAxes(t *testing.T) {
	for _, c := range []struct {
		name string
		ok   string
	}{
		{"map", `fn dbl(x: int) -> int { return x * 2 }
fn main()->int{ let f: fn(int[], fn(int)->int) -> int[] = array.map; let _: int[] = f([1], dbl); return 0 }`},
		{"map", `fn dbl(x: int) -> int { return x * 2 }
fn main()->int{ let f: fn(Optional[int], fn(int)->int) -> Optional[int] = array.map; return 0 }`},
		{"map", `fn dbl(x: int) -> int { return x * 2 }
fn main()->int{ let f: fn(Result[int], fn(int)->int) -> Result[int] = array.map; return 0 }`},
		{"filter", `fn pos(x: int) -> bool { return x > 0 }
fn main()->int{ let f: fn(int[], fn(int)->bool) -> int[] = array.filter; let _: int[] = f([1], pos); return 0 }`},
		{"filter", `fn pos(x: int) -> bool { return x > 0 }
fn main()->int{ let f: fn(Optional[int], fn(int)->bool) -> Optional[int] = array.filter; return 0 }`},
	} {
		info := wantNsOK(t, "array", c.ok)
		if len(info.MemberFuncRefs) == 0 {
			t.Fatalf("%s: %q: expected a MemberFuncRef recorded", c.name, c.ok)
		}
	}

	// Ambiguous: a context matching no axis is rejected, naming the containers.
	for _, c := range []struct{ src, want string }{
		{`fn main()->int{ let f: fn(int)->int = array.map; return 0 }`,
			`"map" has no function-reference form matching fn(int)->int; supported containers: array, optional, result`},
		{`fn main()->int{ let f: fn(int[], fn(int)->void) -> int[] = array.map; return 0 }`, // f must return non-void
			`"map" has no function-reference form matching fn([int],fn(int)->void)->[int]; supported containers: array, optional, result`},
		{`fn main()->int{ let f: fn(int)->bool = array.filter; return 0 }`,
			`"filter" has no function-reference form matching fn(int)->bool; supported containers: array, optional`},
		{`fn main()->int{ let f: fn(int[], fn(int)->int) -> int[] = array.filter; return 0 }`, // f must return bool
			`"filter" has no function-reference form matching fn([int],fn(int)->int)->[int]; supported containers: array, optional`},
	} {
		wantNsErr(t, "array", c.src, c.want)
	}
}

func TestGenericFuncrefMapNoContext(t *testing.T) {
	// True no-annotation case (want == Invalid) for the generic family,
	// mirroring TestCoreMathFuncrefValueAbsNoContext for the overloaded family.
	wantNsErr(t, "array", `fn main() -> int { array.map; return 0 }`,
		`reference to generic builtin "map" needs a function-reference type annotation to select a container overload`)
}

// TestGenericFuncref_ArrayOnlyBuiltins pins the array-only generic higher-order
// builtins (each/reduce/sort_by/find/any/all/count_where): a correctly-shaped
// annotation resolves; a mismatched one is rejected as ambiguous. and_then/
// or_else/map_err stay flat, so they are exercised bare.
func TestGenericFuncref_ArrayOnlyBuiltins(t *testing.T) {
	// Removable array-only builtins: namespaced references record a MemberFuncRef.
	for _, c := range []struct {
		name string
		ok   string
	}{
		{"each", `fn show(x: int) -> void { }
fn main()->int{ let f: fn(int[], fn(int)->void) -> void = array.each; return 0 }`},
		{"reduce", `fn add(a: int, b: int) -> int { return a + b }
fn main()->int{ let f: fn(int[], int, fn(int,int)->int) -> int = array.reduce; return 0 }`},
		{"sort_by", `fn lt(a: int, b: int) -> bool { return a < b }
fn main()->int{ let f: fn(int[], fn(int,int)->bool) -> int[] = array.sort_by; return 0 }`},
		{"find", `fn even(x: int) -> bool { return x % 2 == 0 }
fn main()->int{ let f: fn(int[], fn(int)->bool) -> Optional[int] = array.find; return 0 }`},
		{"any", `fn even(x: int) -> bool { return x % 2 == 0 }
fn main()->int{ let f: fn(int[], fn(int)->bool) -> bool = array.any; return 0 }`},
		{"all", `fn even(x: int) -> bool { return x % 2 == 0 }
fn main()->int{ let f: fn(int[], fn(int)->bool) -> bool = array.all; return 0 }`},
		{"count_where", `fn even(x: int) -> bool { return x % 2 == 0 }
fn main()->int{ let f: fn(int[], fn(int)->bool) -> int = array.count_where; return 0 }`},
	} {
		info := wantNsOK(t, "array", c.ok)
		if len(info.MemberFuncRefs) == 0 {
			t.Fatalf("%s: %q: expected a MemberFuncRef recorded", c.name, c.ok)
		}
	}

	// Stays-flat combinators: bare references record a FuncRef.
	for _, c := range []struct {
		name string
		ok   string
	}{
		{"and_then", `fn safeHalf(x: int) -> Optional[int] { return Some(x / 2) }
fn main()->int{ let f: fn(Optional[int], fn(int)->Optional[int]) -> Optional[int] = and_then; return 0 }`},
		{"and_then", `fn doubleSafe(x: int) -> Result[int] { return Ok(x * 2) }
fn main()->int{ let f: fn(Result[int], fn(int)->Result[int]) -> Result[int] = and_then; return 0 }`},
		{"or_else", `fn fallback() -> Optional[int] { return Some(0) }
fn main()->int{ let f: fn(Optional[int], fn()->Optional[int]) -> Optional[int] = or_else; return 0 }`},
		{"or_else", `fn rescue(e: error) -> Result[int] { return Ok(0) }
fn main()->int{ let f: fn(Result[int], fn(error)->Result[int]) -> Result[int] = or_else; return 0 }`},
		{"map_err", `fn wrapErr(e: error) -> error { return e }
fn main()->int{ let f: fn(Result[int], fn(error)->error) -> Result[int] = map_err; return 0 }`},
	} {
		info := check(t, c.ok)
		if len(info.Errors) != 0 {
			t.Fatalf("%s: %q: unexpected errors: %v", c.name, c.ok, errMsgs(info))
		}
		if len(info.FuncRefs) == 0 {
			t.Fatalf("%s: %q: expected a FuncRef recorded", c.name, c.ok)
		}
	}

	// Ambiguous: no annotation, or one matching no axis (each has only one axis, so
	// any mismatch on shape is enough to prove it's not silently coerced).
	for _, c := range []struct{ name, want string }{
		{"each", `"each" has no function-reference form matching fn(int)->int; supported containers: array`},
		{"reduce", `"reduce" has no function-reference form matching fn(int)->int; supported containers: array`},
		{"sort_by", `"sort_by" has no function-reference form matching fn(int)->int; supported containers: array`},
		{"find", `"find" has no function-reference form matching fn(int)->int; supported containers: array`},
		{"any", `"any" has no function-reference form matching fn(int)->int; supported containers: array`},
		{"all", `"all" has no function-reference form matching fn(int)->int; supported containers: array`},
		{"count_where", `"count_where" has no function-reference form matching fn(int)->int; supported containers: array`},
	} {
		src := `fn main()->int{ let f: fn(int)->int = array.` + c.name + `; return 0 }`
		wantNsErr(t, "array", src, c.want)
	}
	for _, c := range []struct{ src, want string }{
		{`fn main()->int{ let f: fn(int)->int = and_then; return 0 }`,
			`"and_then" has no function-reference form matching fn(int)->int; supported containers: optional, result`},
		{`fn main()->int{ let f: fn(int)->int = or_else; return 0 }`,
			`"or_else" has no function-reference form matching fn(int)->int; supported containers: optional, result`},
		{`fn main()->int{ let f: fn(int)->int = map_err; return 0 }`,
			`"map_err" has no function-reference form matching fn(int)->int; supported containers: result`},
	} {
		expectErr(t, c.src, c.want)
	}
}
