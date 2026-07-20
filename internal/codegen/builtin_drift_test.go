package codegen

import (
	"strings"
	"testing"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/token"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// TestGenBuiltinCall_UnhandledBuiltinPanics: genBuiltinCall's default arm guards
// against checker/codegen drift -- a builtin the checker accepts but codegen has
// no case for. It must panic loudly (naming the builtin), matching the invariant
// panics elsewhere in this file, rather than silently emitting an empty string.
func TestGenBuiltinCall_UnhandledBuiltinPanics(t *testing.T) {
	const drift = "definitely_not_a_real_builtin"
	g := &gen{}
	ci := &types.CallInfo{Kind: types.CallBuiltin, Builtin: drift}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("genBuiltinCall(%q) did not panic; the drift guard is missing", drift)
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is %T, want string: %v", r, r)
		}
		if !strings.Contains(msg, drift) {
			t.Fatalf("panic message %q does not name the unhandled builtin %q", msg, drift)
		}
	}()

	g.genBuiltinCall(&ast.CallExpr{}, ci)
}

// TestGenUnary_UnhandledOpPanics: genUnary's default arm guards against
// parser/checker drift -- an operator token the checker accepts on a
// UnaryExpr but codegen has no case for.
func TestGenUnary_UnhandledOpPanics(t *testing.T) {
	g := &gen{}
	n := &ast.UnaryExpr{Op: token.Kind(9999), X: &ast.IntLit{Raw: "1"}}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("genUnary with unhandled op did not panic; the drift guard is missing")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is %T, want string: %v", r, r)
		}
		if !strings.Contains(msg, "genUnary") {
			t.Fatalf("panic message %q does not name genUnary", msg)
		}
	}()

	g.genUnary(n)
}

// TestGenBinary_UnhandledOpPanics: genBinary's default arm guards against an
// operator token the checker accepts on a non-float BinaryExpr but codegen
// has no case for.
func TestGenBinary_UnhandledOpPanics(t *testing.T) {
	g := &gen{info: &types.Info{}}
	n := &ast.BinaryExpr{
		Op: token.Kind(9999),
		L:  &ast.IntLit{Raw: "1"},
		R:  &ast.IntLit{Raw: "2"},
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("genBinary with unhandled op did not panic; the drift guard is missing")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is %T, want string: %v", r, r)
		}
		if !strings.Contains(msg, "genBinary") {
			t.Fatalf("panic message %q does not name genBinary", msg)
		}
	}()

	g.genBinary(n)
}

// TestGenFloatBinary_UnhandledOpPanics: genFloatBinary's default arm guards
// against an operator token reaching float-typed binary codegen without an
// explicit case.
func TestGenFloatBinary_UnhandledOpPanics(t *testing.T) {
	g := &gen{}
	n := &ast.BinaryExpr{Op: token.Kind(9999)}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("genFloatBinary with unhandled op did not panic; the drift guard is missing")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is %T, want string: %v", r, r)
		}
		if !strings.Contains(msg, "genFloatBinary") {
			t.Fatalf("panic message %q does not name genFloatBinary", msg)
		}
	}()

	g.genFloatBinary(n, litAtom("1"), litAtom("2"))
}

// TestGenCall_UnhandledKindPanics: genCall's default arm guards against a
// hypothetical 4th types.CallKind value silently aliasing to CallUser's
// codegen path.
func TestGenCall_UnhandledKindPanics(t *testing.T) {
	// out must be a real strings.Builder, not nil: before this fix, the
	// default arm falls through to genUserCall, which calls g.line and would
	// nil-dereference g.out before the test's recover() sees anything -- that
	// would make the pre-fix "did not panic" failure assertion fire for the
	// wrong reason (a runtime nil-pointer panic, not a clean non-panic return).
	g := &gen{info: &types.Info{Calls: map[*ast.CallExpr]*types.CallInfo{}}, out: &strings.Builder{}}
	n := &ast.CallExpr{}
	g.info.Calls[n] = &types.CallInfo{Kind: types.CallKind(99)}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("genCall with unhandled CallKind did not panic; the drift guard is missing")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is %T, want string: %v", r, r)
		}
		if !strings.Contains(msg, "genCall") {
			t.Fatalf("panic message %q does not name genCall", msg)
		}
	}()

	g.genCall(n)
}

// TestMatchTagField_UnhandledVariantPanics: matchTagField's default arm
// guards against a variant string reaching codegen that the checker's
// ConstructorPat.Variant validation should have already rejected.
func TestMatchTagField_UnhandledVariantPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("matchTagField with unhandled variant did not panic; the drift guard is missing")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is %T, want string: %v", r, r)
		}
		if !strings.Contains(msg, "matchTagField") {
			t.Fatalf("panic message %q does not name matchTagField", msg)
		}
	}()

	matchTagField("x", "NotAVariant")
}

// TestEmitSortLess_UnhandledElemTypePanics: emitSortLess's default arm
// guards against an element type reaching sort codegen that the checker's
// isOrderedElem gate should have already rejected.
func TestEmitSortLess_UnhandledElemTypePanics(t *testing.T) {
	// out must be a real strings.Builder, not nil: before this fix, the
	// default arm (current "Int" behavior) calls g.line, which would
	// nil-dereference g.out before the test's recover() sees anything -- see
	// the identical note on TestGenCall_UnhandledKindPanics above.
	g := &gen{out: &strings.Builder{}}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("emitSortLess with unhandled element type did not panic; the drift guard is missing")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is %T, want string: %v", r, r)
		}
		if !strings.Contains(msg, "emitSortLess") {
			t.Fatalf("panic message %q does not name emitSortLess", msg)
		}
	}()

	g.emitSortLess(types.Type("NotAnOrderedType"), "1:1", "a", "b")
}

// TestCasePattern_DriftPanics: casePattern returns the folded value for every
// accepted case value (the checker records a non-nil FoldedValues entry for each
// type-valid constant case value, so the leading guard always fires). Any node
// reaching the post-guard path is checker/codegen drift and must panic loud. An
// empty types.Info{} defeats the guard, so each out-of-domain node below reaches
// the drift sentinel: *ast.CallExpr (never a legal case value), a non-const
// *ast.Ident (rejected by the checker before codegen), and an *ast.BinaryExpr
// with no folded value (the checker guarantees one for every operator case value).
func TestCasePattern_DriftPanics(t *testing.T) {
	cases := []struct {
		name string
		node ast.Expr
	}{
		{"call_expr", &ast.CallExpr{}},
		{"non_const_ident", &ast.Ident{Name: "not_a_const"}},
		{"unfolded_binary", &ast.BinaryExpr{Op: token.Plus, L: &ast.IntLit{Raw: "1"}, R: &ast.IntLit{Raw: "2"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := &gen{info: &types.Info{}}
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("casePattern(%s) did not panic; the drift guard is missing", tc.name)
				}
				msg, ok := r.(string)
				if !ok {
					t.Fatalf("panic value is %T, want string: %v", r, r)
				}
				if !strings.Contains(msg, "casePattern") {
					t.Fatalf("panic message %q does not name casePattern", msg)
				}
			}()
			g.casePattern(tc.node)
		})
	}
}
