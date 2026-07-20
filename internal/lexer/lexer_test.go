package lexer

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/token"
)

// kinds lexes src and returns the kinds (dropping the trailing EOF) or fails the
// test on a lex error.
func kinds(t *testing.T, src string) []token.Kind {
	t.Helper()
	toks, err := Lex(src, "test.wisp")
	if err != nil {
		t.Fatalf("Lex(%q) unexpected error: %v", src, err)
	}
	if len(toks) == 0 || toks[len(toks)-1].Kind != token.EOF {
		t.Fatalf("Lex(%q): expected trailing EOF token", src)
	}
	out := make([]token.Kind, 0, len(toks)-1)
	for _, tk := range toks[:len(toks)-1] {
		out = append(out, tk.Kind)
	}
	return out
}

func lexOK(t *testing.T, src string) []token.Token {
	t.Helper()
	toks, err := Lex(src, "test.wisp")
	if err != nil {
		t.Fatalf("Lex(%q) unexpected error: %v", src, err)
	}
	return toks
}

func wantKinds(t *testing.T, src string, want ...token.Kind) {
	t.Helper()
	got := kinds(t, src)
	if len(got) != len(want) {
		t.Fatalf("Lex(%q) kinds = %v, want %v", src, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Lex(%q) kinds = %v, want %v (mismatch at %d)", src, got, want, i)
		}
	}
}

func TestOperatorsAndPunctuation(t *testing.T) {
	wantKinds(t, "+ - * / % ! && || == != < <= > >= = -> ( ) { } , :",
		token.Plus, token.Minus, token.Star, token.Slash, token.Percent,
		token.Bang, token.AndAnd, token.OrOr, token.Eq, token.Neq,
		token.Lt, token.Lte, token.Gt, token.Gte, token.Assign, token.Arrow,
		token.LParen, token.RParen, token.LBrace, token.RBrace, token.Comma, token.Colon,
	)
}

func TestKeywordsAndIdents(t *testing.T) {
	wantKinds(t, "let fn return if else while for switch case default break continue true false",
		token.Let, token.Fn, token.Return, token.If, token.Else, token.While,
		token.For, token.Switch, token.Case, token.Default, token.Break,
		token.Continue, token.True, token.False,
	)
	wantKinds(t, "int bool string void",
		token.TypeInt, token.TypeBool, token.TypeString, token.TypeVoid)
	wantKinds(t, "foo _x x9 __ret main",
		token.Ident, token.Ident, token.Ident, token.Ident, token.Ident)
}

func TestIntLiterals(t *testing.T) {
	toks := lexOK(t, "0 42 100")
	want := []string{"0", "42", "100"}
	var got []string
	for _, tk := range toks {
		if tk.Kind == token.Int {
			got = append(got, tk.Lit)
		}
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("int lits = %v, want %v", got, want)
	}
	// leading '-' is a separate unary operator, NOT part of the literal
	wantKinds(t, "-7", token.Minus, token.Int)
}

func TestFloatLiterals(t *testing.T) {
	toks := lexOK(t, "3.14 0.5 12.0 100.001")
	want := []string{"3.14", "0.5", "12.0", "100.001"}
	var got []string
	for _, tk := range toks {
		if tk.Kind == token.FloatLit {
			got = append(got, tk.Lit)
		}
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("float lits = %v, want %v", got, want)
	}
	// leading '-' is a separate unary operator, NOT part of the literal
	wantKinds(t, "-2.0", token.Minus, token.FloatLit)
	// a bare int next to a float must not merge
	wantKinds(t, "2 3.5", token.Int, token.FloatLit)
}

func TestFloatLiteralRejectsTrailingDot(t *testing.T) {
	err := lexErr(t, "3.")
	if !strings.Contains(err.Error(), "test.wisp:1:2") {
		t.Fatalf("trailing-dot error missing/incorrect position: %v", err)
	}
}

func TestFloatLiteralRejectsLeadingDot(t *testing.T) {
	err := lexErr(t, ".5")
	if !strings.Contains(err.Error(), "test.wisp:1:1") {
		t.Fatalf("leading-dot error missing/incorrect position: %v", err)
	}
}

func TestFloatLiteralRejectsNonDigitAfterDot(t *testing.T) {
	lexErr(t, "3.x")
}

func TestFloatLiteralRejectsExponent(t *testing.T) {
	// "3e5" hits the integer-position reject (lexNumber); "3.14e5"/"3.14E2" hit
	// the float-position reject. Both must report the exponent limitation without
	// leaking the internal milestone codename into user-facing text.
	for _, src := range []string{"3e5", "3.14e5", "3.14E2"} {
		err := lexErr(t, src)
		if !strings.Contains(err.Error(), "exponent form is not supported") {
			t.Fatalf("%q: exponent error text unexpected: %v", src, err)
		}
		if strings.Contains(err.Error(), "M3") {
			t.Fatalf("%q: exponent error leaks internal milestone codename: %v", src, err)
		}
	}
}

func TestFloatLiteralRejectsSecondDot(t *testing.T) {
	lexErr(t, "3.1.4")
}

func TestLineCommentSkipped(t *testing.T) {
	// comment is dropped; the newline after it is still a separator
	wantKinds(t, "let // a comment\nx",
		token.Let, token.Separator, token.Ident)
	wantKinds(t, "// whole line\n", token.Separator)
}

func TestSeparators(t *testing.T) {
	wantKinds(t, "a;b", token.Ident, token.Separator, token.Ident)
	wantKinds(t, "a\nb", token.Ident, token.Separator, token.Ident)
	// CRLF: the \n is the separator; \r is consumed
	wantKinds(t, "a\r\nb", token.Ident, token.Separator, token.Ident)
}

func TestSingleQuotedString(t *testing.T) {
	toks := lexOK(t, `'hello'`)
	if toks[0].Kind != token.String || toks[0].Lit != "hello" {
		t.Fatalf("got %v / %q", toks[0].Kind, toks[0].Lit)
	}
	// only \' and \\ escapes
	toks = lexOK(t, `'a\'b\\c'`)
	if toks[0].Lit != `a'b\c` {
		t.Fatalf("single-quote escapes decoded to %q", toks[0].Lit)
	}
	// a backslash before any other char in a single-quoted string is literal
	toks = lexOK(t, `'a\nb'`)
	if toks[0].Lit != `a\nb` {
		t.Fatalf("single-quote backslash-n decoded to %q, want literal backslash-n", toks[0].Lit)
	}
}

func TestDoubleQuotedStringPlain(t *testing.T) {
	// "hi" -> StringStart, StringText("hi"), StringEnd
	wantKinds(t, `"hi"`, token.StringStart, token.StringText, token.StringEnd)
	toks := lexOK(t, `"hi"`)
	if toks[1].Kind != token.StringText || toks[1].Lit != "hi" {
		t.Fatalf("text chunk = %v / %q", toks[1].Kind, toks[1].Lit)
	}
}

func TestDoubleQuotedEscapes(t *testing.T) {
	toks := lexOK(t, `"a\nb\tc\"d\\e\$f"`)
	// single text chunk
	var text string
	for _, tk := range toks {
		if tk.Kind == token.StringText {
			text += tk.Lit
		}
	}
	want := "a\nb\tc\"d\\e$f"
	if text != want {
		t.Fatalf("decoded = %q, want %q", text, want)
	}
}

func TestInterpolation(t *testing.T) {
	// "count: ${n}" ->
	//   StringStart, StringText("count: "), InterpOpen, Ident(n), InterpClose, StringEnd
	wantKinds(t, `"count: ${n}"`,
		token.StringStart, token.StringText, token.InterpOpen,
		token.Ident, token.InterpClose, token.StringEnd)

	// expression inside ${ } uses the normal token stream
	wantKinds(t, `"sum: ${a + b}"`,
		token.StringStart, token.StringText, token.InterpOpen,
		token.Ident, token.Plus, token.Ident, token.InterpClose, token.StringEnd)

	// leading interpolation with no preceding text emits no empty StringText
	wantKinds(t, `"${x}"`,
		token.StringStart, token.InterpOpen, token.Ident, token.InterpClose, token.StringEnd)

	// text after interpolation
	wantKinds(t, `"${x}!"`,
		token.StringStart, token.InterpOpen, token.Ident, token.InterpClose,
		token.StringText, token.StringEnd)

	// \$ suppresses interpolation: \${x} is the literal text ${x}
	toks := lexOK(t, `"\${x}"`)
	var text string
	for _, tk := range toks {
		if tk.Kind == token.InterpOpen {
			t.Fatalf(`\${x} should not open an interpolation`)
		}
		if tk.Kind == token.StringText {
			text += tk.Lit
		}
	}
	if text != "${x}" {
		t.Fatalf(`\${x} decoded to %q, want "${x}"`, text)
	}
}

func TestPositions(t *testing.T) {
	// line/col tracking, 1-based
	toks := lexOK(t, "let\n  x")
	if toks[0].Pos.Line != 1 || toks[0].Pos.Col != 1 {
		t.Fatalf("let at %v, want 1:1", toks[0].Pos)
	}
	// toks: let, separator(\n), x
	x := toks[2]
	if x.Kind != token.Ident || x.Pos.Line != 2 || x.Pos.Col != 3 {
		t.Fatalf("x at %v (kind %v), want 2:3", x.Pos, x.Kind)
	}
	if toks[0].Pos.File != "test.wisp" {
		t.Fatalf("file = %q, want test.wisp", toks[0].Pos.File)
	}
}

func lexErr(t *testing.T, src string) error {
	t.Helper()
	_, err := Lex(src, "test.wisp")
	if err == nil {
		t.Fatalf("Lex(%q): expected error, got none", src)
	}
	return err
}

func TestErrorBadEscapeDouble(t *testing.T) {
	err := lexErr(t, `"a\qb"`)
	if !strings.Contains(err.Error(), "test.wisp:1:") {
		t.Fatalf("bad-escape error missing position: %v", err)
	}
}

func TestErrorNoBackslashCharInDoubleString(t *testing.T) {
	// \r is not in the allowed double-quote escape set
	lexErr(t, `"a\rb"`)
}

func TestErrorNUL(t *testing.T) {
	lexErr(t, "'a\x00b'")
	lexErr(t, "\"a\x00b\"")
}

func TestErrorUnterminatedString(t *testing.T) {
	lexErr(t, `'abc`)
	lexErr(t, `"abc`)
	lexErr(t, `"abc${x}`) // unterminated after interpolation
}

func TestErrorUnterminatedInterp(t *testing.T) {
	lexErr(t, `"${x"`) // missing }
}

func TestErrorBadByte(t *testing.T) {
	// a non-ASCII byte where a token is expected is a lex error with position
	err := lexErr(t, "let \xc3\xa9 = 1") // 'é' in UTF-8
	if !strings.Contains(err.Error(), "test.wisp:1:5") {
		t.Fatalf("bad-byte error position = %v, want test.wisp:1:5", err)
	}
	// stray '@'
	lexErr(t, "a @ b")
}

func TestErrorIdentCannotStartWithDigit(t *testing.T) {
	// "9x" lexes as Int(9) then Ident(x); not an error, just two tokens
	wantKinds(t, "9x", token.Int, token.Ident)
}

func TestFunctionStream(t *testing.T) {
	src := "fn add(a: int, b: int) -> int {\n  return a + b\n}"
	wantKinds(t, src,
		token.Fn, token.Ident, token.LParen,
		token.Ident, token.Colon, token.TypeInt, token.Comma,
		token.Ident, token.Colon, token.TypeInt, token.RParen,
		token.Arrow, token.TypeInt, token.LBrace, token.Separator,
		token.Return, token.Ident, token.Plus, token.Ident, token.Separator,
		token.RBrace,
	)
}

func TestBitwiseTokens(t *testing.T) {
	cases := []struct {
		src  string
		want []token.Kind
	}{
		{"a & b", []token.Kind{token.Ident, token.Amp, token.Ident}},
		{"a | b", []token.Kind{token.Ident, token.Pipe, token.Ident}},
		{"a ^ b", []token.Kind{token.Ident, token.Caret, token.Ident}},
		{"a << b", []token.Kind{token.Ident, token.Shl, token.Ident}},
		{"a >> b", []token.Kind{token.Ident, token.Shr, token.Ident}},
		{"a && b", []token.Kind{token.Ident, token.AndAnd, token.Ident}}, // still works
		{"a || b", []token.Kind{token.Ident, token.OrOr, token.Ident}},   // still works
		{"a < b", []token.Kind{token.Ident, token.Lt, token.Ident}},      // lone < still Lt
		{"a <= b", []token.Kind{token.Ident, token.Lte, token.Ident}},    // <= still Lte
		{"a > b", []token.Kind{token.Ident, token.Gt, token.Ident}},      // lone > still Gt
		{"a >= b", []token.Kind{token.Ident, token.Gte, token.Ident}},    // >= still Gte (not Shr+=)
	}
	for _, c := range cases {
		t.Run(c.src, func(t *testing.T) {
			got := kinds(t, c.src)
			if len(got) != len(c.want) {
				t.Fatalf("kinds = %v, want %v", got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Fatalf("kinds[%d] = %v, want %v (full: got %v want %v)", i, got[i], c.want[i], got, c.want)
				}
			}
		})
	}
}
