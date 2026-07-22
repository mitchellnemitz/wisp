package types

import "testing"

func TestEnumSwitchReturns_ExhaustiveNoDefault_OK(t *testing.T) {
	// success criterion 1: mirrors TestMatchReturns's shape
	// (match_test.go:116), substituting an exhaustive defaultless enum
	// switch for match. No trailing return after the switch.
	src := `enum Color: int { Red, Green, Blue }
fn f(c: Color) -> int {
    switch (c) {
        case Color.Red { return 1 }
        case Color.Green { return 2 }
        case Color.Blue { return 3 }
    }
}
fn main() -> int { return 0 }
`
	expectOK(t, src)
}

func TestEnumSwitchReturns_NonReturningCase_Err(t *testing.T) {
	// success criterion 3: coverage is complete (checkSwitch accepts this
	// as a legal statement -- it only checks variant coverage, not
	// per-case returns), but Color.Red's case does not return, so the
	// enclosing function must still fail all-paths-return.
	src := `enum Color: int { Red, Green, Blue }
fn f(c: Color) -> int {
    switch (c) {
        case Color.Red { print("r") }
        case Color.Green { return 2 }
        case Color.Blue { return 3 }
    }
}
fn main() -> int { return 0 }
`
	expectErr(t, src, "must return int on every path")
}

func TestEnumSwitchReturns_WithDefaultAllReturn_OK(t *testing.T) {
	// success criterion 4: regression guard on the pre-existing,
	// already-correct n.Default != nil branch (returns.go:48-59 today),
	// now the `if n.Default != nil` arm post-fix.
	src := `enum Color: int { Red, Green, Blue }
fn f(c: Color) -> int {
    switch (c) {
        case Color.Red { return 1 }
        default { return 0 }
    }
}
fn main() -> int { return 0 }
`
	expectOK(t, src)
}

func TestNonEnumSwitchReturns_MissingDefault_StillRequiresDefault_Err(t *testing.T) {
	// success criterion 5: a non-enum defaultless switch is unaffected
	// by the enum-only exhaustiveness lift -- it still fails at
	// checkSwitch's mandatory-default rule (stmt.go:865), same as
	// TestNonEnumSwitch_StillRequiresDefault, confirmed here in the
	// all-paths-return context (ending a non-void function).
	src := `fn f(n: int) -> int {
switch (n) {
    case 1 { return 1 }
}
}
fn main() -> int { return 0 }
`
	expectErr(t, src, "switch must have a default clause")
}

func TestNonEnumSwitchReturns_WithDefaultAllReturn_OK(t *testing.T) {
	// success criterion 5: non-enum switch's already-correct terminator
	// behavior (default present, all cases + default return) is
	// unaffected by the enum-only branch added to stmtReturns.
	src := `fn f(n: int) -> int {
switch (n) {
    case 1 { return 1 }
    default { return 0 }
}
}
fn main() -> int { return 0 }
`
	expectOK(t, src)
}
