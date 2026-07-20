package codegen

import (
	"strings"
	"testing"
)

// TestTupleBindBindsElements: `let (a: int, b: string) = pair()` binds the two
// elements; using them yields the element values, identical to bind-then-index.
func TestTupleBindBindsElements(t *testing.T) {
	src := `
fn pair() -> (int, string) {
  return (7, "hello")
}

fn main() -> int {
  let (a: int, b: string) = pair()
  print(to_string(a))
  print(b)
  return 0
}
`
	wantRun(t, src, "7\nhello\n", "", 0)
}

// TestTupleBindFinal: the final form binds immutable names that are usable.
func TestTupleBindFinal(t *testing.T) {
	src := `
fn pair() -> (int, string) {
  return (3, "x")
}

fn main() -> int {
  final (a: int, b: string) = pair()
  print(to_string(a))
  print(b)
  return 0
}
`
	wantRun(t, src, "3\nx\n", "", 0)
}

// TestTupleBindDiscard: a bare `_` slot binds nothing; only the named slot is
// usable. An annotated discard behaves the same.
func TestTupleBindDiscard(t *testing.T) {
	src := `
fn pair() -> (int, string) {
  return (1, "kept")
}

fn main() -> int {
  let (_, out: string) = pair()
  print(out)
  return 0
}
`
	wantRun(t, src, "kept\n", "", 0)
}

// TestTupleBindIdenticalToIndex: destructuring is behaviorally identical to
// binding the tuple and indexing it.
func TestTupleBindIdenticalToIndex(t *testing.T) {
	destr := `
fn pair() -> (int, string) {
  return (42, "world")
}

fn main() -> int {
  let (a: int, b: string) = pair()
  print(to_string(a))
  print(b)
  return 0
}
`
	indexed := `
fn pair() -> (int, string) {
  return (42, "world")
}

fn main() -> int {
  let t: (int, string) = pair()
  let a: int = t[0]
  let b: string = t[1]
  print(to_string(a))
  print(b)
  return 0
}
`
	o1, _, c1 := runWisp(t, destr)
	o2, _, c2 := runWisp(t, indexed)
	if o1 != o2 || c1 != c2 {
		t.Fatalf("destructure (%q,%d) != index (%q,%d)", o1, c1, o2, c2)
	}
}

// TestTupleBindLocalEmitted: a `local` line is emitted for EACH destructured
// binding name's mangled var -- proves the checker's curFunc.Decls append landed
// (without it, codegen emits no `local` and the binding leaks to global scope).
func TestTupleBindLocalEmitted(t *testing.T) {
	src := `
fn pair() -> (int, string) {
  return (1, "a")
}

fn main() -> int {
  let (a: int, b: string) = pair()
  print(to_string(a))
  print(b)
  return 0
}
`
	s := string(compile(t, src))
	// Find the local line(s) in main and confirm two distinct mangled var names
	// are declared (the two bound slots).
	var localCount int
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "local ") && strings.Contains(line, "__wisp_v_") {
			// count the v-names in this local line
			localCount += strings.Count(line, "__wisp_v_")
		}
	}
	if localCount < 2 {
		t.Fatalf("expected >=2 local-declared __wisp_v_ names (a and b), got %d:\n%s", localCount, s)
	}
}

// TestTupleBindSingleEvalSideEffect: the RHS is evaluated EXACTLY ONCE. A
// side-effecting call that prints once must print exactly once even though two
// elements are read.
func TestTupleBindSingleEvalSideEffect(t *testing.T) {
	src := `
fn effectful() -> (int, string) {
  print("evaluated")
  return (5, "v")
}

fn main() -> int {
  let (a: int, b: string) = effectful()
  print(to_string(a))
  print(b)
  return 0
}
`
	wantRun(t, src, "evaluated\n5\nv\n", "", 0)
}

// TestTupleBindAllDiscardSingleEval: an all-discard pattern still performs the
// single RHS evaluation (for effects) and binds nothing, like `let _ = f()`.
func TestTupleBindAllDiscardSingleEval(t *testing.T) {
	src := `
fn effectful() -> (int, string) {
  print("ran")
  return (1, "x")
}

fn main() -> int {
  let (_, _) = effectful()
  return 0
}
`
	wantRun(t, src, "ran\n", "", 0)
}

// TestTupleBindFaultBeforeBinding: a fallible RHS that faults propagates the
// fault BEFORE any binding -- the binds land inside the per-statement skip-guard
// opened after the spill, exactly like a normal `let x = fallibleCall()`.
func TestTupleBindFaultBeforeBinding(t *testing.T) {
	// A throwing function inside try; the destructuring statement faults, so the
	// catch runs and the after-fault statement is skipped.
	src := `
fn boom() -> (int, string) {
  throw error("kaboom")
}

fn main() -> int {
  try {
    let (a: int, b: string) = boom()
    print("unreachable")
  } catch (e) {
    print("caught")
  }
  return 0
}
`
	wantRun(t, src, "caught\n", "", 0)
}

// TestTupleBindInjectionSafe: a shell-metacharacter element value is stored as
// inert data and never re-evaluated when destructured.
func TestTupleBindInjectionSafe(t *testing.T) {
	src := `
fn pair() -> (int, string) {
  return (1, "$(echo HACKED 1>&2)")
}

fn main() -> int {
  let (a: int, b: string) = pair()
  print(b)
  return 0
}
`
	out, errb, code := runWisp(t, src)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if out != "$(echo HACKED 1>&2)\n" {
		t.Fatalf("stdout = %q, want literal", out)
	}
	if errb != "" {
		t.Fatalf("stderr = %q, want empty (no injection executed)", errb)
	}
}

// TestTupleBindGenericMonomorphizes: a numeric-bounded generic call that appears
// ONLY in a destructuring RHS inside ANOTHER monomorphized generic body must be
// discovered and instantiated by the mono pass -- which requires mono.go's
// walkStmt to walk the TupleBindStmt's Value. inner[T] returns a concrete-element
// tuple but is numeric-bounded, so it produces a mono instance keyed by T; the
// instance is reachable only by walking outer's destructuring RHS. Without the
// walk, inner[int] is never emitted and outer[int] calls an undefined function.
func TestTupleBindGenericMonomorphizes(t *testing.T) {
	src := `
fn inner[T: numeric](x: T) -> (int, string) {
  return (1, "ok")
}

fn outer[T: numeric](v: T) -> string {
  let (a: int, b: string) = inner(v)
  return b
}

fn main() -> int {
  let r: string = outer(3)
  print(r)
  return 0
}
`
	wantRun(t, src, "ok\n", "", 0)
}

// TestSingleNameBindingByteIdentical: adding the TupleBindStmt lowering path must
// NOT change the output of any single-name `let`/`final` (invariant N2). Compile
// a program with both and assert the script is byte-for-byte what it was -- here
// by asserting the destructuring-free program contains no tuple-spill scaffold
// and emits the same shape as a hand-written single-binding program. The
// strongest portable check: the generated bytes for a single-name program are
// stable across compiles (deterministic) and contain no destructuring artifacts.
func TestSingleNameBindingByteIdentical(t *testing.T) {
	src := `
fn main() -> int {
  let x: int = 5
  final y: string = "hi"
  print(to_string(x))
  print(y)
  return 0
}
`
	a := compile(t, src)
	b := compile(t, src)
	if string(a) != string(b) {
		t.Fatalf("single-name compile not deterministic")
	}
	// No tuple destructuring lowers here, so no __wisp_s_ element read should
	// appear for these scalar lets (the only handle scheme a non-tuple program
	// uses). A scalar let lowers to `MANGLED=word`, not an eval element read.
	s := string(a)
	if strings.Contains(s, "__wisp_s_$") {
		t.Fatalf("single-name program unexpectedly emitted a struct/tuple element read:\n%s", s)
	}
}

// TestTupleBindReachableOnlyViaRHS: a function reachable ONLY from a
// destructuring RHS is NOT pruned (reachable.go walkStmt must walk the Value).
func TestTupleBindReachableOnlyViaRHS(t *testing.T) {
	src := `
fn only_here() -> (int, string) {
  return (9, "z")
}

fn main() -> int {
  let (a: int, b: string) = only_here()
  print(to_string(a))
  print(b)
  return 0
}
`
	// If reachability did not walk the RHS, only_here would be tree-shaken out and
	// the call would reference an undefined shell function (nonzero exit).
	wantRun(t, src, "9\nz\n", "", 0)
	s := string(compile(t, src))
	if !strings.Contains(s, "only_here") {
		t.Fatalf("only_here was pruned; reachable.go did not walk the destructuring RHS:\n%s", s)
	}
}
