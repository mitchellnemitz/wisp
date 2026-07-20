package codegen

import (
	"os"
	"path/filepath"
	"testing"
)

// TestOnExitSignalsEmittedShellByteIdentical is the byte-identity GATE for the
// signals/traps work (Task 1 on_exit + Task 2 on_signal). It compiles a
// representative program that uses other builtins and control flow but NEITHER
// on_exit NOR on_signal and asserts the emitted .sh equals a snapshot captured
// by compiling the same source with the compiler at the branch parent 23fae22
// (git merge-base HEAD main). Because the snapshot is the parent-revision bytes,
// a green run proves that programs not using on_exit/on_signal are byte-identical
// to before this feature (helpers are tree-shaken).
//
// Regenerate the snapshot ONLY when the parent baseline genuinely moves:
// UPDATE_ONEXITSIGNALS_SNAPSHOT=1 go test ./internal/codegen -run TestOnExitSignalsEmittedShellByteIdentical
func TestOnExitSignalsEmittedShellByteIdentical(t *testing.T) {
	const src = `fn greet(name: string) -> void {
  print("hello ${name}")
}
fn main() -> int {
  let x: int = 1 + 2
  let s: string = "world"
  greet(s)
  if (x > 0) {
    print("positive")
  }
  return 0
}`
	got := compile(t, src)
	snap := filepath.Join("testdata", "onexitsignals_byteid.sh")
	if os.Getenv("UPDATE_ONEXITSIGNALS_SNAPSHOT") == "1" {
		if err := os.MkdirAll(filepath.Dir(snap), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(snap, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote snapshot %s (%d bytes)", snap, len(got))
		return
	}
	want, err := os.ReadFile(snap)
	if err != nil {
		t.Fatalf("read snapshot %s: %v (capture from merge-base 23fae22)", snap, err)
	}
	if string(got) != string(want) {
		t.Fatalf("emitted .sh changed (byte-identity gate failed).\n--- got (%d bytes) ---\n%s\n--- want (%d bytes) ---\n%s",
			len(got), got, len(want), want)
	}
}
