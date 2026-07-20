package codegen

import (
	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// reachableFuncs returns the set of functions transitively reachable from the
// root's `fn main` via the resolved call graph: direct/qualified user calls
// (CallInfo.Func) and function references (FuncRef.Mangled, resolved through an
// index over the combined info.Funcs). Only reachable functions are emitted, so
// an unused function -- including every function of an unused import/include --
// contributes nothing to the output (spec acceptance 6). Mangled names are
// modid-qualified, so the index is unambiguous across modules.
func reachableFuncs(info *types.Info) map[*ast.FuncDecl]bool {
	return reachableFrom(info, []*ast.FuncDecl{info.Main}, nil)
}

// reachableFromTests is the test-mode reachability root set (spec R12): a
// `*_test.wisp` has no `fn main`, so reachability is seeded from every test
// body and from the lifecycle hooks (`fn setup`/`fn teardown` if present),
// rather than from main. Functions reachable only from a `test` block are kept;
// everything else is tree-shaken as usual.
func reachableFromTests(info *types.Info, tests []*ast.TestDecl, hooks []*ast.FuncDecl) map[*ast.FuncDecl]bool {
	var testBodies [][]ast.Stmt
	for _, td := range tests {
		testBodies = append(testBodies, td.Body)
	}
	return reachableFrom(info, hooks, testBodies)
}

// reachableFrom computes the reachable function set from a set of seed
// functions plus extra statement blocks (test bodies have no FuncDecl).
func reachableFrom(info *types.Info, seeds []*ast.FuncDecl, extraBodies [][]ast.Stmt) map[*ast.FuncDecl]bool {
	byMangled := make(map[string]*ast.FuncDecl, len(info.Funcs))
	for fn, fi := range info.Funcs {
		byMangled[fi.Mangled] = fn
	}
	reach := map[*ast.FuncDecl]bool{}
	w := &calleeWalker{info: info, byMangled: byMangled}
	queue := append([]*ast.FuncDecl(nil), seeds...)
	// Seed from the extra (test) bodies: walk each and enqueue its callees.
	for _, body := range extraBodies {
		w.out = w.out[:0]
		w.walkBlock(body)
		queue = append(queue, w.out...)
	}
	for len(queue) > 0 {
		fn := queue[0]
		queue = queue[1:]
		if fn == nil || reach[fn] {
			continue
		}
		reach[fn] = true
		w.out = w.out[:0]
		w.walkBlock(fn.Body)
		for _, c := range w.out {
			if c != nil && !reach[c] {
				queue = append(queue, c)
			}
		}
	}
	return reach
}

// calleeWalker collects the functions called or referenced within a body.
type calleeWalker struct {
	info      *types.Info
	byMangled map[string]*ast.FuncDecl
	out       []*ast.FuncDecl
}

func (w *calleeWalker) walkBlock(stmts []ast.Stmt) {
	for _, s := range stmts {
		w.walkStmt(s)
	}
}

func (w *calleeWalker) walkStmt(s ast.Stmt) {
	switch n := s.(type) {
	case *ast.LetStmt:
		w.walkExpr(n.Value)
	case *ast.TupleBindStmt:
		// A function called ONLY from a destructuring RHS must not be pruned as
		// dead; walk the Value for reachability.
		w.walkExpr(n.Value)
	case *ast.AssignStmt:
		w.walkExpr(n.Value)
	case *ast.FieldAssignStmt:
		w.walkExpr(n.Target)
		w.walkExpr(n.Value)
	case *ast.IndexAssignStmt:
		w.walkExpr(n.Target)
		w.walkExpr(n.Index)
		w.walkExpr(n.Value)
	case *ast.ReturnStmt:
		w.walkExpr(n.Value)
	case *ast.ThrowStmt:
		w.walkExpr(n.X)
	case *ast.ExprStmt:
		w.walkExpr(n.X)
	case *ast.IfStmt:
		w.walkExpr(n.Cond)
		w.walkBlock(n.Then)
		for _, ei := range n.ElseIfs {
			w.walkExpr(ei.Cond)
			w.walkBlock(ei.Body)
		}
		w.walkBlock(n.Else)
	case *ast.WhileStmt:
		w.walkExpr(n.Cond)
		w.walkBlock(n.Body)
	case *ast.ForStmt:
		w.walkStmt(n.Init)
		w.walkExpr(n.Cond)
		w.walkStmt(n.Post)
		w.walkBlock(n.Body)
	case *ast.ForInStmt:
		w.walkExpr(n.Coll)
		w.walkBlock(n.Body)
	case *ast.SwitchStmt:
		w.walkExpr(n.Subject)
		for _, cs := range n.Cases {
			for _, v := range cs.Values {
				w.walkExpr(v)
			}
			w.walkBlock(cs.Body)
		}
		w.walkBlock(n.Default)
	case *ast.TryStmt:
		w.walkBlock(n.Body)
		w.walkBlock(n.Catch)
		w.walkBlock(n.Finally)
	case *ast.MatchStmt:
		w.walkExpr(n.Scrutinee)
		for _, arm := range n.Arms {
			w.walkBlock(arm.Body)
		}
	case *ast.FinalStmt:
		// final is a runtime-immutable local; walk the RHS for reachability.
		w.walkExpr(n.Value)
	case *ast.ConstStmt:
		// const has no runtime variable, but the initializer is a const-expr and
		// cannot contain a user function call; walk anyway for completeness.
		w.walkExpr(n.Value)
	}
}

func (w *calleeWalker) walkExpr(e ast.Expr) {
	if e == nil {
		return
	}
	switch n := e.(type) {
	case *ast.Ident:
		if fr, ok := w.info.FuncRefs[n]; ok {
			w.out = append(w.out, w.byMangled[fr.Mangled])
		}
	case *ast.UnaryExpr:
		w.walkExpr(n.X)
	case *ast.BinaryExpr:
		w.walkExpr(n.L)
		w.walkExpr(n.R)
	case *ast.CallExpr:
		if ci, ok := w.info.Calls[n]; ok && ci.Kind == types.CallUser {
			w.out = append(w.out, ci.Func)
		}
		// The callee expression and arguments may contain further references.
		w.walkExpr(n.Callee)
		for _, a := range n.Args {
			w.walkExpr(a)
		}
	case *ast.StructLit:
		for _, f := range n.Fields {
			w.walkExpr(f.Value)
		}
	case *ast.ArrayLit:
		for _, el := range n.Elems {
			w.walkExpr(el)
		}
	case *ast.TupleLit:
		for _, el := range n.Elems {
			w.walkExpr(el)
		}
	case *ast.DictLit:
		for _, en := range n.Entries {
			w.walkExpr(en.Key)
			w.walkExpr(en.Value)
		}
	case *ast.FieldAccess:
		if fr, ok := w.info.MemberFuncRefs[n]; ok {
			w.out = append(w.out, w.byMangled[fr.Mangled])
		}
		w.walkExpr(n.X)
	case *ast.IndexExpr:
		w.walkExpr(n.X)
		w.walkExpr(n.Index)
	case *ast.StringLit:
		for _, p := range n.Parts {
			if !p.IsText() {
				w.walkExpr(p.Expr)
			}
		}
	}
}
