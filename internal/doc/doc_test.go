package doc

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/lexer"
	"github.com/mitchellnemitz/wisp/internal/parser"
)

// mustParse2 parses src and returns (prog, comments), fataling on error.
func mustParse2(t *testing.T, src string) (*ast.Program, []lexer.Comment) {
	t.Helper()
	prog, comments, err := parser.ParseWithComments(src, "t.wisp")
	if err != nil {
		t.Fatalf("ParseWithComments: %v", err)
	}
	return prog, comments
}

// mustParse parses src and returns (prog, comments).
func mustParse(t *testing.T, src string) (*ast.Program, []lexer.Comment) {
	t.Helper()
	return mustParse2(t, src)
}

// checkDoc parses src, runs Extract, finds the entry named name, and asserts
// its Doc field equals want.
func checkDoc(t *testing.T, src, name, want string) {
	t.Helper()
	prog, comments := mustParse2(t, src)
	entries := Extract(prog, comments)
	for _, e := range entries {
		if e.Name == name {
			if e.Doc != want {
				t.Errorf("doc for %q:\n  got  %q\n  want %q", name, e.Doc, want)
			}
			return
		}
	}
	t.Errorf("no entry named %q; got entries: %v", name, entryNames(entries))
}

func entryNames(entries []DocEntry) []string {
	var ns []string
	for _, e := range entries {
		ns = append(ns, e.Name)
	}
	return ns
}

// assertContains checks that s contains sub.
func assertContains(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("output does not contain %q\nfull output:\n%s", sub, s)
	}
}

func TestIsDocComment(t *testing.T) {
	cases := map[string]bool{
		"/// x":  true,
		"///":    true,
		"//x":    false,
		"// x":   false,
		"//// x": false,
		"//":     false,
	}
	for text, want := range cases {
		if got := isDocComment(text); got != want {
			t.Errorf("isDocComment(%q)=%v want %v", text, got, want)
		}
	}
}

func TestAttachment(t *testing.T) {
	// consecutive /// attaches, joined
	checkDoc(t, "/// line one\n/// line two\nfn f() -> int { return 0 }", "f", "line one\nline two")
	// blank line detaches
	checkDoc(t, "/// doc\n\nfn g() -> int { return 0 }", "g", "")
	// regular // between detaches
	checkDoc(t, "/// doc\n// note\nfn h() -> int { return 0 }", "h", "")
	// trailing /// on preceding line (inline with valid top-level code) does not attach
	checkDoc(t, "const x: int = 1 /// trailing\nfn i() -> int { return 0 }", "i", "")
	// /// inside a body does not attach to anything (the surrounding fn has no doc)
	checkDoc(t, "fn j() -> int {\n  /// in body\n  return 0\n}", "j", "")
	// /// trailing on the DECL line itself does not attach
	checkDoc(t, "fn dl() -> int { return 0 } /// trailing on decl line", "dl", "")
	// struct/enum/const attach
	checkDoc(t, "/// a point\nstruct P { x: int, y: int }", "P", "a point")
	checkDoc(t, "/// a color\nenum C { Red, Green }", "C", "a color")
	checkDoc(t, "/// the max\nconst MAX: int = 9", "MAX", "the max")
	// `export fn` (one line -- the only valid export form) attaches the /// above
	checkDoc(t, "/// exported\nexport fn e() -> void { }", "e", "exported")
}

func TestRenderSignatures(t *testing.T) {
	r := func(src string) string {
		prog, comments := mustParse2(t, src)
		return Render("t.wisp", prog, comments)
	}
	assertContains(t, r("export fn add(a: int, b: int) -> int { return a + b }"), "export fn add(a: int, b: int) -> int")
	assertContains(t, r("fn id[T: comparable](x: T) -> T { return x }"), "fn id[T: comparable](x: T) -> T")
	assertContains(t, r("fn nada() -> void { }"), "fn nada() -> void")
	assertContains(t, r("export struct P { x: int }"), "export struct P { x: int }")
	assertContains(t, r("export const MAX: int = 9"), "export const MAX: int")
	assertContains(t, r("enum C { Red, Green, Blue }"), "enum C { Red, Green, Blue }")
	// param default is OMITTED from the signature
	assertContains(t, r("fn withdef(a: int = 5) -> int { return a }"), "fn withdef(a: int) -> int")
}

func TestRenderCanonical(t *testing.T) {
	src := "/// Returns n doubled.\nfn foo(n: int) -> int { return n + n }\nfn bar() -> void { }"
	prog, comments := mustParse(t, src)
	got := Render("a.wisp", prog, comments)
	want := "## a.wisp\n\n### foo\n\n```\nfn foo(n: int) -> int\n```\n\nReturns n doubled.\n\n### bar\n\n```\nfn bar() -> void\n```\n"
	if got != want {
		t.Errorf("Render mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}
