package codegen

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/module"
	"github.com/mitchellnemitz/wisp/internal/parser"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// End-to-end coverage for the generic higher-order builtins' funcref wrappers
// (map/filter/each/reduce/sort_by/find/any/all/count_where/and_then/or_else/
// map_err) and contains/index_of's overloaded array arm. Reconstructed for the
// modules-only surface: the removable higher-order builtins are referenced as
// member funcref VALUES through their namespace (array.map, string.contains,
// ...), which resolve to the SAME synthesized wrappers the pre-removal bare
// funcref did. and_then/or_else/map_err stay flat (not modularized) and keep
// their bare funcref spelling. Each test proves the referenced builtin actually
// RUNS through its wrapper, not just that the checker accepts it.
//
// contains/index_of are members of the string namespace only; their array arm is
// reached by delegation (string.contains / string.index_of typed with array
// signatures), exactly as the flat overload resolver reached it pre-removal.

// TestGenericFuncref_ContainsIndexOfArms: the named arm matrix for
// contains/index_of, both a string arm AND an array arm (pinned to int[]),
// invoked indirectly through their arm-suffixed wrappers.
func TestGenericFuncref_ContainsIndexOfArms(t *testing.T) {
	out, errb, code := runNS(t, `fn main() -> int {
  let cs: fn(string, string) -> bool = string.contains
  let ca: fn(int[], int) -> bool = string.contains
  let isf: fn(string, string) -> Optional[int] = string.index_of
  let iaf: fn(int[], int) -> Optional[int] = string.index_of
  print(to_string(cs("hello world", "wor")))
  let xs: int[] = [10, 20, 30]
  print(to_string(ca(xs, 20)))
  print(to_string(ca(xs, 99)))
  let r1: Optional[int] = isf("hello", "ll")
  print(to_string(unwrap(r1)))
  let r2: Optional[int] = iaf(xs, 30)
  print(to_string(unwrap(r2)))
  let r3: Optional[int] = iaf(xs, 99)
  print(to_string(is_none(r3)))
  return 0
}`, "string")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb)
	}
	want := "true\ntrue\nfalse\n2\n2\ntrue\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

// TestGenericFuncref_IndexOfSentinelOverwritten: the internal -1 "not found"
// sentinel used to build the Optional[int] result must not leak as, or be
// confused with, a real result. Finding at index 0 (a value outside the
// sentinel's own domain of "not found") must report Some(0), and a genuine
// miss must report None, not Some(-1).
func TestGenericFuncref_IndexOfSentinelOverwritten(t *testing.T) {
	out, errb, code := runNS(t, `fn main() -> int {
  let f: fn(string, string) -> Optional[int] = string.index_of
  let r1: Optional[int] = f("abc", "a")
  print(to_string(is_some(r1)))
  print(to_string(unwrap(r1)))
  let r2: Optional[int] = f("abc", "z")
  print(to_string(is_none(r2)))
  return 0
}`, "string")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb)
	}
	want := "true\n0\ntrue\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

// TestGenericFuncref_MapContainerAxes: map's three container-overload axes
// (array, Optional, Result funcref) all resolve and run correctly.
func TestGenericFuncref_MapContainerAxes(t *testing.T) {
	out, errb, code := runNS(t, `fn dbl(x: int) -> int { return x * 2 }
fn main() -> int {
  let ma: fn(int[], fn(int) -> int) -> int[] = array.map
  let ys: int[] = ma([1, 2, 3], dbl)
  print(to_string(ys[0] + ys[1] + ys[2]))
  let mo: fn(Optional[int], fn(int) -> int) -> Optional[int] = array.map
  let o: Optional[int] = Some(5)
  let ro: Optional[int] = mo(o, dbl)
  print(to_string(unwrap(ro)))
  let mr: fn(Result[int], fn(int) -> int) -> Result[int] = array.map
  let r: Result[int] = Ok(7)
  let rr: Result[int] = mr(r, dbl)
  print(to_string(unwrap(rr)))
  return 0
}`, "array")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb)
	}
	want := "12\n10\n14\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

// TestGenericFuncref_MapArrayTwoInstantiations: the array arm's
// generic-instantiation axis at two distinct element/result type pairs
// (fn(int)->int and fn(string)->bool), proving the single __wisp_builtin_
// map_array wrapper is not hard-coded to one scalar ABI.
func TestGenericFuncref_MapArrayTwoInstantiations(t *testing.T) {
	out, errb, code := runNS(t, `fn dbl(x: int) -> int { return x * 2 }
fn nonEmpty(s: string) -> bool { return !string.is_empty(s) }
fn main() -> int {
  let m1: fn(int[], fn(int) -> int) -> int[] = array.map
  let ys: int[] = m1([1, 2, 3], dbl)
  print(to_string(ys[0] + ys[1] + ys[2]))
  let m2: fn(string[], fn(string) -> bool) -> bool[] = array.map
  let zs: bool[] = m2(["a", "", "c"], nonEmpty)
  print(to_string(zs[0]))
  print(to_string(zs[1]))
  print(to_string(zs[2]))
  return 0
}`, "array", "string")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb)
	}
	want := "12\ntrue\nfalse\ntrue\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

// TestGenericFuncref_MapArrayNoCrossHandleAliasing: two independent
// invocations of the SAME map_array wrapper (same shell function, called
// twice from two separate statements) must allocate distinct output array
// handles; if the second call clobbered/aliased the first's handle, both
// results would read back as the second call's values.
func TestGenericFuncref_MapArrayNoCrossHandleAliasing(t *testing.T) {
	out, errb, code := runNS(t, `fn inc(x: int) -> int { return x + 1 }
fn main() -> int {
  let m: fn(int[], fn(int) -> int) -> int[] = array.map
  let a: int[] = m([1, 2, 3], inc)
  let b: int[] = m([10, 20, 30], inc)
  print(to_string(a[0] + a[1] + a[2]))
  print(to_string(b[0] + b[1] + b[2]))
  return 0
}`, "array")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb)
	}
	want := "9\n63\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

// TestGenericFuncref_FilterContainerAxes: filter's two container-overload
// axes (array, Optional funcref) both resolve and run correctly.
func TestGenericFuncref_FilterContainerAxes(t *testing.T) {
	out, errb, code := runNS(t, `fn pos(x: int) -> bool { return x > 0 }
fn main() -> int {
  let fa: fn(int[], fn(int) -> bool) -> int[] = array.filter
  let ys: int[] = fa([1, -2, 3], pos)
  print(to_string(length(ys)))
  let fo: fn(Optional[int], fn(int) -> bool) -> Optional[int] = array.filter
  let o: Optional[int] = Some(5)
  let ro: Optional[int] = fo(o, pos)
  print(to_string(is_some(ro)))
  return 0
}`, "array")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb)
	}
	want := "2\ntrue\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

// TestGenericFuncref_FilterArrayTwoInstantiations: filter's array arm at two
// distinct element type instantiations.
func TestGenericFuncref_FilterArrayTwoInstantiations(t *testing.T) {
	out, errb, code := runNS(t, `fn posI(x: int) -> bool { return x > 0 }
fn nonEmptyS(s: string) -> bool { return !string.is_empty(s) }
fn main() -> int {
  let f1: fn(int[], fn(int) -> bool) -> int[] = array.filter
  let ys: int[] = f1([1, -2, 3], posI)
  print(to_string(length(ys)))
  let f2: fn(string[], fn(string) -> bool) -> string[] = array.filter
  let zs: string[] = f2(["a", "", "c"], nonEmptyS)
  print(to_string(length(zs)))
  return 0
}`, "array", "string")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb)
	}
	want := "2\n2\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

// TestGenericFuncref_ArrayOnlyBuiltins: each/reduce/sort_by/find/any/all/
// count_where, each invoked indirectly through its funcref wrapper.
func TestGenericFuncref_ArrayOnlyBuiltins(t *testing.T) {
	out, errb, code := runNS(t, `fn show(x: int) -> void { print(to_string(x)) }
fn add(a: int, b: int) -> int { return a + b }
fn lt(a: int, b: int) -> bool { return a < b }
fn even(x: int) -> bool { return x % 2 == 0 }
fn gt10(x: int) -> bool { return x > 10 }
fn main() -> int {
  let xs: int[] = [3, 1, 2]
  let e: fn(int[], fn(int) -> void) -> void = array.each
  e(xs, show)
  let r: fn(int[], int, fn(int, int) -> int) -> int = array.reduce
  print(to_string(r(xs, 0, add)))
  let s: fn(int[], fn(int, int) -> bool) -> int[] = array.sort_by
  let sorted: int[] = s(xs, lt)
  print(to_string(sorted[0]))
  print(to_string(sorted[1]))
  print(to_string(sorted[2]))
  let fnd: fn(int[], fn(int) -> bool) -> Optional[int] = array.find
  let fo: Optional[int] = fnd(xs, even)
  print(to_string(unwrap(fo)))
  let an: fn(int[], fn(int) -> bool) -> bool = array.any
  print(to_string(an(xs, gt10)))
  let al: fn(int[], fn(int) -> bool) -> bool = array.all
  print(to_string(al(xs, even)))
  let cw: fn(int[], fn(int) -> bool) -> int = array.count_where
  print(to_string(cw(xs, even)))
  return 0
}`, "array")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb)
	}
	want := "3\n1\n2\n6\n1\n2\n3\n2\nfalse\nfalse\n1\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

// TestGenericFuncref_AndThenOrElseMapErr: and_then/or_else's Optional AND
// Result axes, plus map_err's Result axis, each invoked indirectly.
// and_then/or_else/map_err are not modularized (they stay flat), so the bare
// funcref value spelling is retained.
func TestGenericFuncref_AndThenOrElseMapErr(t *testing.T) {
	out, errb, code := runWisp(t, `fn safeHalf(x: int) -> Optional[int] {
  if (x == 0) { return None }
  return Some(x / 2)
}
fn fallback() -> Optional[int] { return Some(99) }
fn doubleSafe(x: int) -> Result[int] { return Ok(x * 2) }
fn rescue(e: error) -> Result[int] { return Ok(0) }
fn wrapErr(e: error) -> error { return error("wrapped: " + e.message) }
fn main() -> int {
  let at: fn(Optional[int], fn(int) -> Optional[int]) -> Optional[int] = and_then
  let s: Optional[int] = Some(10)
  print(to_string(unwrap(at(s, safeHalf))))
  let atr: fn(Result[int], fn(int) -> Result[int]) -> Result[int] = and_then
  let ok: Result[int] = Ok(3)
  print(to_string(unwrap(atr(ok, doubleSafe))))
  let oe: fn(Optional[int], fn() -> Optional[int]) -> Optional[int] = or_else
  let n: Optional[int] = None
  print(to_string(unwrap(oe(n, fallback))))
  let oer: fn(Result[int], fn(error) -> Result[int]) -> Result[int] = or_else
  let e: error = error("orig")
  let err: Result[int] = Err(e)
  print(to_string(unwrap(oer(err, rescue))))
  let me: fn(Result[int], fn(error) -> error) -> Result[int] = map_err
  let e2: error = error("a")
  let err2: Result[int] = Err(e2)
  let r2: Result[int] = me(err2, wrapErr)
  print(unwrap_err(r2).message)
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb)
	}
	want := "5\n6\n99\n0\nwrapped: a\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

// TestMemberFuncref_GenericAxis_ArrayMapRuntime: the Part 3 namespaced-member
// path for a generic higher-order builtin (array.map in value position) runs
// through the SAME __wisp_builtin_map_array wrapper as the bare-ident path.
func TestMemberFuncref_GenericAxis_ArrayMapRuntime(t *testing.T) {
	root, err := parser.Parse(`fn dbl(x: int) -> int { return x * 2 }
fn main() -> int {
  let f: fn(int[], fn(int) -> int) -> int[] = array.map
  let ys: int[] = f([1, 2, 3], dbl)
  print(to_string(ys[0] + ys[1] + ys[2]))
  return 0
}`, "test.wisp")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	linked := &module.Linked{Modules: []*module.Module{
		{ID: 0, Prog: root, Namespaces: map[string]int{"array": 1}},
		{ID: 1, Prog: &ast.Program{}, Namespaces: map[string]int{}, Core: "array"},
	}}
	info := types.CheckLinked(linked)
	if len(info.Errors) > 0 {
		t.Fatalf("check errors: %v", info.Errors)
	}
	script, _, err := GenerateLinked(linked, info)
	if err != nil {
		t.Fatalf("GenerateLinked: %v", err)
	}
	if !strings.Contains(string(script), "__wisp_builtin_map_array") {
		t.Error("__wisp_builtin_map_array wrapper not emitted for array.map used as a value")
	}
	out, errb, code := run(t, script)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb)
	}
	if strings.TrimRight(out, "\n") != "12" {
		t.Errorf("array.map funcref output = %q, want %q", out, "12")
	}
}

// TestMemberFuncref_GenericAxis_StringContainsRuntime: the Part 3 path for
// contains's overloaded string arm, referenced through string.contains.
func TestMemberFuncref_GenericAxis_StringContainsRuntime(t *testing.T) {
	root, err := parser.Parse(`fn main() -> int {
  let f: fn(string, string) -> bool = string.contains
  print(to_string(f("hello world", "wor")))
  return 0
}`, "test.wisp")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	linked := &module.Linked{Modules: []*module.Module{
		{ID: 0, Prog: root, Namespaces: map[string]int{"string": 1}},
		{ID: 1, Prog: &ast.Program{}, Namespaces: map[string]int{}, Core: "string"},
	}}
	info := types.CheckLinked(linked)
	if len(info.Errors) > 0 {
		t.Fatalf("check errors: %v", info.Errors)
	}
	script, _, err := GenerateLinked(linked, info)
	if err != nil {
		t.Fatalf("GenerateLinked: %v", err)
	}
	out, errb, code := run(t, script)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb)
	}
	if strings.TrimRight(out, "\n") != "true" {
		t.Errorf("string.contains funcref output = %q, want %q", out, "true")
	}
}
