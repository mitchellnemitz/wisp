package codegen_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/driver"
)

// TestMonoFinalInitializerCall pins the A4 fix: a numeric-bounded generic call
// that appears ONLY inside a `final` initializer (within another generic's
// body) must still be collected for monomorphization, so its concrete instance
// is emitted. Before the fix, ciWalker.walkStmt had no *ast.FinalStmt case, so
// `__wisp_f_m0_inner__int` was never generated and the `outer` instance called a
// non-existent function ("command not found" at runtime, silent wrong output).
//
// The `let` variant is the control: it exercises the already-working path and
// guards against a regression that would break it. The two programs differ only
// by the binding keyword, so a single template covers both.
func TestMonoFinalInitializerCall(t *testing.T) {
	prog := func(bind string) string {
		return fmt.Sprintf(`fn inner[T: numeric](x: T) -> T {
  return x + x
}
fn outer[T: numeric](x: T) -> T {
  %s y: T = inner(x)
  return y
}
fn main() -> int {
  print(to_string(outer(21)))
  return 0
}
`, bind)
	}

	// `final` is the fix under test; `let` is the control (must stay working).
	for _, bind := range []string{"final", "let"} {
		script, _, diags := driver.Compile("repro.wisp", prog(bind))
		for _, d := range diags {
			if d.Severity == driver.Error {
				t.Fatalf("%s: unexpected compile error: %s", bind, d)
			}
		}
		// Assert the DEFINITION exists, not just the name: the bare name also
		// appears at the (unresolved) call site inside `outer`, so a name-only
		// check would pass even pre-fix. The definition line is emitted only
		// when the instance is actually monomorphized.
		if !strings.Contains(string(script), "__wisp_f_m0_inner__int() {") {
			t.Fatalf("%s: generated script does not define the concrete instance __wisp_f_m0_inner__int; the generic call in the initializer was not monomorphized\nscript:\n%s", bind, script)
		}
		out, stderr, code := runScript(t, script)
		if code != 0 {
			t.Fatalf("%s: script exited %d; stderr=%q", bind, code, stderr)
		}
		if got := strings.TrimSpace(out); got != "42" {
			t.Fatalf("%s: stdout = %q, want %q", bind, got, "42")
		}
	}
}
