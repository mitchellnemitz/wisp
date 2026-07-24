package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// process.pid_alive is a removable builtin whose bare call no longer resolves
// in the single-module codegen check, so pidAliveProg and the three runtime
// tests below compile through compileNS in a linked module set with
// process/env bound.
const pidAliveProg = `fn main() -> int {
  print(to_string(process.pid_alive(unwrap_or(parse_int(unwrap_or(env.get("WISP_TEST_PID"), "0")), 0))))
  return 0
}`

// TestPidAlive_Live (AC1): a known-live pid (a child the test owns) -> true.
func TestPidAlive_Live(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "pa.sh")
	if err := os.WriteFile(script, compileNS(t, pidAliveProg, "process", "env"), 0o755); err != nil {
		t.Fatal(err)
	}
	live := exec.Command("sleep", "30")
	if err := live.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = live.Process.Kill(); _ = live.Wait() }()
	for _, sh := range execShells(t) {
		t.Run(sh.label, func(t *testing.T) {
			args := append(append([]string{}, sh.args...), script)
			cmd := exec.Command(sh.bin, args...)
			cmd.Dir = t.TempDir()
			cmd.Env = append(os.Environ(), "WISP_TEST_PID="+strconv.Itoa(live.Process.Pid))
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if strings.TrimSpace(string(out)) != "true" {
				t.Errorf("%s: pid_alive(live %d) = %q, want true", sh.label, live.Process.Pid, out)
			}
		})
	}
}

// TestPidAlive_Dead (AC2): a fork+reaped (now-dead) pid -> false.
func TestPidAlive_Dead(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "pa.sh")
	if err := os.WriteFile(script, compileNS(t, pidAliveProg, "process", "env"), 0o755); err != nil {
		t.Fatal(err)
	}
	dead := exec.Command("true")
	if err := dead.Start(); err != nil {
		t.Fatal(err)
	}
	pid := dead.Process.Pid
	if err := dead.Wait(); err != nil {
		t.Fatal(err)
	}
	for _, sh := range execShells(t) {
		t.Run(sh.label, func(t *testing.T) {
			args := append(append([]string{}, sh.args...), script)
			cmd := exec.Command(sh.bin, args...)
			cmd.Dir = t.TempDir()
			cmd.Env = append(os.Environ(), "WISP_TEST_PID="+strconv.Itoa(pid))
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if strings.TrimSpace(string(out)) != "false" {
				t.Errorf("%s: pid_alive(dead %d) = %q, want false", sh.label, pid, out)
			}
		})
	}
}

// TestPidAlive_NegativeTotal (AC3): -1 renders a bool, program exits 0 (value
// is environment-defined; do not pin it).
func TestPidAlive_NegativeTotal(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "pa.sh")
	if err := os.WriteFile(script, compileNS(t, pidAliveProg, "process", "env"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, sh := range execShells(t) {
		t.Run(sh.label, func(t *testing.T) {
			args := append(append([]string{}, sh.args...), script)
			cmd := exec.Command(sh.bin, args...)
			cmd.Dir = t.TempDir()
			cmd.Env = append(os.Environ(), "WISP_TEST_PID="+strconv.Itoa(-1))
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("run: %v (program must exit 0 -- total)", err)
			}
			s := strings.TrimSpace(string(out))
			if s != "true" && s != "false" {
				t.Errorf("%s: pid_alive(-1) = %q, want a rendered bool (total)", sh.label, out)
			}
		})
	}
}
