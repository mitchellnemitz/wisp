package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// env.set / env.unset are removable builtins (bare set_env/unset_env no longer
// resolve in the single-module check), so the programs below compile through
// compileNS with the env and process namespaces bound (process.run drives the
// child echo in TestSetEnv_InjectionInert).

func TestSetEnv_InjectionInert(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "se.sh")
	// set_env a hostile value, then echo it back via a child so it reaches stdout.
	src := "fn main() -> int {\n" +
		"  env.set(\"WISP_INJ\", \"$(touch PWNED); `id`; *\")\n" +
		"  print(process.run([\"sh\", \"-c\", \"printf %s \\\"$WISP_INJ\\\"\"]))\n" +
		"  return 0\n}"
	if err := os.WriteFile(script, compileNS(t, src, "env", "process"), 0o755); err != nil {
		t.Fatal(err)
	}
	const want = "$(touch PWNED); `id`; *\n" // run-result + print newline
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
				t.Errorf("%s: PWNED present (or unexpected stat err %v) -- command substitution may have executed", sh.label, statErr)
			}
		})
	}
}

func TestSetEnv_NameValidation(t *testing.T) {
	invalid := []string{"1BAD", "HAS SPACE", "HAS=EQ", ""}
	for _, sh := range execShells(t) {
		for _, name := range invalid {
			name := name
			t.Run(sh.label+"/invalid/"+name, func(t *testing.T) {
				// Build the wisp source with the name as a wisp string literal.
				src := "fn main() -> int {\n  env.set(" + wispStr(name) + ", \"x\")\n  return 0\n}"
				dir := t.TempDir()
				script := filepath.Join(dir, "v.sh")
				if err := os.WriteFile(script, compileNS(t, src, "env"), 0o755); err != nil {
					t.Fatal(err)
				}
				args := append(append([]string{}, sh.args...), script)
				cmd := exec.Command(sh.bin, args...)
				cmd.Dir = t.TempDir()
				var stderr strings.Builder
				cmd.Stderr = &stderr
				err := cmd.Run()
				if err == nil {
					t.Errorf("%s: set_env(%q) should abort non-zero", sh.label, name)
				}
				if !strings.Contains(stderr.String(), "set_env: invalid environment variable name") {
					t.Errorf("%s: set_env(%q) stderr = %q, want located invalid-name", sh.label, name, stderr.String())
				}
			})
		}
		// unset_env invalid name.
		t.Run(sh.label+"/unset-invalid", func(t *testing.T) {
			src := "fn main() -> int {\n  env.unset(\"1BAD\")\n  return 0\n}"
			dir := t.TempDir()
			script := filepath.Join(dir, "u.sh")
			if err := os.WriteFile(script, compileNS(t, src, "env"), 0o755); err != nil {
				t.Fatal(err)
			}
			args := append(append([]string{}, sh.args...), script)
			cmd := exec.Command(sh.bin, args...)
			cmd.Dir = t.TempDir()
			var stderr strings.Builder
			cmd.Stderr = &stderr
			if cmd.Run() == nil {
				t.Errorf("%s: unset_env(\"1BAD\") should abort non-zero", sh.label)
			}
			if !strings.Contains(stderr.String(), "unset_env: invalid environment variable name") {
				t.Errorf("%s: unset_env stderr = %q, want located invalid-name", sh.label, stderr.String())
			}
		})
		// valid edges succeed.
		t.Run(sh.label+"/valid", func(t *testing.T) {
			src := "fn main() -> int {\n  env.set(\"_OK\", \"1\")\n  env.set(\"A1_B2\", \"2\")\n  return 0\n}"
			dir := t.TempDir()
			script := filepath.Join(dir, "ok.sh")
			if err := os.WriteFile(script, compileNS(t, src, "env"), 0o755); err != nil {
				t.Fatal(err)
			}
			args := append(append([]string{}, sh.args...), script)
			cmd := exec.Command(sh.bin, args...)
			cmd.Dir = t.TempDir()
			if err := cmd.Run(); err != nil {
				t.Errorf("%s: valid names should succeed: %v", sh.label, err)
			}
		})
	}
}

// wispStr returns a wisp string literal for s. Sufficient for ASCII names that
// contain only [A-Za-z0-9_ =] and the empty string (none contain " or \).
func wispStr(s string) string {
	return "\"" + s + "\""
}
