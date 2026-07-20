package types

// Task-5: final declaration semantics.
//
// Tests cover:
//   - final x: int = expr is legal and the binding is usable afterward
//   - reassigning a final name produces "cannot assign to final"
//   - unused final warns (like let)
//   - redeclaring/shadowing a final name is rejected
//   - final _: int = expr discards (no binding, no Var in FinalVars)
//   - final value is usable in later expressions
//   - info.FinalVars maps *ast.FinalStmt to its Var with Immutable == true
//   - Var.Mangled is non-empty (final goes in curFunc.Decls like let)
//   - info.Uses maps an Ident referencing a final to its Var

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/parser"
)

// TestFinalStmt_Legal verifies that `final x: int = 42` type-checks without
// errors and the binding is visible after the declaration.
func TestFinalStmt_Legal(t *testing.T) {
	src := wrapMain(`final x: int = 42
let y: int = x`)
	expectOK(t, src)
}

// TestFinalStmt_ReassignRejected verifies that assigning to a final-bound name
// produces an error containing "cannot assign to final".
func TestFinalStmt_ReassignRejected(t *testing.T) {
	src := wrapMain(`final x: int = 42
x = 99`)
	expectErr(t, src, "cannot assign to final")
}

// TestFinalStmt_UnusedWarns verifies that an unused final binding triggers the
// unused-variable warning, exactly like an unused let.
func TestFinalStmt_UnusedWarns(t *testing.T) {
	src := wrapMain(`final x: int = 42`)
	info := check(t, src)
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %s", diagList(info.Errors))
	}
	found := false
	for _, w := range info.Warnings {
		if strings.Contains(w.Msg, "unused") && strings.Contains(w.Msg, "x") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected an unused warning for final x, got warnings: %s", diagList(info.Warnings))
	}
}

// TestFinalStmt_RedeclareRejected verifies that redeclaring a final name in
// the same or an enclosing scope is rejected.
func TestFinalStmt_RedeclareRejected(t *testing.T) {
	src := wrapMain(`final x: int = 1
final x: int = 2`)
	info := check(t, src)
	if len(info.Errors) == 0 {
		t.Fatal("expected error for redeclaring final x, got none")
	}
	found := false
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "x") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected an error mentioning x, got: %s", diagList(info.Errors))
	}
}

// TestFinalStmt_ShadowTopConstRejected verifies that a function-local final
// cannot reuse a same-named top-level const (strict no-shadowing).
func TestFinalStmt_ShadowTopConstRejected(t *testing.T) {
	src := "const X: int = 1\n" +
		"fn main() -> int {\n" +
		"final X: int = 2\n" +
		"return 0\n" +
		"}"
	expectErr(t, src, "no shadowing")
}

// TestFinalStmt_ShadowLetRejected verifies that a final cannot shadow an
// existing let binding (no-shadowing rule).
func TestFinalStmt_ShadowLetRejected(t *testing.T) {
	src := wrapMain(`let x: int = 5
final x: int = 10`)
	info := check(t, src)
	if len(info.Errors) == 0 {
		t.Fatal("expected error for final shadowing let x, got none")
	}
}

// TestFinalStmt_BlankDiscard verifies that `final _: int = expr` type-checks
// the RHS but creates no binding (FinalVars is empty).
func TestFinalStmt_BlankDiscard(t *testing.T) {
	src := wrapMain(`final _: int = 42`)
	info := expectOK(t, src)
	if len(info.FinalVars) != 0 {
		t.Fatalf("expected empty FinalVars for blank final, got %d entries", len(info.FinalVars))
	}
}

// TestFinalStmt_ValueUsableInExpression verifies that a final binding's value
// is usable in a subsequent binary expression and in a return statement.
func TestFinalStmt_ValueUsableInExpression(t *testing.T) {
	src := "fn main() -> int {\nfinal base: int = 10\nreturn base + 1\n}"
	expectOK(t, src)
}

// TestFinalStmt_FinalVarsRecord verifies that info.FinalVars maps the
// *ast.FinalStmt to its resolved Var, that the Var has Immutable == true,
// and that Mangled is non-empty (final is a runtime local, not inlined).
func TestFinalStmt_FinalVarsRecord(t *testing.T) {
	src := wrapMain(`final limit: int = 99`)
	prog, err := parser.Parse(src, "test.wisp")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	info := Check(prog)
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %s", diagList(info.Errors))
	}
	fn := prog.Funcs[0]
	stmt, ok := fn.Body[0].(*ast.FinalStmt)
	if !ok {
		t.Fatalf("expected first stmt to be *ast.FinalStmt, got %T", fn.Body[0])
	}
	v, found := info.FinalVars[stmt]
	if !found {
		t.Fatal("info.FinalVars does not contain the FinalStmt")
	}
	if !v.Immutable {
		t.Fatalf("expected FinalVars entry Immutable == true, got false")
	}
	if v.IsConst {
		t.Fatalf("expected FinalVars entry IsConst == false for final (not a const), got true")
	}
	if v.Mangled == "" {
		t.Fatalf("expected non-empty Mangled for final (runtime local), got empty")
	}
	if v.Name != "limit" {
		t.Fatalf("expected Var.Name = limit, got %q", v.Name)
	}
	if v.Type != Int {
		t.Fatalf("expected Var.Type = int, got %s", v.Type)
	}
}

// TestFinalStmt_InDecls verifies that a final binding appears in curFunc.Decls
// (codegen needs it to emit `local`), unlike const which is absent from Decls.
func TestFinalStmt_InDecls(t *testing.T) {
	src := wrapMain(`final n: int = 7`)
	prog, err := parser.Parse(src, "test.wisp")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	info := Check(prog)
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %s", diagList(info.Errors))
	}
	fn := prog.Funcs[0]
	fi := info.Funcs[fn]
	if fi == nil {
		t.Fatal("no FuncInfo for main")
	}
	stmt, ok := fn.Body[0].(*ast.FinalStmt)
	if !ok {
		t.Fatalf("expected first stmt to be *ast.FinalStmt, got %T", fn.Body[0])
	}
	v := info.FinalVars[stmt]
	if v == nil {
		t.Fatal("FinalVars is empty")
	}
	for _, dv := range fi.Decls {
		if dv == v {
			return // found
		}
	}
	t.Fatalf("final Var not found in FuncInfo.Decls; Decls = %+v", fi.Decls)
}

// TestFinalStmt_UsesPopulated verifies that an Ident referencing a final
// binding is present in info.Uses and points to the Immutable Var.
func TestFinalStmt_UsesPopulated(t *testing.T) {
	src := wrapMain(`final cap: int = 50
let n: int = cap`)
	prog, err := parser.Parse(src, "test.wisp")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	info := Check(prog)
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %s", diagList(info.Errors))
	}
	fn := prog.Funcs[0]
	letStmt, ok := fn.Body[1].(*ast.LetStmt)
	if !ok {
		t.Fatalf("expected second stmt to be *ast.LetStmt, got %T", fn.Body[1])
	}
	id, ok := letStmt.Value.(*ast.Ident)
	if !ok {
		t.Fatalf("expected let value to be *ast.Ident, got %T", letStmt.Value)
	}
	v, found := info.Uses[id]
	if !found {
		t.Fatal("info.Uses does not contain the cap ident")
	}
	if !v.Immutable {
		t.Fatalf("expected Uses[cap].Immutable == true, got false; Var = %+v", v)
	}
}

// TestFinalStmt_TypeMismatch verifies that a final whose initializer type does
// not match the annotation is a compile error.
func TestFinalStmt_TypeMismatch(t *testing.T) {
	src := wrapMain(`final x: int = "hello"`)
	expectErr(t, src, "")
}
