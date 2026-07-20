// Package token defines the lexical tokens of the wisp language.
package token

import "strconv"

// Kind is the category of a token.
type Kind int

const (
	// Illegal is a token that could not be lexed (used in error positions).
	Illegal Kind = iota
	// EOF marks the end of input.
	EOF

	// Separator is a statement separator: a newline or ';'. The parser treats
	// both equivalently.
	Separator

	// Ident is an identifier matching [A-Za-z_][A-Za-z0-9_]*.
	Ident
	// Int is an integer literal (decimal digits, no sign).
	Int
	// FloatLit is a float literal: <digits>.<digits> (at least one digit on each
	// side of the dot, no sign, no exponent). Lit holds the source text.
	FloatLit
	// String is a single-quoted string literal (no interpolation). Lit holds the
	// decoded bytes.
	String

	// StringStart opens a double-quoted string. The sequence of tokens that
	// follows is zero or more StringText / interpolation groups, terminated by
	// StringEnd.
	StringStart
	// StringText is a literal text chunk inside a double-quoted string. Lit holds
	// the decoded bytes.
	StringText
	// InterpOpen opens a ${ ... } interpolation inside a double-quoted string.
	// The tokens of the embedded expression follow, terminated by InterpClose.
	InterpOpen
	// InterpClose closes a ${ ... } interpolation.
	InterpClose
	// StringEnd terminates a double-quoted string.
	StringEnd

	// Keywords.
	Let
	Fn
	Return
	If
	Else
	While
	For
	Switch
	Case
	Default
	Break
	Continue
	Match
	True
	False
	// Test introduces a `test ("name") { ... }` declaration. It is a keyword
	// everywhere (so the name is reserved), but a `test` declaration is only
	// accepted by the parser in a file whose name ends in `_test.wisp`.
	Test

	// Type-name keywords.
	TypeInt
	TypeBool
	TypeString
	TypeVoid
	// Float is the `float` type-name keyword (M3). It was reserved in M1 and is
	// now an active type name, parallel to TypeInt/TypeBool/TypeString.
	Float

	// Reserved-for-later keywords (recognized but not yet usable in M1).
	Struct
	Enum
	Try
	Catch
	Finally
	Throw
	Error
	Const
	Final
	Import
	Export
	Include
	// Type introduces a `type Name = T` transparent type-alias declaration. It is
	// a declaration keyword like Struct/Enum/Const (no parenthesized head), not a
	// type-name keyword like TypeInt/TypeBool.
	Type

	// Operators.
	Plus     // +
	Minus    // -
	Star     // *
	Slash    // /
	Percent  // %
	Bang     // !
	AndAnd   // &&
	OrOr     // ||
	Eq       // ==
	Neq      // !=
	Lt       // <
	Lte      // <=
	Gt       // >
	Gte      // >=
	Assign   // =
	Arrow    // ->
	FatArrow // =>
	Amp      // &
	Pipe     // |
	Caret    // ^
	Shl      // <<
	Shr      // >>

	// Punctuation.
	LParen   // (
	RParen   // )
	LBrace   // {
	RBrace   // }
	LBracket // [
	RBracket // ]
	Comma    // ,
	Colon    // :
	Dot      // .
)

// Position is a source location. File may be empty.
type Position struct {
	File string
	Line int
	Col  int
}

// String renders the position as file:line:col, or line:col if File is empty.
func (p Position) String() string {
	if p.File == "" {
		return strconv.Itoa(p.Line) + ":" + strconv.Itoa(p.Col)
	}
	return p.File + ":" + strconv.Itoa(p.Line) + ":" + strconv.Itoa(p.Col)
}

// Token is a lexed token.
type Token struct {
	Kind Kind
	Lit  string
	Pos  Position
}

var keywords = map[string]Kind{
	"let":      Let,
	"fn":       Fn,
	"return":   Return,
	"if":       If,
	"else":     Else,
	"while":    While,
	"for":      For,
	"switch":   Switch,
	"case":     Case,
	"default":  Default,
	"break":    Break,
	"continue": Continue,
	"match":    Match,
	"true":     True,
	"false":    False,
	"test":     Test,

	"int":    TypeInt,
	"bool":   TypeBool,
	"string": TypeString,
	"void":   TypeVoid,

	"struct":  Struct,
	"enum":    Enum,
	"float":   Float,
	"try":     Try,
	"catch":   Catch,
	"finally": Finally,
	"throw":   Throw,
	"error":   Error,
	"const":   Const,
	"final":   Final,
	"import":  Import,
	"export":  Export,
	"include": Include,
	"type":    Type,
}

// Lookup returns the keyword Kind for lit and true if lit is a keyword,
// otherwise it returns (Ident, false).
func Lookup(lit string) (Kind, bool) {
	if k, ok := keywords[lit]; ok {
		return k, true
	}
	return Ident, false
}

var kindNames = map[Kind]string{
	Illegal:     "Illegal",
	EOF:         "EOF",
	Separator:   "Separator",
	Ident:       "Ident",
	Int:         "Int",
	FloatLit:    "FloatLit",
	String:      "String",
	StringStart: "StringStart",
	StringText:  "StringText",
	InterpOpen:  "InterpOpen",
	InterpClose: "InterpClose",
	StringEnd:   "StringEnd",
	Let:         "let",
	Fn:          "fn",
	Return:      "return",
	If:          "if",
	Else:        "else",
	While:       "while",
	For:         "for",
	Switch:      "switch",
	Case:        "case",
	Default:     "default",
	Break:       "break",
	Continue:    "continue",
	Match:       "match",
	True:        "true",
	False:       "false",
	Test:        "test",
	TypeInt:     "int",
	TypeBool:    "bool",
	TypeString:  "string",
	TypeVoid:    "void",
	Struct:      "struct",
	Enum:        "enum",
	Float:       "float",
	Try:         "try",
	Catch:       "catch",
	Finally:     "finally",
	Throw:       "throw",
	Error:       "error",
	Const:       "const",
	Final:       "final",
	Import:      "import",
	Export:      "export",
	Include:     "include",
	Type:        "type",
	Plus:        "+",
	Minus:       "-",
	Star:        "*",
	Slash:       "/",
	Percent:     "%",
	Bang:        "!",
	AndAnd:      "&&",
	OrOr:        "||",
	Eq:          "==",
	Neq:         "!=",
	Lt:          "<",
	Lte:         "<=",
	Gt:          ">",
	Gte:         ">=",
	Assign:      "=",
	Arrow:       "->",
	FatArrow:    "=>",
	Amp:         "&",
	Pipe:        "|",
	Caret:       "^",
	Shl:         "<<",
	Shr:         ">>",
	LParen:      "(",
	RParen:      ")",
	LBrace:      "{",
	RBrace:      "}",
	LBracket:    "[",
	RBracket:    "]",
	Comma:       ",",
	Colon:       ":",
	Dot:         ".",
}

// String returns a human-readable name for the kind.
func (k Kind) String() string {
	if s, ok := kindNames[k]; ok {
		return s
	}
	return "Kind(" + strconv.Itoa(int(k)) + ")"
}
