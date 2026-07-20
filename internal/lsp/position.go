package lsp

import (
	"strings"

	"github.com/mitchellnemitz/wisp/internal/token"
)

// splitLines splits a document into lines without separators, indexable by
// 0-based line number. The split is on '\n' only; any '\r' stays in the line
// bytes so a byte column lines up with what the lexer counted.
func splitLines(text string) []string {
	return strings.Split(text, "\n")
}

// utf16Units counts the UTF-16 code units in s. A rune outside the Basic
// Multilingual Plane encodes as a surrogate pair (two units); everything else
// is one. This is the unit LSP positions are measured in.
func utf16Units(s string) int {
	n := 0
	for _, r := range s {
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// toLSPPosition converts a token.Position (1-based line, 1-based BYTE column)
// to an LSP position (0-based line, UTF-16 character). The character is the
// number of UTF-16 code units in the line prefix up to the byte column, so a
// multibyte character before the column shifts the LSP character correctly.
func toLSPPosition(lines []string, pos token.Position) lspPosition {
	line := pos.Line - 1
	if line < 0 {
		line = 0
	}
	col := pos.Col - 1 // byte offset within the line
	if col < 0 {
		col = 0
	}
	prefix := ""
	if line < len(lines) {
		lt := lines[line]
		if col > len(lt) {
			col = len(lt)
		}
		prefix = lt[:col]
	}
	return lspPosition{Line: line, Character: utf16Units(prefix)}
}

// toLSPRange builds a range starting at pos and extending widthBytes bytes on
// the same line. BOTH endpoints are converted through toLSPPosition, so the
// multibyte shift applies to the end column too (not just the start). A
// non-positive width yields a zero-width range at the start.
func toLSPRange(lines []string, pos token.Position, widthBytes int) lspRange {
	start := toLSPPosition(lines, pos)
	if widthBytes <= 0 {
		return lspRange{Start: start, End: start}
	}
	endPos := pos
	endPos.Col = pos.Col + widthBytes // still a byte column on the same line
	return lspRange{Start: start, End: toLSPPosition(lines, endPos)}
}

// tokenWidthBytes is the source-text byte width of a token, used to size a
// diagnostic range. Identifiers, ints, and floats carry their source text in
// Lit. Keywords, operators, and punctuation render their fixed source spelling
// via Kind.String(). String pieces are decoded (Lit may differ from the
// source), so they report width 0 -> a zero-width range, which is the
// documented fallback.
func tokenWidthBytes(tok token.Token) int {
	switch tok.Kind {
	case token.Ident, token.Int, token.FloatLit:
		return len(tok.Lit)
	case token.String, token.StringStart, token.StringText, token.StringEnd,
		token.InterpOpen, token.InterpClose, token.EOF:
		return 0
	default:
		return len(tok.Kind.String())
	}
}

// rangeAtPos finds the token starting exactly at pos among toks and returns a
// range covering it; if no token starts there it returns a zero-width range at
// pos. (Two tokens can never share a start position, so the first match is the
// token at that position.)
func rangeAtPos(lines []string, toks []token.Token, pos token.Position) lspRange {
	for _, tk := range toks {
		if tk.Pos.Line == pos.Line && tk.Pos.Col == pos.Col {
			return toLSPRange(lines, pos, tokenWidthBytes(tk))
		}
	}
	return toLSPRange(lines, pos, 0)
}
