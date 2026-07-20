package parser

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
)

func tupleStmt(t *testing.T, body string) *ast.TupleBindStmt {
	t.Helper()
	prog := parseOK(t, wrap(body))
	b := mainBody(t, prog)
	if len(b) != 1 {
		t.Fatalf("body len = %d, want 1", len(b))
	}
	ts, ok := b[0].(*ast.TupleBindStmt)
	if !ok {
		t.Fatalf("stmt 0 = %T, want *ast.TupleBindStmt", b[0])
	}
	return ts
}

func TestLetTupleTwoBindings(t *testing.T) {
	ts := tupleStmt(t, "let (a: int, b: string) = e")
	if ts.Final {
		t.Errorf("Final = true, want false for let")
	}
	if len(ts.Slots) != 2 {
		t.Fatalf("Slots = %d, want 2", len(ts.Slots))
	}
	s0 := ts.Slots[0]
	if s0.Blank {
		t.Errorf("slot 0 Blank = true, want false")
	}
	if s0.Name != "a" {
		t.Errorf("slot 0 Name = %q, want a", s0.Name)
	}
	if s0.Type != ast.TypeInt {
		t.Errorf("slot 0 Type = %q, want int", s0.Type)
	}
	if s0.Pos.Col == 0 {
		t.Errorf("slot 0 Pos.Col == 0, expected a real position")
	}
	s1 := ts.Slots[1]
	if s1.Name != "b" {
		t.Errorf("slot 1 Name = %q, want b", s1.Name)
	}
	if s1.Type != ast.TypeString {
		t.Errorf("slot 1 Type = %q, want string", s1.Type)
	}
	if s1.Pos.Col == 0 {
		t.Errorf("slot 1 Pos.Col == 0, expected a real position")
	}
	if s1.Pos.Col <= s0.Pos.Col {
		t.Errorf("slot 1 Pos.Col (%d) should be after slot 0 (%d)", s1.Pos.Col, s0.Pos.Col)
	}
	if ts.KwPos.Col == 0 {
		t.Errorf("KwPos.Col == 0, expected a real position")
	}
	if _, ok := ts.Value.(*ast.Ident); !ok {
		t.Errorf("Value = %T, want *ast.Ident", ts.Value)
	}
}

func TestFinalTupleTwoBindings(t *testing.T) {
	ts := tupleStmt(t, "final (a: int, b: string) = e")
	if !ts.Final {
		t.Errorf("Final = false, want true for final")
	}
	if len(ts.Slots) != 2 {
		t.Fatalf("Slots = %d, want 2", len(ts.Slots))
	}
	if ts.Slots[0].Name != "a" || ts.Slots[0].Type != ast.TypeInt {
		t.Errorf("slot 0 = %+v, want a:int", ts.Slots[0])
	}
	if ts.Slots[1].Name != "b" || ts.Slots[1].Type != ast.TypeString {
		t.Errorf("slot 1 = %+v, want b:string", ts.Slots[1])
	}
}

func TestLetTupleBareDiscard(t *testing.T) {
	ts := tupleStmt(t, "let (_, out: string) = e")
	if len(ts.Slots) != 2 {
		t.Fatalf("Slots = %d, want 2", len(ts.Slots))
	}
	s0 := ts.Slots[0]
	if !s0.Blank {
		t.Errorf("slot 0 Blank = false, want true")
	}
	if s0.Type != "" {
		t.Errorf("slot 0 Type = %q, want empty (nil) for bare _", s0.Type)
	}
	if s0.Pos.Col == 0 {
		t.Errorf("slot 0 Pos.Col == 0, expected a real position")
	}
	s1 := ts.Slots[1]
	if s1.Blank {
		t.Errorf("slot 1 Blank = true, want false")
	}
	if s1.Name != "out" || s1.Type != ast.TypeString {
		t.Errorf("slot 1 = %+v, want out:string", s1)
	}
}

func TestLetTupleAnnotatedDiscard(t *testing.T) {
	ts := tupleStmt(t, "let (_: int, out: string) = e")
	if len(ts.Slots) != 2 {
		t.Fatalf("Slots = %d, want 2", len(ts.Slots))
	}
	s0 := ts.Slots[0]
	if !s0.Blank {
		t.Errorf("slot 0 Blank = false, want true")
	}
	if s0.Type != ast.TypeInt {
		t.Errorf("slot 0 Type = %q, want int for annotated discard", s0.Type)
	}
	if ts.Slots[1].Name != "out" || ts.Slots[1].Type != ast.TypeString {
		t.Errorf("slot 1 = %+v, want out:string", ts.Slots[1])
	}
}

func TestLetTupleTrailingComma(t *testing.T) {
	ts := tupleStmt(t, "let (a: int, b: string,) = e")
	if len(ts.Slots) != 2 {
		t.Fatalf("Slots = %d, want 2", len(ts.Slots))
	}
	if ts.Slots[0].Name != "a" || ts.Slots[1].Name != "b" {
		t.Errorf("slots = %q, %q; want a, b", ts.Slots[0].Name, ts.Slots[1].Name)
	}
}

func TestLetTupleSingleSlotIsError(t *testing.T) {
	err := parseErr(t, wrap("let (a: int) = e"))
	errAt(t, err, "test.wisp:")
}

func TestFinalTupleSingleSlotIsError(t *testing.T) {
	err := parseErr(t, wrap("final (a: int) = e"))
	errAt(t, err, "test.wisp:")
}

func TestBareLetStillParsesToLetStmt(t *testing.T) {
	prog := parseOK(t, wrap("let a: int = e"))
	b := mainBody(t, prog)
	if _, ok := b[0].(*ast.LetStmt); !ok {
		t.Fatalf("stmt 0 = %T, want *ast.LetStmt", b[0])
	}
}

func TestBareFinalStillParsesToFinalStmt(t *testing.T) {
	prog := parseOK(t, wrap("final a: int = e"))
	b := mainBody(t, prog)
	if _, ok := b[0].(*ast.FinalStmt); !ok {
		t.Fatalf("stmt 0 = %T, want *ast.FinalStmt", b[0])
	}
}

// const gains NO destructuring: a leading `(` routes through parseConstAnnotated,
// whose expect(Ident) yields an "expected identifier"-class located error.
func TestConstParenIsNotDestructuring(t *testing.T) {
	err := parseErr(t, wrap("const (a: int, b: int) = e"))
	if !strings.Contains(err.Error(), "expected Ident") {
		t.Errorf("const-paren error %q should be an 'expected Ident'-class error (const path unchanged)", err.Error())
	}
}
