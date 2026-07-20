package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSymlinkForce_HelperShape pins the __wisp_symlink_force helper byte-shape.
// Reconstructed with the namespaced fs.symlink_force.
func TestSymlinkForce_HelperShape(t *testing.T) {
	sh := string(compileNS(t, `fn main() -> int {
  fs.symlink_force("t", "l")
  return 0
}`, "fs"))
	for _, want := range []string{
		"__wisp_symlink_force() {",
		`rm -f -- "$3" && ln -s -- "$2" "$3"`,
		`symlink_force: failed`,
	} {
		if !strings.Contains(sh, want) {
			t.Errorf("emitted shell missing %q", want)
		}
	}
	// The args must never be re-evaluated: assert no eval of the target/link_path.
	if strings.Contains(sh, "eval") && strings.Contains(sh, "symlink_force") {
		// eval anywhere in the symlink_force helper would be a red flag; the helper
		// has no eval. (The prelude has eval elsewhere; scope the check to the helper body.)
		body := sh[strings.Index(sh, "__wisp_symlink_force() {"):]
		if end := strings.Index(body, "\n}"); end >= 0 {
			body = body[:end]
		}
		if strings.Contains(body, "eval") {
			t.Errorf("__wisp_symlink_force must not use eval")
		}
	}
}

// TestSymlinkForce_NoUse_ByteIdentical: a program using symlink (NOT symlink_force)
// emits shell byte-identical to before this feature -> __wisp_symlink_force is
// tree-shaken (AC7). The namespaced delegate lowering is byte-identical to the
// pre-removal flat call, so the pre-removal snapshot still matches. Regenerate with:
//
//	UPDATE_SYMLINK_FORCE_SNAPSHOT=1 go test ./internal/codegen -run TestSymlinkForce_NoUse_ByteIdentical
func TestSymlinkForce_NoUse_ByteIdentical(t *testing.T) {
	const src = `fn main() -> int {
  fs.symlink("t", "l")
  return 0
}`
	got := compileNS(t, src, "fs")
	snap := filepath.Join("testdata", "symlink_force_byteidentity.sh")
	if os.Getenv("UPDATE_SYMLINK_FORCE_SNAPSHOT") == "1" {
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
		t.Errorf("emitted shell drifted; re-mint with UPDATE_SYMLINK_FORCE_SNAPSHOT=1 if intentional")
	}
	if strings.Contains(string(got), "__wisp_symlink_force") {
		t.Errorf("__wisp_symlink_force leaked into a program that does not call symlink_force")
	}
}
