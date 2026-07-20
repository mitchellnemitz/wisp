package codegen

import (
	"strconv"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/runtime"
)

// genTupleLit lowers `(e1, e2, ..., en)`: evaluate each element in source order,
// allocate a fresh handle, then setHandleVar for each element using the struct
// backing variable scheme __wisp_s_${id}_<i>. Returns varAtom(id).
//
// Mirrors genArrayLit (aggregate.go) and genStructLit (aggregate.go): collect
// element atoms first (preserving source order and ensuring element side effects
// precede the allocHandle call), then emit setHandleVar for each.
// setHandleVar spills its value via spillToTemp internally, so no explicit spill
// is needed here.
func (g *gen) genTupleLit(n *ast.TupleLit) atom {
	elems := make([]atom, len(n.Elems))
	for i, e := range n.Elems {
		elems[i] = g.genExpr(e)
	}
	id := g.allocHandle()
	for i, a := range elems {
		g.setHandleVar("__wisp_s_${"+id+"}_"+strconv.Itoa(i), a)
	}
	return varAtom(id)
}

// genZip lowers zip(a:T[], b:U[]) -> (T,U)[]: pair the two input arrays
// element-wise up to the shorter length. The result is a fresh array of tuple
// handles. The loop is written manually (over a computed minLen) because the
// loop bound is min(len(a), len(b)), not a single array's length. The body
// contains no break/continue and no fallible call, so loopPendingBreak is not
// needed (unlike beginArrayLoop which adds it for user-closure invocations).
func (g *gen) genZip(n *ast.CallExpr, args []ast.Expr) atom {
	// Evaluate both array handles before allocating any result handles.
	aID := g.spillToTemp(g.genExpr(args[0]))
	bID := g.spillToTemp(g.genExpr(args[1]))

	// Compute min(len(a), len(b)).
	aLen := g.arrayLen(aID)
	bLen := g.arrayLen(bID)
	minLen := g.newTemp()
	g.line("if [ \"$%s\" -le \"$%s\" ]; then %s=$%s; else %s=$%s; fi", aLen, bLen, minLen, aLen, minLen, bLen)

	// Allocate the result array.
	out := g.allocHandle()
	outLen := g.newTemp()
	g.line("%s=0", outLen)

	// Loop index 0..minLen-1.
	idx := g.newTemp()
	g.line("%s=0", idx)
	g.line("while [ \"$%s\" -lt \"$%s\" ]; do", idx, minLen)
	g.indent++
	g.shellDepth++

	// Read a[idx] and b[idx].
	aElem := g.spillToTemp(g.readHandleVar(g.arrayElemNameDyn(aID, idx)))
	bElem := g.spillToTemp(g.readHandleVar(g.arrayElemNameDyn(bID, idx)))

	// Build a fresh tuple handle for this pair.
	tupleID := g.allocHandle()
	g.setHandleVar("__wisp_s_${"+tupleID+"}_0", varAtom(aElem))
	g.setHandleVar("__wisp_s_${"+tupleID+"}_1", varAtom(bElem))

	// Store the tuple handle as result[outLen].
	g.setHandleVar(g.arrayElemNameDyn(out, outLen), varAtom(tupleID))
	g.line("%s=$(( $%s + 1 ))", outLen, outLen)
	g.line("%s=$(( $%s + 1 ))", idx, idx)

	g.shellDepth--
	g.indent--
	g.line("done")
	g.line("eval \"__wisp_a_${%s}_len=\\$%s\"", out, outLen)
	return varAtom(out)
}

// genParseArgs lowers parse_args(args:string[], value_flags:string[]) ->
// ({string:string}, string[], string[]). It pre-allocates the three result
// handles (a values dict + a switches array + a positionals array), initializes
// the dict's insertion-order key list to empty (the genDictLit convention), and
// lets the __wisp_parse_args prelude helper scan the args array and fill all
// three handles' backing vars. The handles are then wrapped in a fresh 3-tuple
// (the genTupleLit field scheme). parse_args is TOTAL -- no fault path, so no
// guardAfterSpill boilerplate (unlike the fallible genSplit/genListDir).
func (g *gen) genParseArgs(args []ast.Expr) atom {
	// Evaluate both input array handles before allocating any result handles.
	argsID := g.spillToTemp(g.genExpr(args[0]))
	vfID := g.spillToTemp(g.genExpr(args[1]))

	// Allocate the result handles. The dict's key list starts empty (matching
	// genDictLit) so the helper's append-if-new sees a well-formed list.
	dictID := g.allocHandle()
	g.line("eval \"%s=''\"", g.dictKeysName(dictID))
	swID := g.allocHandle()
	posID := g.allocHandle()

	g.use(runtime.ParseArgs)
	g.line("__wisp_parse_args \"$%s\" \"$%s\" \"$%s\" \"$%s\" \"$%s\"", dictID, swID, posID, argsID, vfID)

	// Wrap the three handles in a fresh 3-tuple.
	tup := g.allocHandle()
	g.setHandleVar("__wisp_s_${"+tup+"}_0", varAtom(dictID))
	g.setHandleVar("__wisp_s_${"+tup+"}_1", varAtom(swID))
	g.setHandleVar("__wisp_s_${"+tup+"}_2", varAtom(posID))
	return varAtom(tup)
}

// genTupleIndex lowers a constant-index read t[k] where the checker has already
// proven k is an *ast.IntLit in range. Reads __wisp_s_${id}_<k> directly via
// readHandleVar -- a pure read of a known-present field, no bounds check, no
// abort path (the checker guarantees bounds at compile time).
//
// INJECTION SAFETY: the index MUST be parsed to an int via strconv.Atoi and
// re-rendered with strconv.Itoa, NOT used as lit.Raw verbatim. The lexer accepts
// leading zeros, so t[01] has lit.Raw "01" while genTupleLit wrote the field as
// _1 (strconv.Itoa(1)). Using Raw verbatim would read the nonexistent field
// __wisp_s_${id}_01 and silently return the empty string.
func (g *gen) genTupleIndex(n *ast.IndexExpr) atom {
	id := g.genExpr(n.X)
	lit := n.Index.(*ast.IntLit)  // checker guaranteed this is *ast.IntLit
	k, _ := strconv.Atoi(lit.Raw) // checker proved this parses and is in range
	return g.readHandleVar("__wisp_s_${" + id.name + "}_" + strconv.Itoa(k))
}
