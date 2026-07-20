package codegen

import (
	"strings"
	"testing"
)

// TestIntMin_LetEmitsLiteralNotArith: a let binding of the INT_MIN literal must
// emit a plain =-9223372036854775808 assignment, not a $(( 0 - ... )) form that
// would put the out-of-range 2^63 magnitude token inside arithmetic expansion.
func TestIntMin_LetEmitsLiteralNotArith(t *testing.T) {
	src := `fn main() -> int {
  let m: int = -9223372036854775808
  print(to_string(m))
  return 0
}`
	s := string(compile(t, src))

	if !strings.Contains(s, "=-9223372036854775808") {
		t.Fatalf("expected bare literal assignment =-9223372036854775808 in output:\n%s", s)
	}
	if strings.Contains(s, "$(( 0 - 9223372036854775808 ))") {
		t.Fatalf("INT_MIN must not appear as $(( 0 - 9223372036854775808 )) in output:\n%s", s)
	}
}

// TestIntMin_ArithOperandSpills: when INT_MIN appears as an arithmetic operand,
// arith() must spill it to a temp with a bare literal assignment and reference
// that temp BARE (no leading $) inside $(( )). The dollar form $<temp> would
// string-expand to the literal -9223372036854775808 and re-lex the 2^63 token,
// reproducing the dash off-by-one.
func TestIntMin_ArithOperandSpills(t *testing.T) {
	src := `fn main() -> int {
  let z: int = -9223372036854775808 + 1
  print(to_string(z))
  return 0
}`
	s := string(compile(t, src))

	// The spill assignment must exist.
	if !strings.Contains(s, "=-9223372036854775808") {
		t.Fatalf("expected spill assignment =-9223372036854775808 in output:\n%s", s)
	}
	// The bare 2^63 literal must not appear directly inside $(( )).
	if strings.Contains(s, "$(( -9223372036854775808") {
		t.Fatalf("INT_MIN must not appear bare inside $(( )) in output:\n%s", s)
	}
	// The dollar form $<temp> must not appear inside a $(( )) arith expression
	// containing the spilled temp -- the temp must be referenced BARE.
	// We check by confirming no $(( $... form follows the spill.
	if strings.Contains(s, "$(( $") {
		t.Fatalf("spilled INT_MIN temp must be referenced BARE (no $) inside $(( )) in output:\n%s", s)
	}
}

// TestIntMin_ConstArithSpills: same spill requirement when the INT_MIN value
// comes through a const (the foldedLitAtom -> litAtom entry point).
func TestIntMin_ConstArithSpills(t *testing.T) {
	src := `fn main() -> int {
  const M: int = -9223372036854775808
  let z: int = M + 1
  print(to_string(z))
  return 0
}`
	s := string(compile(t, src))

	if !strings.Contains(s, "=-9223372036854775808") {
		t.Fatalf("expected spill assignment =-9223372036854775808 in output:\n%s", s)
	}
	if strings.Contains(s, "$(( -9223372036854775808") {
		t.Fatalf("INT_MIN must not appear bare inside $(( )) in output:\n%s", s)
	}
	if strings.Contains(s, "$(( $") {
		t.Fatalf("spilled INT_MIN temp must be referenced BARE (no $) inside $(( )) in output:\n%s", s)
	}
}

// TestIntMin_Canonicalization: only the exact INT_MIN magnitude takes the new
// branch. Leading-zero spellings canonicalize to INT_MIN and also spill;
// small literals like -007 and -0 use the existing $(( 0 - ... )) form.
func TestIntMin_Canonicalization(t *testing.T) {
	// Leading-zero INT_MIN: -09223372036854775808 canonicalizes to
	// -9223372036854775808 and must also hit the spill branch.
	srcLeadZero := `fn main() -> int {
  let m: int = -09223372036854775808
  print(to_string(m))
  return 0
}`
	sLZ := string(compile(t, srcLeadZero))
	if !strings.Contains(sLZ, "=-9223372036854775808") {
		t.Fatalf("leading-zero INT_MIN: expected spill =-9223372036854775808 in output:\n%s", sLZ)
	}
	if strings.Contains(sLZ, "$(( 0 - 9223372036854775808 ))") {
		t.Fatalf("leading-zero INT_MIN: must not appear as $(( 0 - 9223372036854775808 )):\n%s", sLZ)
	}

	// -007 is not INT_MIN; must emit $(( 0 - 7 )).
	srcSeven := `fn main() -> int {
  let x: int = -007
  print(to_string(x))
  return 0
}`
	sSeven := string(compile(t, srcSeven))
	if !strings.Contains(sSeven, "$(( 0 - 7 ))") {
		t.Fatalf("-007: expected $(( 0 - 7 )) in output:\n%s", sSeven)
	}

	// -0 is not INT_MIN; must emit $(( 0 - 0 )).
	srcZero := `fn main() -> int {
  let x: int = -0
  print(to_string(x))
  return 0
}`
	sZero := string(compile(t, srcZero))
	if !strings.Contains(sZero, "$(( 0 - 0 ))") {
		t.Fatalf("-0: expected $(( 0 - 0 )) in output:\n%s", sZero)
	}
}

// TestIntMin_NonLiteralNegationUnchanged: the fix must not affect non-literal
// negation. All three operand shapes must still emit the $(( 0 - ... )) form.
func TestIntMin_NonLiteralNegationUnchanged(t *testing.T) {
	// Variable operand: -x
	srcVar := `fn main() -> int {
  let x: int = 5
  let y: int = -x
  print(to_string(y))
  return 0
}`
	sVar := string(compile(t, srcVar))
	if !strings.Contains(sVar, "$(( 0 - ") {
		t.Fatalf("variable negation: expected $(( 0 - ... )) in output:\n%s", sVar)
	}
	if strings.Contains(sVar, "=-9223372036854775808") {
		t.Fatalf("variable negation must not emit INT_MIN spill:\n%s", sVar)
	}

	// Parenthesized expression: -(a + b)
	srcParen := `fn main() -> int {
  let a: int = 2
  let b: int = 3
  let y: int = -(a + b)
  print(to_string(y))
  return 0
}`
	sParen := string(compile(t, srcParen))
	if !strings.Contains(sParen, "$(( 0 - ") {
		t.Fatalf("paren negation: expected $(( 0 - ... )) in output:\n%s", sParen)
	}
	if strings.Contains(sParen, "=-9223372036854775808") {
		t.Fatalf("paren negation must not emit INT_MIN spill:\n%s", sParen)
	}

	// Call operand: -to_int(...). (abs is a removable builtin (math.abs) whose bare
	// call no longer resolves in the single-module check; to_int is stays-flat and
	// serves identically as a call operand for this negation-shape assertion.)
	srcCall := `fn main() -> int {
  let x: int = 5
  let y: int = -to_int(to_string(x))
  print(to_string(y))
  return 0
}`
	sCall := string(compile(t, srcCall))
	if !strings.Contains(sCall, "$(( 0 - ") {
		t.Fatalf("call negation: expected $(( 0 - ... )) in output:\n%s", sCall)
	}
	if strings.Contains(sCall, "=-9223372036854775808") {
		t.Fatalf("call negation must not emit INT_MIN spill:\n%s", sCall)
	}
}
