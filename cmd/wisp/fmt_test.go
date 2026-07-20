package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// helloCanonical is `hello` after canonical formatting (4-space indent).
const helloCanonical = "fn main() -> int {\n    print(\"hi\")\n    return 0\n}\n"

func TestFmtStdout(t *testing.T) {
	src := writeTmp(t, "p.wisp", hello)
	var so, se bytes.Buffer
	if code := run([]string{"fmt", src}, &so, &se); code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, se.String())
	}
	if so.String() != helloCanonical {
		t.Fatalf("stdout=%q want %q", so.String(), helloCanonical)
	}
	// the file is untouched (no -w)
	b, _ := os.ReadFile(src)
	if string(b) != hello {
		t.Fatalf("fmt without -w modified the file")
	}
}

func TestFmtCheckUnformatted(t *testing.T) {
	src := writeTmp(t, "p.wisp", hello) // 2-space indent: not canonical
	var so, se bytes.Buffer
	code := run([]string{"fmt", "--check", src}, &so, &se)
	if code == 0 {
		t.Fatalf("--check on unformatted source must be non-zero")
	}
	if so.Len() != 0 {
		t.Fatalf("--check wrote to stdout: %q", so.String())
	}
}

func TestFmtCheckFormatted(t *testing.T) {
	src := writeTmp(t, "p.wisp", helloCanonical)
	var so, se bytes.Buffer
	if code := run([]string{"fmt", "--check", src}, &so, &se); code != 0 {
		t.Fatalf("--check on formatted source exit=%d want 0 stderr=%q", code, se.String())
	}
	if so.Len() != 0 {
		t.Fatalf("--check wrote to stdout: %q", so.String())
	}
}

func TestFmtWriteRewrites(t *testing.T) {
	src := writeTmp(t, "p.wisp", hello)
	var so, se bytes.Buffer
	if code := run([]string{"fmt", "-w", src}, &so, &se); code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, se.String())
	}
	if so.Len() != 0 {
		t.Fatalf("-w wrote to stdout: %q", so.String())
	}
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != helloCanonical {
		t.Fatalf("file not rewritten canonically: %q", string(b))
	}
	// running -w again is a no-op (already canonical) and still exits 0
	if code := run([]string{"fmt", "-w", src}, &so, &se); code != 0 {
		t.Fatalf("second -w exit=%d", code)
	}
}

func TestFmtParseErrorLocatedNoStdout(t *testing.T) {
	src := writeTmp(t, "bad.wisp", "fn main( -> int { return 0 }\n")
	var so, se bytes.Buffer
	code := run([]string{"fmt", src}, &so, &se)
	if code == 0 {
		t.Fatal("parse error must exit non-zero")
	}
	if so.Len() != 0 {
		t.Fatalf("parse error wrote to stdout: %q", so.String())
	}
	if !strings.Contains(se.String(), "bad.wisp:") {
		t.Fatalf("stderr not located: %q", se.String())
	}
}

func TestFmtParseErrorCheckNoWrite(t *testing.T) {
	// On a parse error, --check must not write the file and must exit non-zero.
	const bad = "fn main( -> int { return 0 }\n"
	src := writeTmp(t, "bad.wisp", bad)
	var so, se bytes.Buffer
	code := run([]string{"fmt", "--check", src}, &so, &se)
	if code == 0 {
		t.Fatal("parse error with --check must exit non-zero")
	}
	if so.Len() != 0 {
		t.Fatalf("wrote to stdout: %q", so.String())
	}
	b, _ := os.ReadFile(src)
	if string(b) != bad {
		t.Fatal("file was modified on a parse error")
	}
}

func TestFmtMissingFileExit2(t *testing.T) {
	var so, se bytes.Buffer
	if code := run([]string{"fmt"}, &so, &se); code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
}

func TestFmtConflictingFlagsExit2(t *testing.T) {
	src := writeTmp(t, "p.wisp", hello)
	var so, se bytes.Buffer
	if code := run([]string{"fmt", "-w", "--check", src}, &so, &se); code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
}

func TestFmtUsageMentionsFmt(t *testing.T) {
	if !strings.Contains(usage, "fmt") {
		t.Fatal("usage string does not mention fmt")
	}
}

// writeFileAt writes content to path, creating parent directories as needed.
func writeFileAt(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestFmtMultiFileCheckOffenders covers acceptance criterion 1: 3 files, 2
// non-canonical, args deliberately out of lexical order; stdout lists the
// offenders in lexical (not arg) order, one per line, exit 1, stderr empty.
func TestFmtMultiFileCheckOffenders(t *testing.T) {
	dir := t.TempDir()
	a := writeFileAt(t, filepath.Join(dir, "a.wisp"), hello)
	b := writeFileAt(t, filepath.Join(dir, "b.wisp"), hello)
	c := writeFileAt(t, filepath.Join(dir, "c.wisp"), helloCanonical)

	var so, se bytes.Buffer
	code := run([]string{"fmt", "--check", c, b, a}, &so, &se)
	if code != 1 {
		t.Fatalf("exit=%d want 1 stderr=%q", code, se.String())
	}
	want := filepath.Clean(a) + "\n" + filepath.Clean(b) + "\n"
	if so.String() != want {
		t.Fatalf("stdout=%q want %q", so.String(), want)
	}
	if se.Len() != 0 {
		t.Fatalf("stderr=%q want empty", se.String())
	}
}

// TestFmtMultiFileCheckClean covers acceptance criterion 2: a fully canonical
// set is silent on both streams, exit 0.
func TestFmtMultiFileCheckClean(t *testing.T) {
	dir := t.TempDir()
	a := writeFileAt(t, filepath.Join(dir, "a.wisp"), helloCanonical)
	b := writeFileAt(t, filepath.Join(dir, "b.wisp"), helloCanonical)
	c := writeFileAt(t, filepath.Join(dir, "c.wisp"), helloCanonical)

	var so, se bytes.Buffer
	code := run([]string{"fmt", "--check", a, b, c}, &so, &se)
	if code != 0 {
		t.Fatalf("exit=%d want 0 stderr=%q", code, se.String())
	}
	if so.Len() != 0 || se.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q want both empty", so.String(), se.String())
	}
}

// TestFmtDirWriteRewritesChangedSkipsUnchanged covers acceptance criterion 3:
// -w over a directory rewrites the non-canonical file and leaves the
// already-canonical file's mtime untouched (a sentinel past mtime proves the
// unchanged file was never passed to os.WriteFile).
func TestFmtDirWriteRewritesChangedSkipsUnchanged(t *testing.T) {
	dir := t.TempDir()
	changed := writeFileAt(t, filepath.Join(dir, "changed.wisp"), hello)
	same := writeFileAt(t, filepath.Join(dir, "same.wisp"), helloCanonical)

	t0 := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(same, t0, t0); err != nil {
		t.Fatal(err)
	}

	var so, se bytes.Buffer
	code := run([]string{"fmt", "-w", dir}, &so, &se)
	if code != 0 {
		t.Fatalf("exit=%d want 0 stderr=%q", code, se.String())
	}
	b, err := os.ReadFile(changed)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != helloCanonical {
		t.Fatalf("changed.wisp not rewritten canonically: %q", string(b))
	}
	info, err := os.Stat(same)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(t0) {
		t.Fatalf("same.wisp mtime changed: got %v want %v", info.ModTime(), t0)
	}
}

// TestFmtMultiFileParseErrorReportAndContinue covers acceptance criterion 4: a
// parse error on one file does not abort processing of the others.
func TestFmtMultiFileParseErrorReportAndContinue(t *testing.T) {
	dir := t.TempDir()
	a := writeFileAt(t, filepath.Join(dir, "a.wisp"), hello)
	bad := writeFileAt(t, filepath.Join(dir, "bad.wisp"), "fn main( -> int { return 0 }\n")
	c := writeFileAt(t, filepath.Join(dir, "c.wisp"), hello)

	var so, se bytes.Buffer
	code := run([]string{"fmt", "--check", a, bad, c}, &so, &se)
	if code != 1 {
		t.Fatalf("exit=%d want 1", code)
	}
	if !strings.Contains(se.String(), "bad.wisp:") {
		t.Fatalf("stderr not located: %q", se.String())
	}
	want := filepath.Clean(a) + "\n" + filepath.Clean(c) + "\n"
	if so.String() != want {
		t.Fatalf("stdout=%q want %q (bad file must not abort the sweep)", so.String(), want)
	}
}

// TestFmtNoFlagDirectoryExit2 covers acceptance criterion 5.
func TestFmtNoFlagDirectoryExit2(t *testing.T) {
	dir := t.TempDir()
	writeFileAt(t, filepath.Join(dir, "a.wisp"), hello)

	var so, se bytes.Buffer
	code := run([]string{"fmt", dir}, &so, &se)
	if code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
	const want = "wisp: fmt to stdout requires exactly one file (use -w or --check)\n"
	if se.String() != want {
		t.Fatalf("stderr=%q want %q", se.String(), want)
	}
	if so.Len() != 0 {
		t.Fatalf("stdout=%q want empty", so.String())
	}
}

// TestFmtNoFlagMultipleFilesExit2 covers acceptance criterion 6.
func TestFmtNoFlagMultipleFilesExit2(t *testing.T) {
	dir := t.TempDir()
	a := writeFileAt(t, filepath.Join(dir, "a.wisp"), hello)
	b := writeFileAt(t, filepath.Join(dir, "b.wisp"), hello)

	var so, se bytes.Buffer
	code := run([]string{"fmt", a, b}, &so, &se)
	if code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
	const want = "wisp: fmt to stdout requires exactly one file (use -w or --check)\n"
	if se.String() != want {
		t.Fatalf("stderr=%q want %q", se.String(), want)
	}
}

// TestFmtDirectoryWalkOrderIndependent covers acceptance criterion 12: the
// same fixture tree via two directories in opposite arg order produces
// byte-identical stdout.
func TestFmtDirectoryWalkOrderIndependent(t *testing.T) {
	root := t.TempDir()
	dir1 := filepath.Join(root, "dir1")
	dir2 := filepath.Join(root, "dir2")
	writeFileAt(t, filepath.Join(dir1, "a.wisp"), hello)
	writeFileAt(t, filepath.Join(dir1, "sub", "b.wisp"), hello)
	writeFileAt(t, filepath.Join(dir2, "c.wisp"), hello)

	var so1, se1, so2, se2 bytes.Buffer
	code1 := run([]string{"fmt", "--check", dir1, dir2}, &so1, &se1)
	code2 := run([]string{"fmt", "--check", dir2, dir1}, &so2, &se2)
	if code1 != code2 {
		t.Fatalf("exit codes differ: %d vs %d", code1, code2)
	}
	if so1.String() != so2.String() {
		t.Fatalf("stdout differs by arg order:\n%q\n%q", so1.String(), so2.String())
	}
}

// TestFmtDedupeExplicitAndDirectory covers acceptance criterion 14: the same
// file reached via an explicit arg and a directory arg is reported exactly
// once.
func TestFmtDedupeExplicitAndDirectory(t *testing.T) {
	dir := t.TempDir()
	a := writeFileAt(t, filepath.Join(dir, "a.wisp"), hello)

	var so, se bytes.Buffer
	code := run([]string{"fmt", "--check", dir, a}, &so, &se)
	if code != 1 {
		t.Fatalf("exit=%d want 1 stderr=%q", code, se.String())
	}
	want := filepath.Clean(a) + "\n"
	if so.String() != want {
		t.Fatalf("stdout=%q want %q (exactly once)", so.String(), want)
	}
}

// TestFmtSkipsDotWispSubtree covers acceptance criterion 15: a `.wisp/`
// subdirectory (the package/module cache) is never walked into.
func TestFmtSkipsDotWispSubtree(t *testing.T) {
	dir := t.TempDir()
	real := writeFileAt(t, filepath.Join(dir, "real.wisp"), hello)
	writeFileAt(t, filepath.Join(dir, ".wisp", "junk.wisp"), hello)

	var so, se bytes.Buffer
	code := run([]string{"fmt", "--check", dir}, &so, &se)
	if code != 1 {
		t.Fatalf("exit=%d want 1 stderr=%q", code, se.String())
	}
	want := filepath.Clean(real) + "\n"
	if so.String() != want {
		t.Fatalf("stdout=%q want %q (.wisp/ subtree must be excluded)", so.String(), want)
	}
}

// TestFmtEmptyDirectory covers acceptance criterion 16: a directory with no
// *.wisp files is vacuously clean under --check, and a no-op success under -w.
func TestFmtEmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	var so, se bytes.Buffer
	if code := run([]string{"fmt", "--check", dir}, &so, &se); code != 0 {
		t.Fatalf("--check exit=%d want 0 stderr=%q", code, se.String())
	}
	if so.Len() != 0 || se.Len() != 0 {
		t.Fatalf("--check stdout=%q stderr=%q want both empty", so.String(), se.String())
	}

	so.Reset()
	se.Reset()
	if code := run([]string{"fmt", "-w", dir}, &so, &se); code != 0 {
		t.Fatalf("-w exit=%d want 0 stderr=%q", code, se.String())
	}
}
