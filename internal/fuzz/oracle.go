// internal/fuzz/oracle.go
package fuzz

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/mitchellnemitz/wisp/internal/driver"
	"github.com/mitchellnemitz/wisp/internal/testrunner"
)

type ShellRun struct {
	Label  string
	Stdout []byte
	Exit   int
}

type OracleResult struct {
	Diverged  bool
	Unstable  bool
	Runs      []ShellRun
	CarvedZsh bool
	Detail    string
}

// missingShells returns the expected shells absent from the discovered set.
func missingShells(shells []testrunner.Shell) []string {
	want := map[string]bool{"dash": false, "busybox-sh": false, "bash": false, "zsh": false}
	for _, s := range shells {
		if _, ok := want[s.Label]; ok {
			want[s.Label] = true
		}
	}
	var missing []string
	for _, label := range []string{"dash", "busybox-sh", "bash", "zsh"} {
		if !want[label] {
			missing = append(missing, label)
		}
	}
	return missing
}

// RequirePrereqs returns the discovered shells, failing fast unless ALL FOUR are
// present AND the compiler works (FR-019). AvailableShells silently omits an
// absent shell, so completeness is asserted here; a compiler smoke-compile fails
// fast before any generation.
func RequirePrereqs() ([]testrunner.Shell, error) {
	shells := testrunner.AvailableShells()
	if m := missingShells(shells); len(m) > 0 {
		return nil, fmt.Errorf("required shells missing: %v (install them; no reduced matrix)", m)
	}
	if _, err := compileOnce("fn main() -> int {\n  print(\"${1 + 1}\")\n  return 0\n}\n"); err != nil {
		return nil, fmt.Errorf("compiler preflight failed: %w", err)
	}
	return shells, nil
}

// compareRuns reports whether the runs disagree on raw stdout bytes or exit status
// (byte-exact, no normalization -- FR-007). carveZsh excludes zsh for the
// documented int64-boundary residual (FR-015 Design-B + the max-side extension,
// see intmin.go); non-zsh shells must still agree.
func compareRuns(runs []ShellRun, carveZsh bool) (bool, string) {
	var ref *ShellRun
	for i := range runs {
		if carveZsh && runs[i].Label == "zsh" {
			continue
		}
		if ref == nil {
			ref = &runs[i]
			continue
		}
		if runs[i].Exit != ref.Exit || !bytes.Equal(runs[i].Stdout, ref.Stdout) {
			return true, fmt.Sprintf("%s vs %s: exit %d/%d, stdout %q/%q",
				ref.Label, runs[i].Label, ref.Exit, runs[i].Exit, ref.Stdout, runs[i].Stdout)
		}
	}
	return false, ""
}

func compileOnce(src string) ([]byte, error) {
	script, _, diags := driver.Compile("fuzz.wisp", src)
	for _, d := range diags {
		if d.Severity == driver.Error {
			return nil, fmt.Errorf("compile error: %s", d.String())
		}
	}
	if len(script) == 0 {
		return nil, fmt.Errorf("no script produced")
	}
	return script, nil
}

// runUnder executes script under one shell, mirroring internal/testrunner's
// invocation (exec sh.Bin with sh.Args before the script path; signal death -> 1).
// The environment is fixed for byte-determinism (FR-007/FR-016): locale pinned to
// LC_ALL=C, and the host PATH inherited so the prelude's coreutils (awk/printf) are
// found wherever they live (Homebrew/Nix/BSD layouts vary; a hardcoded
// /usr/bin:/bin can make every program fail or -- worse -- diverge when one shell
// has a builtin and another needs the external). PATH is IDENTICAL across all four
// shells in a run, so it cannot manufacture a cross-shell divergence, and it is
// stable within the process, so same-seed determinism (SC-002) holds; only the shell
// (and thus the compiler's output) is the variable under test.
func runUnder(sh testrunner.Shell, scriptPath string) ShellRun {
	args := append(append([]string{}, sh.Args...), scriptPath)
	cmd := exec.Command(sh.Bin, args...)
	cmd.Env = []string{"LC_ALL=C", "PATH=" + os.Getenv("PATH")}
	var out bytes.Buffer
	cmd.Stdout = &out
	exit := 0
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exit = ee.ExitCode()
			if exit < 0 {
				exit = 1
			}
		} else {
			exit = 1
		}
	}
	return ShellRun{Label: sh.Label, Stdout: out.Bytes(), Exit: exit}
}

// execUnder is the seam runOracleSource dispatches through; production is runUnder,
// a CI test swaps it to assert every shell runs the SAME artifact path (SC-015).
var execUnder = runUnder

// RunOracle compiles p once, runs it under every shell, and applies both oracles.
// The zsh carve-out decision is computed structurally (with data flow) from the IR:
// a program is carved when a value at the int64 boundary (min or max side) can
// reach `$(( ))` arithmetic -- the documented zsh residual (see intmin.go).
func RunOracle(shells []testrunner.Shell, p *Program) (OracleResult, error) {
	return runOracleSource(shells, Print(p), programReachesBoundaryArith(p))
}

// runOracleSource is the shared execution core: given source and an explicit carve
// decision, it compiles once, checks recompile stability, runs all shells, and
// compares. Replay uses it with the stored source + stored carve flag.
func runOracleSource(shells []testrunner.Shell, src string, carveZsh bool) (OracleResult, error) {
	script, err := compileOnce(src)
	if err != nil {
		return OracleResult{}, err
	}
	script2, err := compileOnce(src)
	if err != nil {
		return OracleResult{}, err
	}
	res := OracleResult{CarvedZsh: carveZsh}
	if !bytes.Equal(script, script2) {
		res.Unstable = true
		res.Detail = "recompile produced a different artifact"
		return res, nil
	}
	tmp, err := os.CreateTemp("", "wisp-fuzz-*.sh")
	if err != nil {
		return OracleResult{}, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(script); err != nil {
		tmp.Close()
		return OracleResult{}, err
	}
	if err := tmp.Close(); err != nil {
		return OracleResult{}, err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return OracleResult{}, err
	}
	// Every shell executes the SAME compiled bytes: one compile, one on-disk
	// artifact, every shell invoked against that one path (single artifact -- SC-015).
	// execUnder is a seam so a CI test can assert the same-path contract without shells.
	for _, sh := range shells {
		res.Runs = append(res.Runs, execUnder(sh, tmp.Name()))
	}
	if carveZsh {
		log.Printf("fuzz: INT-boundary-arith carve-out excluding zsh for this program")
	}
	res.Diverged, res.Detail = compareRuns(res.Runs, carveZsh)
	return res, nil
}
