package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// set_stdin(content) -> void: runtime behavior against read_line/read_stdin
// (Gap 1 design, Testing section). set_stdin, read_line and read_stdin all STAY
// FLAT (they are not in coreCatalog), so these programs compile single-module.
// The read_line/read_stdin consumers are covered by internal/golden
// (io_tail_read_stdin_* fixtures); the two arms below assert set_stdin's per-call
// buffer replacement, which the golden harness cannot express: the empty-buffer
// arm (set_stdin("") must ACTIVELY replace a non-empty ambient stdin) and the
// later-call-replaces-earlier-buffer arm.

func TestSetStdin_Empty(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "ss.sh")
	src := `fn main() -> int {
  set_stdin("")
  let line: Optional[string] = read_line()
  print("line=${to_string(is_some(line))}")
  return 0
}`
	if err := os.WriteFile(script, compile(t, src), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, sh := range execShells(t) {
		t.Run(sh.label, func(t *testing.T) {
			args := append(append([]string{}, sh.args...), script)
			cmd := exec.Command(sh.bin, args...)
			cmd.Dir = t.TempDir()
			// Feed a non-empty ambient stdin so set_stdin("") must ACTIVELY
			// replace it: a no-op would surface "leftover" as Some -> line=true.
			cmd.Stdin = strings.NewReader("leftover\n")
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if string(out) != "line=false\n" {
				t.Errorf("%s: out=%q, want %q", sh.label, out, "line=false\n")
			}
		})
	}

	// read_stdin() on an empty buffer is "".
	dir2 := t.TempDir()
	script2 := filepath.Join(dir2, "ss2.sh")
	src2 := `fn main() -> int {
  set_stdin("")
  let s: string = read_stdin()
  print("[${s}]")
  return 0
}`
	if err := os.WriteFile(script2, compile(t, src2), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, sh := range execShells(t) {
		t.Run(sh.label+"/read_stdin", func(t *testing.T) {
			args := append(append([]string{}, sh.args...), script2)
			cmd := exec.Command(sh.bin, args...)
			cmd.Dir = t.TempDir()
			// Non-empty ambient stdin: set_stdin("") must replace it, else
			// read_stdin() would return "leftover" -> "[leftover]".
			cmd.Stdin = strings.NewReader("leftover\n")
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if string(out) != "[]\n" {
				t.Errorf("%s: out=%q, want %q", sh.label, out, "[]\n")
			}
		})
	}
}

func TestSetStdin_LaterCallReplacesEarlierBuffer(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "ss.sh")
	src := `fn main() -> int {
  set_stdin("first\n")
  set_stdin("second\n")
  let line: Optional[string] = read_line()
  print(unwrap(line))
  return 0
}`
	if err := os.WriteFile(script, compile(t, src), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, sh := range execShells(t) {
		t.Run(sh.label, func(t *testing.T) {
			args := append(append([]string{}, sh.args...), script)
			cmd := exec.Command(sh.bin, args...)
			cmd.Dir = t.TempDir()
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if string(out) != "second\n" {
				t.Errorf("%s: out=%q, want %q", sh.label, out, "second\n")
			}
		})
	}
}
