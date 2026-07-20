package types

// Task-4: const declaration semantics + navigation record.
//
// Tests cover:
//   - const _ : int = 5 creates no binding (no Var, no scope entry)
//   - local const used after declaration resolves correctly
//   - unused local const does NOT produce an unused-variable warning
//   - const x: int = "hello" (type mismatch) is a compile error
//   - reassigning a const name produces "cannot assign to constant"
//   - redeclaring/shadowing a const name is rejected
//   - a const reference appears in info.Uses after checking
//   - top-level const used inside a function body resolves to a const-flagged Var
//   - ConstVars maps the ConstStmt to its Var
//   - TopConstVars maps the ConstDecl to its Var

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/parser"
)

// parseConstSrc parses src and runs the checker, returning Info.
// It fails the test if parsing fails.
func parseConstSrc(t *testing.T, src string) *Info {
	t.Helper()
	prog, err := parser.Parse(src, "test.wisp")
	if err != nil {
		t.Fatalf("parse error: %v\nsrc:\n%s", err, src)
	}
	return Check(prog)
}

// TestConstStmt_BlankDiscard verifies that `const _: int = 5` folds without
// creating a binding (no Var in ConstVars, no scope entry).
func TestConstStmt_BlankDiscard(t *testing.T) {
	src := wrapMain(`const _: int = 5`)
	info := expectOK(t, src)
	if len(info.ConstVars) != 0 {
		t.Fatalf("expected empty ConstVars for blank const, got %d entries", len(info.ConstVars))
	}
}

// TestConstStmt_UsedAfterDeclaration verifies that a local const declared inside
// a function body is visible to expressions that follow it in the same scope.
func TestConstStmt_UsedAfterDeclaration(t *testing.T) {
	src := wrapMain(`const X: int = 42
let y: int = X`)
	expectOK(t, src)
}

// TestConstStmt_UnusedNoWarning verifies that an unused local const does NOT
// trigger the unused-variable warning (consts are exempt from rule 6/10).
func TestConstStmt_UnusedNoWarning(t *testing.T) {
	src := wrapMain(`const X: int = 99`)
	info := expectOK(t, src)
	for _, w := range info.Warnings {
		if strings.Contains(w.Msg, "unused") && strings.Contains(w.Msg, "X") {
			t.Fatalf("unexpected unused warning for const X: %s", w.Msg)
		}
	}
}

// TestConstStmt_TypeMismatch verifies that a const whose initializer type does
// not match the annotation is a compile error.
func TestConstStmt_TypeMismatch(t *testing.T) {
	src := wrapMain(`const X: int = "hello"`)
	expectErr(t, src, "type mismatch")
}

// TestConstStmt_ReassignRejected verifies that assigning to a const-bound name
// is a compile error containing "cannot assign to constant".
func TestConstStmt_ReassignRejected(t *testing.T) {
	src := wrapMain(`const X: int = 10
X = 20`)
	expectErr(t, src, "cannot assign to constant")
}

// TestConstStmt_RedeclareRejected verifies that redeclaring a const name in
// the same or an enclosing scope is rejected (same rule as let no-shadow).
func TestConstStmt_RedeclareRejected(t *testing.T) {
	src := wrapMain(`const X: int = 1
const X: int = 2`)
	info := parseConstSrc(t, src)
	if len(info.Errors) == 0 {
		t.Fatal("expected error for redeclaring const X, got none")
	}
	found := false
	for _, d := range info.Errors {
		if strings.Contains(d.Msg, "X") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected an error mentioning X, got: %s", diagList(info.Errors))
	}
}

// TestConstStmt_ShadowTopConstRejected verifies that a function-local const
// cannot shadow a same-named top-level const (strict no-shadowing).
func TestConstStmt_ShadowTopConstRejected(t *testing.T) {
	src := "const X: int = 1\n" +
		"fn main() -> int {\n" +
		"const X: int = 2\n" +
		"return 0\n" +
		"}"
	expectErr(t, src, "no shadowing")
}

// TestConstStmt_ShadowLetRejected verifies that a const cannot shadow an
// existing let binding (no-shadowing rule).
func TestConstStmt_ShadowLetRejected(t *testing.T) {
	src := wrapMain(`let x: int = 5
const x: int = 10`)
	info := parseConstSrc(t, src)
	if len(info.Errors) == 0 {
		t.Fatal("expected error for const shadowing let x, got none")
	}
}

// TestConstStmt_UsesPopulated verifies that when a const name is referenced in
// a value expression, info.Uses maps the *ast.Ident to a const-flagged Var.
func TestConstStmt_UsesPopulated(t *testing.T) {
	src := wrapMain(`const LIMIT: int = 100
let n: int = LIMIT`)
	prog, err := parser.Parse(src, "test.wisp")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	info := Check(prog)
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %s", diagList(info.Errors))
	}
	// Find the Ident node for LIMIT in the let initializer.
	fn := prog.Funcs[0]
	// Second statement is the let; its Value is an *ast.Ident.
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
		t.Fatal("info.Uses does not contain the LIMIT ident")
	}
	if !v.IsConst {
		t.Fatalf("expected Uses[LIMIT].IsConst == true, got false; Var = %+v", v)
	}
}

// TestConstStmt_ConstVarsRecord verifies that info.ConstVars maps the
// *ast.ConstStmt to its resolved Var, and the Var has IsConst == true.
func TestConstStmt_ConstVarsRecord(t *testing.T) {
	src := wrapMain(`const LIMIT: int = 100`)
	prog, err := parser.Parse(src, "test.wisp")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	info := Check(prog)
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %s", diagList(info.Errors))
	}
	fn := prog.Funcs[0]
	stmt, ok := fn.Body[0].(*ast.ConstStmt)
	if !ok {
		t.Fatalf("expected first stmt to be *ast.ConstStmt, got %T", fn.Body[0])
	}
	v, found := info.ConstVars[stmt]
	if !found {
		t.Fatal("info.ConstVars does not contain the ConstStmt")
	}
	if !v.IsConst {
		t.Fatalf("expected ConstVars entry IsConst == true, got false")
	}
	if v.Name != "LIMIT" {
		t.Fatalf("expected Var.Name = LIMIT, got %q", v.Name)
	}
	if v.Type != Int {
		t.Fatalf("expected Var.Type = int, got %s", v.Type)
	}
}

// TestConstStmt_TopConstVarsRecord verifies that info.TopConstVars maps each
// *ast.ConstDecl (top-level) to a const-flagged Var after the pass runs.
func TestConstStmt_TopConstVarsRecord(t *testing.T) {
	src := "const TIMEOUT: int = 30\nfn main() -> int { return 0 }"
	prog, err := parser.Parse(src, "test.wisp")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	info := Check(prog)
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %s", diagList(info.Errors))
	}
	if len(prog.Consts) == 0 {
		t.Fatal("expected prog.Consts to have at least one ConstDecl")
	}
	decl := prog.Consts[0]
	v, found := info.TopConstVars[decl]
	if !found {
		t.Fatal("info.TopConstVars does not contain the ConstDecl for TIMEOUT")
	}
	if !v.IsConst {
		t.Fatalf("expected TopConstVars entry IsConst == true, got false")
	}
	if v.Name != "TIMEOUT" {
		t.Fatalf("expected Var.Name = TIMEOUT, got %q", v.Name)
	}
}

// TestConstStmt_TopLevelUsesPopulated verifies that when a top-level const name
// is referenced in a function body, info.Uses maps the Ident to a const-flagged Var.
func TestConstStmt_TopLevelUsesPopulated(t *testing.T) {
	src := "const MAX: int = 256\nfn main() -> int {\nlet n: int = MAX\nreturn n\n}"
	prog, err := parser.Parse(src, "test.wisp")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	info := Check(prog)
	if len(info.Errors) != 0 {
		t.Fatalf("unexpected errors: %s", diagList(info.Errors))
	}
	fn := prog.Funcs[0]
	letStmt, ok := fn.Body[0].(*ast.LetStmt)
	if !ok {
		t.Fatalf("expected first stmt to be *ast.LetStmt, got %T", fn.Body[0])
	}
	id, ok := letStmt.Value.(*ast.Ident)
	if !ok {
		t.Fatalf("expected let value to be *ast.Ident, got %T", letStmt.Value)
	}
	v, found := info.Uses[id]
	if !found {
		t.Fatal("info.Uses does not contain the MAX ident for top-level const reference")
	}
	if !v.IsConst {
		t.Fatalf("expected Uses[MAX].IsConst == true, got false; Var = %+v", v)
	}
}

// TestConstStmt_LocalConstReferenceInExpr verifies that a local const used in
// a binary expression inside the same body resolves and folds correctly.
func TestConstStmt_LocalConstReferenceInExpr(t *testing.T) {
	src := wrapMain(`const BASE: int = 10
const DOUBLE: int = BASE * 2
let n: int = DOUBLE`)
	expectOK(t, src)
}

// TestConstStmt_TopLevelConstInLocalConst verifies that a top-level const can
// be referenced inside a function-local const initializer and folds correctly.
func TestConstStmt_TopLevelConstInLocalConst(t *testing.T) {
	src := "const BASE: int = 10\nfn main() -> int {\nconst X: int = BASE * 2\nreturn X\n}"
	expectOK(t, src)
}

// TestConstStmt_TopLevelConstReassignRejected verifies that assigning to a
// top-level const name inside a function body produces "cannot assign to constant".
func TestConstStmt_TopLevelConstReassignRejected(t *testing.T) {
	src := "const LIMIT: int = 5\nfn main() -> int {\nLIMIT = 99\nreturn 0\n}"
	expectErr(t, src, "cannot assign to constant")
}

// TestConstStmt_FailedFoldNoNilPanic verifies that when a const initializer
// fails to fold (e.g. references a non-const local), a subsequent const that
// references the failed const produces a normal error and DOES NOT panic.
// Before the fix, the failed const was still entered into localConsts with a
// nil value, and a later type-assertion on that nil (lv.(int64)) caused a
// runtime panic in foldBinary.
func TestConstStmt_FailedFoldNoNilPanic(t *testing.T) {
	// let x is not a const, so const A's initializer fails to fold.
	// const B references A; before the fix this panics in foldBinary.
	src := wrapMain(`let x: int = 5
const A: int = x
const B: int = A + 1`)
	info := check(t, src)
	if len(info.Errors) == 0 {
		t.Fatal("expected at least one error for non-const initializer, got none")
	}
}
