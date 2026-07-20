package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runErrSnippet emits the helpers in ERROR MODE (mode-aware __wisp_fail +
// __wisp_throw) plus an init of the runtime state vars, then the driver, and
// runs it under dash.
func runErrSnippet(t *testing.T, helpers []string, driver string) (string, string, int) {
	t.Helper()
	dash := dashPath(t)
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString(EmitMode(helpers, true, false))
	b.WriteString("\n")
	b.WriteString("__wisp_try_depth=0\n__wisp_err_pending=\n__wisp_err_msg=\n")
	b.WriteString(driver)
	b.WriteString("\n")

	dir := t.TempDir()
	script := filepath.Join(dir, "snippet.sh")
	if err := os.WriteFile(script, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(dash, script)
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("run: %v (stderr=%q)", err, errb.String())
		}
	}
	return out.String(), errb.String(), code
}

// At depth 0, the mode-aware __wisp_fail still aborts located + exit 1 (M1).
func TestFailModeDepth0Aborts(t *testing.T) {
	_, errb, code := runErrSnippet(t, []string{Fail}, `__wisp_fail "prog.wisp:1:1" "boom"`)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if errb != "wisp: prog.wisp:1:1: boom\n" {
		t.Fatalf("stderr = %q", errb)
	}
}

// At depth > 0, __wisp_fail sets pending + position-prefixed msg and RETURNS
// (no exit). e.message for a fault is the located text.
func TestFailModeDepthSetsPendingAndReturns(t *testing.T) {
	out, _, code := runErrSnippet(t, []string{Fail},
		`__wisp_try_depth=1; __wisp_fail "prog.wisp:2:5" "division by zero"; printf 'after\n'; printf 'pending=%s\n' "$__wisp_err_pending"; printf 'msg=%s\n' "$__wisp_err_msg"`)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (no exit at depth>0)", code)
	}
	want := "after\npending=1\nmsg=prog.wisp:2:5: division by zero\n"
	if out != want {
		t.Fatalf("stdout = %q, want %q", out, want)
	}
}

// First-fault-wins: a second fault at depth>0 does NOT overwrite the message.
func TestFailModeFirstFaultWins(t *testing.T) {
	out, _, _ := runErrSnippet(t, []string{Fail},
		`__wisp_try_depth=1; __wisp_fail "p:1:1" "first"; __wisp_fail "p:2:2" "second"; printf 'msg=%s\n' "$__wisp_err_msg"`)
	if out != "msg=p:1:1: first\n" {
		t.Fatalf("stdout = %q, want first fault preserved", out)
	}
}

// __wisp_throw at depth>0 stores the RAW message (not position-prefixed).
func TestThrowModeDepthRawMessage(t *testing.T) {
	out, _, code := runErrSnippet(t, []string{Throw},
		`__wisp_try_depth=1; __wisp_throw "p:3:3" "boom"; printf 'pending=%s\n' "$__wisp_err_pending"; printf 'msg=%s\n' "$__wisp_err_msg"`)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if out != "pending=1\nmsg=boom\n" {
		t.Fatalf("stdout = %q, want raw message boom", out)
	}
}

// __wisp_throw at depth 0 aborts located with the raw message.
func TestThrowModeDepth0Aborts(t *testing.T) {
	_, errb, code := runErrSnippet(t, []string{Throw}, `__wisp_throw "p:3:3" "boom"`)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if errb != "wisp: p:3:3: boom\n" {
		t.Fatalf("stderr = %q", errb)
	}
}

// fdiv short-circuits: at depth>0 a zero divisor sets pending and RETURNS
// without performing the division (no second fault, no hard abort).
func TestFdivShortCircuits(t *testing.T) {
	out, _, code := runErrSnippet(t, []string{FDiv},
		`__wisp_try_depth=1; __wisp_fdiv "p:1:1" "5" "0"; printf 'pending=%s\n' "$__wisp_err_pending"; printf 'msg=%s\n' "$__wisp_err_msg"`)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if out != "pending=1\nmsg=p:1:1: division by zero\n" {
		t.Fatalf("stdout = %q", out)
	}
}

// __wisp_int short-circuits after a fault at depth>0.
func TestIntShortCircuits(t *testing.T) {
	out, _, code := runErrSnippet(t, []string{Int},
		`__wisp_try_depth=1; __wisp_int "p:1:1" "abc"; printf 'pending=%s\n' "$__wisp_err_pending"; printf 'msg=%s\n' "$__wisp_err_msg"`)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if out != "pending=1\nmsg=p:1:1: to_int(): not an integer: \"abc\"\n" {
		t.Fatalf("stdout = %q", out)
	}
}

// __wisp_int short-circuits on an out-of-range value via the length-comparison
// branch (prelude.go's first "out of range" branch — a value with more digits
// than int64 max can ever have).
func TestIntOutOfRangeLengthBranch(t *testing.T) {
	out, _, code := runErrSnippet(t, []string{Int},
		`__wisp_try_depth=1; __wisp_int "p:1:1" "99999999999999999999"; printf 'pending=%s\n' "$__wisp_err_pending"; printf 'msg=%s\n' "$__wisp_err_msg"`)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if out != "pending=1\nmsg=p:1:1: to_int(): out of range: \"99999999999999999999\"\n" {
		t.Fatalf("stdout = %q", out)
	}
}

// __wisp_int short-circuits on an out-of-range value via the digit-by-digit
// lexical-compare branch (same digit count as int64 max, one greater by value).
func TestIntOutOfRangeLexicalBranch(t *testing.T) {
	out, _, code := runErrSnippet(t, []string{Int},
		`__wisp_try_depth=1; __wisp_int "p:1:1" "9223372036854775808"; printf 'pending=%s\n' "$__wisp_err_pending"; printf 'msg=%s\n' "$__wisp_err_msg"`)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if out != "pending=1\nmsg=p:1:1: to_int(): out of range: \"9223372036854775808\"\n" {
		t.Fatalf("stdout = %q", out)
	}
}

// __wisp_bool_str short-circuits on a non-bool value, naming to_bool.
func TestBoolStrShortCircuits(t *testing.T) {
	out, _, code := runErrSnippet(t, []string{BoolStr},
		`__wisp_try_depth=1; __wisp_bool_str "p:1:1" "maybe"; printf 'pending=%s\n' "$__wisp_err_pending"; printf 'msg=%s\n' "$__wisp_err_msg"`)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if out != "pending=1\nmsg=p:1:1: to_bool(): not a bool: \"maybe\"\n" {
		t.Fatalf("stdout = %q", out)
	}
}

// __wisp_ffloat_s short-circuits on a non-float value, naming to_float.
func TestFFloatSShortCircuits(t *testing.T) {
	out, _, code := runErrSnippet(t, []string{FFloatS},
		`__wisp_try_depth=1; __wisp_ffloat_s "p:1:1" "abc"; printf 'pending=%s\n' "$__wisp_err_pending"; printf 'msg=%s\n' "$__wisp_err_msg"`)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if out != "pending=1\nmsg=p:1:1: to_float(): not a float: \"abc\"\n" {
		t.Fatalf("stdout = %q", out)
	}
}

// Non-error-mode build keeps the M1 __wisp_fail (no __wisp_try_depth in body).
func TestEmitModeFalseKeepsM1Fail(t *testing.T) {
	src := EmitMode([]string{Fail}, false, false)
	if strings.Contains(src, "__wisp_try_depth") {
		t.Fatalf("non-errMode fail should not reference __wisp_try_depth:\n%s", src)
	}
	if strings.Contains(src, "__wisp_throw") {
		t.Fatalf("non-errMode build should not emit __wisp_throw:\n%s", src)
	}
}

// errMode build emits the mode-aware fail referencing the depth.
func TestEmitModeTrueModeAwareFail(t *testing.T) {
	src := EmitMode([]string{Fail}, true, false)
	if !strings.Contains(src, "__wisp_try_depth") {
		t.Fatalf("errMode fail must reference __wisp_try_depth:\n%s", src)
	}
}
