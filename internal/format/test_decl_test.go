package format

import (
	"strings"
	"testing"
)

// fmtTestFile formats src under a *_test.wisp filename, since `test` decls are
// only allowed there. Fails the test on a format error.
func fmtTestFile(t *testing.T, src string) string {
	t.Helper()
	out, err := Format(src, "x_test.wisp")
	if err != nil {
		t.Fatalf("Format(%q): %v", src, err)
	}
	return out
}

// TestTestDeclPreserved is a regression test for the formatter silently dropping
// top-level `test (...)` declarations. program() collected funcs/structs/enums/
// consts/imports/includes but never prog.Tests, so every `test` block and its
// body vanished on format, leaving only the include line and orphaned doc
// comments (a *_test.wisp file would shrink to a few lines). Catastrophic data
// loss from a tool whose contract is to rewrite source in place.
func TestTestDeclPreserved(t *testing.T) {
	src := "include \"./impl.wisp\" as impl\n" +
		"\n" +
		"test (\"greet works\") {\n" +
		"    assert_eq(impl.greet(\"world\"), \"hi world\")\n" +
		"}\n" +
		"\n" +
		"test (\"shout works\") {\n" +
		"    let xs: string[] = [\"a\", \"b\"]\n" +
		"    assert_eq(length(xs), 2)\n" +
		"}\n"
	got := fmtTestFile(t, src)
	if got != src {
		t.Fatalf("test decls not preserved (already-canonical input must round-trip):\n--got--\n%s\n--want--\n%s", got, src)
	}
	// Guard the specific data-loss symptom explicitly.
	for _, must := range []string{
		`test ("greet works") {`,
		`assert_eq(impl.greet("world"), "hi world")`,
		`test ("shout works") {`,
		`assert_eq(length(xs), 2)`,
	} {
		if !strings.Contains(got, must) {
			t.Errorf("formatted output dropped %q:\n%s", must, got)
		}
	}
	if twice := fmtTestFile(t, got); twice != got {
		t.Fatalf("not idempotent:\n--once--\n%s\n--twice--\n%s", got, twice)
	}
}

// TestTestDeclDocCommentPreserved: a `///` doc comment leading a test decl must
// stay attached to that test, not leak into a prior decl's body (composes with
// the declBoundary fix) and must not be dropped along with the test.
func TestTestDeclDocCommentPreserved(t *testing.T) {
	src := "/// checks greeting\n" +
		"test (\"greet works\") {\n" +
		"    assert_eq(1, 1)\n" +
		"}\n"
	got := fmtTestFile(t, src)
	if got != src {
		t.Fatalf("doc-commented test decl not preserved:\n--got--\n%s\n--want--\n%s", got, src)
	}
}
