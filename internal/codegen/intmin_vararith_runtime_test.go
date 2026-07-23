package codegen

// INT_MIN (-9223372036854775808) held in a VARIABLE (not a bare literal operand)
// and used in arithmetic must evaluate identically and correctly on every non-zsh
// shell. Before the arith() bare-operand fix, the variable reached $(( )) as
// $name, whose string-expanded 2^63 magnitude dash re-lexed one past INT_MAX
// (silent off-by-one) and zsh truncated. The literal-operand path (covered by
// TestIntMin_ArithRuntime) was already correct; this covers the variable/sum
// paths that were not. zsh is skipped: its $(( )) cannot represent 2^63 even from
// a variable -- documented residual.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runShellsExpect compiles src and asserts stdout == want on every non-zsh shell.
func runShellsExpect(t *testing.T, name, src, want string) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, name+".sh")
	if err := os.WriteFile(script, compile(t, src), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, sh := range execShells(t) {
		sh := sh
		if sh.label == "zsh" {
			continue
		}
		t.Run(sh.label, func(t *testing.T) {
			args := append(append([]string{}, sh.args...), script)
			out, err := exec.Command(sh.bin, args...).Output()
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if string(out) != want {
				t.Errorf("%s: stdout = %q, want %q", sh.label, out, want)
			}
		})
	}
}

func TestIntMin_VarArithRuntime(t *testing.T) {
	const decl = "  let m: int = -9223372036854775808\n"
	cases := []struct {
		name, expr, want string
	}{
		{"add_zero", "m + 0", "-9223372036854775808\n"},
		{"and_neg1", "m & -1", "-9223372036854775808\n"},
		{"mul_neg1", "m * -1", "-9223372036854775808\n"}, // wraparound, representable
		{"shr_one", "m >> 1", "-4611686018427387904\n"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			src := "fn main() -> int {\n" + decl +
				"  print(to_string(" + tc.expr + "))\n" +
				"  return 0\n}"
			runShellsExpect(t, tc.name, src, tc.want)
		})
	}
}

func TestIntMin_SumRuntime(t *testing.T) {
	// array.sum needs the array module; compile through the module-aware genCore
	// harness (namespace bound to a synthetic core module), not single-file
	// compile(). genSum's int accumulator ($(( acc + e )) over user elements) is
	// the second value-carrying arith site fixed alongside arith().
	src := "fn main() -> int {\n" +
		"  let xs: int[] = [-9223372036854775808, 0]\n" +
		"  print(to_string(array.sum(xs)))\n" +
		"  return 0\n}"
	got := []byte(genCore(t, src, map[string]int{"array": 1}, "array"))
	dir := t.TempDir()
	script := filepath.Join(dir, "sum_intmin.sh")
	if err := os.WriteFile(script, got, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, sh := range execShells(t) {
		sh := sh
		if sh.label == "zsh" {
			continue
		}
		t.Run(sh.label, func(t *testing.T) {
			args := append(append([]string{}, sh.args...), script)
			out, err := exec.Command(sh.bin, args...).Output()
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if string(out) != "-9223372036854775808\n" {
				t.Errorf("%s: stdout = %q, want %q", sh.label, out, "-9223372036854775808\n")
			}
		})
	}
}

// INT_MIN % -1 has the representable result 0 (unlike INT_MIN / -1, whose
// quotient 2^63 is unrepresentable and faults). It must be 0, not a crash, on
// every shell and architecture. On arm64 all shells already return 0; the act/CI
// run is the x86 check (x86 idiv can SIGFPE on INT_MIN % -1). If x86 diverges,
// genIntDivMod gains a guard short-circuiting INT_MIN % -1 to 0 (see the plan).
func TestIntMin_ModNegOne(t *testing.T) {
	src := "fn main() -> int {\n" +
		"  let m: int = -9223372036854775808\n" +
		"  let d: int = -1\n" +
		"  print(to_string(m % d))\n" +
		"  return 0\n}"
	runShellsExpect(t, "mod_neg1", src, "0\n")
}

// INT_MIN / -1 has an unrepresentable quotient and must fault located
// ("division overflow"), nonzero exit, identically on every shell. This guards
// the div path against the bare-operand change; it already passes (the guard
// detects INT_MIN by a `[ -eq ]` numeric test before any $(( ))).
func TestIntMin_DivNegOneFaults(t *testing.T) {
	src := "fn main() -> int {\n" +
		"  let m: int = -9223372036854775808\n" +
		"  let d: int = -1\n" +
		"  print(to_string(m / d))\n" +
		"  return 0\n}"
	dir := t.TempDir()
	script := filepath.Join(dir, "div_neg1.sh")
	if err := os.WriteFile(script, compile(t, src), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, sh := range execShells(t) {
		sh := sh
		if sh.label == "zsh" {
			continue
		}
		t.Run(sh.label, func(t *testing.T) {
			args := append(append([]string{}, sh.args...), script)
			out, err := exec.Command(sh.bin, args...).CombinedOutput()
			if err == nil {
				t.Fatalf("%s: expected nonzero exit for INT_MIN / -1, got success; output=%q", sh.label, out)
			}
			if !strings.Contains(string(out), "division overflow") {
				t.Errorf("%s: want 'division overflow' in output, got %q", sh.label, out)
			}
		})
	}
}

// TestIntMin_VarArithBareOperand: a user int variable used as an arithmetic
// operand must be referenced BARE inside $(( )) (reads the stored value, dash-
// safe for INT_MIN), never as $name (string-expands and re-lexes 2^63).
func TestIntMin_VarArithBareOperand(t *testing.T) {
	src := "fn main() -> int {\n" +
		"  let m: int = -9223372036854775808\n" +
		"  print(to_string(m + 0))\n" +
		"  return 0\n}"
	got := string(compile(t, src))
	if !strings.Contains(got, "$(( __wisp_v_1 + 0 ))") {
		t.Errorf("want bare operand `$(( __wisp_v_1 + 0 ))` in output:\n%s", got)
	}
	if strings.Contains(got, "$(( $__wisp_v_1") {
		t.Errorf("found dollar-form arith operand `$(( $__wisp_v_1` (dash-unsafe for INT_MIN):\n%s", got)
	}
}
