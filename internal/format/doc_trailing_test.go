package format

import "testing"

// TestDocCommentNotLeakedPastTrailingComment (B2 follow-up): a trailing comment
// on the prior declaration's last line must not defeat declBoundary. A `///`
// doc comment that leads the NEXT declaration must still stay attached to that
// declaration, not be swept into the prior declaration's body. The original
// declBoundary forward scan bailed on the first pending trailing comment, so any
// trailing comment in the gap re-exposed the exact doc-comment-leak bug B2 fixed.
func TestDocCommentNotLeakedPastTrailingComment(t *testing.T) {
	src := "export fn a() -> int {\n" +
		"    return 0 // trailing here\n" +
		"}\n" +
		"\n" +
		"/// doc for b\n" +
		"export fn b() -> int {\n" +
		"    return 1\n" +
		"}\n"
	got := mustFormat(t, src)
	if got != src {
		t.Fatalf("doc comment leaked past a trailing comment:\n--got--\n%s\n--want--\n%s", got, src)
	}
	if twice := mustFormat(t, got); twice != got {
		t.Fatalf("not idempotent:\n--once--\n%s\n--twice--\n%s", got, twice)
	}
}

// TestDanglingTailStaysInBodyWithAdjacentDecl (regression guard for the named
// case 2c): advancing the forward scan past pending comments must NOT pull a
// genuine full-line dangling tail comment -- the last line of the prior decl's
// body, with an adjacent next decl and no blank line -- out of that body. It
// stays inside the prior block; a blank line is inserted before the next decl.
func TestDanglingTailStaysInBodyWithAdjacentDecl(t *testing.T) {
	src := "fn a() -> int {\n" +
		"    return 0\n" +
		"    // dangling tail inside a\n" +
		"}\n" +
		"fn b() -> int {\n" +
		"    return 1\n" +
		"}\n"
	want := "fn a() -> int {\n" +
		"    return 0\n" +
		"    // dangling tail inside a\n" +
		"}\n" +
		"\n" +
		"fn b() -> int {\n" +
		"    return 1\n" +
		"}\n"
	got := mustFormat(t, src)
	if got != want {
		t.Fatalf("dangling tail not kept inside prior body:\n--got--\n%s\n--want--\n%s", got, want)
	}
	if twice := mustFormat(t, got); twice != got {
		t.Fatalf("not idempotent:\n--once--\n%s\n--twice--\n%s", got, twice)
	}
}

// TestTrailingOnDeclLineMinusOnePlusDoc (regression guard for the line the
// backward `!Trailing` guard protects): a trailing comment on the prior decl's
// last line, immediately followed by a `///` doc on the next decl, must keep the
// trailing comment on its own line and the doc with the next decl.
func TestTrailingOnDeclLineMinusOnePlusDoc(t *testing.T) {
	src := "const X: int = 5 // trailing on X\n" +
		"\n" +
		"/// doc for b\n" +
		"fn b() -> int {\n" +
		"    return 1\n" +
		"}\n"
	got := mustFormat(t, src)
	if got != src {
		t.Fatalf("trailing+doc at decl boundary not preserved:\n--got--\n%s\n--want--\n%s", got, src)
	}
	if twice := mustFormat(t, got); twice != got {
		t.Fatalf("not idempotent:\n--once--\n%s\n--twice--\n%s", got, twice)
	}
}

// TestStackedDocNotLeakedPastMidBodyTrailing (B2 follow-up, reviewer input B): a
// trailing comment mid-body plus a stacked `///` doc run above the next decl --
// both doc lines must stay with the next decl.
func TestStackedDocNotLeakedPastMidBodyTrailing(t *testing.T) {
	src := "fn a() -> int {\n" +
		"    let z: int = 5 // note z\n" +
		"    return z\n" +
		"}\n" +
		"\n" +
		"/// doc one\n" +
		"/// doc two\n" +
		"fn b() -> int {\n" +
		"    return 1\n" +
		"}\n"
	got := mustFormat(t, src)
	if got != src {
		t.Fatalf("stacked doc leaked past a mid-body trailing comment:\n--got--\n%s\n--want--\n%s", got, src)
	}
	if twice := mustFormat(t, got); twice != got {
		t.Fatalf("not idempotent:\n--once--\n%s\n--twice--\n%s", got, twice)
	}
}
