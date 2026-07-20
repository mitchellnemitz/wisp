package parser

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
)

// TestParseTupleType: tuple TYPE annotations in return and parameter position.
func TestParseTupleType(t *testing.T) {
	parseOK(t, "fn f() -> (int,string) { return f() }\nfn main() -> int { return 0 }\n")
	parseOK(t, "fn f() -> (int,string,) { return f() }\nfn main() -> int { return 0 }\n") // trailing comma
	parseOK(t, "fn f(t: (int,bool)) -> int { return 0 }\nfn main() -> int { return 0 }\n")
	parseOK(t, "fn f() -> (int,(bool,int)) { return f() }\nfn main() -> int { return 0 }\n") // nested
}

// TestParseTupleTypeErrors: () and (T,) as TYPES are parse errors, with
// distinct diagnostics per spec section 4.1: empty -> "empty tuple is not a type";
// single element -> "tuple type requires at least two element types". AC 23: each
// error carries a source position (non-zero *parser.Error.Pos.Line).
func TestParseTupleTypeErrors(t *testing.T) {
	// Empty () -- assert wording AND that the error is positioned.
	got := parseErr(t, "fn f() -> () { return f() }\nfn main() -> int { return 0 }\n")
	if !strings.Contains(got.Error(), "empty tuple is not a type") {
		t.Errorf("empty () type: error = %q, want substring %q", got, "empty tuple is not a type")
	}
	if pe, ok := got.(*Error); !ok || pe.Pos.Line == 0 {
		t.Errorf("empty () type: expected positioned *parser.Error with non-zero line, got %T %v", got, got)
	}
	// Single-element `(int)` is now parenthesized grouping: it unwraps to `int`,
	// not a 1-tuple and not an error (the `(T)`-grouping proof).
	prog := parseOK(t, "fn f() -> (int) { return f() }\nfn main() -> int { return 0 }\n")
	if rt := prog.Funcs[0].RetType; rt != ast.TypeInt {
		t.Errorf("(int) return type = %q, want int", rt)
	}
	if got := parseErr(t, "fn f() -> (int,) { return f() }\nfn main() -> int { return 0 }\n"); !strings.Contains(got.Error(), "tuple type requires at least two element types") {
		t.Errorf("(int,) type: error = %q, want substring %q", got, "tuple type requires at least two element types")
	}
}

// TestParseTupleLit: comma-separated group becomes a tuple literal.
func TestParseTupleLit(t *testing.T) {
	parseOK(t, wrap("let t: (int,int) = (1, 2)\nreturn 0"))                // basic 2-tuple
	parseOK(t, wrap("let t: (int,int,int) = (1, 2, 3)\nreturn 0"))         // 3-tuple
	parseOK(t, wrap("let t: (int,int) = (1, 2,)\nreturn 0"))               // trailing comma, 2 elems -> OK
	parseOK(t, wrap("let t: ((int,bool),int) = ((1, true), 3)\nreturn 0")) // nested tuple literal
}

// TestParseGroupedExpr: single parens still group (not a tuple).
func TestParseGroupedExpr(t *testing.T) {
	parseOK(t, wrap("let x: int = (1 + 2)\nreturn 0"))
}

// TestParseTupleLitArityErrors: () and one-element (1,) in value position are
// parse errors (n >= 2 enforced in parsePrimary). Assert the exact wording AND,
// for AC 23, that each error is positioned (non-zero *parser.Error.Pos.Line).
func TestParseTupleLitArityErrors(t *testing.T) {
	empty := parseErr(t, wrap("let t: (int,int) = ()\nreturn 0"))
	if !strings.Contains(empty.Error(), "empty tuple is not a value") {
		t.Errorf("(): error = %q, want substring %q", empty, "empty tuple is not a value")
	}
	if pe, ok := empty.(*Error); !ok || pe.Pos.Line == 0 {
		t.Errorf("(): expected positioned *parser.Error with non-zero line, got %T %v", empty, empty)
	}
	one := parseErr(t, wrap("let t: (int,int) = (1,)\nreturn 0"))
	if !strings.Contains(one.Error(), "tuple literal requires at least two elements") {
		t.Errorf("(1,): error = %q, want substring %q", one, "tuple literal requires at least two elements")
	}
	if pe, ok := one.(*Error); !ok || pe.Pos.Line == 0 {
		t.Errorf("(1,): expected positioned *parser.Error with non-zero line, got %T %v", one, one)
	}
}

// TestParseTupleLitShape: the parsed node is an *ast.TupleLit with the right arity.
func TestParseTupleLitShape(t *testing.T) {
	prog := parseOK(t, wrap("let t: (int,int,int) = (1, 2, 3)\nreturn 0"))
	ls, ok := mainBody(t, prog)[0].(*ast.LetStmt)
	if !ok {
		t.Fatalf("expected *ast.LetStmt, got %T", mainBody(t, prog)[0])
	}
	tl, ok := ls.Value.(*ast.TupleLit)
	if !ok {
		t.Fatalf("expected *ast.TupleLit, got %T", ls.Value)
	}
	if len(tl.Elems) != 3 {
		t.Errorf("arity = %d, want 3", len(tl.Elems))
	}
}
