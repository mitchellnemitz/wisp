package codegen

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPowEmittedShellByteIdentical is the byte-identity GATE for the extended-math
// work (T1), reconstructed for the modules-only surface. It compiles a fixed pow
// program -- a float-exponent call and an integer-exponent call, now spelled
// math.pow -- and asserts the emitted .sh equals the checked-in snapshot. The
// snapshot was captured pre-removal from the byte-identical flat lowering (pow ==
// math.pow delegate); the golden harness only checks runtime stdout, not the .sh
// bytes, so this test pins the bytes so the shared exp/ln awk-fragment refactor
// cannot silently change pow's emitted shell (AC3 / N5).
//
// Regenerate the snapshot intentionally with:
// UPDATE_POW_SNAPSHOT=1 go test ./internal/codegen -run TestPowEmittedShellByteIdentical
func TestPowEmittedShellByteIdentical(t *testing.T) {
	const src = `fn main() -> int {
  let a: float = math.pow(2.0, 0.5)
  let b: float = math.pow(2.0, 10.0)
  print("${a}")
  print("${b}")
  return 0
}`
	got := compileNS(t, src, "math")

	snap := filepath.Join("testdata", "pow_emitted.sh")
	if os.Getenv("UPDATE_POW_SNAPSHOT") == "1" {
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
		t.Fatalf("read snapshot %s: %v (run with UPDATE_POW_SNAPSHOT=1 to create)", snap, err)
	}
	if string(got) != string(want) {
		t.Fatalf("pow emitted .sh changed (byte-identity gate failed).\n--- got (%d bytes) ---\n%s\n--- want (%d bytes) ---\n%s",
			len(got), got, len(want), want)
	}
}
