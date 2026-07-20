package codegen

import (
	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/runtime"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// Collections/numeric overloaded builtins clamp and sign, lowered as nested
// comparisons (no dedicated runtime helper). Int uses shell `[ ]`; float uses the
// M3 __wisp_fcmp. Both select an operand unchanged (no arithmetic), so the float
// case adds no finiteness obligation and never aborts.

// genClamp lowers clamp(x, lo, hi) = max(lo, min(x, hi)). The contract requires
// lo <= hi; this is a documented precondition (see the stdlib guide), not a
// runtime abort. With lo > hi the outer max(lo, ...) wins and the result is lo,
// the defined-but-unchecked behavior of a violated precondition.
func (g *gen) genClamp(n *ast.CallExpr, args []ast.Expr) atom {
	x := g.spillToTemp(g.genExpr(args[0]))
	lo := g.spillToTemp(g.genExpr(args[1]))
	hi := g.spillToTemp(g.genExpr(args[2]))
	isFloat := g.info.Types[args[0]] == types.Float
	pos := g.posLiteral(args[0].Pos())

	m := g.newTemp() // m = min(x, hi)
	if isFloat {
		g.use(runtime.FCmp)
		g.line("__wisp_fcmp %s %s \"$%s\" \"$%s\"", pos, shellSingleQuote("le"), x, hi)
		g.line("if [ \"$__ret\" = true ]; then %s=\"$%s\"; else %s=\"$%s\"; fi", m, x, m, hi)
	} else {
		g.line("if [ \"$%s\" -le \"$%s\" ]; then %s=\"$%s\"; else %s=\"$%s\"; fi", x, hi, m, x, m, hi)
	}

	r := g.newTemp() // r = max(lo, m)
	if isFloat {
		g.use(runtime.FCmp)
		g.line("__wisp_fcmp %s %s \"$%s\" \"$%s\"", pos, shellSingleQuote("ge"), lo, m)
		g.line("if [ \"$__ret\" = true ]; then %s=\"$%s\"; else %s=\"$%s\"; fi", r, lo, r, m)
	} else {
		g.line("if [ \"$%s\" -ge \"$%s\" ]; then %s=\"$%s\"; else %s=\"$%s\"; fi", lo, m, r, lo, r, m)
	}
	return varAtom(r)
}

// genSign lowers sign(x) -> -1/0/1.
func (g *gen) genSign(n *ast.CallExpr, args []ast.Expr) atom {
	x := g.spillToTemp(g.genExpr(args[0]))
	r := g.newTemp()
	if g.info.Types[args[0]] == types.Float {
		pos := g.posLiteral(args[0].Pos())
		g.use(runtime.FCmp)
		lt := g.newTemp()
		g.line("__wisp_fcmp %s %s \"$%s\" 0", pos, shellSingleQuote("lt"), x)
		g.line("%s=\"$__ret\"", lt)
		g.line("__wisp_fcmp %s %s \"$%s\" 0", pos, shellSingleQuote("gt"), x)
		g.line("if [ \"$%s\" = true ]; then %s=-1; elif [ \"$__ret\" = true ]; then %s=1; else %s=0; fi", lt, r, r, r)
	} else {
		g.line("if [ \"$%s\" -lt 0 ]; then %s=-1; elif [ \"$%s\" -gt 0 ]; then %s=1; else %s=0; fi", x, r, x, r, r)
	}
	return varAtom(r)
}
