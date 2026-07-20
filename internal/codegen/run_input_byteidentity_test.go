package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunInput_HelperShape pins the __wisp_run_input / __wisp_run_input_full
// helper byte-shapes. Reconstructed with namespaced process.run_input /
// process.run_input_full.
func TestRunInput_HelperShape(t *testing.T) {
	sh := string(compileNS(t, `fn main() -> int {
  let s: string = process.run_input(["cat"], "hi")
  let r: RunResult = process.run_input_full(["cat"], "hi")
  return 0
}`, "process"))
	for _, want := range []string{
		"__wisp_run_input() {",
		"__wisp_run_input_full() {",
		`printf '%s' "$__wisp_ri_stdin" | "$@"`,
		`printf '%s' "$__wisp_rif_stdin" | "$@" > "$__wisp_rif_t1" 2> "$__wisp_rif_t2"`,
		`run_input: $1 exited with status`,
	} {
		if !strings.Contains(sh, want) {
			t.Errorf("emitted shell missing %q", want)
		}
	}
}

// TestRunInput_NoUse_ByteIdentical: a program using process.run / process.run_full
// but NOT run_input* emits shell byte-identical to before this feature (AC7). The
// namespaced delegate lowering is byte-identical to the pre-removal flat call, so
// the pre-removal snapshot still matches. Regenerate with:
//
//	UPDATE_RUN_INPUT_SNAPSHOT=1 go test ./internal/codegen -run TestRunInput_NoUse_ByteIdentical
func TestRunInput_NoUse_ByteIdentical(t *testing.T) {
	const src = `fn main() -> int {
  let s: string = process.run(["echo", "hi"])
  let r: RunResult = process.run_full(["echo", "world"])
  print(s)
  return 0
}`
	got := compileNS(t, src, "process")
	snap := filepath.Join("testdata", "run_input_byteidentity.sh")
	if os.Getenv("UPDATE_RUN_INPUT_SNAPSHOT") == "1" {
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
		t.Errorf("emitted shell drifted; re-mint with UPDATE_RUN_INPUT_SNAPSHOT=1 if intentional")
	}
	for _, helper := range []string{"__wisp_run_input", "__wisp_run_input_full"} {
		if strings.Contains(string(got), helper) {
			t.Errorf("%s leaked into a program that does not call run_input*", helper)
		}
	}
}
