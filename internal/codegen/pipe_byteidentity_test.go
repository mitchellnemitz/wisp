package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPipe_HelperShape pins the __wisp_pipe helper byte-shape. Reconstructed with
// the namespaced process.pipe; the delegate emits the same __wisp_pipe helper the
// pre-removal bare call did.
func TestPipe_HelperShape(t *testing.T) {
	sh := string(compileNS(t, `fn main() -> int {
  let r: RunResult = process.pipe([["echo","hi"], ["cat"]])
  return 0
}`, "process"))
	// POSITIVE: the load-bearing tokens must be present.
	for _, want := range []string{
		"*[!0-9]*",                      // digit guard on the stage id
		"</dev/null",                    // first-stage stdin redirect (per-segment)
		`>\"\$__wisp_pp_t1\"`,           // last-stage stdout redirect: temp-path VAR REFERENCE (escaped \$), expands at eval time -- C2 fix, no path value in the eval string
		`2>\"\$__wisp_pp_t2\"`,          // last-stage stderr redirect: temp-path VAR REFERENCE (escaped \$), expands at eval time -- C2 fix
		`eval "$__wisp_pp_str"`,         // eval of the constructed string VARIABLE (not inlined data)
		`__wisp_pp_seg="$__wisp_pp_seg`, // redirects are appended to the SEGMENT var (per-segment), not the pipeline
	} {
		if !strings.Contains(sh, want) {
			t.Errorf("pipe helper missing %q in emitted shell", want)
		}
	}
	// NEGATIVE (real assertion, not a no-op): the redirect must be built into a SEGMENT,
	// never applied to the eval'd whole pipeline. There must be NO `eval "$__wisp_pp_str" >`
	// or `eval "$__wisp_pp_str" 2>` (a trailing whole-pipeline redirect, which binds
	// non-portably between zsh and dash/bash/busybox).
	for _, forbidden := range []string{
		`eval "$__wisp_pp_str" >`,
		`eval "$__wisp_pp_str" 2>`,
	} {
		if strings.Contains(sh, forbidden) {
			t.Errorf("pipe helper has a trailing whole-pipeline redirect %q -- redirects MUST be per-segment", forbidden)
		}
	}
}

// TestPipe_NoUse_ByteIdentical: a program using the run family (process.run /
// process.run_full) but NOT pipe must emit byte-identical shell to before this
// feature (tree-shake / N2). The namespaced delegate lowering is byte-identical
// to the pre-removal flat call, so the pre-removal snapshot still matches.
// Regenerate with: UPDATE_PIPE_SNAPSHOT=1 go test ./internal/codegen -run TestPipe_NoUse_ByteIdentical
func TestPipe_NoUse_ByteIdentical(t *testing.T) {
	const src = `fn main() -> int {
  let out: string = process.run(["echo", "hi"])
  let r: RunResult = process.run_full(["echo", "world"])
  print(out)
  print(r.stdout)
  print("code=${r.code}")
  return 0
}`
	got := compileNS(t, src, "process")
	snap := filepath.Join("testdata", "pipe_byteidentity.sh")
	if os.Getenv("UPDATE_PIPE_SNAPSHOT") == "1" {
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

func TestPipe_NoUse_TreeShaken(t *testing.T) {
	sh := string(compileNS(t, `fn main() -> int {
  let r: RunResult = process.run_full(["echo","hi"])
  print(r.stdout)
  return 0
}`, "process"))
	for _, helper := range []string{"__wisp_pipe()", "__wisp_pipe_exec()"} {
		if strings.Contains(sh, helper) {
			t.Errorf("helper %q emitted in a program that does not use pipe (tree-shake failure)", helper)
		}
	}
}
