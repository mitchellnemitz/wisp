package types

// Task 7: switch cases accept const-exprs.
//
// Tests cover:
//   - case with a const ident is accepted (no error)
//   - case with a folded arithmetic expr is accepted (no error)
//   - case with a non-constant (let variable / function call) is still rejected
//   - duplicate case values detected by folded value (3600 vs 60*60)
//   - duplicate case values detected by folded value (const MAX=3600 vs literal 3600)

import (
	"strings"
	"testing"
)

// TestSwitchCase_ConstRef_Accepted verifies that `case MAX {` where MAX is a
// const int is accepted without error.
func TestSwitchCase_ConstRef_Accepted(t *testing.T) {
	src := wrapMain(`const MAX: int = 3600
let n: int = 1
switch (n) {
  case MAX { print("hour") }
  default {}
}`)
	expectOK(t, src)
}

// TestSwitchCase_FoldedArith_Accepted verifies that `case 60*60 {` (a folded
// arithmetic expression) is accepted without error.
func TestSwitchCase_FoldedArith_Accepted(t *testing.T) {
	src := wrapMain(`let n: int = 1
switch (n) {
  case 60*60 { print("hour") }
  default {}
}`)
	expectOK(t, src)
}

// TestSwitchCase_LetVar_Rejected verifies that a plain let variable used as a
// case value is rejected with a constant-expression error.
func TestSwitchCase_LetVar_Rejected(t *testing.T) {
	src := wrapMain(`let n: int = 1
let m: int = 2
switch (n) {
  case m { print("x") }
  default {}
}`)
	expectErr(t, src, "constant expression")
}

// TestSwitchCase_DuplicateFolded_Rejected verifies that duplicate case values
// detected by comparing folded values produce a "duplicate switch case" error.
// 3600 and 60*60 fold to the same int64 value so must collide.
func TestSwitchCase_DuplicateFolded_Rejected(t *testing.T) {
	src := wrapMain(`let n: int = 1
switch (n) {
  case 3600 { print("a") }
  case 60*60 { print("b") }
  default {}
}`)
	expectErr(t, src, "duplicate switch case")
}

// TestSwitchCase_ConstDupLiteral_Rejected verifies that a const reference and
// a literal with the same folded value also collide.
func TestSwitchCase_ConstDupLiteral_Rejected(t *testing.T) {
	src := wrapMain(`const HOUR: int = 3600
let n: int = 1
switch (n) {
  case HOUR { print("a") }
  case 3600 { print("b") }
  default {}
}`)
	expectErr(t, src, "duplicate switch case")
}

// TestSwitchCase_NonConstFunc_Rejected verifies that a function call used as a
// case value is rejected with a constant-expression error.
func TestSwitchCase_NonConstFunc_Rejected(t *testing.T) {
	src := `fn get() -> int { return 1 }
fn main() -> int {
  let n: int = 1
  switch (n) {
    case get() { print("x") }
    default {}
  }
  return 0
}`
	expectErr(t, src, "constant expression")
}

// TestSwitchCase_ExistingNonLiteral_Negative is the updated version of
// TestRule8_CaseNonLiteral_Negative: using a let variable as a case value is
// rejected; the error now comes from checkConstExpr (not isLiteralExpr).
func TestSwitchCase_ExistingRule8_ConstantExprError(t *testing.T) {
	src := wrapMain(`let n: int = 1
let m: int = 2
switch (n) { case m { print("x") } default {} }`)
	info := check(t, src)
	if len(info.Errors) == 0 {
		t.Fatal("expected an error for non-constant case value, got none")
	}
	// Must mention the variable or constant expression -- not just "must be a literal".
	found := false
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "constant") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected error about constant expression, got: %s", diagList(info.Errors))
	}
}

// TestSwitchConst_NoDuplicateErrorOnTypeMismatch verifies that a case value
// whose type mismatches the subject does not also trigger a spurious
// "duplicate switch case" error on top of the primary type-mismatch error.
func TestSwitchConst_NoDuplicateErrorOnTypeMismatch(t *testing.T) {
	src := `fn main() -> int {
  let n: int = 1
  switch (n) {
    case "x" { return 1 }
    case "x" { return 2 }
    default { return 0 }
  }
}`
	info := check(t, src)
	if len(info.Errors) == 0 {
		t.Fatal("expected type-mismatch errors for string cases on an int subject")
	}
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "duplicate switch case") {
			t.Errorf("type-mismatched case should not also produce a duplicate error: %s", d.Msg)
		}
	}
}
