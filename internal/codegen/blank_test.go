package codegen

import (
	"strings"
	"testing"
)

// A blank let must still call a side-effecting RHS, but emit no variable
// assignment for `_`.
func TestBlankLetEvaluatesRHSNoBinding(t *testing.T) {
	src := "fn beep() -> int { print(\"beep\"); return 1 }\n" +
		wrapMainCG("let _: int = beep()")
	out, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if out != "beep\n" {
		t.Errorf("stdout = %q, want %q (RHS must run)", out, "beep\n")
	}
}

func TestBlankAssignEvaluatesRHSNoBinding(t *testing.T) {
	src := "fn beep() -> int { print(\"beep\"); return 1 }\n" +
		wrapMainCG("_ = beep()")
	out, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if out != "beep\n" {
		t.Errorf("stdout = %q, want %q (RHS must run)", out, "beep\n")
	}
}

func TestBlankForInArrayIterates(t *testing.T) {
	src := wrapMainCG("let xs: int[] = [10, 20, 30]\nfor (_ in xs) { print(\"tick\") }")
	out, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if out != "tick\ntick\ntick\n" {
		t.Errorf("stdout = %q, want three ticks", out)
	}
}

func TestBlankForInDictIterates(t *testing.T) {
	src := wrapMainCG("let d: {string: int} = {\"a\": 1, \"b\": 2}\nfor (_ in d) { print(\"tick\") }")
	out, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if out != "tick\ntick\n" {
		t.Errorf("stdout = %q, want two ticks", out)
	}
}

// A blank param must not consume the positional slot of a following named param.
func TestBlankParamPositional(t *testing.T) {
	// f(_: int, x: int): x must read $2, not $1.
	src := "fn f(_: int, x: int) -> int { return x }\n" +
		"fn main() -> int {\n" +
		"print(to_string(f(11, 22)))\n" +
		"return 0\n}"
	out, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if out != "22\n" {
		t.Errorf("stdout = %q, want 22 (x must bind $2)", out)
	}
}

func TestBlankParamSandwiched(t *testing.T) {
	// f(a: int, _: int, c: int): a reads $1, c reads $3.
	src := "fn f(a: int, _: int, c: int) -> int { return a * 100 + c }\n" +
		"fn main() -> int {\n" +
		"print(to_string(f(1, 2, 3)))\n" +
		"return 0\n}"
	out, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if out != "103\n" {
		t.Errorf("stdout = %q, want 103 (a=$1, c=$3)", out)
	}
}

// The generated shell for a blank param must not emit a $1-to-x assignment when a
// blank precedes x.
func TestBlankParamShapeNoMisassign(t *testing.T) {
	src := "fn f(_: int, x: int) -> int { return x }\n" +
		"fn main() -> int { return f(1, 2) }\n"
	out := string(compile(t, src))

	// Isolate the generated body of f (between its opening brace and the blank
	// line that follows it).  The prelude contains many ="$2" occurrences so
	// asserting on the raw output is not specific enough.
	start := strings.Index(out, "__wisp_f_m0_f()")
	if start == -1 {
		t.Fatalf("could not find f's function definition in output:\n%s", out)
	}
	// Find the closing brace line of the function body.
	end := strings.Index(out[start:], "\n}\n")
	if end == -1 {
		t.Fatalf("could not find end of f's function body in output:\n%s", out)
	}
	fBody := out[start : start+end]

	// x is the only named param; it sits in slot 2 so it must read $2.
	if !strings.Contains(fBody, `="$2"`) {
		t.Errorf("f body: expected x to be assigned from $2, got:\n%s", fBody)
	}
	// No positional copy from $1 — that slot is blank and must be silently dropped.
	if strings.Contains(fBody, `="$1"`) {
		t.Errorf("f body: unexpected $1 assignment (blank param must not be copied), got:\n%s", fBody)
	}
}

func TestBlankMatchSome(t *testing.T) {
	src := wrapMainCG("let o: Optional[int] = Some(5)\nmatch (o) { case Some(_) { print(\"yes\") } case None { print(\"no\") } }")
	out, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if out != "yes\n" {
		t.Errorf("stdout = %q, want yes", out)
	}
}

func TestBlankMatchErrBranch(t *testing.T) {
	src := wrapMainCG("let r: Result[int] = Err(error(\"boom\"))\nmatch (r) { case Err(_) { print(\"caught\") } case Ok(_) { print(\"ok\") } }")
	out, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if out != "caught\n" {
		t.Errorf("stdout = %q, want caught", out)
	}
}

func TestBlankMatchNoneTakesOther(t *testing.T) {
	src := wrapMainCG("let o: Optional[int] = None\nmatch (o) { case Some(_) { print(\"yes\") } case None { print(\"no\") } }")
	out, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if out != "no\n" {
		t.Errorf("stdout = %q, want no", out)
	}
}

// blank Ok arm must not fire on an Err value.
func TestBlankMatchOkOnErrTakesOther(t *testing.T) {
	src := wrapMainCG("let r: Result[int] = Err(error(\"boom\"))\nmatch (r) { case Ok(_) { print(\"yes\") } case Err(_) { print(\"no\") } }")
	out, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if out != "no\n" {
		t.Errorf("stdout = %q, want no (Ok arm must not match Err)", out)
	}
}

// blank Err arm must not fire on an Ok value.
func TestBlankMatchErrOnOkTakesOther(t *testing.T) {
	src := wrapMainCG("let r: Result[int] = Ok(1)\nmatch (r) { case Err(_) { print(\"yes\") } case Ok(_) { print(\"no\") } }")
	out, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if out != "no\n" {
		t.Errorf("stdout = %q, want no (Err arm must not match Ok)", out)
	}
}

func TestBlankCatchRunsHandler(t *testing.T) {
	src := wrapMainCG("try { throw error(\"boom\") } catch (_) { print(\"caught\") }")
	out, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, out=%q", code, out)
	}
	if out != "caught\n" {
		t.Errorf("stdout = %q, want caught", out)
	}
}

// AC 8b: with no fault thrown, a blank-catch handler must NOT run.
func TestBlankCatchNoErrorSkipsHandler(t *testing.T) {
	src := wrapMainCG("try { print(\"body\") } catch (_) { print(\"caught\") }")
	out, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, out=%q", code, out)
	}
	if out != "body\n" {
		t.Errorf("stdout = %q, want body (handler must not run when no fault)", out)
	}
}
