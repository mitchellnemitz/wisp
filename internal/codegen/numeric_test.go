package codegen

import (
	"strings"
	"testing"
)

func numOut(t *testing.T, body string) string {
	t.Helper()
	out, errb, code := runWisp(t, "fn main() -> int {\n"+body+"\nreturn 0\n}\n")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errb)
	}
	return out
}

// numOutNS is numOut for programs that need namespaced members
// (clamp/sign/floor/ceil/round/trunc/sqrt/gcd/lcm -> math.*), since their bare
// spelling no longer resolves in the single-module check.
func numOutNS(t *testing.T, body string, namespaces ...string) string {
	t.Helper()
	out, errb, code := runNS(t, "fn main() -> int {\n"+body+"\nreturn 0\n}\n", namespaces...)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errb)
	}
	return out
}

func TestNumParseIntParseFloatUnwrapOr(t *testing.T) {
	out := numOutNS(t, `
print(to_string(unwrap_or(parse_int("007"), 0)))
print(to_string(unwrap_or(parse_int("x"), 9)))
print(to_string(unwrap_or(parse_int(" 5"), 9)))
print(to_string(unwrap_or(parse_int("-0"), 5)))
print(to_string(unwrap_or(parse_int("99999999999999999999999"), 7)))
print(to_string(unwrap_or(parse_float("1.50"), 0.0)))
print(to_string(unwrap_or(parse_float("1e5"), 0.0)))
print(to_string(unwrap_or(parse_float(" 1.0"), 0.0)))`)
	want := "7\n9\n9\n0\n7\n1.5\n0\n0\n"
	if out != want {
		t.Errorf("unwrap_or(parse_int/parse_float) = %q want %q", out, want)
	}
}

func TestNumClampSign(t *testing.T) {
	out := numOutNS(t, `
print(to_string(math.clamp(5, 1, 10)) + "," + to_string(math.clamp(15, 1, 10)) + "," + to_string(math.clamp(-3, 1, 10)))
print(to_string(math.clamp(5, 10, 1)))
print(to_string(math.clamp(2.5, 0.0, 2.0)) + "," + to_string(math.clamp(0.5, 1.0, 3.0)))
print(to_string(math.sign(-7)) + "," + to_string(math.sign(0)) + "," + to_string(math.sign(3)))
print(to_string(math.sign(-2.5)) + "," + to_string(math.sign(-0.0)) + "," + to_string(math.sign(4.5)))`, "math")
	want := "5,10,1\n10\n2,1\n-1,0,1\n-1,0,1\n"
	if out != want {
		t.Errorf("clamp/sign = %q want %q", out, want)
	}
}

func TestNumRounding(t *testing.T) {
	out := numOutNS(t, `
print(to_string(math.floor(2.7)) + "," + to_string(math.floor(-1.5)))
print(to_string(math.ceil(2.1)) + "," + to_string(math.ceil(-1.5)))
print(to_string(math.round(0.5)) + "," + to_string(math.round(-0.5)) + "," + to_string(math.round(2.5)) + "," + to_string(math.round(-1.6)))
print(to_string(math.trunc(1.9)) + "," + to_string(math.trunc(-1.9)))`, "math")
	want := "2,-2\n3,-1\n1,0,3,-2\n1,-1\n"
	if out != want {
		t.Errorf("rounding = %q want %q", out, want)
	}
}

func TestNumSqrtGcdLcm(t *testing.T) {
	// sqrt is pinned only on exact squares: Newton's method lands the irrational
	// roots within ~1 ulp, which would make a %.17g comparison brittle.
	out := numOutNS(t, `
print(to_string(math.sqrt(9.0)) + "," + to_string(math.sqrt(2.25)) + "," + to_string(math.sqrt(144.0)) + "," + to_string(math.sqrt(0.25)))
print(to_string(math.gcd(12, 18)) + "," + to_string(math.gcd(0, 0)) + "," + to_string(math.gcd(-12, 8)))
print(to_string(math.lcm(4, 6)) + "," + to_string(math.lcm(0, 5)) + "," + to_string(math.lcm(21, 6)))`, "math")
	want := "3,1.5,12,0.5\n6,0,4\n12,0,42\n"
	if out != want {
		t.Errorf("sqrt/gcd/lcm = %q want %q", out, want)
	}
}

func TestNumFaultsCaught(t *testing.T) {
	// sqrt(neg) reliably faults and is catchable. (floor/ceil/round are range-wired
	// via __wisp_int but their overflow is not portably triggerable: awk's %d clamps
	// huge doubles, so the out-of-range string never reaches the check - documented
	// as effectively total.)
	out := numOutNS(t, `
try { print(to_string(math.sqrt(-1.0))) } catch (e) { print("sqrt caught") }`, "math")
	if out != "sqrt caught\n" {
		t.Errorf("faults caught = %q", out)
	}
}

func TestFloatOverflowLiteralRejectedAtCompileTime(t *testing.T) {
	// R2 domain check: a float literal with a huge integer part (overflows to
	// +inf in awk, %.17g exponent form) is now rejected at compile time with a
	// "float literal out of domain" error. The +inf path in the sqrt Newton loop
	// is therefore unreachable via literals; verify the compile error is produced
	// instead of the literal reaching runtime.
	big := strings.Repeat("9", 400)
	if compileDecision(big + ".0") {
		t.Fatal("expected compile rejection for out-of-domain float literal, got accepted")
	}
}

func TestNumFaultTopLevel(t *testing.T) {
	for _, c := range []struct{ body, msg string }{
		{`print(to_string(math.sqrt(-1.0)))`, "sqrt(): non-finite"},
	} {
		_, errb, code := runNS(t, "fn main() -> int {\n"+c.body+"\nreturn 0\n}\n", "math")
		if code == 0 {
			t.Errorf("%q should abort", c.body)
		}
		if !strings.Contains(errb, c.msg) {
			t.Errorf("%q stderr = %q want %q", c.body, errb, c.msg)
		}
	}
}
