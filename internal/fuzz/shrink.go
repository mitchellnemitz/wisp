// internal/fuzz/shrink.go
package fuzz

type DivergePredicate func(p *Program) bool

// Shrink reduces p to 1-minimal under reductions(): repeatedly apply the first
// candidate that stays type-correct and divergent, until none qualifies (FR-011).
func Shrink(p *Program, stillTypeChecks func(*Program) bool, stillDiverges DivergePredicate) *Program {
	cur := p
	for {
		progressed := false
		for _, cand := range reductions(cur) {
			if stillTypeChecks(cand) && stillDiverges(cand) {
				cur = cand
				progressed = true
				break
			}
		}
		if !progressed {
			return cur
		}
	}
}

// reductions returns all single-step reductions. Each shares unmodified nodes with
// p but has its own Body slice; any replaced node is fresh -- reductions never
// mutates a node in place, so cloneProgram's shallow Body copy is safe.
func reductions(p *Program) []*Program {
	var out []*Program
	for i := range p.Body {
		cp := cloneProgram(p)
		cp.Body = append(cp.Body[:i:i], cp.Body[i+1:]...)
		out = append(out, cp)
	}
	for i, s := range p.Body {
		switch n := s.(type) {
		case *LetStmt:
			for _, alt := range simplifyExpr(n.Init) {
				cp := cloneProgram(p)
				cp.Body[i] = &LetStmt{Name: n.Name, T: n.T, Init: alt}
				out = append(out, cp)
			}
		case *PrintStmt:
			for _, alt := range simplifyExpr(n.Arg) {
				cp := cloneProgram(p)
				cp.Body[i] = &PrintStmt{Arg: alt}
				out = append(out, cp)
			}
		case *IfStmt:
			if len(n.Else) > 0 {
				cp := cloneProgram(p)
				cp.Body[i] = &IfStmt{Cond: n.Cond, Then: n.Then, Else: nil}
				out = append(out, cp)
			}
		}
	}
	return out
}

func simplifyExpr(e Expr) []Expr {
	var out []Expr
	switch n := e.(type) {
	case *Binary:
		if n.T.Kind == KInt || n.T.Kind == KFloat {
			out = append(out, n.L)
		}
	case *Call:
		out = append(out, leafOfType(n.T))
	case *ArrayLit:
		if len(n.Elems) > 1 {
			for i := range n.Elems {
				out = append(out, &ArrayLit{Elem: n.Elem, Elems: append(append([]Expr{}, n.Elems[:i]...), n.Elems[i+1:]...)})
			}
		}
	case *DictLit:
		if len(n.Keys) > 1 {
			for i := range n.Keys {
				out = append(out, &DictLit{
					Val:  n.Val,
					Keys: append(append([]string{}, n.Keys[:i]...), n.Keys[i+1:]...),
					Vals: append(append([]Expr{}, n.Vals[:i]...), n.Vals[i+1:]...),
				})
			}
		}
	}
	return out
}

func cloneProgram(p *Program) *Program {
	body := make([]Stmt, len(p.Body))
	copy(body, p.Body)
	return &Program{Imports: append([]string(nil), p.Imports...), Body: body, Cats: p.Cats, Fams: p.Fams}
}

func programMentionsVar(p *Program, v string) bool {
	for _, s := range p.Body {
		if stmtMentions(s, v) {
			return true
		}
	}
	return false
}

func stmtMentions(s Stmt, v string) bool {
	switch n := s.(type) {
	case *LetStmt:
		return n.Name == v || exprMentions(n.Init, v)
	case *PrintStmt:
		return exprMentions(n.Arg, v)
	case *IfStmt:
		if exprMentions(n.Cond, v) {
			return true
		}
		for _, t := range n.Then {
			if stmtMentions(t, v) {
				return true
			}
		}
		for _, e := range n.Else {
			if stmtMentions(e, v) {
				return true
			}
		}
	}
	return false
}

func exprMentions(e Expr, v string) bool {
	switch n := e.(type) {
	case *Var:
		return n.Name == v
	case *Binary:
		return exprMentions(n.L, v) || exprMentions(n.R, v)
	case *Index:
		return exprMentions(n.Base, v) || exprMentions(n.Key, v)
	case *Call:
		for _, a := range n.Args {
			if exprMentions(a, v) {
				return true
			}
		}
	}
	return false
}
