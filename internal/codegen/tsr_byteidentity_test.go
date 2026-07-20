package codegen

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTSRByteIdentical is the byte-identity GATE for the time/sleep/random work
// (T1+T2), reconstructed for the modules-only surface. It compiles a fixed
// program that uses print, math.int_max(), and an if branch -- but NONE of
// now/sleep/random -- and asserts the emitted .sh equals the checked-in snapshot.
// The snapshot was captured pre-removal from the byte-identical flat lowering
// (int_max == math.int_max delegate). The helpers for now/sleep/random are
// tree-shaken, so a program that does not call them produces byte-for-byte
// identical shell; a failure means a newly-added helper or catalog entry
// perturbs output for programs that do not use it.
//
// Regenerate the snapshot intentionally with:
// UPDATE_TSR_SNAPSHOT=1 go test ./internal/codegen -run TestTSRByteIdentical
func TestTSRByteIdentical(t *testing.T) {
	const src = `fn main() -> int {
  let m: int = math.int_max()
  if (m > 0) {
    print("big")
  } else {
    print("small")
  }
  return 0
}`
	got := compileNS(t, src, "math")

	snap := filepath.Join("testdata", "tsr_byteidentity.sh")
	if os.Getenv("UPDATE_TSR_SNAPSHOT") == "1" {
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
		t.Fatalf("read snapshot %s: %v (run with UPDATE_TSR_SNAPSHOT=1 to create)", snap, err)
	}
	if string(got) != string(want) {
		t.Fatalf("tsr emitted .sh changed (byte-identity gate failed).\n--- got (%d bytes) ---\n%s\n--- want (%d bytes) ---\n%s",
			len(got), got, len(want), want)
	}
}
