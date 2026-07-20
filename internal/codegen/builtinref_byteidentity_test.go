package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuiltinRef_NoUse_ByteIdentical: a program that calls builtins DIRECTLY
// (no builtin-as-value) and uses a USER funcref must emit byte-identical shell
// to before the eta-expansion feature. Reconstructed for the modules-only
// surface: the direct calls are namespaced (string.trim / array.map), and their
// delegate lowering is byte-identical to the pre-removal flat call, so the
// pre-removal snapshot still matches.
//
// Regenerate the snapshot intentionally with:
// UPDATE_BUILTINREF_SNAPSHOT=1 go test ./internal/codegen -run TestBuiltinRef_NoUse_ByteIdentical
func TestBuiltinRef_NoUse_ByteIdentical(t *testing.T) {
	const src = `fn dbl(x: int) -> int { return x * 2 }
fn main() -> int {
  let s: string = string.trim("  x  ")
  let ys: int[] = array.map([1, 2], dbl)
  print(s)
  print("${ys[0]}")
  return 0
}`
	got := compileNS(t, src, "string", "array")
	snap := filepath.Join("testdata", "builtinref_byteidentity.sh")
	if os.Getenv("UPDATE_BUILTINREF_SNAPSHOT") == "1" {
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
		t.Fatalf("read snapshot %s: %v (mint with UPDATE_BUILTINREF_SNAPSHOT=1)", snap, err)
	}
	if string(got) != string(want) {
		t.Fatalf("no-use program .sh changed (byte-identity gate failed).\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestBuiltinRef_TreeShake verifies that:
//   - a program using a builtin DIRECTLY (not as a value) emits no __wisp_builtin_* wrapper, and
//   - a program referencing a member builtin AS A VALUE emits that builtin's
//     wrapper and its underlying helper, but not unrelated wrappers.
//
// Reconstructed with namespaced member references (string.trim / string.upper);
// the funcref-value eta-expansion synthesizes the same __wisp_builtin_trim
// wrapper the pre-removal bare reference did.
func TestBuiltinRef_TreeShake(t *testing.T) {
	noref := string(compileNS(t, `fn main() -> int {
  let s: string = string.trim("x")
  print(s)
  return 0
}`, "string"))
	if strings.Contains(noref, "__wisp_builtin_") {
		t.Error("wrapper emitted for a direct call (no builtin-as-value program should not emit any __wisp_builtin_* wrapper)")
	}

	withref := string(compileNS(t, `fn main() -> int {
  let f: fn(string) -> string = string.trim
  print(f("x"))
  return 0
}`, "string"))
	if !strings.Contains(withref, "__wisp_builtin_trim") {
		t.Error("__wisp_builtin_trim wrapper not emitted when string.trim is used as a value")
	}
	if !strings.Contains(withref, "__wisp_trim") {
		t.Error("__wisp_trim helper not emitted when string.trim is used as a value (wrapper must dep its helper)")
	}
	if strings.Contains(withref, "__wisp_builtin_upper") {
		t.Error("__wisp_builtin_upper wrapper emitted in a program that only references trim (tree-shake failure)")
	}
}
