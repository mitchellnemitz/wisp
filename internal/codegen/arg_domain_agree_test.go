package codegen

// TestArgDomainAgree: AC5 compile/runtime agreement gate for the integer-argument
// domain checks (spec Section 3.1). For each construct and boundary value, the
// COMPILE decision (checker emits the domain diagnostic on the constant form) must
// equal the RUNTIME decision (the emitted guard aborts when the value is supplied
// NON-constant, so the guard is actually exercised), on every shell in execShells.
//
// INT_MIN (abs/gcd) is carved out -- zsh cannot inject INT_MIN as a runtime literal
// (spec 2.4); abs/gcd agreement is covered by the Task 3 checker reject test plus
// internal/runtime/absgcd_intmin_guard_test.go.
//
// The removable-builtin vectors (repeat/random/format_float/chr/remove_at/
// insert_at) are driven through their namespaced spelling (string.repeat,
// math.random, ...): a vector carrying a non-empty `namespaces` compiles and
// checks in a linked module set (compileRejectsNS / runtimeAbortsNS) instead of
// the single-module path. The checker still enforces the integer-argument domain
// on the namespaced form, and the delegate lowering is byte-identical to the
// pre-removal flat call, so the compile/runtime agreement invariant is exercised
// exactly as before.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/module"
	"github.com/mitchellnemitz/wisp/internal/parser"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// argDomainVector is one boundary case. constSrc supplies the boundary value as a
// compile-time constant (drives the COMPILE decision). nonConstSrc supplies the
// SAME value via a non-constant binding (drives the RUNTIME decision -- the guard
// is emitted and run). wantReject is the expected in-scope classification.
// namespaces, when non-empty, binds the named core namespaces so a namespaced
// member call (string.repeat, math.random, ...) resolves; the vector is then
// checked and compiled in a linked module set.
type argDomainVector struct {
	name        string
	constSrc    string // full program: value as a literal/const expression
	nonConstSrc string // full program: value via a variable, guard emitted at runtime
	wantReject  bool
	namespaces  []string // non-empty -> resolve/compile in a linked module set
}

// nonConstInt wraps a value behind a variable so the probe sees it as non-constant
// and the runtime guard is emitted. A plain `let v: int = <value>` is non-constant
// to the probe (it folds only literals/operators, never identifiers).
func ncIntProgram(call string, value int64) string {
	return fmt.Sprintf("fn main() -> int {\n  let v: int = %d\n  %s\n  return 0\n}", value, call)
}

func constProgram(body string) string {
	return "fn main() -> int {\n  " + body + "\n  return 0\n}"
}

var argDomainVectors = []argDomainVector{
	// #1 div by zero -- benign dividend (7, not INT_MIN) so the overflow guard is not tripped.
	// Boundary 0 with bound +/- 1: -1 and 1 both accept (only 0 rejects).
	{"div_zero", constProgram("let x: int = 7 / 0\n  print(to_string(x))"), ncIntProgram("let x: int = 7 / v\n  print(to_string(x))", 0), true, nil},
	{"div_one", constProgram("let x: int = 7 / 1\n  print(to_string(x))"), ncIntProgram("let x: int = 7 / v\n  print(to_string(x))", 1), false, nil},
	{"div_neg1", constProgram("let x: int = 7 / -1\n  print(to_string(x))"), ncIntProgram("let x: int = 7 / v\n  print(to_string(x))", -1), false, nil},
	// #2 mod by zero (bound +/- 1).
	{"mod_zero", constProgram("let x: int = 7 % 0\n  print(to_string(x))"), ncIntProgram("let x: int = 7 % v\n  print(to_string(x))", 0), true, nil},
	{"mod_one", constProgram("let x: int = 7 % 1\n  print(to_string(x))"), ncIntProgram("let x: int = 7 % v\n  print(to_string(x))", 1), false, nil},
	{"mod_neg1", constProgram("let x: int = 7 % -1\n  print(to_string(x))"), ncIntProgram("let x: int = 7 % v\n  print(to_string(x))", -1), false, nil},
	// #3 repeat negative count (string.repeat).
	{"repeat_neg", constProgram(`let s: string = string.repeat("x", -1)`), ncIntProgram(`let s: string = string.repeat("x", v)`, -1), true, []string{"string"}},
	{"repeat_zero", constProgram(`let s: string = string.repeat("x", 0)`), ncIntProgram(`let s: string = string.repeat("x", v)`, 0), false, []string{"string"}},
	// #4 random non-positive (boundary 0: reject 0 and -1, accept 1) (math.random).
	{"random_zero", constProgram("let n: int = math.random(0)\n  print(to_string(n))"), ncIntProgram("let n: int = math.random(v)\n  print(to_string(n))", 0), true, []string{"math"}},
	{"random_neg", constProgram("let n: int = math.random(-1)\n  print(to_string(n))"), ncIntProgram("let n: int = math.random(v)\n  print(to_string(n))", -1), true, []string{"math"}},
	{"random_one", constProgram("let n: int = math.random(1)\n  print(to_string(n))"), ncIntProgram("let n: int = math.random(v)\n  print(to_string(n))", 1), false, []string{"math"}},
	// #5 format_float negative decimals (string.format_float).
	{"ff_neg", constProgram("let s: string = string.format_float(3.14, -1)"), ncIntProgram("let s: string = string.format_float(3.14, v)", -1), true, []string{"string"}},
	{"ff_zero", constProgram("let s: string = string.format_float(3.14, 0)"), ncIntProgram("let s: string = string.format_float(3.14, v)", 0), false, []string{"string"}},
	// #6 chr two-sided (string.chr).
	{"chr_0", constProgram("let s: string = string.chr(0)"), ncIntProgram("let s: string = string.chr(v)", 0), true, []string{"string"}},
	{"chr_1", constProgram("let s: string = string.chr(1)"), ncIntProgram("let s: string = string.chr(v)", 1), false, []string{"string"}},
	{"chr_255", constProgram("let s: string = string.chr(255)"), ncIntProgram("let s: string = string.chr(v)", 255), false, []string{"string"}},
	{"chr_256", constProgram("let s: string = string.chr(256)"), ncIntProgram("let s: string = string.chr(v)", 256), true, []string{"string"}},
	// #7 sleep negative -- value 0 is accepted; -1 rejected. (sleep 0 returns immediately.)
	{"sleep_neg", constProgram("sleep(-1)"), ncIntProgram("sleep(v)", -1), true, nil},
	{"sleep_zero", constProgram("sleep(0)"), ncIntProgram("sleep(v)", 0), false, nil},
	// #9 remove_at negative -- oversized array so non-negative boundary is in-bounds (array.remove_at).
	{"removeat_neg", constProgram("let a: int[] = [1, 2, 3]\n  array.remove_at(a, -1)"), ncIntProgram("let a: int[] = [1, 2, 3]\n  array.remove_at(a, v)", -1), true, []string{"array"}},
	{"removeat_zero", constProgram("let a: int[] = [1, 2, 3]\n  array.remove_at(a, 0)"), ncIntProgram("let a: int[] = [1, 2, 3]\n  array.remove_at(a, v)", 0), false, []string{"array"}},
	// #10 insert_at negative (array.insert_at).
	{"insertat_neg", constProgram("let a: int[] = [1, 2, 3]\n  array.insert_at(a, -1, 9)"), ncIntProgram("let a: int[] = [1, 2, 3]\n  array.insert_at(a, v, 9)", -1), true, []string{"array"}},
	{"insertat_zero", constProgram("let a: int[] = [1, 2, 3]\n  array.insert_at(a, 0, 9)"), ncIntProgram("let a: int[] = [1, 2, 3]\n  array.insert_at(a, v, 9)", 0), false, []string{"array"}},
	// #11 array index negative -- oversized array (length 3, index 0 in-bounds).
	{"index_neg", constProgram("let a: int[] = [1, 2, 3]\n  let x: int = a[-1]\n  print(to_string(x))"), ncIntProgram("let a: int[] = [1, 2, 3]\n  let x: int = a[v]\n  print(to_string(x))", -1), true, nil},
	{"index_zero", constProgram("let a: int[] = [1, 2, 3]\n  let x: int = a[0]\n  print(to_string(x))"), ncIntProgram("let a: int[] = [1, 2, 3]\n  let x: int = a[v]\n  print(to_string(x))", 0), false, nil},
}

// compileRejects parses + type-checks src and reports whether the checker produced
// any error. (For these vectors the only possible error is the domain diagnostic;
// the accepted-boundary programs are otherwise valid.) Never fatals on a checker error.
func compileRejects(src string) bool {
	prog, err := parser.Parse(src, "agree.wisp")
	if err != nil {
		return true
	}
	return len(types.Check(prog).Errors) != 0
}

// linkNS builds a Linked module set: root at id 0 with each named core namespace
// bound to a synthetic core module. It mirrors compileNS's construction but is
// reusable by the checker-only path (which must inspect errors, not fatal).
// Returns (nil, false) if the root fails to parse.
func linkNS(src string, namespaces []string) (*module.Linked, bool) {
	root, err := parser.Parse(src, "agree.wisp")
	if err != nil {
		return nil, false
	}
	root0 := &module.Module{ID: 0, Prog: root, Namespaces: map[string]int{}}
	mods := []*module.Module{root0}
	for i, ns := range namespaces {
		id := i + 1
		root0.Namespaces[ns] = id
		mods = append(mods, &module.Module{ID: id, Prog: &ast.Program{}, Namespaces: map[string]int{}, Core: ns})
	}
	return &module.Linked{Modules: mods}, true
}

// compileRejectsNS is compileRejects for the namespaced spelling: it type-checks
// src in a linked module set and reports whether any error (the domain diagnostic)
// was produced. A parse failure counts as a rejection.
func compileRejectsNS(src string, namespaces []string) bool {
	linked, ok := linkNS(src, namespaces)
	if !ok {
		return true
	}
	return len(types.CheckLinked(linked).Errors) != 0
}

// runtimeAborts compiles the non-const program and runs it on sh; it returns true
// iff the script exits non-zero (the guard aborted). wait_any (#8) is NOT a vector
// here: its first arg is a live Process[] and its runtime guard checks the empty-
// list precondition before the poll<0 guard, so a scalar poll injection cannot
// isolate the in-scope cause. wait_any is carved out (spec Section 3.1): its
// compile side is the Task 3 checker reject/accept test, and its runtime side is
// the NAMED prelude guard test TestWaitAnyPollRuntimeGuard added in Step 3b below.
//
// execShells(t) returns []struct{label,bin string; args []string} (an anonymous
// struct, verified at exec_command_runtime_test.go:14); the parameter type below
// must match it exactly.
func runtimeAborts(t *testing.T, sh struct {
	label string
	bin   string
	args  []string
}, src string) bool {
	t.Helper()
	return runScriptAborts(t, sh, compile(t, src))
}

// runtimeAbortsNS is runtimeAborts for the namespaced spelling: the non-const
// program type-checks (the value is non-constant, so the domain check is deferred
// to the emitted runtime guard), so compileNS is safe here.
func runtimeAbortsNS(t *testing.T, sh struct {
	label string
	bin   string
	args  []string
}, src string, namespaces []string) bool {
	t.Helper()
	return runScriptAborts(t, sh, compileNS(t, src, namespaces...))
}

// runScriptAborts writes script, runs it on sh, and returns true iff it exits nonzero.
func runScriptAborts(t *testing.T, sh struct {
	label string
	bin   string
	args  []string
}, script []byte) bool {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "p.sh")
	if err := os.WriteFile(path, script, 0o755); err != nil {
		t.Fatal(err)
	}
	args := append(append([]string{}, sh.args...), path)
	cmd := exec.Command(sh.bin, args...)
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	return cmd.Run() != nil
}

func TestArgDomainAgree(t *testing.T) {
	for _, vec := range argDomainVectors {
		vec := vec
		t.Run(vec.name, func(t *testing.T) {
			var compRejected bool
			if len(vec.namespaces) > 0 {
				compRejected = compileRejectsNS(vec.constSrc, vec.namespaces)
			} else {
				compRejected = compileRejects(vec.constSrc)
			}
			if compRejected != vec.wantReject {
				t.Errorf("compile decision for %s = %v, want %v (constSrc rejected mismatch)", vec.name, compRejected, vec.wantReject)
			}
			for _, sh := range execShells(t) {
				sh := sh
				t.Run(sh.label, func(t *testing.T) {
					var rtAborted bool
					if len(vec.namespaces) > 0 {
						rtAborted = runtimeAbortsNS(t, sh, vec.nonConstSrc, vec.namespaces)
					} else {
						rtAborted = runtimeAborts(t, sh, vec.nonConstSrc)
					}
					if compRejected != rtAborted {
						t.Errorf("DIVERGENCE %s on %s: compile-rejects=%v runtime-aborts=%v", vec.name, sh.label, compRejected, rtAborted)
					}
				})
			}
		})
	}
}
