package lsp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/format"
	"github.com/mitchellnemitz/wisp/internal/lexer"
	"github.com/mitchellnemitz/wisp/internal/parser"
	"github.com/mitchellnemitz/wisp/internal/token"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// server is a wisp language server over one JSON-RPC connection. It keeps an
// in-memory map of open document text and recomputes analysis on demand from
// the compiler's own lexer/parser/checker/formatter.
type server struct {
	conn *conn
	errw io.Writer
	docs map[string]string

	shutdown bool

	// Capabilities advertised in `initialize` are gated on these flags so a
	// capability is announced if and only if it is implemented (never falsely
	// advertised). Both are implemented (A3), so both are true.
	hoverEnabled      bool
	completionEnabled bool
}

// NewServer builds a server reading framed JSON-RPC from in, writing to out,
// and logging panics to errw.
func newServer(in io.Reader, out, errw io.Writer) *server {
	return &server{
		conn:              newConn(in, out),
		errw:              errw,
		docs:              map[string]string{},
		hoverEnabled:      true,
		completionEnabled: true,
	}
}

// Serve runs the read/dispatch loop until exit or end of stream and returns the
// process exit code. Per LSP, `exit` (or stream end) returns 0 when a prior
// `shutdown` was seen, else 1.
func Serve(in io.Reader, out, errw io.Writer) int {
	return newServer(in, out, errw).serve()
}

func (s *server) serve() int {
	for {
		msg, err := s.conn.readMessage()
		if err != nil {
			switch {
			case errors.Is(err, errParseError):
				// Body was read whole but is not valid JSON: reply parse-error and
				// keep serving (the next message reads fine).
				_ = s.conn.respondError(nil, codeParseError, "parse error")
				continue
			case errors.Is(err, errBadFrame):
				return 1 // unrecoverable framing
			default: // io.EOF
				if s.shutdown {
					return 0
				}
				return 1
			}
		}
		if msg.Method == "exit" {
			if s.shutdown {
				return 0
			}
			return 1
		}
		s.handle(msg)
	}
}

// handle dispatches one message under panic recovery.
func (s *server) handle(msg *message) {
	s.safely(msg, func() { s.dispatch(msg) })
}

// safely runs fn, recovering from any panic: the panic is logged to stderr and,
// for a request, an internal-error response is returned, so a single bad
// message never takes the server down.
func (s *server) safely(msg *message, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(s.errw, "wisp-lsp: recovered panic handling %q: %v\n", msg.Method, r)
			if msg.isRequest() {
				_ = s.conn.respondError(msg.ID, codeInternalError, "internal error")
			}
		}
	}()
	fn()
}

// unmarshalParams decodes a request's params into v. On malformed params it
// sends a JSON-RPC -32602 Invalid params error for the request id and returns
// false; the caller must return without producing a result. JSON null decodes
// into a zero-valued v without error, matching the prior lenient behavior for
// param-less requests. Returns true on success.
func (s *server) unmarshalParams(msg *message, v any) bool {
	if err := json.Unmarshal(msg.Params, v); err != nil {
		_ = s.conn.respondError(msg.ID, codeInvalidParams, "invalid params")
		return false
	}
	return true
}

// unmarshalNotifyParams decodes a notification's params into v. A notification
// gets no error reply per JSON-RPC, so on malformed params it logs to stderr and
// returns false; the caller must return without acting on a zero-valued struct.
// Returns true on success.
func (s *server) unmarshalNotifyParams(msg *message, v any) bool {
	if err := json.Unmarshal(msg.Params, v); err != nil {
		fmt.Fprintf(s.errw, "wisp-lsp: ignoring %q notification with invalid params: %v\n", msg.Method, err)
		return false
	}
	return true
}

func (s *server) dispatch(msg *message) {
	switch msg.Method {
	case "initialize":
		_ = s.conn.respond(msg.ID, s.capabilities())
	case "initialized", "$/setTrace", "$/cancelRequest", "textDocument/didSave",
		"workspace/didChangeConfiguration":
		// notifications we accept and ignore
	case "shutdown":
		s.shutdown = true
		_ = s.conn.respond(msg.ID, nil)
	case "textDocument/didOpen":
		var p didOpenParams
		if !s.unmarshalNotifyParams(msg, &p) {
			return
		}
		s.docs[p.TextDocument.URI] = p.TextDocument.Text
		s.publishDiagnostics(p.TextDocument.URI)
	case "textDocument/didChange":
		var p didChangeParams
		if !s.unmarshalNotifyParams(msg, &p) {
			return
		}
		if n := len(p.ContentChanges); n > 0 {
			// Full sync: the last change carries the entire document.
			s.docs[p.TextDocument.URI] = p.ContentChanges[n-1].Text
		}
		s.publishDiagnostics(p.TextDocument.URI)
	case "textDocument/didClose":
		var p didCloseParams
		if !s.unmarshalNotifyParams(msg, &p) {
			return
		}
		delete(s.docs, p.TextDocument.URI)
		// Clear diagnostics for the closed document.
		_ = s.conn.notify("textDocument/publishDiagnostics",
			publishDiagnosticsParams{URI: p.TextDocument.URI, Diagnostics: []lspDiagnostic{}})
	case "textDocument/formatting":
		s.handleFormatting(msg)
	case "textDocument/documentSymbol":
		s.handleDocumentSymbol(msg)
	case "textDocument/hover":
		s.handleHover(msg)
	case "textDocument/completion":
		s.handleCompletion(msg)
	case "textDocument/definition":
		s.handleDefinition(msg)
	case "textDocument/references":
		s.handleReferences(msg)
	case "textDocument/rename":
		s.handleRename(msg)
	case "textDocument/signatureHelp":
		s.handleSignatureHelp(msg)
	case "textDocument/codeAction":
		s.handleCodeAction(msg)
	default:
		if msg.isRequest() {
			_ = s.conn.respondError(msg.ID, codeMethodNotFound, "method not found: "+msg.Method)
		}
		// unknown notification: ignore
	}
}

func (s *server) capabilities() any {
	caps := map[string]any{
		"textDocumentSync":           map[string]any{"openClose": true, "change": 1}, // 1 = Full
		"documentFormattingProvider": true,
		"documentSymbolProvider":     true,
		"definitionProvider":         true,
		"referencesProvider":         true,
		"renameProvider":             true,
		"codeActionProvider":         map[string]any{"codeActionKinds": []string{"quickfix"}},
		"signatureHelpProvider":      map[string]any{"triggerCharacters": []string{"(", ","}},
	}
	if s.hoverEnabled {
		caps["hoverProvider"] = true
	}
	if s.completionEnabled {
		caps["completionProvider"] = map[string]any{"triggerCharacters": []string{"."}}
	}
	return map[string]any{"capabilities": caps}
}

func (s *server) publishDiagnostics(uri string) {
	diags := computeDiagnostics(s.docs[uri])
	_ = s.conn.notify("textDocument/publishDiagnostics",
		publishDiagnosticsParams{URI: uri, Diagnostics: diags})
}

// computeDiagnostics lexes+parses, then type-checks. A parse (or lex) error
// yields exactly one Error diagnostic; otherwise the checker's errors and
// warnings map to Error/Warning diagnostics.
func computeDiagnostics(text string) []lspDiagnostic {
	lines := splitLines(text)
	toks, _ := lexer.Lex(text, "")
	prog, perr := parser.Parse(text, "")
	if perr != nil {
		pos, msg := stageErrorPosMsg(perr)
		return []lspDiagnostic{{
			Range: rangeAtPos(lines, toks, pos), Severity: severityError,
			Source: "wisp", Message: msg,
		}}
	}
	info := types.Check(prog)
	// The LSP checks a single open buffer and does not resolve the project, so a
	// qualified reference through a declared import/include alias would otherwise
	// surface a spurious "undeclared"/"namespace" error. Suppress those for the
	// aliases the buffer makes knowable (explicit `as` aliases, unaliased
	// core/package imports' default namespace, and include file stems); full
	// project-aware resolution is deferred (spec section 6). This is best-effort
	// and never suppresses errors unrelated to a known alias.
	aliases := bufferAliases(prog)
	diags := []lspDiagnostic{}
	for _, d := range info.Errors {
		if suppressedForAlias(d.Msg, d.Pos, toks, aliases) {
			continue
		}
		diags = append(diags, lspDiagnostic{
			Range: rangeAtPos(lines, toks, d.Pos), Severity: severityError,
			Source: "wisp", Message: d.Msg,
		})
	}
	for _, d := range info.Warnings {
		diags = append(diags, lspDiagnostic{
			Range: rangeAtPos(lines, toks, d.Pos), Severity: severityWarning,
			Source: "wisp", Message: d.Msg,
		})
	}
	return diags
}

// bufferAliases returns the namespace aliases knowable from the buffer alone:
// every explicit `as` alias on an import/include, the default namespace of an
// unaliased import (the bare path for a core module, or the final path
// segment for a package import), plus the default file stem of each include.
func bufferAliases(prog *ast.Program) map[string]bool {
	out := map[string]bool{}
	for _, im := range prog.Imports {
		if im.Alias != "" {
			out[im.Alias] = true
			continue
		}
		name := im.Path
		if i := strings.LastIndexByte(name, '/'); i >= 0 {
			name = name[i+1:]
		}
		if name != "" {
			out[name] = true
		}
	}
	for _, in := range prog.Includes {
		if in.Alias != "" {
			out[in.Alias] = true
		} else {
			base := in.Path
			if i := strings.LastIndexByte(base, '/'); i >= 0 {
				base = base[i+1:]
			}
			base = strings.TrimSuffix(base, ".wisp")
			if base != "" {
				out[base] = true
			}
		}
	}
	return out
}

// suppressedForAlias reports whether msg is an "undeclared name"/"unknown
// namespace"/"was moved to a module"/"is not imported" diagnostic about a
// known buffer alias that is actually used as a QUALIFIER at pos (the
// identifier is immediately followed by a `.`). Gating on the trailing dot
// means a bare misuse of the alias as a value (`let x = lib`) is still
// reported; only the valid-but-unresolvable `lib.member` form is silenced.
// The "unknown namespace " shape is the one exception: resolveNamedType
// never reports it at a position followed by a dot on any reachable call
// path, so it is suppressed on the alias-substring match alone -- safe
// because that message only fires for a name string that already contains a
// literal dot, so its existence alone proves a qualified-type usage. The
// "is not imported" shape is the single-buffer checker's own message for a
// coreCatalog module it can't see the import for (bufferAliases derives
// aliases independently of the checker's own namespace tracking); it fires
// at the qualifier position exactly like "undeclared name " does, so it is
// gated the same way. A "module namespace, not a value" error is never
// suppressed.
func suppressedForAlias(msg string, pos token.Position, toks []token.Token, aliases map[string]bool) bool {
	if len(aliases) == 0 {
		return false
	}
	var matched bool
	for a := range aliases {
		if strings.Contains(msg, "\""+a+"\"") {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}
	if strings.Contains(msg, "unknown namespace ") {
		return true
	}
	if !strings.Contains(msg, "undeclared name ") && !strings.Contains(msg, "was moved to a module") && !strings.Contains(msg, "is not imported") {
		return false
	}
	return followedByDot(toks, pos)
}

// followedByDot reports whether the token at pos is immediately followed by a Dot
// token (the identifier is the namespace part of a qualified reference).
func followedByDot(toks []token.Token, pos token.Position) bool {
	for i, t := range toks {
		if t.Pos.Line == pos.Line && t.Pos.Col == pos.Col {
			return i+1 < len(toks) && toks[i+1].Kind == token.Dot
		}
	}
	return false
}

// precededByDot reports whether the token at pos is immediately preceded by a
// Dot token.
func precededByDot(toks []token.Token, pos token.Position) bool {
	for i, t := range toks {
		if t.Pos.Line == pos.Line && t.Pos.Col == pos.Col {
			return i > 0 && toks[i-1].Kind == token.Dot
		}
	}
	return false
}

// precedingQualifier reports whether the token at pos is immediately preceded
// by "<qualifier> .", the mirror of followedByDot. The qualifier token is not
// required to be token.Ident: a namespace name that collides with a
// type-keyword spelling (e.g. "string") still lexes as its own keyword kind
// (TypeString, ...), and the parser accepts it as the namespace identifier
// when followed by "." (parser.go's primary-expr TypeInt/TypeBool/TypeString/
// Float/Error case) -- so the qualifier is identified by adjacency alone.
// <qualifier> must be BARE -- not itself preceded by a "." -- so a chained
// field access like obj.math.floor (where "floor" is preceded by "math .",
// but "math" is itself preceded by "obj .") does not false-match. Returns the
// qualifier's literal.
func precedingQualifier(toks []token.Token, pos token.Position) (string, bool) {
	for i, t := range toks {
		if t.Pos.Line == pos.Line && t.Pos.Col == pos.Col {
			if i < 2 || toks[i-1].Kind != token.Dot {
				return "", false
			}
			if i >= 3 && toks[i-3].Kind == token.Dot {
				return "", false
			}
			return toks[i-2].Lit, true
		}
	}
	return "", false
}

func stageErrorPosMsg(err error) (token.Position, string) {
	var le *lexer.Error
	if errors.As(err, &le) {
		return le.Pos, le.Msg
	}
	var pe *parser.Error
	if errors.As(err, &pe) {
		return pe.Pos, pe.Msg
	}
	return token.Position{Line: 1, Col: 1}, err.Error()
}

func (s *server) handleFormatting(msg *message) {
	var p docRequestParams
	if !s.unmarshalParams(msg, &p) {
		return
	}
	text, ok := s.docs[p.TextDocument.URI]
	if !ok {
		_ = s.conn.respond(msg.ID, []textEdit{})
		return
	}
	formatted, err := format.Format(text, "")
	if err != nil || formatted == text {
		// No edits on a parse error, and none when already canonical.
		_ = s.conn.respond(msg.ID, []textEdit{})
		return
	}
	lines := splitLines(text)
	endLine := len(lines) - 1
	endChar := utf16Units(lines[endLine]) // UTF-16 code units, not bytes
	edit := textEdit{
		Range: lspRange{
			Start: lspPosition{Line: 0, Character: 0},
			End:   lspPosition{Line: endLine, Character: endChar},
		},
		NewText: formatted,
	}
	_ = s.conn.respond(msg.ID, []textEdit{edit})
}

func (s *server) handleDocumentSymbol(msg *message) {
	var p docRequestParams
	if !s.unmarshalParams(msg, &p) {
		return
	}
	text := s.docs[p.TextDocument.URI]
	lines := splitLines(text)
	toks, _ := lexer.Lex(text, "")
	syms := []documentSymbol{}
	prog, err := parser.Parse(text, "")
	if err != nil || prog == nil {
		_ = s.conn.respond(msg.ID, syms)
		return
	}
	for _, st := range prog.Structs {
		end := declEnd(toks, st.KwPos)
		syms = append(syms, documentSymbol{
			Name: st.Name, Kind: symbolKindStruct,
			Range: lspRange{Start: toLSPPosition(lines, st.KwPos), End: toLSPPosition(lines, end)},
			// StructDecl carries NamePos directly.
			SelectionRange: toLSPRange(lines, st.NamePos, len(st.Name)),
		})
	}
	for _, en := range prog.Enums {
		end := declEnd(toks, en.KwPos)
		syms = append(syms, documentSymbol{
			Name: en.Name, Kind: symbolKindEnum,
			Range: lspRange{Start: toLSPPosition(lines, en.KwPos), End: toLSPPosition(lines, end)},
			// EnumDecl carries NamePos directly.
			SelectionRange: toLSPRange(lines, en.NamePos, len(en.Name)),
		})
	}
	for _, a := range prog.Aliases {
		// A type alias is a single-line, braceless decl; declEnd (which scans for a
		// balanced brace) would run into a later function's body, so bound the range
		// to the alias's own source line instead.
		endCol := a.KwPos.Col
		if a.KwPos.Line >= 1 && a.KwPos.Line <= len(lines) {
			endCol = len(lines[a.KwPos.Line-1]) + 1
		}
		end := token.Position{File: a.KwPos.File, Line: a.KwPos.Line, Col: endCol}
		syms = append(syms, documentSymbol{
			Name: a.Name, Kind: symbolKindTypeParameter,
			Range:          lspRange{Start: toLSPPosition(lines, a.KwPos), End: toLSPPosition(lines, end)},
			SelectionRange: toLSPRange(lines, a.NamePos, len(a.Name)),
		})
	}
	for _, fn := range prog.Funcs {
		end := declEnd(toks, fn.KwPos)
		// FuncDecl has no NamePos: derive the name position by scanning from the
		// `fn` keyword past whitespace to the name (selectionRange must cover the
		// NAME, never the keyword).
		namePos, nameWidth := deriveFuncName(lines, fn)
		syms = append(syms, documentSymbol{
			Name: fn.Name, Kind: symbolKindFunction,
			Range:          lspRange{Start: toLSPPosition(lines, fn.KwPos), End: toLSPPosition(lines, end)},
			SelectionRange: toLSPRange(lines, namePos, nameWidth),
		})
	}
	_ = s.conn.respond(msg.ID, syms)
}

// declEnd returns the position just past the closing brace that matches the
// first opening brace at or after start. It counts braces over the token stream
// (token.LBrace/token.RBrace), so braces inside a string literal -- which never
// lex as brace tokens -- are naturally ignored and cannot desync the counter. If
// no brace body is found it returns start.
func declEnd(toks []token.Token, start token.Position) token.Position {
	depth := 0
	sawBrace := false
	for _, t := range toks {
		if !sawBrace && posBefore(t.Pos, start) {
			continue
		}
		switch t.Kind {
		case token.LBrace:
			depth++
			sawBrace = true
		case token.RBrace:
			depth--
			if sawBrace && depth == 0 {
				return token.Position{File: start.File, Line: t.Pos.Line, Col: t.Pos.Col + 1}
			}
		}
	}
	return start
}

// posBefore reports whether a precedes b in (line, byte-col) order.
func posBefore(a, b token.Position) bool {
	return a.Line < b.Line || (a.Line == b.Line && a.Col < b.Col)
}

// deriveFuncName returns the position and byte width of a function's name by
// scanning from the `fn` keyword past whitespace. wisp has only line comments,
// which cannot sit between `fn` and the name on one line, so this is exact.
func deriveFuncName(lines []string, fn *ast.FuncDecl) (token.Position, int) {
	return declNamePos(lines, fn.KwPos, len("fn")), len(fn.Name)
}

func (s *server) handleHover(msg *message) {
	var p docRequestParams
	if !s.unmarshalParams(msg, &p) {
		return
	}
	text := s.docs[p.TextDocument.URI]
	lines := splitLines(text)
	toks, _ := lexer.Lex(text, "")
	tk, ok := tokenAtCursor(lines, toks, p.Position)
	if !ok {
		_ = s.conn.respond(msg.ID, nil)
		return
	}
	var info *types.Info
	prog, perr := parser.Parse(text, "")
	if perr == nil && prog != nil {
		info = types.Check(prog)
	}
	detail := hoverDetail(tk, prog, info, toks)
	if detail == "" {
		_ = s.conn.respond(msg.ID, nil)
		return
	}
	r := toLSPRange(lines, tk.Pos, tokenWidthBytes(tk))
	_ = s.conn.respond(msg.ID, hoverResult{
		Contents: markupContent{Kind: "plaintext", Value: detail},
		Range:    &r,
	})
}

// tokenAtCursor returns the token whose range contains the cursor (a 0-based
// UTF-16 LSP position). Zero-width tokens (string pieces) are skipped.
//
// Matching uses end-exclusive bounds: a token strictly contains the cursor when
// Start <= cur < End. This ensures that when the cursor is on the first
// character of an identifier that abuts a preceding token (e.g. b in a+b, or x
// in p.x), the identifier wins over the preceding token whose End equals cur.
// When no token strictly contains the cursor, we fall back to tokens where
// cur == End, which handles the common case of a cursor resting just past the
// last character of an identifier (e.g. the cursor after a word with no further
// adjacent token on the same line).
func tokenAtCursor(lines []string, toks []token.Token, cur lspPosition) (token.Token, bool) {
	var fallback token.Token
	hasFallback := false
	for _, tk := range toks {
		w := tokenWidthBytes(tk)
		if w == 0 {
			continue
		}
		r := toLSPRange(lines, tk.Pos, w)
		if r.Start.Line != cur.Line {
			continue
		}
		if cur.Character >= r.Start.Character && cur.Character < r.End.Character {
			return tk, true
		}
		if cur.Character == r.End.Character && !hasFallback {
			fallback = tk
			hasFallback = true
		}
	}
	return fallback, hasFallback
}

var typeNameSet = func() map[string]bool {
	m := map[string]bool{}
	for _, t := range types.TypeNames() {
		m[t] = true
	}
	return m
}()

var builtinNameSet = func() map[string]bool {
	m := map[string]bool{}
	for _, b := range types.BuiltinNames() {
		m[b] = true
	}
	return m
}()

var coreNamespaceSet = func() map[string]bool {
	m := map[string]bool{}
	for _, n := range types.CoreNamespaces() {
		m[n] = true
	}
	return m
}()

// hoverDetail renders a plaintext description for the token at the cursor, or ""
// if there is nothing useful to say (so the server replies null).
//
// The two core-module branches sit at the very top, before the info.Calls
// loop below: CallExpr.CalleePos points at the QUALIFIER for a namespaced call
// (string.trim(x)), so a qualifier-hover check placed after that loop would be
// unreachable in call position -- the loop would already have matched and
// returned "(builtin) trim" first. Placed here, both branches run before that
// loop ever sees the token.
func hoverDetail(tk token.Token, prog *ast.Program, info *types.Info, toks []token.Token) string {
	pos := tk.Pos
	if ns, ok := precedingQualifier(toks, pos); ok {
		if d, ok2 := types.CoreMemberHover(ns, tk.Lit); ok2 {
			return d
		}
	} else if followedByDot(toks, pos) && coreNamespaceSet[tk.Lit] && !precededByDot(toks, pos) {
		return "(module) " + tk.Lit
	}
	if info != nil {
		for call, ci := range info.Calls {
			if call.CalleePos == pos {
				switch ci.Kind {
				case types.CallBuiltin:
					return "(builtin) " + ci.Builtin
				case types.CallUser:
					if ci.Func != nil {
						return funcSignature(ci.Func)
					}
				}
			}
		}
		for ident, v := range info.Uses {
			if ident.NamePos == pos && v != nil {
				// types.Type mirrors ast.TypeName's encoding; the cast is safe.
				return v.Name + ": " + format.FormatType(ast.TypeName(v.Type))
			}
		}
		for ident, fr := range info.FuncRefs {
			if ident.NamePos == pos && fr != nil {
				return ident.Name + ": " + format.FormatType(ast.TypeName(fr.Type))
			}
		}
	}
	name := tk.Lit
	if prog != nil {
		for _, fn := range prog.Funcs {
			if fn.Name == name {
				return funcSignature(fn)
			}
		}
		for _, st := range prog.Structs {
			if st.Name == name {
				return "struct " + st.Name
			}
		}
		for _, en := range prog.Enums {
			if en.Name == name {
				return enumSignature(en)
			}
		}
		for _, a := range prog.Aliases {
			if a.Name == name {
				return "type " + a.Name + " = " + format.FormatType(a.Type)
			}
		}
	}
	if typeNameSet[name] {
		return "(type) " + name
	}
	if builtinNameSet[name] {
		return "(builtin) " + name
	}
	if _, isKw := token.Lookup(name); isKw && name != "" {
		return "(keyword) " + name
	}
	return ""
}

// enumSignature renders a one-line hover summary of an enum, mirroring the
// single-line decl form: `enum Color { Red, Green, Blue }`.
func enumSignature(en *ast.EnumDecl) string {
	var b strings.Builder
	b.WriteString("enum ")
	b.WriteString(en.Name)
	b.WriteString(" { ")
	for i, v := range en.Variants {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(v.Name)
	}
	b.WriteString(" }")
	return b.String()
}

func funcSignature(fn *ast.FuncDecl) string {
	var b strings.Builder
	b.WriteString("fn ")
	b.WriteString(fn.Name)
	b.WriteByte('(')
	for i, p := range fn.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Name)
		b.WriteString(": ")
		b.WriteString(format.FormatType(p.Type))
		if p.Default != nil {
			b.WriteString(" = ...")
		}
	}
	b.WriteString(") -> ")
	b.WriteString(format.FormatType(fn.RetType))
	return b.String()
}

// completionNamespacePrefix reports whether the document's current line, up
// to the cursor, ends in a bare "<ns>.<partial>" core-module qualifier --
// pure current-line text matching, no lexer pass (mirrors handleSignatureHelp's
// approach). <ns> must be a core namespace and a BARE reference: the
// character immediately before it must be neither "." nor an identifier
// character, so a chained field access like "obj.math." does not false-match
// (the same bare-qualifier guard as hover's precedingQualifier).
func completionNamespacePrefix(text string, pos lspPosition) (string, bool) {
	lines := splitLines(text)
	if pos.Line < 0 || pos.Line >= len(lines) {
		return "", false
	}
	line := lines[pos.Line]
	byteIdx := utf16ToByte(line, pos.Character)
	if byteIdx > len(line) {
		byteIdx = len(line)
	}
	prefix := line[:byteIdx]

	// Trim the (possibly empty) partial member: [a-z0-9_]*.
	i := len(prefix)
	for i > 0 && isPartialMemberByte(prefix[i-1]) {
		i--
	}
	if i == 0 || prefix[i-1] != '.' {
		return "", false
	}
	dot := i - 1
	j := dot
	for j > 0 && isIdentByte(prefix[j-1]) {
		j--
	}
	if j == dot {
		return "", false // no identifier immediately before the dot
	}
	if j > 0 && (prefix[j-1] == '.' || isIdentByte(prefix[j-1])) {
		return "", false // not a bare qualifier (chained field access)
	}
	ns := prefix[j:dot]
	if !coreNamespaceSet[ns] {
		return "", false
	}
	return ns, true
}

func isPartialMemberByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
}

func (s *server) handleCompletion(msg *message) {
	var p docRequestParams
	if !s.unmarshalParams(msg, &p) {
		return
	}
	text := s.docs[p.TextDocument.URI]

	if ns, ok := completionNamespacePrefix(text, p.Position); ok {
		items := make([]completionItem, 0, len(types.CoreMembers(ns)))
		for _, m := range types.CoreMembers(ns) {
			detail, _ := types.CoreMemberHover(ns, m)
			items = append(items, completionItem{Label: m, Kind: completionKindFunction, Detail: detail})
		}
		_ = s.conn.respond(msg.ID, items)
		return
	}

	items := []completionItem{}
	// Types (error among them). Control keywords and builtins exclude the type
	// names so each name appears once, under its single scope.
	for _, t := range types.TypeNames() {
		items = append(items, completionItem{Label: t, Kind: completionKindClass, Detail: "type"})
	}
	for _, k := range token.Keywords() {
		if typeNameSet[k] {
			continue
		}
		items = append(items, completionItem{Label: k, Kind: completionKindKeyword, Detail: "keyword"})
	}
	for _, b := range types.BuiltinNames() {
		if typeNameSet[b] {
			continue
		}
		items = append(items, completionItem{Label: b, Kind: completionKindFunction, Detail: "builtin"})
	}
	for _, c := range types.ReservedConstants() {
		items = append(items, completionItem{Label: c, Kind: completionKindConstant, Detail: "constant"})
	}
	// Declared names, when the document currently parses.
	if prog, perr := parser.Parse(text, ""); perr == nil && prog != nil {
		for _, st := range prog.Structs {
			items = append(items, completionItem{Label: st.Name, Kind: completionKindClass, Detail: "struct"})
		}
		for _, en := range prog.Enums {
			items = append(items, completionItem{Label: en.Name, Kind: completionKindEnum, Detail: "enum"})
		}
		for _, a := range prog.Aliases {
			items = append(items, completionItem{Label: a.Name, Kind: completionKindClass, Detail: "type alias"})
		}
		for _, fn := range prog.Funcs {
			items = append(items, completionItem{Label: fn.Name, Kind: completionKindFunction, Detail: funcSignature(fn)})
		}
		info := types.Check(prog)
		seen := map[string]bool{}
		for _, v := range info.Vars {
			if v == nil || seen[v.Name] {
				continue
			}
			seen[v.Name] = true
			items = append(items, completionItem{Label: v.Name, Kind: completionKindVariable, Detail: string(v.Type)})
		}
	}
	_ = s.conn.respond(msg.ID, items)
}
