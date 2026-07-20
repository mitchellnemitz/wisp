package codegen

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFloatLiteralDomain_NoChange_ByteIdentical: a program using floats IN-DOMAIN
// (values whose %.17g form is plain decimal, not exponent form) plus int arithmetic
// and bool must emit byte-identical shell to before the domain-check feature was
// added. The checker change is front-end only and must not alter emitted shell for
// valid in-domain programs. Snapshot minted from merge-base b3a054e -- non-circular:
// the on-branch test passing proves the domain checker did NOT change emitted shell.
// Regenerate (ONLY from a merge-base worktree):
// UPDATE_FLOATDOM_SNAPSHOT=1 go test ./internal/codegen -run TestFloatLiteralDomain_NoChange_ByteIdentical
func TestFloatLiteralDomain_NoChange_ByteIdentical(t *testing.T) {
	const src = `fn main() -> int {
  let x: float = 3.14
  let y: float = 0.0005
  let z: float = 100.0
  let n: int = 7
  let ok: bool = n > 0
  print("${x}")
  print("${y}")
  print("${z}")
  print("${ok}")
  return 0
}`
	got := compile(t, src)
	snap := filepath.Join("testdata", "float_literal_domain_byteidentity.sh")
	if os.Getenv("UPDATE_FLOATDOM_SNAPSHOT") == "1" {
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
		t.Fatalf("float-domain (in-domain) program .sh changed (byte-identity gate failed).\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
