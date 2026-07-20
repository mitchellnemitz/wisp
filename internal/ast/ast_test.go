package ast

import (
	"testing"

	"github.com/mitchellnemitz/wisp/internal/token"
)

func TestNodesImplementInterfaces(t *testing.T) {
	var _ Node = (*Program)(nil)
	var _ Node = (*FuncDecl)(nil)
	var _ Node = (*Param)(nil)

	stmts := []Stmt{
		(*LetStmt)(nil), (*AssignStmt)(nil), (*ReturnStmt)(nil), (*IfStmt)(nil),
		(*WhileStmt)(nil), (*ForStmt)(nil), (*SwitchStmt)(nil), (*BreakStmt)(nil),
		(*ContinueStmt)(nil), (*ExprStmt)(nil),
	}
	for _, s := range stmts {
		var _ Stmt = s
	}

	exprs := []Expr{
		(*IntLit)(nil), (*BoolLit)(nil), (*StringLit)(nil), (*Ident)(nil),
		(*UnaryExpr)(nil), (*BinaryExpr)(nil), (*CallExpr)(nil),
	}
	for _, e := range exprs {
		var _ Expr = e
	}
}

func TestPos(t *testing.T) {
	p := token.Position{File: "f.wisp", Line: 4, Col: 2}
	id := &Ident{NamePos: p, Name: "x"}
	if id.Pos() != p {
		t.Fatalf("Ident.Pos() = %v, want %v", id.Pos(), p)
	}
	// BinaryExpr reports its left operand's position
	bin := &BinaryExpr{OpPos: token.Position{Line: 9}, Op: token.Plus, L: id, R: id}
	if bin.Pos() != p {
		t.Fatalf("BinaryExpr.Pos() = %v, want left operand %v", bin.Pos(), p)
	}
	// TypeAliasDecl reports its `type` keyword position.
	ta := &TypeAliasDecl{KwPos: p, NamePos: token.Position{Line: 4, Col: 7}, Name: "Miles", Type: TypeInt}
	if ta.Pos() != p {
		t.Fatalf("TypeAliasDecl.Pos() = %v, want KwPos %v", ta.Pos(), p)
	}
}

func TestStringPartIsText(t *testing.T) {
	text := StringPart{Text: "hi"}
	if !text.IsText() {
		t.Errorf("text part: IsText() = false, want true")
	}
	interp := StringPart{Expr: &Ident{Name: "x"}}
	if interp.IsText() {
		t.Errorf("interp part: IsText() = true, want false")
	}
}
