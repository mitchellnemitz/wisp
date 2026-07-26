// internal/fuzz/print.go
package fuzz

import (
	"fmt"
	"sort"
	"strings"
)

func (t Type) String() string {
	switch t.Kind {
	case KInt:
		return "int"
	case KFloat:
		return "float"
	case KBool:
		return "bool"
	case KString:
		return "string"
	case KArray:
		return t.Elem.String() + "[]"
	case KDict:
		return "{string: " + t.Elem.String() + "}"
	default:
		return "int"
	}
}

// Print renders a Program as compilable wisp source: sorted imports, then a
// `fn main() -> int` whose body is the statements followed by `return 0`.
func Print(p *Program) string {
	var b strings.Builder
	imps := append([]string(nil), p.Imports...)
	sort.Strings(imps)
	for _, imp := range imps {
		fmt.Fprintf(&b, "import %q\n", imp)
	}
	b.WriteString("fn main() -> int {\n")
	for _, s := range p.Body {
		printStmt(&b, s, 1)
	}
	b.WriteString("  return 0\n")
	b.WriteString("}\n")
	return b.String()
}

func indent(b *strings.Builder, depth int) {
	for i := 0; i < depth; i++ {
		b.WriteString("  ")
	}
}

func printStmt(b *strings.Builder, s Stmt, depth int) {
	switch n := s.(type) {
	case *LetStmt:
		indent(b, depth)
		fmt.Fprintf(b, "let %s: %s = %s\n", n.Name, n.T.String(), printExpr(n.Init))
	case *PrintStmt:
		indent(b, depth)
		fmt.Fprintf(b, "print(\"${%s}\")\n", printExpr(n.Arg))
	case *IfStmt:
		indent(b, depth)
		fmt.Fprintf(b, "if (%s) {\n", printExpr(n.Cond))
		for _, t := range n.Then {
			printStmt(b, t, depth+1)
		}
		indent(b, depth)
		b.WriteString("}")
		if len(n.Else) > 0 {
			b.WriteString(" else {\n")
			for _, e := range n.Else {
				printStmt(b, e, depth+1)
			}
			indent(b, depth)
			b.WriteString("}")
		}
		b.WriteString("\n")
	}
}

func printExpr(e Expr) string {
	switch n := e.(type) {
	case *IntLit:
		return n.V
	case *FloatLit:
		return n.V
	case *BoolLit:
		if n.V {
			return "true"
		}
		return "false"
	case *StringLit:
		return escapeWispString(n.V)
	case *Var:
		return n.Name
	case *Binary:
		return "(" + printExpr(n.L) + " " + n.Op + " " + printExpr(n.R) + ")"
	case *ArrayLit:
		parts := make([]string, len(n.Elems))
		for i, el := range n.Elems {
			parts[i] = printExpr(el)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *DictLit:
		parts := make([]string, len(n.Keys))
		for i := range n.Keys {
			parts[i] = escapeWispString(n.Keys[i]) + ": " + printExpr(n.Vals[i])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case *Index:
		return printExpr(n.Base) + "[" + printExpr(n.Key) + "]"
	case *Call:
		parts := make([]string, len(n.Args))
		for i, a := range n.Args {
			parts[i] = printExpr(a)
		}
		return n.Builtin + "(" + strings.Join(parts, ", ") + ")"
	default:
		return ""
	}
}

// escapeWispString renders s as a wisp double-quoted string literal. wisp
// interpolation treats ${...} specially and \ as an escape, so backslash, quote,
// and $ are escaped; newline/tab become \n/\t. Confirm the exact escape set against
// the lexer's string-literal scanner (internal/lexer/lexer.go) and the interpolation
// handling in the parser before finalizing; the compile test (Task 6,
// TestGeneratedProgramsAllCompile) is the backstop that every emitted literal parses.
func escapeWispString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '$':
			b.WriteString(`\$`)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}
