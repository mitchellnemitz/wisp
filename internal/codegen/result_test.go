package codegen

import (
	"strings"
	"testing"
)

func TestResultLoweringShape(t *testing.T) {
	out := string(compile(t, wrapMainCG(`let r: Result[int] = Ok(7)`+"\n"+`let b: bool = is_ok(r)`+"\n"+`let v: int = unwrap(r)`)))
	if !strings.Contains(out, "}_tag=") {
		t.Errorf("Result: expected a _tag field assignment, got:\n%s", out)
	}
	// Single payload slot: there is no separate _err field.
	if strings.Contains(out, "_err=") {
		t.Errorf("Result: expected NO _err field assignment (single _value slot), got:\n%s", out)
	}
}

func TestResultBehavioralEndToEnd(t *testing.T) {
	src := wrapMainCG(
		`let r: Result[int] = Ok(7)` + "\n" +
			`print(to_string(is_ok(r)))` + "\n" +
			`print(to_string(unwrap(r)))` + "\n" +
			`let e: Result[int] = Err(error("boom"))` + "\n" +
			`print(to_string(is_err(e)))` + "\n" +
			`print(to_string(unwrap_or(e, 9)))` + "\n" +
			`print(unwrap_err(e).message)`)
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	want := "true\n7\ntrue\n9\nboom\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestResultIsErrAndInverse(t *testing.T) {
	src := wrapMainCG(
		`let e: Result[int] = Err(error("x"))` + "\n" +
			`let o: Result[int] = Ok(1)` + "\n" +
			`print(to_string(is_err(e)))` + "\n" +
			`print(to_string(is_ok(e)))` + "\n" +
			`print(to_string(is_err(o)))`)
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	if stdout != "true\nfalse\nfalse\n" {
		t.Errorf("stdout = %q, want %q", stdout, "true\nfalse\nfalse\n")
	}
}

func TestUnwrapOfErrCaughtCarriesMessage(t *testing.T) {
	src := wrapMainCG(
		`let e: Result[int] = Err(error("bad"))` + "\n" +
			`try {` + "\n" +
			`let v: int = unwrap(e)` + "\n" +
			`print(to_string(v))` + "\n" +
			`} catch (x) {` + "\n" +
			`print("caught " + x.message)` + "\n" +
			`}`)
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	// Located abort (__wisp_fail) prefixes the source position, like unwrap-of-None;
	// the carried error message is preserved after it.
	if !strings.HasPrefix(stdout, "caught ") || !strings.Contains(stdout, "bad") {
		t.Errorf("stdout = %q, want a caught message containing the carried %q", stdout, "bad")
	}
}

func TestUnwrapErrOfOkAborts(t *testing.T) {
	src := wrapMainCG(
		`let r: Result[int] = Ok(1)` + "\n" +
			`try {` + "\n" +
			`let e: error = unwrap_err(r)` + "\n" +
			`print(e.message)` + "\n" +
			`} catch (x) {` + "\n" +
			`print("caught " + x.message)` + "\n" +
			`}`)
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	if !strings.HasPrefix(stdout, "caught ") || !strings.Contains(stdout, "unwrap_err of Ok") {
		t.Errorf("stdout = %q, want a caught message containing %q", stdout, "unwrap_err of Ok")
	}
}

func TestUnwrapOrEagerFallback(t *testing.T) {
	src := "fn side() -> int {\nprint(\"evaluated\")\nreturn 9\n}\n" +
		wrapMainCG(`let r: Result[int] = Ok(1)`+"\n"+`let v: int = unwrap_or(r, side())`+"\n"+`print(to_string(v))`)
	stdout, _, code := run(t, compile(t, src))
	if code != 0 {
		t.Fatalf("exit = %d, stdout=%q", code, stdout)
	}
	// Fallback runs (eager) though the value is Ok -> v == 1.
	if stdout != "evaluated\n1\n" {
		t.Errorf("stdout = %q, want %q", stdout, "evaluated\n1\n")
	}
}

func TestNoResultSentinelLeak(t *testing.T) {
	out := string(compile(t, wrapMainCG(`let r: Result[int] = Err(error("x"))`+"\n"+`let o: Result[int] = Ok(2)`+"\n"+`let b: bool = is_ok(o)`)))
	if strings.Contains(out, "[?]") {
		t.Errorf("expected no [?] sentinel in generated shell, got:\n%s", out)
	}
	shellcheck(t, []byte(out))
}
