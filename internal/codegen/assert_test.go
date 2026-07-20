package codegen

import (
	"strings"
	"testing"
)

// Task 2: codegen + runtime for the assertion + skip builtins. The exit-code
// contract (122 fail / 121 skip / 0 pass) is the runner interface, so these
// tests pin it directly. Cross-shell + injection-safety is proved by the golden
// fixtures; these are the dash-level unit checks plus tree-shaking.

func TestAssert_TruePass_NoOp(t *testing.T) {
	out, errb, code := runWisp(t, `fn main() -> int {
  assert(true)
  print("after")
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit = %d stderr=%q", code, errb)
	}
	if out != "after\n" {
		t.Fatalf("stdout = %q (assert(true) must be a no-op and not halt)", out)
	}
}

func TestAssert_FalseExits122(t *testing.T) {
	out, errb, code := runWisp(t, `fn main() -> int {
  assert(false, "boom")
  print("unreached")
  return 0
}`)
	if code != 122 {
		t.Fatalf("exit = %d (want 122) stderr=%q", code, errb)
	}
	if out != "" {
		t.Fatalf("stdout = %q (body after a failed assert must not run)", out)
	}
	if !strings.Contains(errb, "assertion failed: boom") {
		t.Fatalf("stderr = %q (want located assertion message with the user msg)", errb)
	}
}

func TestAssert_FalseNoMsg_Exits122(t *testing.T) {
	_, errb, code := runWisp(t, `fn main() -> int {
  assert(false)
  return 0
}`)
	if code != 122 {
		t.Fatalf("exit = %d (want 122) stderr=%q", code, errb)
	}
	if !strings.Contains(errb, "assertion failed") {
		t.Fatalf("stderr = %q (want bare 'assertion failed')", errb)
	}
}

func TestAssertEq_Mismatch_Exits122_RendersBoth(t *testing.T) {
	_, errb, code := runWisp(t, `fn main() -> int {
  assert_eq(1, 2)
  return 0
}`)
	if code != 122 {
		t.Fatalf("exit = %d (want 122) stderr=%q", code, errb)
	}
	if !strings.Contains(errb, "1 != 2") {
		t.Fatalf("stderr = %q (want debug-rendered '1 != 2')", errb)
	}
}

func TestAssertEq_Match_NoOp(t *testing.T) {
	out, errb, code := runWisp(t, `fn main() -> int {
  assert_eq("x", "x")
  print("ok")
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit = %d stderr=%q", code, errb)
	}
	if out != "ok\n" {
		t.Fatalf("stdout = %q", out)
	}
}

func TestAssertNe_Equal_Exits122(t *testing.T) {
	_, errb, code := runWisp(t, `fn main() -> int {
  assert_ne(3, 3)
  return 0
}`)
	if code != 122 {
		t.Fatalf("exit = %d (want 122) stderr=%q", code, errb)
	}
	if !strings.Contains(errb, "3 == 3") {
		t.Fatalf("stderr = %q (want '3 == 3')", errb)
	}
}

func TestAssertSome_OnNone_Exits122(t *testing.T) {
	_, errb, code := runWisp(t, `fn main() -> int {
  let o: Optional[int] = None
  assert_some(o)
  return 0
}`)
	if code != 122 {
		t.Fatalf("exit = %d (want 122) stderr=%q", code, errb)
	}
	if !strings.Contains(errb, "None") {
		t.Fatalf("stderr = %q (want the actual value 'None' rendered)", errb)
	}
}

func TestAssertOk_OnErr_Exits122(t *testing.T) {
	_, errb, code := runWisp(t, `fn main() -> int {
  let r: Result[int] = Err(error("nope"))
  assert_ok(r)
  return 0
}`)
	if code != 122 {
		t.Fatalf("exit = %d (want 122) stderr=%q", code, errb)
	}
	if !strings.Contains(errb, "nope") {
		t.Fatalf("stderr = %q (want the actual Err value rendered)", errb)
	}
}

func TestAssertErr_OnOk_Exits122(t *testing.T) {
	_, errb, code := runWisp(t, `fn main() -> int {
  let r: Result[int] = Ok(7)
  assert_err(r)
  return 0
}`)
	if code != 122 {
		t.Fatalf("exit = %d (want 122) stderr=%q", code, errb)
	}
	if !strings.Contains(errb, "7") {
		t.Fatalf("stderr = %q (want the actual Ok value rendered)", errb)
	}
}

func TestAssertContains_StringMiss_Exits122(t *testing.T) {
	_, errb, code := runWisp(t, `fn main() -> int {
  assert_contains("hello", "zzz")
  return 0
}`)
	if code != 122 {
		t.Fatalf("exit = %d (want 122) stderr=%q", code, errb)
	}
}

func TestAssertContains_ArrayMiss_Exits122(t *testing.T) {
	_, errb, code := runWisp(t, `fn main() -> int {
  let xs: int[] = [1, 2, 3]
  assert_contains(xs, 9)
  return 0
}`)
	if code != 122 {
		t.Fatalf("exit = %d (want 122) stderr=%q", code, errb)
	}
}

func TestAssertContains_Hit_NoOp(t *testing.T) {
	out, _, code := runWisp(t, `fn main() -> int {
  assert_contains("hello", "ell")
  let xs: int[] = [1, 2, 3]
  assert_contains(xs, 2)
  print("ok")
  return 0
}`)
	if code != 0 || out != "ok\n" {
		t.Fatalf("exit = %d stdout = %q", code, out)
	}
}

func TestSkip_Exits121(t *testing.T) {
	out, errb, code := runWisp(t, `fn main() -> int {
  skip("later")
  print("unreached")
  return 0
}`)
	if code != 121 {
		t.Fatalf("exit = %d (want 121) stderr=%q", code, errb)
	}
	if out != "" {
		t.Fatalf("stdout = %q (body after skip must not run)", out)
	}
	if !strings.Contains(errb, "SKIP: later") {
		t.Fatalf("stderr = %q (want 'SKIP: later')", errb)
	}
}

// Tree-shaking: the new helpers appear ONLY when an assert/skip is used.
func TestAssertSkip_TreeShaken(t *testing.T) {
	noUse := string(compile(t, "fn main() -> int {\n  return 0\n}"))
	if strings.Contains(noUse, "__wisp_assert_fail") {
		t.Fatalf("__wisp_assert_fail emitted for a program that uses no assert")
	}
	if strings.Contains(noUse, "__wisp_skip") {
		t.Fatalf("__wisp_skip emitted for a program that uses no skip")
	}
	withAssert := string(compile(t, "fn main() -> int {\n  assert(true)\n  return 0\n}"))
	if !strings.Contains(withAssert, "__wisp_assert_fail()") {
		t.Fatalf("__wisp_assert_fail helper missing when assert is used")
	}
	if strings.Contains(withAssert, "__wisp_skip()") {
		t.Fatalf("__wisp_skip emitted when only assert is used (over-emission)")
	}
	withSkip := string(compile(t, "fn main() -> int {\n  skip(\"x\")\n  return 0\n}"))
	if !strings.Contains(withSkip, "__wisp_skip()") {
		t.Fatalf("__wisp_skip helper missing when skip is used")
	}
}
