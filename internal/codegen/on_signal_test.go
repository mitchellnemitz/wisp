package codegen

import (
	"strings"
	"testing"
)

// TestOnSignal_EmitsTrapInstall asserts the codegen contract (AC3): on_signal
// lowers to a __wisp_on_signal call passing the handler's bare mangled name and
// the validated literal signal, and the emitted OnSignal helper installs
// `trap "$1" "$2"` -- so the saved action is `trap "<mangled>" "USR1"`.
func TestOnSignal_EmitsTrapInstall(t *testing.T) {
	const src = `fn handler() -> void { print("caught") }
fn main() -> int {
  on_signal("USR1", handler)
  return 0
}`
	s := string(compile(t, src))

	if !strings.Contains(s, `__wisp_on_signal __wisp_f_m0_handler "USR1"`) {
		t.Errorf("expected the on_signal call site `__wisp_on_signal __wisp_f_m0_handler \"USR1\"`, got:\n%s", s)
	}
	if !strings.Contains(s, `trap "$1" "$2"`) {
		t.Errorf("expected the OnSignal helper to install `trap \"$1\" \"$2\"`, got:\n%s", s)
	}
	// TOTAL: no located abort, no __wisp_fail dependency pulled in by on_signal.
	if strings.Contains(s, "__wisp_fail") {
		t.Errorf("on_signal must be total: emitted shell unexpectedly references __wisp_fail:\n%s", s)
	}
}
