package codegen

import (
	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/runtime"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// Collections-core array builtins (sort/sort_by/find/any/all/slice/concat/sum/
// range/first/last). All reuse the M3/M4 handle + counted-loop machinery; values
// flow only through quoted expansions and the eval "name=\$tmp" form.

// genFind lowers find(xs, f) -> Optional[int]: Some(first index where f(x) is
// true), else None. Built directly as the two-field Optional handle.
func (g *gen) genFind(args []ast.Expr) atom {
	id := g.genExpr(args[0])
	f := g.spillToTemp(g.genExpr(args[1]))
	out := g.allocHandle()
	g.setHandleVar(tagFieldName(out), litAtom("none")) // default None
	idxTemp, _ := g.beginArrayLoop(id.name)
	elem := g.readHandleVar(g.arrayElemNameDyn(id.name, idxTemp))
	g.line("\"$%s\" %s", f, g.word(elem))
	g.line("if [ \"$__ret\" = true ]; then")
	g.indent++
	g.setHandleVar(tagFieldName(out), litAtom("some"))
	g.setHandleVar(tagValueName(out), varAtom(idxTemp))
	g.line("break")
	g.indent--
	g.line("fi")
	g.endArrayLoop(idxTemp)
	return varAtom(out)
}

// genAnyAll lowers any/all(xs, f) -> bool with short-circuit. isAny: stop true on
// first match (init false). !isAny (all): stop false on first non-match (init true).
func (g *gen) genAnyAll(args []ast.Expr, isAny bool) atom {
	id := g.genExpr(args[0])
	f := g.spillToTemp(g.genExpr(args[1]))
	res := g.newTemp()
	if isAny {
		g.line("%s=false", res)
	} else {
		g.line("%s=true", res)
	}
	idxTemp, _ := g.beginArrayLoop(id.name)
	elem := g.readHandleVar(g.arrayElemNameDyn(id.name, idxTemp))
	g.line("\"$%s\" %s", f, g.word(elem))
	if isAny {
		g.line("if [ \"$__ret\" = true ]; then %s=true; break; fi", res)
	} else {
		g.line("if [ \"$__ret\" = false ]; then %s=false; break; fi", res)
	}
	g.endArrayLoop(idxTemp)
	return varAtom(res)
}

// genSort lowers sort(xs) -> [xs] with a type-specific comparison.
func (g *gen) genSort(n *ast.CallExpr, args []ast.Expr) atom {
	id := g.spillToTemp(g.genExpr(args[0]))
	et := types.ElemType(g.info.Types[args[0]])
	pos := g.posLiteral(n.Pos())
	out := g.insertionSort(id, func(a, b string) string {
		return g.emitSortLess(et, pos, a, b)
	})
	return varAtom(out)
}

// genSortBy lowers sort_by(xs, less) -> [xs] using the comparator.
func (g *gen) genSortBy(args []ast.Expr) atom {
	id := g.spillToTemp(g.genExpr(args[0]))
	less := g.spillToTemp(g.genExpr(args[1]))
	out := g.insertionSort(id, func(a, b string) string {
		cmp := g.newTemp()
		g.line("\"$%s\" \"$%s\" \"$%s\"", less, a, b)
		g.line("%s=\"$__ret\"", cmp)
		return cmp
	})
	return varAtom(out)
}

// emitSortLess emits a strict "a < b" comparison for sort's element type into a
// fresh bool temp (true/false), returning its name. aTmp/bTmp hold the values.
func (g *gen) emitSortLess(et types.Type, pos, aTmp, bTmp string) string {
	cmp := g.newTemp()
	// Dispatch on the single comparison-class resolver so enum-backing handling
	// cannot drift from the ordering operators and min/max. Float (raw + float-
	// backed enum), string, and int stay byte-identical to their prior arms.
	switch g.comparisonClass(et) {
	case cmpFloat:
		g.use(runtime.FCmp)
		g.line("__wisp_fcmp %s %s \"$%s\" \"$%s\"", pos, shellSingleQuote("lt"), aTmp, bTmp)
		g.line("%s=\"$__ret\"", cmp)
	case cmpString:
		g.use(runtime.Scmp)
		g.line("__wisp_scmp \"$%s\" \"$%s\"", aTmp, bTmp)
		g.line("%s=\"$__ret\"", cmp)
	case cmpBool:
		// Map each operand's true/false text to 1/0, then compare numerically so
		// false sorts below true. Both b2i calls write __ret, so spill the first.
		g.use(runtime.B2i)
		ai := g.newTemp()
		g.line("__wisp_b2i \"$%s\"", aTmp)
		g.line("%s=\"$__ret\"", ai)
		bi := g.newTemp()
		g.line("__wisp_b2i \"$%s\"", bTmp)
		g.line("%s=\"$__ret\"", bi)
		g.line("if [ \"$%s\" -lt \"$%s\" ]; then %s=true; else %s=false; fi", ai, bi, cmp, cmp)
	default: // cmpInt (plain int + int-backed value enum)
		g.line("if [ \"$%s\" -lt \"$%s\" ]; then %s=true; else %s=false; fi", aTmp, bTmp, cmp, cmp)
	}
	return cmp
}

// insertionSort copies the array in srcId into a fresh array, then stable
// insertion-sorts the fresh array in place. emitLess(aTmp, bTmp) emits a "aVal <
// bVal" comparison returning a bool temp name. The inner shift loop's bound is
// `j >= 0` (tested first), so a non-total comparator terminates with a safe
// permutation and never reads a negative index (spec). The source is unchanged.
func (g *gen) insertionSort(srcID string, emitLess func(a, b string) string) string {
	out := g.allocHandle()
	lenT := g.arrayLen(srcID)

	// Copy src -> out.
	ci := g.newTemp()
	g.line("%s=0", ci)
	g.line("while [ \"$%s\" -lt \"$%s\" ]; do", ci, lenT)
	g.indent++
	cp := g.readHandleVar(g.arrayElemNameDyn(srcID, ci))
	g.setHandleVar(g.arrayElemNameDyn(out, ci), cp)
	g.line("%s=$(( $%s + 1 ))", ci, ci)
	g.indent--
	g.line("done")
	g.line("eval \"__wisp_a_${%s}_len=\\$%s\"", out, lenT)

	// Insertion sort out[0..len).
	i := g.newTemp()
	g.line("%s=1", i)
	g.line("while [ \"$%s\" -lt \"$%s\" ]; do", i, lenT)
	g.indent++
	key := g.spillToTemp(g.readHandleVar(g.arrayElemNameDyn(out, i)))
	j := g.newTemp()
	g.line("%s=$(( $%s - 1 ))", j, i)
	g.line("while [ \"$%s\" -ge 0 ]; do", j)
	g.indent++
	oj := g.spillToTemp(g.readHandleVar(g.arrayElemNameDyn(out, j)))
	cmp := emitLess(key, oj) // key < out[j] ?
	g.line("if [ \"$%s\" != true ]; then break; fi", cmp)
	jp1 := g.newTemp()
	g.line("%s=$(( $%s + 1 ))", jp1, j)
	g.setHandleVar(g.arrayElemNameDyn(out, jp1), varAtom(oj))
	g.line("%s=$(( $%s - 1 ))", j, j)
	g.indent--
	g.line("done")
	ins := g.newTemp()
	g.line("%s=$(( $%s + 1 ))", ins, j)
	g.setHandleVar(g.arrayElemNameDyn(out, ins), varAtom(key))
	g.line("%s=$(( $%s + 1 ))", i, i)
	g.indent--
	g.line("done")
	return out
}

// genSlice lowers slice(xs, start, end) -> T[]: copy the half-open range; abort
// located on any out-of-range or inverted range (checked before any copy).
func (g *gen) genSlice(n *ast.CallExpr, args []ast.Expr) atom {
	id := g.spillToTemp(g.genExpr(args[0]))
	start := g.spillToTemp(g.genExpr(args[1]))
	end := g.spillToTemp(g.genExpr(args[2]))
	lenT := g.arrayLen(id)
	g.use(runtime.Fail)
	g.line("if [ \"$%s\" -lt 0 ] || [ \"$%s\" -lt \"$%s\" ] || [ \"$%s\" -gt \"$%s\" ]; then __wisp_fail %s %s; fi",
		start, end, start, end, lenT, g.posLiteral(n.Pos()), shellSingleQuote("slice: range out of bounds"))
	g.guardAfterSpill()
	out := g.allocHandle()
	i := g.newTemp()
	g.line("%s=\"$%s\"", i, start)
	o := g.newTemp()
	g.line("%s=0", o)
	g.line("while [ \"$%s\" -lt \"$%s\" ]; do", i, end)
	g.indent++
	e := g.readHandleVar(g.arrayElemNameDyn(id, i))
	g.setHandleVar(g.arrayElemNameDyn(out, o), e)
	g.line("%s=$(( $%s + 1 ))", i, i)
	g.line("%s=$(( $%s + 1 ))", o, o)
	g.indent--
	g.line("done")
	g.line("eval \"__wisp_a_${%s}_len=\\$%s\"", out, o)
	return varAtom(out)
}

// genConcat lowers concat(a, b) -> T[]: a's elements then b's, into a fresh array.
func (g *gen) genConcat(args []ast.Expr) atom {
	a := g.spillToTemp(g.genExpr(args[0]))
	b := g.spillToTemp(g.genExpr(args[1]))
	out := g.allocHandle()
	o := g.newTemp()
	g.line("%s=0", o)
	g.copyInto(a, out, o)
	g.copyInto(b, out, o)
	g.line("eval \"__wisp_a_${%s}_len=\\$%s\"", out, o)
	return varAtom(out)
}

// copyInto appends every element of srcID to out starting at the output index
// held in oTemp (advancing it). Used by concat.
func (g *gen) copyInto(srcID, out, oTemp string) {
	lenT := g.arrayLen(srcID)
	i := g.newTemp()
	g.line("%s=0", i)
	g.line("while [ \"$%s\" -lt \"$%s\" ]; do", i, lenT)
	g.indent++
	e := g.readHandleVar(g.arrayElemNameDyn(srcID, i))
	g.setHandleVar(g.arrayElemNameDyn(out, oTemp), e)
	g.line("%s=$(( $%s + 1 ))", i, i)
	g.line("%s=$(( $%s + 1 ))", oTemp, oTemp)
	g.indent--
	g.line("done")
}

// genSum lowers sum(xs) -> int|float. Int uses shell arithmetic (total). Float
// accumulates via __wisp_fadd (init 0.0; non-finite total faults located).
func (g *gen) genSum(n *ast.CallExpr, ci *types.CallInfo, args []ast.Expr) atom {
	id := g.spillToTemp(g.genExpr(args[0]))
	lenT := g.arrayLen(id)
	acc := g.newTemp()
	isFloat := ci.Result == types.Float
	if isFloat {
		g.line("%s=0.0", acc)
	} else {
		g.line("%s=0", acc)
	}
	i := g.newTemp()
	g.line("%s=0", i)
	g.line("while [ \"$%s\" -lt \"$%s\" ]; do", i, lenT)
	g.indent++
	e := g.spillToTemp(g.readHandleVar(g.arrayElemNameDyn(id, i)))
	if isFloat {
		g.use(runtime.FAdd)
		g.line("__wisp_fadd %s \"$%s\" \"$%s\"", g.posLiteral(n.Pos()), acc, e)
		g.line("%s=\"$__ret\"", acc)
		if g.errMode {
			g.line("if [ -n \"$__wisp_err_pending\" ]; then break; fi")
		}
	} else {
		// Bare operands (no leading $): both the running total and a summed element
		// can be INT_MIN, and $name inside $(( )) re-lexes the string-expanded 2^63
		// (dash off-by-one). A bare name reads the stored value directly and is
		// correct on dash/busybox/bash/sh -- same reason as arith(). The loop
		// counter below stays $-form: it is bounded to the array length, never
		// INT_MIN.
		g.line("%s=$(( %s + %s ))", acc, acc, e)
	}
	g.line("%s=$(( $%s + 1 ))", i, i)
	g.indent--
	g.line("done")
	if isFloat {
		g.guardAfterSpill()
	}
	return varAtom(acc)
}

// genRange lowers range(n) -> int[]: [0..n-1]; n<=0 yields an empty array.
func (g *gen) genRange(args []ast.Expr) atom {
	nT := g.spillToTemp(g.genExpr(args[0]))
	out := g.allocHandle()
	i := g.newTemp()
	g.line("%s=0", i)
	g.line("while [ \"$%s\" -lt \"$%s\" ]; do", i, nT)
	g.indent++
	g.setHandleVar(g.arrayElemNameDyn(out, i), varAtom(i))
	g.line("%s=$(( $%s + 1 ))", i, i)
	g.indent--
	g.line("done")
	g.line("eval \"__wisp_a_${%s}_len=\\$%s\"", out, i)
	return varAtom(out)
}

// genLines lowers lines(s) -> string[]: alloc a fresh array handle and let
// __wisp_lines fill its backing vars (newline split with the trailing-newline drop
// and empty-input base case). Not fallible (no <pos>, no guardAfterSpill).
func (g *gen) genLines(args []ast.Expr) atom {
	s := g.genExpr(args[0])
	id := g.allocHandle()
	g.use(runtime.Lines)
	g.line("__wisp_lines \"$%s\" %s", id, g.word(s))
	return varAtom(id)
}

// genFirstLast lowers first/last(xs) -> T: the first/last element; empty array is
// a located abort naming the op.
func (g *gen) genFirstLast(n *ast.CallExpr, args []ast.Expr, first bool) atom {
	id := g.spillToTemp(g.genExpr(args[0]))
	lenT := g.arrayLen(id)
	g.use(runtime.Fail)
	op := "first"
	if !first {
		op = "last"
	}
	g.line("if [ \"$%s\" -le 0 ]; then __wisp_fail %s %s; fi", lenT, g.posLiteral(n.Pos()), shellSingleQuote(op+": empty array"))
	g.guardAfterSpill()
	idx := g.newTemp()
	if first {
		g.line("%s=0", idx)
	} else {
		g.line("%s=$(( $%s - 1 ))", idx, lenT)
	}
	return g.readHandleVar(g.arrayElemNameDyn(id, idx))
}

// --- Collections-tail array builtins (count_where/flatten/unique/
// take/drop/pop/remove_at/insert_at). Same handle/counted-loop/eval machinery. ---

// genIndexOfElem lowers the array branch of index_of(xs, x) -> Optional[int]:
// Some(first matching index) or None. Element equality is text ("=") -- safe for
// int/bool/string per spec.
func (g *gen) genIndexOfElem(args []ast.Expr) atom {
	id := g.genExpr(args[0])
	target := g.spillToTemp(g.genExpr(args[1]))
	elemIsFloat := g.comparesAsFloat(types.ElemType(g.info.Types[args[0]]))
	out := g.allocHandle()
	g.setHandleVar(tagFieldName(out), litAtom("none"))
	idxTemp, _ := g.beginArrayLoop(id.name)
	elemTemp := g.spillToTemp(g.readHandleVar(g.arrayElemNameDyn(id.name, idxTemp)))
	eq := g.emitScalarEq(elemTemp, target, elemIsFloat, args[1].Pos())
	g.line("if [ \"$%s\" = true ]; then", eq)
	g.indent++
	g.setHandleVar(tagFieldName(out), litAtom("some"))
	g.setHandleVar(tagValueName(out), varAtom(idxTemp))
	g.line("break")
	g.indent--
	g.line("fi")
	g.endArrayLoop(idxTemp)
	return varAtom(out)
}

// genCountWhere lowers count_where(xs, f) -> int: number of elements where f
// returns true. Calls f for every element (no short-circuit). A faulting callback
// stops the loop via loopPendingBreak.
func (g *gen) genCountWhere(args []ast.Expr) atom {
	id := g.genExpr(args[0])
	f := g.spillToTemp(g.genExpr(args[1]))
	acc := g.newTemp()
	g.line("%s=0", acc)
	idxTemp, _ := g.beginArrayLoop(id.name)
	elem := g.readHandleVar(g.arrayElemNameDyn(id.name, idxTemp))
	g.line("\"$%s\" %s", f, g.word(elem))
	cbRes := g.newTemp()
	g.line("%s=\"$__ret\"", cbRes)
	g.line("if [ \"$%s\" = true ]; then %s=$(( $%s + 1 )); fi", cbRes, acc, acc)
	g.endArrayLoop(idxTemp)
	g.guardAfterSpill()
	return varAtom(acc)
}

// genFlatten lowers flatten(xs) -> T[]: all elements of each inner array
// concatenated in order. Empty inners contribute nothing.
func (g *gen) genFlatten(args []ast.Expr) atom {
	id := g.genExpr(args[0])
	out := g.allocHandle()
	outLen := g.newTemp()
	g.line("%s=0", outLen)
	// Outer loop: each element is a sub-array handle.
	outerIdx, _ := g.beginArrayLoop(id.name)
	subID := g.spillToTemp(g.readHandleVar(g.arrayElemNameDyn(id.name, outerIdx)))
	// Inner loop: copy every element of subID into out.
	innerIdx := g.newTemp()
	innerLen := g.arrayLen(subID)
	g.line("%s=0", innerIdx)
	g.line("while [ \"$%s\" -lt \"$%s\" ]; do", innerIdx, innerLen)
	g.indent++
	g.shellDepth++
	elem := g.readHandleVar(g.arrayElemNameDyn(subID, innerIdx))
	g.setHandleVar(g.arrayElemNameDyn(out, outLen), elem)
	g.line("%s=$(( $%s + 1 ))", innerIdx, innerIdx)
	g.line("%s=$(( $%s + 1 ))", outLen, outLen)
	g.shellDepth--
	g.indent--
	g.line("done")
	g.endArrayLoop(outerIdx)
	g.line("eval \"__wisp_a_${%s}_len=\\$%s\"", out, outLen)
	return varAtom(out)
}

// genUnique lowers unique(xs) -> T[]: first-occurrence-ordered deduplicated copy.
// O(n^2): acceptable for the shell target. Element equality is text ("=").
func (g *gen) genUnique(args []ast.Expr) atom {
	id := g.genExpr(args[0])
	elemIsFloat := g.comparesAsFloat(types.ElemType(g.info.Types[args[0]]))
	out := g.allocHandle()
	outLen := g.newTemp()
	g.line("%s=0", outLen)
	idxTemp, _ := g.beginArrayLoop(id.name)
	elem := g.spillToTemp(g.readHandleVar(g.arrayElemNameDyn(id.name, idxTemp)))
	// Scan out[0..outLen-1] for a duplicate.
	found := g.newTemp()
	g.line("%s=false", found)
	scanIdx := g.newTemp()
	g.line("%s=0", scanIdx)
	g.line("while [ \"$%s\" -lt \"$%s\" ]; do", scanIdx, outLen)
	g.indent++
	g.shellDepth++
	existing := g.spillToTemp(g.readHandleVar(g.arrayElemNameDyn(out, scanIdx)))
	eq := g.emitScalarEq(existing, elem, elemIsFloat, args[0].Pos())
	g.line("if [ \"$%s\" = true ]; then %s=true; break; fi", eq, found)
	g.line("%s=$(( $%s + 1 ))", scanIdx, scanIdx)
	g.shellDepth--
	g.indent--
	g.line("done")
	g.line("if [ \"$%s\" = false ]; then", found)
	g.indent++
	g.setHandleVar(g.arrayElemNameDyn(out, outLen), varAtom(elem))
	g.line("%s=$(( $%s + 1 ))", outLen, outLen)
	g.indent--
	g.line("fi")
	g.endArrayLoop(idxTemp)
	g.line("eval \"__wisp_a_${%s}_len=\\$%s\"", out, outLen)
	return varAtom(out)
}

// genTakeDrop lowers take/drop(xs, n) -> T[]: first n / all-but-first-n elements,
// clamped (n < 0 -> 0, n > len -> len). No abort.
func (g *gen) genTakeDrop(args []ast.Expr, isTake bool) atom {
	id := g.spillToTemp(g.genExpr(args[0]))
	nT := g.spillToTemp(g.genExpr(args[1]))
	lenT := g.arrayLen(id)
	// Clamp k into [0, len].
	k := g.newTemp()
	g.line("%s=\"$%s\"", k, nT)
	g.line("if [ \"$%s\" -lt 0 ]; then %s=0; fi", k, k)
	g.line("if [ \"$%s\" -gt \"$%s\" ]; then %s=\"$%s\"; fi", k, lenT, k, lenT)
	out := g.allocHandle()
	// Source index and dest index temps.
	si := g.newTemp()
	di := g.newTemp()
	if isTake {
		// Copy xs[0..k-1] -> out[0..k-1].
		g.line("%s=0", si)
		g.line("%s=0", di)
		g.line("while [ \"$%s\" -lt \"$%s\" ]; do", si, k)
		g.indent++
		g.shellDepth++
		e := g.readHandleVar(g.arrayElemNameDyn(id, si))
		g.setHandleVar(g.arrayElemNameDyn(out, di), e)
		g.line("%s=$(( $%s + 1 ))", si, si)
		g.line("%s=$(( $%s + 1 ))", di, di)
		g.shellDepth--
		g.indent--
		g.line("done")
		g.line("eval \"__wisp_a_${%s}_len=\\$%s\"", out, k)
	} else {
		// Copy xs[k..len-1] -> out[0..len-k-1].
		g.line("%s=\"$%s\"", si, k)
		g.line("%s=0", di)
		g.line("while [ \"$%s\" -lt \"$%s\" ]; do", si, lenT)
		g.indent++
		g.shellDepth++
		e := g.readHandleVar(g.arrayElemNameDyn(id, si))
		g.setHandleVar(g.arrayElemNameDyn(out, di), e)
		g.line("%s=$(( $%s + 1 ))", si, si)
		g.line("%s=$(( $%s + 1 ))", di, di)
		g.shellDepth--
		g.indent--
		g.line("done")
		dropLen := g.newTemp()
		g.line("%s=$(( $%s - $%s ))", dropLen, lenT, k)
		g.line("eval \"__wisp_a_${%s}_len=\\$%s\"", out, dropLen)
	}
	return varAtom(out)
}

// genPop lowers pop(xs) -> T: remove and return the last element; abort located if
// the array is empty.
func (g *gen) genPop(n *ast.CallExpr, args []ast.Expr) atom {
	id := g.spillToTemp(g.genExpr(args[0]))
	lenT := g.arrayLen(id)
	g.use(runtime.Fail)
	g.line("if [ \"$%s\" -le 0 ]; then __wisp_fail %s %s; fi", lenT, g.posLiteral(n.Pos()), shellSingleQuote("pop: empty array"))
	g.guardAfterSpill()
	// Read last element.
	slotIdx := g.newTemp()
	g.line("%s=$(( $%s - 1 ))", slotIdx, lenT)
	result := g.spillToTemp(g.readHandleVar(g.arrayElemNameDyn(id, slotIdx)))
	// Unset the backing slot (slotIdx is an integer temp, not an expression).
	g.line("eval \"unset %s\"", g.arrayElemNameDyn(id, slotIdx))
	// Decrement length.
	newLen := g.newTemp()
	g.line("%s=$(( $%s - 1 ))", newLen, lenT)
	g.line("eval \"__wisp_a_${%s}_len=\\$%s\"", id, newLen)
	return varAtom(result)
}

// genRemoveAt lowers remove_at(xs, i) -> void: remove element at index i by
// left-shifting the tail; abort located if out of range.
func (g *gen) genRemoveAt(n *ast.CallExpr, args []ast.Expr) atom {
	id := g.spillToTemp(g.genExpr(args[0]))
	iT := g.spillToTemp(g.genExpr(args[1]))
	lenT := g.arrayLen(id)
	// Bounds check: 0 <= i < len.
	g.use(runtime.Fail)
	g.line("case \"$%s\" in -*) __wisp_fail %s %s ;; esac", iT, g.posLiteral(n.Pos()), shellSingleQuote("remove_at: index out of range"))
	g.line("if [ \"$%s\" -ge \"$%s\" ]; then __wisp_fail %s %s; fi", iT, lenT, g.posLiteral(n.Pos()), shellSingleQuote("remove_at: index out of range"))
	g.guardAfterSpill()
	// Left-shift: xs[j] = xs[j+1] for j in [i, len-2].
	j := g.newTemp()
	g.line("%s=\"$%s\"", j, iT)
	bound := g.newTemp()
	g.line("%s=$(( $%s - 1 ))", bound, lenT)
	g.line("while [ \"$%s\" -lt \"$%s\" ]; do", j, bound)
	g.indent++
	g.shellDepth++
	jp1 := g.newTemp()
	g.line("%s=$(( $%s + 1 ))", jp1, j)
	src := g.readHandleVar(g.arrayElemNameDyn(id, jp1))
	g.setHandleVar(g.arrayElemNameDyn(id, j), src)
	g.line("%s=$(( $%s + 1 ))", j, j)
	g.shellDepth--
	g.indent--
	g.line("done")
	// Unset stale tail slot (j now equals len-1 = bound).
	g.line("eval \"unset %s\"", g.arrayElemNameDyn(id, bound))
	// Decrement length.
	newLen := g.newTemp()
	g.line("%s=$(( $%s - 1 ))", newLen, lenT)
	g.line("eval \"__wisp_a_${%s}_len=\\$%s\"", id, newLen)
	return litAtom("''")
}

// genInsertAt lowers insert_at(xs, i, v) -> void: insert v at index i by
// right-shifting the tail (high-to-low); abort located if out of range.
func (g *gen) genInsertAt(n *ast.CallExpr, args []ast.Expr) atom {
	id := g.spillToTemp(g.genExpr(args[0]))
	iT := g.spillToTemp(g.genExpr(args[1]))
	v := g.genExpr(args[2])
	lenT := g.arrayLen(id)
	// Bounds check: 0 <= i <= len (i==len is a valid append).
	g.use(runtime.Fail)
	g.line("case \"$%s\" in -*) __wisp_fail %s %s ;; esac", iT, g.posLiteral(n.Pos()), shellSingleQuote("insert_at: index out of range"))
	g.line("if [ \"$%s\" -gt \"$%s\" ]; then __wisp_fail %s %s; fi", iT, lenT, g.posLiteral(n.Pos()), shellSingleQuote("insert_at: index out of range"))
	g.guardAfterSpill()
	// Right-shift: xs[j+1] = xs[j] for j from len-1 down to i.
	j := g.newTemp()
	g.line("%s=$(( $%s - 1 ))", j, lenT)
	g.line("while [ \"$%s\" -ge \"$%s\" ]; do", j, iT)
	g.indent++
	g.shellDepth++
	jp1 := g.newTemp()
	g.line("%s=$(( $%s + 1 ))", jp1, j)
	src := g.readHandleVar(g.arrayElemNameDyn(id, j))
	g.setHandleVar(g.arrayElemNameDyn(id, jp1), src)
	g.line("%s=$(( $%s - 1 ))", j, j)
	g.shellDepth--
	g.indent--
	g.line("done")
	// Write v at position i.
	vT := g.spillToTemp(v)
	g.setHandleVar(g.arrayElemNameDyn(id, iT), varAtom(vT))
	// Increment length.
	newLen := g.newTemp()
	g.line("%s=$(( $%s + 1 ))", newLen, lenT)
	g.line("eval \"__wisp_a_${%s}_len=\\$%s\"", id, newLen)
	return litAtom("''")
}
