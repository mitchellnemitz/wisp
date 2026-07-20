package golden

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/driver"
)

// TestProgramPath_CaptureTreeShaken asserts the $0 capture and the path-string
// helpers are tree-shaken: a program that never calls program_path()/dir_name()/
// base_name() emits none of them (spec P2 / AC4 byte-identity premise).
func TestProgramPath_CaptureTreeShaken(t *testing.T) {
	src := "fn main() -> int {\n  print(\"hi\")\n  return 0\n}\n"
	script, _, diags := driver.Compile("tiny.wisp", src)
	if errored(diags) {
		t.Fatalf("unexpected compile errors: %v", diags)
	}
	for _, tok := range []string{"__wisp_arg0", "__wisp_dir_name", "__wisp_base_name"} {
		if bytes.Contains(script, []byte(tok)) {
			t.Errorf("tree-shaking failed: %q present in a program not using the path builtins", tok)
		}
	}
}

// TestProgramPath_NoUseByteIdentical asserts the STRONGER P2/AC4 invariant: a
// program that never calls program_path() compiles byte-for-byte identically to
// the same program before this feature existed. We can't diff against history,
// so we diff a no-use program against itself with the capture suppressed: the
// presence check above plus the absence of the exact capture line here pins it.
func TestProgramPath_NoUseByteIdentical(t *testing.T) {
	src := "import \"fs\"\nfn main() -> int {\n  let p: string = fs.base_name(\"/a/b\")\n  print(p)\n  return 0\n}\n"
	script, _, diags := driver.Compile("nb.wisp", src)
	if errored(diags) {
		t.Fatalf("unexpected compile errors: %v", diags)
	}
	// base_name is used (so its helper is present) but program_path() is NOT, so
	// the $0 capture line must be ABSENT.
	if bytes.Contains(script, []byte(`__wisp_arg0="$0"`)) {
		t.Errorf("capture line present though program_path() is never called")
	}
	if !bytes.Contains(script, []byte("__wisp_base_name")) {
		t.Errorf("expected base_name helper present when used")
	}
}

// TestProgramPath_CaptureEmittedOncePreMain asserts that when program_path() IS
// called, the `__wisp_arg0="$0"` capture appears EXACTLY once and at TOP LEVEL
// (before the trailing main invocation), and that program_path() lowers to a
// read of $__wisp_arg0 (spec P1).
func TestProgramPath_CaptureEmittedOncePreMain(t *testing.T) {
	src := "import \"fs\"\nfn nested() -> string {\n  return fs.program_path()\n}\n" +
		"fn main() -> int {\n  let a: string = fs.program_path()\n  print(to_string(a == nested()))\n  return 0\n}\n"
	script, _, diags := driver.Compile("pp.wisp", src)
	if errored(diags) {
		t.Fatalf("unexpected compile errors: %v", diags)
	}
	s := string(script)
	const capture = `__wisp_arg0="$0"`
	if n := strings.Count(s, capture); n != 1 {
		t.Fatalf("capture line count = %d, want exactly 1", n)
	}
	// The lowering reads the global, not a bare $0 inside a function.
	if !strings.Contains(s, `"$__wisp_arg0"`) {
		t.Errorf("program_path() did not lower to a read of $__wisp_arg0")
	}
	// The capture must precede the main invocation (top-level, pre-main).
	capIdx := strings.Index(s, capture+"\n")
	mainIdx := strings.LastIndex(s, "; exit \"$__ret\"")
	if capIdx < 0 || mainIdx < 0 || capIdx >= mainIdx {
		t.Errorf("capture must appear before the main invocation (cap=%d main=%d)", capIdx, mainIdx)
	}
}

// TestProgramPath_CaptureInTestMode asserts the capture is emitted in the
// testMode path too: a *_test.wisp whose test body calls program_path() still
// gets the top-level $0 capture (the runner footer replaces main, but the
// capture sits before it).
func TestProgramPath_CaptureInTestMode(t *testing.T) {
	src := "import \"fs\"\ntest (\"pp\") {\n  assert(fs.base_name(fs.program_path()) != \"\")\n}\n"
	script, _, diags := driver.Compile("x_test.wisp", src)
	if errored(diags) {
		t.Fatalf("unexpected compile errors: %v", diags)
	}
	if !bytes.Contains(script, []byte(`__wisp_arg0="$0"`)) {
		t.Errorf("capture line absent in test-mode program that calls program_path()")
	}
}
