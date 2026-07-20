package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// docFixture is a .wisp file with documented fn/struct/enum/const + one undocumented fn.
const docFixtureSrc = `/// Returns n doubled.
fn foo(n: int) -> int { return n + n }
/// A point in 2D space.
export struct Point { x: int, y: int }
/// Direction enum.
enum Dir { North, South }
/// Max allowed value.
export const MAX: int = 100
fn bar() -> void { }
`

// docFixtureWant is the exact canonical Markdown output for docFixtureSrc when
// the path header is the canonical path (a simple filename).
const docFixtureWant = "## doc_fixture.wisp\n\n" +
	"### foo\n\n```\nfn foo(n: int) -> int\n```\n\nReturns n doubled.\n\n" +
	"### Point\n\n```\nexport struct Point { x: int, y: int }\n```\n\nA point in 2D space.\n\n" +
	"### Dir\n\n```\nenum Dir { North, South }\n```\n\nDirection enum.\n\n" +
	"### MAX\n\n```\nexport const MAX: int\n```\n\nMax allowed value.\n\n" +
	"### bar\n\n```\nfn bar() -> void\n```\n"

// TestDocAC2ExactStdout: cmdDoc with one file -> exact canonical Markdown, exit 0.
func TestDocAC2ExactStdout(t *testing.T) {
	src := writeTmp(t, "doc_fixture.wisp", docFixtureSrc)
	var so, se bytes.Buffer
	code := cmdDoc([]string{src}, &so, &se)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, se.String())
	}
	// Derive the expected header from the canonical path of src.
	clean := filepath.Clean(src)
	want := strings.ReplaceAll(docFixtureWant, "## doc_fixture.wisp\n", "## "+clean+"\n")
	got := so.String()
	if got != want {
		t.Errorf("stdout mismatch\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestDocAC3NoArgs: no arguments -> exit 2, usage on stderr, empty stdout.
func TestDocAC3NoArgs(t *testing.T) {
	var so, se bytes.Buffer
	code := cmdDoc(nil, &so, &se)
	if code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
	if so.Len() != 0 {
		t.Fatalf("stdout not empty: %q", so.String())
	}
	if !strings.Contains(se.String(), "usage") {
		t.Errorf("expected usage on stderr, got %q", se.String())
	}
}

// TestDocAC3ParseError: a parse-error file -> exit 1, located stderr, empty stdout.
func TestDocAC3ParseError(t *testing.T) {
	src := writeTmp(t, "bad.wisp", "fn main( -> int { return 0 }\n")
	var so, se bytes.Buffer
	code := cmdDoc([]string{src}, &so, &se)
	if code != 1 {
		t.Fatalf("exit=%d want 1", code)
	}
	if so.Len() != 0 {
		t.Fatalf("stdout not empty on parse error: %q", so.String())
	}
	if !strings.Contains(se.String(), "bad.wisp") {
		t.Errorf("stderr not located: %q", se.String())
	}
}

// TestDocAC3GoodAndBad: good file + bad file -> exit 1, EMPTY stdout (fail-fast buffered).
func TestDocAC3GoodAndBad(t *testing.T) {
	good := writeTmp(t, "good.wisp", docFixtureSrc)
	bad := writeTmp(t, "bad.wisp", "fn oops( -> void { }\n")
	var so, se bytes.Buffer
	code := cmdDoc([]string{good, bad}, &so, &se)
	if code != 1 {
		t.Fatalf("exit=%d want 1", code)
	}
	// fail-fast buffered: nothing written to stdout even though good was processed first
	if so.Len() != 0 {
		t.Fatalf("stdout not empty on partial failure: %q", so.String())
	}
}

// TestDocAC3Nonexistent: nonexistent path -> exit 1, stderr, empty stdout.
func TestDocAC3Nonexistent(t *testing.T) {
	var so, se bytes.Buffer
	code := cmdDoc([]string{"/no/such/file.wisp"}, &so, &se)
	if code != 1 {
		t.Fatalf("exit=%d want 1", code)
	}
	if so.Len() != 0 {
		t.Fatalf("stdout not empty: %q", so.String())
	}
	if se.Len() == 0 {
		t.Fatal("expected error on stderr")
	}
}

// TestDocAC3NonWispFile: a regular file not ending .wisp -> exit 1.
func TestDocAC3NonWispFile(t *testing.T) {
	src := writeTmp(t, "script.sh", "#!/bin/sh\necho hi\n")
	var so, se bytes.Buffer
	code := cmdDoc([]string{src}, &so, &se)
	if code != 1 {
		t.Fatalf("exit=%d want 1", code)
	}
	if so.Len() != 0 {
		t.Fatalf("stdout not empty: %q", so.String())
	}
}

// TestDocAC3Unreadable: an unreadable file -> exit 1.
func TestDocAC3Unreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root (e.g. CI golden container): chmod 0000 does not block root reads, so an unreadable file cannot be simulated")
	}
	src := writeTmp(t, "unreadable.wisp", "fn f() -> void { }\n")
	if err := os.Chmod(src, 0o000); err != nil {
		t.Skip("cannot chmod, skipping unreadable test")
	}
	t.Cleanup(func() { os.Chmod(src, 0o644) })
	var so, se bytes.Buffer
	code := cmdDoc([]string{src}, &so, &se)
	if code != 1 {
		t.Fatalf("exit=%d want 1", code)
	}
	if so.Len() != 0 {
		t.Fatalf("stdout not empty: %q", so.String())
	}
}

// TestDocAC3TwoFiles: two good files -> exit 0, both sections in argument order with blank line between.
func TestDocAC3TwoFiles(t *testing.T) {
	const srcA = "/// First function.\nfn alpha() -> void { }\n"
	const srcB = "/// Second function.\nfn beta() -> void { }\n"
	a := writeTmp(t, "a.wisp", srcA)
	b := writeTmp(t, "b.wisp", srcB)
	cleanA := filepath.Clean(a)
	cleanB := filepath.Clean(b)
	var so, se bytes.Buffer
	code := cmdDoc([]string{a, b}, &so, &se)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, se.String())
	}
	got := so.String()
	// First section starts at the top.
	wantA := "## " + cleanA + "\n\n### alpha\n\n```\nfn alpha() -> void\n```\n\nFirst function.\n"
	wantB := "## " + cleanB + "\n\n### beta\n\n```\nfn beta() -> void\n```\n\nSecond function.\n"
	want := wantA + "\n" + wantB
	if got != want {
		t.Errorf("two-file stdout mismatch\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestDocAC3DirSorted: dir arg -> sorted .wisp files, proper headers.
func TestDocAC3DirSorted(t *testing.T) {
	dir := t.TempDir()
	// write z.wisp first, a.wisp second -- output must be sorted
	if err := os.WriteFile(filepath.Join(dir, "z.wisp"), []byte("fn zz() -> void { }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.wisp"), []byte("fn aa() -> void { }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var so, se bytes.Buffer
	code := cmdDoc([]string{dir}, &so, &se)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, se.String())
	}
	got := so.String()
	// a.wisp section must appear before z.wisp section
	idxA := strings.Index(got, "## "+filepath.Join(dir, "a.wisp"))
	idxZ := strings.Index(got, "## "+filepath.Join(dir, "z.wisp"))
	if idxA < 0 || idxZ < 0 {
		t.Fatalf("missing section headers in output:\n%q", got)
	}
	if idxA >= idxZ {
		t.Errorf("a.wisp section (%d) should come before z.wisp section (%d)", idxA, idxZ)
	}
}

// TestDocAC3EmptyDir: empty dir -> exit 0, empty stdout, no error.
func TestDocAC3EmptyDir(t *testing.T) {
	dir := t.TempDir()
	var so, se bytes.Buffer
	code := cmdDoc([]string{dir}, &so, &se)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, se.String())
	}
	if so.Len() != 0 {
		t.Fatalf("expected empty stdout for empty dir, got %q", so.String())
	}
	if se.Len() != 0 {
		t.Fatalf("expected empty stderr for empty dir, got %q", se.String())
	}
}

// TestDocAC3DirVariants: ./dir/, dir/, dir/. all produce identical headers.
func TestDocAC3DirVariants(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f.wisp"), []byte("fn f() -> void { }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(arg string) string {
		var so, se bytes.Buffer
		if code := cmdDoc([]string{arg}, &so, &se); code != 0 {
			t.Fatalf("arg=%q exit=%d stderr=%q", arg, code, se.String())
		}
		return so.String()
	}
	// All three dir forms: dir, dir/, dir/.
	// filepath.Join normalizes them all the same way.
	got1 := run(dir)
	got2 := run(dir + "/")
	got3 := run(dir + "/.")
	if got1 != got2 || got1 != got3 {
		t.Errorf("dir variant outputs differ:\n  dir: %q\n  dir/: %q\n  dir/.: %q", got1, got2, got3)
	}
}

// TestDocAC3DotSlashFile: ./a.wisp -> header "## a.wisp" (filepath.Clean strips ./).
func TestDocAC3DotSlashFile(t *testing.T) {
	// We need a file in the current working directory with a ./ prefix.
	// Use TempDir and manually prefix.
	src := writeTmp(t, "a.wisp", "fn g() -> void { }\n")
	dir := filepath.Dir(src)
	dotSlashPath := filepath.Join(dir, ".", "a.wisp") // dir/./a.wisp -> filepath.Clean -> dir/a.wisp
	clean := filepath.Clean(dotSlashPath)
	var so, se bytes.Buffer
	code := cmdDoc([]string{dotSlashPath}, &so, &se)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, se.String())
	}
	got := so.String()
	if !strings.Contains(got, "## "+clean+"\n") {
		t.Errorf("expected header %q in output %q", "## "+clean, got)
	}
}

// TestDocAC3DuplicateFile: a file given twice is documented twice (no dedup).
func TestDocAC3DuplicateFile(t *testing.T) {
	src := writeTmp(t, "dup.wisp", "fn d() -> void { }\n")
	var so, se bytes.Buffer
	code := cmdDoc([]string{src, src}, &so, &se)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, se.String())
	}
	got := so.String()
	clean := filepath.Clean(src)
	count := strings.Count(got, "## "+clean+"\n")
	if count != 2 {
		t.Errorf("expected 2 occurrences of header, got %d in %q", count, got)
	}
}

// TestDocAC3ZeroDeclFile: a .wisp file with only a comment (no documentable decls)
// contributes no section -- empty output when it is the only input.
func TestDocAC3ZeroDeclFile(t *testing.T) {
	src := writeTmp(t, "nodecls.wisp", "// just a comment\n")
	var so, se bytes.Buffer
	code := cmdDoc([]string{src}, &so, &se)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, se.String())
	}
	if so.Len() != 0 {
		t.Fatalf("expected empty stdout for zero-decl file, got %q", so.String())
	}
}

// TestDocAC3Determinism: running the same command twice produces byte-identical stdout.
func TestDocAC3Determinism(t *testing.T) {
	src := writeTmp(t, "det.wisp", docFixtureSrc)
	run := func() string {
		var so, se bytes.Buffer
		if code := cmdDoc([]string{src}, &so, &se); code != 0 {
			t.Fatalf("exit=%d stderr=%q", code, se.String())
		}
		return so.String()
	}
	out1 := run()
	out2 := run()
	if out1 != out2 {
		t.Errorf("non-deterministic output:\nrun1: %q\nrun2: %q", out1, out2)
	}
}

// TestDocAC4FmtRoundTrip: a ///‑documented program through cmdFmt keeps its ///
// lines and re-parses to the same attached doc; fmt is idempotent.
func TestDocAC4FmtRoundTrip(t *testing.T) {
	const src = "/// Returns n doubled.\nfn foo(n: int) -> int { return n + n }\n"
	f := writeTmp(t, "fmt_doc.wisp", src)

	// Format once.
	var so1, se1 bytes.Buffer
	if code := cmdFmt([]string{f}, &so1, &se1); code != 0 {
		t.Fatalf("fmt exit=%d stderr=%q", code, se1.String())
	}
	formatted1 := so1.String()

	// The formatted output must still contain the /// line.
	if !strings.Contains(formatted1, "/// Returns n doubled.") {
		t.Errorf("fmt dropped /// comment: %q", formatted1)
	}

	// Format the formatted output (idempotent check).
	f2 := writeTmp(t, "fmt_doc2.wisp", formatted1)
	var so2, se2 bytes.Buffer
	if code := cmdFmt([]string{f2}, &so2, &se2); code != 0 {
		t.Fatalf("fmt2 exit=%d stderr=%q", code, se2.String())
	}
	formatted2 := so2.String()
	if formatted1 != formatted2 {
		t.Errorf("fmt not idempotent:\nfmt1: %q\nfmt2: %q", formatted1, formatted2)
	}

	// Run cmdDoc on the formatted file and confirm the doc attaches correctly.
	fmtFile := writeTmp(t, "fmt_doc_final.wisp", formatted1)
	var so3, se3 bytes.Buffer
	if code := cmdDoc([]string{fmtFile}, &so3, &se3); code != 0 {
		t.Fatalf("doc after fmt exit=%d stderr=%q", code, se3.String())
	}
	if !strings.Contains(so3.String(), "Returns n doubled.") {
		t.Errorf("doc after fmt lost prose: %q", so3.String())
	}
}
