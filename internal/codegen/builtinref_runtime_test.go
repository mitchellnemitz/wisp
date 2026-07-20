package codegen

import (
	"strings"
	"testing"
)

// Representative end-to-end coverage that a referenced builtin actually RUNS
// correctly through its synthesized __wisp_builtin_<name> wrapper. The
// total-passthrough (trim) and located-through-map (sqrt) shapes are covered by
// the owner file member_funcref_test.go; the remaining wrapper shapes -- void
// located (set_env), the nullary holdouts (log10 / log2 / pi / program_path),
// the overloaded arms (abs / min / max / clamp / sign), and the two located
// failure-forwarding paths (abs arm, read_file) -- are reconstructed below with
// the namespaced spelling. Each namespaced member mints the SAME
// __wisp_builtin_<name> wrapper and funcref type as the pre-removal bare-ident
// path (see resolveQualifiedConst in internal/types/expr.go), so the runtime
// behavior is identical.

// TestBuiltinRef_Runtime_VoidSideEffect: a void located builtin funcref
// (env.set) performs its side effect when invoked indirectly; the change is
// observable via a following env read (env.get).
func TestBuiltinRef_Runtime_VoidSideEffect(t *testing.T) {
	out, errb, code := runNS(t, `fn main() -> int {
  let setter: fn(string, string) -> void = env.set
  setter("WISP_FUNCREF_TEST", "on")
  print(env.get("WISP_FUNCREF_TEST"))
  return 0
}`, "env")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb)
	}
	if strings.TrimRight(out, "\n") != "on" {
		t.Errorf("env.set funcref side effect not observed: env read = %q, want %q", out, "on")
	}
}

// TestBuiltinRef_Runtime_NullaryHoldouts: the four monomorphic holdouts
// (log10, log2, pi, program_path) run correctly through their synthesized
// wrappers, exercising the standalone prelude helpers rather than an inline-only
// lowering.
func TestBuiltinRef_Runtime_NullaryHoldouts(t *testing.T) {
	out, errb, code := runNS(t, `fn main() -> int {
  let l10: fn(float) -> float = math.log10
  let l2: fn(float) -> float = math.log2
  let getPi: fn() -> float = math.pi
  let getPath: fn() -> string = fs.program_path
  print(to_string(l10(100.0)))
  print(to_string(l2(8.0)))
  print(to_string(getPi()))
  print(to_string(string.is_empty(getPath())))
  return 0
}`, "math", "fs", "string")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("output = %q, want 4 lines", out)
	}
	if !strings.HasPrefix(lines[0], "2") && !strings.HasPrefix(lines[0], "1.9999999999") {
		t.Errorf("log10(100) funcref = %q, want ~2", lines[0])
	}
	if !strings.HasPrefix(lines[1], "3") && !strings.HasPrefix(lines[1], "2.9999999999") {
		t.Errorf("log2(8) funcref = %q, want ~3", lines[1])
	}
	if !strings.HasPrefix(lines[2], "3.14159") {
		t.Errorf("pi funcref = %q, want ~3.14159", lines[2])
	}
	if lines[3] != "false" {
		t.Errorf("program_path funcref returned empty; is_empty = %q, want false", lines[3])
	}
}

// TestBuiltinRef_Runtime_OverloadedArms: each int/float arm of the overloaded
// builtins (abs/min/max/clamp/sign) computes correctly when invoked indirectly
// through its arm-suffixed wrapper (__wisp_builtin_<name>_int/_float), the
// funcref counterpart of TestCoreMathDelegateOverloads' direct-call coverage.
func TestBuiltinRef_Runtime_OverloadedArms(t *testing.T) {
	out, errb, code := runNS(t, `fn main() -> int {
  let absI: fn(int) -> int = math.abs
  let absF: fn(float) -> float = math.abs
  let minI: fn(int, int) -> int = math.min
  let minF: fn(float, float) -> float = math.min
  let maxI: fn(int, int) -> int = math.max
  let maxF: fn(float, float) -> float = math.max
  let clampI: fn(int, int, int) -> int = math.clamp
  let clampF: fn(float, float, float) -> float = math.clamp
  let signI: fn(int) -> int = math.sign
  let signF: fn(float) -> int = math.sign
  print(to_string(absI(-5)))
  print(to_string(absF(-5.0)))
  print(to_string(minI(3, 7)))
  print(to_string(minF(3.0, 7.0)))
  print(to_string(maxI(3, 7)))
  print(to_string(maxF(3.0, 7.0)))
  print(to_string(clampI(15, 0, 10)))
  print(to_string(clampF(15.0, 0.0, 10.0)))
  print(to_string(signI(-3)))
  print(to_string(signF(2.5)))
  return 0
}`, "math")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	want := []string{"5", "5", "3", "3", "7", "7", "10", "10", "-1", "1"}
	if len(lines) != len(want) {
		t.Fatalf("output = %q, want %d lines", out, len(want))
	}
	for i, w := range want {
		got := lines[i]
		if got != w && !strings.HasPrefix(got, w+".") {
			t.Errorf("line %d: overload-arm funcref = %q, want ~%q", i, got, w)
		}
	}
}

// TestBuiltinRef_Runtime_AbsArmFailureForwardsBareName: abs's int-arm wrapper
// (__wisp_builtin_abs_int) must inject the BARE name "abs" as the located $1,
// not "abs_int", so an abort through the funcref reports the same diagnostic
// name as a direct math.abs() call (AC4 degradation, arm-invariant).
func TestBuiltinRef_Runtime_AbsArmFailureForwardsBareName(t *testing.T) {
	_, errb, code := runNS(t, `fn main() -> int {
  let absI: fn(int) -> int = math.abs
  print(to_string(absI(-9223372036854775808)))
  return 0
}`, "math")
	if code == 0 {
		t.Fatalf("expected nonzero exit for abs(INT_MIN)")
	}
	if !strings.Contains(errb, "abs(") {
		t.Errorf("failure diagnostic did not forward the bare builtin name %q; stderr = %q", "abs", errb)
	}
	if strings.Contains(errb, "abs_int") {
		t.Errorf("failure diagnostic leaked the arm-suffixed name; stderr = %q", errb)
	}
}

// TestBuiltinRef_Runtime_FailurePathForwardsName: a located wrapper injects the
// builtin name as $1, so a runtime failure through the funcref reports that name
// (AC4 degradation). read_file on a missing path fails and the diagnostic names
// read_file.
func TestBuiltinRef_Runtime_FailurePathForwardsName(t *testing.T) {
	_, errb, code := runNS(t, `fn main() -> int {
  let reader: fn(string) -> string = fs.read_file
  print(reader("/no/such/wisp/funcref/path"))
  return 0
}`, "fs")
	if code == 0 {
		t.Fatalf("expected nonzero exit for read_file of a missing path")
	}
	if !strings.Contains(errb, "read_file") {
		t.Errorf("failure diagnostic did not forward the builtin name; stderr = %q", errb)
	}
}
