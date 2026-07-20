package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProcess_SpawnHelperShape pins the __wisp_spawn helper shape. Reconstructed
// with namespaced process.spawn / process.wait.
func TestProcess_SpawnHelperShape(t *testing.T) {
	sh := string(compileNS(t, `fn main() -> int {
  let p: Process = process.spawn(["echo","hi"])
  let r: RunResult = process.wait(p)
  return 0
}`, "process"))
	for _, want := range []string{
		"10000000",       // concrete pid-capture cap
		"IFS= read -r",   // sentinel-gated spin (not bare [ -s ])
		"*[!0-9]*",       // digit guard
		"printf '%s\\n'", // newline-sentinel pid publication
	} {
		if !strings.Contains(sh, want) {
			t.Errorf("spawn helper missing %q in emitted shell", want)
		}
	}
}

// TestProcess_NoUse_ByteIdentical is the byte-identity GATE: the inline program
// uses the run family (process.run / process.run_full) but NONE of the six
// background-process builtins, so a green run proves the Process type machinery +
// the six helpers are tree-shaken out of no-use programs (AC7 / N2). The
// namespaced delegate lowering is byte-identical to the pre-removal flat call, so
// the pre-removal snapshot still matches.
//
// Regenerate intentionally with:
// UPDATE_PROCESS_SNAPSHOT=1 go test ./internal/codegen -run TestProcess_NoUse_ByteIdentical
func TestProcess_NoUse_ByteIdentical(t *testing.T) {
	const src = `fn main() -> int {
  let out: string = process.run(["echo", "hi"])
  let r: RunResult = process.run_full(["echo", "world"])
  print(out)
  print(r.stdout)
  print("code=${r.code}")
  return 0
}`
	got := compileNS(t, src, "process")
	snap := filepath.Join("testdata", "process_byteidentity.sh")
	if os.Getenv("UPDATE_PROCESS_SNAPSHOT") == "1" {
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

func TestProcess_NoUse_TreeShaken(t *testing.T) {
	sh := string(compileNS(t, `fn main() -> int {
  let r: RunResult = process.run_full(["echo","hi"])
  print(r.stdout)
  return 0
}`, "process"))
	for _, helper := range []string{
		"__wisp_spawn()", "__wisp_wait()", "__wisp_is_done()",
		"__wisp_signal()", "__wisp_wait_any()", "__wisp_make_fifo()",
	} {
		if strings.Contains(sh, helper) {
			t.Errorf("helper %q emitted in a program that does not use it (tree-shake failure)", helper)
		}
	}
}
