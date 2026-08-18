// internal/fuzz/intmin.go
package fuzz

const intMinLiteral = "-9223372036854775808"
const intMaxLiteral = "9223372036854775807"

// programReachesIntMinArith reports whether the program feeds INT_MIN into a
// SOURCE-LEVEL shell arithmetic expansion `$(( ))` -- the ONLY construct that hits
// zsh's 2^63 limitation (internal/codegen/expr.go arith(); zsh cannot represent the
// magnitude in `$(( ))` bare or dollar-prefixed). It tracks variables bound
// (directly, or transitively via a simple alias) to an INT_MIN source and flags
// exactly two shapes: (a) an INT_MIN source as a direct arithmetic (`+ - * / %`)
// operand, or (b) such a variable used as an arithmetic operand.
//
// It deliberately does NOT carve INT_MIN passed to arith-related builtins
// (abs/sign/min/max/clamp/gcd/lcm). Verified empirically against the real
// codegen/runtime by running the four shells: abs/gcd/lcm detect INT_MIN via a
// bit-doubling overflow model plus a `[ "$x" -eq "$min" ]` guard and ABORT
// uniformly on all four shells (identical stdout+exit, so compareRuns sees no
// divergence); min/max/clamp/sign lower to `[ -lt/-le/-gt/-ge ]` integer TESTS,
// which zsh evaluates correctly on an INT_MIN shell word (only `$(( ))` LITERAL
// parsing truncates). None of them diverges on zsh, so carving them would suppress
// nothing on the correct compiler while MASKING any genuine zsh-specific regression
// in those builtins -- exactly what FR-015's "narrowest carve" forbids.
//
// This function is the DONE-SIGNAL predicate (SC-008): reverting the INT_MIN
// arithmetic fix reintroduces a divergence the oracle cannot carve, and
// TestDoneSignalReachableViaVar fails. It deliberately covers ONLY the direct
// min-side shapes; the general boundary carve (max side, and min-side data flow
// through collection/call boundaries) lives in programReachesBoundaryArith.
func programReachesIntMinArith(p *Program) bool {
	imVars := map[string]bool{}
	return bodyHasIntMinArith(p.Body, imVars)
}

func bodyHasIntMinArith(body []Stmt, imVars map[string]bool) bool {
	for _, s := range body {
		switch n := s.(type) {
		case *LetStmt:
			if exprHasIntMinArith(n.Init, imVars) {
				return true
			}
			if isIntMinSource(n.Init) || isIntMinVar(n.Init, imVars) {
				imVars[n.Name] = true // this binding now holds INT_MIN
			}
		case *PrintStmt:
			if exprHasIntMinArith(n.Arg, imVars) {
				return true
			}
		case *IfStmt:
			if exprHasIntMinArith(n.Cond, imVars) {
				return true
			}
			// branch scopes inherit the outer imVars (conservative, safe).
			if bodyHasIntMinArith(n.Then, copySet(imVars)) {
				return true
			}
			if bodyHasIntMinArith(n.Else, copySet(imVars)) {
				return true
			}
		}
	}
	return false
}

func copySet(m map[string]bool) map[string]bool {
	c := make(map[string]bool, len(m))
	for k := range m {
		c[k] = true
	}
	return c
}

func isIntMinSource(e Expr) bool {
	switch n := e.(type) {
	case *IntLit:
		return n.V == intMinLiteral
	case *Call:
		return n.Builtin == "math.int_min"
	}
	return false
}

func isIntMinVar(e Expr, imVars map[string]bool) bool {
	v, ok := e.(*Var)
	return ok && imVars[v.Name]
}

func isArithOp(op string) bool {
	switch op {
	case "+", "-", "*", "/", "%":
		return true
	}
	return false
}

// exprHasIntMinArith returns true if an arithmetic Binary has an operand that is
// an INT_MIN source or an INT_MIN-bearing variable.
func exprHasIntMinArith(e Expr, imVars map[string]bool) bool {
	switch n := e.(type) {
	case *Binary:
		if isArithOp(n.Op) {
			if isIntMinSource(n.L) || isIntMinVar(n.L, imVars) || isIntMinSource(n.R) || isIntMinVar(n.R, imVars) {
				return true
			}
		}
		return exprHasIntMinArith(n.L, imVars) || exprHasIntMinArith(n.R, imVars)
	case *Call:
		// Builtins are NOT an arithmetic entry point for the carve (see
		// programReachesIntMinArith): INT_MIN into abs/gcd/lcm aborts uniformly and
		// into min/max/clamp/sign uses `[ -lt ]` tests, neither hitting zsh's $(( )).
		// Only recurse into args to catch a nested source-level Binary.
		for _, a := range n.Args {
			if exprHasIntMinArith(a, imVars) {
				return true
			}
		}
	case *Index:
		return exprHasIntMinArith(n.Base, imVars) || exprHasIntMinArith(n.Key, imVars)
	case *ArrayLit:
		for _, el := range n.Elems {
			if exprHasIntMinArith(el, imVars) {
				return true
			}
		}
	case *DictLit:
		for _, v := range n.Vals {
			if exprHasIntMinArith(v, imVars) {
				return true
			}
		}
	}
	return false
}

// programReachesBoundaryArith is the general boundary carve (the extension
// FR-015's design discipline calls for when a NEW documented-zsh-residual shape
// is empirically demonstrated). It reports whether a value at the int64 boundary
// -- INT64_MIN or INT64_MAX, from a literal or math.int_min()/math.int_max() --
// can reach shell arithmetic `$(( ))`, tracking the taint through the IR-visible
// data flow (variable bindings, arithmetic composition, collection literals, and
// the dict.get/unwrap_or/index carriers). Such a reach is a `carved_zsh` program.
//
// WHY THIS EXTENSION EXISTS (empirical record, 2026-08-17, first real four-shell
// run after busybox was built locally): the FIRST version of this detector
// claimed "computed overflow (e.g. INT_MAX + 1 wrapping at runtime) is NOT
// carved ... all four shells wrap 64-bit arithmetic identically". That is true
// for the wrap itself, but FALSE for what follows it: the wrapped value is stored
// in a variable, and the NEXT `$(( ))` reads that variable's VALUE TEXT. zsh's
// arithmetic engine converts parameter strings with its own number parser, which
// truncates any magnitude above 2^63-1 after 18 digits and continues with the
// truncated value (exit 0) -- the documented "zsh residual" from language.md and
// design-decisions.md (a warning followed by a truncated wrong value, not an
// abort), reached
// here from the POSITIVE side (int_max() + 1, or an INT64_MAX literal + 1)
// instead of an INT_MIN literal. The baseline's first run diverged on exactly
// this shape 4 times (seed 20260725 idx 142/220/392/408), plus one min-side case
// hidden behind dict.get/unwrap_or (seed 1 idx 20) and one max-side literal case
// (seed 2 idx 34). All six are the SAME documented zsh residual, structurally
// reachable from boundary SOURCES, so the carve is widened to those sources and
// a taint trace through the abstraction boundaries that can carry a boundary
// value. Everything outside this reach stays a hard bug: any other divergence is
// still compared against all four shells, zsh included.
func programReachesBoundaryArith(p *Program) bool {
	return bodyReachesBoundaryArith(p.Body, map[string]bool{})
}

func bodyReachesBoundaryArith(body []Stmt, tv map[string]bool) bool {
	for _, s := range body {
		switch n := s.(type) {
		case *LetStmt:
			if exprHasBoundaryArith(n.Init, tv) {
				return true
			}
			if exprTainted(n.Init, tv) {
				tv[n.Name] = true
			}
		case *PrintStmt:
			if exprHasBoundaryArith(n.Arg, tv) {
				return true
			}
		case *IfStmt:
			if exprHasBoundaryArith(n.Cond, tv) {
				return true
			}
			if bodyReachesBoundaryArith(n.Then, copySet(tv)) {
				return true
			}
			if bodyReachesBoundaryArith(n.Else, copySet(tv)) {
				return true
			}
		}
	}
	return false
}

// exprTainted reports whether e's value (or, for a container, any element/value
// it carries) derives from a boundary source. The carriers are deliberately
// narrow: containers and the identity-ish extraction builtins dict.get, unwrap_or,
// and Index. Other builtins (math.*, string.*, ...) do NOT propagate taint:
// abs/gcd/lcm abort uniformly on INT_MIN and min/max/clamp/sign use `[ ]` tests
// that zsh evaluates correctly, so a tainted value THROUGH them never reaches
// `$(( ))` with the boundary magnitude intact.
func exprTainted(e Expr, tv map[string]bool) bool {
	switch n := e.(type) {
	case *IntLit:
		return n.V == intMinLiteral || n.V == intMaxLiteral
	case *Var:
		return tv[n.Name]
	case *Binary:
		if isArithOp(n.Op) {
			return exprTainted(n.L, tv) || exprTainted(n.R, tv)
		}
		// Comparison/boolean results are bool; the boundary magnitude does not
		// survive, so the result is not tainted.
		return false
	case *ArrayLit:
		for _, el := range n.Elems {
			if exprTainted(el, tv) {
				return true
			}
		}
		return false
	case *DictLit:
		for _, v := range n.Vals {
			if exprTainted(v, tv) {
				return true
			}
		}
		return false
	case *Index:
		return exprTainted(n.Base, tv)
	case *Call:
		switch n.Builtin {
		case "math.int_min", "math.int_max":
			return true
		case "dict.get", "unwrap_or":
			// dict.get(tainted-container, k) extracts a carried value; unwrap_or
			// returns the carried value or the default (either may be tainted).
			for _, a := range n.Args {
				if exprTainted(a, tv) {
					return true
				}
			}
		}
		return false
	}
	return false
}

// exprHasBoundaryArith reports whether an arithmetic Binary has an operand whose
// value is boundary-tainted -- the reach that hits zsh's `$(( ))` truncation.
func exprHasBoundaryArith(e Expr, tv map[string]bool) bool {
	switch n := e.(type) {
	case *Binary:
		if isArithOp(n.Op) {
			if exprTainted(n.L, tv) || exprTainted(n.R, tv) {
				return true
			}
		}
		return exprHasBoundaryArith(n.L, tv) || exprHasBoundaryArith(n.R, tv)
	case *Call:
		for _, a := range n.Args {
			if exprHasBoundaryArith(a, tv) {
				return true
			}
		}
	case *Index:
		return exprHasBoundaryArith(n.Base, tv) || exprHasBoundaryArith(n.Key, tv)
	case *ArrayLit:
		for _, el := range n.Elems {
			if exprHasBoundaryArith(el, tv) {
				return true
			}
		}
	case *DictLit:
		for _, v := range n.Vals {
			if exprHasBoundaryArith(v, tv) {
				return true
			}
		}
	}
	return false
}
