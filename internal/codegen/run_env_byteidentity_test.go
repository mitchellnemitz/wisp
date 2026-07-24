package codegen

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRunEnvEmittedShellByteIdentical is the byte-identity GATE for the run_env
// family, reconstructed for the modules-only surface. It compiles a fixed program
// that exercises the existing run family (process.run / process.run_status /
// process.run_full) plus unwrap_or(dict.get(...), fb), but NONE of the three
// run_env builtins (run_env, run_env_status, run_env_full). The namespaced
// delegate lowering is byte-identical to the pre-removal flat call, so the
// pre-removal snapshot still matches; a green run proves the new catalog
// entries, the shared __wisp_run_env_argv builder, and the RunEnv helper are
// tree-shaken out of programs that do not call them (AC5 / N2).
//
// Regenerate the snapshot intentionally with:
// UPDATE_RUNENV_SNAPSHOT=1 go test ./internal/codegen -run TestRunEnvEmittedShellByteIdentical
func TestRunEnvEmittedShellByteIdentical(t *testing.T) {
	const src = `fn main() -> int {
  let out: string = process.run(["echo", "hi"])
  let rc: int = process.run_status(["true"])
  let r: RunResult = process.run_full(["echo", "world"])
  let e: {string: string} = {"K": "v"}
  let v: string = unwrap_or(dict.get(e, "K"), "x")
  print(out)
  print(to_string(rc))
  print(r.stdout)
  print(v)
  return 0
}`
	got := compileNS(t, src, "process", "dict")

	snap := filepath.Join("testdata", "run_env_byteidentity.sh")
	if os.Getenv("UPDATE_RUNENV_SNAPSHOT") == "1" {
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
		t.Fatalf("read snapshot %s: %v (run with UPDATE_RUNENV_SNAPSHOT=1 to create)", snap, err)
	}
	if string(got) != string(want) {
		t.Fatalf("run_env emitted .sh changed (byte-identity gate failed).\n--- got (%d bytes) ---\n%s\n--- want (%d bytes) ---\n%s",
			len(got), got, len(want), want)
	}
}
