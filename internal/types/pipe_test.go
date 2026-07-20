package types

import "testing"

// process.pipe is a removable builtin (bare pipe no longer resolves in the
// single-module check), so this test checks through the linked module set
// with the process namespace bound.
func TestPipe_AC0(t *testing.T) {
	// valid: string[][] -> RunResult
	expectOKNS(t, `fn main() -> int {
  let r: RunResult = process.pipe([["echo","hi"], ["cat"]])
  print(r.stdout)
  print("${r.code}")
  return 0
}`, "process")
	// valid: runtime stage count (non-literal)
	expectOKNS(t, `fn main() -> int {
  let s: string[][] = [["echo","hi"], ["cat"]]
  let r: RunResult = process.pipe(s)
  print(r.stdout)
  return 0
}`, "process")

	// arity
	expectErrNS(t, `fn main()->int{ let r:RunResult=process.pipe(); return 0 }`, "pipe", "process")
	expectErrNS(t, `fn main()->int{ let r:RunResult=process.pipe([["a"]],[["b"]]); return 0 }`, "pipe", "process")
	// type: must be string[][], not string[]
	expectErrNS(t, `fn main()->int{ let r:RunResult=process.pipe(["a","b"]); return 0 }`, "string[][]", "process")
	expectErrNS(t, `fn main()->int{ let r:RunResult=process.pipe("x"); return 0 }`, "string[][]", "process")
	expectErrNS(t, `fn main()->int{ let r:RunResult=process.pipe([1]); return 0 }`, "string[][]", "process")
	// result is RunResult, not int
	expectErrNS(t, `fn main()->int{ let x:int=process.pipe([["a"]]); return 0 }`, "", "process")
	// "pipe" is a removable builtin: its flat name was freed by the modules-only
	// migration (isReservedName excludes the removable set), so it is now an
	// ordinary identifier a user may bind -- unlike the pre-removal original,
	// which reserved it as a bare builtin name.
	expectOKNS(t, `fn main()->int{ let pipe:int=1; return 0 }`, "process")
	expectOKNS(t, `fn pipe()->int{ return 0 }
fn main()->int{ return 0 }`, "process")
}
