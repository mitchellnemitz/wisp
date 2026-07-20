// Package golden runs the golden-file test harness (spec section 12).
//
// Each fixture is a `<name>.wisp` source plus a sidecar `<name>.json`
// describing the expected behavior. The harness compiles the source through the
// in-process driver and, for compiling fixtures, runs the generated script
// under every available shell (dash always; busybox sh/ash only when busybox is
// in PATH) plus ShellCheck, asserting stdout, exit status, and the stderr
// assertion for each shell. Compile-error fixtures assert a positioned
// diagnostic and run nothing.
//
// # Sidecar schema (`<name>.json`)
//
//	{
//	  "desc":         "human description (optional)",
//	  "stdout":       "exact expected stdout",         // compiling fixtures
//	  "exit":         0,                                // expected exit code
//	  "args":         ["a", "b"],                       // optional argv for run
//	  "stderr": {                                       // optional
//	     "exact":    "...",        // exact-match form, OR
//	     "contains": "...",        // substring form (requires nonEmpty)
//	     "nonEmpty": true
//	  },
//	  "compileError": true,        // fixture must NOT compile; no script run
//	  "diagContains": "substr",    // optional: a diagnostic msg must contain this
//	  "stubPath":     "somedir",   // optional: dir under testdata/golden/ prepended to $PATH
//	  "requiresExternal": "name",  // optional: skip a shell that reserves `name` as a builtin
//	  "isolatePath": true          // optional: strips real $PATH dirs containing requiresExternal
//	}
package golden

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/driver"
)

// stderrSpec is the two-form stderr assertion (spec section 12).
type stderrSpec struct {
	Exact    *string `json:"exact"`
	Contains string  `json:"contains"`
	NonEmpty bool    `json:"nonEmpty"`
}

type fixtureSpec struct {
	Desc         string      `json:"desc"`
	Stdout       string      `json:"stdout"`
	Exit         int         `json:"exit"`
	Args         []string    `json:"args"`
	Stderr       *stderrSpec `json:"stderr"`
	CompileError bool        `json:"compileError"`
	DiagContains string      `json:"diagContains"`
	// WarnContains, on a compiling fixture, requires a Warning diagnostic whose
	// message contains this substring (the warning is a compile-time diagnostic,
	// not script stderr). Verifies the non-gating warning path (spec rule 6).
	WarnContains string `json:"warnContains"`
	// AllowNoPos permits a program-level error diagnostic with no source line
	// (e.g. a missing main, which has no offending token). Without it, every
	// error diagnostic must carry a position (spec AC 3).
	AllowNoPos bool `json:"allowNoPos"`
	// Stdin is the content fed to the script's standard input. When empty
	// (the default for all existing fixtures), cmd.Stdin remains nil, which
	// provides an immediate EOF on stdin -- identical to an empty reader.
	Stdin string `json:"stdin"`
	// StubPath, when set, names a directory (relative to fixtureDir)
	// containing fixture-provided executables to prepend to $PATH for this
	// fixture's run. Empty (the default) leaves cmd.Env nil, unchanged from
	// today's behavior.
	StubPath string `json:"stubPath"`
	// RequiresExternal names a command that this fixture's assertions depend
	// on resolving to an external executable, not a shell builtin. When set,
	// checkFixture probes each shell live via shellReservesBuiltin and skips
	// that shell's subtest if it reserves the name as a builtin (e.g. zsh's
	// own `print`). At least one shell must actually run the fixture, or the
	// fixture's top-level test fails.
	RequiresExternal string `json:"requiresExternal"`
	// IsolatePath, when true (only meaningful together with StubPath and
	// RequiresExternal), removes any $PATH directory that contains a file
	// named RequiresExternal before appending the rest of the real host
	// $PATH after StubPath. Used by fixtures asserting a "command not
	// found" outcome for RequiresExternal, so the result is deterministic
	// regardless of what the host machine's own $PATH contains, without
	// removing directories the runtime's own helpers (e.g. mktemp/cat/rm)
	// need to resolve.
	IsolatePath bool `json:"isolatePath"`
}

const fixtureDir = "../../testdata/golden"

func TestGolden(t *testing.T) {
	entries, err := filepath.Glob(filepath.Join(fixtureDir, "*.wisp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatalf("no fixtures found in %s", fixtureDir)
	}

	shells := availableShells(t)

	for _, wispPath := range entries {
		name := strings.TrimSuffix(filepath.Base(wispPath), ".wisp")
		t.Run(name, func(t *testing.T) {
			spec := loadSpec(t, wispPath)
			src, err := os.ReadFile(wispPath)
			if err != nil {
				t.Fatal(err)
			}
			// Single-file fixtures compile by base name (no project on disk).
			script, _, diags := driver.Compile(filepath.Base(wispPath), string(src))
			checkFixture(t, spec, script, diags, shells)
		})
	}

	// Multi-module fixtures (M8): a `<name>.dir/` directory holding main.wisp plus
	// any included files and a hand-populated .wisp/modules/ package tree, with a
	// sibling `<name>.dir.json` sidecar. The root is compiled by its REAL path so
	// the loader resolves includes/imports.
	dirs, err := filepath.Glob(filepath.Join(fixtureDir, "*.dir"))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dirs {
		name := strings.TrimSuffix(filepath.Base(d), ".dir")
		t.Run(name, func(t *testing.T) {
			spec := loadSpecPath(t, d+".json")
			rootPath := filepath.Join(d, "main.wisp")
			src, err := os.ReadFile(rootPath)
			if err != nil {
				t.Fatalf("missing %s/main.wisp: %v", d, err)
			}
			script, _, diags := driver.Compile(rootPath, string(src))
			checkFixture(t, spec, script, diags, shells)
		})
	}
}

// checkFixture runs the shared assertions for one fixture's compile result.
func checkFixture(t *testing.T, spec fixtureSpec, script []byte, diags []driver.Diagnostic, shells []shell) {
	t.Helper()
	if spec.CompileError {
		assertCompileError(t, spec, script, diags)
		return
	}
	if errored(diags) {
		t.Fatalf("unexpected compile errors: %v", diags)
	}
	if len(script) == 0 {
		t.Fatal("expected a generated script")
	}
	if spec.WarnContains != "" {
		assertWarn(t, spec.WarnContains, diags)
	}
	shellcheckScript(t, script)
	ranAny := false
	for _, sh := range shells {
		sh := sh
		if spec.RequiresExternal != "" && shellReservesBuiltin(sh, spec.RequiresExternal) {
			t.Logf("%s reserves %q as a builtin; skipping (F1 argv[0] guarantee does not apply)", sh.label, spec.RequiresExternal)
			continue
		}
		ranAny = true
		t.Run(sh.label, func(t *testing.T) {
			out, errb, code := runUnder(t, sh, script, spec)
			assertRun(t, spec, out, errb, code)
		})
	}
	if spec.RequiresExternal != "" && !ranAny {
		t.Errorf("requiresExternal=%q but every available shell reserves it as a builtin; nothing was asserted", spec.RequiresExternal)
	}
}

func assertCompileError(t *testing.T, spec fixtureSpec, script []byte, diags []driver.Diagnostic) {
	t.Helper()
	if script != nil {
		t.Fatalf("compileError fixture produced a script (%d bytes)", len(script))
	}
	var errDiags []driver.Diagnostic
	for _, d := range diags {
		if d.Severity == driver.Error {
			errDiags = append(errDiags, d)
		}
	}
	if len(errDiags) == 0 {
		t.Fatalf("expected an Error diagnostic, got %v", diags)
	}
	// Every error diagnostic must carry a source position (spec AC 3), unless
	// the fixture opts out for a program-level diagnostic (e.g. missing main).
	if !spec.AllowNoPos {
		for _, d := range errDiags {
			if d.Pos.Line == 0 {
				t.Errorf("diagnostic missing position: %v", d)
			}
		}
	}
	if spec.DiagContains != "" {
		var ok bool
		for _, d := range errDiags {
			if strings.Contains(d.Msg, spec.DiagContains) {
				ok = true
				break
			}
		}
		if !ok {
			t.Fatalf("no diagnostic contains %q; got %v", spec.DiagContains, errDiags)
		}
	}
}

func assertWarn(t *testing.T, want string, diags []driver.Diagnostic) {
	t.Helper()
	for _, d := range diags {
		if d.Severity == driver.Warning && strings.Contains(d.Msg, want) {
			if d.Pos.Line == 0 {
				t.Errorf("warning missing position: %v", d)
			}
			return
		}
	}
	t.Errorf("no warning diagnostic contains %q; got %v", want, diags)
}

func assertRun(t *testing.T, spec fixtureSpec, out, errb string, code int) {
	t.Helper()
	if out != spec.Stdout {
		t.Errorf("stdout = %q, want %q", out, spec.Stdout)
	}
	if code != spec.Exit {
		t.Errorf("exit = %d, want %d (stderr=%q)", code, spec.Exit, errb)
	}
	if spec.Stderr != nil {
		assertStderr(t, *spec.Stderr, errb)
	}
}

func assertStderr(t *testing.T, spec stderrSpec, errb string) {
	t.Helper()
	if spec.Exact != nil {
		if errb != *spec.Exact {
			t.Errorf("stderr = %q, want exact %q", errb, *spec.Exact)
		}
		return
	}
	if spec.NonEmpty && strings.TrimSpace(errb) == "" {
		t.Errorf("stderr empty, want non-empty")
	}
	if spec.Contains != "" && !strings.Contains(errb, spec.Contains) {
		t.Errorf("stderr = %q, want substring %q", errb, spec.Contains)
	}
}

func loadSpec(t *testing.T, wispPath string) fixtureSpec {
	t.Helper()
	return loadSpecPath(t, strings.TrimSuffix(wispPath, ".wisp")+".json")
}

func loadSpecPath(t *testing.T, jsonPath string) fixtureSpec {
	t.Helper()
	b, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("missing sidecar %s: %v", filepath.Base(jsonPath), err)
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	var spec fixtureSpec
	if err := dec.Decode(&spec); err != nil {
		t.Fatalf("bad sidecar %s: %v", filepath.Base(jsonPath), err)
	}
	return spec
}

func errored(diags []driver.Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == driver.Error {
			return true
		}
	}
	return false
}

// --- shell discovery & execution ---

type shell struct {
	label string
	bin   string
	args  []string // leading args before the script path
}

// availableShells returns the shells to test under. dash must be present
// (otherwise the suite is skipped, never failed). busybox sh is added only when
// busybox is in PATH; its absence is logged, not failed (spec section 12 / the
// harness contract).
func availableShells(t *testing.T) []shell {
	t.Helper()
	var shells []shell

	if bin, err := exec.LookPath("dash"); err == nil {
		shells = append(shells, shell{label: "dash", bin: bin})
	} else {
		t.Skip("dash not available; cannot run golden fixtures")
	}

	if bin, err := exec.LookPath("busybox"); err == nil {
		shells = append(shells, shell{label: "busybox-sh", bin: bin, args: []string{"sh"}})
	} else {
		t.Log("busybox not in PATH; skipping busybox shell (CI provides it)")
	}

	// bash and zsh are POSIX supersets; wisp's output runs under both (zsh via the
	// header word-split shim). Run each directly on the script. As with busybox,
	// absence is logged and skipped, never failed; CI provides them.
	if bin, err := exec.LookPath("bash"); err == nil {
		shells = append(shells, shell{label: "bash", bin: bin})
	} else {
		t.Log("bash not in PATH; skipping bash shell (CI provides it)")
	}
	if bin, err := exec.LookPath("zsh"); err == nil {
		// -f (NO_RCS) disables ALL startup files. zsh sources ~/.zshenv and
		// /etc/zshenv even for non-interactive script runs (only ~/.zshrc is
		// interactive-only), so -f is required to keep the harness hermetic on a
		// developer machine.
		shells = append(shells, shell{label: "zsh", bin: bin, args: []string{"-f"}})
	} else {
		t.Log("zsh not in PATH; skipping zsh shell (CI provides it)")
	}
	return shells
}

// shellReservesBuiltin reports whether the given shell resolves name as one
// of its own builtins (e.g. zsh's own `print`), rather than searching $PATH
// for an external command. Probes live via `type` (POSIX-required, present
// in every shell in this repo's matrix) rather than a hardcoded per-shell
// list, so the answer stays correct on whatever shell build CI actually
// runs. Takes the full shell struct, not just its binary path, because
// busybox-sh's leading args (sh.args == []string{"sh"}) select the sh
// applet -- invoking the bare busybox binary without them dispatches on
// argv[0] alone and does not run a shell at all. A probe-launch failure is
// treated as "reserved" (fail closed / skip the fixture) rather than
// silently defaulting to "not reserved", since a missing/non-executable
// shell binary otherwise produces a confusing unrelated failure downstream
// instead of a clean skip.
func shellReservesBuiltin(sh shell, name string) bool {
	args := append(append([]string{}, sh.args...), "-c", `type "$1" 2>&1`, "--", name)
	out, err := exec.Command(sh.bin, args...).CombinedOutput()
	if err != nil && len(out) == 0 {
		return true
	}
	return strings.Contains(strings.ToLower(string(out)), "builtin")
}

func runUnder(t *testing.T, sh shell, script []byte, spec fixtureSpec) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "out.sh")
	if err := os.WriteFile(path, script, 0o755); err != nil {
		t.Fatal(err)
	}
	cmdArgs := append(append([]string{}, sh.args...), path)
	cmdArgs = append(cmdArgs, spec.Args...)
	cmd := exec.Command(sh.bin, cmdArgs...)
	// Run in the per-fixture temp dir so the I/O fixtures (read_file/write_file/
	// append_file with relative paths) are hermetic: their writes land in this
	// throwaway directory, isolated from the repo and cleaned up by t.TempDir().
	// Existing fixtures do not touch the filesystem or depend on cwd, so this is
	// a no-op for them.
	cmd.Dir = dir
	if spec.StubPath != "" {
		stubDir, err := filepath.Abs(filepath.Join(fixtureDir, spec.StubPath))
		if err != nil {
			t.Fatalf("resolving stubPath: %v", err)
		}
		env := os.Environ()
		filtered := make([]string, 0, len(env))
		var existingPath string
		for _, kv := range env {
			if after, ok := strings.CutPrefix(kv, "PATH="); ok {
				existingPath = after
			} else {
				filtered = append(filtered, kv)
			}
		}
		existingPath = strings.TrimLeft(existingPath, ":")
		segments := []string{stubDir}
		if spec.IsolatePath && spec.RequiresExternal != "" {
			segments = append(segments, bridgeCoreUtils(t, "mktemp", "cat", "rm"))
			existingPath = excludePathDirsContaining(existingPath, spec.RequiresExternal)
		}
		if existingPath != "" {
			segments = append(segments, existingPath)
		}
		cmd.Env = append(filtered, "PATH="+strings.Join(segments, ":"))
	}
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if spec.Stdin != "" {
		cmd.Stdin = strings.NewReader(spec.Stdin)
	}
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("run under %s: %v", sh.label, err)
		}
	}
	return out.String(), errb.String(), code
}

// excludePathDirsContaining returns pathList with any directory entry removed
// that contains a file named name. Used by IsolatePath fixtures to guarantee
// no real external `name` is reachable via $PATH, without also removing the
// (unrelated) directories that provide coreutils like mktemp/cat/rm that
// __wisp_run_full's implementation itself shells out to -- dropping the whole
// host $PATH (as an earlier version of this design did) breaks those helper
// internals before the fixture can even observe the intended "not found"
// outcome.
func excludePathDirsContaining(pathList, name string) string {
	var kept []string
	for _, dir := range strings.Split(pathList, ":") {
		if dir == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			continue
		}
		kept = append(kept, dir)
	}
	return strings.Join(kept, ":")
}

// bridgeCoreUtils symlinks the named coreutils into a fresh temp directory,
// resolved from the test process's own real, unfiltered $PATH. Placing this
// directory ahead of the (possibly filtered) real $PATH in cmd.Env
// guarantees these utilities stay resolvable even when
// excludePathDirsContaining removes their real directory because it also
// happens to contain the fixture's excluded external command (e.g. a
// hypothetical system where /usr/bin ships both `print` and `mktemp`).
func bridgeCoreUtils(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		real, err := exec.LookPath(name)
		if err != nil {
			t.Fatalf("bridgeCoreUtils: %s not found on PATH: %v", name, err)
		}
		if err := os.Symlink(real, filepath.Join(dir, name)); err != nil {
			t.Fatalf("bridgeCoreUtils: symlink %s: %v", name, err)
		}
	}
	return dir
}

func shellcheckScript(t *testing.T, script []byte) {
	t.Helper()
	sc, err := exec.LookPath("shellcheck")
	if err != nil {
		t.Log("shellcheck not available; skipping lint gate")
		return
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "out.sh")
	if err := os.WriteFile(path, script, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(sc, "--shell", "sh", "--severity", "warning", path)
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("shellcheck found issues:\n%s\n--- script ---\n%s", out.String(), script)
	}
}
