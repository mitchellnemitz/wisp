package testrunner

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/codegen"
	"github.com/mitchellnemitz/wisp/internal/driver"
)

// Options configures a single Run invocation.
type Options struct {
	// Path is the directory or single *_test.wisp file to test. Defaults to ".".
	Path string

	// Filter is a Go regexp applied to the parsed TAP results: only tests whose
	// name matches are reported. All tests still execute under each shell; the
	// filter narrows the reported results, not what runs. Empty means all tests.
	Filter string

	// ShellOnly restricts execution to a single named shell (e.g. "dash").
	// Empty means all available shells.
	ShellOnly string

	// TAP switches from the human summary to raw TAP-13 output.
	TAP bool

	// Coverage enables native code coverage (spec R15-R17): each file is compiled
	// in coverage mode, run with a per-file $COVFILE, and after the normal
	// pass/fail summary a per-source-file coverage report (covered/total + % +
	// uncovered lines) is printed. Off = today's behavior, byte-for-byte.
	Coverage bool

	// FakeShells, when non-nil, overrides shell discovery entirely. Used by
	// tests to inject shells whose runners emit synthetic TAP output.
	FakeShells []Shell

	Stdout io.Writer
	Stderr io.Writer
}

// hasErrors reports whether diags contains any Error-severity diagnostic.
func hasErrors(diags []driver.Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == driver.Error {
			return true
		}
	}
	return false
}

// fileResult holds the aggregate result for one *_test.wisp file across all shells.
type fileResult struct {
	path  string
	diags []driver.Diagnostic // compile-time errors
	runs  []shellRun

	// coverage-mode fields (populated only when Options.Coverage is set):
	// universe is the instrumented (file,line) set the compiler emitted for this
	// build; covHits is the union of recorded hits across this file's shell runs
	// (each shell appended to the same per-file COVFILE).
	universe []codegen.CoverInst
	covHits  map[codegen.CoverInst]bool
}

// shellRun is one (file, shell) execution pair.
type shellRun struct {
	shell  Shell
	suite  TAPSuite
	raw    string // raw TAP output
	stderr string // stderr captured from the runner
	// exitMismatch is set when the runner exit code disagrees with the TAP content.
	exitMismatch bool
	// tapError is non-empty when the TAP output is incomplete or malformed
	// (missing plan line, result count != plan N). It is always reported
	// regardless of --filter.
	tapError string
}

// Run discovers, compiles, and runs all *_test.wisp files under opts.Path. It
// returns an exit code: 0 iff all selected tests passed or were skipped on
// every shell; nonzero otherwise. Usage errors (bad regex, unreadable path)
// return 2; all other errors return 1.
func Run(opts Options) int {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	// Validate the filter regex up front (usage error = exit 2).
	var filterRE *regexp.Regexp
	if opts.Filter != "" {
		var err error
		filterRE, err = regexp.Compile(opts.Filter)
		if err != nil {
			fmt.Fprintf(stderr, "wisp test: invalid --filter regexp: %v\n", err)
			return 2
		}
	}

	// Resolve the path.
	root := opts.Path
	if root == "" {
		root = "."
	}

	// Discover test files.
	files, err := discoverTestFiles(root)
	if err != nil {
		fmt.Fprintf(stderr, "wisp test: %v\n", err)
		return 2
	}

	// Determine the shells to use.
	var shells []Shell
	if opts.FakeShells != nil {
		shells = opts.FakeShells
	} else if opts.ShellOnly != "" {
		all := AvailableShells()
		for _, sh := range all {
			if sh.Label == opts.ShellOnly {
				shells = append(shells, sh)
				break
			}
		}
		if len(shells) == 0 {
			fmt.Fprintf(stderr, "wisp test: shell %q not available\n", opts.ShellOnly)
			return 1
		}
	} else {
		shells = AvailableShells()
	}

	// Zero shells is a hard error (AC17).
	if len(shells) == 0 {
		fmt.Fprintf(stderr, "wisp test: no shells available; cannot run tests\n")
		return 1
	}

	// No test files found -- exit 0 cleanly.
	if len(files) == 0 {
		fmt.Fprintln(stdout, "wisp test: no *_test.wisp files found")
		return 0
	}

	// Compile and run each file.
	results := make([]fileResult, 0, len(files))
	for _, f := range files {
		res := compileAndRun(f, shells, filterRE, opts.Coverage, stderr)
		results = append(results, res)
	}

	// Emit output.
	if opts.TAP {
		code := emitTAP(results, stdout)
		if opts.Coverage {
			emitCoverage(results, stdout)
		}
		return code
	}
	code := emitSummary(results, stdout)
	if opts.Coverage {
		emitCoverage(results, stdout)
	}
	return code
}

// discoverTestFiles returns sorted *_test.wisp files under root. If root is
// itself a *_test.wisp file, it returns just that file.
func discoverTestFiles(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if strings.HasSuffix(root, "_test.wisp") {
			abs, err := filepath.Abs(root)
			if err != nil {
				return nil, err
			}
			return []string{abs}, nil
		}
		return nil, fmt.Errorf("path %q is not a *_test.wisp file", root)
	}

	var files []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, "_test.wisp") {
			abs, ferr := filepath.Abs(path)
			if ferr != nil {
				return ferr
			}
			files = append(files, abs)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// compileAndRun compiles one test file and runs it under each shell. When
// coverage is set the file is compiled in coverage mode, run with a per-file
// $COVFILE (shared across this file's shell runs so a line hit under ANY shell
// counts -- the union), and the recorded hits are read back and unioned.
func compileAndRun(path string, shells []Shell, filter *regexp.Regexp, coverage bool, stderr io.Writer) fileResult {
	res := fileResult{path: path}

	src, err := os.ReadFile(path)
	if err != nil {
		res.diags = []driver.Diagnostic{{Msg: err.Error()}}
		return res
	}

	var script []byte
	var diags []driver.Diagnostic
	var covFile string
	if coverage {
		var universe []codegen.CoverInst
		script, universe, diags = driver.CompileCoverage(path, string(src))
		res.universe = universe
		res.covHits = map[codegen.CoverInst]bool{}
	} else {
		script, _, diags = driver.Compile(path, string(src))
	}
	if hasErrors(diags) {
		res.diags = diags
		for _, d := range diags {
			if d.Severity == driver.Error {
				fmt.Fprintln(stderr, d.String())
			}
		}
		return res
	}

	if coverage {
		// One COVFILE per test file, shared across that file's shell runs: each
		// shell appends to it, so a line hit under ANY shell is recorded once and
		// the reported coverage is the cross-shell union (spec multi-shell).
		f, err := os.CreateTemp("", "wisp-cov-*.dat")
		if err != nil {
			res.diags = []driver.Diagnostic{{Msg: "covfile: " + err.Error()}}
			return res
		}
		covFile = f.Name()
		f.Close()
		defer os.Remove(covFile)
	}

	// Write the runner to a temp file; reuse across shells.
	tmp, err := os.CreateTemp("", "wisp-test-*.sh")
	if err != nil {
		res.diags = []driver.Diagnostic{{Msg: "temp file: " + err.Error()}}
		return res
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(script); err != nil {
		tmp.Close()
		res.diags = []driver.Diagnostic{{Msg: "write temp: " + err.Error()}}
		return res
	}
	if err := tmp.Close(); err != nil {
		res.diags = []driver.Diagnostic{{Msg: "close temp: " + err.Error()}}
		return res
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		res.diags = []driver.Diagnostic{{Msg: "chmod temp: " + err.Error()}}
		return res
	}

	for _, sh := range shells {
		run := runUnder(sh, tmpPath, filter, covFile)
		res.runs = append(res.runs, run)
	}

	// Read back the accumulated hits (union across this file's shell runs).
	if coverage && covFile != "" {
		if data, err := os.ReadFile(covFile); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if line == "" {
					continue
				}
				if inst, ok := parseCovRecord(line); ok {
					res.covHits[inst] = true
				}
			}
		}
	}

	return res
}

// parseCovRecord splits a `<file>:<line>` coverage record back into a CoverInst.
// The line component is the final colon-delimited field (a compiler integer);
// the file may itself contain colons, so split from the right. A malformed
// record (no colon, non-integer line) is ignored.
func parseCovRecord(s string) (codegen.CoverInst, bool) {
	i := strings.LastIndexByte(s, ':')
	if i < 0 {
		return codegen.CoverInst{}, false
	}
	ln, err := strconv.Atoi(s[i+1:])
	if err != nil {
		return codegen.CoverInst{}, false
	}
	return codegen.CoverInst{File: s[:i], Line: ln}, true
}

// runUnder executes the runner script under one shell and parses the TAP output.
// When covFile is non-empty the run is given COVFILE=covFile in its environment
// so the coverage-instrumented script's per-statement markers append there; the
// per-test subshells inherit the exported variable, so hits accumulate across
// them.
func runUnder(sh Shell, scriptPath string, filter *regexp.Regexp, covFile string) shellRun {
	cmdArgs := append(append([]string{}, sh.Args...), scriptPath)
	cmd := exec.Command(sh.Bin, cmdArgs...)
	if covFile != "" {
		cmd.Env = append(os.Environ(), "COVFILE="+covFile)
	}
	// No stdin; capture stdout (TAP) and stderr (located error messages).
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
			if exitCode < 0 {
				exitCode = 1 // killed by signal
			}
		} else {
			exitCode = 1
		}
	}

	raw := outBuf.String()
	suite := ParseTAP(raw)

	// TAP-completeness check: always runs, independent of filter.
	// A runner that crashes or truncates its output must not pass silently.
	var tapErr string
	if !suite.Complete() {
		if !suite.PlanExplicit {
			tapErr = fmt.Sprintf("incomplete TAP: no plan line in output (got %d results)", len(suite.Results))
		} else {
			tapErr = fmt.Sprintf("incomplete TAP: expected %d results, got %d", suite.Plan, len(suite.Results))
		}
	}

	// Detect exit-code/TAP disagreement on the UNFILTERED suite. A filter
	// removes results post-hoc from a full runner execution, so the runner's
	// exit code legitimately reflects tests outside our filtered view.
	unfilteredFailed := false
	for _, r := range suite.Results {
		if !r.OK && !r.Skip {
			unfilteredFailed = true
			break
		}
	}
	exitMismatch := filter == nil && tapErr == "" && (exitCode == 0) != (!unfilteredFailed)

	// Apply filter: remove results that don't match.
	if filter != nil {
		var kept []TAPResult
		for _, r := range suite.Results {
			if filter.MatchString(r.Name) {
				kept = append(kept, r)
			}
		}
		suite.Results = kept
		suite.Plan = len(kept)
	}

	return shellRun{
		shell:        sh,
		suite:        suite,
		raw:          raw,
		stderr:       errBuf.String(),
		exitMismatch: exitMismatch,
		tapError:     tapErr,
	}
}

// runFailure is one failing test within a shell run, captured for the summary's
// failure-detail lines.
type runFailure struct {
	name string
	diag string
}

// runStats holds the per-run aggregate for one (file, shell) pair.
type runStats struct {
	label        string
	pass         int
	fail         int
	skip         int
	failures     []runFailure // in TAP result order
	exitMismatch bool
	tapError     string
}

// failed reports whether this run counts as a failure for status/exit purposes.
func (r runStats) failed() bool {
	return r.fail > 0 || r.exitMismatch || r.tapError != ""
}

// divergence is a test that passed on some shells and failed on others (AC9).
type divergence struct {
	name       string
	failShells []string // sorted
}

// fileStats is the fully-computed summary aggregate for one test file.
type fileStats struct {
	shortPath   string
	compileErr  bool
	runs        []runStats
	divergences []divergence
	failed      bool // file contributes a nonzero exit
}

// summaryStats is the whole-run aggregate: per-file stats plus the grand totals
// and overall pass/fail verdict. It is computed in a single pass over results.
type summaryStats struct {
	files     []fileStats
	pass      int
	fail      int
	skip      int
	overallOK bool
}

// computeFileStats aggregates results in ONE pass into a summaryStats: per-run
// pass/fail/skip counts, failure details, cross-shell divergence, and the grand
// totals. It performs no I/O and no formatting -- printFileStats renders it.
func computeFileStats(results []fileResult) summaryStats {
	stats := summaryStats{overallOK: true}

	for _, fr := range results {
		fs := fileStats{shortPath: filepath.Base(fr.path)}

		if len(fr.diags) > 0 {
			fs.compileErr = true
			fs.failed = true
			stats.overallOK = false
			// A compile-error file counts as a single failure in the totals.
			stats.fail++
			stats.files = append(stats.files, fs)
			continue
		}
		if len(fr.runs) == 0 {
			stats.files = append(stats.files, fs)
			continue
		}

		// passByShell[testName][shellLabel] = passed-or-skipped, used to detect
		// cross-shell divergence after all runs are counted.
		passByShell := map[string]map[string]bool{}

		for _, run := range fr.runs {
			rs := runStats{
				label:        run.shell.Label,
				exitMismatch: run.exitMismatch,
				tapError:     run.tapError,
			}
			for _, r := range run.suite.Results {
				if passByShell[r.Name] == nil {
					passByShell[r.Name] = map[string]bool{}
				}
				passByShell[r.Name][run.shell.Label] = r.OK || r.Skip

				switch {
				case r.Skip:
					rs.skip++
					stats.skip++
				case r.OK:
					rs.pass++
					stats.pass++
				default:
					rs.fail++
					stats.fail++
					rs.failures = append(rs.failures, runFailure{name: r.Name, diag: r.Diag})
				}
			}
			if rs.failed() {
				fs.failed = true
			}
			fs.runs = append(fs.runs, rs)
		}

		// Cross-shell divergence: a test that passed somewhere and failed
		// somewhere. failShells is sorted; the divergence list follows map
		// iteration order (as before) -- deterministic when a file has at most
		// one diverging test.
		for name, shellMap := range passByShell {
			hasPass, hasFail := false, false
			var failShells []string
			for sh, ok := range shellMap {
				if ok {
					hasPass = true
				} else {
					hasFail = true
					failShells = append(failShells, sh)
				}
			}
			if hasPass && hasFail {
				sort.Strings(failShells)
				fs.divergences = append(fs.divergences, divergence{name: name, failShells: failShells})
				fs.failed = true
			}
		}

		if fs.failed {
			stats.overallOK = false
		}
		stats.files = append(stats.files, fs)
	}

	return stats
}

// printFileStats renders a computed summaryStats to w and returns the exit code
// (1 if any file failed, else 0). It is pure formatting -- all aggregation
// happened in computeFileStats.
func printFileStats(stats summaryStats, w io.Writer) int {
	for _, fs := range stats.files {
		if fs.compileErr {
			fmt.Fprintf(w, "FAIL  %s (compile error)\n", fs.shortPath)
			continue
		}
		for _, rs := range fs.runs {
			status := "ok  "
			if rs.failed() {
				status = "FAIL"
			}
			fmt.Fprintf(w, "%s  %s [%s]: %d passed, %d failed, %d skipped\n",
				status, fs.shortPath, rs.label, rs.pass, rs.fail, rs.skip)

			for _, f := range rs.failures {
				fmt.Fprintf(w, "      FAIL: %s (%s)\n", f.name, rs.label)
				if f.diag != "" {
					for _, dline := range strings.Split(f.diag, "\n") {
						fmt.Fprintf(w, "            %s\n", strings.TrimPrefix(dline, "# "))
					}
				}
			}
			if rs.exitMismatch {
				fmt.Fprintf(w, "      ERROR: runner exit code disagrees with TAP (%s)\n", rs.label)
			}
			if rs.tapError != "" {
				fmt.Fprintf(w, "      ERROR: %s [%s]: %s\n", fs.shortPath, rs.label, rs.tapError)
			}
		}

		for _, d := range fs.divergences {
			fmt.Fprintf(w, "      DIVERGE: %q passes on some shells but fails on: %s\n",
				d.name, strings.Join(d.failShells, ", "))
		}
	}

	fmt.Fprintf(w, "---\n%d passed, %d failed, %d skipped\n", stats.pass, stats.fail, stats.skip)

	if !stats.overallOK {
		return 1
	}
	return 0
}

// emitSummary prints the human-readable summary and returns the exit code.
func emitSummary(results []fileResult, w io.Writer) int {
	return printFileStats(computeFileStats(results), w)
}

// emitTAP emits raw TAP-13 output aggregated across all files and shells, and
// returns the exit code.
func emitTAP(results []fileResult, w io.Writer) int {
	// Count total test cases across all files/shells to build the plan.
	// For TAP output we emit each (file x shell) run sequentially, and use
	// a single cumulative numbering.
	type tapEntry struct {
		num  int
		line string // "ok N - ..." or "not ok N - ..."
		diag string
	}

	var entries []tapEntry
	n := 0

	for _, fr := range results {
		if len(fr.diags) > 0 {
			n++
			entries = append(entries, tapEntry{
				num:  n,
				line: fmt.Sprintf("not ok %d - %s (compile error)", n, filepath.Base(fr.path)),
			})
			continue
		}
		for _, run := range fr.runs {
			for _, r := range run.suite.Results {
				n++
				var line string
				if r.OK || r.Skip {
					if r.Skip {
						line = fmt.Sprintf("ok %d - %s [%s] # SKIP %s",
							n, r.Name, run.shell.Label, r.SkipReason)
					} else {
						line = fmt.Sprintf("ok %d - %s [%s]", n, r.Name, run.shell.Label)
					}
				} else {
					line = fmt.Sprintf("not ok %d - %s [%s]", n, r.Name, run.shell.Label)
				}
				entries = append(entries, tapEntry{num: n, line: line, diag: r.Diag})
			}
			// Emit a synthetic failure line for any run whose own output was
			// incomplete/malformed or whose exit code disagreed with its TAP.
			// This keeps the --tap exit code consistent with emitSummary: such a
			// run must force a nonzero exit even if all its emitted lines are ok.
			if run.tapError != "" {
				n++
				entries = append(entries, tapEntry{
					num:  n,
					line: fmt.Sprintf("not ok %d - %s [%s] (%s)", n, filepath.Base(fr.path), run.shell.Label, run.tapError),
				})
			} else if run.exitMismatch {
				n++
				entries = append(entries, tapEntry{
					num:  n,
					line: fmt.Sprintf("not ok %d - %s [%s] (runner exit code disagrees with TAP)", n, filepath.Base(fr.path), run.shell.Label),
				})
			}
		}
	}

	fmt.Fprintln(w, "TAP version 13")
	fmt.Fprintf(w, "1..%d\n", n)
	failed := false
	for _, e := range entries {
		fmt.Fprintln(w, e.line)
		if e.diag != "" {
			fmt.Fprintln(w, e.diag)
		}
		if strings.HasPrefix(e.line, "not ok") {
			failed = true
		}
	}
	if failed {
		return 1
	}
	return 0
}

// emitCoverage renders the per-source-file coverage report (spec R15). The
// universe is the set of instrumented (file,line) pairs the compiler emitted
// (returned by the coverage-mode build), aggregated across every test file
// built; the hits are the cross-shell union read back from each file's COVFILE.
// For each source file it prints `covered/total (NN%)` and the sorted list of
// uncovered line numbers. Output is deterministic: files sorted, lines sorted.
//
// A line is COVERED if hit under any shell. Coverage % is integer-truncated
// (covered*100/total). A file with an empty universe (no instrumented lines) is
// omitted. The report is appended AFTER the normal pass/fail summary and never
// changes the exit code.
func emitCoverage(results []fileResult, w io.Writer) {
	// fileLines[srcFile] is the set of instrumented lines for that source file;
	// hitLines[srcFile] is the subset that was hit. The universe spans both the
	// test files and the code-under-test they link in (modules link into one .sh,
	// so a single build's universe carries every reachable source file's lines).
	fileLines := map[string]map[int]bool{}
	hitLines := map[string]map[int]bool{}
	ensure := func(m map[string]map[int]bool, f string) map[int]bool {
		if m[f] == nil {
			m[f] = map[int]bool{}
		}
		return m[f]
	}

	for _, fr := range results {
		if fr.universe == nil {
			continue
		}
		for _, inst := range fr.universe {
			ensure(fileLines, inst.File)[inst.Line] = true
			if fr.covHits[inst] {
				ensure(hitLines, inst.File)[inst.Line] = true
			}
		}
	}

	if len(fileLines) == 0 {
		return
	}

	files := make([]string, 0, len(fileLines))
	for f := range fileLines {
		files = append(files, f)
	}
	sort.Strings(files)

	fmt.Fprintln(w, "--- coverage ---")
	for _, f := range files {
		total := len(fileLines[f])
		covered := len(hitLines[f])
		pct := 0
		if total > 0 {
			pct = covered * 100 / total
		}
		// Uncovered = instrumented lines never hit, sorted.
		var uncovered []int
		for ln := range fileLines[f] {
			if !hitLines[f][ln] {
				uncovered = append(uncovered, ln)
			}
		}
		sort.Ints(uncovered)

		fmt.Fprintf(w, "%s: %d/%d (%d%%)\n", filepath.Base(f), covered, total, pct)
		if len(uncovered) > 0 {
			parts := make([]string, len(uncovered))
			for i, ln := range uncovered {
				parts[i] = strconv.Itoa(ln)
			}
			fmt.Fprintf(w, "      uncovered: %s\n", strings.Join(parts, " "))
		}
	}
}
