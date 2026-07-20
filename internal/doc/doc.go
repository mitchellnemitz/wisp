// Package doc extracts /// doc-comments from wisp source and renders them as
// canonical Markdown. It is TOOL-ONLY: it reads the AST + comments and does not
// touch the lexer, parser, AST, or codegen. It calls the formatter's
// FormatType to render type annotations in their canonical surface spelling
// (postfix arrays), sharing that one translation rather than duplicating it.
package doc

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/format"
	"github.com/mitchellnemitz/wisp/internal/lexer"
)

// isDocComment reports whether text is a /// doc-comment (not //// or more).
func isDocComment(text string) bool {
	if !strings.HasPrefix(text, "///") {
		return false
	}
	return len(text) == 3 || text[3] != '/'
}

// stripDoc strips the /// prefix and one optional leading space.
func stripDoc(text string) string {
	return strings.TrimPrefix(strings.TrimPrefix(text, "///"), " ")
}

// attachDoc returns the maximal run of consecutive full-line /// comments
// ending at anchorLine-1, walking upward from that line. Returns "" when none.
func attachDoc(anchorLine int, comments []lexer.Comment) string {
	byLine := make(map[int]lexer.Comment, len(comments))
	for _, c := range comments {
		if !c.Trailing {
			byLine[c.Pos.Line] = c
		}
	}
	var lines []string
	for ln := anchorLine - 1; ; ln-- {
		c, ok := byLine[ln]
		if !ok || !isDocComment(c.Text) {
			break
		}
		lines = append([]string{stripDoc(c.Text)}, lines...)
	}
	return strings.Join(lines, "\n")
}

// declView is a uniform view over the four typed decl slices.
type declView struct {
	name   string
	kwLine int
	kwCol  int
	sig    string
}

func collect(prog *ast.Program) []declView {
	var ds []declView
	for _, f := range prog.Funcs {
		ds = append(ds, viewFunc(f))
	}
	for _, s := range prog.Structs {
		ds = append(ds, viewStruct(s))
	}
	for _, e := range prog.Enums {
		ds = append(ds, viewEnum(e))
	}
	for _, a := range prog.Aliases {
		ds = append(ds, viewAlias(a))
	}
	for _, c := range prog.Consts {
		ds = append(ds, viewConst(c))
	}
	sort.SliceStable(ds, func(i, j int) bool {
		if ds[i].kwLine != ds[j].kwLine {
			return ds[i].kwLine < ds[j].kwLine
		}
		return ds[i].kwCol < ds[j].kwCol
	})
	return ds
}

func viewFunc(f *ast.FuncDecl) declView {
	var b strings.Builder
	if f.Exported {
		b.WriteString("export ")
	}
	b.WriteString("fn ")
	b.WriteString(f.Name)
	if len(f.TypeParams) > 0 {
		b.WriteByte('[')
		for i, tp := range f.TypeParams {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(tp)
			if bound, ok := f.TypeParamBounds[tp]; ok {
				b.WriteString(": ")
				b.WriteString(bound)
			}
		}
		b.WriteByte(']')
	}
	b.WriteByte('(')
	for i, p := range f.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Name)
		b.WriteString(": ")
		b.WriteString(format.FormatType(p.Type))
	}
	b.WriteString(") -> ")
	b.WriteString(format.FormatType(f.RetType))
	return declView{name: f.Name, kwLine: f.KwPos.Line, kwCol: f.KwPos.Col, sig: b.String()}
}

func viewStruct(s *ast.StructDecl) declView {
	var b strings.Builder
	if s.Exported {
		b.WriteString("export ")
	}
	b.WriteString("struct ")
	b.WriteString(s.Name)
	if len(s.TypeParams) > 0 {
		b.WriteByte('[')
		for i, tp := range s.TypeParams {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(tp)
		}
		b.WriteByte(']')
	}
	b.WriteString(" { ")
	for i, f := range s.Fields {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(f.Name)
		b.WriteString(": ")
		b.WriteString(format.FormatType(f.Type))
	}
	b.WriteString(" }")
	return declView{name: s.Name, kwLine: s.KwPos.Line, kwCol: s.KwPos.Col, sig: b.String()}
}

func viewEnum(e *ast.EnumDecl) declView {
	var b strings.Builder
	b.WriteString("enum ")
	b.WriteString(e.Name)
	b.WriteString(" { ")
	for i, v := range e.Variants {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(v.Name)
	}
	b.WriteString(" }")
	return declView{name: e.Name, kwLine: e.KwPos.Line, kwCol: e.KwPos.Col, sig: b.String()}
}

func viewAlias(a *ast.TypeAliasDecl) declView {
	sig := "type " + a.Name + " = " + format.FormatType(a.Type)
	return declView{name: a.Name, kwLine: a.KwPos.Line, kwCol: a.KwPos.Col, sig: sig}
}

func viewConst(c *ast.ConstDecl) declView {
	var b strings.Builder
	if c.Exported {
		b.WriteString("export ")
	}
	b.WriteString("const ")
	b.WriteString(c.Name)
	b.WriteString(": ")
	b.WriteString(string(c.Type))
	return declView{name: c.Name, kwLine: c.KwPos.Line, kwCol: c.KwPos.Col, sig: b.String()}
}

// DocEntry is the structured extraction result (spec R1).
type DocEntry struct {
	Name string
	Sig  string
	Doc  string // attached /// text; "" if undocumented
}

// Extract returns one DocEntry per top-level fn/struct/enum/const in source
// order, with each entry's Doc field set to the attached /// text (or "").
func Extract(prog *ast.Program, comments []lexer.Comment) []DocEntry {
	ds := collect(prog)
	out := make([]DocEntry, len(ds))
	for i, d := range ds {
		out[i] = DocEntry{Name: d.name, Sig: d.sig, Doc: attachDoc(d.kwLine, comments)}
	}
	return out
}

// Render returns the canonical Markdown section for path, ending with a single
// trailing newline. Blocks are joined with "\n" (one blank line between them).
func Render(path string, prog *ast.Program, comments []lexer.Comment) string {
	entries := Extract(prog, comments)
	blocks := make([]string, len(entries))
	for i, e := range entries {
		blk := "### " + e.Name + "\n\n```\n" + e.Sig + "\n```\n"
		if e.Doc != "" {
			blk += "\n" + e.Doc + "\n"
		}
		blocks[i] = blk
	}
	return "## " + filepath.Clean(path) + "\n\n" + strings.Join(blocks, "\n")
}
