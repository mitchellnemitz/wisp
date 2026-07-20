package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestRunInput_Injection proves that hostile data in both the stdin argument
// and an argv element is passed inertly -- no command substitution executes
// and no side-effect file is created. process.run_input is a removable builtin
// (bare run_input no longer resolves in the single-module check), so the
// program compiles through compileNS with the process namespace bound.
func TestRunInput_Injection(t *testing.T) {
	t.Run("hostile_stdin", func(t *testing.T) {
		// stdin value contains command substitution, backtick, semicolon, and glob.
		// cat echoes it; print adds a newline.  No PWNED file must appear.
		src := "fn main() -> int {\n" +
			"  print(process.run_input([\"cat\"], \"$(touch PWNED); `id`; *\"))\n" +
			"  return 0\n}"
		dir := t.TempDir()
		script := filepath.Join(dir, "ri_stdin.sh")
		if err := os.WriteFile(script, compileNS(t, src, "process"), 0o755); err != nil {
			t.Fatal(err)
		}
		const want = "$(touch PWNED); `id`; *\n"
		for _, sh := range execShells(t) {
			t.Run(sh.label, func(t *testing.T) {
				run := t.TempDir()
				if err := os.WriteFile(filepath.Join(run, "decoy"), nil, 0o644); err != nil {
					t.Fatal(err)
				}
				args := append(append([]string{}, sh.args...), script)
				cmd := exec.Command(sh.bin, args...)
				cmd.Dir = run
				out, err := cmd.Output()
				if err != nil {
					t.Fatalf("run: %v", err)
				}
				if string(out) != want {
					t.Errorf("%s: stdout = %q, want inert literal %q", sh.label, out, want)
				}
				if _, statErr := os.Stat(filepath.Join(run, "PWNED")); !os.IsNotExist(statErr) {
					t.Errorf("%s: PWNED present (or unexpected stat err %v) -- stdin command substitution may have executed", sh.label, statErr)
				}
			})
		}
	})

	t.Run("hostile_argv", func(t *testing.T) {
		// argv element contains command substitution.  printf %s emits it literally;
		// print adds a newline.  No PWNED2 file must appear.
		src := "fn main() -> int {\n" +
			"  print(process.run_input([\"printf\", \"%s\", \"$(touch PWNED2)\"], \"ignored\"))\n" +
			"  return 0\n}"
		dir := t.TempDir()
		script := filepath.Join(dir, "ri_argv.sh")
		if err := os.WriteFile(script, compileNS(t, src, "process"), 0o755); err != nil {
			t.Fatal(err)
		}
		const want = "$(touch PWNED2)\n"
		for _, sh := range execShells(t) {
			t.Run(sh.label, func(t *testing.T) {
				run := t.TempDir()
				if err := os.WriteFile(filepath.Join(run, "decoy"), nil, 0o644); err != nil {
					t.Fatal(err)
				}
				args := append(append([]string{}, sh.args...), script)
				cmd := exec.Command(sh.bin, args...)
				cmd.Dir = run
				out, err := cmd.Output()
				if err != nil {
					t.Fatalf("run: %v", err)
				}
				if string(out) != want {
					t.Errorf("%s: stdout = %q, want inert literal %q", sh.label, out, want)
				}
				if _, statErr := os.Stat(filepath.Join(run, "PWNED2")); !os.IsNotExist(statErr) {
					t.Errorf("%s: PWNED2 present (or unexpected stat err %v) -- argv command substitution may have executed", sh.label, statErr)
				}
			})
		}
	})
}
