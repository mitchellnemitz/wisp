package types

import (
	"strings"
	"testing"
)

// countErrContains returns the number of info.Errors whose Msg contains want.
// expectErr/hasErr are substring-only and stop at the first match, so they
// cannot see a duplicate; this counts every occurrence.
func countErrContains(info *Info, want string) int {
	n := 0
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, want) {
			n++
		}
	}
	return n
}

// TestDedup_ToInt_ArgErrorReportedOnce pins that to_int's probe-then-fallthrough
// argument check reports the undeclared-name error exactly once, not twice.
func TestDedup_ToInt_ArgErrorReportedOnce(t *testing.T) {
	info := check(t, wrapMain(`let n: int = to_int(nope)`))
	if got := countErrContains(info, "undeclared name"); got != 1 {
		t.Fatalf("to_int(nope): got %d \"undeclared name\" diagnostics, want 1:\n%s", got, diagList(info.Errors))
	}
}

// TestDedup_ToString_ArgErrorReportedOnce mirrors the to_int case for to_string.
func TestDedup_ToString_ArgErrorReportedOnce(t *testing.T) {
	info := check(t, wrapMain(`let s: string = to_string(nope)`))
	if got := countErrContains(info, "undeclared name"); got != 1 {
		t.Fatalf("to_string(nope): got %d \"undeclared name\" diagnostics, want 1:\n%s", got, diagList(info.Errors))
	}
}

// TestDedup_Length_ArgErrorReportedOnce mirrors the to_int case for length.
func TestDedup_Length_ArgErrorReportedOnce(t *testing.T) {
	info := check(t, wrapMain(`let n: int = length(nope)`))
	if got := countErrContains(info, "undeclared name"); got != 1 {
		t.Fatalf("length(nope): got %d \"undeclared name\" diagnostics, want 1:\n%s", got, diagList(info.Errors))
	}
}

// TestDedup_NegativeControl_DifferentMessages guards against a Msg-agnostic
// key: two distinct undeclared names on one line produce two DIFFERENT
// messages ("undeclared name \"nope\"" vs "undeclared name \"nope2\""), so
// both must still be reported (count 2 by the shared substring).
func TestDedup_NegativeControl_DifferentMessages(t *testing.T) {
	info := check(t, wrapMain(`let m: int = nope + nope2`))
	if got := countErrContains(info, "undeclared name"); got != 2 {
		t.Fatalf("nope + nope2: got %d \"undeclared name\" diagnostics, want 2 (distinct names):\n%s", got, diagList(info.Errors))
	}
}

// TestDedup_NegativeControl_SameMessageDifferentColumns guards against a
// column-agnostic key (e.g. one keyed on line+msg only): two references to
// the same undeclared name on one line produce the identical message
// "undeclared name \"nope\"" at two different columns, and both must still be
// reported (verified pre-fix at HEAD: the checker does not dedup repeated
// undeclared names by name, only errf's new (Pos,Msg) key would).
func TestDedup_NegativeControl_SameMessageDifferentColumns(t *testing.T) {
	info := check(t, wrapMain(`let m: int = nope + nope`))
	if got := countErrContains(info, `undeclared name "nope"`); got != 2 {
		t.Fatalf("nope + nope: got %d \"undeclared name\" diagnostics, want 2 (same name, different columns):\n%s", got, diagList(info.Errors))
	}
}
