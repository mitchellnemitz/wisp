package types

import "testing"

// TestProcess_AC0 pins the Process/RunResult HANDLE-type properties that are
// independent of the (now removed) bare builtin surface: opacity (no ==, no
// interpolation), field access, and void-in-expression rejection. Builtin
// resolution and result types are covered by core_process_test.go; the
// on_signal KILL/STOP/CONT allowlist by on_signal_test.go. Handles are
// constructed via the namespaced process members since the bare spellings
// (spawn/wait/...) no longer resolve.
func TestProcess_AC0(t *testing.T) {
	// Field access: Process exposes only pid; RunResult exposes stdout/stderr/code.
	ok := checkProcessProg(t, `fn main() -> int {
  let p: Process = process.spawn(["a"]);
  let r: RunResult = process.wait(p);
  let n: int = p.pid;
  let o: string = r.stdout;
  let e: string = r.stderr;
  let c: int = r.code;
  return 0
}`)
	if len(ok.Errors) != 0 {
		t.Fatalf("handle field access should type-check; errors: %v", errMsgs(ok))
	}

	for _, c := range []struct{ src, want string }{
		// Process exposes only pid.
		{`fn main() -> int { let p: Process = process.spawn(["a"]); let c: int = p.code; return 0 }`, "Process has no field"},
		// Opacity: no equality on an opaque handle.
		{`fn main() -> int { let p: Process = process.spawn(["a"]); let q: Process = process.spawn(["b"]); if (p == q) { return 1 }; return 0 }`, ""},
		// Opacity: no interpolation of a Process handle.
		{`fn main() -> int { let p: Process = process.spawn(["a"]); print("${p}"); return 0 }`, "opaque"},
		// Opacity: no interpolation of a RunResult handle.
		{`fn main() -> int { let p: Process = process.spawn(["a"]); let r: RunResult = process.wait(p); print("${r}"); return 0 }`, "opaque"},
		// void member in expression position is rejected.
		{`fn main() -> int { let p: Process = process.spawn(["a"]); let x: int = process.signal(p, "TERM"); return 0 }`, "void"},
	} {
		info := checkProcessProg(t, c.src)
		if !hasErr(info, c.want) {
			t.Fatalf("%q: want %q, got %v", c.src, c.want, errMsgs(info))
		}
	}
}
