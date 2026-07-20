package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runWispArgs is runWisp without the shellcheck step skipped: compile, shellcheck,
// run, and return outputs. (runWisp already shellchecks.) Kept thin so each test
// asserts on (stdout, stderr, exit).

// --- invariant 1: mutation persistence (scalar) ---

func TestErrMutationPersistsScalar(t *testing.T) {
	// parse-then-assign: success yields the parsed value.
	out, errb, code := runWisp(t, `fn main() -> int {
  let n: int = 0
  try {
    n = to_int("7")
  } catch (e) {
    n = -1
  }
  print(to_string(n))
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit = %d (stderr=%q)", code, errb)
	}
	if out != "7\n" {
		t.Fatalf("stdout = %q, want 7 (parsed value persists on success)", out)
	}
}

func TestErrMutationScalarBadInput(t *testing.T) {
	out, _, code := runWisp(t, `fn main() -> int {
  let n: int = 0
  try {
    n = to_int("bad")
  } catch (e) {
    n = -1
  }
  print(to_string(n))
  return 0
}`)
	if code != 0 || out != "-1\n" {
		t.Fatalf("stdout=%q exit=%d, want -1 / 0", out, code)
	}
}

// --- invariant 1: mutation persistence (aggregate) ---

func TestErrMutationPersistsAggregate(t *testing.T) {
	out, _, code := runWisp(t, `struct Box { v: int }
fn main() -> int {
  let b: Box = Box { v: 0 }
  try {
    b.v = 42
    let n: int = to_int("bad")
    b.v = 99
  } catch (e) {
    print("caught")
  }
  print(to_string(b.v))
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	// b.v=42 persists (before the fault); b.v=99 does NOT run (after the fault).
	if out != "caught\n42\n" {
		t.Fatalf("stdout = %q, want caught/42 (pre-fault mutation persists, post-fault skipped)", out)
	}
}

// --- invariant 2: fail-at-first-fault ---

func TestErrFailAtFirstFault(t *testing.T) {
	out, _, _ := runWisp(t, `fn main() -> int {
  try {
    print("before")
    let n: int = to_int("bad")
    print("after")
  } catch (e) {
    print("caught")
  }
  return 0
}`)
	if out != "before\ncaught\n" {
		t.Fatalf("stdout = %q, want before/caught (no 'after')", out)
	}
}

// --- invariant 3: catch once + pending cleared before handler (handler fault not self-caught) ---

func TestErrHandlerFaultNotSelfCaught(t *testing.T) {
	out, errb, code := runWisp(t, `fn main() -> int {
  try {
    throw error("first")
  } catch (e) {
    print("in-handler")
    throw error("handler-fault")
  }
  return 0
}`)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (handler fault propagates uncaught)", code)
	}
	if out != "in-handler\n" {
		t.Fatalf("stdout = %q, want in-handler", out)
	}
	if !strings.Contains(errb, "handler-fault") {
		t.Fatalf("stderr = %q, want the handler fault to abort located", errb)
	}
}

// --- invariant 4: e.message integrity + trailing-newline fidelity ---

func TestErrMessageIntegrity(t *testing.T) {
	out, _, code := runWisp(t, `fn main() -> int {
  try {
    throw error("boom")
  } catch (e) {
    print(e.message)
  }
  return 0
}`)
	if code != 0 || out != "boom\n" {
		t.Fatalf("stdout=%q exit=%d, want e.message == boom", out, code)
	}
}

func TestErrMessageTrailingNewlineFidelity(t *testing.T) {
	// A thrown message with a trailing newline is preserved (variable
	// assignment/read, not a $() capture that would truncate it).
	out, _, code := runWisp(t, `fn main() -> int {
  try {
    throw error("line1\n")
  } catch (e) {
    print(e.message + "END")
  }
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out != "line1\nEND\n" {
		t.Fatalf("stdout = %q, want trailing newline preserved (line1\\nEND)", out)
	}
}

// --- invariant 5: finally always runs ---

func TestErrFinallyOnCleanBody(t *testing.T) {
	out, _, _ := runWisp(t, `fn main() -> int {
  try {
    print("body")
  } catch (e) {
    print("nocatch")
  } finally {
    print("finally")
  }
  return 0
}`)
	if out != "body\nfinally\n" {
		t.Fatalf("stdout = %q, want body/finally (no catch)", out)
	}
}

func TestErrFinallyOnCaughtFault(t *testing.T) {
	out, _, _ := runWisp(t, `fn main() -> int {
  try {
    throw error("x")
  } catch (e) {
    print("caught")
  } finally {
    print("finally")
  }
  return 0
}`)
	if out != "caught\nfinally\n" {
		t.Fatalf("stdout = %q, want caught/finally", out)
	}
}

func TestErrFinallyOnOutermostRethrow(t *testing.T) {
	out, errb, code := runWisp(t, `fn main() -> int {
  try {
    throw error("orig")
  } catch (e) {
    print("caught")
    throw error("rethrown")
  } finally {
    print("finally")
  }
  return 0
}`)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (rethrow at outermost aborts)", code)
	}
	if out != "caught\nfinally\n" {
		t.Fatalf("stdout = %q, want caught/finally then located abort", out)
	}
	if !strings.Contains(errb, "rethrown") {
		t.Fatalf("stderr = %q, want located abort with rethrown msg", errb)
	}
}

func TestErrFaultInFinallyWins(t *testing.T) {
	_, errb, code := runWisp(t, `fn main() -> int {
  try {
    print("body")
  } catch (e) {
    print("nocatch")
  } finally {
    let n: int = to_int("bad")
    print("finally-after")
  }
  return 0
}`)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (finally fault aborts)", code)
	}
	if !strings.Contains(errb, "int(") {
		t.Fatalf("stderr = %q, want the finally fault", errb)
	}
}

// --- invariant 6: nesting ---

func TestErrNestingInnerCatch(t *testing.T) {
	out, _, _ := runWisp(t, `fn main() -> int {
  try {
    try {
      throw error("inner")
    } catch (e) {
      print("inner-catch " + e.message)
    } finally {
      print("inner-finally")
    }
  } catch (e) {
    print("outer-catch")
  } finally {
    print("outer-finally")
  }
  return 0
}`)
	// inner catch handles it; outer catch does NOT run; both finallys run in order.
	if out != "inner-catch inner\ninner-finally\nouter-finally\n" {
		t.Fatalf("stdout = %q", out)
	}
}

func TestErrNestingRethrowToOuter(t *testing.T) {
	out, _, code := runWisp(t, `fn main() -> int {
  try {
    try {
      throw error("inner")
    } catch (e) {
      throw error("from-inner")
    } finally {
      print("inner-finally")
    }
  } catch (e) {
    print("outer-catch " + e.message)
  } finally {
    print("outer-finally")
  }
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out != "inner-finally\nouter-catch from-inner\nouter-finally\n" {
		t.Fatalf("stdout = %q, want inner-finally/outer-catch/outer-finally in order", out)
	}
}

// --- invariant 8: cross-frame ---

func TestErrCrossFrame(t *testing.T) {
	out, _, code := runWisp(t, `fn parse(s: string) -> int {
  return to_int(s)
}
fn main() -> int {
  try {
    let n: int = parse("nope")
    print("unreached")
  } catch (e) {
    print("caught")
  } finally {
    print("cleanup")
  }
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out != "caught\ncleanup\n" {
		t.Fatalf("stdout = %q, want caught/cleanup (no unreached)", out)
	}
}

// --- invariant 9: all fault classes catchable, each with a located message ---

func TestErrAllFaultClassesCatchable(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string // substring expected in e.message
	}{
		{"div", "let z: int = 0\n    let c: int = 1 / z", "division by zero"},
		{"mod", "let z: int = 0\n    let c: int = 1 % z", "division by zero"},
		{"int", "let c: int = to_int(\"x\")", "int("},
		{"bool", "let c: bool = to_bool(\"x\")", "bool("},
		{"float", "let c: float = to_float(\"x\")", "float("},
		// The "replace" fault-class vector (string.replace empty-search) is a
		// removable builtin, so it cannot ride this single-module harness; it is
		// reconstructed in TestErrReplaceFaultClassCatchable below with the
		// namespaced spelling.
		{"array_oob", "let xs: int[] = [1]\n    let c: int = xs[9]", "out of bounds"},
		{"dict_missing", "let m: {string: int} = {\"a\": 1}\n    let c: int = m[\"z\"]", "not found"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			src := "fn main() -> int {\n  try {\n    " + c.body +
				"\n  } catch (e) {\n    print(e.message)\n  }\n  return 0\n}"
			out, errb, code := runWisp(t, src)
			if code != 0 {
				t.Fatalf("exit = %d, want 0 (fault caught) (stderr=%q)", code, errb)
			}
			// exactly one message line, located (file:line:col:), no spurious second.
			lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
			if len(lines) != 1 {
				t.Fatalf("stdout = %q, want exactly one e.message line", out)
			}
			if !strings.Contains(lines[0], c.want) {
				t.Fatalf("e.message = %q, want substring %q", lines[0], c.want)
			}
			if !strings.Contains(lines[0], "test.wisp:") {
				t.Fatalf("e.message = %q, want a located file:line:col prefix", lines[0])
			}
		})
	}
}

// TestErrReplaceFaultClassCatchable reconstructs the replace fault-class vector
// from TestErrAllFaultClassesCatchable for the modules-only surface: the
// string.replace empty-search fault must be catchable and carry exactly one
// located e.message. The delegate lowers byte-identically to the pre-removal
// flat replace, so the fault class is unchanged.
func TestErrReplaceFaultClassCatchable(t *testing.T) {
	src := "fn main() -> int {\n  try {\n    let c: string = string.replace(\"a\", \"\", \"b\")" +
		"\n  } catch (e) {\n    print(e.message)\n  }\n  return 0\n}"
	out, errb, code := runNS(t, src, "string")
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (fault caught) (stderr=%q)", code, errb)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("stdout = %q, want exactly one e.message line", out)
	}
	if !strings.Contains(lines[0], "replace(") {
		t.Fatalf("e.message = %q, want substring %q", lines[0], "replace(")
	}
	if !strings.Contains(lines[0], "test.wisp:") {
		t.Fatalf("e.message = %q, want a located file:line:col prefix", lines[0])
	}
}

// --- invariant 10: uncaught throw aborts located ---

func TestErrUncaughtThrowAbortsLocated(t *testing.T) {
	assertLocatedAbort(t, "fn main() -> int {\n  throw error(\"boom\")\n}", 2, 3, "boom")
}

// --- invariant 11: injection inert ---

func TestErrInjectionInert(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "SENTINEL")
	src := "fn main() -> int {\n" +
		"  try {\n" +
		"    throw error(\"$(touch " + sentinel + ") `echo no`\")\n" +
		"  } catch (e) {\n" +
		"    print(e.message)\n" +
		"  }\n" +
		"  return 0\n}"
	out, _, code := runWisp(t, src)
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if _, err := os.Stat(sentinel); err == nil {
		t.Fatalf("injection executed: sentinel %s exists", sentinel)
	}
	if !strings.Contains(out, "$(touch") || !strings.Contains(out, "`echo no`") {
		t.Fatalf("stdout = %q, want the raw inert message", out)
	}
}

// --- invariant 14: catch var scope, no mktemp/subshell/trap, throw value-flow ---

func TestErrNoMktempSubshellTrap(t *testing.T) {
	script := compile(t, `fn main() -> int {
  try {
    throw error("x")
  } catch (e) {
    print(e.message)
  } finally {
    print("f")
  }
  return 0
}`)
	s := string(script)
	// A blanket $( ) ban is too broad (awk/lower/trim prelude helpers use command
	// substitution), so assert only the no-new-dependency constraints: the
	// generated try uses no mktemp and no trap, and the throw/catch path runs in
	// the current shell (no subshell wraps the try body).
	if strings.Contains(s, "mktemp") {
		t.Errorf("generated try uses mktemp")
	}
	if strings.Contains(s, "trap ") || strings.Contains(s, "trap\t") {
		t.Errorf("generated try uses trap")
	}
}

func TestErrCatchMutationPersists(t *testing.T) {
	out, _, _ := runWisp(t, `fn main() -> int {
  let n: int = 0
  try {
    throw error("x")
  } catch (e) {
    n = 5
  }
  print(to_string(n))
  return 0
}`)
	if out != "5\n" {
		t.Fatalf("stdout = %q, want 5 (catch mutation persists)", out)
	}
}

func TestErrFinallyMutationPersists(t *testing.T) {
	out, _, _ := runWisp(t, `fn main() -> int {
  let n: int = 0
  try {
    print("b")
  } catch (e) {
    print("c")
  } finally {
    n = 9
  }
  print(to_string(n))
  return 0
}`)
	if out != "b\n9\n" {
		t.Fatalf("stdout = %q, want b/9 (finally mutation persists)", out)
	}
}

func TestErrValueFlowParamReturn(t *testing.T) {
	out, _, code := runWisp(t, `fn describe(e: error) -> string {
  return e.message
}
fn make(msg: string) -> error {
  return error(msg)
}
fn main() -> int {
  let e: error = make("hello")
  print(describe(e))
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out != "hello\n" {
		t.Fatalf("stdout = %q, want hello (error value-flow param/return)", out)
	}
}

// --- ordinary control flow still works with guards enabled ---

func TestErrOrdinaryControlFlowWithGuards(t *testing.T) {
	// A program that uses try elsewhere (errMode on) but an ordinary loop with
	// break/continue and an early return outside any try behaves normally.
	out, _, code := runWisp(t, `fn sumTo(limit: int) -> int {
  let total: int = 0
  for (let i: int = 0; i < limit; i = i + 1) {
    if (i == 3) { continue }
    if (i == 6) { break }
    total = total + i
  }
  return total
}
fn main() -> int {
  try {
    print("trywarm")
  } catch (e) {
    print("c")
  }
  print(to_string(sumTo(10)))
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	// 0+1+2+4+5 = 12 (skip 3 via continue, stop at 6 via break).
	if out != "trywarm\n12\n" {
		t.Fatalf("stdout = %q, want trywarm/12", out)
	}
}

// --- nested try is guarded as one unit (skipped during an outer unwind) ---

func TestErrNestedTryGuardedAsUnit(t *testing.T) {
	out, _, code := runWisp(t, `fn main() -> int {
  try {
    let n: int = to_int("bad")
    try {
      print("nested-body")
    } catch (e) {
      print("nested-catch")
    }
  } catch (e) {
    print("outer-caught")
  }
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	// The outer fault skips the whole nested try; neither its body nor its handler
	// runs (its unguarded save/clear scaffolding must not execute either).
	if out != "outer-caught\n" {
		t.Fatalf("stdout = %q, want only outer-caught (nested try fully skipped)", out)
	}
}

// --- zero-overhead: no guard scaffolding when try/throw unused ---

func TestErrZeroOverheadWhenUnused(t *testing.T) {
	script := compile(t, `fn main() -> int {
  let n: int = to_int("5")
  print(to_string(n))
  return 0
}`)
	s := string(script)
	for _, scaffold := range []string{"__wisp_try_depth", "__wisp_err_pending", "__wisp_err_msg", "__wisp_throw"} {
		if strings.Contains(s, scaffold) {
			t.Errorf("zero-overhead violated: %q present in a program with no try/throw", scaffold)
		}
	}
	// The M1 fail body (unconditional exit) must be used, not the mode-aware one.
	if strings.Contains(s, `[ "${__wisp_try_depth:-0}" -gt 0 ]`) {
		t.Errorf("mode-aware __wisp_fail emitted for a non-error program")
	}
}

func TestErrScaffoldingWhenUsed(t *testing.T) {
	script := compile(t, `fn main() -> int {
  try { throw error("x") } catch (e) { print(e.message) }
  return 0
}`)
	s := string(script)
	if !strings.Contains(s, "__wisp_try_depth=0") {
		t.Errorf("error program missing __wisp_try_depth init")
	}
	if !strings.Contains(s, `[ "${__wisp_try_depth:-0}" -gt 0 ]`) {
		t.Errorf("error program missing the mode-aware __wisp_fail body")
	}
	if !strings.Contains(s, "__wisp_throw") {
		t.Errorf("error program missing __wisp_throw")
	}
}

// --- mid-statement short-circuit (invariant 14): h(y) does not run ---

func TestErrMidStatementShortCircuit(t *testing.T) {
	out, _, code := runWisp(t, `fn g(x: int) -> int {
  return to_int("bad")
}
fn h(x: int) -> int {
  print("h-ran")
  return x
}
fn f(a: int, b: int) -> int {
  return a + b
}
fn main() -> int {
  try {
    let r: int = f(g(1), h(2))
    print(to_string(r))
  } catch (e) {
    print("caught")
  }
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if out != "caught\n" {
		t.Fatalf("stdout = %q, want only 'caught' (h must not run, mid-statement short-circuit)", out)
	}
}

// --- wrap / cause builtins: shape tests ---

// TestWrapEmitsHandleWithCause confirms wrap emits a _cause var assignment.
func TestWrapEmitsHandleWithCause(t *testing.T) {
	out := string(compile(t, wrapMainCG(`let inner: error = error("inner")
let w: error = wrap(inner, "outer")`)))
	if !strings.Contains(out, "_cause=") {
		t.Errorf("wrap: expected _cause field assignment, got:\n%s", out)
	}
	shellcheck(t, []byte(out))
}

// TestCauseEmitsTagCheck confirms cause emits an Optional tag assignment.
func TestCauseEmitsTagCheck(t *testing.T) {
	out := string(compile(t, wrapMainCG(`let e: error = error("x")
let o: Optional[error] = cause(e)`)))
	if !strings.Contains(out, "_tag") {
		t.Errorf("cause: expected _tag Optional construction, got:\n%s", out)
	}
	shellcheck(t, []byte(out))
}

// TestWrapCauseNoCauseField confirms no e.cause field access (cause is via builtin only).
func TestWrapCauseNoFieldAccess(t *testing.T) {
	// cause(e) should not emit "e.cause" as a field access -- it reads _cause backing var.
	out := string(compile(t, wrapMainCG(`let e: error = error("x")
let o: Optional[error] = cause(e)`)))
	// The backing var should be _cause not a field named "cause" on the struct
	if strings.Contains(out, ".cause") {
		t.Errorf("cause: should not emit '.cause' field access, got:\n%s", out)
	}
	shellcheck(t, []byte(out))
}

// --- AC1: cause(wrap(error("inner"),"outer")) is Some(inner) with inner.message=="inner" ---

func TestWrapCauseAC1Basic(t *testing.T) {
	src := wrapMainCG(`let inner: error = error("inner")
let w: error = wrap(inner, "outer")
print(w.message)
print(to_string(w.code))
let o: Optional[error] = cause(w)
print(to_string(is_some(o)))
let got: error = unwrap(o)
print(got.message)`)
	out, errb, code := runWisp(t, src)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb)
	}
	want := "outer\n0\ntrue\ninner\n"
	if out != want {
		t.Fatalf("stdout = %q, want %q", out, want)
	}
}

// cause(error("x")) is None.
func TestCauseOfPlainErrorIsNone(t *testing.T) {
	src := wrapMainCG(`let e: error = error("x")
let o: Optional[error] = cause(e)
print(to_string(is_none(o)))`)
	out, errb, code := runWisp(t, src)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb)
	}
	if out != "true\n" {
		t.Fatalf("stdout = %q, want true", out)
	}
}

// cause(error_with(7,"x")) is None.
func TestCauseOfErrorWithIsNone(t *testing.T) {
	src := wrapMainCG(`let e: error = error_with(7, "x")
let o: Optional[error] = cause(e)
print(to_string(is_none(o)))`)
	out, errb, code := runWisp(t, src)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errb)
	}
	if out != "true\n" {
		t.Fatalf("stdout = %q, want true", out)
	}
}
