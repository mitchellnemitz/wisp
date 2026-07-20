package codegen

import (
	"strings"
	"testing"
)

// TestInterpolationInert verifies a value containing shell-active text is
// inserted as inert data: it neither executes nor mis-parses (spec section 5.1 /
// 9.6 invariant 2, acceptance criterion 7).
func TestInterpolationInert(t *testing.T) {
	src := "fn main() -> int {\n" +
		"  let danger: string = \"$(echo PWNED); `echo BAD`; \\\"q\\\" & ; |\"\n" +
		"  print(\"value=${danger}\")\n" +
		"  return 0\n" +
		"}"
	out, errb, code := runWisp(t, src)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, errb)
	}
	want := "value=$(echo PWNED); `echo BAD`; \"q\" & ; |\n"
	if out != want {
		t.Fatalf("stdout = %q, want %q", out, want)
	}
	if strings.Contains(out, "PWNED") && !strings.Contains(out, "$(echo PWNED)") {
		t.Fatalf("command substitution executed: %q", out)
	}
}

// TestStringLiteralInert verifies a plain (non-interpolated) string literal with
// shell-active bytes is re-encoded and emitted inert (invariant 1).
func TestStringLiteralInert(t *testing.T) {
	// single-quoted source literal: no interpolation, literal bytes including a
	// single quote (via escape), dollar, backtick, backslash.
	src := "fn main() -> int {\n" +
		"  print('a$b`c\\\\d\\'e')\n" +
		"  return 0\n" +
		"}"
	out, errb, code := runWisp(t, src)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, errb)
	}
	want := "a$b`c\\d'e\n"
	if out != want {
		t.Fatalf("stdout = %q, want %q", out, want)
	}
}

// TestDollarSuppression verifies \$ is the literal text and \${x} does not begin
// an interpolation (spec section 5.1).
func TestDollarSuppression(t *testing.T) {
	src := "fn main() -> int {\n" +
		"  let x: int = 5\n" +
		"  print(\"literal \\${x} and value ${x}\")\n" +
		"  return 0\n" +
		"}"
	out, errb, code := runWisp(t, src)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, errb)
	}
	want := "literal ${x} and value 5\n"
	if out != want {
		t.Fatalf("stdout = %q, want %q", out, want)
	}
}

// TestSwitchCaseInjectionInert verifies a case value with command-substitution
// text matches literally and never executes.
func TestSwitchCaseInjectionInert(t *testing.T) {
	src := "fn main() -> int {\n" +
		"  let v: string = \"$(echo X)\"\n" +
		"  switch (v) {\n" +
		"    case \"$(echo X)\" {\n" +
		"      print(\"literal-match\")\n" +
		"    }\n" +
		"    default {\n" +
		"      print(\"default\")\n" +
		"    }\n" +
		"  }\n" +
		"  return 0\n" +
		"}"
	out, errb, code := runWisp(t, src)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, errb)
	}
	if out != "literal-match\n" {
		t.Fatalf("stdout = %q, want literal-match (case substitution executed or mismatched)", out)
	}
}

// TestAllExpansionsQuoted checks invariant 3: no unquoted parameter expansion is
// emitted. We scan emitted lines for a bare $var token outside the contexts
// where the compiler legitimately uses an unquoted form ($(( )) arithmetic and
// the `$N` positional copies, which are int/positional and safe).
func TestAllExpansionsQuoted(t *testing.T) {
	src := `fn add(a: int, b: int) -> int {
  return a + b
}
fn main() -> int {
  let s: string = "hi"
  print(s)
  print("${add(1, 2)}")
  return 0
}`
	script := string(compile(t, src))
	for _, ln := range strings.Split(script, "\n") {
		trimmed := strings.TrimSpace(ln)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		// arithmetic uses $name inside $(( )); skip those lines.
		if strings.Contains(ln, "$((") {
			continue
		}
		// detect an unquoted $word that is a wisp variable/temp/ret (begins __wisp
		// or __ret), i.e. "$__..." must always be preceded by a double quote.
		scanUnquoted(t, ln)
	}
}

// scanUnquoted fails if ln contains a `$__...` expansion that is not immediately
// preceded by a double quote (every compiler-emitted variable expansion must be
// double-quoted; section 9.6 invariant 3).
func scanUnquoted(t *testing.T, ln string) {
	t.Helper()
	for i := 0; i < len(ln); i++ {
		if ln[i] != '$' {
			continue
		}
		// skip $(( and $( and ${ — handled elsewhere or are command structure
		if i+1 < len(ln) && (ln[i+1] == '(' || ln[i+1] == '{') {
			continue
		}
		rest := ln[i+1:]
		if strings.HasPrefix(rest, "__wisp") || strings.HasPrefix(rest, "__ret") {
			if i == 0 || ln[i-1] != '"' {
				t.Fatalf("unquoted expansion in line: %q", ln)
			}
		}
	}
}

// TestTreeShaking checks acceptance criterion 5: a minimal program omits every
// helper it does not use.
func TestTreeShaking(t *testing.T) {
	// A program using only print must NOT emit replace/trim/lower/upper/int/bool
	// helper definitions.
	script := string(compile(t, `fn main() -> int {
  print("hi")
  return 0
}`))
	absent := []string{
		"__wisp_replace()",
		"__wisp_trim()",
		"__wisp_lower()",
		"__wisp_upper()",
		"__wisp_int()",
		"__wisp_bool_int()",
		"__wisp_bool_str()",
		"__wisp_length()",
		"__wisp_string()",
		"__wisp_fail()",
	}
	for _, a := range absent {
		if strings.Contains(script, a) {
			t.Fatalf("tree-shaking failed: unused helper %q present:\n%s", a, script)
		}
	}
	if !strings.Contains(script, "print()") {
		t.Fatalf("used helper print() missing")
	}
}

// TestTreeShakingPullsDeps checks that using `int` pulls in its __wisp_fail
// dependency but still omits unrelated helpers.
func TestTreeShakingPullsDeps(t *testing.T) {
	script := string(compile(t, `fn main() -> int {
  print("${to_int("42")}")
  return 0
}`))
	if !strings.Contains(script, "__wisp_int()") {
		t.Fatalf("int helper missing")
	}
	if !strings.Contains(script, "__wisp_fail()") {
		t.Fatalf("int's dependency __wisp_fail missing (tree-shake deps)")
	}
	if strings.Contains(script, "__wisp_replace()") {
		t.Fatalf("unrelated helper replace present")
	}
}
