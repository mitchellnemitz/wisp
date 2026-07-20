package types

import (
	"strings"
	"testing"
)

// TestCallTypeArgs_RejectedOnNonConsumers: explicit type args are a clean located
// error on every callee form that does not consume them, and never silently
// ignored. Each case asserts the "does not take type arguments" diagnostic.
func TestCallTypeArgs_RejectedOnNonConsumers(t *testing.T) {
	const helpers = "fn inc(x: int) -> int {\n  return x\n}\n"
	cases := []struct {
		name string
		body string
	}{
		{"builtin_map", "let xs: int[] = [1]\nlet r: int[] = map[int](xs, inc)"},
		{"builtin_unwrap", "let o: Optional[int] = Some(1)\nlet v: int = unwrap[int](o)"},
		{"builtin_push", "let xs: int[] = [1]\npush[int](xs, 2)"},
		{"ctor_some", "let o: Optional[int] = Some[int](1)"},
		{"ctor_ok", "let r: Result[int] = Ok[int](1)"},
		{"reserved_const", "print[int](\"x\")\nlet z: int = stdout[int]()"},
		{"funcref_local", "let g: fn(int) -> int = inc\nlet v: int = g[int](5)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			expectErr(t, helpers+wrapMain(c.body), "does not take type arguments")
		})
	}
}

// TestCallTypeArgs_RejectedOnIndirectCall: a non-identifier callee (indirect
// call) with type args errors.
func TestCallTypeArgs_RejectedOnIndirectCall(t *testing.T) {
	src := "fn getf() -> fn(int) -> int {\n  return getf()\n}\n" +
		wrapMain("let v: int = getf()[int](5)")
	expectErr(t, src, "does not take type arguments")
}

// TestCallTypeArgs_RejectionIsNonCascading: rejecting type args on a builtin still
// lets the call resolve; the only type-args error is the single rejection.
func TestCallTypeArgs_RejectionIsNonCascading(t *testing.T) {
	info := check(t, "fn inc(x: int) -> int {\n  return x\n}\n"+
		wrapMain("let xs: int[] = [1]\nlet r: int[] = map[int](xs, inc)"))
	n := 0
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "does not take type arguments") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("expected exactly 1 type-args rejection, got %d:\n%s", n, diagList(info.Errors))
	}
}
