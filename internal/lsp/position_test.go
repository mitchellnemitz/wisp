package lsp

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/lexer"
	"github.com/mitchellnemitz/wisp/internal/token"
)

func TestUTF16Units(t *testing.T) {
	cases := []struct {
		s    string
		want int
	}{
		{"", 0},
		{"abc", 3},
		{"é", 1},     // 2-byte UTF-8, one UTF-16 unit
		{"héllo", 5}, // 6 bytes, 5 units
		{"😀", 2},     // astral plane -> surrogate pair, two units
		{"a😀b", 4},
	}
	for _, c := range cases {
		if got := utf16Units(c.s); got != c.want {
			t.Errorf("utf16Units(%q) = %d, want %d", c.s, got, c.want)
		}
	}
}

func TestToLSPPositionMultibyte(t *testing.T) {
	// Bytes: a(1) é(2,3) ' '(4) b(5). byte columns are 1-based.
	lines := []string{"aé b"}
	// Byte col 4 = the space; prefix "aé" = 2 runes -> 2 UTF-16 units.
	if got := toLSPPosition(lines, token.Position{Line: 1, Col: 4}); got.Line != 0 || got.Character != 2 {
		t.Errorf("col 4 -> %+v, want {0,2}", got)
	}
	// Byte col 5 = 'b'; prefix "aé " = 3 runes -> 3 units (byte offset is 4).
	if got := toLSPPosition(lines, token.Position{Line: 1, Col: 5}); got.Character != 3 {
		t.Errorf("col 5 -> char %d, want 3", got.Character)
	}
}

func TestToLSPRangeBothEndpointsConverted(t *testing.T) {
	// Bytes: a(1) é(2,3) ' '(4) b(5) b(6).
	lines := []string{"aé bb"}
	// Start at first 'b' (byte col 5), width 2 bytes ("bb").
	r := toLSPRange(lines, token.Position{Line: 1, Col: 5}, 2)
	if r.Start.Character != 3 {
		t.Errorf("start char %d, want 3", r.Start.Character)
	}
	// End byte col 7 (offset 6); prefix "aé bb" = 5 runes -> 5 units. The end is
	// converted through UTF-16 too, not left as the byte column (6).
	if r.End.Character != 5 {
		t.Errorf("end char %d, want 5 (and != byte offset 6)", r.End.Character)
	}

	// Zero width -> start == end.
	z := toLSPRange(lines, token.Position{Line: 1, Col: 5}, 0)
	if z.Start != z.End {
		t.Errorf("zero-width range start %v != end %v", z.Start, z.End)
	}
}

func TestTokenWidthBytes(t *testing.T) {
	toks, err := lexer.Lex("fn foo123 + 42", "")
	if err != nil {
		t.Fatalf("lex: %v", err)
	}
	want := map[token.Kind]int{
		token.Fn:    2, // "fn"
		token.Ident: 6, // "foo123"
		token.Plus:  1, // "+"
		token.Int:   2, // "42"
	}
	seen := map[token.Kind]bool{}
	for _, tk := range toks {
		if w, ok := want[tk.Kind]; ok {
			seen[tk.Kind] = true
			if got := tokenWidthBytes(tk); got != w {
				t.Errorf("width(%v %q) = %d, want %d", tk.Kind, tk.Lit, got, w)
			}
		}
	}
	for k := range want {
		if !seen[k] {
			t.Errorf("token kind %v not seen in lex output", k)
		}
	}
}

// TestTokenAtCursorEndExclusive verifies that tokenAtCursor uses end-exclusive
// matching so a cursor on the first character of an identifier that abuts the
// end of a preceding token resolves to the identifier, not the preceding token.
func TestTokenAtCursorEndExclusive(t *testing.T) {
	// "a+b" on a single line. LSP chars (0-indexed): a=0, +=1, b=2.
	src := "a+b"
	toks, _ := lexer.Lex(src, "")
	lines := splitLines(src)

	// Cursor on first char of b (char 2): must resolve b, not +.
	tk, ok := tokenAtCursor(lines, toks, lspPosition{Line: 0, Character: 2})
	if !ok {
		t.Fatal("tokenAtCursor returned nothing for char 2 in a+b")
	}
	if tk.Kind != token.Ident || tk.Lit != "b" {
		t.Errorf("char 2 in a+b: got Kind=%v Lit=%q, want Ident b", tk.Kind, tk.Lit)
	}

	// "p.x": LSP chars p=0, .=1, x=2. Cursor on first char of x (char 2).
	src2 := "p.x"
	toks2, _ := lexer.Lex(src2, "")
	lines2 := splitLines(src2)
	tk2, ok2 := tokenAtCursor(lines2, toks2, lspPosition{Line: 0, Character: 2})
	if !ok2 {
		t.Fatal("tokenAtCursor returned nothing for char 2 in p.x")
	}
	if tk2.Kind != token.Ident || tk2.Lit != "x" {
		t.Errorf("char 2 in p.x: got Kind=%v Lit=%q, want Ident x", tk2.Kind, tk2.Lit)
	}

	// Regression: cursor at end of a token with whitespace following (no adjacent
	// next token). In "a + b", a is at chars 0..1, + at 2..3, b at 4..5.
	// Cursor at char 1 (the exclusive end of a) has no token strictly containing
	// it, so the fallback must return a.
	src3 := "a + b"
	toks3, _ := lexer.Lex(src3, "")
	lines3 := splitLines(src3)
	tk3, ok3 := tokenAtCursor(lines3, toks3, lspPosition{Line: 0, Character: 1})
	if !ok3 {
		t.Fatal("tokenAtCursor returned nothing for char 1 in 'a + b'")
	}
	if tk3.Kind != token.Ident || tk3.Lit != "a" {
		t.Errorf("char 1 in 'a + b': got Kind=%v Lit=%q, want Ident a", tk3.Kind, tk3.Lit)
	}
}
