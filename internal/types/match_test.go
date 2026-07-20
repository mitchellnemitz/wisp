package types

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/parser"
)

// --- B1 TDD gate: three targeted bug regression tests ---

// Bug 1a (AC-10): non-sum scrutinee should emit the scrutinee error AND still
// type-check arm bodies so further errors surface together.
func TestMatchBug1a_NonSumScrutineeBodyChecked(t *testing.T) {
	// Arm body has an independent type error (want string, got int). Before the
	// fix, checkMatch returns early and the body error is never emitted.
	src := wrapMain(`let n: int = 0
match (n) { case Some(x) { let y: string = 1 } }`)
	info := check(t, src)
	hasScrutineeErr := false
	hasBodyErr := false
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "match requires") {
			hasScrutineeErr = true
		}
		if strings.Contains(d.Msg, "want string") {
			hasBodyErr = true
		}
	}
	if !hasScrutineeErr {
		t.Errorf("expected scrutinee error containing \"match requires\", got: %s", diagList(info.Errors))
	}
	if !hasBodyErr {
		t.Errorf("expected body error containing \"want string\" (body must still be checked), got: %s", diagList(info.Errors))
	}
}

// Bug 1b (AC-8): wildcard not last must emit the wildcard error but must NOT
// emit a false duplicate-arm error for the subsequent Some(x) arm. Before the
// fix, remaining is cleared unconditionally, so Some is no longer in remaining
// when the Some(x) arm is processed, triggering a spurious "duplicate" error.
func TestMatchBug1b_WildcardNotLastNoFalseDuplicate(t *testing.T) {
	src := wrapMain(`let o: Optional[int] = Some(1)
match (o) { case _ { } case Some(x) { print(x) } }`)
	info := check(t, src)
	hasWildcardErr := false
	hasDuplicateErr := false
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "wildcard") && strings.Contains(d.Msg, "last") {
			hasWildcardErr = true
		}
		if strings.Contains(d.Msg, "duplicate") {
			hasDuplicateErr = true
		}
	}
	if !hasWildcardErr {
		t.Errorf("expected wildcard-not-last error, got: %s", diagList(info.Errors))
	}
	if hasDuplicateErr {
		t.Errorf("got spurious duplicate-arm error for Some(x) after non-last wildcard; remaining was incorrectly cleared: %s", diagList(info.Errors))
	}
}

// Bug 1c (AC-17): variant with no payload must reject a binding pattern.
// None has no payload, so None(x) must emit an arity error.
func TestMatchBug1c_NoPayloadVariantRejected(t *testing.T) {
	src := wrapMain(`let o: Optional[int] = Some(1)
match (o) { case None(x) { } case Some(v) { } }`)
	expectErr(t, src, "no payload")
}

// --- B2 TDD gate: AC-26 zero-coverage wildcard ---

// AC-26: a trailing wildcard that covers zero variants (exhaustive explicit arms
// already cover everything) must not count as a non-returning path. The function
// must compile without a "missing return" error.
func TestMatchAC26_ZeroCoverageWildcardDoesNotBreakReturn(t *testing.T) {
	src := `fn f(r: Result[int]) -> int {
    match (r) {
        case Ok(v) { return v }
        case Err(e) { return 0 }
        case _ { }
    }
}
fn main() -> int { return 0 }
`
	expectOK(t, src)
}

// --- B8a: positive checker tests ---

func TestMatchOptionalValid(t *testing.T) {
	// AC-1: Some(x)/None full coverage, x bound correctly.
	expectOK(t, wrapMain(`let o: Optional[int] = Some(1)
match (o) { case Some(x) { let y: int = x } case None { } }`))
}

func TestMatchResultValid(t *testing.T) {
	// AC-2: Ok(v)/Err(e) full coverage, types correct.
	expectOK(t, wrapMain(`let r: Result[int] = Ok(1)
match (r) { case Ok(v) { let y: int = v } case Err(e) { let m: string = e.message } }`))
}

func TestMatchWildcardLast(t *testing.T) {
	// AC-3: explicit arm + trailing wildcard.
	expectOK(t, wrapMain(`let o: Optional[int] = Some(1)
match (o) { case Some(x) { } case _ { } }`))
}

func TestMatchDiscard(t *testing.T) {
	// AC-4: Some(_) discard pattern -- no binding, no error.
	expectOK(t, wrapMain(`let o: Optional[int] = Some(1)
match (o) { case Some(_) { } case None { } }`))
}

func TestMatchReturns(t *testing.T) {
	// AC-13: match where all arms return satisfies all-paths-return.
	src := `fn f(o: Optional[int]) -> int {
    match (o) {
        case Some(x) { return x }
        case None { return 0 }
    }
}
fn main() -> int { return 0 }
`
	expectOK(t, src)
}

func TestMatchSingleWildcard(t *testing.T) {
	// AC-16: match with only a wildcard arm compiles.
	expectOK(t, wrapMain(`let o: Optional[int] = Some(1)
match (o) { case _ { } }`))
}

func TestMatchRedundantWildcard(t *testing.T) {
	// AC-25: explicit full coverage + trailing wildcard compiles without error.
	expectOK(t, wrapMain(`let o: Optional[int] = Some(1)
match (o) { case Some(x) { } case None { } case _ { } }`))
}

func TestMatchCrossArmBinding(t *testing.T) {
	// AC-27: same binding name in two separate arms is legal (scoped per arm).
	expectOK(t, wrapMain(`let r: Result[int] = Ok(1)
match (r) { case Ok(x) { let y: int = x } case Err(x) { let m: string = x.message } }`))
}

// --- B8b: negative checker tests ---

func TestMatchMissingArm(t *testing.T) {
	// AC-5: missing variant.
	expectErr(t, wrapMain(`let o: Optional[int] = Some(1)
match (o) { case Some(x) { } }`), "exhaustive")
}

func TestMatchDuplicateArm(t *testing.T) {
	// AC-6: duplicate arm.
	expectErr(t, wrapMain(`let o: Optional[int] = Some(1)
match (o) { case Some(x) { } case Some(y) { } case None { } }`), "duplicate")
}

func TestMatchUnknownVariant(t *testing.T) {
	// AC-7: variant not a constructor of the type.
	expectErr(t, wrapMain(`let o: Optional[int] = Some(1)
match (o) { case Ok(x) { } case None { } }`), "not a constructor")
}

func TestMatchWildcardNotLast(t *testing.T) {
	// AC-8: wildcard must be the last arm (duplicate with bug 1b regression).
	expectErr(t, wrapMain(`let o: Optional[int] = Some(1)
match (o) { case _ { } case Some(x) { } }`), "last")
}

func TestMatchPayloadRequiredSome(t *testing.T) {
	// AC-9: Some without a binding name must be rejected.
	expectErr(t, wrapMain(`let o: Optional[int] = Some(1)
match (o) { case Some { } case None { } }`), "payload")
}

func TestMatchPayloadRequiredOk(t *testing.T) {
	// AC-9: Ok without a binding name must be rejected.
	expectErr(t, wrapMain(`let r: Result[int] = Ok(1)
match (r) { case Ok { } case Err(e) { } }`), "payload")
}

func TestMatchNoPayloadNone(t *testing.T) {
	// AC-17: None(x) - None has no payload.
	expectErr(t, wrapMain(`let o: Optional[int] = Some(1)
match (o) { case None(x) { } case Some(v) { } }`), "no payload")
}

func TestMatchNoPayloadNoneDiscard(t *testing.T) {
	// AC-17: None(_) - None has no payload (discard form also rejected).
	expectErr(t, wrapMain(`let o: Optional[int] = Some(1)
match (o) { case None(_) { } case Some(v) { } }`), "no payload")
}

func TestMatchNonSumScrutinee(t *testing.T) {
	// AC-10: non-sum scrutinee.
	expectErr(t, wrapMain(`let n: int = 0
match (n) { case Some(x) { } }`), "match requires")
}

func TestMatchShadow(t *testing.T) {
	// AC-11: arm binding shadows outer variable.
	expectErr(t, wrapMain(`let x: int = 1
let o: Optional[int] = Some(1)
match (o) { case Some(x) { } case None { } }`), "shadow")
}

func TestMatchKeywordIdent(t *testing.T) {
	// AC-18b: match used as an identifier must be a parse error.
	src := wrapMain("let match: int = 0")
	_, err := parser.Parse(src, "test.wisp")
	if err == nil {
		t.Errorf("expected parse error for `let match: int = 0`, got none")
	}
}

func TestMatchNoParens(t *testing.T) {
	// AC-23: match without parens around scrutinee is a parse error.
	src := wrapMain("let o: Optional[int] = Some(1)\nmatch o { case Some(x) { } case None { } }")
	_, err := parser.Parse(src, "test.wisp")
	if err == nil {
		t.Errorf("expected parse error for match without parens, got none")
	}
}

func TestMatchZeroArms(t *testing.T) {
	// AC-24: empty match must fail exhaustiveness.
	expectErr(t, wrapMain(`let o: Optional[int] = Some(1)
match (o) { }`), "exhaustive")
}

func TestMatchAC22FatArrowNonReg(t *testing.T) {
	// AC-22: compiling a function that uses both -> (return type) and = (assignment)
	// alongside match must produce no errors.
	src := `fn classify(r: Result[int]) -> string {
    let label: string = "unknown"
    match (r) {
        case Ok(v) { label = "ok" }
        case Err(e) { label = "err" }
    }
    return label
}
fn main() -> int { return 0 }
`
	expectOK(t, src)
}
