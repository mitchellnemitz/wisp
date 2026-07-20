package codegen

import (
	"strings"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// monoInst is one monomorphized instantiation of a numeric-bounded function.
type monoInst struct {
	name      string
	typeSubst map[types.Type]types.Type // "$T" -> types.Int or types.Float
}

// collectMonoInstances finds all numeric-bounded generic functions that are
// instantiated with concrete types and returns the required instances per
// FuncDecl. It performs transitive closure so that a numeric generic calling
// another numeric generic propagates concrete bindings transitively.
func collectMonoInstances(info *types.Info) map[*ast.FuncDecl][]monoInst {
	result := map[*ast.FuncDecl][]monoInst{}
	seen := map[*ast.FuncDecl]map[string]bool{}

	type workItem struct {
		fn     *ast.FuncDecl
		tsubst map[types.Type]types.Type
	}
	var queue []workItem

	isTVar := func(t types.Type) bool { return len(t) > 0 && t[0] == '$' }

	instKeyFor := func(fn *ast.FuncDecl, tsubst map[types.Type]types.Type) string {
		var parts []string
		for _, tp := range fn.TypeParams {
			if fn.TypeParamBounds[tp] != "numeric" {
				continue
			}
			v := tsubst[types.Type("$"+tp)]
			parts = append(parts, tp+"="+string(v))
		}
		return strings.Join(parts, ",")
	}

	nameFor := func(fn *ast.FuncDecl, tsubst map[types.Type]types.Type) string {
		fi := info.Funcs[fn]
		var sb strings.Builder
		sb.WriteString(fi.Mangled)
		for _, tp := range fn.TypeParams {
			if fn.TypeParamBounds[tp] != "numeric" {
				continue
			}
			v := tsubst[types.Type("$"+tp)]
			sb.WriteString("__")
			sb.WriteString(string(v))
		}
		return sb.String()
	}

	addInst := func(fn *ast.FuncDecl, tsubst map[types.Type]types.Type) {
		key := instKeyFor(fn, tsubst)
		if seen[fn] == nil {
			seen[fn] = map[string]bool{}
		}
		if seen[fn][key] {
			return
		}
		seen[fn][key] = true
		name := nameFor(fn, tsubst)
		result[fn] = append(result[fn], monoInst{name: name, typeSubst: tsubst})
		queue = append(queue, workItem{fn: fn, tsubst: tsubst})
	}

	// Seed: all direct calls with all-concrete TypeSubst.
	for _, ci := range info.Calls {
		if ci.Kind != types.CallUser || len(ci.TypeSubst) == 0 {
			continue
		}
		allConcrete := true
		for _, v := range ci.TypeSubst {
			if isTVar(v) {
				allConcrete = false
				break
			}
		}
		if !allConcrete {
			continue
		}
		tsubst := make(map[types.Type]types.Type, len(ci.TypeSubst))
		for name, t := range ci.TypeSubst {
			tsubst[types.Type("$"+name)] = t
		}
		addInst(ci.Func, tsubst)
	}

	// BFS: for each instantiation, walk the function body to find calls to other
	// numeric-bounded functions with type-variable TypeSubst, resolve those vars
	// through the current substitution, and collect derived instantiations.
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		w := &ciWalker{info: info}
		w.walkBlock(item.fn.Body)
		for _, ci := range w.out {
			resolved := make(map[types.Type]types.Type, len(ci.TypeSubst))
			allConcrete := true
			for name, v := range ci.TypeSubst {
				concrete := v
				if isTVar(v) {
					if c, ok := item.tsubst[v]; ok {
						concrete = c
					} else {
						allConcrete = false
						break
					}
				}
				resolved[types.Type("$"+name)] = concrete
			}
			if !allConcrete {
				continue
			}
			addInst(ci.Func, resolved)
		}
	}

	return result
}

// ciWalker collects CallInfo entries for user calls with non-nil TypeSubst
// (numeric-bounded calls) within a function body.
type ciWalker struct {
	info *types.Info
	out  []*types.CallInfo
}

func (w *ciWalker) walkBlock(stmts []ast.Stmt) {
	for _, s := range stmts {
		w.walkStmt(s)
	}
}

func (w *ciWalker) walkStmt(s ast.Stmt) {
	switch n := s.(type) {
	case *ast.LetStmt:
		w.walkExpr(n.Value)
	case *ast.TupleBindStmt:
		// A generic call appearing ONLY in a destructuring RHS must still be
		// discovered for monomorphization; walk the Value.
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
		// A generic call appearing ONLY in a final initializer must still be
		// discovered for monomorphization; walk the Value. (Mirrors LetStmt.)
		w.walkExpr(n.Value)
	}
}

func (w *ciWalker) walkExpr(e ast.Expr) {
	if e == nil {
		return
	}
	switch n := e.(type) {
	case *ast.CallExpr:
		if ci, ok := w.info.Calls[n]; ok && ci.Kind == types.CallUser && len(ci.TypeSubst) > 0 {
			w.out = append(w.out, ci)
		}
		w.walkExpr(n.Callee)
		for _, a := range n.Args {
			w.walkExpr(a)
		}
	case *ast.UnaryExpr:
		w.walkExpr(n.X)
	case *ast.BinaryExpr:
		w.walkExpr(n.L)
		w.walkExpr(n.R)
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
