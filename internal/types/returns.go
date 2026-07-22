package types

import "github.com/mitchellnemitz/wisp/internal/ast"

// blockReturns reports whether the statement list definitely returns on every
// control-flow path (rule 4). This is a conservative structural analysis:
//
//   - a return statement returns;
//   - an if/else-if/else returns iff every arm (including a present else)
//     returns; an if without an else does not guarantee a return;
//   - a switch returns iff every case body AND the (mandatory) default returns;
//   - while/for are not treated as guaranteed-returning (the body may run zero
//     times, and break can exit early);
//   - any later statement after a guaranteed-returning statement keeps the
//     block returning.
func blockReturns(stmts []ast.Stmt, info *Info) bool {
	for _, s := range stmts {
		if stmtReturns(s, info) {
			return true
		}
	}
	return false
}

func stmtReturns(s ast.Stmt, info *Info) bool {
	switch n := s.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.ThrowStmt:
		// throw terminates a path like return (spec 3): a function whose every path
		// ends in return or throw satisfies all-paths-return. A TryStmt is NOT
		// terminating (it falls through; return is forbidden in its body) and relies
		// on the default:false below.
		return true
	case *ast.IfStmt:
		if n.Else == nil {
			return false
		}
		if !blockReturns(n.Then, info) {
			return false
		}
		for _, ei := range n.ElseIfs {
			if !blockReturns(ei.Body, info) {
				return false
			}
		}
		return blockReturns(n.Else, info)
	case *ast.SwitchStmt:
		// A present default must itself return (unchanged). A defaultless
		// switch is only a terminator candidate when checkSwitch (stmt.go:
		// 850-864) would have accepted it as exhaustive over an enum subject
		// -- recomputed here independently, mirroring matchReturns's own
		// recomputation of Optional/Result coverage rather than a stored flag.
		if n.Default != nil {
			if !blockReturns(n.Default, info) {
				return false
			}
		} else if !enumSwitchExhaustive(n, info) {
			return false
		}
		for _, cs := range n.Cases {
			if !blockReturns(cs.Body, info) {
				return false
			}
		}
		return true
	case *ast.MatchStmt:
		return matchReturns(n, info)
	default:
		return false
	}
}

// enumSwitchExhaustive reports whether a defaultless switch's subject is an
// enum and every variant is covered by the case set. It recomputes coverage
// independently of checkSwitch's own exhaustiveness pass (stmt.go:787-836),
// the same way matchReturns recomputes Optional/Result coverage from
// info.Types rather than consuming a stored flag -- returns.go runs as a
// separate, later analysis over the already-type-checked AST + Info, and
// takes *Info only (no *checker), matching matchReturns's signature.
func enumSwitchExhaustive(n *ast.SwitchStmt, info *Info) bool {
	subj := info.Types[n.Subject]
	enum, ok := info.Enums[string(subj)]
	if !ok {
		return false
	}
	remaining := make(map[int64]bool, len(enum.Consts))
	for _, cv := range enum.Consts {
		if iv, ok := cv.(int64); ok {
			remaining[iv] = true
		}
	}
	for _, cs := range n.Cases {
		for _, v := range cs.Values {
			if fv, ok := info.FoldedValues[v]; ok {
				if iv, ok := fv.(int64); ok {
					delete(remaining, iv)
				}
			}
		}
	}
	return len(remaining) == 0
}

// matchReturns reports whether a match statement definitely returns on every
// reachable path. A trailing wildcard that covers zero variants (zero-coverage,
// unreachable) is excluded from the analysis per AC-26.
func matchReturns(n *ast.MatchStmt, info *Info) bool {
	if len(n.Arms) == 0 {
		return false
	}
	st := info.Types[n.Scrutinee]
	allVariants := variantsOf(st, info)
	allVariantSet := make(map[string]bool, len(allVariants))
	for _, v := range allVariants {
		allVariantSet[v] = true
	}
	covered := make(map[string]bool)
	for _, arm := range n.Arms {
		if cp, ok := arm.Pattern.(*ast.ConstructorPat); ok {
			if allVariantSet[cp.Variant] {
				covered[cp.Variant] = true
			}
		}
	}
	wildcardIsCoverage := len(covered) < len(allVariants)
	for _, arm := range n.Arms {
		if _, ok := arm.Pattern.(*ast.WildcardPat); ok {
			if !wildcardIsCoverage {
				continue // zero-coverage wildcard: unreachable (AC-26)
			}
		}
		if !blockReturns(arm.Body, info) {
			return false
		}
	}
	return true
}
