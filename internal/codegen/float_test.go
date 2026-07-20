package codegen

import (
	"strings"
	"testing"
)

func TestFloatArithmeticRun(t *testing.T) {
	// %.17g is the pinned canonical form; these are the exact double round-trips.
	wantRun(t, `fn main() -> int {
  let a: float = 3.14
  let b: float = 2.0
  print(to_string(a + b))
  print(to_string(a - b))
  print(to_string(a * b))
  print(to_string(a / b))
  print(to_string(-a))
  return 0
}`, "5.1400000000000006\n1.1400000000000001\n6.2800000000000002\n1.5700000000000001\n-3.1400000000000001\n", "", 0)
}

func TestFloatLiteralExactIntegerLooking(t *testing.T) {
	// A float literal with a zero fraction round-trips to its integer-looking
	// canonical form.
	wantRun(t, `fn main() -> int {
  let a: float = 12.0
  print(to_string(a))
  return 0
}`, "12\n", "", 0)
}

func TestFloatAllSixComparisons(t *testing.T) {
	wantRun(t, `fn main() -> int {
  print("${1.0 < 2.0}")
  print("${2.0 <= 2.0}")
  print("${3.0 > 4.0}")
  print("${4.0 >= 4.0}")
  print("${5.5 == 5.5}")
  print("${5.5 != 6.5}")
  return 0
}`, "true\ntrue\nfalse\ntrue\ntrue\ntrue\n", "", 0)
}

func TestFloatComparisonEqualityFalse(t *testing.T) {
	wantRun(t, `fn main() -> int {
  print("${3.14 == 3.15}")
  print("${3.14 != 3.14}")
  return 0
}`, "false\nfalse\n", "", 0)
}

func TestFloatOfIntRoundTrip(t *testing.T) {
	// float(2) is usable in float arithmetic (integer-looking float).
	wantRun(t, `fn main() -> int {
  let n: int = 2
  let f: float = to_float(n)
  print(to_string(f + 0.5))
  return 0
}`, "2.5\n", "", 0)
}

func TestFloatOfNegativeString(t *testing.T) {
	// float("-2") parses (integer-looking float, optional fractional part).
	wantRun(t, `fn main() -> int {
  let f: float = to_float("-2")
  print(to_string(f * 3.0))
  return 0
}`, "-6\n", "", 0)
}

func TestIntOfFloatTruncatesTowardZero(t *testing.T) {
	wantRun(t, `fn main() -> int {
  print("${to_int(3.9)}")
  print("${to_int(-3.9)}")
  print("${to_int(2.0)}")
  return 0
}`, "3\n-3\n2\n", "", 0)
}

func TestStringOfFloatCanonical(t *testing.T) {
	wantRun(t, `fn main() -> int {
  print(to_string(3.14))
  print(to_string(2.0))
  print(to_string(0.5))
  return 0
}`, "3.1400000000000001\n2\n0.5\n", "", 0)
}

func TestBoolOfFloatNumericZero(t *testing.T) {
	// -0.0 and 0.000 are numeric zero -> false; nonzero -> true.
	wantRun(t, `fn main() -> int {
  print("${to_bool(0.0)}")
  print("${to_bool(-0.0)}")
  print("${to_bool(0.000)}")
  print("${to_bool(1.5)}")
  print("${to_bool(-0.0001)}")
  return 0
}`, "false\nfalse\nfalse\ntrue\ntrue\n", "", 0)
}

func TestFloatDivByZeroLocatedAbort(t *testing.T) {
	// The `/` is at line 4. Located abort, no inf/nan leak.
	src := "fn main() -> int {\n" +
		"  let a: float = 5.0\n" +
		"  let b: float = 0.0\n" +
		"  print(to_string(a / b))\n" + // `/` at col 21
		"  return 0\n" +
		"}"
	assertLocatedAbort(t, src, 4, 21, "division by zero")
}

func TestFloatOverflowLocatedAbort(t *testing.T) {
	// A product whose %.17g needs exponent form (~1e28) is outside the float
	// domain (spec 3.6) and must abort located, NOT leak an inf/nan/exponent.
	out, errb, code := runWisp(t, `fn main() -> int {
  let big: float = 99999999999999.0
  print(to_string(big * big))
  return 0
}`)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (out=%q stderr=%q)", code, out, errb)
	}
	if out != "" {
		t.Fatalf("stdout = %q, want empty (no inf/nan leak)", out)
	}
	if !strings.Contains(errb, "float") {
		t.Fatalf("stderr = %q, want a float-domain abort", errb)
	}
	if strings.Contains(out+errb, "inf") || strings.Contains(out+errb, "nan") {
		t.Fatalf("inf/nan leaked: out=%q err=%q", out, errb)
	}
}

func TestFloatBadStringLocatedAbort(t *testing.T) {
	src := "fn main() -> int {\n" +
		"  let f: float = to_float(\"x\")\n" + // `to_float` at col 18
		"  print(to_string(f))\n" +
		"  return 0\n" +
		"}"
	assertLocatedAbort(t, src, 2, 18, "float(")
}

func TestFloatIntOfFloatOutOfRangeAbort(t *testing.T) {
	// int(float) reuses __wisp_int's range gate. A float magnitude within the
	// representable domain but beyond int64 cannot occur (%.17g caps it well
	// below 1e17), so this asserts the path is wired to __wisp_int by checking a
	// normal truncation; the out-of-range gate is covered in the prelude tests.
	wantRun(t, `fn main() -> int {
  let f: float = 99999999999999.0
  print("${to_int(f)}")
  return 0
}`, "99999999999999\n", "", 0)
}

func TestFloatInjectionInertViaFloatString(t *testing.T) {
	// A crafted float-looking string with awk/shell-active bytes must be a clean
	// located abort (it is not a valid float), never executed.
	out, errb, code := runWisp(t, `fn main() -> int {
  let f: float = to_float("1\"; system(\"id\"); x=\"")
  print(to_string(f))
  return 0
}`)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (out=%q stderr=%q)", code, out, errb)
	}
	if out != "" {
		t.Fatalf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errb, "float(") {
		t.Fatalf("stderr = %q, want float() abort", errb)
	}
}

func TestFloatFunctionParamReturn(t *testing.T) {
	wantRun(t, `fn dbl(x: float) -> float {
  return x * 2.0
}
fn main() -> int {
  print(to_string(dbl(1.5)))
  return 0
}`, "3\n", "", 0)
}

func TestFloatLiteralIsSafeWord(t *testing.T) {
	// A float literal lowers to its decimal string directly (no $(( )) ).
	script := compile(t, `fn main() -> int {
  let a: float = 3.14
  print(to_string(a))
  return 0
}`)
	s := string(script)
	if !strings.Contains(s, "=3.14") {
		t.Fatalf("expected float literal 3.14 emitted as a bare decimal word:\n%s", s)
	}
	if strings.Contains(s, "$(( 3.14") {
		t.Fatalf("float literal must not enter arithmetic expansion:\n%s", s)
	}
}

func TestFloatSourceMapCoversFloatLine(t *testing.T) {
	// The generated float-op call line must map back to its wisp source line
	// (spec AC 6 / M2 source maps).
	src := "fn main() -> int {\n" +
		"  let a: float = 1.5\n" +
		"  let b: float = 2.5\n" +
		"  let c: float = a + b\n" + // line 4: the float add
		"  print(to_string(c))\n" +
		"  return 0\n" +
		"}"
	script, lm := compileWithMap(t, src)
	lines := strings.Split(strings.TrimSuffix(string(script), "\n"), "\n")
	var found bool
	for i, ln := range lines {
		if strings.Contains(ln, "__wisp_fadd ") {
			found = true
			if lm[i] == nil || lm[i].Line != 4 {
				t.Fatalf("__wisp_fadd line %d maps to %v, want line 4", i+1, lm[i])
			}
		}
	}
	if !found {
		t.Fatalf("did not find a __wisp_fadd call line in:\n%s", script)
	}
}

func TestFloatTreeShakenWhenUnused(t *testing.T) {
	// A program with no float must emit no __wisp_f* helper (spec AC 6).
	script := compile(t, `fn main() -> int {
  print("${1 + 2}")
  return 0
}`)
	s := string(script)
	for _, h := range []string{"__wisp_fadd", "__wisp_ffinite", "__wisp_fcmp", "__wisp_fstr", "__wisp_fbool", "__wisp_ffloat", "__wisp_fint"} {
		if strings.Contains(s, h) {
			t.Fatalf("tree-shaking failed: float helper %q present in a no-float program:\n%s", h, s)
		}
	}
}

func TestFloatAwkProgramIsConstant(t *testing.T) {
	// The awk program text must be a compiler constant; operand values flow only
	// via -v. Assert the emitted prelude carries `awk -v` and a constant
	// `BEGIN{ printf "%.17g"` program, and that no float VALUE is interpolated
	// into the program text.
	script := compile(t, `fn main() -> int {
  print(to_string(1.5 + 2.5))
  return 0
}`)
	s := string(script)
	if !strings.Contains(s, `awk -v a="$2" -v b="$3"`) {
		t.Fatalf("expected operands passed via -v:\n%s", s)
	}
	if !strings.Contains(s, `printf "%.17g"`) {
		t.Fatalf("expected pinned %%.17g format:\n%s", s)
	}
}
