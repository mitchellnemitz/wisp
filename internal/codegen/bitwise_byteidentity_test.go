package codegen

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBitwise_NoUse_ByteIdentical: a program using arithmetic/comparison/logical
// but NONE of the bitwise operators & | ^ << >> must emit byte-identical shell to
// before this feature. The Task-1 precedence-ladder renumber must not change the
// emitted output for existing operators. Snapshot minted by the merge-base
// compiler (git merge-base HEAD main) -- non-circular: the on-branch test passing
// proves the renumber did NOT change emitted shell for a non-bitwise program.
// Regenerate (ONLY from a merge-base worktree):
// UPDATE_BITWISE_SNAPSHOT=1 go test ./internal/codegen -run TestBitwise_NoUse_ByteIdentical
func TestBitwise_NoUse_ByteIdentical(t *testing.T) {
	const src = `fn main() -> int {
  let a: int = 1 + 2 * 3
  let b: bool = a == 7 && a > 0
  let c: int = (a - 1) / 2 + a % 3
  print("${a}")
  print("${b}")
  print("${c}")
  return 0
}`
	got := compile(t, src)
	snap := filepath.Join("testdata", "bitwise_byteidentity.sh")
	if os.Getenv("UPDATE_BITWISE_SNAPSHOT") == "1" {
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
		t.Fatalf("read snapshot %s: %v", snap, err)
	}
	if string(got) != string(want) {
		t.Fatalf("no-use program .sh changed (byte-identity gate failed).\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
