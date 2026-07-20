package codegen

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/parser"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// locatedAbortRe matches a fully-located abort line: wisp: file:line:col: msg.
var locatedAbortRe = regexp.MustCompile(`^wisp: ([^:]+):(\d+):(\d+): (.*)$`)

// assertLocatedAbort runs src, requires exit 1, and asserts stderr's first line
// is `wisp: <file>:<line>:<col>: <msg>` where msg contains wantMsgSub and the
// position equals wantLine:wantCol with file test.wisp.
func assertLocatedAbort(t *testing.T, src string, wantLine, wantCol int, wantMsgSub string) {
	t.Helper()
	_, errb, code := runWisp(t, src)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr=%q)", code, errb)
	}
	first := strings.SplitN(strings.TrimRight(errb, "\n"), "\n", 2)[0]
	m := locatedAbortRe.FindStringSubmatch(first)
	if m == nil {
		t.Fatalf("stderr first line %q does not match `wisp: file:line:col: msg`", first)
	}
	file, line, col, msg := m[1], m[2], m[3], m[4]
	if file != "test.wisp" {
		t.Errorf("file = %q, want test.wisp", file)
	}
	if line != itoa(wantLine) || col != itoa(wantCol) {
		t.Errorf("position = %s:%s, want %d:%d", line, col, wantLine, wantCol)
	}
	if !strings.Contains(msg, wantMsgSub) {
		t.Errorf("message %q lacks substring %q", msg, wantMsgSub)
	}
}

// assertLocatedAbortNS is the modules-only analogue of assertLocatedAbort for a
// program that calls namespaced members: it compiles multi-module with the given
// namespaces bound, then makes the same located-abort assertions.
func assertLocatedAbortNS(t *testing.T, src string, wantLine, wantCol int, wantMsgSub string, namespaces ...string) {
	t.Helper()
	_, errb, code := runNS(t, src, namespaces...)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr=%q)", code, errb)
	}
	first := strings.SplitN(strings.TrimRight(errb, "\n"), "\n", 2)[0]
	m := locatedAbortRe.FindStringSubmatch(first)
	if m == nil {
		t.Fatalf("stderr first line %q does not match `wisp: file:line:col: msg`", first)
	}
	file, line, col, msg := m[1], m[2], m[3], m[4]
	if file != "test.wisp" {
		t.Errorf("file = %q, want test.wisp", file)
	}
	if line != itoa(wantLine) || col != itoa(wantCol) {
		t.Errorf("position = %s:%s, want %d:%d", line, col, wantLine, wantCol)
	}
	if !strings.Contains(msg, wantMsgSub) {
		t.Errorf("message %q lacks substring %q", msg, wantMsgSub)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// Div-by-zero: the `/` operator is at line 4. The located message preserves M1's
// "division by zero" label.
func TestLocatedDivByZero(t *testing.T) {
	src := "fn main() -> int {\n" + // 1
		"  let a: int = 5\n" + // 2
		"  let b: int = 0\n" + // 3
		"  return a / b\n" + // 4: `/` at col 12
		"}" // 5
	assertLocatedAbort(t, src, 4, 12, "division by zero")
}

// Mod-by-zero shares the div guard and PRESERVES the "division by zero" label
// (no separate "modulo by zero" label).
func TestLocatedModByZero(t *testing.T) {
	src := "fn main() -> int {\n" +
		"  let a: int = 5\n" +
		"  let b: int = 0\n" +
		"  return a % b\n" + // `%` at col 12
		"}"
	assertLocatedAbort(t, src, 4, 12, "division by zero")
}

// to_int() on a non-integer aborts with the call site located. The call
// `to_int(...)` starts at col 16 of line 2.
func TestLocatedIntBadInput(t *testing.T) {
	src := "fn main() -> int {\n" +
		"  let x: int = to_int(\"abc\")\n" + // `to_int` at col 16
		"  return 0\n" +
		"}"
	assertLocatedAbort(t, src, 2, 16, "int(")
}

// to_bool() on a non-bool string aborts with the call site located.
func TestLocatedBoolBadString(t *testing.T) {
	src := "fn main() -> int {\n" +
		"  let b: bool = to_bool(\"yes\")\n" + // `to_bool` at col 17
		"  return 0\n" +
		"}"
	assertLocatedAbort(t, src, 2, 17, "bool(")
}

// string.replace() with an empty search aborts with the call site located.
// Reconstructed with the namespaced string.replace; the delegate reports the
// same located position (member path start) and message as the pre-removal flat
// replace.
func TestLocatedReplaceEmptySearch(t *testing.T) {
	src := "fn main() -> int {\n" +
		"  print(string.replace(\"abc\", \"\", \"x\"))\n" + // `string.replace` at col 9
		"  return 0\n" +
		"}"
	assertLocatedAbortNS(t, src, 2, 9, "replace(", "string")
}

// TestLocatedAbortShellActivePath proves the <pos> literal is safely quoted: a
// source path containing shell-active characters ($, backtick, etc.) appears in
// the located stderr verbatim and is never expanded or able to inject (spec
// section 4, plan M2-T2).
func TestLocatedAbortShellActivePath(t *testing.T) {
	filename := "a$b`whoami`.wisp"
	src := "fn main() -> int {\n" +
		"  let a: int = 5\n" +
		"  let b: int = 0\n" +
		"  return a / b\n" +
		"}"
	prog, err := parser.Parse(src, filename)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info := types.Check(prog)
	if len(info.Errors) > 0 {
		t.Fatalf("check: %v", info.Errors)
	}
	script, err := Generate(prog, info)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	shellcheck(t, script)
	_, errb, code := run(t, script)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr=%q)", code, errb)
	}
	want := "wisp: " + filename + ":4:12: division by zero\n"
	if errb != want {
		t.Fatalf("stderr = %q, want %q", errb, want)
	}
}
