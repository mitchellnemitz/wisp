package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExecCommand_HelperShape asserts the emitted helper rebuilds argv the
// run-family way and ENDS at `exec "$@"` (no post-exec line -- a failed exec
// exits the shell, so any trailing statement would be unreachable dead code).
// Reconstructed with the namespaced process.exec_command.
func TestExecCommand_HelperShape(t *testing.T) {
	sh := string(compileNS(t, `fn main() -> int {
  process.exec_command(["echo", "hi"])
  return 0
}`, "process"))
	for _, want := range []string{
		"__wisp_exec_command() {",
		`exec_command: empty argv`,
		`exec "$@"`,
	} {
		if !strings.Contains(sh, want) {
			t.Errorf("emitted shell missing %q", want)
		}
	}
	// `exec "$@"` must be the helper's FINAL statement: the very next non-empty
	// line of the __wisp_exec_command function body is its closing `}`. This is
	// stronger than scanning for a specific abort token -- ANY trailing statement
	// (regardless of wording) would be unreachable dead code after exec.
	body := sh[strings.Index(sh, "__wisp_exec_command() {"):]
	if end := strings.Index(body, "\n}"); end >= 0 {
		body = body[:end] // helper body up to (not including) the closing brace line
	}
	lines := strings.Split(body, "\n")
	last := ""
	for _, ln := range lines {
		if strings.TrimSpace(ln) != "" {
			last = strings.TrimSpace(ln)
		}
	}
	if last != `exec "$@"` {
		t.Errorf("helper's last body statement is %q, want `exec \"$@\"` (no post-exec dead code)", last)
	}
}

// TestExecCommand_NoUse_ByteIdentical is the byte-identity GATE: the inline
// program uses the run family (process.run) but NOT exec_command, so a green run
// proves the __wisp_exec_command helper is tree-shaken out of no-use programs
// (AC6 / N2). The namespaced delegate lowering is byte-identical to the
// pre-removal flat call, so the pre-removal snapshot still matches.
//
// Regenerate with:
// UPDATE_EXEC_COMMAND_SNAPSHOT=1 go test ./internal/codegen -run TestExecCommand_NoUse_ByteIdentical
func TestExecCommand_NoUse_ByteIdentical(t *testing.T) {
	const src = `fn main() -> int {
  let out: string = process.run(["echo", "hi"])
  print(out)
  return 0
}`
	got := compileNS(t, src, "process")
	snap := filepath.Join("testdata", "exec_command_byteidentity.sh")
	if os.Getenv("UPDATE_EXEC_COMMAND_SNAPSHOT") == "1" {
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
		t.Errorf("emitted shell drifted from snapshot; if intentional, re-mint with UPDATE_EXEC_COMMAND_SNAPSHOT=1")
	}
	// Tree-shake: __wisp_exec_command absent from a no-use program.
	if strings.Contains(string(got), "__wisp_exec_command") {
		t.Errorf("__wisp_exec_command leaked into a program that does not call exec_command")
	}
}
