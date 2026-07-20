package codegen

import (
	"strings"
	"testing"
)

// TestDebugShapeOptional asserts that debug(Optional[int]) reads _tag and
// branches on the "some" tag in a case statement.
func TestDebugShapeOptional(t *testing.T) {
	script := string(compile(t, `fn main() -> int {
  let o: Optional[int] = Some(1)
  let s: string = debug(o)
  print(s)
  return 0
}`))
	if !strings.Contains(script, "_tag") {
		t.Errorf("debug(Optional) should read _tag field:\n%s", script)
	}
	if !strings.Contains(script, "some)") {
		t.Errorf("debug(Optional) should branch on 'some' tag:\n%s", script)
	}
}

// TestDebugShapeArray asserts that debug(int[]) reads _len (array length) and
// iterates elements.
func TestDebugShapeArray(t *testing.T) {
	script := string(compile(t, `fn main() -> int {
  let xs: int[] = [1, 2]
  let s: string = debug(xs)
  print(s)
  return 0
}`))
	if !strings.Contains(script, "_len") {
		t.Errorf("debug([int]) should read _len:\n%s", script)
	}
}

// TestDebugShapeNoUnquotedExpansion asserts no unquoted $var in shell brackets.
func TestDebugShapeNoUnquotedExpansion(t *testing.T) {
	script := string(compile(t, `fn main() -> int {
  let xs: string[] = ["hello"]
  print(debug(xs))
  return 0
}`))
	if strings.Contains(script, "[ $") {
		t.Errorf("debug should not produce [ $var (unquoted):\n%s", script)
	}
}

// TestDebugShapeStringWrapsInQuotes asserts that debug(string) wraps the value
// in literal double-quote characters.
func TestDebugShapeStringWrapsInQuotes(t *testing.T) {
	script := string(compile(t, `fn main() -> int {
  let s: string = "hello"
  print(debug(s))
  return 0
}`))
	// The shell code should contain the quote-wrapping pattern '"'
	if !strings.Contains(script, `'"'`) {
		t.Errorf("debug(string) should produce quote-wrapping pattern:\n%s", script)
	}
}

// TestDebugShapeDictKeyDecDoubleQuoted asserts that the dict-key decode call
// routes the token through a double-quoted expansion ("$var"), not through a
// printf format string, eval operand, or bare word. Would fail if the routing
// regressed to an unquoted or format-string path.
func TestDebugShapeDictKeyDecDoubleQuoted(t *testing.T) {
	script := string(compile(t, `fn main() -> int {
  let m: {string: int} = { "x": 1 }
  print(debug(m))
  return 0
}`))
	// __wisp_dkey_dec must be called with a double-quoted "$var" argument.
	if !strings.Contains(script, `__wisp_dkey_dec "$`) {
		t.Errorf("dict key decode must use double-quoted expansion:\n%s", script)
	}
	// The decoded key must be wrapped via the '"'"$var"'"' double-quote pattern,
	// not placed in a printf format string.
	if !strings.Contains(script, `'"'"$`) {
		t.Errorf("dict key render must use double-quoted wrapping pattern:\n%s", script)
	}
	// The key must NOT appear directly in a printf format string.
	if strings.Contains(script, `printf '%s' $`) {
		t.Errorf("dict key must not flow through printf format string:\n%s", script)
	}
}

// TestDebugShapeErrorMessageDoubleQuoted asserts that the error message field
// is rendered through a double-quoted "$var" expansion, not a printf format
// string or eval operand. Would fail if the routing regressed.
func TestDebugShapeErrorMessageDoubleQuoted(t *testing.T) {
	script := string(compile(t, `fn main() -> int {
  let e: error = error("msg")
  print(debug(e))
  return 0
}`))
	// The error message assembly must use "$ (double-quoted var expansion).
	if !strings.Contains(script, `'"$`) {
		t.Errorf("error message must be rendered via double-quoted expansion:\n%s", script)
	}
	// Must not embed the message in a printf format string position.
	if strings.Contains(script, `printf "$`) {
		t.Errorf("error message must not flow through printf format string:\n%s", script)
	}
}

// TestDebugShapeShellcheck runs ShellCheck on debug-generated code.
func TestDebugShapeShellcheck(t *testing.T) {
	script := compile(t, `fn main() -> int {
  let xs: string[] = ["$(touch pwned)"]
  let o: Optional[int] = Some(1)
  let r: Result[int] = Ok(42)
  print(debug(xs))
  print(debug(o))
  print(debug(r))
  return 0
}`)
	shellcheck(t, script)
}
