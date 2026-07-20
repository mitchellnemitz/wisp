package codegen_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/driver"
	"github.com/mitchellnemitz/wisp/internal/runtime"
)

// compileTest compiles a *_test.wisp source through the driver and returns the
// generated runner script. It fails the test on any compile error.
func compileTest(t *testing.T, filename, src string) []byte {
	t.Helper()
	if !strings.HasSuffix(filename, "_test.wisp") {
		t.Fatalf("compileTest filename must end in _test.wisp, got %q", filename)
	}
	script, _, diags := driver.Compile(filename, src)
	for _, d := range diags {
		if d.Severity == driver.Error {
			t.Fatalf("unexpected compile error: %s", d)
		}
	}
	if len(script) == 0 {
		t.Fatal("expected a generated runner script")
	}
	return script
}

// runScript writes the script to a temp dir and runs it under /bin/dash,
// returning stdout, stderr, and the exit code.
func runScript(t *testing.T, script []byte) (string, string, int) {
	t.Helper()
	bin, err := exec.LookPath("dash")
	if err != nil {
		t.Skip("dash not available")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "runner.sh")
	if err := os.WriteFile(path, script, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, path)
	cmd.Dir = dir
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	code := 0
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("run: %v", err)
		}
	}
	return out.String(), errb.String(), code
}

// TestNonTestFileByteIdentical: a non-`*_test.wisp` program compiles EXACTLY as
// before the test-mode path existed. The test-mode branch keys off the filename
// suffix; the identical source compiled under a `.wisp` name vs a `_test.wisp`
// name must produce a DIFFERENT shape only for the test file. Here we assert the
// non-test shape is the ordinary main footer and carries no runner scaffolding.
func TestNonTestFileByteIdentical(t *testing.T) {
	src := `fn main() -> int {
  print("hello")
  return 0
}
`
	script, _, diags := driver.Compile("plain.wisp", src)
	for _, d := range diags {
		if d.Severity == driver.Error {
			t.Fatalf("unexpected compile error: %s", d)
		}
	}
	s := string(script)
	// The ordinary main footer is present; no test-runner scaffolding leaked in.
	if !strings.Contains(s, "; exit \"$__ret\"\n") {
		t.Errorf("non-test output missing the ordinary main footer:\n%s", s)
	}
	for _, marker := range []string{"__wisp_test_main", "__wisp_test_", "TAP version 13", "__wisp_ttmp"} {
		if strings.Contains(s, marker) {
			t.Errorf("non-test output unexpectedly contains test-mode marker %q", marker)
		}
	}
}

// TestRunnerEntryDoesNotCollideWithRunBuiltin is a regression test for a fork
// bomb in the test runner. The TAP runner's entry function was named
// __wisp_run -- identical to the run() builtin's shell helper (runtime.Run).
// In a test that called run(), the runner footer's definition shadowed the
// builtin, so run() re-entered the whole suite: unbounded recursion. The
// runner's entry function must not share a name with any builtin helper, so a
// test that uses run() must emit exactly one definition of the run() helper.
func TestRunnerEntryDoesNotCollideWithRunBuiltin(t *testing.T) {
	src := `import "process"
test ("uses run") {
  let out: string = process.run(["echo", "hi"])
  assert_eq(out, "hi")
}
`
	s := string(compileTest(t, "uses_run_test.wisp", src))
	def := runtime.Run + "() {"
	if n := strings.Count(s, def); n != 1 {
		t.Fatalf("expected exactly one definition of the run() builtin helper %q, got %d; the test runner footer collides with it (fork bomb)", def, n)
	}
}

// TestRunnerTAPBasic: a *_test.wisp with one pass, one assert-fail, one skip
// emits well-formed TAP-13 and exits nonzero (a test failed).
func TestRunnerTAPBasic(t *testing.T) {
	src := `test ("a passes") {
  assert_eq(1 + 1, 2)
}

test ("b fails") {
  assert_eq(1, 2)
}

test ("c skips") {
  skip("not ready")
}
`
	script := compileTest(t, "basic_test.wisp", src)
	out, _, code := runScript(t, script)

	wantLines := []string{
		"1..3",
		"ok 1 - a passes",
		"not ok 2 - b fails",
		"ok 3 - c skips # SKIP not ready",
	}
	for _, w := range wantLines {
		if !strings.Contains(out, w) {
			t.Errorf("stdout missing %q\n--- stdout ---\n%s", w, out)
		}
	}
	if code == 0 {
		t.Errorf("runner exit = 0, want nonzero (a test failed)")
	}
}

// TestRunnerAllPassExitZero: all tests pass -> exit 0.
func TestRunnerAllPassExitZero(t *testing.T) {
	src := `test ("one") { assert(true) }
test ("two") { assert_eq(2, 2) }
`
	script := compileTest(t, "pass_test.wisp", src)
	out, _, code := runScript(t, script)
	if code != 0 {
		t.Errorf("runner exit = %d, want 0", code)
	}
	if !strings.Contains(out, "1..2") || !strings.Contains(out, "ok 1 - one") || !strings.Contains(out, "ok 2 - two") {
		t.Errorf("bad TAP:\n%s", out)
	}
}

// TestRunnerLifecycleOrderAndTeardownAlways: setup runs before each test,
// teardown after each (even on fail/skip), and state does not leak between
// tests (subshell isolation). The lifecycle writes ordering breadcrumbs to a
// file under test_tmpdir's PARENT (a fixed path) so we can inspect order.
func TestRunnerLifecycleOrderAndTeardownAlways(t *testing.T) {
	// setup/teardown append to a trace file in the cwd (the temp run dir). Each
	// test appends its own marker. A failing test must STILL get its teardown.
	src := `import "fs"
fn setup() -> void {
  fs.append_file("trace.txt", "setup\n")
}
fn teardown() -> void {
  fs.append_file("trace.txt", "teardown\n")
}
test ("t1 pass") {
  fs.append_file("trace.txt", "body1\n")
}
test ("t2 fail") {
  fs.append_file("trace.txt", "body2\n")
  assert(false)
}
test ("t3 skip") {
  fs.append_file("trace.txt", "body3\n")
  skip("later")
}
`
	script := compileTest(t, "lifecycle_test.wisp", src)
	// runScript uses an internal temp dir we can't inspect, so run in a dedicated
	// dir here and read back the trace file.
	bin, err := exec.LookPath("dash")
	if err != nil {
		t.Skip("dash not available")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "runner.sh")
	if err := os.WriteFile(path, script, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, path)
	cmd.Dir = dir
	_ = cmd.Run()
	trace, err := os.ReadFile(filepath.Join(dir, "trace.txt"))
	if err != nil {
		t.Fatalf("trace.txt not written: %v", err)
	}
	got := string(trace)
	want := "setup\nbody1\nteardown\nsetup\nbody2\nteardown\nsetup\nbody3\nteardown\n"
	if got != want {
		t.Errorf("lifecycle trace mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}
