package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPidAlive_HelperShape pins the __wisp_pid_alive helper body shape.
// Reconstructed with the namespaced call process.pid_alive.
func TestPidAlive_HelperShape(t *testing.T) {
	sh := string(compileNS(t, `fn main() -> int {
  let b: bool = process.pid_alive(1)
  return 0
}`, "process"))
	for _, want := range []string{
		"__wisp_pid_alive() {",
		`kill -0 "$1" 2>/dev/null`,
		`__ret=true`,
		`__ret=false`,
	} {
		if !strings.Contains(sh, want) {
			t.Errorf("emitted shell missing %q", want)
		}
	}
}

// TestPidAlive_NoUse_ByteIdentical: a program NOT calling pid_alive emits shell
// byte-identical to before this feature (AC5). The no-use program is spelled with
// fs.file_exists (byte-identical delegate lowering), so the pre-removal snapshot
// still matches. Regenerate with:
//
//	UPDATE_PID_ALIVE_SNAPSHOT=1 go test ./internal/codegen -run TestPidAlive_NoUse_ByteIdentical
func TestPidAlive_NoUse_ByteIdentical(t *testing.T) {
	const src = `fn main() -> int {
  let b: bool = fs.file_exists("/tmp")
  print(to_string(b))
  return 0
}`
	got := compileNS(t, src, "fs")
	snap := filepath.Join("testdata", "pid_alive_byteidentity.sh")
	if os.Getenv("UPDATE_PID_ALIVE_SNAPSHOT") == "1" {
		if err := os.WriteFile(snap, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote snapshot %s (%d bytes)", snap, len(got))
		return
	}
	want, err := os.ReadFile(snap)
	if err != nil {
		t.Fatalf("read snapshot %s: %v", snap, err)
	}
	if string(got) != string(want) {
		t.Errorf("emitted shell drifted; re-mint with UPDATE_PID_ALIVE_SNAPSHOT=1 if intentional")
	}
	if strings.Contains(string(got), "__wisp_pid_alive") {
		t.Errorf("__wisp_pid_alive leaked into a program that does not call pid_alive")
	}
}
