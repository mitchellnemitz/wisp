// Package format implements `wisp fmt`: the canonical, comment-preserving
// source formatter (spec section 3.1). It parses source to the AST (with the
// lexer's retained comment side channel), then deterministically pretty-prints
// it. Formatting is idempotent: format(format(x)) == format(x).
package format

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/lexer"
	"github.com/mitchellnemitz/wisp/internal/parser"
	"github.com/mitchellnemitz/wisp/internal/token"
)

// indentUnit is one level of indentation: 4 spaces, no tabs (spec 3.1).
const indentUnit = "    "

// Format parses src and returns its canonical formatting. On a parse error it
// returns ("", err) where err is the located lexer/parser error (no output is
// produced for invalid source, spec 3.1). A parser/format drift that reaches
// one of the printer's exhaustiveness-guard panics is recovered here and
// returned as an error rather than crashing the caller.
func Format(src, filename string) (string, error) {
	prog, comments, err := parser.ParseWithComments(src, filename)
	if err != nil {
		return "", err
	}
	return formatProgram(prog, comments)
}

// formatProgram renders prog to its canonical formatting, recovering any
// panic from the printer's exhaustiveness guards into a returned error so a
// parser/format drift never crashes the caller.
func formatProgram(prog *ast.Program, comments []lexer.Comment) (result string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("format: %v", r)
		}
	}()
	p := &printer{comments: comments}
	p.program(prog)
	return p.finish(), nil
}

// printer accumulates formatted output line by line and weaves comments back in
// by source position. Comments are consumed in source order: a full-line
// comment is emitted on its own line at the current block indent before the
// code at or after its line; a trailing comment is appended to the line of the
// code it followed in the source, separated by one space.
type printer struct {
	out      strings.Builder
	comments []lexer.Comment
	ci       int // index of the next unconsumed comment
}

// --- comment weaving ---

// leadingComments emits, at indent depth, every pending full-line comment whose
// source line is strictly before line. Trailing comments are NOT emitted here
// (they belong to a preceding code line and are consumed by trailingComment).
// A pending trailing comment before `line` that was never attached (its code
// line produced no trailing slot) is demoted to a full-line comment so it is
// never lost.
func (p *printer) leadingComments(line, depth int) {
	for p.ci < len(p.comments) && p.comments[p.ci].Pos.Line < line {
		c := p.comments[p.ci]
		p.ci++
		p.writeLine(depth, c.Text)
	}
}

// trailingComment returns the text of a pending trailing comment that sits on
// the given source line, consuming it; it returns ("", false) when none does.
func (p *printer) trailingComment(line int) (string, bool) {
	if p.ci < len(p.comments) && p.comments[p.ci].Trailing && p.comments[p.ci].Pos.Line == line {
		c := p.comments[p.ci]
		p.ci++
		return c.Text, true
	}
	return "", false
}

// declBoundary pulls a top-level closeLine candidate (the next declaration's
// source line) back to exclude a comment run that attaches to that next
// declaration instead of trailing the current one: a maximal run of full-line
// comments ending immediately before declLine with no blank line in between,
// mirroring the adjacency rule internal/doc uses to attach a `///` doc comment
// to its declaration. Without this, flushBlockTail cannot tell a dangling tail
// comment (which belongs inside the current block) from the next
// declaration's leading doc comment (which does not), and sweeps both inside
// the current block ahead of its closing brace.
func (p *printer) declBoundary(declLine int) int {
	end := p.ci
	for end < len(p.comments) && p.comments[end].Pos.Line < declLine {
		end++
	}
	// Walk back over the contiguous run of FULL-LINE comments ending immediately
	// above declLine. A trailing comment in the gap (e.g. on the prior decl's
	// last statement) must not stop the walk -- it is attached to its own line,
	// not part of the next decl's leading run -- so skip it rather than break.
	line := declLine
	for i := end - 1; i >= p.ci && !p.comments[i].Trailing && p.comments[i].Pos.Line == line-1; i-- {
		line = p.comments[i].Pos.Line
	}
	return line
}

// flushBlockTail emits, at depth, every pending full-line comment whose source
// line is strictly before closeLine (the source line that follows this block:
// its closing brace, or the next sibling/outer construct). This keeps an
// end-of-block comment inside its own block instead of letting the next
// outer/sibling leadingComments consume it at the wrong indent (H8/H9/M8). Only
// full-line comments are flushed; a pending trailing comment is left for its
// own code line. closeLine <= 0 means "no following-line information"; nothing
// is flushed.
func (p *printer) flushBlockTail(closeLine, depth int) {
	if closeLine <= 0 {
		return
	}
	for p.ci < len(p.comments) && !p.comments[p.ci].Trailing && p.comments[p.ci].Pos.Line < closeLine {
		c := p.comments[p.ci]
		p.ci++
		p.writeLine(depth, c.Text)
	}
}

// remainingComments flushes any comments left after the last declaration (e.g.
// trailing comments at end of file) as full-line comments at depth 0.
func (p *printer) remainingComments() {
	for p.ci < len(p.comments) {
		p.writeLine(0, p.comments[p.ci].Text)
		p.ci++
	}
}

// --- low-level emit ---

// writeLine writes one indented line of text followed by a newline. text must
// not contain a trailing newline. Trailing whitespace is trimmed so blank-ish
// lines never carry spaces (spec 3.1: no trailing whitespace).
func (p *printer) writeLine(depth int, text string) {
	line := strings.Repeat(indentUnit, depth) + text
	line = strings.TrimRight(line, " \t")
	p.out.WriteString(line)
	p.out.WriteByte('\n')
}

// blank writes a single empty line.
func (p *printer) blank() { p.out.WriteByte('\n') }

// finish returns the accumulated output normalized to exactly one trailing
// newline and no leading blank line (spec 3.1).
func (p *printer) finish() string {
	s := p.out.String()
	s = strings.TrimLeft(s, "\n")
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	return s + "\n"
}

// --- top level ---

// program prints all declarations in source order with exactly one blank line
// between top-level declarations and no leading blank line. Structs and
// functions are interleaved by source position so the output preserves the
// author's declaration order.
func (p *printer) program(prog *ast.Program) {
	type decl struct {
		pos token.Position
		// isDirective marks a module directive (import/include). Adjacent
		// directives with no interleaved comment collapse their blank line
		// (spec: import-spacing rule).
		isDirective bool
		// render emits the declaration. closeLine is the source line of the next
		// top-level declaration (0 for the last), used to keep an end-of-body
		// comment inside the declaration's block (H8/H9/M8).
		render func(closeLine int)
	}
	var decls []decl
	for _, s := range prog.Structs {
		sd := s
		decls = append(decls, decl{sd.Pos(), false, func(int) { p.structDecl(sd) }})
	}
	for _, e := range prog.Enums {
		ed := e
		decls = append(decls, decl{ed.Pos(), false, func(int) { p.enumDecl(ed) }})
	}
	for _, a := range prog.Aliases {
		ad := a
		decls = append(decls, decl{ad.Pos(), false, func(int) { p.typeAliasDecl(ad) }})
	}
	for _, f := range prog.Funcs {
		fn := f
		decls = append(decls, decl{fn.Pos(), false, func(closeLine int) { p.funcDecl(fn, closeLine) }})
	}
	for _, t := range prog.Tests {
		td := t
		decls = append(decls, decl{td.Pos(), false, func(closeLine int) { p.testDecl(td, closeLine) }})
	}
	// Module directives are interleaved in source order (no hoisting), so the
	// author's ordering and comment placement are preserved (M8).
	for _, c := range prog.Consts {
		cd := c
		decls = append(decls, decl{cd.Pos(), false, func(int) { p.constDecl(cd) }})
	}
	for _, im := range prog.Imports {
		d := im
		decls = append(decls, decl{d.Pos(), true, func(int) { p.importDecl(d) }})
	}
	for _, in := range prog.Includes {
		d := in
		decls = append(decls, decl{d.Pos(), true, func(int) { p.includeDecl(d) }})
	}
	sort.SliceStable(decls, func(i, j int) bool {
		a, b := decls[i].pos, decls[j].pos
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Col < b.Col
	})

	for i, d := range decls {
		// Adjacent module directives (import/include) with no comment sitting
		// between them collapse their blank line. p.ci still pointing at a
		// comment strictly before d.pos.Line means that comment sits in the
		// source gap between the two directives (decl[i-1]'s own trailing
		// comment, if any, was already consumed by its render call above), so
		// the collapse is suppressed and the blank kept.
		collapse := i > 0 && d.isDirective && decls[i-1].isDirective &&
			!(p.ci < len(p.comments) && p.comments[p.ci].Pos.Line < d.pos.Line)
		if i > 0 && !collapse {
			p.blank()
		}
		p.leadingComments(d.pos.Line, 0)
		closeLine := 0
		if i+1 < len(decls) {
			closeLine = p.declBoundary(decls[i+1].pos.Line)
		}
		d.render(closeLine)
	}
	p.remainingComments()
}

// importDecl / includeDecl render a module directive canonically: the path is
// re-quoted as a double-quoted string, with an optional `as alias` clause.
func (p *printer) importDecl(d *ast.ImportDecl) {
	line := "import \"" + escapeStringText(d.Path) + "\""
	if d.Alias != "" {
		line += " as " + d.Alias
	}
	p.lineStmt(0, d.Pos().Line, line)
}

func (p *printer) constDecl(cd *ast.ConstDecl) {
	p.lineStmt(0, cd.Pos().Line, "const "+cd.Name+": "+formatType(cd.Type)+" = "+p.expr(cd.Value, 0))
}

func (p *printer) typeAliasDecl(d *ast.TypeAliasDecl) {
	p.lineStmt(0, d.Pos().Line, "type "+d.Name+" = "+formatType(d.Type))
}

func (p *printer) includeDecl(d *ast.IncludeDecl) {
	line := "include \"" + escapeStringText(d.Path) + "\""
	if d.Alias != "" {
		line += " as " + d.Alias
	}
	p.lineStmt(0, d.Pos().Line, line)
}

func (p *printer) structDecl(sd *ast.StructDecl) {
	prefix := ""
	if sd.Exported {
		prefix = "export "
	}
	nameWithParams := sd.Name
	if len(sd.TypeParams) > 0 {
		nameWithParams += "[" + strings.Join(sd.TypeParams, ", ") + "]"
	}
	if len(sd.Fields) == 0 {
		p.writeLine(0, prefix+"struct "+nameWithParams+" {}")
		return
	}
	head := prefix + "struct " + nameWithParams
	if sd.Multiline {
		items := make([]string, len(sd.Fields))
		for i, f := range sd.Fields {
			items[i] = f.Name + ": " + formatType(f.Type)
		}
		p.writeLine(0, head+" {"+p.multilineItems(items, "}", 0))
		return
	}
	var b strings.Builder
	b.WriteString(head)
	b.WriteString(" { ")
	for i, f := range sd.Fields {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(f.Name)
		b.WriteString(": ")
		b.WriteString(formatType(f.Type))
	}
	b.WriteString(" }")
	p.writeLine(0, b.String())
}

// enumDecl renders an enum declaration: single-line `enum Name { V, V = n }` when
// the source was single-line, or multi-line (one variant per line at depth 1 with
// a trailing comma, closer at depth 0) per the Multiline flag, mirroring
// structDecl. An explicit `= value` is preserved. An exported enum gets the
// `export ` prefix, mirroring export struct/fn/const.
func (p *printer) enumDecl(ed *ast.EnumDecl) {
	variant := func(v ast.EnumVariant) string {
		name := v.Name
		if v.Payload != "" {
			name += "(" + string(v.Payload) + ")"
		}
		if v.Value != nil {
			return name + " = " + p.expr(v.Value, 0)
		}
		return name
	}
	prefix := ""
	if ed.Exported {
		prefix = "export "
	}
	head := prefix + "enum " + ed.Name
	if ed.Backing != "" {
		head += ": " + string(ed.Backing)
	}
	if ed.Multiline {
		items := make([]string, len(ed.Variants))
		for i, v := range ed.Variants {
			items[i] = variant(v)
		}
		p.writeLine(0, head+" {"+p.multilineItems(items, "}", 0))
		return
	}
	var b strings.Builder
	b.WriteString(head)
	b.WriteString(" { ")
	for i, v := range ed.Variants {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(variant(v))
	}
	b.WriteString(" }")
	p.writeLine(0, b.String())
}

func (p *printer) funcDecl(fn *ast.FuncDecl, closeLine int) {
	var b strings.Builder
	if fn.Exported {
		b.WriteString("export ")
	}
	b.WriteString("fn ")
	b.WriteString(fn.Name)
	if len(fn.TypeParams) > 0 {
		b.WriteByte('[')
		for i, tp := range fn.TypeParams {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(tp)
			if bound, ok := fn.TypeParamBounds[tp]; ok && bound != "" {
				b.WriteString(": ")
				b.WriteString(bound)
			}
		}
		b.WriteByte(']')
	}
	b.WriteByte('(')
	for i, param := range fn.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(param.Name)
		b.WriteString(": ")
		b.WriteString(formatType(param.Type))
		if param.Default != nil {
			b.WriteString(" = ")
			b.WriteString(p.expr(param.Default, 0))
		}
	}
	b.WriteString(") -> ")
	b.WriteString(formatType(fn.RetType))
	b.WriteString(" {")
	p.openBlock(fn.KwPos, b.String(), 0, fn.Body, closeLine)
}

// testDecl prints a `test ("name") { ... }` declaration. The name is a static
// string literal; it is re-quoted from the decoded TestDecl.Name with the same
// escaping the lexer recognizes. The body is printed like a function body.
func (p *printer) testDecl(d *ast.TestDecl, closeLine int) {
	head := `test ("` + escapeStringText(d.Name) + `") {`
	p.openBlock(d.KwPos, head, 0, d.Body, closeLine)
}

// openBlock writes the `<head> {` opening line (head already ends with " {"),
// emits the body at depth+1, and closes with `}` at depth. headPos.Line is the
// source line of the construct, used to attach a trailing comment to the
// opening line. closeLine is the source line of this block's closing brace (or
// the next construct that follows it); it bounds the end-of-block comment flush
// so a tail comment stays inside the block (H8/H9/M8).
func (p *printer) openBlock(headPos token.Position, head string, depth int, body []ast.Stmt, closeLine int) {
	if tc, ok := p.trailingComment(headPos.Line); ok {
		head += " " + tc
	}
	p.writeLine(depth, head)
	p.block(body, depth+1, closeLine)
	p.writeLine(depth, "}")
}

// block prints a statement list at depth, weaving leading/trailing comments.
// closeLine is the source line that follows the block (its closing brace or the
// next sibling/outer construct). Each statement is given the line of the next
// statement (or closeLine for the last) so a nested block it opens flushes its
// own trailing comments before its `}` rather than leaking them outward. After
// the last statement, any remaining full-line comments before closeLine are
// emitted here at this block's depth (this is the actual H8/H9/M8 fix; an empty
// block flushes a sole body comment the same way).
func (p *printer) block(stmts []ast.Stmt, depth, closeLine int) {
	for i, s := range stmts {
		next := closeLine
		if i+1 < len(stmts) {
			next = stmts[i+1].Pos().Line
		}
		p.stmt(s, depth, next)
	}
	p.flushBlockTail(closeLine, depth)
}

// --- statements ---

// stmt prints one statement at depth. closeLine is the source line that follows
// this statement at the same level (the next statement, or the enclosing
// block's closing brace for the last statement); a statement that opens a
// nested block uses it as that block's closeLine so an end-of-block comment is
// flushed inside the nested block (H8/H9/M8).
func (p *printer) stmt(s ast.Stmt, depth, closeLine int) {
	// Emit any full-line comments that precede this statement.
	p.leadingComments(s.Pos().Line, depth)

	switch n := s.(type) {
	case *ast.LetStmt:
		p.lineStmt(depth, n.Pos().Line, "let "+n.Name+": "+formatType(n.Type)+" = "+p.expr(n.Value, depth))
	case *ast.ConstStmt:
		p.lineStmt(depth, n.Pos().Line, "const "+n.Name+": "+formatType(n.Type)+" = "+p.expr(n.Value, depth))
	case *ast.FinalStmt:
		p.lineStmt(depth, n.Pos().Line, "final "+n.Name+": "+formatType(n.Type)+" = "+p.expr(n.Value, depth))
	case *ast.TupleBindStmt:
		p.lineStmt(depth, lastLine(n.Value, n.Pos().Line), p.tupleBind(n, depth))
	case *ast.AssignStmt:
		p.lineStmt(depth, n.Pos().Line, n.Name+" = "+p.expr(n.Value, depth))
	case *ast.FieldAssignStmt:
		p.lineStmt(depth, lastLine(n.Value, n.Pos().Line), p.expr(n.Target, depth)+"."+n.Field+" = "+p.expr(n.Value, depth))
	case *ast.IndexAssignStmt:
		p.lineStmt(depth, lastLine(n.Value, n.Pos().Line), p.expr(n.Target, depth)+"["+p.expr(n.Index, depth)+"] = "+p.expr(n.Value, depth))
	case *ast.ReturnStmt:
		if n.Value == nil {
			p.lineStmt(depth, n.Pos().Line, "return")
		} else {
			p.lineStmt(depth, lastLine(n.Value, n.Pos().Line), "return "+p.expr(n.Value, depth))
		}
	case *ast.BreakStmt:
		p.lineStmt(depth, n.Pos().Line, "break")
	case *ast.ContinueStmt:
		p.lineStmt(depth, n.Pos().Line, "continue")
	case *ast.ThrowStmt:
		p.lineStmt(depth, lastLine(n.X, n.Pos().Line), "throw "+p.expr(n.X, depth))
	case *ast.ExprStmt:
		p.lineStmt(depth, lastLine(n.X, n.Pos().Line), p.expr(n.X, depth))
	case *ast.IfStmt:
		p.ifStmt(n, depth, closeLine)
	case *ast.WhileStmt:
		p.openBlock(n.KwPos, "while ("+p.expr(n.Cond, depth)+") {", depth, n.Body, closeLine)
	case *ast.ForStmt:
		p.forStmt(n, depth, closeLine)
	case *ast.ForInStmt:
		p.openBlock(n.KwPos, "for ("+n.Var+" in "+p.expr(n.Coll, depth)+") {", depth, n.Body, closeLine)
	case *ast.SwitchStmt:
		p.switchStmt(n, depth, closeLine)
	case *ast.TryStmt:
		p.tryStmt(n, depth, closeLine)
	case *ast.MatchStmt:
		p.matchStmt(n, depth, closeLine)
	}
}

// tupleBind renders a tuple-destructuring binding on one line:
// `let (a: int, b: string) = <value>` (or `final (...)`). A binding slot prints
// `name: Type`, a bare discard prints `_`, an annotated discard prints
// `_: Type`. The pattern is always single-line; the value is rendered by the
// depth-aware expr printer so a multi-line RHS literal still composes.
func (p *printer) tupleBind(n *ast.TupleBindStmt, depth int) string {
	var b strings.Builder
	if n.Final {
		b.WriteString("final (")
	} else {
		b.WriteString("let (")
	}
	for i, s := range n.Slots {
		if i > 0 {
			b.WriteString(", ")
		}
		switch {
		case s.Blank && s.Type == "":
			b.WriteByte('_')
		case s.Blank:
			b.WriteString("_: ")
			b.WriteString(formatType(s.Type))
		default:
			b.WriteString(s.Name)
			b.WriteString(": ")
			b.WriteString(formatType(s.Type))
		}
	}
	b.WriteString(") = ")
	b.WriteString(p.expr(n.Value, depth))
	return b.String()
}

// lineStmt emits a single-line statement, attaching a trailing comment found on
// codeLine (the source line where this statement's code ends).
func (p *printer) lineStmt(depth, codeLine int, text string) {
	if tc, ok := p.trailingComment(codeLine); ok {
		text += " " + tc
	}
	p.writeLine(depth, text)
}

// firstStmtLine returns the source line of the first statement in stmts, or 0
// when the block is empty.
func firstStmtLine(stmts []ast.Stmt) int {
	if len(stmts) == 0 {
		return 0
	}
	return stmts[0].Pos().Line
}

func (p *printer) ifStmt(n *ast.IfStmt, depth, closeLine int) {
	head := "if (" + p.expr(n.Cond, depth) + ") {"
	if tc, ok := p.trailingComment(n.KwPos.Line); ok {
		head += " " + tc
	}
	p.writeLine(depth, head)

	// The boundary that follows the then-block is the first `} else if`/`} else`
	// arm head, falling back to the whole construct's closeLine. branchBound
	// resolves that line for arm index i (in n.ElseIfs) onward.
	branchBound := func(i int) int {
		for ; i < len(n.ElseIfs); i++ {
			if l := n.ElseIfs[i].Cond.Pos().Line; l > 0 {
				return l
			}
		}
		if n.Else != nil {
			if l := firstStmtLine(n.Else); l > 0 {
				return l
			}
		}
		return closeLine
	}

	p.block(n.Then, depth+1, branchBound(0))

	for i, ei := range n.ElseIfs {
		mid := "} else if (" + p.expr(ei.Cond, depth) + ") {"
		p.writeLine(depth, mid)
		p.block(ei.Body, depth+1, branchBound(i+1))
	}
	if n.Else != nil {
		p.writeLine(depth, "} else {")
		p.block(n.Else, depth+1, closeLine)
	}
	p.writeLine(depth, "}")
}

func (p *printer) forStmt(n *ast.ForStmt, depth, closeLine int) {
	var b strings.Builder
	b.WriteString("for (")
	if n.Init != nil {
		b.WriteString(p.simpleStmtText(n.Init, depth))
	}
	b.WriteString("; ")
	if n.Cond != nil {
		b.WriteString(p.expr(n.Cond, depth))
	}
	b.WriteString("; ")
	if n.Post != nil {
		b.WriteString(p.simpleStmtText(n.Post, depth))
	}
	b.WriteString(") {")
	p.openBlock(n.KwPos, b.String(), depth, n.Body, closeLine)
}

// simpleStmtText renders an init/post clause of a C-style for header inline (no
// indent, no newline, no trailing comment).
func (p *printer) simpleStmtText(s ast.Stmt, depth int) string {
	switch n := s.(type) {
	case *ast.LetStmt:
		return "let " + n.Name + ": " + formatType(n.Type) + " = " + p.expr(n.Value, depth)
	case *ast.TupleBindStmt:
		return p.tupleBind(n, depth)
	case *ast.AssignStmt:
		return n.Name + " = " + p.expr(n.Value, depth)
	case *ast.FieldAssignStmt:
		return p.expr(n.Target, depth) + "." + n.Field + " = " + p.expr(n.Value, depth)
	case *ast.IndexAssignStmt:
		return p.expr(n.Target, depth) + "[" + p.expr(n.Index, depth) + "] = " + p.expr(n.Value, depth)
	case *ast.ExprStmt:
		return p.expr(n.X, depth)
	default:
		panic(fmt.Sprintf("simpleStmtText: no printer case for %T (parser/format drift)", n))
	}
}

func (p *printer) switchStmt(n *ast.SwitchStmt, depth, closeLine int) {
	head := "switch (" + p.expr(n.Subject, depth) + ") {"
	if tc, ok := p.trailingComment(n.KwPos.Line); ok {
		head += " " + tc
	}
	p.writeLine(depth, head)

	// caseHeadLine is the source line of a case clause's head (its first value),
	// or 0 when unknown. The boundary that follows a case body is the next case
	// head, then the default body's first statement, then the switch closeLine.
	caseHeadLine := func(cs ast.SwitchCase) int {
		if len(cs.Values) > 0 {
			return cs.Values[0].Pos().Line
		}
		return 0
	}
	caseBound := func(i int) int {
		for ; i < len(n.Cases); i++ {
			if l := caseHeadLine(n.Cases[i]); l > 0 {
				return l
			}
		}
		if n.Default != nil {
			if l := firstStmtLine(n.Default); l > 0 {
				return l
			}
		}
		return closeLine
	}

	for ci, cs := range n.Cases {
		var vals []string
		for _, v := range cs.Values {
			vals = append(vals, p.expr(v, depth+1))
		}
		caseHead := "case " + strings.Join(vals, ", ") + " {"
		caseLine := depth + 1
		if len(cs.Values) > 0 {
			if tc, ok := p.trailingComment(cs.Values[0].Pos().Line); ok {
				caseHead += " " + tc
			}
		}
		p.writeLine(caseLine, caseHead)
		p.block(cs.Body, caseLine+1, caseBound(ci+1))
		p.writeLine(caseLine, "}")
	}
	if n.Default != nil {
		p.writeLine(depth+1, "default {")
		p.block(n.Default, depth+2, closeLine)
		p.writeLine(depth+1, "}")
	}
	p.writeLine(depth, "}")
}

func (p *printer) tryStmt(n *ast.TryStmt, depth, closeLine int) {
	head := "try {"
	if tc, ok := p.trailingComment(n.KwPos.Line); ok {
		head += " " + tc
	}
	p.writeLine(depth, head)
	// The try body is bounded by the `} catch` line; the catch body by the
	// `} finally` line (or the construct closeLine when there is no finally).
	p.block(n.Body, depth+1, n.CatchPos.Line)
	p.writeLine(depth, "} catch ("+n.CatchVar+") {")
	catchBound := closeLine
	if n.HasFinally {
		catchBound = firstStmtLine(n.Finally)
		if catchBound <= 0 {
			catchBound = closeLine
		}
	}
	p.block(n.Catch, depth+1, catchBound)
	if n.HasFinally {
		p.writeLine(depth, "} finally {")
		p.block(n.Finally, depth+1, closeLine)
	}
	p.writeLine(depth, "}")
}

func (p *printer) matchStmt(n *ast.MatchStmt, depth, closeLine int) {
	head := "match (" + p.expr(n.Scrutinee, depth) + ") {"
	if tc, ok := p.trailingComment(n.KwPos.Line); ok {
		head += " " + tc
	}
	p.writeLine(depth, head)
	// An arm body is bounded by the next arm's pattern line (its `} case <pat> {`
	// boundary), falling back to the match closeLine for the last arm.
	armBound := func(i int) int {
		for ; i < len(n.Arms); i++ {
			if l := patternLine(n.Arms[i].Pattern); l > 0 {
				return l
			}
		}
		return closeLine
	}
	for ai, arm := range n.Arms {
		pat := p.matchPattern(arm.Pattern)
		armHead := "case " + pat + " {"
		if patLine := patternLine(arm.Pattern); patLine > 0 {
			if tc, ok := p.trailingComment(patLine); ok {
				armHead += " " + tc
			}
		}
		p.writeLine(depth+1, armHead)
		p.block(arm.Body, depth+2, armBound(ai+1))
		p.writeLine(depth+1, "}")
	}
	p.writeLine(depth, "}")
}

// patternLine returns the source line of a match arm pattern, or 0 if unknown.
func patternLine(pat ast.MatchPattern) int {
	switch p := pat.(type) {
	case *ast.ConstructorPat:
		return p.VariantPos.Line
	case *ast.WildcardPat:
		return p.Pos.Line
	}
	return 0
}

func (p *printer) matchPattern(pat ast.MatchPattern) string {
	switch n := pat.(type) {
	case *ast.ConstructorPat:
		if n.Name == "" {
			return n.Variant
		}
		return n.Variant + "(" + n.Name + ")"
	case *ast.WildcardPat:
		return "_"
	default:
		panic(fmt.Sprintf("matchPattern: no printer case for %T (parser/format drift)", n))
	}
}

// --- expressions ---

// expr renders e at the enclosing indent depth. A single-line expression
// returns one line with no leading indentation (M6 does no line wrapping). A
// multi-line collection/struct literal returns a string whose FIRST line is bare
// (the caller indents the opener at depth) and whose CONTINUATION lines carry
// their full absolute indentation baked in, so nesting composes (R9).
func (p *printer) expr(e ast.Expr, depth int) string {
	switch n := e.(type) {
	case *ast.IntLit:
		return n.Raw
	case *ast.FloatLit:
		return n.Raw
	case *ast.BoolLit:
		if n.Value {
			return "true"
		}
		return "false"
	case *ast.StringLit:
		return p.stringLit(n, depth)
	case *ast.Ident:
		return n.Name
	case *ast.UnaryExpr:
		// A unary operator binds tighter than any binary operator, so a binary
		// (or unary) operand must be parenthesized to preserve the tree, and the
		// re-parse, exactly (e.g. -(a + b), !(a && b)). The operator binds to its
		// operand with no space (spec: no space inside the operator).
		return n.Op.String() + p.operand(n.X, unaryPrec, depth)
	case *ast.BinaryExpr:
		prec := binPrec(n.Op)
		// Left operand: parenthesize only when strictly lower precedence (binary
		// operators are left-associative, so equal precedence on the left needs no
		// parens). Right operand: parenthesize at equal-or-lower precedence to keep
		// left-associativity (a - (b - c) must keep its parens).
		left := p.operand(n.L, prec, depth)
		right := p.operandRight(n.R, prec, depth)
		return left + " " + n.Op.String() + " " + right
	case *ast.CallExpr:
		return p.callExpr(n, depth)
	case *ast.StructLit:
		return p.structLit(n, depth)
	case *ast.ArrayLit:
		return p.arrayLit(n, depth)
	case *ast.DictLit:
		return p.dictLit(n, depth)
	case *ast.FieldAccess:
		return p.expr(n.X, depth) + "." + n.Field
	case *ast.TupleLit:
		s := "("
		for i, el := range n.Elems {
			if i > 0 {
				s += ", "
			}
			s += p.expr(el, depth)
		}
		return s + ")"
	case *ast.IndexExpr:
		return p.expr(n.X, depth) + "[" + p.expr(n.Index, depth) + "]"
	default:
		panic(fmt.Sprintf("expr: no printer case for %T (parser/format drift)", n))
	}
}

// unaryPrec is the binding precedence of a unary operator: tighter than every
// binary operator (which top out at 6 in binPrec).
const unaryPrec = 7

// binPrec mirrors the parser's binary-operator precedence (higher binds
// tighter); 0 for a non-binary token.
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
	case token.Plus, token.Minus:
		return 5
	case token.Star, token.Slash, token.Percent:
		return 6
	default:
		return 0
	}
}

// exprPrec is the precedence of an expression for parenthesization: a binary
// expression's operator precedence, a unary expression's unaryPrec, else a high
// value (atoms/postfix bind tightest and never need parens).
func exprPrec(e ast.Expr) int {
	switch n := e.(type) {
	case *ast.BinaryExpr:
		return binPrec(n.Op)
	case *ast.UnaryExpr:
		return unaryPrec
	default:
		return 100
	}
}

// operand parenthesizes child when its precedence is strictly below min. It
// serves both the unary operand and a binary expression's left child: both
// parenthesize only on strictly-lower precedence (a unary operator binds
// tighter than any binary operator; binary operators are left-associative, so
// equal precedence on the left needs no parens).
func (p *printer) operand(child ast.Expr, min, depth int) string {
	if exprPrec(child) < min {
		return "(" + p.expr(child, depth) + ")"
	}
	return p.expr(child, depth)
}

// operandRight renders a binary expression's right child: parenthesize when its
// precedence is lower than OR EQUAL to the parent's, preserving left
// associativity (a - (b - c) keeps parens; a - b - c does not gain any).
func (p *printer) operandRight(child ast.Expr, parentPrec, depth int) string {
	if exprPrec(child) <= parentPrec {
		return "(" + p.expr(child, depth) + ")"
	}
	return p.expr(child, depth)
}

func (p *printer) callExpr(n *ast.CallExpr, depth int) string {
	var b strings.Builder
	b.WriteString(p.callee(n, depth))
	if len(n.TypeArgs) > 0 {
		b.WriteByte('[')
		for i, ta := range n.TypeArgs {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(FormatType(ta.Name))
		}
		b.WriteByte(']')
	}
	b.WriteByte('(')
	for i, a := range n.Args {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.expr(a, depth))
	}
	b.WriteByte(')')
	return b.String()
}

// callee renders the callee expression. A bare-identifier callee uses its name;
// any other callee is rendered as an expression.
func (p *printer) callee(n *ast.CallExpr, depth int) string {
	if n.CalleeName != "" {
		return n.CalleeName
	}
	return p.expr(n.Callee, depth)
}

// multilineItems renders the shared multi-line body for a collection/struct
// literal whose opener has already been written (bare) by the caller: each item
// on its own line at indent (depth+1), a trailing comma after every item
// (including the last), then the closer on its own line at indent depth. Items
// are rendered at depth+1 so a nested multi-line literal indents one level
// deeper (R9: nesting composes). The returned string starts with "\n".
func (p *printer) multilineItems(items []string, closer string, depth int) string {
	var b strings.Builder
	inner := strings.Repeat(indentUnit, depth+1)
	for _, item := range items {
		b.WriteByte('\n')
		b.WriteString(inner)
		b.WriteString(item)
		b.WriteByte(',')
	}
	b.WriteByte('\n')
	b.WriteString(strings.Repeat(indentUnit, depth))
	b.WriteString(closer)
	return b.String()
}

func (p *printer) structLit(n *ast.StructLit, depth int) string {
	name := n.Name
	if n.Namespace != "" {
		name = n.Namespace + "." + n.Name
	}
	if len(n.Fields) == 0 {
		return name + " {}"
	}
	field := func(f ast.StructLitField) string {
		return f.Name + ": " + p.expr(f.Value, depth+1)
	}
	if n.Multiline {
		items := make([]string, len(n.Fields))
		for i, f := range n.Fields {
			items[i] = field(f)
		}
		return name + " {" + p.multilineItems(items, "}", depth)
	}
	var b strings.Builder
	b.WriteString(name)
	b.WriteString(" { ")
	for i, f := range n.Fields {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(field(f))
	}
	b.WriteString(" }")
	return b.String()
}

func (p *printer) arrayLit(n *ast.ArrayLit, depth int) string {
	if n.Multiline && len(n.Elems) > 0 {
		items := make([]string, len(n.Elems))
		for i, e := range n.Elems {
			items[i] = p.expr(e, depth+1)
		}
		return "[" + p.multilineItems(items, "]", depth)
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, e := range n.Elems {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.expr(e, depth))
	}
	b.WriteByte(']')
	return b.String()
}

func (p *printer) dictLit(n *ast.DictLit, depth int) string {
	if len(n.Entries) == 0 {
		return "{}"
	}
	entry := func(e ast.DictLitEntry) string {
		return p.expr(e.Key, depth+1) + ": " + p.expr(e.Value, depth+1)
	}
	if n.Multiline {
		items := make([]string, len(n.Entries))
		for i, e := range n.Entries {
			items[i] = entry(e)
		}
		return "{" + p.multilineItems(items, "}", depth)
	}
	var b strings.Builder
	b.WriteString("{ ")
	for i, e := range n.Entries {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(entry(e))
	}
	b.WriteString(" }")
	return b.String()
}

// stringLit renders a string literal. A single-quoted literal (one text part,
// no interpolation) round-trips as a double-quoted literal in canonical form so
// there is one canonical string form; escapes are re-applied. A literal with
// interpolation parts renders as "...${expr}...".
func (p *printer) stringLit(n *ast.StringLit, depth int) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, part := range n.Parts {
		if part.IsText() {
			b.WriteString(escapeStringText(part.Text))
		} else {
			b.WriteString("${")
			b.WriteString(p.expr(part.Expr, depth))
			b.WriteByte('}')
		}
	}
	b.WriteByte('"')
	return b.String()
}

// escapeStringText re-encodes decoded string bytes back into double-quoted
// string source, applying the escapes the lexer recognizes (\n \t \" \\ and a
// literal $ as \$ so it is never read as an interpolation opener).
func escapeStringText(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '$':
			b.WriteString(`\$`)
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// lastLine returns the source line where expression e ends, falling back to
// def when e has no nested rightmost position more useful than its own. Because
// the formatter does not wrap, a multi-line source expression collapses to one
// line; the trailing-comment slot uses the expression's last source line so a
// trailing comment written after the whole statement still attaches.
func lastLine(e ast.Expr, def int) int {
	l := rightmostLine(e)
	if l > def {
		return l
	}
	return def
}

// rightmostLine returns the largest source line touched by e (its own position
// or any nested operand/argument position). It is a best-effort lower bound for
// where a trailing comment after the statement could sit.
func rightmostLine(e ast.Expr) int {
	max := e.Pos().Line
	consider := func(x ast.Expr) {
		if x == nil {
			return
		}
		if l := rightmostLine(x); l > max {
			max = l
		}
	}
	switch n := e.(type) {
	case *ast.UnaryExpr:
		consider(n.X)
	case *ast.BinaryExpr:
		consider(n.L)
		consider(n.R)
	case *ast.CallExpr:
		consider(n.Callee)
		for _, a := range n.Args {
			consider(a)
		}
	case *ast.FieldAccess:
		consider(n.X)
	case *ast.IndexExpr:
		consider(n.X)
		consider(n.Index)
	case *ast.ArrayLit:
		for _, x := range n.Elems {
			consider(x)
		}
	case *ast.TupleLit:
		for _, x := range n.Elems {
			consider(x)
		}
	case *ast.DictLit:
		for _, en := range n.Entries {
			consider(en.Key)
			consider(en.Value)
		}
	case *ast.StructLit:
		for _, f := range n.Fields {
			consider(f.Value)
		}
	case *ast.StringLit:
		for _, part := range n.Parts {
			if !part.IsText() {
				consider(part.Expr)
			}
		}
	}
	return max
}

// FormatType renders a type annotation's internal encoding into its canonical
// surface spelling (e.g. "[int]" -> "int[]"). It is the single shared entry
// point for the non-formatter display paths (wisp doc, LSP hover) so the
// prefix-to-postfix array translation lives in exactly one place.
func FormatType(t ast.TypeName) string { return formatType(t) }

// formatType delegates to ast.CanonicalType, the relocated renderer.
func formatType(t ast.TypeName) string { return ast.CanonicalType(t) }
