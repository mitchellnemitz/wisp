package codegen

import (
	"strings"
	"testing"
)

// TestCauseOnlyEmitsNoThreading is AC6(b): a program that uses `cause` but NOT
// `wrap` must emit NO __wisp_err_cause throw-path threading -- only the inline
// `cause` read (which always sees an empty _cause and returns None). The
// threading is gated on uses-`wrap`, not uses-`cause`: `wrap` is the only
// producer of a cause, so a cause-only program can never carry one. This program
// uses try/throw/catch AND a fault AND cause(), exercising every gated site
// (genThrow, bindCatchVar, __wisp_fail, genTry); none may reference
// __wisp_err_cause.
func TestCauseOnlyEmitsNoThreading(t *testing.T) {
	const src = `fn main() -> int {
  try {
    throw error("boom")
  } catch (e) {
    let o: Optional[error] = cause(e)
    print(to_string(is_none(o)))
  }
  let a: int = 1
  let b: int = 0
  try {
    print(to_string(a / b))
  } catch (f) {
    let o2: Optional[error] = cause(f)
    print(to_string(is_none(o2)))
  }
  return 0
}`
	got := compile(t, src)
	if strings.Contains(string(got), "__wisp_err_cause") {
		t.Fatalf("cause-only program emitted __wisp_err_cause threading (gate should be on uses-wrap, not uses-cause):\n%s", got)
	}
}

// TestWrapEmitsThreading is the positive control: a program that DOES use `wrap`
// emits the __wisp_err_cause threading, so the gate genuinely turns it on.
func TestWrapEmitsThreading(t *testing.T) {
	const src = `fn main() -> int {
  try {
    throw wrap(error("inner"), "outer")
  } catch (e) {
    print(e.message)
  }
  return 0
}`
	got := compile(t, src)
	if !strings.Contains(string(got), "__wisp_err_cause") {
		t.Fatalf("wrap program did NOT emit __wisp_err_cause threading; gate failed to engage:\n%s", got)
	}
}
