package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// process.exec_command is a removable builtin (bare exec_command no longer
// resolves in the single-module check), so the two runtime tests below compile
// through compileNS/runNS in a linked module set with the process namespace
// bound. The delegate lowering is byte-identical to the pre-removal flat call,
// so the runtime behavior (process replacement, injection inertness) is
// unchanged.

// execShells returns the shells to run compiled scripts under. dash is required;
// bash/zsh/busybox are added when present. Mirrors the golden harness discovery.
func execShells(t *testing.T) []struct {
	label string
	bin   string
	args  []string
} {
	t.Helper()
	var out []struct {
		label string
		bin   string
		args  []string
	}
	if bin, err := exec.LookPath("dash"); err == nil {
		out = append(out, struct {
			label string
			bin   string
			args  []string
		}{"dash", bin, nil})
	} else {
		t.Skip("dash not available")
	}
	// busybox runs the script as `busybox sh <script>` (ash), matching the golden
	// harness's busybox-sh label -- required for the full four-shell coverage.
	if bin, err := exec.LookPath("busybox"); err == nil {
		out = append(out, struct {
			label string
			bin   string
			args  []string
		}{"busybox-sh", bin, []string{"sh"}})
	}
	if bin, err := exec.LookPath("bash"); err == nil {
		out = append(out, struct {
			label string
			bin   string
			args  []string
		}{"bash", bin, nil})
	}
	if bin, err := exec.LookPath("zsh"); err == nil {
		out = append(out, struct {
			label string
			bin   string
			args  []string
		}{"zsh", bin, []string{"-f"}})
	}
	return out
}

// TestExecCommand_SamePID (AC1b) proves process REPLACEMENT, distinguishing exec
// from a wrong spawn+exit impl. The compiled script's process is exec.Command's
// Process.Pid; after a correct exec, the exec'd `sh -c 'echo $$ > pidfile'` IS
// that same process, so pidfile == Process.Pid. A child+exit impl would write a
// child pid (different), failing the assertion.
func TestExecCommand_SamePID(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "ec.sh")
	src := `fn main() -> int {
  process.exec_command(["sh", "-c", "echo $$ > pidfile"])
  return 0
}`
	if err := os.WriteFile(script, compileNS(t, src, "process"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, sh := range execShells(t) {
		t.Run(sh.label, func(t *testing.T) {
			run := t.TempDir()
			args := append(append([]string{}, sh.args...), script)
			cmd := exec.Command(sh.bin, args...)
			cmd.Dir = run
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			scriptPID := cmd.Process.Pid
			if err := cmd.Wait(); err != nil {
				t.Fatalf("script run: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(run, "pidfile"))
			if err != nil {
				t.Fatalf("read pidfile: %v", err)
			}
			got, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				t.Fatalf("parse pid %q: %v", data, err)
			}
			if got != scriptPID {
				t.Errorf("%s: exec'd pid %d != script pid %d (process not replaced -- spawn+exit?)", sh.label, got, scriptPID)
			}
		})
	}
}

// TestExecCommand_InjectionNoFile (AC5) confirms no PWNED file is created in the
// run directory: the argv reaches the exec'd printf as inert data.
func TestExecCommand_InjectionNoFile(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "inj.sh")
	src := "fn main() -> int {\n  process.exec_command([\"printf\", \"%s\", \"$(touch PWNED); `id`; *\"])\n  return 0\n}"
	if err := os.WriteFile(script, compileNS(t, src, "process"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, sh := range execShells(t) {
		t.Run(sh.label, func(t *testing.T) {
			run := t.TempDir()
			args := append(append([]string{}, sh.args...), script)
			cmd := exec.Command(sh.bin, args...)
			cmd.Dir = run
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if string(out) != "$(touch PWNED); `id`; *" {
				t.Errorf("%s: stdout = %q, want inert literal", sh.label, out)
			}
			if _, err := os.Stat(filepath.Join(run, "PWNED")); err == nil {
				t.Errorf("%s: PWNED file created -- command substitution executed", sh.label)
			}
		})
	}
}
