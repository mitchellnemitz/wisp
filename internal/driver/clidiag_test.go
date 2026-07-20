package driver

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/token"
)

// H7: Build must refuse to write when the resolved output path is the source
// path itself (e.g. `wisp build foo.sh` -> DefaultOutPath returns foo.sh). It
// must leave the source untouched and return non-zero.
func TestBuildRefusesToOverwriteSource(t *testing.T) {
	dir := t.TempDir()
	// A .sh source whose DefaultOutPath collides with itself.
	src := filepath.Join(dir, "foo.sh")
	if err := os.WriteFile(src, []byte(helloSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	code := Build(src, helloSrc, src, false, &stderr)
	if code == 0 {
		t.Fatalf("build over source returned 0, want non-zero")
	}
	// The source file must be untouched.
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != helloSrc {
		t.Fatalf("source file was overwritten: %q", string(b))
	}
	if !strings.Contains(stderr.String(), "would overwrite") && !strings.Contains(stderr.String(), "same as the source") {
		t.Errorf("expected an explanatory error on stderr, got %q", stderr.String())
	}
}

// L4: building over a pre-existing output that is not executable must reset the
// mode to 0755 (os.WriteFile ignores perm on an existing file).
func TestBuildResetsModeOnExistingOutput(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out.sh")
	// Pre-create a non-executable file.
	if err := os.WriteFile(out, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	code := Build("hello.wisp", helloSrc, out, false, &stderr)
	if code != 0 {
		t.Fatalf("build exit = %d, stderr=%q", code, stderr.String())
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("output mode = %v, want 0755", info.Mode().Perm())
	}
}

// M7: the caret must align by display width, not byte count, when a multibyte
// UTF-8 rune precedes the error column.
func TestBuildCaretMultibyteAlignment(t *testing.T) {
	// "é" is two bytes (0xC3 0xA9) but one display column. The error is at the
	// byte column just past it. Lexer columns are byte-based, so the position's
	// Col is the byte offset; the caret must still land one display column in.
	line := "  é x"
	// Byte layout: ' '(1) ' '(2) é(3,4) ' '(5) 'x'(6). 'x' is at byte col 6.
	caret := buildCaret(line, 6)
	// Display: two leading spaces + one column for é + one space = 4 columns of
	// prefix, then '^'. So the caret prefix must be 4 spaces wide, not 5.
	if want := "    ^"; caret != want {
		t.Fatalf("caret = %q, want %q", caret, want)
	}
}

// M7 (tab preserved): a tab before the error column stays a tab in the caret.
func TestBuildCaretTabPreserved(t *testing.T) {
	line := "\tx"
	caret := buildCaret(line, 2) // 'x' at byte col 2
	if want := "\t^"; caret != want {
		t.Fatalf("caret = %q, want %q", caret, want)
	}
}

// M7 end-to-end through renderDiag: a multibyte char before the column aligns.
func TestRenderDiagMultibyteCaret(t *testing.T) {
	// The identifier "café" precedes the error; the offending token is after it.
	src := "fn main() -> int {\n  let café x = 1\n}\n"
	// Byte columns on line 2: leading "  " then "let " then "café" (c,a,f,é=5
	// bytes) then a space then 'x'. Compute the byte col of 'x'.
	l2 := "  let café x = 1"
	xByte := strings.IndexByte(l2[strings.Index(l2, "café"):], 'x') + strings.Index(l2, "café")
	d := Diagnostic{Pos: token.Position{File: "m.wisp", Line: 2, Col: xByte + 1}, Severity: Error, Msg: "boom"}
	got := renderDiag("m.wisp", src, d)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines:\n%s", got)
	}
	caretBody := lines[2][strings.Index(lines[2], "| ")+2:]
	srcBody := lines[1][strings.Index(lines[1], "| ")+2:]
	// The caret's display column must equal the display width of srcBody up to
	// 'x'. Since é counts as one column, prefix width = display width of
	// "  let café " = 11 columns.
	want := strings.Repeat(" ", 11) + "^"
	if caretBody != want {
		t.Fatalf("caret body = %q, want %q (src line %q)", caretBody, want, srcBody)
	}
}

// L6: a CRLF source line must not carry a trailing \r into the snippet.
func TestRenderDiagCRLFTrimsCarriageReturn(t *testing.T) {
	src := "fn main() -> int {\r\n  bad\r\n}\r\n"
	d := Diagnostic{Pos: token.Position{File: "crlf.wisp", Line: 2, Col: 3}, Severity: Error, Msg: "boom"}
	got := renderDiag("crlf.wisp", src, d)
	if strings.Contains(got, "\r") {
		t.Fatalf("snippet retained a carriage return: %q", got)
	}
}

// M6: an unterminated program produces a diagnostic with the filename and a
// real (non-zero) position, never `0:0`.
func TestCompileEOFErrorHasFilenameAndPosition(t *testing.T) {
	// A missing closing brace forces a parse error anchored at the EOF token.
	src := "fn main() -> int {\n  return 0\n"
	_, _, diags := Compile("eof.wisp", src)
	if len(diags) == 0 {
		t.Fatal("expected a diagnostic")
	}
	d := diags[0]
	if d.Pos.File != "eof.wisp" {
		t.Errorf("diagnostic file = %q, want eof.wisp", d.Pos.File)
	}
	if d.Pos.Line <= 0 || d.Pos.Col <= 0 {
		t.Errorf("diagnostic position = %d:%d, want real position", d.Pos.Line, d.Pos.Col)
	}
}

// M6: an empty file (no main) produces a no-main diagnostic carrying the
// filename and a real position, never `0:0`.
func TestCompileEmptyFileNoMainHasFilename(t *testing.T) {
	_, _, diags := Compile("empty.wisp", "")
	if len(diags) == 0 {
		t.Fatal("expected a no-main diagnostic")
	}
	var found bool
	for _, d := range diags {
		if strings.Contains(d.Msg, "main function") {
			found = true
			if d.Pos.File != "empty.wisp" {
				t.Errorf("no-main file = %q, want empty.wisp", d.Pos.File)
			}
			if d.Pos.Line <= 0 || d.Pos.Col <= 0 {
				t.Errorf("no-main position = %d:%d, want real position", d.Pos.Line, d.Pos.Col)
			}
		}
	}
	if !found {
		t.Fatalf("no no-main diagnostic in %v", diags)
	}
}
