package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSetStdin_HelperShape pins the __wisp_set_stdin helper shape. set_stdin
// stays flat (it is not a modularized member), so it keeps its bare spelling and
// compiles single-module.
func TestSetStdin_HelperShape(t *testing.T) {
	sh := string(compile(t, `fn main() -> int {
  set_stdin("yes\n")
  return 0
}`))
	for _, want := range []string{
		"__wisp_set_stdin() {",
		"local __wisp_ss_tmp",
		`__ret="$(mktemp)" || __wisp_fail "$1" "set_stdin: mktemp failed"`,
		`__wisp_ss_tmp="$__ret"`,
		`printf '%s' "$2" > "$__wisp_ss_tmp" || __wisp_fail "$1" "set_stdin: cannot write stdin buffer"`,
		`exec 0< "$__wisp_ss_tmp" || __wisp_fail "$1" "set_stdin: cannot reopen stdin"`,
		`rm -f "$__wisp_ss_tmp"`,
	} {
		if !strings.Contains(sh, want) {
			t.Errorf("emitted shell missing %q", want)
		}
	}
	// The void-located call shape: no __ret read after the call.
	if !strings.Contains(sh, `__wisp_set_stdin '`) {
		t.Errorf("emitted shell missing the located call shape:\n%s", sh)
	}
}

// TestSetStdin_NoUse_ByteIdentical: a program that does NOT call set_stdin emits
// shell byte-identical to before this feature, proving the helper is tree-shaken.
// The no-use program uses process.run (byte-identical delegate lowering), so the
// pre-removal snapshot still matches. Regenerate with:
//
//	UPDATE_SET_STDIN_SNAPSHOT=1 go test ./internal/codegen -run TestSetStdin_NoUse_ByteIdentical
func TestSetStdin_NoUse_ByteIdentical(t *testing.T) {
	const src = `fn main() -> int {
  let out: string = process.run(["echo", "hi"])
  print(out)
  return 0
}`
	got := compileNS(t, src, "process")
	snap := filepath.Join("testdata", "set_stdin_byteidentity.sh")
	if os.Getenv("UPDATE_SET_STDIN_SNAPSHOT") == "1" {
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
		t.Errorf("emitted shell drifted from snapshot; re-mint with UPDATE_SET_STDIN_SNAPSHOT=1 if intentional")
	}
	if strings.Contains(string(got), "__wisp_set_stdin") {
		t.Errorf("__wisp_set_stdin leaked into a program that does not call set_stdin")
	}
}
