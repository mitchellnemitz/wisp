package driver

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mitchellnemitz/wisp/internal/token"
)

// TestRenderDiagErrorSnippet renders an error with a source line and caret.
func TestRenderDiagErrorSnippet(t *testing.T) {
	src := "fn main() -> int {\n  while (count) {\n  }\n}\n"
	d := Diagnostic{
		Pos:      token.Position{File: "prog.wisp", Line: 2, Col: 10},
		Severity: Error,
		Msg:      "condition must be bool, got int",
	}
	got := renderDiag("prog.wisp", src, d)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d:\n%s", len(lines), got)
	}
	if lines[0] != "prog.wisp:2:10: condition must be bool, got int" {
		t.Errorf("head = %q", lines[0])
	}
	// Source line, with a gutter.
	if !strings.Contains(lines[1], "while (count) {") {
		t.Errorf("source line = %q", lines[1])
	}
	// Caret line: the ^ must align under column 10. Find the caret offset and the
	// matching offset in the source line; they must coincide.
	caretOff := strings.IndexByte(lines[2], '^')
	if caretOff < 0 {
		t.Fatalf("no caret in %q", lines[2])
	}
	if lines[1][caretOff] != src[len("fn main() -> int {\n")+9] {
		// the char under the caret in the source line should be the 10th byte ('c').
		t.Errorf("caret not aligned: source char under caret = %q", string(lines[1][caretOff]))
	}
	if lines[1][caretOff] != 'c' {
		t.Errorf("expected caret under 'c' (col 10), got %q", string(lines[1][caretOff]))
	}
}

// TestRenderDiagWarningSnippet renders a warning with the "warning: " marker and
// a snippet.
func TestRenderDiagWarningSnippet(t *testing.T) {
	src := "fn main() -> int {\n  let unused: int = 5\n  return 0\n}\n"
	d := Diagnostic{
		Pos:      token.Position{File: "w.wisp", Line: 2, Col: 7},
		Severity: Warning,
		Msg:      "unused local",
	}
	got := renderDiag("w.wisp", src, d)
	if !strings.HasPrefix(got, "w.wisp:2:7: warning: unused local\n") {
		t.Fatalf("warning head wrong:\n%s", got)
	}
	if !strings.Contains(got, "let unused: int = 5") {
		t.Errorf("missing source line:\n%s", got)
	}
	if !strings.Contains(got, "^") {
		t.Errorf("missing caret:\n%s", got)
	}
}

// TestRenderDiagTabIndent verifies a tab in the source's leading whitespace is
// copied verbatim into the caret line, so the ^ aligns under tab-stop rendering.
func TestRenderDiagTabIndent(t *testing.T) {
	// A tab-indented line; the offending token starts after the tab (col 2).
	src := "fn main() -> int {\n\tbroken\n}\n"
	d := Diagnostic{
		Pos:      token.Position{File: "t.wisp", Line: 2, Col: 2},
		Severity: Error,
		Msg:      "boom",
	}
	got := renderDiag("t.wisp", src, d)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines:\n%s", got)
	}
	// The caret line, after its gutter, must begin with the tab copied verbatim
	// then the ^ (so it lands under 'b' at the same tab stop).
	caret := lines[2]
	gutterEnd := strings.Index(caret, "| ")
	if gutterEnd < 0 {
		t.Fatalf("no gutter in caret line %q", caret)
	}
	body := caret[gutterEnd+2:]
	if body != "\t^" {
		t.Fatalf("caret body = %q, want %q (verbatim tab + caret)", body, "\t^")
	}
}

// TestWriteDiagsWarningOnlyExitsZeroAndStderr verifies a warning-only compile
// still exits 0 and the rendered diagnostic goes to stderr (the writer), not
// stdout. (Check writes to the provided writer; exit code is 0.)
func TestWriteDiagsWarningOnlyExitsZeroAndStderr(t *testing.T) {
	src := "fn main() -> int {\n  let unused: int = 5\n  return 0\n}\n"
	var stderr bytes.Buffer
	code := Check("w.wisp", src, &stderr)
	if code != 0 {
		t.Fatalf("warning-only check exit = %d, want 0", code)
	}
	out := stderr.String()
	if !strings.Contains(out, "warning:") {
		t.Errorf("expected a warning on stderr: %q", out)
	}
	if !strings.Contains(out, "^") {
		t.Errorf("expected a caret in the rendered warning: %q", out)
	}
}

// TestRenderDiagNoPositionNoSnippet: a position-less diagnostic (e.g. a
// program-level error) renders just the head line, no snippet/caret.
func TestRenderDiagNoPositionNoSnippet(t *testing.T) {
	d := Diagnostic{Severity: Error, Msg: "program has no main function"}
	got := renderDiag("x.wisp", "whatever\n", d)
	if strings.Contains(got, "\n") {
		t.Fatalf("position-less diagnostic should be a single line, got:\n%s", got)
	}
	if strings.Contains(got, "^") {
		t.Fatalf("no caret expected: %q", got)
	}
}

// TestRenderDiagSanitizesControlBytesInSnippet is the primary kill test
// (criterion 1): a source line carrying raw C0 control bytes and DEL must not
// leak any of them into the rendered output. Reverting the snippet's
// sanitizeForTerminal wrap fails this test.
func TestRenderDiagSanitizesControlBytesInSnippet(t *testing.T) {
	src := "fn main() -> int {\n  bad\x1b\x07\x7f\x00\x0b\x0c x\n}\n"
	d := Diagnostic{Pos: token.Position{File: "prog.wisp", Line: 2, Col: 3}, Severity: Error, Msg: "boom"}
	got := renderDiag("prog.wisp", src, d)
	if strings.ContainsAny(got, "\x1b\x07\x7f\x00\x0b\x0c") {
		t.Fatalf("rendered output leaked a raw control byte: %q", got)
	}
}

// TestRenderDiagSanitizesMsgInWarningBranch is a kill test (criterion 2) for
// the Warning branch, which builds its head manually and does not call
// Diagnostic.String(). Reverting that branch's sanitizeForTerminal wrap fails
// this test.
func TestRenderDiagSanitizesMsgInWarningBranch(t *testing.T) {
	d := Diagnostic{Pos: token.Position{File: "prog.wisp", Line: 1, Col: 1}, Severity: Warning, Msg: "bad\x1btoken"}
	got := renderDiag("prog.wisp", "x\n", d)
	if strings.ContainsAny(got, "\x1b\x07\x7f\x00") {
		t.Fatalf("warning head leaked a raw control byte: %q", got)
	}
}

// TestDiagnosticStringSanitizesMsg is a kill test (criterion 2b) for the
// testrunner path: internal/testrunner/testrunner.go:246 calls d.String()
// directly, bypassing renderDiag entirely. Reverting Diagnostic.String()'s
// sanitizeForTerminal wrap fails this test.
func TestDiagnosticStringSanitizesMsg(t *testing.T) {
	d := Diagnostic{Pos: token.Position{File: "t.wisp", Line: 1, Col: 1}, Severity: Error, Msg: "bad\x1b\x07\x7f\x00token"}
	got := d.String()
	if strings.ContainsAny(got, "\x1b\x07\x7f\x00") {
		t.Fatalf("String() leaked a raw control byte: %q", got)
	}
	if !strings.Contains(got, "␛␇␡␀") {
		t.Errorf("String() missing expected placeholder glyphs: %q", got)
	}
	if !strings.HasPrefix(got, "t.wisp:1:1: ") {
		t.Errorf("String() prefix = %q, want unmodified %q", got, "t.wisp:1:1: ")
	}
}

// TestRenderDiagControlBytePlaceholders (criterion 3): each of the six control
// bytes in the snippet must be replaced by its exact Control Pictures glyph,
// with DEL special-cased to U+2421 rather than the linear U+247F.
func TestRenderDiagControlBytePlaceholders(t *testing.T) {
	src := "fn main() -> int {\n  bad\x1b\x07\x7f\x00\x0b\x0c x\n}\n"
	d := Diagnostic{Pos: token.Position{File: "prog.wisp", Line: 2, Col: 3}, Severity: Error, Msg: "boom"}
	got := renderDiag("prog.wisp", src, d)
	for _, glyph := range []string{"␛", "␇", "␀", "␡", "␋", "␌"} {
		if !strings.Contains(got, glyph) {
			t.Errorf("rendered output missing placeholder %q:\n%s", glyph, got)
		}
	}
}

// TestRenderDiagCaretAlignsByRuneAfterSanitization (criterion 4): the caret
// must align by RUNE column against the sanitized snippet, not byte offset --
// the ␛ placeholder is a 3-byte rune, so a byte-index comparison would be
// wrong once sanitization is in place.
func TestRenderDiagCaretAlignsByRuneAfterSanitization(t *testing.T) {
	src := "fn main() -> int {\n  \x1bx\n}\n"
	d := Diagnostic{Pos: token.Position{File: "prog.wisp", Line: 2, Col: 4}, Severity: Error, Msg: "boom"}
	got := renderDiag("prog.wisp", src, d)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines:\n%s", got)
	}
	snipBody := lines[1][strings.Index(lines[1], "| ")+2:]
	caretBody := lines[2][strings.Index(lines[2], "| ")+2:]
	caretCol := utf8.RuneCountInString(caretBody[:strings.IndexByte(caretBody, '^')])
	snipRunes := []rune(snipBody)
	if caretCol >= len(snipRunes) || snipRunes[caretCol] != 'x' {
		t.Fatalf("caret rune column %d does not land on 'x' in snippet %q (caret %q)", caretCol, snipBody, caretBody)
	}
}

// TestRenderDiagTabPreservedWithSanitization (criterion 5): a leading tab must
// still be copied verbatim into the caret line alongside a sanitized control
// byte elsewhere in the line.
func TestRenderDiagTabPreservedWithSanitization(t *testing.T) {
	src := "fn main() -> int {\n\t\x1bx\n}\n"
	d := Diagnostic{Pos: token.Position{File: "prog.wisp", Line: 2, Col: 3}, Severity: Error, Msg: "boom"}
	got := renderDiag("prog.wisp", src, d)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines:\n%s", got)
	}
	caret := lines[2]
	gutterEnd := strings.Index(caret, "| ")
	if gutterEnd < 0 {
		t.Fatalf("no gutter in caret line %q", caret)
	}
	body := caret[gutterEnd+2:]
	if !strings.HasPrefix(body, "\t") {
		t.Fatalf("caret body = %q, want a leading tab preserved", body)
	}
}

// TestRenderDiagCleanASCIINoRegression (criterion 6): a clean ASCII input must
// render byte-identical output to the pre-sanitization implementation.
func TestRenderDiagCleanASCIINoRegression(t *testing.T) {
	src := "fn main() -> int {\n  while (count) {\n  }\n}\n"
	d := Diagnostic{
		Pos:      token.Position{File: "prog.wisp", Line: 2, Col: 10},
		Severity: Error,
		Msg:      "condition must be bool, got int",
	}
	got := renderDiag("prog.wisp", src, d)
	want := "prog.wisp:2:10: condition must be bool, got int\n  2 | " +
		"  while (count) {" + "\n    | " + strings.Repeat(" ", 9) + "^"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// TestRenderDiagMultibyteNoRegression (criterion 7): valid multibyte UTF-8
// (bytes >= 0x80) must pass through sanitizeForTerminal untouched, verified by
// a full-output exact comparison (not a substring check).
func TestRenderDiagMultibyteNoRegression(t *testing.T) {
	src := "fn main() -> int {\n  let café x = 1\n}\n"
	l2 := "  let café x = 1"
	xByte := strings.IndexByte(l2[strings.Index(l2, "café"):], 'x') + strings.Index(l2, "café")
	d := Diagnostic{Pos: token.Position{File: "m.wisp", Line: 2, Col: xByte + 1}, Severity: Error, Msg: "boom"}
	got := renderDiag("m.wisp", src, d)
	head := fmt.Sprintf("m.wisp:2:%d: boom", xByte+1)
	want := head + "\n  2 | " + l2 + "\n    | " + strings.Repeat(" ", 11) + "^"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// TestRenderDiagMsgIdempotentOnQuotedContent (criterion 8): a message already
// %q-escaped (so the ESC byte appears as the literal 4 ASCII characters
// `\x1b`, not a raw ESC byte) must pass through sanitizeForTerminal unchanged
// -- no double-escaping.
func TestRenderDiagMsgIdempotentOnQuotedContent(t *testing.T) {
	msg := fmt.Sprintf("unexpected character %q", "\x1b")
	d := Diagnostic{Pos: token.Position{File: "prog.wisp", Line: 1, Col: 1}, Severity: Error, Msg: msg}
	got := renderDiag("prog.wisp", "x\n", d)
	if !strings.Contains(got, `\x1b`) {
		t.Errorf("expected the literal escaped substring %s in %q", `\x1b`, got)
	}
	if strings.Contains(got, "␛") {
		t.Errorf("already-escaped content was double-sanitized: %q", got)
	}
}
