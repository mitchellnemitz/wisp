package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/testrunner"
)

// hasShells reports whether any shell is available for running tests.
func hasShells() bool { return len(testrunner.AvailableShells()) > 0 }

const simplePassSrc = `test ("a passes") {
  assert_eq(1 + 1, 2)
}
`

const simpleFailSrc = `test ("a passes") {
  assert_eq(1 + 1, 2)
}

test ("b fails") {
  assert_eq(1, 2)
}
`

// TestTestCommandNoArgs verifies `wisp test` (no path, defaults to cwd) exits 0
// when there are no test files in cwd (AC16: non-regression, build/run/check/fmt
// unchanged). We run it from a temp dir to avoid discovering real test files.
func TestTestCommandNoTestFiles(t *testing.T) {
	dir := t.TempDir()
	// Change cwd for this test.
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)

	var so, se bytes.Buffer
	code := run([]string{"test"}, &so, &se)
	if code != 0 {
		t.Fatalf("exit=%d, want 0 (no test files)\nstdout=%q\nstderr=%q", code, so.String(), se.String())
	}
}

// TestTestCommandUnknownFlag verifies that an unknown flag exits 2.
func TestTestCommandUnknownFlag(t *testing.T) {
	var so, se bytes.Buffer
	code := run([]string{"test", "--bogus"}, &so, &se)
	if code != 2 {
		t.Fatalf("exit=%d, want 2 for unknown flag", code)
	}
}

// TestTestCommandBadRegex verifies that an invalid --filter regexp exits 2.
func TestTestCommandBadRegex(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a_test.wisp")
	if err := os.WriteFile(p, []byte(simplePassSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	var so, se bytes.Buffer
	code := run([]string{"test", dir, "--filter", "[invalid"}, &so, &se)
	if code != 2 {
		t.Fatalf("exit=%d, want 2 for invalid filter regex", code)
	}
}

// TestTestCommandPass verifies `wisp test <dir>` exits 0 when all tests pass.
func TestTestCommandPass(t *testing.T) {
	if !hasShells() {
		t.Skip("no shells available")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "a_test.wisp")
	if err := os.WriteFile(p, []byte(simplePassSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	var so, se bytes.Buffer
	code := run([]string{"test", dir}, &so, &se)
	if code != 0 {
		t.Fatalf("exit=%d, want 0\nstdout=%q\nstderr=%q", code, so.String(), se.String())
	}
}

// TestTestCommandFail verifies `wisp test <dir>` exits nonzero when a test fails.
func TestTestCommandFail(t *testing.T) {
	if !hasShells() {
		t.Skip("no shells available")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "fail_test.wisp")
	if err := os.WriteFile(p, []byte(simpleFailSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	var so, se bytes.Buffer
	code := run([]string{"test", dir}, &so, &se)
	if code == 0 {
		t.Fatalf("expected non-zero, got 0\nstdout=%q\nstderr=%q", so.String(), se.String())
	}
}

// TestTestCommandTAP verifies --tap emits a TAP-13 header.
func TestTestCommandTAP(t *testing.T) {
	if !hasShells() {
		t.Skip("no shells available")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "tap_test.wisp")
	if err := os.WriteFile(p, []byte(simplePassSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	var so, se bytes.Buffer
	code := run([]string{"test", dir, "--tap"}, &so, &se)
	if code != 0 {
		t.Fatalf("exit=%d, want 0\nstdout=%q\nstderr=%q", code, so.String(), se.String())
	}
	if !strings.HasPrefix(so.String(), "TAP version 13") {
		t.Errorf("expected TAP version 13, got: %q", so.String())
	}
}

// TestTestCommandFilter verifies --filter selects only matching tests.
func TestTestCommandFilter(t *testing.T) {
	if !hasShells() {
		t.Skip("no shells available")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "filter_test.wisp")
	if err := os.WriteFile(p, []byte(simpleFailSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	var so, se bytes.Buffer
	// Only select the passing test; should exit 0.
	code := run([]string{"test", dir, "--filter", "a passes"}, &so, &se)
	if code != 0 {
		t.Fatalf("exit=%d, want 0 (filter hides failing test)\nstdout=%q\nstderr=%q", code, so.String(), se.String())
	}
}

// TestTestCommandCoverage verifies `--coverage` is recognized (not a usage
// error) and prints the per-source-file coverage section after the summary.
func TestTestCommandCoverage(t *testing.T) {
	if !hasShells() {
		t.Skip("no shells available")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "cov_test.wisp")
	if err := os.WriteFile(p, []byte(simplePassSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	var so, se bytes.Buffer
	code := run([]string{"test", dir, "--coverage"}, &so, &se)
	if code != 0 {
		t.Fatalf("exit=%d, want 0\nstdout=%q\nstderr=%q", code, so.String(), se.String())
	}
	if !strings.Contains(so.String(), "--- coverage ---") {
		t.Errorf("expected coverage section, got: %q", so.String())
	}
	if !strings.Contains(so.String(), "cov_test.wisp:") {
		t.Errorf("expected per-file coverage line, got: %q", so.String())
	}
}

// TestNoRegressionOtherSubcommands verifies build/run/check/fmt still work
// after the test subcommand was added (AC16).
func TestNoRegressionOtherSubcommands(t *testing.T) {
	p := writeTmp(t, "hello.wisp", hello)
	tests := []struct {
		args     []string
		wantCode int
	}{
		{[]string{"build", p, "-o", p + ".sh"}, 0},
		{[]string{"check", p}, 0},
		{[]string{"fmt", p}, 0},
		{[]string{"run", p}, 0},
	}
	for _, tc := range tests {
		var so, se bytes.Buffer
		code := run(tc.args, &so, &se)
		if code != tc.wantCode {
			t.Errorf("run(%v) = %d, want %d\nstdout=%q\nstderr=%q",
				tc.args, code, tc.wantCode, so.String(), se.String())
		}
	}
}

// TestTestCommandUsageInUsageString verifies "test" appears in the usage string.
func TestTestCommandUsageInUsageString(t *testing.T) {
	var so, se bytes.Buffer
	run(nil, &so, &se) // no args -> usage
	if !strings.Contains(se.String(), "test") {
		t.Errorf("expected 'test' in usage, got: %q", se.String())
	}
}
