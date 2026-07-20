package testrunner_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/testrunner"
)

// writeFile creates a file with given content in the given directory.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// simplePassTest is a minimal test file with one passing test.
const simplePassTest = `test ("a passes") {
  assert_eq(1 + 1, 2)
}
`

// simpleFailTest has one passing and one failing test.
const simpleFailTest = `test ("a passes") {
  assert_eq(1 + 1, 2)
}

test ("b fails") {
  assert_eq(1, 2)
}
`

// simpleSkipTest has only a skip.
const simpleSkipTest = `test ("a is skipped") {
  skip("not ready")
}
`

// TestDiscoveryAndExit verifies that Run discovers *_test.wisp recursively in
// sorted order, runs them, and exits 0 iff all tests pass/skip (AC1, AC10).
func TestDiscoveryAndExit(t *testing.T) {
	shells := testrunner.AvailableShells()
	if len(shells) == 0 {
		t.Skip("no shells available; cannot run testrunner tests")
	}

	dir := t.TempDir()
	writeFile(t, dir, "a_test.wisp", simplePassTest)
	writeFile(t, dir, "sub/b_test.wisp", simpleSkipTest)

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:   dir,
		Stdout: &out,
		Stderr: &out,
	}
	code := testrunner.Run(opts)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\noutput:\n%s", code, out.String())
	}
	// Output must mention both files (sorted order: a_test.wisp before sub/b_test.wisp).
	s := out.String()
	if !strings.Contains(s, "a_test.wisp") {
		t.Errorf("output missing a_test.wisp: %q", s)
	}
	if !strings.Contains(s, "b_test.wisp") {
		t.Errorf("output missing b_test.wisp: %q", s)
	}
}

// TestExitNonzeroOnFailure verifies exit != 0 when a test fails.
func TestExitNonzeroOnFailure(t *testing.T) {
	shells := testrunner.AvailableShells()
	if len(shells) == 0 {
		t.Skip("no shells available")
	}

	dir := t.TempDir()
	writeFile(t, dir, "fail_test.wisp", simpleFailTest)

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:   dir,
		Stdout: &out,
		Stderr: &out,
	}
	code := testrunner.Run(opts)
	if code == 0 {
		t.Fatalf("expected non-zero exit, got 0\noutput:\n%s", out.String())
	}
}

// TestTAPFlag verifies --tap emits raw TAP-13.
func TestTAPFlag(t *testing.T) {
	shells := testrunner.AvailableShells()
	if len(shells) == 0 {
		t.Skip("no shells available")
	}

	dir := t.TempDir()
	writeFile(t, dir, "tap_test.wisp", simplePassTest)

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:   dir,
		TAP:    true,
		Stdout: &out,
		Stderr: &out,
	}
	code := testrunner.Run(opts)
	if code != 0 {
		t.Fatalf("exit=%d, want 0\noutput:\n%s", code, out.String())
	}
	s := out.String()
	if !strings.HasPrefix(s, "TAP version 13") {
		t.Errorf("expected TAP version 13 header, got: %q", s)
	}
	if !strings.Contains(s, "1..") {
		t.Errorf("expected plan line, got: %q", s)
	}
}

// TestFilterFlag verifies --filter selects tests by name regex.
func TestFilterFlag(t *testing.T) {
	shells := testrunner.AvailableShells()
	if len(shells) == 0 {
		t.Skip("no shells available")
	}

	dir := t.TempDir()
	writeFile(t, dir, "filter_test.wisp", simpleFailTest) // "a passes" and "b fails"

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:   dir,
		Filter: "a passes", // only select the passing test
		Stdout: &out,
		Stderr: &out,
	}
	code := testrunner.Run(opts)
	if code != 0 {
		t.Fatalf("expected exit 0 with filter selecting only passing test, got %d\noutput:\n%s", code, out.String())
	}
}

// TestShellRestriction verifies --shell restricts to the named shell.
func TestShellRestriction(t *testing.T) {
	shells := testrunner.AvailableShells()
	if len(shells) == 0 {
		t.Skip("no shells available")
	}

	dir := t.TempDir()
	writeFile(t, dir, "shell_test.wisp", simplePassTest)

	// Use the first available shell.
	shellName := shells[0].Label

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:      dir,
		ShellOnly: shellName,
		Stdout:    &out,
		Stderr:    &out,
	}
	code := testrunner.Run(opts)
	if code != 0 {
		t.Fatalf("exit=%d, want 0\noutput:\n%s", code, out.String())
	}
}

// TestZeroShellsError verifies that when no shells are available, Run returns
// non-zero (AC17).
func TestZeroShellsError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "noshell_test.wisp", simplePassTest)

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:      dir,
		ShellOnly: "no-such-shell-xyzzy", // will match nothing
		Stdout:    &out,
		Stderr:    &out,
	}
	code := testrunner.Run(opts)
	if code == 0 {
		t.Fatal("expected non-zero exit when zero shells available, got 0")
	}
}

// TestBadFilterRegex verifies that an invalid regex is a usage error (exit 2).
func TestBadFilterRegex(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "re_test.wisp", simplePassTest)

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:   dir,
		Filter: "[invalid",
		Stdout: &out,
		Stderr: &out,
	}
	code := testrunner.Run(opts)
	if code != 2 {
		t.Fatalf("expected exit 2 for invalid regex, got %d", code)
	}
}

// TestSingleFileArg verifies that passing a single *_test.wisp file directly works.
func TestSingleFileArg(t *testing.T) {
	shells := testrunner.AvailableShells()
	if len(shells) == 0 {
		t.Skip("no shells available")
	}

	dir := t.TempDir()
	p := writeFile(t, dir, "single_test.wisp", simplePassTest)

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:   p,
		Stdout: &out,
		Stderr: &out,
	}
	code := testrunner.Run(opts)
	if code != 0 {
		t.Fatalf("exit=%d, want 0\noutput:\n%s", code, out.String())
	}
}

// TestNoTestFiles verifies that a directory with no *_test.wisp files exits 0.
func TestNoTestFiles(t *testing.T) {
	dir := t.TempDir()
	// No test files at all.

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:   dir,
		Stdout: &out,
		Stderr: &out,
	}
	code := testrunner.Run(opts)
	if code != 0 {
		t.Fatalf("expected exit 0 with no test files, got %d\noutput:\n%s", code, out.String())
	}
}

// TestSortedOrder verifies files are discovered in sorted path order.
func TestSortedOrder(t *testing.T) {
	shells := testrunner.AvailableShells()
	if len(shells) == 0 {
		t.Skip("no shells available")
	}

	dir := t.TempDir()
	writeFile(t, dir, "z_test.wisp", simplePassTest)
	writeFile(t, dir, "a_test.wisp", simplePassTest)
	writeFile(t, dir, "m_test.wisp", simplePassTest)

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:   dir,
		Stdout: &out,
		Stderr: &out,
	}
	code := testrunner.Run(opts)
	if code != 0 {
		t.Fatalf("exit=%d\noutput:\n%s", code, out.String())
	}
	s := out.String()
	aIdx := strings.Index(s, "a_test.wisp")
	mIdx := strings.Index(s, "m_test.wisp")
	zIdx := strings.Index(s, "z_test.wisp")
	if aIdx < 0 || mIdx < 0 || zIdx < 0 {
		t.Fatalf("missing files in output: %q", s)
	}
	if !(aIdx < mIdx && mIdx < zIdx) {
		t.Errorf("files not in sorted order: a=%d m=%d z=%d in %q", aIdx, mIdx, zIdx, s)
	}
}

// makeFakeShell writes a shell script that emits fixed TAP output and returns a
// Shell whose Bin is "/bin/sh" and whose first arg is the script path.
// The script is created in t.TempDir() and is executable.
func makeFakeShell(t *testing.T, label, tapOutput string) testrunner.Shell {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "fake-runner.sh")
	// The script ignores its argument and just prints the fixed TAP output.
	content := "#!/bin/sh\nprintf '%s' " + "'" + tapOutput + "'\n"
	if err := os.WriteFile(scriptPath, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return testrunner.Shell{
		Label: label,
		Bin:   "/bin/sh",
		Args:  []string{scriptPath},
	}
}

// TestIncompleteTAPAlwaysErrors verifies that a shell run whose TAP output
// declares a plan of 3 but only emits 2 result lines is classified as an ERROR
// and causes a nonzero exit -- even under a --filter that would otherwise select
// only a passing subset.
func TestIncompleteTAPAlwaysErrors(t *testing.T) {
	shells := testrunner.AvailableShells()
	if len(shells) == 0 {
		t.Skip("no shells available")
	}

	// Build a fake shell that emits 1..3 but only 2 result lines.
	incompleteTAP := "TAP version 13\n1..3\nok 1 - passes\nok 2 - also passes\n"
	fakeShell := makeFakeShell(t, "fake-incomplete", incompleteTAP)

	dir := t.TempDir()
	writeFile(t, dir, "incomplete_test.wisp", simplePassTest)

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:       dir,
		Filter:     "passes", // filter selects only passing tests
		FakeShells: []testrunner.Shell{fakeShell},
		Stdout:     &out,
		Stderr:     &out,
	}
	code := testrunner.Run(opts)
	if code == 0 {
		t.Fatalf("expected nonzero exit for incomplete TAP even under filter, got 0\noutput:\n%s", out.String())
	}
}

// TestIncompleteTAPReported verifies that a (file, shell) with incomplete TAP
// is reported in the summary naming the file, shell, and reason.
func TestIncompleteTAPReported(t *testing.T) {
	shells := testrunner.AvailableShells()
	if len(shells) == 0 {
		t.Skip("no shells available")
	}

	incompleteTAP := "TAP version 13\n1..3\nok 1 - passes\nok 2 - also passes\n"
	fakeShell := makeFakeShell(t, "fake-incomplete", incompleteTAP)

	dir := t.TempDir()
	writeFile(t, dir, "reported_test.wisp", simplePassTest)

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:       dir,
		FakeShells: []testrunner.Shell{fakeShell},
		Stdout:     &out,
		Stderr:     &out,
	}
	testrunner.Run(opts)
	s := out.String()
	if !strings.Contains(s, "reported_test.wisp") {
		t.Errorf("output does not name the file: %q", s)
	}
	if !strings.Contains(s, "fake-incomplete") {
		t.Errorf("output does not name the shell: %q", s)
	}
	if !strings.Contains(s, "incomplete TAP") {
		t.Errorf("output does not mention incomplete TAP: %q", s)
	}
}

// TestFilterSuppressesNormalFailure is a regression guard: under --filter that
// selects only passing tests, a run where OTHER tests fail (but TAP is complete)
// still exits 0. This must not be broken by the completeness check.
func TestFilterSuppressesNormalFailure(t *testing.T) {
	shells := testrunner.AvailableShells()
	if len(shells) == 0 {
		t.Skip("no shells available")
	}

	// Complete TAP with 2 results: one pass, one fail. The filter selects "a passes".
	completeTAP := "TAP version 13\n1..2\nok 1 - a passes\nnot ok 2 - b fails\n"
	fakeShell := makeFakeShell(t, "fake-complete", completeTAP)

	dir := t.TempDir()
	writeFile(t, dir, "suppress_test.wisp", simpleFailTest)

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:       dir,
		Filter:     "a passes",
		FakeShells: []testrunner.Shell{fakeShell},
		Stdout:     &out,
		Stderr:     &out,
	}
	code := testrunner.Run(opts)
	if code != 0 {
		t.Fatalf("expected exit 0 when filter selects only passing test and TAP is complete, got %d\noutput:\n%s", code, out.String())
	}
}

// TestTAPIncompleteExitsNonzero verifies that under --tap, a (file, shell) run
// with incomplete TAP (plan of 3 but only 2 ok lines) returns nonzero and prints
// a diagnostic -- the exit code must agree with the summary path.
func TestTAPIncompleteExitsNonzero(t *testing.T) {
	shells := testrunner.AvailableShells()
	if len(shells) == 0 {
		t.Skip("no shells available")
	}

	// 1..3 plan but only 2 result lines, both ok.
	incompleteTAP := "TAP version 13\n1..3\nok 1 - passes\nok 2 - also passes\n"
	fakeShell := makeFakeShell(t, "fake-incomplete", incompleteTAP)

	dir := t.TempDir()
	writeFile(t, dir, "taptrunc_test.wisp", simplePassTest)

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:       dir,
		TAP:        true,
		FakeShells: []testrunner.Shell{fakeShell},
		Stdout:     &out,
		Stderr:     &out,
	}
	code := testrunner.Run(opts)
	if code == 0 {
		t.Fatalf("expected nonzero exit for incomplete TAP under --tap, got 0\noutput:\n%s", out.String())
	}
	s := out.String()
	if !strings.Contains(s, "not ok") {
		t.Errorf("expected a not-ok diagnostic line, got: %q", s)
	}
	if !strings.Contains(s, "incomplete TAP") {
		t.Errorf("expected incomplete TAP reason, got: %q", s)
	}
	if !strings.Contains(s, "fake-incomplete") {
		t.Errorf("expected shell label in diagnostic, got: %q", s)
	}
}

// TestTAPCleanExitsZero is a regression guard: a clean --tap run still exits 0.
func TestTAPCleanExitsZero(t *testing.T) {
	shells := testrunner.AvailableShells()
	if len(shells) == 0 {
		t.Skip("no shells available")
	}

	cleanTAP := "TAP version 13\n1..2\nok 1 - a passes\nok 2 - b passes\n"
	fakeShell := makeFakeShell(t, "fake-clean", cleanTAP)

	dir := t.TempDir()
	writeFile(t, dir, "tapclean_test.wisp", simplePassTest)

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:       dir,
		TAP:        true,
		FakeShells: []testrunner.Shell{fakeShell},
		Stdout:     &out,
		Stderr:     &out,
	}
	code := testrunner.Run(opts)
	if code != 0 {
		t.Fatalf("expected exit 0 for clean --tap run, got %d\noutput:\n%s", code, out.String())
	}
}

// TestCrossShellDivergenceReported (AC9): a test passing under one shell and
// failing under another must be reported as a FAILURE that names the diverging
// shell(s), with a nonzero exit. Uses the FakeShells seam: two fake shells emit
// the SAME test name with opposite results.
func TestCrossShellDivergenceReported(t *testing.T) {
	passTAP := "TAP version 13\n1..1\nok 1 - flaky\n"
	failTAP := "TAP version 13\n1..1\nnot ok 1 - flaky\n"
	shellA := makeFakeShell(t, "fake-pass", passTAP)
	shellB := makeFakeShell(t, "fake-fail", failTAP)

	dir := t.TempDir()
	writeFile(t, dir, "diverge_test.wisp", simplePassTest)

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:       dir,
		FakeShells: []testrunner.Shell{shellA, shellB},
		Stdout:     &out,
		Stderr:     &out,
	}
	code := testrunner.Run(opts)
	s := out.String()
	if code == 0 {
		t.Fatalf("expected nonzero exit for cross-shell divergence, got 0\noutput:\n%s", s)
	}
	if !strings.Contains(s, "DIVERGE") {
		t.Errorf("output missing DIVERGE line\noutput:\n%s", s)
	}
	if !strings.Contains(s, "fake-fail") {
		t.Errorf("DIVERGE line does not name the diverging shell\noutput:\n%s", s)
	}
}

// TestNameWithSkipDirectivePassReportedPass guards the TAP `#`/SKIP escaping
// seam: a PASSING test literally named `has # SKIP in name` must be reported as
// a pass with its name intact -- never misclassified as a skip because of the
// `# SKIP` substring in the name. The name is escaped at codegen time (`#` ->
// `\#`) and round-tripped by the parser.
func TestNameWithSkipDirectivePassReportedPass(t *testing.T) {
	shells := testrunner.AvailableShells()
	if len(shells) == 0 {
		t.Skip("no shells available")
	}

	src := "test (\"has # SKIP in name\") {\n  assert_eq(1 + 1, 2)\n}\n"
	dir := t.TempDir()
	writeFile(t, dir, "skipname_pass_test.wisp", src)

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:   dir,
		TAP:    true, // TAP mode echoes the per-test name even on a pass.
		Stdout: &out,
		Stderr: &out,
	}
	code := testrunner.Run(opts)
	s := out.String()
	if code != 0 {
		t.Fatalf("expected exit 0 (the test passes), got %d\noutput:\n%s", code, s)
	}
	// The aggregate TAP line is `ok N - <name> [shell]`. A misclassified skip
	// would instead carry a `# SKIP` directive AFTER the `[shell]` label and the
	// name would be truncated at the `#`. Assert the full pass line, with the
	// name round-tripped, and no trailing SKIP directive on it.
	if !strings.Contains(s, "- has # SKIP in name [") {
		t.Errorf("reported name did not round-trip on a pass line; output:\n%s", s)
	}
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "has # SKIP in name") && strings.Contains(line, "] # SKIP") {
			t.Errorf("a passing test was misclassified as skipped: %q", line)
		}
	}
}

// TestNameWithSkipDirectiveFailReportedFail is the companion: a FAILING test
// named `has # SKIP in name` must be reported as a FAILURE with its assertion
// diagnostic intact, never as a skip with the diagnostic lost.
func TestNameWithSkipDirectiveFailReportedFail(t *testing.T) {
	shells := testrunner.AvailableShells()
	if len(shells) == 0 {
		t.Skip("no shells available")
	}

	src := "test (\"has # SKIP in name\") {\n  assert_eq(1, 2)\n}\n"
	dir := t.TempDir()
	writeFile(t, dir, "skipname_fail_test.wisp", src)

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:   dir,
		Stdout: &out,
		Stderr: &out,
	}
	code := testrunner.Run(opts)
	s := out.String()
	if code == 0 {
		t.Fatalf("expected nonzero exit (the test fails), got 0\noutput:\n%s", s)
	}
	if !strings.Contains(s, "FAIL: has # SKIP in name") {
		t.Errorf("a failing test was not reported as a FAILURE with its name intact\noutput:\n%s", s)
	}
	// The assertion diagnostic must survive (assert_eq prints the mismatch).
	if !strings.Contains(s, "1") || !strings.Contains(s, "2") {
		t.Errorf("assertion diagnostic appears lost\noutput:\n%s", s)
	}
}

// makeFakeShellExit is makeFakeShell with an explicit process exit code, so a
// fake shell that emits TAP failures can also exit nonzero -- keeping the
// runner's exit code in agreement with its TAP content (no spurious
// exitMismatch ERROR lines in the summary).
func makeFakeShellExit(t *testing.T, label, tapOutput string, exitCode int) testrunner.Shell {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "fake-runner.sh")
	content := "#!/bin/sh\nprintf '%s' '" + tapOutput + "'\nexit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(scriptPath, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return testrunner.Shell{
		Label: label,
		Bin:   "/bin/sh",
		Args:  []string{scriptPath},
	}
}

// TestSummaryGoldenMix pins the EXACT byte output of the human summary for a
// representative mix: passes, a failure with a diagnostic, a skip, and a
// cross-shell divergence, across two shells. This is the byte-for-byte
// regression guard for emitSummary's formatting and its single-pass aggregation.
func TestSummaryGoldenMix(t *testing.T) {
	// sh-a: alpha passes, beta fails (with diag), gamma skips, delta passes.
	tapA := "TAP version 13\n1..4\n" +
		"ok 1 - alpha_pass\n" +
		"not ok 2 - beta_fail\n# expected 1 got 2\n" +
		"ok 3 - gamma_skip # SKIP not ready\n" +
		"ok 4 - delta_flaky\n"
	// sh-b: same, except delta FAILS -> cross-shell divergence on delta_flaky.
	tapB := "TAP version 13\n1..4\n" +
		"ok 1 - alpha_pass\n" +
		"not ok 2 - beta_fail\n# expected 1 got 2\n" +
		"ok 3 - gamma_skip # SKIP not ready\n" +
		"not ok 4 - delta_flaky\n"
	// Both shells have failing tests, so both exit nonzero to agree with TAP.
	shellA := makeFakeShellExit(t, "sh-a", tapA, 1)
	shellB := makeFakeShellExit(t, "sh-b", tapB, 1)

	dir := t.TempDir()
	writeFile(t, dir, "mix_test.wisp", simplePassTest)

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:       dir,
		FakeShells: []testrunner.Shell{shellA, shellB},
		Stdout:     &out,
		Stderr:     &out,
	}
	code := testrunner.Run(opts)
	if code != 1 {
		t.Fatalf("exit=%d, want 1\noutput:\n%s", code, out.String())
	}

	want := "FAIL  mix_test.wisp [sh-a]: 2 passed, 1 failed, 1 skipped\n" +
		"      FAIL: beta_fail (sh-a)\n" +
		"            expected 1 got 2\n" +
		"FAIL  mix_test.wisp [sh-b]: 1 passed, 2 failed, 1 skipped\n" +
		"      FAIL: beta_fail (sh-b)\n" +
		"            expected 1 got 2\n" +
		"      FAIL: delta_flaky (sh-b)\n" +
		"      DIVERGE: \"delta_flaky\" passes on some shells but fails on: sh-b\n" +
		"---\n3 passed, 3 failed, 2 skipped\n"
	if got := out.String(); got != want {
		t.Errorf("summary output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestSummaryGoldenAllPass pins the exact output of the all-pass single-shell
// case: an "ok  " status line and a clean totals line, exit 0.
func TestSummaryGoldenAllPass(t *testing.T) {
	tap := "TAP version 13\n1..2\nok 1 - one\nok 2 - two\n"
	shell := makeFakeShellExit(t, "sh", tap, 0)

	dir := t.TempDir()
	writeFile(t, dir, "pass_test.wisp", simplePassTest)

	var out bytes.Buffer
	opts := testrunner.Options{
		Path:       dir,
		FakeShells: []testrunner.Shell{shell},
		Stdout:     &out,
		Stderr:     &out,
	}
	code := testrunner.Run(opts)
	if code != 0 {
		t.Fatalf("exit=%d, want 0\noutput:\n%s", code, out.String())
	}
	want := "ok    pass_test.wisp [sh]: 2 passed, 0 failed, 0 skipped\n" +
		"---\n2 passed, 0 failed, 0 skipped\n"
	if got := out.String(); got != want {
		t.Errorf("summary output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestSummaryContainsShellName verifies the summary output names the shell.
func TestSummaryContainsShellName(t *testing.T) {
	shells := testrunner.AvailableShells()
	if len(shells) == 0 {
		t.Skip("no shells available")
	}

	dir := t.TempDir()
	writeFile(t, dir, "named_test.wisp", simplePassTest)

	var out bytes.Buffer
	shellName := shells[0].Label
	opts := testrunner.Options{
		Path:      dir,
		ShellOnly: shellName,
		Stdout:    &out,
		Stderr:    &out,
	}
	code := testrunner.Run(opts)
	if code != 0 {
		t.Fatalf("exit=%d\noutput:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), shellName) {
		t.Errorf("expected shell name %q in output, got: %q", shellName, out.String())
	}
}
