package types

import (
	"strings"
	"testing"
)

// helper: assert an error containing `want` exists AND it carries the given
// did-you-mean suffix.
func expectSuggestion(t *testing.T, src, errSubstr, suggest string) {
	t.Helper()
	d := expectErr(t, src, errSubstr)
	want := `; did you mean "` + suggest + `"?`
	if !strings.Contains(d.Msg, want) {
		t.Fatalf("expected suggestion %q in %q", want, d.Msg)
	}
}

// helper: assert an error containing `want` exists and carries NO suggestion.
func expectNoSuggestion(t *testing.T, src, errSubstr string) {
	t.Helper()
	d := expectErr(t, src, errSubstr)
	if strings.Contains(d.Msg, "did you mean") {
		t.Fatalf("expected NO suggestion, got %q", d.Msg)
	}
}

func TestDidYouMeanBuiltin(t *testing.T) {
	// "lenght" -> "length" (distance 2, unique closest builtin)
	expectSuggestion(t, wrapMain(`let n: int = lenght("hi")`),
		"undeclared function", "length")
}

func TestDidYouMeanUserFunction(t *testing.T) {
	src := `fn helper() -> int {
  return 1
}
fn main() -> int {
  let n: int = helpr()
  return n
}`
	expectSuggestion(t, src, "undeclared function", "helper")
}

func TestDidYouMeanType(t *testing.T) {
	// "strng" -> "string" (distance 1)
	expectSuggestion(t, wrapMain(`let s: strng = "hi"`),
		"unknown type", "string")
}

func TestDidYouMeanStructType(t *testing.T) {
	src := `struct Point { x: int, y: int }
fn main() -> int {
  let p: Point = Poimt { x: 1, y: 2 }
  return p.x
}`
	expectSuggestion(t, src, "unknown struct type", "Point")
}

func TestDidYouMeanVariable(t *testing.T) {
	expectSuggestion(t, wrapMain(`let count: int = 1
let n: int = cont`),
		"undeclared name", "count")
}

func TestDidYouMeanAssignVariable(t *testing.T) {
	expectSuggestion(t, wrapMain(`let total: int = 0
totl = 5`),
		"assignment to undeclared variable", "total")
}

func TestDidYouMeanNoCloseMatch(t *testing.T) {
	// "zzzzz" is far from any builtin/function (> 2 edits).
	expectNoSuggestion(t, wrapMain(`let n: int = zzzzz("hi")`),
		"undeclared function")
}

func TestDidYouMeanTieYieldsPlain(t *testing.T) {
	// Two user functions equidistant (distance 1) from the typo => no suggestion.
	// "fo" is distance 1 from both "foo" and "fob".
	src := `fn foo() -> int {
  return 1
}
fn fob() -> int {
  return 2
}
fn main() -> int {
  let n: int = fo()
  return n
}`
	expectNoSuggestion(t, src, "undeclared function")
}

// TestDidYouMeanDoesNotChangeAcceptance verifies the suggestion is purely
// additive: a program that compiled before still compiles, and a program that
// failed before still fails (the suggestion text changes only the message).
func TestDidYouMeanDoesNotChangeAcceptance(t *testing.T) {
	// Valid program: a near-miss is NOT present, so it must still type-check.
	expectOK(t, `fn length_helper() -> int {
  return 1
}
fn main() -> int {
  let n: int = length_helper()
  return n
}`)
	// Invalid program with a typo: still rejected (one error), suggestion or not.
	info := check(t, wrapMain(`let n: int = lenght("hi")`))
	if len(info.Errors) == 0 {
		t.Fatal("a misspelled builtin must still be rejected")
	}
}
