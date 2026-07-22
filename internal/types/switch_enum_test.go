package types

// Task 3: exhaustive switch over enum subjects.
//
// An enum-subject switch accepts the enum as the switch subject, lifts the
// mandatory-default requirement when every variant is covered, and rejects raw
// integer cases or another enum's variants. Non-enum switches keep the
// mandatory-default rule unchanged.

import "testing"

// TestEnumSwitch_ExhaustiveNoDefault_OK: covering every variant without a
// default compiles (the mandatory-default rule is lifted for enum subjects).
func TestEnumSwitch_ExhaustiveNoDefault_OK(t *testing.T) {
	expectOK(t, `enum Color: int { Red, Green, Blue }
fn main() -> int {
  let c: Color = Color.Green
  switch (c) {
    case Color.Red { print("r") }
    case Color.Green { print("g") }
    case Color.Blue { print("b") }
  }
  return 0
}`)
}

// TestEnumSwitch_MissingVariantNoDefault_Err: a defaultless enum switch that
// omits a variant is not exhaustive.
func TestEnumSwitch_MissingVariantNoDefault_Err(t *testing.T) {
	expectErr(t, `enum Color: int { Red, Green, Blue }
fn main() -> int {
  let c: Color = Color.Green
  switch (c) {
    case Color.Red { print("r") }
    case Color.Green { print("g") }
  }
  return 0
}`, "switch is not exhaustive: missing Blue")
}

// TestEnumSwitch_PartialWithDefault_OK: a default arm allows partial coverage.
func TestEnumSwitch_PartialWithDefault_OK(t *testing.T) {
	expectOK(t, `enum Color: int { Red, Green, Blue }
fn main() -> int {
  let c: Color = Color.Green
  switch (c) {
    case Color.Red { print("r") }
    default { print("other") }
  }
  return 0
}`)
}

// TestEnumSwitch_RawIntCase_Err: a raw-int case in an enum switch is rejected.
func TestEnumSwitch_RawIntCase_Err(t *testing.T) {
	expectErr(t, `enum Color: int { Red, Green, Blue }
fn main() -> int {
  let c: Color = Color.Green
  switch (c) {
    case 0 { print("zero") }
    default {}
  }
  return 0
}`, "must be a variant of enum")
}

// TestEnumSwitch_WrongEnumCase_Err: a different enum's variant is rejected.
func TestEnumSwitch_WrongEnumCase_Err(t *testing.T) {
	expectErr(t, `enum Color: int { Red, Green, Blue }
enum Size: int { Small, Large }
fn main() -> int {
  let c: Color = Color.Green
  switch (c) {
    case Color.Red { print("r") }
    case Size.Small { print("s") }
    default {}
  }
  return 0
}`, "must be a variant of enum")
}

// TestEnumSwitch_DuplicateCase_Err: a duplicate variant case uses the existing
// dedup error.
func TestEnumSwitch_DuplicateCase_Err(t *testing.T) {
	expectErr(t, `enum Color: int { Red, Green, Blue }
fn main() -> int {
  let c: Color = Color.Green
  switch (c) {
    case Color.Red { print("r") }
    case Color.Red { print("r2") }
    case Color.Green { print("g") }
    case Color.Blue { print("b") }
  }
  return 0
}`, "duplicate switch case")
}

// TestNonEnumSwitch_StillRequiresDefault: a non-enum int switch with no default
// is still rejected (the lift is enum-only).
func TestNonEnumSwitch_StillRequiresDefault(t *testing.T) {
	expectErr(t, wrapMain(`let n: int = 1
switch (n) {
  case 1 { print("a") }
}`), "switch must have a default clause")
}
