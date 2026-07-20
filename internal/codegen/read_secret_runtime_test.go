package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadSecret_Injection (AC3) confirms that hostile metacharacters fed via
// stdin to read_secret are returned as inert data -- no command substitution,
// no glob expansion, no PWNED file created.
func TestReadSecret_Injection(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "rs_inj.sh")
	src := `fn main() -> int {
  match (read_secret("pw: ")) {
    case Some(v) { print(v) }
    case None { print("none") }
  }
  return 0
}`
	if err := os.WriteFile(script, compile(t, src), 0o755); err != nil {
		t.Fatal(err)
	}
	const hostile = "$(touch PWNED); `id`; *\n"
	const wantOut = "$(touch PWNED); `id`; *\n"
	for _, sh := range execShells(t) {
		t.Run(sh.label, func(t *testing.T) {
			run := t.TempDir()
			// Plant a decoy so glob * would find something if expansion ran.
			if err := os.WriteFile(filepath.Join(run, "decoy"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
			args := append(append([]string{}, sh.args...), script)
			cmd := exec.Command(sh.bin, args...)
			cmd.Dir = run
			cmd.Stdin = strings.NewReader(hostile)
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if string(out) != wantOut {
				t.Errorf("%s: stdout = %q, want %q", sh.label, out, wantOut)
			}
			if _, statErr := os.Stat(filepath.Join(run, "PWNED")); !os.IsNotExist(statErr) {
				t.Errorf("%s: PWNED file present (stat err=%v) -- command substitution may have executed", sh.label, statErr)
			}
		})
	}
}
