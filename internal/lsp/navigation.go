package lsp

import (
	"regexp"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/lexer"
	"github.com/mitchellnemitz/wisp/internal/parser"
	"github.com/mitchellnemitz/wisp/internal/token"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// symbol is the resolved identifier under a cursor: its declaration position,
// the byte length of its name, and every occurrence with the declaration
// included.
type symbol struct {
	found   bool
	defPos  token.Position
	nameLen int
	refs    []token.Position
}

// resolveSymbol finds the variable or user function named by the identifier at
// pos and collects all of its occurrences. Only identifiers resolve; builtins,
// keywords, types, and literals return found=false. It also returns the
// document lines so callers can convert positions without resplitting.
func (s *server) resolveSymbol(text string, pos lspPosition) (symbol, []string) {
	lines := splitLines(text)
	toks, _ := lexer.Lex(text, "")
	tk, ok := tokenAtCursor(lines, toks, pos)
	if !ok || tk.Kind != token.Ident {
		return symbol{}, lines
	}
	prog, err := parser.Parse(text, "")
	if err != nil || prog == nil {
		return symbol{}, lines
	}
	info := types.Check(prog)
	target := tk.Pos

	// Build the correct NAME position for every variable. A let binding stores
	// the `let` keyword position, so its name is derived; parameters, for-in
	// bindings, and catch bindings already store the name position.
	namePosByVar := map[*types.Var]token.Position{}
	for _, fi := range info.Funcs {
		for _, v := range fi.Decls {
			namePosByVar[v] = v.Pos
		}
	}
	for _, v := range info.ForInVars {
		namePosByVar[v] = v.Pos
	}
	for _, v := range info.CatchVars {
		namePosByVar[v] = v.Pos
	}
	for _, v := range info.MatchArmVars {
		namePosByVar[v] = v.Pos
	}
	// const bindings have no runtime variable, so they never reach fi.Decls;
	// these two loops are the only source of their name position.
	for cs, v := range info.ConstVars {
		namePosByVar[v] = cs.NamePos
	}
	for cd, v := range info.TopConstVars {
		namePosByVar[v] = cd.NamePos
	}
	for ls, v := range info.Vars {
		namePosByVar[v] = declNamePos(lines, ls.KwPos, len("let"))
	}

	// Variable: cursor on a use or on a declaration name.
	var hit *types.Var
	for id, v := range info.Uses {
		if id.NamePos == target {
			hit = v
			break
		}
	}
	if hit == nil {
		for v, np := range namePosByVar {
			if np == target {
				hit = v
				break
			}
		}
	}
	if hit != nil {
		defPos, ok := namePosByVar[hit]
		if !ok {
			defPos = hit.Pos
		}
		refs := []token.Position{defPos}
		for id, v := range info.Uses {
			if v == hit {
				refs = append(refs, id.NamePos)
			}
		}
		return symbol{found: true, defPos: defPos, nameLen: len(hit.Name), refs: dedupePositions(refs)}, lines
	}

	if fd := functionAt(lines, info, prog, target); fd != nil {
		np, nlen := deriveFuncName(lines, fd)
		refs := []token.Position{np}
		mangled := ""
		if fi := info.Funcs[fd]; fi != nil {
			mangled = fi.Mangled
		}
		for ce, ci := range info.Calls {
			if ci.Kind == types.CallUser && ci.Func == fd {
				refs = append(refs, ce.CalleePos)
			}
		}
		for id, fr := range info.FuncRefs {
			if mangled != "" && fr.Mangled == mangled {
				refs = append(refs, id.NamePos)
			}
		}
		return symbol{found: true, defPos: np, nameLen: nlen, refs: dedupePositions(refs)}, lines
	}

	return symbol{}, lines
}

// declNamePos returns the position of the declared name that follows a keyword
// at kwPos (for example `let` or `fn`), by skipping kwLen bytes and any
// whitespace. wisp has only line comments, which cannot sit between the keyword
// and the name on one line, so this is exact.
func declNamePos(lines []string, kwPos token.Position, kwLen int) token.Position {
	li := kwPos.Line - 1
	if li < 0 || li >= len(lines) {
		return kwPos
	}
	line := lines[li]
	i := kwPos.Col - 1 + kwLen
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return token.Position{File: kwPos.File, Line: kwPos.Line, Col: i + 1}
}

// functionAt returns the FuncDecl named at target, whether target is the
// declaration name, a call site, or a function reference.
func functionAt(lines []string, info *types.Info, prog *ast.Program, target token.Position) *ast.FuncDecl {
	for _, f := range prog.Funcs {
		if np, _ := deriveFuncName(lines, f); np == target {
			return f
		}
	}
	for ce, ci := range info.Calls {
		if ce.CalleePos == target && ci.Kind == types.CallUser && ci.Func != nil {
			return ci.Func
		}
	}
	for id, fr := range info.FuncRefs {
		if id.NamePos == target {
			for _, f := range prog.Funcs {
				if fi := info.Funcs[f]; fi != nil && fi.Mangled == fr.Mangled {
					return f
				}
			}
		}
	}
	return nil
}

func dedupePositions(ps []token.Position) []token.Position {
	seen := map[token.Position]bool{}
	out := ps[:0:0]
	for _, p := range ps {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

func (s *server) handleDefinition(msg *message) {
	var p docRequestParams
	if !s.unmarshalParams(msg, &p) {
		return
	}
	sym, lines := s.resolveSymbol(s.docs[p.TextDocument.URI], p.Position)
	if !sym.found {
		_ = s.conn.respond(msg.ID, nil)
		return
	}
	_ = s.conn.respond(msg.ID, location{
		URI:   p.TextDocument.URI,
		Range: toLSPRange(lines, sym.defPos, sym.nameLen),
	})
}

func (s *server) handleReferences(msg *message) {
	var p referenceParams
	if !s.unmarshalParams(msg, &p) {
		return
	}
	sym, lines := s.resolveSymbol(s.docs[p.TextDocument.URI], p.Position)
	locs := []location{}
	if sym.found {
		for _, r := range sym.refs {
			if !p.Context.IncludeDeclaration && r == sym.defPos {
				continue
			}
			locs = append(locs, location{URI: p.TextDocument.URI, Range: toLSPRange(lines, r, sym.nameLen)})
		}
	}
	_ = s.conn.respond(msg.ID, locs)
}

func (s *server) handleRename(msg *message) {
	var p renameParams
	if !s.unmarshalParams(msg, &p) {
		return
	}
	sym, lines := s.resolveSymbol(s.docs[p.TextDocument.URI], p.Position)
	if !sym.found {
		_ = s.conn.respond(msg.ID, nil)
		return
	}
	edits := make([]textEdit, 0, len(sym.refs))
	for _, r := range sym.refs {
		edits = append(edits, textEdit{Range: toLSPRange(lines, r, sym.nameLen), NewText: p.NewName})
	}
	_ = s.conn.respond(msg.ID, workspaceEdit{Changes: map[string][]textEdit{p.TextDocument.URI: edits}})
}

var didYouMeanRe = regexp.MustCompile(`did you mean "([^"]+)"`)

// handleCodeAction turns a "did you mean X" diagnostic into a quick fix that
// replaces the flagged range with the suggestion.
func (s *server) handleCodeAction(msg *message) {
	var p codeActionParams
	if !s.unmarshalParams(msg, &p) {
		return
	}
	actions := []codeAction{}
	for _, d := range p.Context.Diagnostics {
		m := didYouMeanRe.FindStringSubmatch(d.Message)
		if m == nil {
			continue
		}
		suggestion := m[1]
		actions = append(actions, codeAction{
			Title:       `Change to "` + suggestion + `"`,
			Kind:        "quickfix",
			Diagnostics: []lspDiagnostic{d},
			Edit: workspaceEdit{Changes: map[string][]textEdit{
				p.TextDocument.URI: {{Range: d.Range, NewText: suggestion}},
			}},
		})
	}
	_ = s.conn.respond(msg.ID, actions)
}

func (s *server) handleSignatureHelp(msg *message) {
	var p docRequestParams
	if !s.unmarshalParams(msg, &p) {
		return
	}
	text := s.docs[p.TextDocument.URI]
	lines := splitLines(text)
	prog, err := parser.Parse(text, "")
	if err != nil || prog == nil || p.Position.Line < 0 || p.Position.Line >= len(lines) {
		_ = s.conn.respond(msg.ID, nil)
		return
	}
	line := lines[p.Position.Line]
	byteIdx := utf16ToByte(line, p.Position.Character)
	name, active, ok := enclosingCall(line[:byteIdx])
	if !ok {
		_ = s.conn.respond(msg.ID, nil)
		return
	}
	for _, f := range prog.Funcs {
		if f.Name != name {
			continue
		}
		params := make([]parameterInformation, 0, len(f.Params))
		for _, pa := range f.Params {
			params = append(params, parameterInformation{Label: pa.Name + ": " + string(pa.Type)})
		}
		if active < 0 {
			active = 0
		}
		if len(f.Params) > 0 && active >= len(f.Params) {
			active = len(f.Params) - 1
		}
		_ = s.conn.respond(msg.ID, signatureHelp{
			Signatures:      []signatureInformation{{Label: funcSignature(f), Parameters: params}},
			ActiveSignature: 0,
			ActiveParameter: active,
		})
		return
	}
	_ = s.conn.respond(msg.ID, nil)
}

// utf16ToByte returns the byte index in line at the given UTF-16 code-unit
// offset (an LSP character).
func utf16ToByte(line string, u int) int {
	units := 0
	for i, r := range line {
		if units >= u {
			return i
		}
		if r > 0xFFFF {
			units += 2
		} else {
			units++
		}
	}
	return len(line)
}

// enclosingCall scans a line prefix backward for the innermost unclosed '(',
// returning the identifier before it and the active parameter index (the count
// of top-level commas after the '('). ok is false when the cursor is not inside
// a call's argument list on this line.
//
// This works on raw text, not tokens, so parens or commas inside a string
// literal or comment on the same line can skew the depth or the parameter
// count. Signature help is a best-effort hint and the active index is clamped
// by the caller, so the worst case is a slightly-off highlight, never a crash.
func enclosingCall(prefix string) (name string, active int, ok bool) {
	depth := 0
	open := -1
	for i := len(prefix) - 1; i >= 0 && open < 0; i-- {
		switch prefix[i] {
		case ')':
			depth++
		case '(':
			if depth == 0 {
				open = i
			} else {
				depth--
			}
		}
	}
	if open < 0 {
		return "", 0, false
	}
	j := open - 1
	for j >= 0 && (prefix[j] == ' ' || prefix[j] == '\t') {
		j--
	}
	end := j + 1
	for j >= 0 && isIdentByte(prefix[j]) {
		j--
	}
	name = prefix[j+1 : end]
	if name == "" {
		return "", 0, false
	}
	d := 0
	for k := open + 1; k < len(prefix); k++ {
		switch prefix[k] {
		case '(', '[', '{':
			d++
		case ')', ']', '}':
			d--
		case ',':
			if d == 0 {
				active++
			}
		}
	}
	return name, active, true
}

func isIdentByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
