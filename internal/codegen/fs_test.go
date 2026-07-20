package codegen

import "testing"

// Filesystem + process stdlib codegen behavior. The golden suite covers most
// fs/env builtins end-to-end; this file holds the cases the golden harness
// cannot express. env.get_or's set-but-empty arm needs a per-run environment
// variable, which the golden harness does not provide per fixture, so it runs
// here via runNSDir's env plumbing.

// TestFS_EnvOr_SetButEmpty_ReturnsValue: a set-but-empty variable returns ""
// (present), NOT the fallback (spec 3.5). env_or is modularized as env.get_or;
// the delegate lowers byte-identically to the pre-removal flat env_or, so the
// set-but-empty semantics are unchanged.
func TestFS_EnvOr_SetButEmpty_ReturnsValue(t *testing.T) {
	out, errb, code := runNSDir(t, `fn main() -> int {
  print("[${env.get_or("WISP_EMPTY", "FB")}]")
  return 0
}`, []string{"WISP_EMPTY="}, "env")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb)
	}
	if out != "[]\n" {
		t.Errorf("out=%q, want \"[]\\n\" (set-but-empty returns the value, not the fallback)", out)
	}
}
