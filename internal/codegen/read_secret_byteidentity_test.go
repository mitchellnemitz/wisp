package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadSecret_HelperShape(t *testing.T) {
	sh := string(compile(t, `fn main() -> int {
  let r: Optional[string] = read_secret("pw: ")
  return 0
}`))
	for _, want := range []string{
		"__wisp_read_secret() {",
		`printf '%s' "$1" >&2`,                // prompt to stderr
		`stty -g 2>/dev/null`,                 // save (AC4)
		`stty -echo 2>/dev/null`,              // suppress (AC4)
		`stty "$__wisp_rs_saved" 2>/dev/null`, // restore (AC4)
		`IFS= read -r __wisp_rs_line`,
		`__wisp_rs_rc=$?`, // rc captured (before restore)
	} {
		if !strings.Contains(sh, want) {
			t.Errorf("emitted shell missing %q", want)
		}
	}
	// Extract the helper body for ORDERING assertions. Guard against a panic if the
	// helper is absent (the positive loop already errored; bail cleanly, no slice panic).
	hi := strings.Index(sh, "__wisp_read_secret() {")
	if hi < 0 {
		return // helper missing -> already reported by the positive loop above
	}
	body := sh[hi:]
	if end := strings.Index(body, "\n}"); end >= 0 {
		body = body[:end]
	}
	// Helper for "A appears strictly before B" in the body.
	before := func(a, b string) bool {
		ia, ib := strings.Index(body, a), strings.Index(body, b)
		return ia >= 0 && ib >= 0 && ia < ib
	}
	readNeedle := `IFS= read -r`
	rcNeedle := `__wisp_rs_rc=$?`
	restoreNeedle := `stty "$__wisp_rs_saved"`
	nlNeedle := `printf '\n' >&2`
	branchNeedle := `if [ "$__wisp_rs_rc"`
	// AC1: prompt printed BEFORE the read.
	if !before(`printf '%s' "$1" >&2`, readNeedle) {
		t.Errorf("prompt must be emitted before the read")
	}
	// AC4: stty -echo BEFORE the read (else echo is suppressed too late on a real TTY).
	if !before(`stty -echo 2>/dev/null`, readNeedle) {
		t.Errorf("stty -echo must run before the read")
	}
	// R2: rc captured immediately after read and BEFORE the restore (else rc = stty's exit -> EOF misclassified).
	if !before(readNeedle, rcNeedle) || !before(rcNeedle, restoreNeedle) {
		t.Errorf("rc must be captured after the read and before the stty restore")
	}
	// AC5: the stty restore AND the trailing newline run BEFORE the EOF branch (so EOF never leaves echo off).
	if !before(restoreNeedle, branchNeedle) {
		t.Errorf("stty restore must run before the EOF branch")
	}
	if !before(nlNeedle, branchNeedle) {
		t.Errorf("trailing newline must run before the EOF branch")
	}
	// Total: no __wisp_fail in the helper body (read_secret never aborts).
	if strings.Contains(body, "__wisp_fail") {
		t.Errorf("read_secret must be total (no __wisp_fail)")
	}
}

// TestReadSecret_NoUse_ByteIdentical: a program using read_line (NOT read_secret)
// emits shell byte-identical to before this feature (AC6). Mint at the merge-base:
//
//	UPDATE_READ_SECRET_SNAPSHOT=1 go test ./internal/codegen -run TestReadSecret_NoUse_ByteIdentical
func TestReadSecret_NoUse_ByteIdentical(t *testing.T) {
	const src = `fn main() -> int {
  match (read_line()) {
    case Some(v) { print(v) }
    case None { print("none") }
  }
  return 0
}`
	got := compile(t, src)
	snap := filepath.Join("testdata", "read_secret_byteidentity.sh")
	if os.Getenv("UPDATE_READ_SECRET_SNAPSHOT") == "1" {
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
		t.Errorf("emitted shell drifted; re-mint at the merge-base with UPDATE_READ_SECRET_SNAPSHOT=1 if intentional")
	}
	if strings.Contains(string(got), "__wisp_read_secret") {
		t.Errorf("__wisp_read_secret leaked into a program that does not call read_secret")
	}
}
