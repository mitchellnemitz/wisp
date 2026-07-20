package lexer

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/token"
)

func TestBracketsAndDot(t *testing.T) {
	wantKinds(t, "[ ] .",
		token.LBracket, token.RBracket, token.Dot,
	)
}

func TestFieldAccessTokens(t *testing.T) {
	// p.x must lex as Ident Dot Ident (the dot is field access, not a float).
	wantKinds(t, "p.x", token.Ident, token.Dot, token.Ident)
}

func TestArrayIndexTokens(t *testing.T) {
	wantKinds(t, "xs[0]", token.Ident, token.LBracket, token.Int, token.RBracket)
}

func TestFloatStillLexesNotDotField(t *testing.T) {
	// 3.14 stays a single float literal; the dot is consumed by the number lexer.
	wantKinds(t, "3.14", token.FloatLit)
}

func TestDictLitTokens(t *testing.T) {
	// { 'a': 1 } lexes braces, a string, a colon, an int, a brace.
	wantKinds(t, "{ 'a': 1 }",
		token.LBrace, token.String, token.Colon, token.Int, token.RBrace,
	)
}

func TestDictTypeTokens(t *testing.T) {
	wantKinds(t, "{string: int}",
		token.LBrace, token.TypeString, token.Colon, token.TypeInt, token.RBrace,
	)
}
