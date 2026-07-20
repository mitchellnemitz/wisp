package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// pipeTestSrc is a multi-stage pipe program whose compiled output exercises
// __wisp_pipe. Shape mirrors pipe_two_stage.wisp. pipe is a modularized member
// (process.pipe); its delegate lowering is byte-identical to the pre-removal flat
// pipe, so the emitted __wisp_pipe prelude -- the subject of these tests -- is
// unchanged.
const pipeTestSrc = `fn main() -> int {
  let r: RunResult = process.pipe([["echo", "hello"], ["tr", "a-z", "A-Z"]])
  print(r.stdout)
  return 0
}
`

// TestPipe_EvalStringNoPathInterpolation (AC5) asserts the emitted __wisp_pipe
// prelude uses escaped variable references for the two temp paths, so no path
// VALUE ever enters the eval string. Also asserts structural presence of the
// bad-stage-id guard (AC7b(c)).
func TestPipe_EvalStringNoPathInterpolation(t *testing.T) {
	script := string(compileNS(t, pipeTestSrc, "process"))

	// (1) The post-fix escaped form must be present. The prelude raw-string
	// emits the literal bytes >\"\$__wisp_pp_t1\" into the generated script:
	// backslash-quote, backslash-dollar, name, backslash-quote.
	const wantEscaped = `>\"\$__wisp_pp_t1\" 2>\"\$__wisp_pp_t2\"`
	if !strings.Contains(script, wantEscaped) {
		t.Errorf("generated script does not contain escaped redirect form %q", wantEscaped)
	}

	// (2) The pre-fix bare-dollar form must be absent. Pre-fix the script
	// contained >\"$__wisp_pp_t1\" (no backslash before the dollar).
	// Post-fix the dollar is preceded by a backslash (\$), so the bare form
	// must not appear.
	const badUnescaped = `>\"` + `$` + `__wisp_pp_t1\"`
	if strings.Contains(script, badUnescaped) {
		t.Errorf("generated script still contains pre-fix bare-dollar redirect %q -- var-ref fix not applied", badUnescaped)
	}

	// (3) No file-descriptor form must appear.
	for _, fdForm := range []string{">&3", ">&4", "exec 3>", "exec 4>"} {
		if strings.Contains(script, fdForm) {
			t.Errorf("generated script contains fd form %q -- unexpected fd approach", fdForm)
		}
	}

	// (AC7b(c)) The bad-stage-id guard must still be present (defense-in-depth;
	// unreachable from .wisp source so only structural presence is verified).
	const badStageGuard = "pipe: bad stage id"
	if !strings.Contains(script, badStageGuard) {
		t.Errorf("generated script missing bad-stage-id guard %q -- var-ref fix may have removed it", badStageGuard)
	}
}

// TestPipe_HostileTMPDIR (AC7) runs a compiled pipe program with a hostile
// TMPDIR whose directory name embeds a backtick command substitution. Asserts
// no SENTINEL file is created (always). Also probes mktemp behavior: if
// mktemp honors TMPDIR, asserts the pipe ran correctly (b); otherwise skips (b)
// with a recorded reason.
func TestPipe_HostileTMPDIR(t *testing.T) {
	script := filepath.Join(t.TempDir(), "pipe_hostile.sh")
	if err := os.WriteFile(script, compileNS(t, pipeTestSrc, "process"), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, sh := range execShells(t) {
		sh := sh
		t.Run(sh.label, func(t *testing.T) {
			// Absolute sentinel path -- baked in so the injection is not a no-op.
			sentinel := filepath.Join(t.TempDir(), "SENTINEL")

			// Hostile dir name: contains a literal " and a backtick command
			// substitution that would touch the absolute sentinel path.
			hostileName := "x\"" + "`touch " + sentinel + "`" + "y"
			hostileDir := filepath.Join(t.TempDir(), hostileName)
			if err := os.MkdirAll(hostileDir, 0o755); err != nil {
				t.Fatalf("create hostile dir: %v", err)
			}

			// Probe: does mktemp honor TMPDIR in this shell?
			probeArgs := append(append([]string{}, sh.args...), "-c", "mktemp")
			probeCmd := exec.Command(sh.bin, probeArgs...)
			probeCmd.Env = append(os.Environ(), "TMPDIR="+hostileDir)
			probeOut, probeErr := probeCmd.Output()
			mktempHonors := probeErr == nil && strings.HasPrefix(strings.TrimSpace(string(probeOut)), hostileDir)

			// Run the pipe program under the hostile TMPDIR.
			args := append(append([]string{}, sh.args...), script)
			cmd := exec.Command(sh.bin, args...)
			cmd.Env = append(os.Environ(), "TMPDIR="+hostileDir)
			out, runErr := cmd.Output()

			// (a) ALWAYS assert: sentinel must not exist.
			if _, statErr := os.Stat(sentinel); !os.IsNotExist(statErr) {
				t.Errorf("%s: SENTINEL %s exists or unexpected stat error %v -- injection executed", sh.label, sentinel, statErr)
			}

			// (b) Conditional on mktemp honoring TMPDIR.
			if mktempHonors {
				if runErr != nil {
					t.Errorf("%s: pipe run failed under honored hostile TMPDIR: %v", sh.label, runErr)
				}
				const wantStdout = "HELLO\n\n"
				if string(out) != wantStdout {
					t.Errorf("%s: stdout = %q, want %q", sh.label, out, wantStdout)
				}
			} else {
				t.Logf("%s: mktemp does not honor TMPDIR (macOS); skipping live-path assertion (b); (a) still ran", sh.label)
			}
		})
	}
}

// TestPipe_TempCleanup (AC6 cleanup) asserts that __wisp_pipe does not leave
// temp files behind. Only runs where mktemp honors TMPDIR (Linux/CI); skips
// with a recorded reason on macOS.
func TestPipe_TempCleanup(t *testing.T) {
	script := filepath.Join(t.TempDir(), "pipe_cleanup.sh")
	if err := os.WriteFile(script, compileNS(t, pipeTestSrc, "process"), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, sh := range execShells(t) {
		sh := sh
		t.Run(sh.label, func(t *testing.T) {
			cleanDir := t.TempDir()

			// Probe: does mktemp honor TMPDIR?
			probeArgs := append(append([]string{}, sh.args...), "-c", "mktemp")
			probeCmd := exec.Command(sh.bin, probeArgs...)
			probeCmd.Env = append(os.Environ(), "TMPDIR="+cleanDir)
			probeOut, probeErr := probeCmd.Output()
			probePath := strings.TrimSpace(string(probeOut))
			mktempHonors := probeErr == nil && strings.HasPrefix(probePath, cleanDir)

			if !mktempHonors {
				t.Logf("%s: mktemp does not honor TMPDIR (macOS); skipping cleanup assertion; cleanup logic is shell-identical, Linux/CI is the binding gate", sh.label)
				return
			}

			// Remove the probe's own temp file so cleanDir is empty again.
			if err := os.Remove(probePath); err != nil && !os.IsNotExist(err) {
				t.Fatalf("remove probe file: %v", err)
			}

			// Run the pipe program; it should not leave temp files behind.
			args := append(append([]string{}, sh.args...), script)
			cmd := exec.Command(sh.bin, args...)
			cmd.Env = append(os.Environ(), "TMPDIR="+cleanDir)
			if err := cmd.Run(); err != nil {
				t.Fatalf("%s: pipe run failed: %v", sh.label, err)
			}

			// cleanDir should be empty: __wisp_pipe deleted both mktemp files.
			entries, err := os.ReadDir(cleanDir)
			if err != nil {
				t.Fatalf("read cleanDir: %v", err)
			}
			if len(entries) != 0 {
				names := make([]string, len(entries))
				for i, e := range entries {
					names[i] = e.Name()
				}
				t.Errorf("%s: cleanDir not empty after pipe run; leftover files: %v", sh.label, names)
			}
		})
	}
}
