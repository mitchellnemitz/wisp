package codegen

import "testing"

func TestVoidFnNoLocals(t *testing.T) {
	// A void function with no locals: no `local` line, void return.
	src := `fn noop() -> void {
  return
}
fn main() -> int {
  noop()
  return 0
}`
	_, _, code := runWisp(t, src)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
}

func TestNestedWhileInsideWhileBreak(t *testing.T) {
	// break in an inner while inside an outer while exits only the inner.
	src := `fn main() -> int {
  let i: int = 0
  while (i < 2) {
    let j: int = 0
    while (j < 5) {
      if (j == 1) { break }
      print("i${i}j${j}")
      j = j + 1
    }
    print("outer${i}")
    i = i + 1
  }
  return 0
}`
	wantRun(t, src, "i0j0\nouter0\ni1j0\nouter1\n", "", 0)
}

func TestSwitchIntNegativeCase(t *testing.T) {
	src := `fn main() -> int {
  let v: int = -3
  switch (v) {
    case -3 {
      print("neg-three")
    }
    default {
      print("default")
    }
  }
  return 0
}`
	wantRun(t, src, "neg-three\n", "", 0)
}

func TestEmptyDefaultSwitch(t *testing.T) {
	// switch with an empty default must compile and run (default {}).
	src := `fn main() -> int {
  let v: int = 9
  switch (v) {
    case 1 {
      print("one")
    }
    default {
    }
  }
  print("after")
  return 0
}`
	wantRun(t, src, "after\n", "", 0)
}
