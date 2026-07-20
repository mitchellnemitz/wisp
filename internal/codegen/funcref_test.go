package codegen

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- M4: function-reference + higher-order codegen ---
//
// array.map / array.filter / array.each are removable builtins (bare map/
// filter/each no longer resolve in the single-module check), so the six
// higher-order tests below compile through compileNS/runNS with the array
// namespace bound.

func TestFuncref_IndirectEqualsDirect(t *testing.T) {
	out, errb, code := runWisp(t, `fn add(a: int, b: int) -> int { return a + b }
fn main() -> int {
  let f: fn(int, int) -> int = add
  print(to_string(add(3, 4)))
  print(to_string(f(3, 4)))
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit = %d stderr=%q", code, errb)
	}
	if out != "7\n7\n" {
		t.Errorf("out = %q, want %q", out, "7\n7\n")
	}
}

func TestFuncref_OneSourceOfTruth(t *testing.T) {
	// The funcref VALUE must be the SAME mangled name the script defines for the
	// function (one source of truth). The script both defines that function and
	// assigns the bare name as a value; assert the definition exists and the
	// value form (the bare name as the RHS of an assignment) appears.
	script := compile(t, `fn add(a: int, b: int) -> int { return a + b }
fn main() -> int {
  let f: fn(int, int) -> int = add
  return f(1, 2)
}`)
	if !bytes.Contains(script, []byte("__wisp_f_m0_add() {")) {
		t.Fatalf("expected the function definition __wisp_f_m0_add() in:\n%s", script)
	}
	// The funcref value is emitted as the bare mangled name assigned to f's local.
	if !bytes.Contains(script, []byte("=__wisp_f_m0_add\n")) {
		t.Errorf("expected the funcref value to be the bare mangled name __wisp_f_m0_add in:\n%s", script)
	}
	// The indirect call invokes the spilled name as `"$ftmp" ...`.
	if !bytes.Contains(script, []byte("\"$")) {
		t.Errorf("expected an indirect call form")
	}
}

func TestFuncref_ThroughParamReturnFieldElementDictValue(t *testing.T) {
	out, errb, code := runWisp(t, `struct Op { f: fn(int, int) -> int }
fn add(a: int, b: int) -> int { return a + b }
fn mul(a: int, b: int) -> int { return a * b }
fn getOp() -> fn(int, int) -> int { return add }
fn apply(g: fn(int, int) -> int, a: int, b: int) -> int { return g(a, b) }
fn main() -> int {
  print(to_string(apply(add, 2, 3)))
  print(to_string(getOp()(10, 20)))
  let o: Op = Op { f: mul }
  print(to_string(o.f(4, 5)))
  let fns: (fn(int, int) -> int)[] = [add, mul]
  print(to_string(fns[1](6, 7)))
  let m: {string: fn(int, int) -> int} = { "a": add }
  print(to_string(m["a"](1, 1)))
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit = %d stderr=%q", code, errb)
	}
	want := "5\n30\n20\n42\n2\n"
	if out != want {
		t.Errorf("out = %q, want %q", out, want)
	}
}

func TestHigherOrder_MapInOrder(t *testing.T) {
	out, errb, code := runNS(t, `fn dbl(x: int) -> int { return x * 2 }
fn main() -> int {
  let xs: int[] = [1, 2, 3]
  let ys: int[] = array.map(xs, dbl)
  for (y in ys) { print(to_string(y)) }
  return 0
}`, "array")
	if code != 0 {
		t.Fatalf("exit = %d stderr=%q", code, errb)
	}
	if out != "2\n4\n6\n" {
		t.Errorf("out = %q, want %q", out, "2\n4\n6\n")
	}
}

func TestHigherOrder_FilterInOrder(t *testing.T) {
	out, errb, code := runNS(t, `fn even(x: int) -> bool { return x % 2 == 0 }
fn main() -> int {
  let xs: int[] = [1, 2, 3, 4, 5, 6]
  let ys: int[] = array.filter(xs, even)
  for (y in ys) { print(to_string(y)) }
  return 0
}`, "array")
	if code != 0 {
		t.Fatalf("exit = %d stderr=%q", code, errb)
	}
	if out != "2\n4\n6\n" {
		t.Errorf("out = %q, want %q", out, "2\n4\n6\n")
	}
}

func TestHigherOrder_EachInOrder(t *testing.T) {
	out, errb, code := runNS(t, `fn show(s: string) -> void { print("got ${s}") }
fn main() -> int {
  let xs: string[] = ["a", "b", "c"]
  array.each(xs, show)
  return 0
}`, "array")
	if code != 0 {
		t.Fatalf("exit = %d stderr=%q", code, errb)
	}
	if out != "got a\ngot b\ngot c\n" {
		t.Errorf("out = %q, want %q", out, "got a\ngot b\ngot c\n")
	}
}

func TestHigherOrder_EmptyArray(t *testing.T) {
	out, errb, code := runNS(t, `fn dbl(x: int) -> int { return x * 2 }
fn even(x: int) -> bool { return x % 2 == 0 }
fn noop(x: int) -> void { print("never") }
fn main() -> int {
  let xs: int[] = []
  let m: int[] = array.map(xs, dbl)
  let f: int[] = array.filter(xs, even)
  array.each(xs, noop)
  print(to_string(length(m)))
  print(to_string(length(f)))
  return 0
}`, "array")
	if code != 0 {
		t.Fatalf("exit = %d stderr=%q", code, errb)
	}
	if out != "0\n0\n" {
		t.Errorf("out = %q, want %q", out, "0\n0\n")
	}
}

func TestHigherOrder_MapTransformType(t *testing.T) {
	// map over int[] with fn(int)->string yields string[].
	out, errb, code := runNS(t, `fn label(x: int) -> string { return "n${x}" }
fn main() -> int {
  let xs: int[] = [7, 8]
  let ys: string[] = array.map(xs, label)
  for (y in ys) { print(y) }
  return 0
}`, "array")
	if code != 0 {
		t.Fatalf("exit = %d stderr=%q", code, errb)
	}
	if out != "n7\nn8\n" {
		t.Errorf("out = %q, want %q", out, "n7\nn8\n")
	}
}

// TestFuncref_NoAllocWhenNoAggregateOrMap asserts tree-shaking: a program that
// uses only function references (no array/dict/struct, no map/filter) emits no
// __wisp_alloc (the result-array runtime is the only tree-shakeable dependency
// of the higher-order constructs).
func TestFuncref_NoAllocWhenNoAggregateOrMap(t *testing.T) {
	script := compile(t, `fn add(a: int, b: int) -> int { return a + b }
fn main() -> int {
  let f: fn(int, int) -> int = add
  print(to_string(f(2, 3)))
  return 0
}`)
	if bytes.Contains(script, []byte("__wisp_alloc")) {
		t.Errorf("tree-shaking failed: __wisp_alloc present in a funcref-only, no-aggregate, no-map program:\n%s", script)
	}
}

// TestFuncref_DataNotCallable_SentinelAbsent proves the indirect-call safety
// property concretely: a string holding `$(touch SENTINEL)` is inert data, never
// executed. The script runs in a fresh temp directory; the SENTINEL file must
// NOT exist there afterward (and the bytes appear verbatim on stdout).
func TestFuncref_DataNotCallable_SentinelAbsent(t *testing.T) {
	dash, err := exec.LookPath("dash")
	if err != nil {
		t.Skip("dash not available")
	}
	script := compile(t, `fn main() -> int {
  let cmd: string = "$(touch SENTINEL)"
  print("data=${cmd}")
  return 0
}`)
	dir := t.TempDir()
	path := filepath.Join(dir, "out.sh")
	if err := os.WriteFile(path, script, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(dash, "out.sh")
	cmd.Dir = dir // run with cwd == the temp dir so any SENTINEL lands here
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v stderr=%q", err, errb.String())
	}
	if out.String() != "data=$(touch SENTINEL)\n" {
		t.Errorf("stdout = %q, want the command text verbatim", out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "SENTINEL")); err == nil {
		t.Fatal("SENTINEL file was created: a string was executed as a command (injection!)")
	}
}

// TestHigherOrder_MapEmitsAlloc asserts the converse: map (which builds a fresh
// result array) does emit the alloc runtime.
func TestHigherOrder_MapEmitsAlloc(t *testing.T) {
	script := compileNS(t, `fn dbl(x: int) -> int { return x * 2 }
fn main() -> int {
  let xs: int[] = [1]
  let ys: int[] = array.map(xs, dbl)
  return length(ys)
}`, "array")
	if !bytes.Contains(script, []byte("__wisp_alloc")) {
		t.Errorf("expected __wisp_alloc to be present for a map result array")
	}
}
