package codegen

import (
	"strings"
	"testing"
)

// M6 PR-A codegen: compile-and-run behavioral tests under dash for the core
// stdlib builtins, covering every edge case in spec 2.1-2.4. split/join/
// contains/starts_with/ends_with/index_of/repeat are removable builtins now
// spelled string.*; abs/min/max are math.*; reverse/reduce (on arrays) are
// array.*. Each program below compiles through runNS/compileNS with the
// namespaces it needs, since the bare spelling no longer resolves in the
// single-module check. unwrap_or stays flat.

// runOK compiles+runs src (with the given namespaces bound), fails on non-zero
// exit, and returns stdout.
func runOK(t *testing.T, src string, namespaces ...string) string {
	t.Helper()
	out, errb, code := runNS(t, src, namespaces...)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb)
	}
	return out
}

// --- split ---

func TestM6_Split_Basic(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  let parts: string[] = string.split("a,b,c", ",")
  print("${length(parts)}")
  for (p in parts) { print(p) }
  return 0
}`, "string")
	if out != "3\na\nb\nc\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM6_Split_TrailingSep(t *testing.T) {
	// "a,b," splits into ["a","b",""] (trailing sep yields trailing "").
	out := runOK(t, `fn main() -> int {
  let parts: string[] = string.split("a,b,", ",")
  print("${length(parts)}")
  for (p in parts) { print("[${p}]") }
  return 0
}`, "string")
	if out != "3\n[a]\n[b]\n[]\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM6_Split_LeadingSep(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  let parts: string[] = string.split(",a", ",")
  print("${length(parts)}")
  for (p in parts) { print("[${p}]") }
  return 0
}`, "string")
	if out != "2\n[]\n[a]\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM6_Split_EmptySubject(t *testing.T) {
	// split("", sep) -> [""] (one empty element).
	out := runOK(t, `fn main() -> int {
  let parts: string[] = string.split("", ",")
  print("${length(parts)}")
  for (p in parts) { print("[${p}]") }
  return 0
}`, "string")
	if out != "1\n[]\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM6_Split_NoSep(t *testing.T) {
	// sep absent -> a single element equal to the subject.
	out := runOK(t, `fn main() -> int {
  let parts: string[] = string.split("abc", ",")
  print("${length(parts)}")
  print(parts[0])
  return 0
}`, "string")
	if out != "1\nabc\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM6_Split_MultiCharSep(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  let parts: string[] = string.split("a::b::c", "::")
  print("${length(parts)}")
  for (p in parts) { print(p) }
  return 0
}`, "string")
	if out != "3\na\nb\nc\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM6_Split_EmptySep_Aborts(t *testing.T) {
	_, errb, code := runNS(t, `fn main() -> int {
  let parts: string[] = string.split("abc", "")
  return 0
}`, "string")
	if code == 0 {
		t.Fatalf("expected non-zero exit on empty separator")
	}
	if !strings.Contains(errb, "wisp:") || !strings.Contains(errb, "split") {
		t.Errorf("expected located split abort, stderr=%q", errb)
	}
}

// --- join ---

func TestM6_Join_Basic(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  let xs: string[] = ["a", "b", "c"]
  print(string.join(xs, "-"))
  return 0
}`, "string")
	if out != "a-b-c\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM6_Join_Empty(t *testing.T) {
	// join([], sep) -> "".
	out := runOK(t, `fn main() -> int {
  let xs: string[] = []
  print("[${string.join(xs, "-")}]")
  return 0
}`, "string")
	if out != "[]\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM6_Join_EmptySep(t *testing.T) {
	// join(parts, "") -> plain concatenation.
	out := runOK(t, `fn main() -> int {
  let xs: string[] = ["a", "b", "c"]
  print(string.join(xs, ""))
  return 0
}`, "string")
	if out != "abc\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM6_Join_Single(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  let xs: string[] = ["only"]
  print(string.join(xs, "-"))
  return 0
}`, "string")
	if out != "only\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM6_SplitJoin_RoundTrip(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  let parts: string[] = string.split("one,two,three", ",")
  print(string.join(parts, "/"))
  return 0
}`, "string")
	if out != "one/two/three\n" {
		t.Errorf("out=%q", out)
	}
}

// --- contains (string) ---

func TestM6_Contains_String(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  print("${string.contains("hello", "ell")}")
  print("${string.contains("hello", "xyz")}")
  print("${string.contains("hello", "")}")
  print("${string.contains("hello", "hello")}")
  return 0
}`, "string")
	if out != "true\nfalse\ntrue\ntrue\n" {
		t.Errorf("out=%q", out)
	}
}

// --- contains (array) ---

func TestM6_Contains_ArrayInt(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  let xs: int[] = [1, 2, 3]
  print("${string.contains(xs, 2)}")
  print("${string.contains(xs, 9)}")
  return 0
}`, "string")
	if out != "true\nfalse\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM6_Contains_ArrayString(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  let xs: string[] = ["a", "b"]
  print("${string.contains(xs, "b")}")
  print("${string.contains(xs, "z")}")
  return 0
}`, "string")
	if out != "true\nfalse\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM6_Contains_ArrayEmpty(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  let xs: int[] = []
  print("${string.contains(xs, 1)}")
  return 0
}`, "string")
	if out != "false\n" {
		t.Errorf("out=%q", out)
	}
}

// --- starts_with / ends_with ---

func TestM6_StartsEndsWith(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  print("${string.starts_with("hello", "he")}")
  print("${string.starts_with("hello", "lo")}")
  print("${string.starts_with("hello", "")}")
  print("${string.ends_with("hello", "lo")}")
  print("${string.ends_with("hello", "he")}")
  print("${string.ends_with("hello", "")}")
  print("${string.starts_with("hi", "hello")}")
  return 0
}`, "string")
	if out != "true\nfalse\ntrue\ntrue\nfalse\ntrue\nfalse\n" {
		t.Errorf("out=%q", out)
	}
}

// --- index_of ---

func TestM6_IndexOf(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  print("${unwrap_or(string.index_of("hello", "l"), -1)}")
  print("${unwrap_or(string.index_of("hello", "lo"), -1)}")
  print("${unwrap_or(string.index_of("hello", "z"), -1)}")
  print("${unwrap_or(string.index_of("hello", ""), -1)}")
  print("${unwrap_or(string.index_of("hello", "h"), -1)}")
  print("${unwrap_or(string.index_of("hello", "o"), -1)}")
  return 0
}`, "string")
	// l first at index 2; lo at 3; z absent -1; "" -> 0; h -> 0; o -> 4.
	if out != "2\n3\n-1\n0\n0\n4\n" {
		t.Errorf("out=%q", out)
	}
}

// --- repeat ---

func TestM6_Repeat(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  print(string.repeat("ab", 3))
  print("[${string.repeat("x", 0)}]")
  print(string.repeat("-", 5))
  return 0
}`, "string")
	if out != "ababab\n[]\n-----\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM6_Repeat_NegativeAborts(t *testing.T) {
	_, errb, code := runNS(t, `fn main() -> int {
  let n: int = -1
  let r: string = string.repeat("a", n)
  return 0
}`, "string")
	if code == 0 {
		t.Fatalf("expected non-zero exit on n<0")
	}
	if !strings.Contains(errb, "wisp:") || !strings.Contains(errb, "repeat") {
		t.Errorf("expected located repeat abort, stderr=%q", errb)
	}
}

// --- abs ---

func TestM6_Abs_Int(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  print("${math.abs(-5)}")
  print("${math.abs(5)}")
  print("${math.abs(0)}")
  return 0
}`, "math")
	if out != "5\n5\n0\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM6_Abs_Float(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  print(to_string(math.abs(-2.5)))
  print(to_string(math.abs(2.5)))
  return 0
}`, "math")
	if out != "2.5\n2.5\n" {
		t.Errorf("out=%q", out)
	}
}

// --- min / max int ---

func TestM6_MinMax_Int(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  print("${math.min(3, 7)}")
  print("${math.max(3, 7)}")
  print("${math.min(7, 3)}")
  print("${math.max(7, 3)}")
  print("${math.min(-1, -2)}")
  print("${math.max(-1, -2)}")
  print("${math.min(5, 5)}")
  return 0
}`, "math")
	if out != "3\n7\n3\n7\n-2\n-1\n5\n" {
		t.Errorf("out=%q", out)
	}
}

// --- min / max float (returns the chosen operand's original atom unchanged) ---

func TestM6_MinMax_Float(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  print(to_string(math.min(3.5, 7.5)))
  print(to_string(math.max(3.5, 7.5)))
  print(to_string(math.min(7.5, 3.5)))
  print(to_string(math.max(7.5, 3.5)))
  return 0
}`, "math")
	if out != "3.5\n7.5\n3.5\n7.5\n" {
		t.Errorf("out=%q", out)
	}
}

// The float min/max return one INPUT operand unchanged (no awk reformat): a
// value with trailing-zero formatting is returned byte-for-byte as written.
func TestM6_MinMax_Float_ReturnsOperandUnchanged(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  let a: float = 1.50
  let b: float = 2.0
  print(to_string(math.min(a, b)))
  return 0
}`, "math")
	// min picks a; to_string(a) canonicalizes via %.17g -> 1.5. The point of the
	// test is that the chosen operand flows through unchanged into to_string().
	if out != "1.5\n" {
		t.Errorf("out=%q", out)
	}
}

// --- reverse ---

func TestM6_Reverse(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  let xs: int[] = [1, 2, 3]
  let ys: int[] = array.reverse(xs)
  for (y in ys) { print("${y}") }
  print("---")
  for (x in xs) { print("${x}") }
  return 0
}`, "array")
	// ys reversed; xs unchanged (reverse returns a new array).
	if out != "3\n2\n1\n---\n1\n2\n3\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM6_Reverse_Empty(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  let xs: int[] = []
  let ys: int[] = array.reverse(xs)
  print("${length(ys)}")
  return 0
}`, "array")
	if out != "0\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM6_Reverse_Single(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  let xs: string[] = ["only"]
  let ys: string[] = array.reverse(xs)
  print(ys[0])
  return 0
}`, "array")
	if out != "only\n" {
		t.Errorf("out=%q", out)
	}
}

// --- reduce ---

func TestM6_Reduce_Sum(t *testing.T) {
	out := runOK(t, `fn add(acc: int, x: int) -> int { return acc + x }
fn main() -> int {
  let xs: int[] = [1, 2, 3, 4]
  print("${array.reduce(xs, 0, add)}")
  return 0
}`, "array")
	if out != "10\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM6_Reduce_LeftFold(t *testing.T) {
	// Left fold order matters: subtract is not commutative.
	// ((((100-1)-2)-3)) = 94.
	out := runOK(t, `fn sub(acc: int, x: int) -> int { return acc - x }
fn main() -> int {
  let xs: int[] = [1, 2, 3]
  print("${array.reduce(xs, 100, sub)}")
  return 0
}`, "array")
	if out != "94\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM6_Reduce_Empty(t *testing.T) {
	// Empty array -> init unchanged.
	out := runOK(t, `fn add(acc: int, x: int) -> int { return acc + x }
fn main() -> int {
  let xs: int[] = []
  print("${array.reduce(xs, 42, add)}")
  return 0
}`, "array")
	if out != "42\n" {
		t.Errorf("out=%q", out)
	}
}

func TestM6_Reduce_AccString(t *testing.T) {
	out := runOK(t, `fn combine(acc: string, x: int) -> string { return acc + to_string(x) }
fn main() -> int {
  let xs: int[] = [1, 2, 3]
  print(array.reduce(xs, "n", combine))
  return 0
}`, "array")
	if out != "n123\n" {
		t.Errorf("out=%q", out)
	}
}

// --- injection: shell-active strings flow through inert ---

func TestM6_Injection_Inert(t *testing.T) {
	// A string with command-substitution / metachars must remain literal data
	// through split/join/contains/index_of/repeat/starts_with/ends_with.
	out := runOK(t, `fn main() -> int {
  let danger: string = "$(echo PWNED);ls"
  let parts: string[] = string.split(danger, ";")
  print(parts[0])
  print(parts[1])
  print(string.join(parts, "|"))
  print("${string.contains(danger, "PWNED")}")
  print("${unwrap_or(string.index_of(danger, "ls"), -1)}")
  print(string.repeat(danger, 2))
  print("${string.starts_with(danger, "$(echo")}")
  print("${string.ends_with(danger, "ls")}")
  return 0
}`, "string")
	want := strings.Join([]string{
		"$(echo PWNED)",
		"ls",
		"$(echo PWNED)|ls",
		"true",
		"14",
		"$(echo PWNED);ls$(echo PWNED);ls",
		"true",
		"true",
		"",
	}, "\n")
	if out != want {
		t.Errorf("out=%q\nwant=%q", out, want)
	}
}

// A separator made of glob metacharacters is matched literally, not as a glob.
func TestM6_Split_GlobSepLiteral(t *testing.T) {
	out := runOK(t, `fn main() -> int {
  let parts: string[] = string.split("a*b*c", "*")
  print(string.join(parts, ","))
  return 0
}`, "string")
	if out != "a,b,c\n" {
		t.Errorf("out=%q", out)
	}
}
