// Package parser is a recursive-descent parser producing an *ast.Program.
package parser

import (
	"fmt"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/lexer"
	"github.com/mitchellnemitz/wisp/internal/token"
)

// Error is a parse error with a source position.
type Error struct {
	Pos token.Position
	Msg string
}

func (e *Error) Error() string {
	return e.Pos.String() + ": " + e.Msg
}

// Parse lexes and parses src into a Program. It returns the first error
// encountered (a lexer error or a parse error), each carrying a source
// position. The filename is recorded in positions.
func Parse(src, filename string) (*ast.Program, error) {
	toks, err := lexer.Lex(src, filename)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks, file: filename}
	return p.parseProgram()
}

// ParseWithComments parses src exactly as Parse does and additionally returns
// the retained `//` line comments (lexer side channel) in source order, for the
// formatter (B1). Comments never reach the parser's token consumption, so the
// resulting *ast.Program is identical to Parse's. On any error the returned
// comment slice is nil.
func ParseWithComments(src, filename string) (*ast.Program, []lexer.Comment, error) {
	toks, comments, err := lexer.LexWithComments(src, filename)
	if err != nil {
		return nil, nil, err
	}
	p := &parser{toks: toks, file: filename}
	prog, err := p.parseProgram()
	if err != nil {
		return nil, nil, err
	}
	return prog, comments, nil
}

type parser struct {
	toks []token.Token
	pos  int
	file string
	// noStructLit suppresses parsing `Ident { ... }` as a struct construction
	// while true. It is set in contexts where a following `{` opens a block, not
	// a struct body -- specifically switch case values, where `case X { ... }`
	// would otherwise be read as a struct literal. Restored after the value.
	noStructLit bool
	// depth tracks recursive-descent nesting so enterDepth can reject input
	// that would otherwise overflow the Go call stack with an uncatchable
	// fatal error (P2-1).
	depth int
}

// maxParseDepth bounds recursive-descent nesting (parseExpr, parseUnary,
// parseTypeAtom, parseBlock). Deeply nested input beyond this cap is
// rejected with a parse error instead of crashing the process.
const maxParseDepth = 5000

func (p *parser) cur() token.Token {
	return p.toks[p.pos]
}

// enterDepth increments the recursion-depth counter and returns a closure
// that decrements it, meant to be used as `defer done()` immediately at the
// top of any function whose recursion an attacker could drive unbounded.
// The returned closure is always non-nil; call it unconditionally via
// defer, even when enterDepth also returns an error.
func (p *parser) enterDepth() (func(), error) {
	p.depth++
	if p.depth > maxParseDepth {
		return func() { p.depth-- }, p.errHere("nesting too deep")
	}
	return func() { p.depth-- }, nil
}

func (p *parser) curKind() token.Kind {
	return p.toks[p.pos].Kind
}

func (p *parser) peekKind() token.Kind {
	if p.pos+1 < len(p.toks) {
		return p.toks[p.pos+1].Kind
	}
	return token.EOF
}

func (p *parser) advance() token.Token {
	t := p.toks[p.pos]
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	return t
}

func (p *parser) errf(pos token.Position, format string, args ...any) error {
	return &Error{Pos: pos, Msg: fmt.Sprintf(format, args...)}
}

func (p *parser) errHere(format string, args ...any) error {
	return p.errf(p.cur().Pos, format, args...)
}

// expect consumes the current token if it matches kind, else returns an error.
func (p *parser) expect(kind token.Kind) (token.Token, error) {
	if p.curKind() != kind {
		return token.Token{}, p.errHere("expected %s, got %s", kind, p.describe(p.cur()))
	}
	return p.advance(), nil
}

func (p *parser) describe(t token.Token) string {
	switch t.Kind {
	case token.EOF:
		return "end of input"
	case token.Separator:
		return "end of statement"
	case token.Ident, token.Int:
		return fmt.Sprintf("%s %q", t.Kind, t.Lit)
	default:
		return t.Kind.String()
	}
}

// skipSeparators consumes any run of statement separators.
func (p *parser) skipSeparators() {
	for p.curKind() == token.Separator {
		p.advance()
	}
}

// skipSeparatorsNL consumes any run of statement separators and reports whether
// at least one of them was a real newline (`Lit == "\n"`) rather than a
// semicolon. It also reports whether any separator was consumed at all. This
// lets literal/decl parsers set their Multiline flag for newline layout only,
// so a one-line `;`-separated form is not flagged as multi-line.
func (p *parser) skipSeparatorsNL() (sawNL, sawSep bool) {
	for p.curKind() == token.Separator {
		if p.cur().Lit == "\n" {
			sawNL = true
		}
		sawSep = true
		p.advance()
	}
	return sawNL, sawSep
}

func (p *parser) parseProgram() (*ast.Program, error) {
	prog := &ast.Program{File: p.file}
	p.skipSeparators()
	for p.curKind() != token.EOF {
		switch p.curKind() {
		case token.Fn:
			fn, err := p.parseFuncDecl()
			if err != nil {
				return nil, err
			}
			prog.Funcs = append(prog.Funcs, fn)
		case token.Struct:
			sd, err := p.parseStructDecl()
			if err != nil {
				return nil, err
			}
			prog.Structs = append(prog.Structs, sd)
		case token.Enum:
			ed, err := p.parseEnumDecl()
			if err != nil {
				return nil, err
			}
			prog.Enums = append(prog.Enums, ed)
		case token.Type:
			ta, err := p.parseTypeAliasDecl()
			if err != nil {
				return nil, err
			}
			prog.Aliases = append(prog.Aliases, ta)
		case token.Export:
			if err := p.parseExportDecl(prog); err != nil {
				return nil, err
			}
		case token.Import:
			imp, err := p.parseImportDecl()
			if err != nil {
				return nil, err
			}
			prog.Imports = append(prog.Imports, imp)
		case token.Include:
			inc, err := p.parseIncludeDecl()
			if err != nil {
				return nil, err
			}
			prog.Includes = append(prog.Includes, inc)
		case token.Const:
			cd, err := p.parseConstDecl()
			if err != nil {
				return nil, err
			}
			prog.Consts = append(prog.Consts, cd)
		case token.Test:
			td, err := p.parseTest()
			if err != nil {
				return nil, err
			}
			prog.Tests = append(prog.Tests, td)
		case token.Final:
			return nil, p.errHere("`final` is a function-local binding; use `const` at module scope")
		default:
			return nil, p.errHere("expected a function, struct, enum, type, const, import, include, or export declaration, got %s", p.describe(p.cur()))
		}
		p.skipSeparators()
	}
	return prog, nil
}

// parseExportDecl parses `export fn ...`, `export struct ...`, `export const
// ...`, or `export enum ...`, setting the Exported flag and ExportPos on the
// resulting declaration. `export` modifying anything else is a located error.
func (p *parser) parseExportDecl(prog *ast.Program) error {
	kw := p.advance() // export
	switch p.curKind() {
	case token.Fn:
		fn, err := p.parseFuncDecl()
		if err != nil {
			return err
		}
		fn.Exported = true
		fn.ExportPos = kw.Pos
		prog.Funcs = append(prog.Funcs, fn)
		return nil
	case token.Struct:
		sd, err := p.parseStructDecl()
		if err != nil {
			return err
		}
		sd.Exported = true
		sd.ExportPos = kw.Pos
		prog.Structs = append(prog.Structs, sd)
		return nil
	case token.Const:
		cd, err := p.parseConstDecl()
		if err != nil {
			return err
		}
		if cd.Name == "_" {
			return p.errf(kw.Pos, "export const _ has no importable name; remove `export` or name the constant")
		}
		cd.Exported = true
		cd.ExportPos = kw.Pos
		prog.Consts = append(prog.Consts, cd)
		return nil
	case token.Enum:
		ed, err := p.parseEnumDecl()
		if err != nil {
			return err
		}
		ed.Exported = true
		ed.ExportPos = kw.Pos
		prog.Enums = append(prog.Enums, ed)
		return nil
	case token.Type:
		return p.errf(kw.Pos, "type aliases are module-local and cannot be exported")
	default:
		return p.errf(kw.Pos, "export must modify a function, struct, const, or enum, got %s", p.describe(p.cur()))
	}
}

// parseImportDecl parses `import "owner/repo" [as alias]`. The path is a plain
// (non-interpolated) string; an optional `as <ident>` clause overrides the
// namespace. The owner/repo string is validated by the loader, not here.
func (p *parser) parseImportDecl() (*ast.ImportDecl, error) {
	kw := p.advance() // import
	path, pathPos, err := p.parseQuotedPathString("import path")
	if err != nil {
		return nil, err
	}
	d := &ast.ImportDecl{KwPos: kw.Pos, PathPos: pathPos, Path: path}
	if aliasPos, alias, ok, err := p.parseOptionalAs(); err != nil {
		return nil, err
	} else if ok {
		d.AliasPos = aliasPos
		d.Alias = alias
	}
	return d, nil
}

// parseIncludeDecl parses `include "./rel/path.wisp" [as alias]`.
func (p *parser) parseIncludeDecl() (*ast.IncludeDecl, error) {
	kw := p.advance() // include
	path, pathPos, err := p.parseQuotedPathString("include path")
	if err != nil {
		return nil, err
	}
	d := &ast.IncludeDecl{KwPos: kw.Pos, PathPos: pathPos, Path: path}
	if aliasPos, alias, ok, err := p.parseOptionalAs(); err != nil {
		return nil, err
	} else if ok {
		d.AliasPos = aliasPos
		d.Alias = alias
	}
	return d, nil
}

// parseOptionalAs consumes an optional `as <ident>` clause following an import or
// include path. `as` is a contextual keyword (a plain Ident with literal "as").
// It returns (pos, alias, true, nil) when present, (zero, "", false, nil) when
// absent, or an error if `as` is present but not followed by an identifier.
func (p *parser) parseOptionalAs() (token.Position, string, bool, error) {
	if p.curKind() != token.Ident || p.cur().Lit != "as" {
		return token.Position{}, "", false, nil
	}
	p.advance() // as
	t := p.cur()
	switch t.Kind {
	case token.Ident, token.TypeInt, token.TypeBool, token.TypeString, token.Float, token.Error:
		p.advance()
		return t.Pos, t.Lit, true, nil
	default:
		return token.Position{}, "", false, p.errHere("expected Ident, got %s", p.describe(t))
	}
}

// parseQuotedPathString consumes a single- or double-quoted string literal that
// must be a plain literal (one text part, no interpolation) and returns its
// decoded bytes and position. `what` names the construct for error messages.
func (p *parser) parseQuotedPathString(what string) (string, token.Position, error) {
	switch p.curKind() {
	case token.String:
		t := p.advance()
		return t.Lit, t.Pos, nil
	case token.StringStart:
		start := p.advance()
		// Accept only StringText* with no interpolation, then StringEnd.
		var sb string
		for {
			switch p.curKind() {
			case token.StringText:
				sb += p.advance().Lit
			case token.StringEnd:
				p.advance()
				return sb, start.Pos, nil
			case token.InterpOpen:
				return "", start.Pos, p.errf(start.Pos, "%s must be a plain string, not an interpolated one", what)
			default:
				return "", start.Pos, p.errHere("malformed %s string: unexpected %s", what, p.describe(p.cur()))
			}
		}
	default:
		return "", p.cur().Pos, p.errHere("expected a quoted %s, got %s", what, p.describe(p.cur()))
	}
}

// parseStructDecl parses `struct Name { f: T, ... }` or
// `struct Name[T, U] { f: T, ... }`. Fields are comma- and/or
// newline-separated; a trailing separator is allowed.
func (p *parser) parseStructDecl() (*ast.StructDecl, error) {
	kw := p.advance() // struct
	name, err := p.expect(token.Ident)
	if err != nil {
		return nil, err
	}
	sd := &ast.StructDecl{KwPos: kw.Pos, NamePos: name.Pos, Name: name.Lit}
	if p.curKind() == token.LBracket {
		tps, err := p.parseStructTypeParams()
		if err != nil {
			return nil, err
		}
		sd.TypeParams = tps
	}
	if _, err := p.expect(token.LBrace); err != nil {
		return nil, err
	}
	sawNL, _ := p.skipSeparatorsNL()
	for p.curKind() != token.RBrace {
		if p.curKind() == token.EOF {
			return nil, p.errHere("expected '}', got end of input")
		}
		fname, err := p.expect(token.Ident)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.Colon); err != nil {
			return nil, err
		}
		ty, err := p.parseTypeName(false)
		if err != nil {
			return nil, err
		}
		sd.Fields = append(sd.Fields, ast.StructField{NamePos: fname.Pos, Name: fname.Lit, Type: ty})
		// fields separated by ',' and/or a statement separator; allow either.
		if p.curKind() == token.Comma {
			p.advance()
		}
		if nl, _ := p.skipSeparatorsNL(); nl {
			sawNL = true
		}
	}
	p.advance() // }
	sd.Multiline = sawNL
	return sd, nil
}

// parseTypeAliasDecl parses `type Name = T`, a transparent type-alias
// declaration. The RHS is a full type annotation (allowVoid=false: a bare `void`
// RHS is rejected, but a funcref `fn() -> void` RHS is fine). A `[` immediately
// after the name is a generic-alias attempt, rejected with a dedicated message.
func (p *parser) parseTypeAliasDecl() (*ast.TypeAliasDecl, error) {
	kw := p.advance() // type
	name, err := p.expect(token.Ident)
	if err != nil {
		return nil, err
	}
	if p.curKind() == token.LBracket {
		return nil, p.errf(name.Pos, "generic type aliases are not supported; parameterize the underlying struct instead")
	}
	if _, err := p.expect(token.Assign); err != nil {
		return nil, err
	}
	typePos := p.cur().Pos
	ty, err := p.parseTypeName(false)
	if err != nil {
		return nil, err
	}
	return &ast.TypeAliasDecl{KwPos: kw.Pos, NamePos: name.Pos, Name: name.Lit, TypePos: typePos, Type: ty}, nil
}

// parseEnumDecl parses `enum Name { V1[ = expr], V2, ... }`. Variants are comma-
// and/or newline-separated with an optional trailing comma; at least one variant
// is required (an empty body is a located error). An explicit `= <expr>` is
// parsed as a general expression here; the checker restricts it to an integer
// literal. Multiline is set for newline-only separation, matching parseStructDecl.
func (p *parser) parseEnumDecl() (*ast.EnumDecl, error) {
	kw := p.advance() // enum
	name, err := p.expect(token.Ident)
	if err != nil {
		return nil, err
	}
	ed := &ast.EnumDecl{KwPos: kw.Pos, NamePos: name.Pos, Name: name.Lit}
	if _, err := p.expect(token.LBrace); err != nil {
		return nil, err
	}
	sawNL, _ := p.skipSeparatorsNL()
	for p.curKind() != token.RBrace {
		if p.curKind() == token.EOF {
			return nil, p.errHere("expected '}', got end of input")
		}
		vname, err := p.expect(token.Ident)
		if err != nil {
			return nil, err
		}
		variant := ast.EnumVariant{Name: vname.Lit, NamePos: vname.Pos}
		if p.curKind() == token.Assign {
			p.advance() // =
			val, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			variant.Value = val
		}
		ed.Variants = append(ed.Variants, variant)
		// variants separated by ',' and/or a statement separator; allow either.
		if p.curKind() == token.Comma {
			p.advance()
		}
		if nl, _ := p.skipSeparatorsNL(); nl {
			sawNL = true
		}
	}
	if len(ed.Variants) == 0 {
		return nil, p.errHere("enum %s must declare at least one variant", ed.Name)
	}
	p.advance() // }
	ed.Multiline = sawNL
	return ed, nil
}

func (p *parser) parseFuncDecl() (*ast.FuncDecl, error) {
	kw, err := p.expect(token.Fn)
	if err != nil {
		return nil, err
	}
	name, err := p.expect(token.Ident)
	if err != nil {
		return nil, err
	}
	fn := &ast.FuncDecl{KwPos: kw.Pos, Name: name.Lit}

	if p.curKind() == token.LBracket {
		tps, bounds, err := p.parseTypeParams()
		if err != nil {
			return nil, err
		}
		fn.TypeParams = tps
		fn.TypeParamBounds = bounds
	}

	if _, err := p.expect(token.LParen); err != nil {
		return nil, err
	}
	params, err := p.parseParams()
	if err != nil {
		return nil, err
	}
	fn.Params = params
	if _, err := p.expect(token.RParen); err != nil {
		return nil, err
	}
	if _, err := p.expect(token.Arrow); err != nil {
		return nil, err
	}
	rt, err := p.parseTypeName(true)
	if err != nil {
		return nil, err
	}
	fn.RetType = rt

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	fn.Body = body
	return fn, nil
}

// parseTest parses `test ("name") { block }`. The head is a single plain string
// literal (no interpolation); the body is an ordinary block. A `test` declaration
// is only legal in a file whose name ends in `_test.wisp`; elsewhere it is a parse
// error (the `test` keyword is reserved everywhere, but the construct is gated).
func (p *parser) parseTest() (*ast.TestDecl, error) {
	kw := p.advance() // test
	if !strings.HasSuffix(p.file, "_test.wisp") {
		return nil, p.errf(kw.Pos, "`test` declarations are only allowed in a `*_test.wisp` file")
	}
	if _, err := p.expect(token.LParen); err != nil {
		return nil, err
	}
	name, namePos, err := p.parseQuotedPathString("test name")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.RParen); err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &ast.TestDecl{KwPos: kw.Pos, NamePos: namePos, Name: name, Body: body}, nil
}

// isReservedTypeWord reports whether name collides with a built-in type word.
// Used to reject such names as type parameters at parse time.
func isReservedTypeWord(name string) bool {
	switch name {
	case "int", "bool", "string", "float", "void", "error", "Optional", "Result", "RunResult", "Process":
		return true
	}
	return false
}

// parseTypeParams parses the optional `[T, U, ...]` type-parameter list after a
// function name. The list may not be empty, names must be unique, and a name
// may not collide with a built-in type word.
func (p *parser) parseTypeParams() (names []string, bounds map[string]string, err error) {
	lb := p.advance() // [
	seen := map[string]bool{}
	if p.curKind() == token.RBracket {
		return nil, nil, p.errf(lb.Pos, "type-parameter list cannot be empty")
	}
	for {
		id, err := p.expect(token.Ident)
		if err != nil {
			return nil, nil, err
		}
		if isReservedTypeWord(id.Lit) {
			return nil, nil, p.errf(id.Pos, "type parameter %q collides with a built-in type name", id.Lit)
		}
		if seen[id.Lit] {
			return nil, nil, p.errf(id.Pos, "type parameter %q is declared more than once", id.Lit)
		}
		seen[id.Lit] = true
		names = append(names, id.Lit)
		if p.curKind() == token.Colon {
			p.advance() // :
			b, err := p.expect(token.Ident)
			if err != nil {
				return nil, nil, err
			}
			if b.Lit != "comparable" && b.Lit != "numeric" {
				return nil, nil, p.errf(b.Pos, "unknown bound %q (only 'comparable' and 'numeric' are supported)", b.Lit)
			}
			if bounds == nil {
				bounds = map[string]string{}
			}
			bounds[id.Lit] = b.Lit
		}
		if p.curKind() != token.Comma {
			break
		}
		p.advance()
		if p.curKind() == token.RBracket {
			break
		}
	}
	if _, err := p.expect(token.RBracket); err != nil {
		return nil, nil, err
	}
	return names, bounds, nil
}

// parseStructTypeParams parses the `[T, U, ...]` type-parameter list on a
// struct declaration. Bounds are not supported for struct type params.
func (p *parser) parseStructTypeParams() ([]string, error) {
	lb := p.advance() // [
	seen := map[string]bool{}
	if p.curKind() == token.RBracket {
		return nil, p.errf(lb.Pos, "type-parameter list cannot be empty")
	}
	var names []string
	for {
		id, err := p.expect(token.Ident)
		if err != nil {
			return nil, err
		}
		if isReservedTypeWord(id.Lit) {
			return nil, p.errf(id.Pos, "type parameter %q collides with a built-in type name", id.Lit)
		}
		if seen[id.Lit] {
			return nil, p.errf(id.Pos, "type parameter %q is declared more than once", id.Lit)
		}
		seen[id.Lit] = true
		names = append(names, id.Lit)
		if p.curKind() != token.Comma {
			break
		}
		p.advance()
		if p.curKind() == token.RBracket {
			break
		}
	}
	if _, err := p.expect(token.RBracket); err != nil {
		return nil, err
	}
	return names, nil
}

func (p *parser) parseParams() ([]ast.Param, error) {
	var params []ast.Param
	if p.curKind() == token.RParen {
		return params, nil
	}
	for {
		name, err := p.expect(token.Ident)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.Colon); err != nil {
			return nil, err
		}
		ty, err := p.parseTypeName(false)
		if err != nil {
			return nil, err
		}
		param := ast.Param{NamePos: name.Pos, Name: name.Lit, Type: ty}
		if p.curKind() == token.Assign {
			p.advance()
			def, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			param.Default = def
		}
		params = append(params, param)
		if p.curKind() != token.Comma {
			break
		}
		p.advance()
		if p.curKind() == token.RParen {
			break
		}
	}
	return params, nil
}

// parseTypeName parses a type annotation. allowVoid permits void (return
// types). It parses a base type atom, then applies the postfix `[]` array
// loop: each trailing empty-bracket pair wraps the current type in an array
// (`T[]`, `T[][]`, ...). A `[` in postfix position not immediately followed by
// `]` is left unconsumed for the caller to reject (it is not an array marker,
// and in type position it is nothing else).
func (p *parser) parseTypeName(allowVoid bool) (ast.TypeName, error) {
	t, err := p.parseTypeAtom(allowVoid)
	if err != nil {
		return "", err
	}
	for p.curKind() == token.LBracket && p.peekKind() == token.RBracket {
		p.advance() // [
		p.advance() // ]
		t = ast.ArrayType(t)
	}
	return t, nil
}

// parseTypeAtom parses a single base type: a scalar keyword, a dict, a
// parenthesized group or tuple, a funcref, or an ident/generic/qualified name.
// A leading `[` in type position is a parse error (arrays are postfix `T[]`);
// the postfix `[]` loop in parseTypeName runs on whatever this returns.
func (p *parser) parseTypeAtom(allowVoid bool) (ast.TypeName, error) {
	done, err := p.enterDepth()
	defer done()
	if err != nil {
		return "", err
	}
	switch p.curKind() {
	case token.TypeInt:
		p.advance()
		return ast.TypeInt, nil
	case token.Float:
		p.advance()
		return ast.TypeFloat, nil
	case token.TypeBool:
		p.advance()
		return ast.TypeBool, nil
	case token.TypeString:
		p.advance()
		return ast.TypeString, nil
	case token.Error:
		// `error` is a built-in handle type name (M5), usable in let/param/return
		// annotations. It encodes to the literal "error" Type (the checker treats it
		// as a reserved handle type).
		p.advance()
		return ast.TypeName("error"), nil
	case token.TypeVoid:
		if !allowVoid {
			return "", p.errHere("void is only valid as a return type")
		}
		p.advance()
		return ast.TypeVoid, nil
	case token.LBrace:
		// dict type `{K: V}` (M3 PR-C). K is validated as int/string by the
		// checker; here we just parse two type annotations around the ':'.
		p.advance()
		key, err := p.parseTypeName(false)
		if err != nil {
			return "", err
		}
		if _, err := p.expect(token.Colon); err != nil {
			return "", err
		}
		val, err := p.parseTypeName(false)
		if err != nil {
			return "", err
		}
		if _, err := p.expect(token.RBrace); err != nil {
			return "", err
		}
		return ast.DictType(key, val), nil
	case token.LParen:
		lparen := p.advance() // (
		// Empty () is not a type (distinct diagnostic per spec section 4.1).
		if p.curKind() == token.RParen {
			return "", p.errf(lparen.Pos, "empty tuple is not a type")
		}
		first, err := p.parseTypeName(false) // element types cannot be void
		if err != nil {
			return "", err
		}
		// A single element followed immediately by ) is a parenthesized group:
		// it unwraps to the inner type (NOT a 1-tuple). The postfix `[]` loop in
		// parseTypeName then applies, so `(int)[]` == `int[]` and
		// `(fn(int) -> string)[]` is an array of funcrefs.
		if p.curKind() == token.RParen {
			p.advance()
			return first, nil
		}
		// First element must be followed by a comma.
		if p.curKind() != token.Comma {
			return "", p.errHere("expected ',' or ')' in tuple type")
		}
		p.advance() // consume the comma after the first element
		// Single element with trailing comma: (T,) -- also rejected.
		if p.curKind() == token.RParen {
			p.advance()
			return "", p.errf(lparen.Pos, "tuple type requires at least two element types")
		}
		elems := []ast.TypeName{first}
		second, err := p.parseTypeName(false)
		if err != nil {
			return "", err
		}
		elems = append(elems, second)
		for p.curKind() == token.Comma {
			p.advance()
			if p.curKind() == token.RParen {
				break // trailing comma
			}
			elem, err := p.parseTypeName(false)
			if err != nil {
				return "", err
			}
			elems = append(elems, elem)
		}
		if _, err := p.expect(token.RParen); err != nil {
			return "", err
		}
		return ast.TupleType(elems), nil
	case token.Fn:
		// a function-reference type `fn(T1, T2, ...) -> R` (M4). Params are
		// comma-separated and may be nested composites; void is rejected as a
		// PARAMETER type but allowed as the return type R.
		return p.parseFuncType()
	case token.Ident:
		if p.cur().Lit == "Optional" && p.peekKind() == token.LBracket {
			p.advance()                         // Optional
			p.advance()                         // [
			elem, err := p.parseTypeName(false) // element cannot be void
			if err != nil {
				return "", err
			}
			if _, err := p.expect(token.RBracket); err != nil {
				return "", err
			}
			return ast.OptionalType(elem), nil
		}
		if p.cur().Lit == "Result" && p.peekKind() == token.LBracket {
			p.advance()                         // Result
			p.advance()                         // [
			elem, err := p.parseTypeName(false) // success type cannot be void
			if err != nil {
				return "", err
			}
			if _, err := p.expect(token.RBracket); err != nil {
				return "", err
			}
			return ast.ResultType(elem), nil
		}
		// a named struct type, possibly generic (Box[int]), or a qualified `ns.Type`
		// (M8), or both together (`ns.Type[args]`, M9). Compute the (possibly
		// dotted) base name first, then check for a following non-empty `[...]`.
		// A trailing EMPTY `[]` is the postfix array marker, not a generic arg
		// list, so `Name[]`/`ns.Name[]` falls through to the postfix loop as
		// array-of-Name; only a NON-empty `[...]` is generic instantiation.
		t := p.advance()
		base := t.Lit
		if p.curKind() == token.Dot {
			p.advance() // .
			second, err := p.expect(token.Ident)
			if err != nil {
				return "", err
			}
			base = t.Lit + "." + second.Lit
		}
		if p.curKind() == token.LBracket && p.peekKind() != token.RBracket {
			p.advance() // [
			var args []ast.TypeName
			for {
				arg, err := p.parseTypeName(false)
				if err != nil {
					return "", err
				}
				args = append(args, arg)
				if p.curKind() != token.Comma {
					break
				}
				p.advance()
				if p.curKind() == token.RBracket {
					break
				}
			}
			if _, err := p.expect(token.RBracket); err != nil {
				return "", err
			}
			s := base + "["
			for i, a := range args {
				if i > 0 {
					s += ","
				}
				s += string(a)
			}
			return ast.TypeName(s + "]"), nil
		}
		return ast.TypeName(base), nil
	default:
		return "", p.errHere("expected a type name, got %s", p.describe(p.cur()))
	}
}

// parseFuncType parses a function-reference type annotation `fn(T...) -> R`
// after the leading `fn` token is the current token. Parameter types may not be
// void (mirroring the array-element/dict-value void guard); the return type R
// may be void. Both params and R may be nested composites.
func (p *parser) parseFuncType() (ast.TypeName, error) {
	p.advance() // fn
	if _, err := p.expect(token.LParen); err != nil {
		return "", err
	}
	var params []ast.TypeName
	if p.curKind() != token.RParen {
		for {
			pt, err := p.parseTypeName(false) // a parameter type cannot be void
			if err != nil {
				return "", err
			}
			params = append(params, pt)
			if p.curKind() != token.Comma {
				break
			}
			p.advance()
		}
	}
	if _, err := p.expect(token.RParen); err != nil {
		return "", err
	}
	if _, err := p.expect(token.Arrow); err != nil {
		return "", err
	}
	ret, err := p.parseTypeName(true) // a function's return type may be void
	if err != nil {
		return "", err
	}
	return ast.FuncType(params, ret), nil
}

// parseBlock parses `{ stmt* }`.
func (p *parser) parseBlock() ([]ast.Stmt, error) {
	done, err := p.enterDepth()
	defer done()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.LBrace); err != nil {
		return nil, err
	}
	var stmts []ast.Stmt
	p.skipSeparators()
	for p.curKind() != token.RBrace {
		if p.curKind() == token.EOF {
			return nil, p.errHere("expected '}', got end of input")
		}
		s, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, s)
		// A statement must be followed by a separator or the closing brace.
		if p.curKind() != token.RBrace {
			if p.curKind() != token.Separator {
				return nil, p.errHere("expected end of statement, got %s", p.describe(p.cur()))
			}
			p.skipSeparators()
		}
	}
	p.advance() // }
	return stmts, nil
}

func (p *parser) parseStmt() (ast.Stmt, error) {
	switch p.curKind() {
	case token.Let:
		return p.parseLet()
	case token.Const:
		return p.parseConstStmt()
	case token.Final:
		return p.parseFinalStmt()
	case token.Return:
		return p.parseReturn()
	case token.If:
		return p.parseIf()
	case token.While:
		return p.parseWhile()
	case token.For:
		return p.parseFor()
	case token.Switch:
		return p.parseSwitch()
	case token.Match:
		return p.parseMatch()
	case token.Break:
		t := p.advance()
		return &ast.BreakStmt{KwPos: t.Pos}, nil
	case token.Continue:
		t := p.advance()
		return &ast.ContinueStmt{KwPos: t.Pos}, nil
	case token.Throw:
		return p.parseThrow()
	case token.Try:
		return p.parseTry()
	default:
		return p.parseSimpleStmt()
	}
}

// parseThrow parses `throw <expr>` (M5). The operand is any expression; the
// checker requires it to be of type error.
func (p *parser) parseThrow() (ast.Stmt, error) {
	kw := p.advance() // throw
	x, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &ast.ThrowStmt{KwPos: kw.Pos, X: x}, nil
}

// parseTry parses `try { body } catch (e) { handler } [finally { cleanup }]`
// (M5). The catch clause is required; finally is optional.
func (p *parser) parseTry() (ast.Stmt, error) {
	kw := p.advance() // try
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	ts := &ast.TryStmt{KwPos: kw.Pos, Body: body}

	if p.curKind() != token.Catch {
		return nil, p.errHere("try must be followed by catch, got %s", p.describe(p.cur()))
	}
	cat := p.advance() // catch
	ts.CatchPos = cat.Pos
	if _, err := p.expect(token.LParen); err != nil {
		return nil, err
	}
	v, err := p.expect(token.Ident)
	if err != nil {
		return nil, err
	}
	ts.CatchVarPos = v.Pos
	ts.CatchVar = v.Lit
	if _, err := p.expect(token.RParen); err != nil {
		return nil, err
	}
	handler, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	ts.Catch = handler

	if p.curKind() == token.Finally {
		p.advance() // finally
		fin, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		ts.HasFinally = true
		if fin == nil {
			fin = []ast.Stmt{}
		}
		ts.Finally = fin
	}
	return ts, nil
}

// parseSimpleStmt parses an assignment or an expression statement. An
// assignment target is a bare identifier, a field access (`a.f = v`), or an
// array index (`xs[i] = v`); these are distinguished by parsing a postfix
// expression and checking for a following '=' (never '==', which binds as an
// equality operator inside parseExpr).
func (p *parser) parseSimpleStmt() (ast.Stmt, error) {
	// A bare assignable target is a postfix expression (Ident, FieldAccess, or
	// IndexExpr) followed by '='. Parse the postfix chain, then branch.
	start := p.pos
	lhs, err := p.parsePostfix()
	if err != nil {
		return nil, err
	}
	if p.curKind() == token.Assign {
		p.advance() // =
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		switch t := lhs.(type) {
		case *ast.Ident:
			return &ast.AssignStmt{NamePos: t.NamePos, Name: t.Name, Value: val}, nil
		case *ast.FieldAccess:
			return &ast.FieldAssignStmt{Target: t.X, DotPos: t.DotPos, Field: t.Field, Value: val}, nil
		case *ast.IndexExpr:
			return &ast.IndexAssignStmt{Target: t.X, LBrkPos: t.LBrkPos, Index: t.Index, Value: val}, nil
		default:
			return nil, p.errf(lhs.Pos(), "invalid assignment target")
		}
	}
	// Not an assignment: it is an expression statement. The postfix expression
	// may have stopped early relative to a full binary expression (e.g. `f() + 1`
	// as a statement), so re-parse from the start as a full expression.
	p.pos = start
	x, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &ast.ExprStmt{X: x}, nil
}

func (p *parser) parseLet() (ast.Stmt, error) {
	kw := p.advance() // let
	if p.curKind() == token.LParen {
		return p.parseTupleBind(kw, false)
	}
	name, err := p.expect(token.Ident)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.Colon); err != nil {
		return nil, err
	}
	ty, err := p.parseTypeName(false)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.Assign); err != nil {
		return nil, err
	}
	val, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &ast.LetStmt{KwPos: kw.Pos, Name: name.Lit, Type: ty, Value: val}, nil
}

// parseConstAnnotated parses the shared `NAME : Type = Expr` tail used by
// const declarations (both top-level and statement forms). The annotation is
// mandatory: a missing `:` is a parse error ("expected ':'").
func (p *parser) parseConstAnnotated() (name token.Token, ty ast.TypeName, val ast.Expr, err error) {
	name, err = p.expect(token.Ident)
	if err != nil {
		return
	}
	if _, err = p.expect(token.Colon); err != nil {
		return
	}
	ty, err = p.parseTypeName(false)
	if err != nil {
		return
	}
	if _, err = p.expect(token.Assign); err != nil {
		return
	}
	val, err = p.parseExpr()
	return
}

// parseConstDecl parses a top-level `const NAME: Type = <const-expr>` into a
// *ast.ConstDecl appended to Program.Consts.
func (p *parser) parseConstDecl() (*ast.ConstDecl, error) {
	kw := p.advance() // const
	name, ty, val, err := p.parseConstAnnotated()
	if err != nil {
		return nil, err
	}
	return &ast.ConstDecl{KwPos: kw.Pos, NamePos: name.Pos, Name: name.Lit, Type: ty, Value: val}, nil
}

// parseConstStmt parses a function-body `const NAME: Type = <const-expr>` into
// a *ast.ConstStmt.
func (p *parser) parseConstStmt() (ast.Stmt, error) {
	kw := p.advance() // const
	name, ty, val, err := p.parseConstAnnotated()
	if err != nil {
		return nil, err
	}
	return &ast.ConstStmt{KwPos: kw.Pos, NamePos: name.Pos, Name: name.Lit, Type: ty, Value: val}, nil
}

// parseFinalStmt parses a function-body `final NAME: Type = <expr>` into a
// *ast.FinalStmt. Top-level final is rejected in parseProgram before this is
// ever called.
func (p *parser) parseFinalStmt() (ast.Stmt, error) {
	kw := p.advance() // final
	if p.curKind() == token.LParen {
		return p.parseTupleBind(kw, true)
	}
	name, ty, val, err := p.parseConstAnnotated()
	if err != nil {
		return nil, err
	}
	return &ast.FinalStmt{KwPos: kw.Pos, NamePos: name.Pos, Name: name.Lit, Type: ty, Value: val}, nil
}

// parseTupleBind parses a tuple-destructuring pattern after the `let`/`final`
// keyword (kw) has been consumed and a leading `(` is current:
//
//	( slot ( , slot )* ,? ) = <expr>
//
// where each slot is a binding `name: Type` or a discard `_` (with an optional
// `: Type`). It requires k >= 2 slots (a single-slot pattern is a located
// error guiding toward the bare `let name: T = ...` form). This branch is only
// reachable from parseLet/parseFinalStmt, never from parseConstAnnotated, so
// const gains no destructuring.
func (p *parser) parseTupleBind(kw token.Token, final bool) (ast.Stmt, error) {
	if _, err := p.expect(token.LParen); err != nil {
		return nil, err
	}
	var slots []ast.TupleBindSlot
	for {
		slot, err := p.parseTupleSlot()
		if err != nil {
			return nil, err
		}
		slots = append(slots, slot)
		if p.curKind() != token.Comma {
			break
		}
		p.advance() // ,
		if p.curKind() == token.RParen {
			break // trailing comma
		}
	}
	if _, err := p.expect(token.RParen); err != nil {
		return nil, err
	}
	if len(slots) < 2 {
		return nil, p.errf(kw.Pos, "tuple destructuring requires at least 2 slots; use `%s name: T = ...` for a single binding", kwName(final))
	}
	if _, err := p.expect(token.Assign); err != nil {
		return nil, err
	}
	val, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &ast.TupleBindStmt{KwPos: kw.Pos, Final: final, Slots: slots, Value: val}, nil
}

func kwName(final bool) string {
	if final {
		return "final"
	}
	return "let"
}

// parseTupleSlot parses one tuple-destructuring slot. It reads an ident; a `_`
// ident is a discard whose `: Type` is OPTIONAL, any other ident is a binding
// whose `: Type` is MANDATORY. The slot's source position is recorded.
// (parseParams is not reusable here: it unconditionally expects a `:`, so it
// would reject a bare `_`.)
func (p *parser) parseTupleSlot() (ast.TupleBindSlot, error) {
	name, err := p.expect(token.Ident)
	if err != nil {
		return ast.TupleBindSlot{}, err
	}
	blank := name.Lit == "_"
	slot := ast.TupleBindSlot{Name: name.Lit, Blank: blank, Pos: name.Pos}
	if blank {
		if p.curKind() != token.Colon {
			return slot, nil // bare `_`, no type
		}
		p.advance() // :
	} else {
		if _, err := p.expect(token.Colon); err != nil {
			return ast.TupleBindSlot{}, err
		}
	}
	ty, err := p.parseTypeName(false)
	if err != nil {
		return ast.TupleBindSlot{}, err
	}
	slot.Type = ty
	return slot, nil
}

func (p *parser) parseReturn() (ast.Stmt, error) {
	kw := p.advance() // return
	// void return: nothing before the separator / closing brace
	if p.curKind() == token.Separator || p.curKind() == token.RBrace || p.curKind() == token.EOF {
		return &ast.ReturnStmt{KwPos: kw.Pos}, nil
	}
	val, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return &ast.ReturnStmt{KwPos: kw.Pos, Value: val}, nil
}

// parseParenCond parses a parenthesized condition required by control flow.
func (p *parser) parseParenCond() (ast.Expr, error) {
	if _, err := p.expect(token.LParen); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.RParen); err != nil {
		return nil, err
	}
	return cond, nil
}

func (p *parser) parseIf() (ast.Stmt, error) {
	kw := p.advance() // if
	cond, err := p.parseParenCond()
	if err != nil {
		return nil, err
	}
	then, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	stmt := &ast.IfStmt{KwPos: kw.Pos, Cond: cond, Then: then}

	for p.curKind() == token.Else {
		p.advance() // else
		if p.curKind() == token.If {
			p.advance() // if
			c, err := p.parseParenCond()
			if err != nil {
				return nil, err
			}
			body, err := p.parseBlock()
			if err != nil {
				return nil, err
			}
			stmt.ElseIfs = append(stmt.ElseIfs, ast.ElseIf{Cond: c, Body: body})
			continue
		}
		// final else
		body, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		stmt.Else = body
		break
	}
	return stmt, nil
}

// parseMatch parses `match (scrutinee) { arm... }`.
func (p *parser) parseMatch() (ast.Stmt, error) {
	kw := p.advance() // match
	if _, err := p.expect(token.LParen); err != nil {
		return nil, err
	}
	// noStructLit: the `{` after the closing `)` opens the arm block.
	prev := p.noStructLit
	p.noStructLit = true
	scrutinee, err := p.parseExpr()
	p.noStructLit = prev
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.RParen); err != nil {
		return nil, err
	}
	if _, err := p.expect(token.LBrace); err != nil {
		return nil, err
	}
	stmt := &ast.MatchStmt{KwPos: kw.Pos, Scrutinee: scrutinee}
	p.skipSeparators()
	for p.curKind() != token.RBrace && p.curKind() != token.EOF {
		arm, err := p.parseMatchArm()
		if err != nil {
			return nil, err
		}
		stmt.Arms = append(stmt.Arms, arm)
		p.skipSeparators()
	}
	if _, err := p.expect(token.RBrace); err != nil {
		return nil, err
	}
	return stmt, nil
}

// parseMatchArm parses one `case <pattern> { body }` arm.
func (p *parser) parseMatchArm() (*ast.MatchArm, error) {
	if p.curKind() != token.Case {
		// Migration aid (R1): the old `<pattern> => { }` form starts with a
		// pattern (Ident or `_`) instead of `case`. If a `=>` follows the
		// pattern, emit a located hint toward the new form.
		if cur := p.curKind(); cur == token.Ident {
			if pos, ok := p.peekArrowAfterPattern(); ok {
				return nil, p.errf(pos, "match arms now use `case <pattern> { }`; the `=>` form was removed")
			}
		}
		// R-F11/CI-13: `default` is switch's catch-all spelling, not
		// match's -- intercept it here, before the generic fallback, so
		// the diagnostic names the actual mistake (mirrors the `=>`
		// migration aid above in both shape and placement).
		if p.curKind() == token.Default {
			return nil, p.errHere(`match has no "default" arm; use "case _"`)
		}
		return nil, p.errHere("expected case, got %s", p.describe(p.cur()))
	}
	caseTok := p.advance() // case
	pat, err := p.parseMatchPattern()
	if err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &ast.MatchArm{Pattern: pat, CasePos: caseTok.Pos, Body: body}, nil
}

// peekArrowAfterPattern reports whether a match-arm pattern at the current
// position is followed by a `=>` (the removed fat-arrow arm form). It scans the
// shape `Ident` or `Ident ( Ident )` and checks for a trailing FatArrow without
// consuming any tokens, so the caller can point the migration error at it.
func (p *parser) peekArrowAfterPattern() (token.Position, bool) {
	i := p.pos + 1 // past the variant/wildcard Ident
	if i < len(p.toks) && p.toks[i].Kind == token.LParen {
		// `( Ident )`
		if i+2 < len(p.toks) && p.toks[i+1].Kind == token.Ident && p.toks[i+2].Kind == token.RParen {
			i += 3
		}
	}
	if i < len(p.toks) && p.toks[i].Kind == token.FatArrow {
		return p.toks[i].Pos, true
	}
	return token.Position{}, false
}

// parseMatchPattern parses a single match arm pattern.
func (p *parser) parseMatchPattern() (ast.MatchPattern, error) {
	t := p.cur()
	if t.Kind != token.Ident {
		return nil, p.errHere("match arm pattern must be a variant name or '_', got %s", p.describe(t))
	}
	// Arm-level wildcard: bare `_` not followed by `(`.
	if t.Lit == "_" && p.peekKind() != token.LParen {
		p.advance()
		return &ast.WildcardPat{Pos: t.Pos}, nil
	}
	variantTok := p.advance() // variant name
	if p.curKind() == token.LParen {
		p.advance() // (
		nameTok, err := p.expect(token.Ident)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.RParen); err != nil {
			return nil, err
		}
		return &ast.ConstructorPat{
			Variant: variantTok.Lit, VariantPos: variantTok.Pos,
			Name: nameTok.Lit, NamePos: nameTok.Pos,
		}, nil
	}
	// Bare variant name -- no-payload variant like None.
	return &ast.ConstructorPat{
		Variant: variantTok.Lit, VariantPos: variantTok.Pos,
	}, nil
}

func (p *parser) parseWhile() (ast.Stmt, error) {
	kw := p.advance() // while
	cond, err := p.parseParenCond()
	if err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &ast.WhileStmt{KwPos: kw.Pos, Cond: cond, Body: body}, nil
}

func (p *parser) parseFor() (ast.Stmt, error) {
	kw := p.advance() // for
	if _, err := p.expect(token.LParen); err != nil {
		return nil, err
	}

	// Disambiguate the for-in form `for (x in coll)` from the C-style
	// `for (init; cond; post)`. `in` is a contextual keyword (a plain Ident with
	// literal "in"); it is recognized only here, immediately after the loop var.
	if p.curKind() == token.Ident && p.peekKind() == token.Ident && p.toks[p.pos+1].Lit == "in" {
		return p.parseForIn(kw)
	}

	stmt := &ast.ForStmt{KwPos: kw.Pos}

	// init clause (optional): a let or a simple statement, terminated by ';'
	if p.curKind() != token.Separator {
		var init ast.Stmt
		var err error
		if p.curKind() == token.Let {
			init, err = p.parseLet()
		} else {
			init, err = p.parseSimpleStmt()
		}
		if err != nil {
			return nil, err
		}
		stmt.Init = init
	}
	if _, err := p.expectForSep(); err != nil {
		return nil, err
	}

	// cond clause (required in M1 — there is no implicit truthiness/empty cond)
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	stmt.Cond = cond
	if _, err := p.expectForSep(); err != nil {
		return nil, err
	}

	// post clause (optional), terminated by ')'
	if p.curKind() != token.RParen {
		post, err := p.parseSimpleStmt()
		if err != nil {
			return nil, err
		}
		stmt.Post = post
	}
	if _, err := p.expect(token.RParen); err != nil {
		return nil, err
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	stmt.Body = body
	return stmt, nil
}

// parseForIn parses the body of a for-in loop after `for (` has been consumed:
// `x in coll) { body }`. kw is the `for` keyword token.
func (p *parser) parseForIn(kw token.Token) (ast.Stmt, error) {
	v := p.advance() // loop variable Ident
	in := p.advance()
	if in.Lit != "in" {
		return nil, p.errf(in.Pos, "expected 'in' in for-in loop")
	}
	coll, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.RParen); err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &ast.ForInStmt{KwPos: kw.Pos, VarPos: v.Pos, Var: v.Lit, Coll: coll, Body: body}, nil
}

// expectForSep consumes the ';' separating for-clauses. The lexer emits both
// ';' and newline as Separator, so a clause separator is a single Separator
// token (we require it not to be a newline by construction — for headers are
// single-line in practice, but the grammar accepts either separator).
func (p *parser) expectForSep() (token.Token, error) {
	if p.curKind() != token.Separator {
		return token.Token{}, p.errHere("expected ';' in for-clause, got %s", p.describe(p.cur()))
	}
	return p.advance(), nil
}

func (p *parser) parseSwitch() (ast.Stmt, error) {
	kw := p.advance() // switch
	subject, err := p.parseParenCond()
	if err != nil {
		return nil, err
	}
	stmt := &ast.SwitchStmt{KwPos: kw.Pos, Subject: subject}

	if _, err := p.expect(token.LBrace); err != nil {
		return nil, err
	}
	p.skipSeparators()
	for p.curKind() != token.RBrace {
		switch p.curKind() {
		case token.Case:
			c, err := p.parseSwitchCase()
			if err != nil {
				return nil, err
			}
			stmt.Cases = append(stmt.Cases, c)
		case token.Default:
			p.advance()
			body, err := p.parseBlock()
			if err != nil {
				return nil, err
			}
			stmt.Default = body
			if stmt.Default == nil {
				stmt.Default = []ast.Stmt{} // present but empty
			}
		case token.EOF:
			return nil, p.errHere("expected '}', got end of input")
		default:
			return nil, p.errHere("expected 'case' or 'default', got %s", p.describe(p.cur()))
		}
		p.skipSeparators()
	}
	p.advance() // }
	return stmt, nil
}

func (p *parser) parseSwitchCase() (ast.SwitchCase, error) {
	p.advance() // case
	var values []ast.Expr
	// A case value is followed by the case body's `{`, so an `Ident {` here opens
	// a block, not a struct literal -- suppress struct-literal parsing for the
	// values. (The checker further restricts case values to literals.)
	prev := p.noStructLit
	p.noStructLit = true
	for {
		v, err := p.parseExpr()
		if err != nil {
			p.noStructLit = prev
			return ast.SwitchCase{}, err
		}
		values = append(values, v)
		if p.curKind() != token.Comma {
			break
		}
		p.advance()
	}
	p.noStructLit = prev
	body, err := p.parseBlock()
	if err != nil {
		return ast.SwitchCase{}, err
	}
	return ast.SwitchCase{Values: values, Body: body}, nil
}

// --- expression parsing (precedence climbing) ---

// binPrec returns the left-binding precedence of a binary operator, or 0 if the
// kind is not a binary operator. Higher binds tighter.
func binPrec(k token.Kind) int {
	switch k {
	case token.OrOr:
		return 1
	case token.AndAnd:
		return 2
	case token.Eq, token.Neq:
		return 3
	case token.Lt, token.Lte, token.Gt, token.Gte:
		return 4
	case token.Pipe:
		return 5
	case token.Caret:
		return 6
	case token.Amp:
		return 7
	case token.Shl, token.Shr:
		return 8
	case token.Plus, token.Minus:
		return 9
	case token.Star, token.Slash, token.Percent:
		return 10
	default:
		return 0
	}
}

func (p *parser) parseExpr() (ast.Expr, error) {
	done, err := p.enterDepth()
	defer done()
	if err != nil {
		return nil, err
	}
	return p.parseBinary(1)
}

func (p *parser) parseBinary(minPrec int) (ast.Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		op := p.curKind()
		prec := binPrec(op)
		if prec < minPrec {
			break
		}
		opTok := p.advance()
		// all M1 binary operators are left-associative
		right, err := p.parseBinary(prec + 1)
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{OpPos: opTok.Pos, Op: op, L: left, R: right}
	}
	return left, nil
}

func (p *parser) parseUnary() (ast.Expr, error) {
	done, err := p.enterDepth()
	defer done()
	if err != nil {
		return nil, err
	}
	if p.curKind() == token.Minus || p.curKind() == token.Bang {
		opTok := p.advance()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpr{OpPos: opTok.Pos, Op: opTok.Kind, X: x}, nil
	}
	return p.parsePostfix()
}

// parsePostfix parses a primary expression and any trailing `.field` / `[index]`
// / `(args)` suffixes (left-associative), so `a.b[0].c` and `f()`, `s.op(x)`,
// `fns[0](y)`, `getOp()(z)` all chain correctly. A `(...)` suffix builds a
// CallExpr whose callee is the expression parsed so far (M4); resolving whether
// that callee is a declared function, a local funcref, or a builtin is a checker
// concern.
func (p *parser) parsePostfix() (ast.Expr, error) {
	x, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		switch p.curKind() {
		case token.Dot:
			dot := p.advance()
			field, err := p.expect(token.Ident)
			if err != nil {
				return nil, err
			}
			// A qualified struct literal `ns.Type { ... }` (M8): only when the base is
			// a bare identifier (the namespace), a '{' follows, and struct literals are
			// not suppressed (a switch case value).
			if p.curKind() == token.LBrace && !p.noStructLit {
				if id, ok := x.(*ast.Ident); ok {
					lit := &ast.StructLit{NamePos: field.Pos, Name: field.Lit, Namespace: id.Name}
					if err := p.parseStructLitBody(lit); err != nil {
						return nil, err
					}
					x = lit
					continue
				}
			}
			x = &ast.FieldAccess{X: x, DotPos: dot.Pos, Field: field.Lit}
		case token.LBracket:
			// Disambiguate a value index `X[i]` from an explicit call-site type-arg
			// list `X[T1, T2](args)`. The latter is chosen ONLY when the bracket
			// content parses as a non-empty type-name list AND the closing `]` is
			// immediately followed by `(`. Speculative parse with p.pos rewind.
			if typeArgs, ok := p.tryParseCallTypeArgs(); ok {
				call, err := p.parseCallSuffix(x)
				if err != nil {
					return nil, err
				}
				call.(*ast.CallExpr).TypeArgs = typeArgs
				x = call
				continue
			}
			lb := p.advance()
			idx, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(token.RBracket); err != nil {
				return nil, err
			}
			x = &ast.IndexExpr{X: x, LBrkPos: lb.Pos, Index: idx}
		case token.LParen:
			call, err := p.parseCallSuffix(x)
			if err != nil {
				return nil, err
			}
			x = call
		default:
			return x, nil
		}
	}
}

// parseCallSuffix parses the `(args)` of a call whose callee expression is
// callee, with the opening '(' as the current token. CalleeName is set when the
// callee is a bare identifier, giving the checker a fast path for direct/builtin
// resolution.
func (p *parser) parseCallSuffix(callee ast.Expr) (ast.Expr, error) {
	p.advance() // (
	call := &ast.CallExpr{CalleePos: callee.Pos(), Callee: callee}
	if id, ok := callee.(*ast.Ident); ok {
		call.CalleeName = id.Name
	}
	if p.curKind() != token.RParen {
		for {
			arg, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			call.Args = append(call.Args, arg)
			if p.curKind() != token.Comma {
				break
			}
			p.advance()
			if p.curKind() == token.RParen {
				break
			}
		}
	}
	if _, err := p.expect(token.RParen); err != nil {
		return nil, err
	}
	return call, nil
}

// tryParseCallTypeArgs speculatively parses an explicit call-site type-argument
// list `[T1, T2, ...]` (M9). Precondition: the current token is `[`. On success it
// consumes the `[...]`, leaves the cursor on the following `(`, and returns the
// type args. It succeeds ONLY when the bracket content is a non-empty type-name
// list AND the closing `]` is immediately followed by `(` -- otherwise it fully
// rewinds `p.pos` (the parser has no other mutable state) and returns (nil, false)
// so the caller parses a value index instead. This is the single disambiguation
// point between `f[T](x)` and `a[i]` / index-then-call `fns[1](y)`.
func (p *parser) tryParseCallTypeArgs() ([]ast.TypeArg, bool) {
	start := p.pos
	p.advance() // consume '['
	var args []ast.TypeArg
	for {
		pos := p.cur().Pos
		ty, err := p.parseTypeName(false)
		if err != nil {
			p.pos = start
			return nil, false
		}
		args = append(args, ast.TypeArg{Name: ty, Pos: pos})
		if p.curKind() != token.Comma {
			break
		}
		p.advance() // ','
		if p.curKind() == token.RBracket {
			break // trailing comma
		}
	}
	if p.curKind() != token.RBracket || len(args) == 0 {
		p.pos = start
		return nil, false
	}
	p.advance() // ']'
	if p.curKind() != token.LParen {
		p.pos = start
		return nil, false
	}
	return args, true
}

func (p *parser) parsePrimary() (ast.Expr, error) {
	t := p.cur()
	switch t.Kind {
	case token.Int:
		p.advance()
		return &ast.IntLit{LitPos: t.Pos, Raw: t.Lit}, nil
	case token.FloatLit:
		p.advance()
		return &ast.FloatLit{LitPos: t.Pos, Raw: t.Lit}, nil
	case token.True:
		p.advance()
		return &ast.BoolLit{LitPos: t.Pos, Value: true}, nil
	case token.False:
		p.advance()
		return &ast.BoolLit{LitPos: t.Pos, Value: false}, nil
	case token.String:
		p.advance()
		return &ast.StringLit{LitPos: t.Pos, Parts: []ast.StringPart{{Text: t.Lit}}}, nil
	case token.StringStart:
		return p.parseInterpString()
	case token.LBracket:
		return p.parseArrayLit()
	case token.LBrace:
		// A dict literal `{ k: v, ... }` (M3 PR-C). Suppressed in a no-struct-lit
		// context (a switch case value), where `{` opens the case body.
		if p.noStructLit {
			return nil, p.errHere("expected an expression, got %s", p.describe(t))
		}
		return p.parseDictLit()
	case token.Ident:
		// struct construction, or a bare identifier (a call `name(...)` is built by
		// the postfix `(...)` suffix, with the bare Ident as the callee).
		if p.peekKind() == token.LBrace && !p.noStructLit {
			return p.parseStructLit()
		}
		p.advance()
		return &ast.Ident{NamePos: t.Pos, Name: t.Lit}, nil
	case token.TypeInt, token.TypeBool, token.TypeString, token.Float, token.Error:
		// A type-keyword token may also be a namespace alias (e.g. `import
		// "string"` binds `string` as a namespace) or a plain call callee whose
		// name collides with a type keyword. The keyword token carries its
		// literal ("int"/"float"/"bool"/"string"/"error"), so the postfix
		// `(...)` suffix builds a call and a following `.` lets parsePostfix
		// build a qualified member access/call (`string.upper(...)`). A bare
		// type name (neither followed by '(' nor '.') is not a value.
		if p.peekKind() == token.LParen || p.peekKind() == token.Dot {
			p.advance()
			return &ast.Ident{NamePos: t.Pos, Name: t.Lit}, nil
		}
		return nil, p.errHere("type name %q is not a value", t.Lit)
	case token.LParen:
		lp := p.advance() // (
		// () in expression position is the empty-tuple error.
		if p.curKind() == token.RParen {
			p.advance()
			return nil, p.errf(lp.Pos, "empty tuple is not a value")
		}
		first, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		// A single expression followed by ) is a grouped expression (unchanged behavior).
		if p.curKind() == token.RParen {
			p.advance()
			return first, nil // unwrap: (e) -> e
		}
		// A comma signals a tuple literal; collect remaining elements.
		if p.curKind() != token.Comma {
			return nil, p.errHere("expected ')' or ',' in parenthesized expression")
		}
		elems := []ast.Expr{first}
		for p.curKind() == token.Comma {
			p.advance()
			if p.curKind() == token.RParen {
				break // trailing comma
			}
			elem, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			elems = append(elems, elem)
		}
		if _, err := p.expect(token.RParen); err != nil {
			return nil, err
		}
		// n >= 2 (spec sections 1, 4.2). A one-element literal (1,) reaches here with
		// len(elems) == 1: collect `1`, see `,`, then break on the trailing `)`.
		if len(elems) < 2 {
			return nil, p.errf(lp.Pos, "tuple literal requires at least two elements")
		}
		return &ast.TupleLit{LParenPos: lp.Pos, Elems: elems}, nil
	default:
		return nil, p.errHere("expected an expression, got %s", p.describe(t))
	}
}

// parseArrayLit parses `[a, b, c]` (a trailing comma is allowed; `[]` is the
// empty array literal whose element type comes from the surrounding annotation).
func (p *parser) parseArrayLit() (ast.Expr, error) {
	lb := p.advance() // [
	lit := &ast.ArrayLit{LBrkPos: lb.Pos}
	sawNL, _ := p.skipSeparatorsNL()
	for p.curKind() != token.RBracket {
		if p.curKind() == token.EOF {
			return nil, p.errHere("expected ']', got end of input")
		}
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		lit.Elems = append(lit.Elems, e)
		sawComma := p.curKind() == token.Comma
		if sawComma {
			p.advance()
		}
		nl, sawSep := p.skipSeparatorsNL()
		if nl {
			sawNL = true
		}
		if p.curKind() == token.RBracket {
			break
		}
		if !sawComma && !sawSep {
			return nil, p.errHere("expected ',' or newline between array elements")
		}
	}
	if _, err := p.expect(token.RBracket); err != nil {
		return nil, err
	}
	lit.Multiline = sawNL
	return lit, nil
}

// parseDictLit parses `{ k: v, ... }` (M3 PR-C). Entries are comma- and/or
// newline-separated; a trailing separator is allowed. `{}` is the empty dict
// literal whose type comes from the surrounding annotation. The opening '{'
// has already been confirmed as the current token.
func (p *parser) parseDictLit() (ast.Expr, error) {
	lb := p.advance() // {
	lit := &ast.DictLit{LBrace: lb.Pos}
	sawNL, _ := p.skipSeparatorsNL()
	for p.curKind() != token.RBrace {
		if p.curKind() == token.EOF {
			return nil, p.errHere("expected '}', got end of input")
		}
		key, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		colon, err := p.expect(token.Colon)
		if err != nil {
			return nil, err
		}
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		lit.Entries = append(lit.Entries, ast.DictLitEntry{Key: key, Colon: colon.Pos, Value: val})
		if p.curKind() == token.Comma {
			p.advance()
		}
		if nl, _ := p.skipSeparatorsNL(); nl {
			sawNL = true
		}
	}
	p.advance() // }
	lit.Multiline = sawNL
	return lit, nil
}

// parseStructLit parses `Name { f: v, ... }`. Fields are comma- and/or
// newline-separated; a trailing separator is allowed. The leading Ident has
// already been confirmed to be followed by '{'.
func (p *parser) parseStructLit() (ast.Expr, error) {
	name := p.advance() // Ident
	lit := &ast.StructLit{NamePos: name.Pos, Name: name.Lit}
	if err := p.parseStructLitBody(lit); err != nil {
		return nil, err
	}
	return lit, nil
}

// parseStructLitBody parses the `{ f: v, ... }` body into lit, with the opening
// '{' as the current token. Shared by the local (parsePrimary) and qualified
// (parsePostfix) struct-literal paths.
func (p *parser) parseStructLitBody(lit *ast.StructLit) error {
	p.advance() // {
	sawNL, _ := p.skipSeparatorsNL()
	for p.curKind() != token.RBrace {
		if p.curKind() == token.EOF {
			return p.errHere("expected '}', got end of input")
		}
		fname, err := p.expect(token.Ident)
		if err != nil {
			return err
		}
		if _, err := p.expect(token.Colon); err != nil {
			return err
		}
		val, err := p.parseExpr()
		if err != nil {
			return err
		}
		lit.Fields = append(lit.Fields, ast.StructLitField{NamePos: fname.Pos, Name: fname.Lit, Value: val})
		if p.curKind() == token.Comma {
			p.advance()
		}
		if nl, _ := p.skipSeparatorsNL(); nl {
			sawNL = true
		}
	}
	p.advance() // }
	lit.Multiline = sawNL
	return nil
}

// parseInterpString consumes a double-quoted string token group:
// StringStart (StringText | InterpOpen expr InterpClose)* StringEnd.
func (p *parser) parseInterpString() (ast.Expr, error) {
	start := p.advance() // StringStart
	lit := &ast.StringLit{LitPos: start.Pos}
	for {
		switch p.curKind() {
		case token.StringText:
			t := p.advance()
			lit.Parts = append(lit.Parts, ast.StringPart{Text: t.Lit})
		case token.InterpOpen:
			p.advance()
			expr, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(token.InterpClose); err != nil {
				return nil, err
			}
			lit.Parts = append(lit.Parts, ast.StringPart{Expr: expr})
		case token.StringEnd:
			p.advance()
			return lit, nil
		default:
			return nil, p.errHere("malformed string literal: unexpected %s", p.describe(p.cur()))
		}
	}
}
