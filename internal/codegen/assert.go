package codegen

import (
	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/runtime"
	"github.com/mitchellnemitz/wisp/internal/token"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// The assertion + skip builtins lower to a DIRECT located exit of the current
// (sub)shell via the new __wisp_assert_fail (exit 122) / __wisp_skip (exit 121)
// prelude helpers -- NOT the errMode pending-guard unwind. On the expected case
// each is a pure no-op (just the predicate evaluation). Every value rendered
// into a failure message goes through genDebugValue / a double-quoted expansion,
// so a metacharacter-laden value reaches stderr as inert data only (N1).

// callAssertFail emits `__wisp_assert_fail <pos> "$msg"` and marks the helper
// used. msg is a temp holding the already-built message string (inert data).
func (g *gen) callAssertFail(n *ast.CallExpr, msgTemp string) {
	g.use(runtime.AssertFail)
	g.line("__wisp_assert_fail %s \"$%s\"", g.posLiteral(n.Pos()), msgTemp)
}

// genAssert lowers assert(cond, msg). On a false cond it builds the message
// ("assertion failed" plus ": <msg>" when msg is nonempty) and exits 122. A bool
// has no value to render.
func (g *gen) genAssert(n *ast.CallExpr, args []ast.Expr) atom {
	cond := g.genExpr(args[0])
	// msg is always present (the checker fills the omitted default with "").
	msg := g.spillToTemp(g.genExpr(args[1]))
	g.line("if [ %s != true ]; then", g.word(cond))
	g.indent++
	full := g.newTemp()
	// "assertion failed" + (msg empty ? "" : ": " + msg). msg flows only through a
	// double-quoted expansion.
	g.line(`if [ -n "$%s" ]; then %s="assertion failed: $%s"; else %s="assertion failed"; fi`,
		msg, full, msg, full)
	g.callAssertFail(n, full)
	g.indent--
	g.line("fi")
	return litAtom("''")
}

// genAssertEqNe lowers assert_eq/assert_ne over a comparable type. It computes
// equality exactly like the == operator (scalar string compare, or structural
// Optional equality for a comparable Optional), then on the wrong outcome
// renders BOTH operands via debug() into "<got> <op> <want>" and exits 122.
func (g *gen) genAssertEqNe(n *ast.CallExpr, args []ast.Expr, negated bool) atom {
	t := g.resolveType(g.info.Types[args[0]])
	gotV := g.genExpr(args[0])
	wantV := g.genExpr(args[1])

	var eq atom
	if types.ComparableOptional(t) {
		eq = g.genOptionalEquality(gotV, wantV, t, false, n.CalleePos)
	} else if g.comparesAsFloat(t) {
		eq = g.emitFloatCompare("eq", gotV, wantV, n.CalleePos)
	} else {
		eq = g.genEquality(gotV, "=", wantV)
	}

	// assert_eq fails when not equal; assert_ne fails when equal. The shown
	// operator in the message is the FAILED relation: assert_eq prints "!=",
	// assert_ne prints "==".
	wantEqual := "true"
	op := "!="
	if negated {
		wantEqual = "false"
		op = "=="
	}
	g.line("if [ %s != %s ]; then", g.word(eq), wantEqual)
	g.indent++
	gotR := g.spillToTemp(g.genDebugValue(gotV, t))
	wantR := g.spillToTemp(g.genDebugValue(wantV, t))
	full := g.newTemp()
	// "assertion failed: <got> <op> <want>". Rendered values are inert data in
	// double-quoted expansions; the op literal is a compiler constant.
	g.line(`%s="assertion failed: $%s %s $%s"`, full, gotR, op, wantR)
	g.callAssertFail(n, full)
	g.indent--
	g.line("fi")
	return litAtom("''")
}

// genAssertOptional lowers assert_some/assert_none. wantTag is the tag that must
// hold ("some" / "none"); on the other tag it renders the actual Optional value
// via debug() and exits 122.
func (g *gen) genAssertOptional(n *ast.CallExpr, args []ast.Expr, wantTag string) atom {
	t := g.resolveType(g.info.Types[args[0]])
	o := g.genExpr(args[0])
	tag := g.readHandleVar(tagFieldName(o.name))
	g.line(`if [ "$%s" != %s ]; then`, tag.name, wantTag)
	g.indent++
	rendered := g.spillToTemp(g.genDebugValue(o, t))
	full := g.newTemp()
	g.line(`%s="assertion failed: expected %s, got $%s"`, full, wantTag, rendered)
	g.callAssertFail(n, full)
	g.indent--
	g.line("fi")
	return litAtom("''")
}

// genAssertResult lowers assert_ok/assert_err. wantTag is "ok" / "err"; on the
// other tag it renders the actual Result value via debug() and exits 122.
func (g *gen) genAssertResult(n *ast.CallExpr, args []ast.Expr, wantTag string) atom {
	t := g.resolveType(g.info.Types[args[0]])
	r := g.genExpr(args[0])
	tag := g.readHandleVar(tagFieldName(r.name))
	g.line(`if [ "$%s" != %s ]; then`, tag.name, wantTag)
	g.indent++
	rendered := g.spillToTemp(g.genDebugValue(r, t))
	full := g.newTemp()
	g.line(`%s="assertion failed: expected %s, got $%s"`, full, wantTag, rendered)
	g.callAssertFail(n, full)
	g.indent--
	g.line("fi")
	return litAtom("''")
}

// genAssertContains lowers assert_contains, overloaded on arg-0 like contains:
// (string, string) substring or (T[], T) element membership. The operands are
// evaluated EXACTLY ONCE (into atoms) and reused for both the membership test
// and the debug() rendering on a miss; on a miss it exits 122.
func (g *gen) genAssertContains(n *ast.CallExpr, args []ast.Expr) atom {
	hayT := g.resolveType(g.info.Types[args[0]])
	needleT := g.resolveType(g.info.Types[args[1]])
	hay := g.spillToTemp(g.genExpr(args[0]))
	needle := g.spillToTemp(g.genExpr(args[1]))

	var found atom
	if types.IsArray(hayT) {
		elemIsFloat := g.comparesAsFloat(types.ElemType(hayT))
		found = g.genArrayContainsAtoms(varAtom(hay), varAtom(needle), elemIsFloat, args[1].Pos())
	} else {
		g.use(runtime.Contains)
		g.line(`__wisp_contains "$%s" "$%s"`, hay, needle)
		ft := g.newTemp()
		g.line(`%s="$__ret"`, ft)
		found = varAtom(ft)
	}

	g.line("if [ %s != true ]; then", g.word(found))
	g.indent++
	hayR := g.spillToTemp(g.genDebugValue(varAtom(hay), hayT))
	needleR := g.spillToTemp(g.genDebugValue(varAtom(needle), needleT))
	full := g.newTemp()
	g.line(`%s="assertion failed: $%s does not contain $%s"`, full, hayR, needleR)
	g.callAssertFail(n, full)
	g.indent--
	g.line("fi")
	return litAtom("''")
}

// genArrayContainsAtoms is genArrayContains over PRE-EVALUATED atoms (the array
// handle and the target element), so the operands are not re-evaluated.
// elemIsFloat routes the element compare through __wisp_fcmp (numeric identity)
// instead of a byte-text `=` test; pos locates the fcmp helper.
func (g *gen) genArrayContainsAtoms(arr, target atom, elemIsFloat bool, pos token.Position) atom {
	res := g.newTemp()
	g.line("%s=false", res)
	idxTemp, _ := g.beginArrayLoop(arr.name)
	elemTemp := g.spillToTemp(g.readHandleVar(g.arrayElemNameDyn(arr.name, idxTemp)))
	eq := g.emitScalarEq(elemTemp, target.name, elemIsFloat, pos)
	g.line("if [ \"$%s\" = true ]; then %s=true; fi", eq, res)
	g.endArrayLoop(idxTemp)
	return varAtom(res)
}

// genSkip lowers skip(reason): prints the located reason to stderr and exits the
// current (sub)shell with the reserved SKIP code 121. The rest of the body does
// not run. The reason flows only through a double-quoted expansion.
func (g *gen) genSkip(n *ast.CallExpr, args []ast.Expr) atom {
	reason := g.spillToTemp(g.genExpr(args[0]))
	g.use(runtime.Skip)
	g.line("__wisp_skip %s \"$%s\"", g.posLiteral(n.Pos()), reason)
	return litAtom("''")
}
