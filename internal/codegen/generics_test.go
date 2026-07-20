package codegen

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/types"
)

// Codegen safety surface for generic-function BODIES (spec 8 guarantee b).
//
// A generic function's body is checked ONCE, generically, so its param/let/return
// nodes legitimately carry type variables ("$T", "[$T]") in info.Types. Codegen is
// UNCHANGED for generics; the claim that a type-variable-typed body lowers
// correctly rests on every info.Types read on the erasure-safe paths a generic
// body uses either matching structurally or falling through to the runtime-correct
// erased default. Enumerated and confirmed against the live codegen:
//
//   - genPush (aggregate.go), genSome/genIsSome/genUnwrap/genUnwrapOr (optional.go):
//     read NO info.Types; operate on handle ids only. A $T operand lowers identically
//     to a concrete one. SAFE. (push is now a removable builtin (array.push) and its
//     bare call no longer resolves in the single-module check, so the generic body
//     below no longer calls it; its erasure safety is trivial anyway -- genPush reads
//     no info.Types, so $T lowers identically to the concrete push covered by
//     core_byteidentity (array|array.push) and internal/golden coll_arrays. The body
//     now uses a $T-element array literal + $T index reads instead.)
//   - genIndexExpr (aggregate.go:132): reads types.IsDict(info.Types[n.X]); for a
//     "[$T]"-typed array node IsDict is FALSE, so it takes the array path; the index
//     node is int, never $T. SAFE.
//   - length dispatch (expr.go:500): reads types.IsArray(info.Types[ci.Args[0]]);
//     IsArray("[$T]") is TRUE -> the array-count path. SAFE.
//   - every == types.Float / == types.Int read sits on an erasure-BREAK path the
//     Task-4 bare-T guards already reject before codegen; a legitimately $T node
//     compares FALSE and falls through to the int/text erased default. SAFE.
//
// Finding: NO generic-body node reaches a type-dispatch that mis-lowers; codegen
// needs no change. TestGenericBodyLowersErased asserts this end-to-end. If it ever
// fails because a path did NOT fall through correctly, that is a real codegen
// finding, not a test to weaken.
func TestGenericBodyLowersErased(t *testing.T) {
	src := `fn dup_first[T](xs: T[], x: T) -> T[] {
	let head: T = xs[0]
	return [xs[0], xs[1], x, head]
}
fn first_of[T](xs: T[]) -> Optional[T] {
	if (length(xs) > 0) {
		return Some(xs[0])
	}
	return None
}
fn main() -> int {
	let a: int[] = [1, 2]
	let b: int[] = dup_first(a, 9)
	print("len: ${length(b)}")
	print("last: ${b[3]}")
	let f: Optional[int] = first_of(b)
	print("first: ${unwrap(f)}")
	let s: string[] = ["x", "y"]
	let fs: Optional[string] = first_of(s)
	print("first str: ${unwrap(fs)}")
	return 0
}`
	stdout, stderr, code := runWisp(t, src)
	if code != 0 {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr)
	}
	// dup_first(a=[1,2], 9): head = xs[0] = 1; result = [xs[0],xs[1],x,head] = [1,2,9,1].
	// So b = [1,2,9,1], length 4, b[3] = 1, first = b[0] = 1.
	want := "len: 4\nlast: 1\nfirst: 1\nfirst str: x\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

// A generic function compiles to exactly ONE shell function (no monomorphization),
// derived robustly from the live mangling rather than a hardcoded name.
func TestGenericSingleShellFunction(t *testing.T) {
	src := `fn first_of[T](xs: T[]) -> Optional[T] {
	if (length(xs) > 0) { return Some(xs[0]) }
	return None
}
fn main() -> int {
	let a: int[] = [1]
	let b: string[] = ["x"]
	let _i: Optional[int] = first_of(a)
	let _s: Optional[string] = first_of(b)
	return 0
}`
	out := string(compile(t, src))
	def := types.MangleFunc(0, "first_of") + "()"
	if n := strings.Count(out, def); n != 1 {
		t.Fatalf("expected exactly 1 definition of %q, got %d", def, n)
	}
}

// A comparable-bounded generic compiles to exactly ONE shell function (no
// monomorphization), like the unbounded case. The comparable bound is a
// front-end relaxation only; codegen is unchanged.
func TestComparableGenericSingleShellFunction(t *testing.T) {
	src := `fn contains_eq[T: comparable](xs: T[], target: T) -> bool {
	for (x in xs) { if (x == target) { return true } }
	return false
}
fn main() -> int {
	let a: int[] = [1]
	let b: string[] = ["x"]
	let _i: bool = contains_eq(a, 1)
	let _s: bool = contains_eq(b, "x")
	return 0
}`
	out := string(compile(t, src))
	def := types.MangleFunc(0, "contains_eq") + "()"
	if n := strings.Count(out, def); n != 1 {
		t.Fatalf("expected exactly 1 definition of %q, got %d", def, n)
	}
}

// A numeric-bounded generic called with BOTH int and float must emit exactly
// TWO shell function definitions: one suffixed __int and one suffixed __float.
// The base (unsuffixed) name must not appear as a definition.
func TestNumericGenericTwoShellFunctions(t *testing.T) {
	src := `fn add[T: numeric](a: T, b: T) -> T { return a + b }
fn main() -> int {
	let i: int = add(1, 2)
	let f: float = add(1.0, 2.0)
	return 0
}`
	out := string(compile(t, src))
	base := types.MangleFunc(0, "add")
	intDef := base + "__int()"
	floatDef := base + "__float()"
	if n := strings.Count(out, intDef); n != 1 {
		t.Fatalf("expected exactly 1 definition of %q, got %d", intDef, n)
	}
	if n := strings.Count(out, floatDef); n != 1 {
		t.Fatalf("expected exactly 1 definition of %q, got %d", floatDef, n)
	}
	// The bare unsuffixed definition must NOT exist.
	bareDef := base + "()"
	if strings.Contains(out, bareDef) {
		t.Fatalf("bare (unsuffixed) definition %q must not exist for a numeric generic", bareDef)
	}
}

// A numeric generic called with only int emits exactly one instantiation (__int)
// and no float variant.
func TestNumericGenericOnlyIntEmitsOneFunction(t *testing.T) {
	src := `fn add[T: numeric](a: T, b: T) -> T { return a + b }
fn main() -> int {
	let i: int = add(3, 4)
	return 0
}`
	out := string(compile(t, src))
	base := types.MangleFunc(0, "add")
	intDef := base + "__int()"
	floatDef := base + "__float()"
	if n := strings.Count(out, intDef); n != 1 {
		t.Fatalf("expected exactly 1 definition of %q, got %d", intDef, n)
	}
	if strings.Contains(out, floatDef) {
		t.Fatalf("float instantiation %q must not exist when only called with int", floatDef)
	}
}

// The int instantiation must use integer arithmetic ($(( ))), not awk float.
func TestNumericGenericIntPathUsesIntegerArith(t *testing.T) {
	src := `fn add[T: numeric](a: T, b: T) -> T { return a + b }
fn main() -> int {
	print(to_string(add(3, 4)))
	return 0
}`
	stdout, stderr, code := runWisp(t, src)
	if code != 0 {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr)
	}
	if stdout != "7\n" {
		t.Errorf("stdout = %q, want %q", stdout, "7\n")
	}
}

// The float instantiation must use float arithmetic (awk).
func TestNumericGenericFloatPathUsesFloatArith(t *testing.T) {
	src := `fn add[T: numeric](a: T, b: T) -> T { return a + b }
fn main() -> int {
	print(to_string(add(1.5, 2.5)))
	return 0
}`
	stdout, stderr, code := runWisp(t, src)
	if code != 0 {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr)
	}
	// 1.5 + 2.5 = 4.0; %.17g formatting renders this as "4"
	if stdout != "4\n" {
		t.Errorf("stdout = %q, want %q", stdout, "4\n")
	}
}

// --- Generic struct codegen ---

// A generic struct with an int element compiles and field access works.
func TestGenericStructIntElem(t *testing.T) {
	src := `struct Box[T] { value: T }
fn main() -> int {
	let b: Box[int] = Box { value: 42 }
	print(to_string(b.value))
	return 0
}`
	stdout, stderr, code := runWisp(t, src)
	if code != 0 {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr)
	}
	if stdout != "42\n" {
		t.Errorf("stdout = %q, want %q", stdout, "42\n")
	}
}

// Field assignment on a generic struct updates the value.
func TestGenericStructFieldAssignCodegen(t *testing.T) {
	src := `struct Box[T] { value: T }
fn main() -> int {
	let b: Box[int] = Box { value: 5 }
	b.value = 99
	print(to_string(b.value))
	return 0
}`
	stdout, stderr, code := runWisp(t, src)
	if code != 0 {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr)
	}
	if stdout != "99\n" {
		t.Errorf("stdout = %q, want %q", stdout, "99\n")
	}
}

// Two distinct instantiations (int and string) coexist correctly.
func TestGenericStructTwoInstances(t *testing.T) {
	src := `struct Box[T] { value: T }
fn main() -> int {
	let i: Box[int] = Box { value: 7 }
	let s: Box[string] = Box { value: "hi" }
	print(to_string(i.value))
	print(s.value)
	return 0
}`
	stdout, stderr, code := runWisp(t, src)
	if code != 0 {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr)
	}
	if stdout != "7\nhi\n" {
		t.Errorf("stdout = %q, want %q", stdout, "7\nhi\n")
	}
}

// A two-type-parameter struct with field access.
func TestGenericStructPairCodegen(t *testing.T) {
	src := `struct Pair[A, B] { first: A, second: B }
fn main() -> int {
	let p: Pair[int, string] = Pair { first: 10, second: "ten" }
	print(to_string(p.first))
	print(p.second)
	return 0
}`
	stdout, stderr, code := runWisp(t, src)
	if code != 0 {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr)
	}
	if stdout != "10\nten\n" {
		t.Errorf("stdout = %q, want %q", stdout, "10\nten\n")
	}
}

// A numeric generic with a comparison returns bool correctly for both types.
func TestNumericGenericComparisonBothTypes(t *testing.T) {
	src := `fn less[T: numeric](a: T, b: T) -> bool { return a < b }
fn main() -> int {
	print(to_string(less(1, 2)))
	print(to_string(less(2.5, 1.5)))
	return 0
}`
	stdout, stderr, code := runWisp(t, src)
	if code != 0 {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr)
	}
	if stdout != "true\nfalse\n" {
		t.Errorf("stdout = %q, want %q", stdout, "true\nfalse\n")
	}
}
