// internal/fuzz/intmin.go
package fuzz

const intMinLiteral = "-9223372036854775808"

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
// in those builtins -- exactly what FR-015's "narrowest carve" forbids. COMPUTED
// overflow (e.g. INT_MAX + 1 wrapping at runtime) is likewise NOT carved: it never
// places the 19-digit literal into `$(( ))` source text, so all four shells wrap
// 64-bit arithmetic identically. The carve stays exactly the source-level `$(( ))`
// operand case; anything else is a hard bug.
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
