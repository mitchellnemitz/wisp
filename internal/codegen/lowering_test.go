package codegen

import (
	"strings"
	"testing"
)

func wantRun(t *testing.T, src, wantOut, wantErr string, wantCode int) {
	t.Helper()
	out, errb, code := runWisp(t, src)
	if code != wantCode {
		t.Fatalf("exit = %d, want %d (stderr=%q)", code, wantCode, errb)
	}
	if wantOut != "" || out != "" {
		if out != wantOut {
			t.Fatalf("stdout = %q, want %q", out, wantOut)
		}
	}
	if wantErr != "" && !strings.Contains(errb, wantErr) {
		t.Fatalf("stderr = %q, want substring %q", errb, wantErr)
	}
}

func TestArithmetic(t *testing.T) {
	wantRun(t, `fn main() -> int {
  let a: int = 7
  let b: int = 3
  print("${a + b}")
  print("${a - b}")
  print("${a * b}")
  print("${a / b}")
  print("${a % b}")
  print("${-a}")
  return 0
}`, "10\n4\n21\n2\n1\n-7\n", "", 0)
}

func TestDivByZeroAborts(t *testing.T) {
	wantRun(t, `fn main() -> int {
  let a: int = 5
  let b: int = 0
  print("${a / b}")
  return 0
}`, "", "division by zero", 1)
}

func TestModByZeroAborts(t *testing.T) {
	wantRun(t, `fn main() -> int {
  let a: int = 5
  let b: int = 0
  print("${a % b}")
  return 0
}`, "", "division by zero", 1)
}

func TestIntComparisons(t *testing.T) {
	wantRun(t, `fn main() -> int {
  print("${1 < 2}")
  print("${2 <= 2}")
  print("${3 > 4}")
  print("${4 >= 4}")
  print("${5 == 5}")
  print("${5 != 6}")
  return 0
}`, "true\ntrue\nfalse\ntrue\ntrue\ntrue\n", "", 0)
}

func TestBoolEquality(t *testing.T) {
	wantRun(t, `fn main() -> int {
  let a: bool = true
  let b: bool = false
  print("${a == a}")
  print("${a == b}")
  print("${a != b}")
  return 0
}`, "true\nfalse\ntrue\n", "", 0)
}

func TestStringEquality(t *testing.T) {
	wantRun(t, `fn main() -> int {
  let a: string = "x"
  let b: string = "y"
  print("${a == a}")
  print("${a == b}")
  print("${a != b}")
  return 0
}`, "true\nfalse\ntrue\n", "", 0)
}

func TestBoolOps(t *testing.T) {
	wantRun(t, `fn main() -> int {
  print("${true && true}")
  print("${true && false}")
  print("${false || true}")
  print("${false || false}")
  print("${!true}")
  print("${!false}")
  return 0
}`, "true\nfalse\ntrue\nfalse\nfalse\ntrue\n", "", 0)
}

func TestStringConcat(t *testing.T) {
	wantRun(t, `fn main() -> int {
  let a: string = "foo"
  let b: string = "bar"
  print(a + b)
  return 0
}`, "foobar\n", "", 0)
}

func TestIfElifElse(t *testing.T) {
	src := `fn classify(x: int) -> string {
  if (x > 10) {
    return "big"
  } else if (x > 0) {
    return "small"
  } else {
    return "nonpositive"
  }
}
fn main() -> int {
  print(classify(50))
  print(classify(5))
  print(classify(-3))
  return 0
}`
	wantRun(t, src, "big\nsmall\nnonpositive\n", "", 0)
}

func TestWhile(t *testing.T) {
	wantRun(t, `fn main() -> int {
  let i: int = 0
  let total: int = 0
  while (i < 5) {
    total = total + i
    i = i + 1
  }
  print("${total}")
  return 0
}`, "10\n", "", 0)
}

func TestForLoop(t *testing.T) {
	wantRun(t, `fn main() -> int {
  for (let i: int = 0; i < 3; i = i + 1) {
    print("${i}")
  }
  return 0
}`, "0\n1\n2\n", "", 0)
}

func TestForSiblingLoopVarReuse(t *testing.T) {
	// Two sequential for loops reusing the name i must compile and run (spec rule
	// 11): they are disjoint sibling scopes.
	wantRun(t, `fn main() -> int {
  for (let i: int = 0; i < 2; i = i + 1) {
    print("a${i}")
  }
  for (let i: int = 0; i < 2; i = i + 1) {
    print("b${i}")
  }
  return 0
}`, "a0\na1\nb0\nb1\n", "", 0)
}

func TestForContinueRunsPost(t *testing.T) {
	// continue must still run the post statement (i increments), so this
	// terminates and skips even values' bodies.
	wantRun(t, `fn main() -> int {
  for (let i: int = 0; i < 5; i = i + 1) {
    if (i % 2 == 0) {
      continue
    }
    print("${i}")
  }
  return 0
}`, "1\n3\n", "", 0)
}

func TestForBreak(t *testing.T) {
	wantRun(t, `fn main() -> int {
  for (let i: int = 0; i < 10; i = i + 1) {
    if (i == 3) {
      break
    }
    print("${i}")
  }
  return 0
}`, "0\n1\n2\n", "", 0)
}

// TestNestedBreakContinueCombined proves break targets the real (inner) loop AND
// the outer for's post still runs on continue, together (spec section 9.4,
// acceptance criterion 2).
func TestNestedBreakContinueCombined(t *testing.T) {
	src := `fn main() -> int {
  for (let i: int = 0; i < 4; i = i + 1) {
    if (i == 1) {
      continue
    }
    let j: int = 0
    while (j < 10) {
      if (j == 2) {
        break
      }
      print("i${i}j${j}")
      j = j + 1
    }
  }
  return 0
}`
	// i=0: inner prints j0,j1 then break. i=1: continue (post runs -> i=2).
	// i=2: inner prints j0,j1. i=3: inner prints j0,j1. then i=4 stops.
	want := "i0j0\ni0j1\ni2j0\ni2j1\ni3j0\ni3j1\n"
	wantRun(t, src, want, "", 0)
}

func TestNestedForBreakInnerOnly(t *testing.T) {
	// break in an inner for must exit only the inner for, not the outer.
	src := `fn main() -> int {
  for (let i: int = 0; i < 2; i = i + 1) {
    for (let j: int = 0; j < 5; j = j + 1) {
      if (j == 1) {
        break
      }
      print("i${i}j${j}")
    }
    print("after-inner-${i}")
  }
  return 0
}`
	want := "i0j0\nafter-inner-0\ni1j0\nafter-inner-1\n"
	wantRun(t, src, want, "", 0)
}

func TestSwitchInt(t *testing.T) {
	src := `fn label(code: int) -> string {
  switch (code) {
    case 0 {
      return "ok"
    }
    case 1, 2 {
      return "retry"
    }
    default {
      return "fail"
    }
  }
}
fn main() -> int {
  print(label(0))
  print(label(1))
  print(label(2))
  print(label(9))
  return 0
}`
	wantRun(t, src, "ok\nretry\nretry\nfail\n", "", 0)
}

func TestSwitchStringMetachars(t *testing.T) {
	// Case values with shell-active and case-pattern metacharacters must match
	// literally (spec section 9.4): * ? [ ] \ | and $.
	src := `fn main() -> int {
  let v: string = "a*b"
  switch (v) {
    case "x?y" {
      print("wrong1")
    }
    case "a*b" {
      print("star")
    }
    default {
      print("default")
    }
  }
  let w: string = "literal[x]"
  switch (w) {
    case "literal[x]" {
      print("bracket")
    }
    default {
      print("nope")
    }
  }
  return 0
}`
	wantRun(t, src, "star\nbracket\n", "", 0)
}

func TestSwitchStarSubjectNotGlob(t *testing.T) {
	// A subject of "*" must not act as a glob that matches a literal case "x".
	src := `fn main() -> int {
  let v: string = "*"
  switch (v) {
    case "x" {
      print("wrong")
    }
    case "*" {
      print("literal-star")
    }
    default {
      print("default")
    }
  }
  return 0
}`
	wantRun(t, src, "literal-star\n", "", 0)
}

func TestFunctionsAndRecursion(t *testing.T) {
	src := `fn fact(n: int) -> int {
  if (n <= 1) {
    return 1
  }
  return n * fact(n - 1)
}
fn main() -> int {
  print("${fact(5)}")
  return 0
}`
	wantRun(t, src, "120\n", "", 0)
}

func TestPrintToStderr(t *testing.T) {
	out, errb, code := runWisp(t, `fn main() -> int {
  print("out")
  print("err", stderr)
  return 0
}`)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if out != "out\n" {
		t.Fatalf("stdout = %q", out)
	}
	if errb != "err\n" {
		t.Fatalf("stderr = %q", errb)
	}
}

func TestExitCodeFromMain(t *testing.T) {
	wantRun(t, `fn main() -> int {
  return 7
}`, "", "", 7)
}

// --- builtins ---

// TestBuiltins exercises the stays-flat builtin catalog (length + the to_*
// conversions). The removable lower (string.lower), upper (string.upper), trim
// (string.trim) and replace (string.replace) lines were dropped: their bare
// calls no longer resolve in the single-module codegen check. Their runtime
// behavior lives in internal/golden (builtins_all.wisp uses string.replace and
// string.upper; lower_trailing_nl.wisp uses string.lower; str_ops.wisp exercises
// the string module); string.upper's byte shape is covered by
// core_byteidentity_test.go (the string|string.upper subset twin).
func TestBuiltins(t *testing.T) {
	src := `fn main() -> int {
  print("${length("hello")}")
  print(to_string(42))
  print("${to_int("123")}")
  print("${to_bool(0)}")
  print("${to_bool(5)}")
  print("${to_bool("true")}")
  return 0
}`
	wantRun(t, src, "5\n42\n123\nfalse\ntrue\ntrue\n", "", 0)
}

func TestIntBuiltinBadInputAborts(t *testing.T) {
	wantRun(t, `fn main() -> int {
  print("${to_int("abc")}")
  return 0
}`, "", "int(", 1)
}

func TestBoolBuiltinBadInputAborts(t *testing.T) {
	wantRun(t, `fn main() -> int {
  print("${to_bool("yes")}")
  return 0
}`, "", "bool(", 1)
}

// TestReplaceEmptySearchAborts: an empty search string aborts at runtime (exit
// 1, stderr names the replace() call). Reconstructed with the namespaced
// string.replace; the delegate lowers byte-identically to the pre-removal flat
// replace, so the empty-search abort is unchanged.
func TestReplaceEmptySearchAborts(t *testing.T) {
	_, errb, code := runNS(t, `fn main() -> int {
  print(string.replace("abc", "", "x"))
  return 0
}`, "string")
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr=%q)", code, errb)
	}
	if !strings.Contains(errb, "replace(") {
		t.Fatalf("stderr = %q, want substring %q", errb, "replace(")
	}
}

// --- default arguments ---

func TestDefaultArgument(t *testing.T) {
	src := `fn log(msg: string, prefix: string = "[info] ") -> void {
  print(prefix + msg)
}
fn main() -> int {
  log("hello")
  log("hello", "[warn] ")
  return 0
}`
	wantRun(t, src, "[info] hello\n[warn] hello\n", "", 0)
}

// --- left to right evaluation ---

func TestLeftToRightEvaluation(t *testing.T) {
	src := `fn mark(s: string) -> int {
  print(s)
  return 0
}
fn add(a: int, b: int) -> int {
  return a + b
}
fn main() -> int {
  let r: int = add(mark("first"), mark("second"))
  print("${r}")
  return 0
}`
	wantRun(t, src, "first\nsecond\n0\n", "", 0)
}
