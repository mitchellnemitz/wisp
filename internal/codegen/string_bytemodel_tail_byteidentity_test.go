package codegen

import (
	"os"
	"path/filepath"
	"testing"
)

// TestStringByteModelTail_NoUse_ByteIdentical: a program using arithmetic,
// string interpolation, and print -- but NONE of the four helpers rewritten in
// the string-byte-model-tail branch (contains, replace, replace_first,
// ends_with) and none of substring/char_at/index_of/last_index_of either --
// must emit byte-identical shell to before the string-byte-model-tail branch.
//
// Snapshot re-minted ON-BRANCH for the INT_MIN arith-form fix (bare variable refs
// inside $(( )): $(( $x )) -> $(( x ))): this program's `x + 1 - 1` arithmetic
// emits arith() operands, so that fix intentionally flips them dollar->bare and
// the merge-base anchor no longer holds. The only permitted delta from the merge-
// base snapshot is that dollar->bare flip on arith operands; nothing else changed.
// Regenerate ON-BRANCH:
// UPDATE_STRTAIL_SNAPSHOT=1 go test ./internal/codegen -run TestStringByteModelTail_NoUse_ByteIdentical
func TestStringByteModelTail_NoUse_ByteIdentical(t *testing.T) {
	const src = `fn main() -> int {
  let x: int = 6 * 7
  let s: string = "answer"
  let y: int = x + 1 - 1
  let ok: bool = y == 42
  print("${s} is ${x}")
  print("ok=${ok}")
  return 0
}`
	got := compile(t, src)
	snap := filepath.Join("testdata", "string_bytemodel_tail_byteidentity.sh")
	if os.Getenv("UPDATE_STRTAIL_SNAPSHOT") == "1" {
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
