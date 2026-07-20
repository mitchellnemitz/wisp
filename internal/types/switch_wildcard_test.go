package types

import (
	"strings"
	"testing"
)

func TestSwitchCase_BlankIdent_Err(t *testing.T) {
	// success criterion 1: switch's bare `_` case value (match's wildcard
	// spelling, not switch's) must emit the targeted message AND must NOT
	// still emit the old "a constant expression may not reference a
	// variable" text (the fix `continue`s before checkConstExpr, so the
	// generic const-expr error can no longer fire for this case value).
	info := check(t, wrapMain(`let n: int = 1
switch (n) {
case 1 { print("one") }
case _ { print("other") }
}`))
	foundNew := false
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, `switch has no "_" wildcard; use "default"`) {
			foundNew = true
		}
		if strings.Contains(d.Msg, "may not reference a variable") {
			t.Errorf("old const-expr message still present: %s", diagList(info.Errors))
		}
	}
	if !foundNew {
		t.Errorf("missing targeted `_`-wildcard message, got:\n%s", diagList(info.Errors))
	}
}

func TestSwitchCase_BlankIdent_StacksWithMissingDefault(t *testing.T) {
	// success criterion 2 (STACK, resolved in requirements): for a switch
	// with a bare `_` case value and no default, BOTH the corrected
	// wildcard message AND the existing "switch must have a default
	// clause" message must be present in the SAME diagnostic set -- the
	// second is not suppressed by the first.
	src := wrapMain(`let n: int = 1
switch (n) {
case 1 { print("one") }
case _ { print("other") }
}`)
	info := check(t, src)
	want := []string{
		`switch has no "_" wildcard; use "default"`,
		"switch must have a default clause",
	}
	for _, w := range want {
		found := false
		for _, d := range info.Errors {
			if strings.Contains(d.Msg, w) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected an error containing %q, got:\n%s", w, diagList(info.Errors))
		}
	}
}

func TestSwitchCase_RealVariable_Err(t *testing.T) {
	// success criterion 4 (regression guard): a switch case using an
	// ordinary identifier that is genuinely an undeclared/non-constant
	// variable (not `_`) must still emit the existing generic message,
	// unchanged -- proving the new guard targets bare `_` specifically,
	// not all identifier-valued case values.
	src := wrapMain(`let n: int = 1
switch (n) {
case 1 { print("one") }
case x { print("other") }
}`)
	expectErr(t, src, `a constant expression may not reference a variable ("x")`)
}

func TestSwitchDefault_OK(t *testing.T) {
	// success criterion 5: a switch with a correct `default { }` clause
	// (no case-value `_`) continues to compile with no new diagnostics.
	// `n` is declared so expectOK's zero-error assertion holds.
	src := wrapMain(`let n: int = 1
switch (n) {
case 1 { print("one") }
default { print("other") }
}`)
	expectOK(t, src)
}

func TestMatchWildcard_OK(t *testing.T) {
	// success criterion 5: a match with a correct trailing `case _ { }`
	// wildcard arm continues to compile (parse AND type-check) with no new
	// diagnostics. Uses expectOK (full check), not parseOK, so the claim
	// "compiles with no new diagnostics" is actually verified. Idiom mirrors
	// match_test.go:131-132 (a type-valid lone-wildcard match over Optional).
	expectOK(t, wrapMain(`let o: Optional[int] = Some(1)
match (o) { case Some(x) { print(to_string(x)) } case _ { print("none") } }`))
}
