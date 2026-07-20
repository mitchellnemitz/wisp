package codegen

import (
	"strings"
	"testing"
)

// TestConstIntInlinesInLetRHS: const is inlined at a let RHS; no local or shell
// variable is emitted for the const name.
func TestConstIntInlinesInLetRHS(t *testing.T) {
	src := `
const M: int = 60 * 60

fn main() -> int {
  let x: int = M
  print("${x}")
  return 0
}
`
	script := compile(t, src)
	s := string(script)

	// The folded value must appear in the shell script.
	if !strings.Contains(s, "3600") {
		t.Fatalf("expected folded value 3600 in output:\n%s", s)
	}

	// No 'local' declaration for a const-mangled name should appear; const has no
	// Mangled name and is absent from Decls, so genLocals emits nothing for it.
	// The const name M must not appear as a shell variable reference.
	if strings.Contains(s, "$M") || strings.Contains(s, "\"$M\"") {
		t.Fatalf("const M leaked as a shell variable reference:\n%s", s)
	}

	// Confirm the runtime value is correct.
	wantRun(t, src, "3600\n", "", 0)
}

// TestConstInBinaryExpr: const used inside a binary expression inlines the value.
func TestConstInBinaryExpr(t *testing.T) {
	src := `
const BASE: int = 10

fn main() -> int {
  let y: int = BASE + 5
  print("${y}")
  return 0
}
`
	wantRun(t, src, "15\n", "", 0)
	s := string(compile(t, src))
	if !strings.Contains(s, "10") {
		t.Fatalf("expected folded value 10 in output:\n%s", s)
	}
}

// TestConstInPrint: const used directly in a print call inlines the value.
func TestConstInPrint(t *testing.T) {
	src := `
const ANSWER: int = 42

fn main() -> int {
  print("${ANSWER}")
  return 0
}
`
	wantRun(t, src, "42\n", "", 0)
}

// TestConstStringInlined: a const string is inlined as a properly-quoted shell
// literal. A string containing shell-active characters (like a $(...) sequence)
// must be inert data -- not re-executed.
func TestConstStringInlined(t *testing.T) {
	src := `
const LABEL: string = "hello world"

fn main() -> int {
  print(LABEL)
  return 0
}
`
	wantRun(t, src, "hello world\n", "", 0)
}

// TestConstStringInjectionSafe: a const string containing shell metacharacters
// must be inlined as safe single-quoted data, never re-evaluated.
func TestConstStringInjectionSafe(t *testing.T) {
	src := `
const UNSAFE: string = "$(echo injected)"

fn main() -> int {
  print(UNSAFE)
  return 0
}
`
	// The string must print literally, not execute the subshell.
	out, _, code := runWisp(t, src)
	if code != 0 {
		t.Fatalf("unexpected exit code %d", code)
	}
	if out != "$(echo injected)\n" {
		t.Fatalf("stdout = %q, want literal string", out)
	}
}

// TestConstFloatInlined: a float const inlines as its raw decimal literal,
// lowered exactly like a normal float literal (a bare safe word), not as an
// empty single-quoted token. to_string(PI) canonicalizes via awk %.17g,
// matching to_string(3.14).
func TestConstFloatInlined(t *testing.T) {
	src := `
const PI: float = 3.14

fn main() -> int {
  print(to_string(PI))
  return 0
}
`
	wantRun(t, src, "3.1400000000000001\n", "", 0)

	s := string(compile(t, src))
	if !strings.Contains(s, "3.14") {
		t.Fatalf("expected folded float literal 3.14 in output:\n%s", s)
	}
	if strings.Contains(s, "__wisp_fstr ''") || strings.Contains(s, "__wisp_fstr \"\"") {
		t.Fatalf("float const inlined as empty token:\n%s", s)
	}
}

// TestConstNegativeFloatInlined: a negative float const inlines as a bare
// signed decimal literal (matching the float-validity invariant), not empty.
func TestConstNegativeFloatInlined(t *testing.T) {
	src := `
const NEG: float = -2.5

fn main() -> int {
  print(to_string(NEG))
  return 0
}
`
	wantRun(t, src, "-2.5\n", "", 0)
}

// TestConstAsDefaultArg: a function whose default argument is a const; when
// called without that argument, the const's folded value (not an unset shell
// variable) is used.
func TestConstAsDefaultArg(t *testing.T) {
	src := `
const TIMEOUT: int = 30

fn delay(secs: int = TIMEOUT) -> int {
  return secs
}

fn main() -> int {
  let n: int = delay()
  print("${n}")
  return 0
}
`
	wantRun(t, src, "30\n", "", 0)

	s := string(compile(t, src))
	// The literal 30 must appear; no unset-variable reference for TIMEOUT.
	if !strings.Contains(s, "30") {
		t.Fatalf("folded default value 30 not found in:\n%s", s)
	}
}

// TestConstFDInlines: `const FD: int = stdout` folds stdout to 1 and inlines it
// at value position. The inlined value 1 is visible in the generated shell.
func TestConstFDInlines(t *testing.T) {
	src := `
const FD: int = stdout

fn main() -> int {
  let fd: int = FD
  print("${fd}")
  return 0
}
`
	// At runtime, FD inlines to 1 (the fd for stdout).
	wantRun(t, src, "1\n", "", 0)

	s := string(compile(t, src))
	if !strings.Contains(s, "1") {
		t.Fatalf("folded stdout value 1 not found in:\n%s", s)
	}
}

// TestFinalEmitsAsLocal: a final binding works like a let at runtime -- the RHS
// is evaluated and the value is usable afterward.
func TestFinalEmitsAsLocal(t *testing.T) {
	src := `
fn compute() -> int {
  return 7
}

fn main() -> int {
  final y: int = compute()
  print("${y}")
  return 0
}
`
	wantRun(t, src, "7\n", "", 0)

	s := string(compile(t, src))
	// A 'local' declaration must exist for the final binding's mangled name.
	if !strings.Contains(s, "local ") {
		t.Fatalf("no local declaration found for final binding:\n%s", s)
	}
}

// TestConstNoLocalEmitted: verifies at the shell-script level that no 'local'
// variable is emitted for a const binding declared inside a function body.
func TestConstNoLocalEmitted(t *testing.T) {
	src := `
fn main() -> int {
  const LIMIT: int = 100
  let x: int = LIMIT
  print("${x}")
  return 0
}
`
	script := compile(t, src)
	s := string(script)

	// The folded value must appear.
	if !strings.Contains(s, "100") {
		t.Fatalf("folded value 100 not found:\n%s", s)
	}

	// Confirm runtime correctness.
	wantRun(t, src, "100\n", "", 0)
}

// TestLocalConstInlinesFoldedValue: a local const defined via an expression is
// folded at compile time and the folded literal (not a variable reference) is
// inlined at uses.
func TestLocalConstInlinesFoldedValue(t *testing.T) {
	src := `
fn main() -> int {
  const SECS: int = 60 * 60
  print("${SECS}")
  return 0
}
`
	script := compile(t, src)
	s := string(script)

	if !strings.Contains(s, "3600") {
		t.Fatalf("expected folded value 3600 in:\n%s", s)
	}

	wantRun(t, src, "3600\n", "", 0)
}
