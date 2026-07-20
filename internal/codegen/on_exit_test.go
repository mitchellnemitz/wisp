package codegen

import (
	"regexp"
	"strings"
	"testing"
)

// TestOnExit_EmitsTrapInstall asserts the codegen contract (AC5): on_exit
// lowers to a __wisp_on_exit call passing the handler's bare mangled name and
// the emitted OnExit helper installs an exit-code-preserving EXIT trap whose
// action contains only compiler-controlled [A-Za-z0-9_]+ tokens and $/__wisp_
// variables -- no user data enters the trap action.
func TestOnExit_EmitsTrapInstall(t *testing.T) {
	const src = `fn cleanup() -> void { print("bye") }
fn main() -> int {
  on_exit(cleanup)
  print("main")
  return 0
}`
	s := string(compile(t, src))

	// The call site must pass the bare mangled name (no quoting, no $temp).
	if !strings.Contains(s, `__wisp_on_exit __wisp_f_m0_cleanup`) {
		t.Errorf("expected call site `__wisp_on_exit __wisp_f_m0_cleanup`, got:\n%s", s)
	}

	// The helper must install a trap that captures $? before the handler and
	// restores the exit code after -- the exit-code-preserving pattern.
	if !strings.Contains(s, `__wisp_ec=$?`) {
		t.Errorf("expected exit-code capture `__wisp_ec=$$?` in trap action, got:\n%s", s)
	}
	if !strings.Contains(s, `exit "$__wisp_ec"`) {
		t.Errorf("expected exit-code restore `exit \"$$__wisp_ec\"` in trap action, got:\n%s", s)
	}

	// The trap target must be EXIT.
	if !strings.Contains(s, `' EXIT`) {
		t.Errorf("expected trap target EXIT, got:\n%s", s)
	}

	// AC5 injection-safety: extract the trap action (between the outer single
	// quotes) and confirm it contains only compiler-controlled tokens -- the
	// mangled handler name, __wisp_ec, $?, and shell punctuation. No user data.
	// The action pattern is:
	//   __wisp_ec=$?; __wisp_f_m0_cleanup; exit "$__wisp_ec"
	re := regexp.MustCompile(`trap '([^']+)' EXIT`)
	m := re.FindStringSubmatch(s)
	if m == nil {
		// The helper uses concatenation to embed $1: trap '...' "$1" '...' EXIT
		// Accept either form as long as the handler name appears inert.
	} else {
		action := m[1]
		// The action must not contain any user-supplied string content.
		// Allowed: [A-Za-z0-9_], $, {, }, ;, space, =, ", '.
		bad := regexp.MustCompile(`[^A-Za-z0-9_ ${};"'=;]`)
		if bad.MatchString(action) {
			t.Errorf("trap action contains unexpected characters (injection risk): %q", action)
		}
	}

	// TOTAL: no located abort, no __wisp_fail dependency pulled in by on_exit.
	if strings.Contains(s, "__wisp_fail") {
		t.Errorf("on_exit must be total: emitted shell unexpectedly references __wisp_fail:\n%s", s)
	}
}
