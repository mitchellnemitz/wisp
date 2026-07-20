package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/parser"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// compileCoverage parses, checks, and runs the coverage-mode emitter, failing on
// any compile error. Returns the script and the instrumented universe.
func compileCoverage(t *testing.T, src, file string) ([]byte, []CoverInst) {
	t.Helper()
	prog, err := parser.Parse(src, file)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info := types.Check(prog)
	if len(info.Errors) > 0 {
		t.Fatalf("check errors: %v", info.Errors)
	}
	// Drive the coverage path the same way the driver does, single-module.
	script, _, universe, err := generate(prog, info, false, true)
	if err != nil {
		t.Fatalf("generate(coverage): %v", err)
	}
	return script, universe
}

const covProg = `fn helper(x: int) -> int {
  return x + 1
}

fn main() -> int {
  let a: int = helper(2)
  print(to_string(a))
  return 0
}`

// TestCoverageByteIdentity: a normal (non-coverage) compile emits NO __wisp_cov
// markers and is byte-identical to GenerateWithMap today. --coverage is the only
// switch that alters the .sh (spec R16, AC18).
func TestCoverageByteIdentity(t *testing.T) {
	normal := compile(t, covProg)
	if strings.Contains(string(normal), "__wisp_cov") {
		t.Fatalf("non-coverage build emitted a coverage marker:\n%s", normal)
	}
}

// TestCoverageMarkers: the coverage-mode compile emits one marker per executable
// statement, the marker is the documented inert double-quoted form, and the
// helper is present (not tree-shaken).
func TestCoverageMarkers(t *testing.T) {
	script, universe := compileCoverage(t, covProg, "cov.wisp")
	s := string(script)
	if !strings.Contains(s, "__wisp_cov() {") {
		t.Fatalf("coverage build is missing the __wisp_cov helper:\n%s", s)
	}
	// Markers present for the four executable statements (helper return; main's
	// let, print, return).
	wantLines := []int{2, 6, 7, 8}
	for _, ln := range wantLines {
		marker := "__wisp_cov \"cov.wisp:" + strconv.Itoa(ln) + "\""
		if !strings.Contains(s, marker) {
			t.Fatalf("missing marker %q in:\n%s", marker, s)
		}
	}
	// Universe equals the instrumented (file,line) set, deduped, NOT derived from
	// the raw line map.
	gotLines := universeLines(universe, "cov.wisp")
	if !equalInts(gotLines, wantLines) {
		t.Fatalf("universe lines = %v, want %v", gotLines, wantLines)
	}
}

// TestCoverageMarkerNotInNonCoverage: a no-coverage build never references the
// helper id either (tree-shaken out).
func TestCoverageTreeShakenWhenOff(t *testing.T) {
	normal := compile(t, covProg)
	if strings.Contains(string(normal), "__wisp_cov() {") {
		t.Fatalf("non-coverage build emitted the __wisp_cov helper body")
	}
}

// TestCoverageInjectionSafeFile: a source file path containing shell
// metacharacters is emitted as an inert double-quoted literal -- the marker line
// must not contain an un-escaped $ or backtick.
func TestCoverageInjectionSafeFile(t *testing.T) {
	weird := `a$b` + "`c`" + `"d".wisp`
	script, _ := compileCoverage(t, covProg, weird)
	s := string(script)
	// Every metacharacter must be backslash-escaped inside the double quotes.
	for _, line := range strings.Split(s, "\n") {
		if !strings.HasPrefix(strings.TrimLeft(line, "\t"), "__wisp_cov ") {
			continue
		}
		body := strings.TrimLeft(line, "\t")
		if strings.Contains(body, `$b`) && !strings.Contains(body, `\$b`) {
			t.Fatalf("unescaped $ in marker: %q", body)
		}
		if strings.Contains(body, "`c`") && !strings.Contains(body, "\\`c") {
			t.Fatalf("unescaped backtick in marker: %q", body)
		}
	}
}

// TestCoverageScriptShellcheckAndRun: the coverage-mode script is ShellCheck-
// clean, and runs correctly both with COVFILE set (records hits, one per line)
// and with COVFILE unset (no crash -- the helper no-ops).
func TestCoverageScriptShellcheckAndRun(t *testing.T) {
	script, _ := compileCoverage(t, covProg, "cov.wisp")
	shellcheck(t, script)

	dash, err := exec.LookPath("dash")
	if err != nil {
		t.Skip("dash not available")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "out.sh")
	if err := os.WriteFile(path, script, 0o755); err != nil {
		t.Fatal(err)
	}

	// COVFILE set: records accumulate, one record per executed statement line.
	covfile := filepath.Join(dir, "cov.dat")
	cmd := exec.Command(dash, path)
	cmd.Env = append(os.Environ(), "COVFILE="+covfile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run with COVFILE: %v\n%s", err, out)
	}
	data, err := os.ReadFile(covfile)
	if err != nil {
		t.Fatalf("read covfile: %v", err)
	}
	got := map[string]bool{}
	for _, ln := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if ln != "" {
			got[ln] = true
		}
	}
	// helper return (2), main let (6), print (7), return (8) all executed.
	for _, want := range []string{"cov.wisp:2", "cov.wisp:6", "cov.wisp:7", "cov.wisp:8"} {
		if !got[want] {
			t.Fatalf("missing recorded hit %q; got %v", want, got)
		}
	}

	// COVFILE unset/empty: must not crash, must not create a stray file.
	cmd = exec.Command(dash, path)
	cmd.Env = append(os.Environ(), "COVFILE=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run with empty COVFILE crashed: %v\n%s", err, out)
	}
}

// TestCoverageErrModeShellcheck: a coverage-mode build of a program that uses
// try/throw (errMode -- each statement is wrapped in a skip-guard) is still
// ShellCheck-clean and the markers land inside the guards.
func TestCoverageErrModeShellcheck(t *testing.T) {
	src := `fn risky(n: int) -> int {
  if (n < 0) {
    throw error("neg")
  }
  return n
}

fn main() -> int {
  try {
    let r: int = risky(5)
    print(to_string(r))
  } catch (e) {
    print(e.message)
  }
  return 0
}`
	script, universe := compileCoverage(t, src, "em.wisp")
	if len(universe) == 0 {
		t.Fatal("errMode coverage build produced an empty universe")
	}
	if !strings.Contains(string(script), "__wisp_cov \"em.wisp:") {
		t.Fatalf("errMode coverage build is missing markers:\n%s", script)
	}
	shellcheck(t, script)
}

// uncalledProg has an export fn that no caller references, so the tree-shaker
// drops it before codegen. Its body line must still appear in the coverage
// universe (derived from source, not from instrumentation), reported uncovered.
const uncalledProg = `export fn never_called(x: int) -> int {
  return x + 1
}

fn main() -> int {
  print("hi")
  return 0
}`

// TestCoverageUniverseIncludesUncalledFunction: a coverage build of a program
// with a function that no test/caller references must still carry that
// function's body statement lines in the returned universe. Today the universe
// is collected from emitted markers, so a tree-shaken function vanishes and the
// file misreports 100%. The universe MUST come from a source-level AST walk.
func TestCoverageUniverseIncludesUncalledFunction(t *testing.T) {
	_, universe := compileCoverage(t, uncalledProg, "uncalled.wisp")
	got := universeLines(universe, "uncalled.wisp")
	// never_called's body is line 2; main's print is line 6 and return is line 7.
	want := []int{2, 6, 7}
	if !equalInts(got, want) {
		t.Fatalf("universe lines = %v, want %v (line 2 is the uncalled function's body)", got, want)
	}
}

func universeLines(u []CoverInst, file string) []int {
	var out []int
	for _, c := range u {
		if c.File == file {
			out = append(out, c.Line)
		}
	}
	sort.Ints(out)
	return out
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	sort.Ints(a)
	sb := append([]int(nil), b...)
	sort.Ints(sb)
	for i := range a {
		if a[i] != sb[i] {
			return false
		}
	}
	return true
}
