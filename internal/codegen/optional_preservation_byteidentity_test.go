package codegen

// SC-016: generalizing the shared tag/value lowering and genMatchStmt for enums
// must not change the shell emitted for an Optional/Result program. This is a
// development-aid drift catcher (behavioral preservation is the real requirement,
// verified by the existing Optional/Result suite); re-mint freely with:
//
//	UPDATE_OPTIONAL_PRESERVATION_SNAPSHOT=1 go test ./internal/codegen -run TestOptionalPreservation_ByteIdentical

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOptionalPreservation_ByteIdentical(t *testing.T) {
	const src = `fn main() -> int {
  let s: Optional[int] = Some(42)
  let n: Optional[int] = None
  let r: Result[int] = Ok(10)
  match (s) {
    case Some(v) { print(to_string(v)) }
    case None { print("none") }
  }
  match (r) {
    case Ok(v) { print(to_string(v)) }
    case Err(e) { print(debug(e)) }
  }
  print(debug(n))
  return 0
}`
	got := string(compile(t, src)) // compile returns []byte (codegen_test.go:16); wrap for string compare
	snap := filepath.Join("testdata", "optional_preservation_byteidentity.sh")
	if os.Getenv("UPDATE_OPTIONAL_PRESERVATION_SNAPSHOT") == "1" {
		if err := os.WriteFile(snap, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote snapshot %s", snap)
		return
	}
	want, err := os.ReadFile(snap)
	if err != nil {
		t.Fatalf("read snapshot %s: %v (mint with UPDATE_OPTIONAL_PRESERVATION_SNAPSHOT=1)", snap, err)
	}
	if got != string(want) {
		t.Errorf("Optional/Result emission drifted (SC-016). Re-mint only if the change is intentional and behavior is preserved.")
	}
}
