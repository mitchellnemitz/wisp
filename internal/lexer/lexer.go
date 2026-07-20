// Package lexer turns wisp source text into a token stream.
package lexer

import (
	"fmt"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/token"
)

// Error is a lexical error with a source position.
type Error struct {
	Pos token.Position
	Msg string
}

func (e *Error) Error() string {
	return e.Pos.String() + ": " + e.Msg
}

// Comment is a `//` line comment retained on the lexer's side channel for the
// formatter (B1). It never enters the parser's token stream, so parsing and
// codegen are structurally unaffected by comments. Pos is the position of the
// leading '/'. Trailing is true when code preceded the comment on the same
// source line (a trailing comment), false for a full-line comment. Text is the
// comment as written, including the leading "//", with trailing whitespace and
// any carriage return stripped.
type Comment struct {
	Pos      token.Position
	Text     string
	Trailing bool
}

// Lex tokenizes src and returns the token stream (always ending in an EOF
// token) or the first lexical error encountered. The filename is recorded in
// every token's position. Comments are skipped; the byte-stable signature is
// preserved for existing callers (use LexWithComments to retain comments).
func Lex(src, filename string) ([]token.Token, error) {
	toks, _, err := LexWithComments(src, filename)
	return toks, err
}

// LexWithComments tokenizes src exactly as Lex does (the returned token stream
// is identical) and additionally returns the `//` line comments in source
// order on a side channel for the formatter. The main token stream is
// unchanged: comments are not tokens.
func LexWithComments(src, filename string) ([]token.Token, []Comment, error) {
	l := &lexer{
		src:         src,
		file:        filename,
		line:        1,
		col:         1,
		lastTokLine: 0,
	}
	toks, err := l.run()
	if err != nil {
		return nil, nil, err
	}
	return toks, l.comments, nil
}

type lexer struct {
	src  string
	file string
	pos  int // byte offset into src
	line int
	col  int

	toks []token.Token

	// comments collects retained `//` comments on the side channel.
	comments []Comment
	// lastTokLine is the source line of the most recently emitted real token (a
	// Separator counts as the newline that ends a line, so it is NOT a real
	// token for this purpose). A comment is trailing when it begins on the same
	// line as a preceding real token.
	lastTokLine int
}

func (l *lexer) run() ([]token.Token, error) {
	for l.pos < len(l.src) {
		if err := l.next(); err != nil {
			return nil, err
		}
	}
	// Emit the EOF token at the real end-of-input position (with the file) so a
	// parse error anchored at EOF renders `file:line:col`, not `0:0` (M6).
	l.emitAt(token.EOF, "", l.position())
	return l.toks, nil
}

func (l *lexer) position() token.Position {
	return token.Position{File: l.file, Line: l.line, Col: l.col}
}

func (l *lexer) emitAt(kind token.Kind, lit string, pos token.Position) {
	l.toks = append(l.toks, token.Token{Kind: kind, Lit: lit, Pos: pos})
	l.noteTokLine(kind, pos)
}

// noteTokLine records the line of the most recent real (non-Separator) token so
// the comment branch can decide trailing vs full-line. A Separator is the
// newline/';' that ends a statement, not code, so it does not count.
func (l *lexer) noteTokLine(kind token.Kind, pos token.Position) {
	if kind == token.Separator {
		return
	}
	l.lastTokLine = pos.Line
}

func (l *lexer) errf(pos token.Position, format string, args ...any) error {
	return &Error{Pos: pos, Msg: fmt.Sprintf(format, args...)}
}

// advance consumes one byte, updating line/col.
func (l *lexer) advance() byte {
	c := l.src[l.pos]
	l.pos++
	if c == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return c
}

func (l *lexer) peek() byte {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *lexer) peekAt(off int) byte {
	if l.pos+off >= len(l.src) {
		return 0
	}
	return l.src[l.pos+off]
}

// next lexes one top-level token (or skips whitespace/comments).
func (l *lexer) next() error {
	c := l.peek()
	switch {
	case c == ' ' || c == '\t' || c == '\r':
		l.advance()
		return nil
	case c == '\n':
		pos := l.position()
		l.advance()
		l.emitAt(token.Separator, "\n", pos)
		return nil
	case c == ';':
		pos := l.position()
		l.advance()
		l.emitAt(token.Separator, ";", pos)
		return nil
	case c == '/' && l.peekAt(1) == '/':
		// line comment: retain it on the side channel, then skip to end of line
		// (the newline is handled on the next iteration as a Separator). It never
		// enters the token stream, so parsing/codegen are unaffected.
		pos := l.position()
		trailing := l.lastTokLine == pos.Line
		start := l.pos
		for l.pos < len(l.src) && l.peek() != '\n' {
			l.advance()
		}
		text := strings.TrimRight(l.src[start:l.pos], " \t\r")
		l.comments = append(l.comments, Comment{Pos: pos, Text: text, Trailing: trailing})
		return nil
	case isIdentStart(c):
		l.lexIdent()
		return nil
	case isDigit(c):
		return l.lexNumber()
	case c == '.' && isDigit(l.peekAt(1)):
		// A leading-dot form like `.5` is not a valid float literal (the grammar
		// requires a digit before the dot). Reject it with a located error rather
		// than the generic "unexpected character".
		return l.errf(l.position(), "malformed float literal: a float requires digits before '.' (write 0%s)", l.floatPreview())
	case c == '\'':
		return l.lexSingleString()
	case c == '"':
		return l.lexDoubleString()
	default:
		return l.lexOperator()
	}
}

func (l *lexer) lexIdent() {
	pos := l.position()
	start := l.pos
	for l.pos < len(l.src) && isIdentPart(l.peek()) {
		l.advance()
	}
	lit := l.src[start:l.pos]
	if kind, ok := token.Lookup(lit); ok {
		l.emitAt(kind, lit, pos)
		return
	}
	l.emitAt(token.Ident, lit, pos)
}

// lexNumber lexes an int literal or a float literal. A float is
// <digits>.<digits> (at least one digit each side of a single '.', no sign, no
// exponent). The grammar's malformed neighbours are rejected with a located
// error: a trailing dot (`3.`), a non-digit after the dot (`3.x`), an exponent
// (`3.14e5` / `3e5`), and a second dot (`3.1.4`).
func (l *lexer) lexNumber() error {
	pos := l.position()
	start := l.pos
	for l.pos < len(l.src) && isDigit(l.peek()) {
		l.advance()
	}
	if l.peek() != '.' {
		// Plain integer. A bare exponent form (`3e5`) is not a wisp number at all;
		// reject it located rather than lexing `3` then ident `e5`.
		if l.peek() == 'e' || l.peek() == 'E' {
			return l.errf(l.position(), "malformed number literal: exponent form is not supported")
		}
		l.emitAt(token.Int, l.src[start:l.pos], pos)
		return nil
	}
	// We have digits then a '.'. Require at least one digit after the dot.
	dotPos := l.position()
	if !isDigit(l.peekAt(1)) {
		return l.errf(dotPos, "malformed float literal: a float requires a digit after '.' (got %q)", l.dotContext(start))
	}
	l.advance() // consume '.'
	for l.pos < len(l.src) && isDigit(l.peek()) {
		l.advance()
	}
	// No exponent form and no second dot are accepted.
	switch l.peek() {
	case 'e', 'E':
		return l.errf(l.position(), "malformed float literal: exponent form is not supported")
	case '.':
		return l.errf(l.position(), "malformed float literal: a float has exactly one '.'")
	}
	l.emitAt(token.FloatLit, l.src[start:l.pos], pos)
	return nil
}

// floatPreview renders the leading-dot float at the cursor for the error
// message (e.g. for `.5` it shows ".5" so the hint reads "write 0.5").
func (l *lexer) floatPreview() string {
	end := l.pos + 1
	for end < len(l.src) && isDigit(l.src[end]) {
		end++
	}
	return l.src[l.pos:end]
}

// dotContext renders the malformed number from start through the dot for an
// error message.
func (l *lexer) dotContext(start int) string {
	end := l.pos + 1
	if end > len(l.src) {
		end = len(l.src)
	}
	return l.src[start:end]
}

func (l *lexer) lexSingleString() error {
	pos := l.position()
	l.advance() // opening '
	var b []byte
	for {
		if l.pos >= len(l.src) {
			return l.errf(pos, "unterminated string literal")
		}
		c := l.peek()
		switch c {
		case 0:
			return l.errf(l.position(), "NUL byte in string literal")
		case '\'':
			l.advance()
			l.emitAt(token.String, string(b), pos)
			return nil
		case '\\':
			if l.peekAt(1) == '\'' {
				l.advance()
				l.advance()
				b = append(b, '\'')
				continue
			}
			if l.peekAt(1) == '\\' {
				l.advance()
				l.advance()
				b = append(b, '\\')
				continue
			}
			// any other backslash is literal in a single-quoted string
			l.advance()
			b = append(b, '\\')
		default:
			l.advance()
			b = append(b, c)
		}
	}
}

func (l *lexer) lexDoubleString() error {
	openPos := l.position()
	l.advance() // opening "
	l.emitAt(token.StringStart, "", openPos)

	var b []byte
	chunkPos := l.position()
	flush := func() {
		if len(b) > 0 {
			l.emitAt(token.StringText, string(b), chunkPos)
			b = b[:0]
		}
	}

	for {
		if l.pos >= len(l.src) {
			return l.errf(openPos, "unterminated string literal")
		}
		c := l.peek()
		switch c {
		case 0:
			return l.errf(l.position(), "NUL byte in string literal")
		case '"':
			flush()
			endPos := l.position()
			l.advance()
			l.emitAt(token.StringEnd, "", endPos)
			return nil
		case '\\':
			escPos := l.position()
			n := l.peekAt(1)
			switch n {
			case 'n':
				b = append(b, '\n')
			case 't':
				b = append(b, '\t')
			case '"':
				b = append(b, '"')
			case '\\':
				b = append(b, '\\')
			case '$':
				b = append(b, '$')
			default:
				return l.errf(escPos, "invalid escape sequence %q in string literal", "\\"+string(n))
			}
			l.advance()
			l.advance()
		case '$':
			if l.peekAt(1) == '{' {
				flush()
				if err := l.lexInterp(); err != nil {
					return err
				}
				chunkPos = l.position()
				continue
			}
			// a bare '$' not followed by '{' is literal text
			l.advance()
			if len(b) == 0 {
				chunkPos = token.Position{File: l.file, Line: l.line, Col: l.col - 1}
			}
			b = append(b, '$')
		default:
			if len(b) == 0 {
				chunkPos = l.position()
			}
			l.advance()
			b = append(b, c)
		}
	}
}

// lexInterp lexes a ${ ... } group. The cursor is positioned at '$'. It emits
// InterpOpen, then the normal tokens of the embedded expression, then
// InterpClose. Nested braces are tracked so a '}' that closes a nested brace
// does not terminate the interpolation. (M1 expressions never contain '{', but
// the depth tracking is cheap and correct.)
func (l *lexer) lexInterp() error {
	openPos := l.position()
	l.advance() // $
	l.advance() // {
	l.emitAt(token.InterpOpen, "", openPos)

	depth := 0
	for {
		if l.pos >= len(l.src) {
			return l.errf(openPos, "unterminated interpolation")
		}
		c := l.peek()
		if c == '}' && depth == 0 {
			closePos := l.position()
			l.advance()
			l.emitAt(token.InterpClose, "", closePos)
			return nil
		}
		if c == '\n' {
			// a newline inside ${ } would be a stray separator; disallow it so
			// an unterminated interp does not silently swallow following lines.
			return l.errf(openPos, "unterminated interpolation")
		}
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
		}
		if err := l.next(); err != nil {
			return err
		}
	}
}

func (l *lexer) lexOperator() error {
	pos := l.position()
	c := l.peek()
	two := func(kind token.Kind, lit string) error {
		l.advance()
		l.advance()
		l.emitAt(kind, lit, pos)
		return nil
	}
	one := func(kind token.Kind, lit string) error {
		l.advance()
		l.emitAt(kind, lit, pos)
		return nil
	}
	switch c {
	case '+':
		return one(token.Plus, "+")
	case '-':
		if l.peekAt(1) == '>' {
			return two(token.Arrow, "->")
		}
		return one(token.Minus, "-")
	case '*':
		return one(token.Star, "*")
	case '/':
		return one(token.Slash, "/")
	case '%':
		return one(token.Percent, "%")
	case '!':
		if l.peekAt(1) == '=' {
			return two(token.Neq, "!=")
		}
		return one(token.Bang, "!")
	case '&':
		if l.peekAt(1) == '&' {
			return two(token.AndAnd, "&&")
		}
		return one(token.Amp, "&")
	case '|':
		if l.peekAt(1) == '|' {
			return two(token.OrOr, "||")
		}
		return one(token.Pipe, "|")
	case '^':
		return one(token.Caret, "^")
	case '=':
		if l.peekAt(1) == '>' {
			return two(token.FatArrow, "=>")
		}
		if l.peekAt(1) == '=' {
			return two(token.Eq, "==")
		}
		return one(token.Assign, "=")
	case '<':
		if l.peekAt(1) == '<' {
			return two(token.Shl, "<<")
		}
		if l.peekAt(1) == '=' {
			return two(token.Lte, "<=")
		}
		return one(token.Lt, "<")
	case '>':
		if l.peekAt(1) == '>' {
			return two(token.Shr, ">>")
		}
		if l.peekAt(1) == '=' {
			return two(token.Gte, ">=")
		}
		return one(token.Gt, ">")
	case '(':
		return one(token.LParen, "(")
	case ')':
		return one(token.RParen, ")")
	case '{':
		return one(token.LBrace, "{")
	case '}':
		return one(token.RBrace, "}")
	case '[':
		return one(token.LBracket, "[")
	case ']':
		return one(token.RBracket, "]")
	case ',':
		return one(token.Comma, ",")
	case ':':
		return one(token.Colon, ":")
	case '.':
		// A bare '.' is field access (e.g. p.x). The leading-dot float form (.5)
		// and the trailing-dot float form (3.) are rejected earlier in next()/
		// lexNumber, so any '.' reaching here is a field-access dot.
		return one(token.Dot, ".")
	default:
		return l.errf(pos, "unexpected character %q", string(c))
	}
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || isDigit(c)
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
