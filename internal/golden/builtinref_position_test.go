package golden

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/driver"
)

// posTripleRe matches a source-position triple in a wisp abort message:
// `:<digits>:<digits>:` (file:line:col). A funcref-abort position is the
// builtin name (e.g. "sqrt:") which contains no digits, so this regex does
// NOT false-match it.
var posTripleRe = regexp.MustCompile(`:[0-9]+:[0-9]+:`)

// TestBuiltinRef_NoPositionTriple asserts that a funcref-abort via a builtin
// funcref reports the BUILTIN NAME as the position (not a source file:line:col
// triple). It also asserts the INVERSE: a direct builtin call abort DOES
// carry a source triple, confirming that direct calls are not degraded.
func TestBuiltinRef_NoPositionTriple(t *testing.T) {
	// Funcref abort: sqrt referenced as a value, then called with -1.
	funcrefSrc := `import "math"
fn main() -> int {
  let r: fn(float)->float = math.sqrt
  print("${r(-1.0)}")
  return 0
}`
	funcrefScript, _, funcrefDiags := driver.Compile("builtinref_sqrt_abort.wisp", funcrefSrc)
	if errored(funcrefDiags) {
		t.Fatalf("unexpected compile errors for funcref source: %v", funcrefDiags)
	}

	// Direct abort: sqrt called inline with -1.
	directSrc := `import "math"
fn main() -> int {
  print("${math.sqrt(-1.0)}")
  return 0
}`
	directScript, _, directDiags := driver.Compile("num_sqrt_neg_aborts.wisp", directSrc)
	if errored(directDiags) {
		t.Fatalf("unexpected compile errors for direct source: %v", directDiags)
	}

	shells := availableShells(t)

	for _, sh := range shells {
		sh := sh
		t.Run(sh.label+"/funcref", func(t *testing.T) {
			_, errb, code := runUnder(t, sh, funcrefScript, fixtureSpec{})
			if code != 1 {
				t.Fatalf("expected exit 1 from funcref abort, got %d", code)
			}
			if !strings.Contains(errb, "sqrt") {
				t.Errorf("stderr does not mention sqrt: %q", errb)
			}
			// The funcref position is the builtin NAME ("sqrt"), not a triple.
			if posTripleRe.MatchString(errb) {
				t.Errorf("funcref abort stderr contains a source position triple (:[0-9]+:[0-9]+:); want only builtin name: %q", errb)
			}
		})
		t.Run(sh.label+"/direct", func(t *testing.T) {
			_, errb, code := runUnder(t, sh, directScript, fixtureSpec{})
			if code != 1 {
				t.Fatalf("expected exit 1 from direct abort, got %d", code)
			}
			if !strings.Contains(errb, "sqrt") {
				t.Errorf("stderr does not mention sqrt: %q", errb)
			}
			// Direct call must carry a source position triple.
			if !posTripleRe.MatchString(errb) {
				t.Errorf("direct abort stderr missing source position triple (:[0-9]+:[0-9]+:): %q", errb)
			}
		})
	}
}

// TestBuiltinRef_WrapperNotEmittedForDirectCall asserts the tree-shaking
// property: a program that calls sqrt DIRECTLY (never as a value) must not
// emit the __wisp_builtin_sqrt wrapper.
func TestBuiltinRef_WrapperNotEmittedForDirectCall(t *testing.T) {
	directSrc := `import "math"
fn main() -> int {
  print("${math.sqrt(4.0)}")
  return 0
}`
	script, _, diags := driver.Compile("direct_sqrt.wisp", directSrc)
	if errored(diags) {
		t.Fatalf("unexpected compile errors: %v", diags)
	}
	if bytes.Contains(script, []byte("__wisp_builtin_sqrt")) {
		t.Errorf("wrapper __wisp_builtin_sqrt emitted for a direct sqrt call (tree-shaking failure)")
	}
}

// TestBuiltinRef_WrapperEmittedForValueRef asserts the inverse: when sqrt is
// used as a first-class value, the wrapper IS emitted.
func TestBuiltinRef_WrapperEmittedForValueRef(t *testing.T) {
	funcrefSrc := `import "math"
fn main() -> int {
  let r: fn(float)->float = math.sqrt
  print("${r(4.0)}")
  return 0
}`
	script, _, diags := driver.Compile("funcref_sqrt.wisp", funcrefSrc)
	if errored(diags) {
		t.Fatalf("unexpected compile errors: %v", diags)
	}
	if !bytes.Contains(script, []byte("__wisp_builtin_sqrt")) {
		t.Errorf("wrapper __wisp_builtin_sqrt not emitted for a sqrt value-reference")
	}
}
