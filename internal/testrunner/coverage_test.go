package testrunner_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/testrunner"
)

// covTest exercises PART of the code: classify() is called with 1 (the `> 0`
// branch is taken, the `else` branch line is not). Coverage must report the
// never-taken branch line as uncovered and the executed lines as covered.
const covTest = `fn classify(n: int) -> string {
  if (n > 0) {
    return "pos"
  } else {
    return "nonpos"
  }
}

test ("classify positive") {
  assert_eq(classify(1), "pos")
}
`

// TestCoverageReport verifies `wisp test --coverage` reports per-file
// covered/total + % and the correct uncovered line numbers: the executed lines
// are covered; the never-taken else-branch body line is uncovered. Requires real
// shells (the instrumented script must run and append to COVFILE).
func TestCoverageReport(t *testing.T) {
	shells := testrunner.AvailableShells()
	if len(shells) == 0 {
		t.Skip("no shells available; cannot run coverage test")
	}

	dir := t.TempDir()
	writeFile(t, dir, "classify_test.wisp", covTest)

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:     dir,
		Coverage: true,
		Stdout:   &out,
		Stderr:   &out,
	}
	code := testrunner.Run(opts)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\noutput:\n%s", code, out.String())
	}
	s := out.String()
	// The coverage section is deterministic (golden-style). The universe is lines
	// 2 (if), 3 (return "pos"), 5 (return "nonpos"), 10 (assert_eq). Only line 5
	// (the never-taken else body) is uncovered, regardless of shell; the report is
	// the cross-shell union, so it is identical no matter how many shells ran.
	const want = "--- coverage ---\n" +
		"classify_test.wisp: 3/4 (75%)\n" +
		"      uncovered: 5\n"
	if !strings.Contains(s, want) {
		t.Fatalf("coverage report mismatch.\nwant block:\n%s\ngot:\n%s", want, s)
	}
}

// covLib is a library with a called function and an UNCALLED export fn. The
// uncalled function is tree-shaken out of codegen, but its body line must still
// be in the coverage universe (derived from source) and reported UNCOVERED.
const covLib = `export fn used_fn(x: int) -> int {
  return x + 1
}

export fn never_called(x: int) -> int {
  return x + 2
}
`

const covLibTest = `include "./lib.wisp" as lib

test ("uses used_fn") {
  assert_eq(lib.used_fn(1), 2)
}
`

// TestCoverageUncalledFunctionReportsUncovered verifies that a `wisp test
// --coverage` over a test file that includes a library with a called used_fn
// and an UNCALLED never_called reports the library as covered<total with
// never_called's body line in the UNCOVERED list, NOT 100%. Before the
// source-walk universe, the tree-shaken never_called vanished and the file
// misreported 1/1 (100%).
func TestCoverageUncalledFunctionReportsUncovered(t *testing.T) {
	shells := testrunner.AvailableShells()
	if len(shells) == 0 {
		t.Skip("no shells available; cannot run coverage test")
	}

	dir := t.TempDir()
	writeFile(t, dir, "lib.wisp", covLib)
	writeFile(t, dir, "lib_test.wisp", covLibTest)

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:     dir,
		Coverage: true,
		Stdout:   &out,
		Stderr:   &out,
	}
	code := testrunner.Run(opts)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\noutput:\n%s", code, out.String())
	}
	s := out.String()
	// lib.wisp universe: line 2 (used_fn body, hit) and line 6 (never_called
	// body, never hit). The uncalled function reports uncovered, so the file is
	// 1/2 (50%) with line 6 uncovered -- not 100%.
	const want = "lib.wisp: 1/2 (50%)\n" +
		"      uncovered: 6\n"
	if !strings.Contains(s, want) {
		t.Fatalf("coverage report mismatch.\nwant block:\n%s\ngot:\n%s", want, s)
	}
}

// TestCoverageOffNoSection verifies that without --coverage there is no coverage
// section in the output (coverage off = today's behavior).
func TestCoverageOffNoSection(t *testing.T) {
	shells := testrunner.AvailableShells()
	if len(shells) == 0 {
		t.Skip("no shells available")
	}
	dir := t.TempDir()
	writeFile(t, dir, "classify_test.wisp", covTest)

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:   dir,
		Stdout: &out,
		Stderr: &out,
	}
	testrunner.Run(opts)
	if strings.Contains(out.String(), "--- coverage ---") {
		t.Fatalf("coverage section present without --coverage:\n%s", out.String())
	}
}
