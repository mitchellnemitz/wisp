package codegen

import (
	"strconv"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/runtime"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// Aggregate lowering (M3 PR-B). All three aggregates are reference handles: an
// instance is an integer id from __wisp_alloc, and its fields/elements live in
// namespaced shell variables (spec 4.1):
//   struct field f of id  -> __wisp_s_<id>_<f>
//   array element i of id -> __wisp_a_<id>_<i>
//   array length of id    -> __wisp_a_<id>_len
//
// The id is a runtime value, so the concrete variable name is built at runtime.
// Codegen emits the read/write DIRECTLY (no generic get/set helper) via `eval`
// with a single double-quoted argument: the id is a non-negative decimal int
// (the alloc counter), so the constructed name is a safe identifier, and the
// VALUE is carried by a deferred `\$temp` expansion (or an already-safe literal
// token), so no runtime data is re-parsed (spec 9.6 invariant 7).

// allocHandle calls __wisp_alloc and spills the fresh id into a temp.
func (g *gen) allocHandle() string {
	g.use(runtime.Alloc)
	g.line("__wisp_alloc")
	t := g.newTemp()
	g.line("%s=\"$__ret\"", t)
	return t
}

// setHandleVar emits an assignment of a handle-backing variable. nameExpr is the
// shell text (already safe) that, inside the eval, evaluates to the backing
// variable name -- e.g. `__wisp_s_${idTemp}_x` or `__wisp_a_${idTemp}_0`. The
// value is ALWAYS carried through a shell variable and referenced as a deferred
// `\$temp`: the whole `eval` argument is double-quoted, and inside double quotes
// only `\$temp` (-> `$temp`, expanded exactly once by eval) is injection-safe --
// a raw single-quoted literal token would NOT be inert there (its `$`/backtick/
// quotes stay active). So a literal value is first spilled to a temp.
func (g *gen) setHandleVar(nameExpr string, val atom) {
	vt := g.spillToTemp(val)
	g.line("eval \"%s=\\$%s\"", nameExpr, vt)
}

// setRunResultFields sets the stdout/stderr/code fields of a RunResult handle
// from the __wisp_rf_* temps left by the preceding __wisp_run_full/
// __wisp_run_input_full/__wisp_pipe/__wisp_run_env_argv+__wisp_run_full call.
func (g *gen) setRunResultFields(hid string) {
	g.setHandleVar("__wisp_s_${"+hid+"}_stdout", varAtom("__wisp_rf_stdout"))
	g.setHandleVar("__wisp_s_${"+hid+"}_stderr", varAtom("__wisp_rf_stderr"))
	g.setHandleVar("__wisp_s_${"+hid+"}_code", varAtom("__wisp_rf_code"))
}

// readHandleVar emits a read of a handle-backing variable into a fresh temp.
// nameExpr is the same safe name text used by setHandleVar. The read goes
// through __ret via a deferred `\$<name>` expansion (double-quote safe), then is
// spilled so a later read cannot clobber it.
func (g *gen) readHandleVar(nameExpr string) atom {
	g.line("eval \"__ret=%s\"", "\\$"+nameExpr)
	t := g.newTemp()
	g.line("%s=\"$__ret\"", t)
	return varAtom(t)
}

// --- struct ---

// genStructLit lowers `Name { f: v, ... }`: allocate a handle, then set each
// field var in declaration order (fields evaluated in source order). Returns the
// id atom.
func (g *gen) genStructLit(n *ast.StructLit) atom {
	// info.Structs is keyed by the struct's internal token (Name@modid, M8); the
	// literal's resolved type IS that token, so index by it rather than the source
	// name (which is ambiguous across modules).
	si := g.info.Structs[string(g.info.Types[n])]
	// Evaluate field values in source order, spilling each to a stable atom.
	values := make(map[string]atom, len(n.Fields))
	for _, f := range n.Fields {
		values[f.Name] = g.genExpr(f.Value)
	}
	id := g.allocHandle()
	// Emit field sets in declaration order for deterministic output.
	for _, f := range si.Fields {
		v := values[f.Name]
		g.setHandleVar("__wisp_s_${"+id+"}_"+f.Name, v)
	}
	return varAtom(id)
}

// genFieldAccess lowers `x.f` to a read of __wisp_s_<id>_<f>.
func (g *gen) genFieldAccess(n *ast.FieldAccess) atom {
	x := g.genExpr(n.X)
	return g.readHandleVar("__wisp_s_${" + x.name + "}_" + n.Field)
}

// genFieldAssign lowers `target.f = value`.
func (g *gen) genFieldAssign(n *ast.FieldAssignStmt) {
	x := g.genExpr(n.Target)
	v := g.genExpr(n.Value)
	g.setHandleVar("__wisp_s_${"+x.name+"}_"+n.Field, v)
}

// --- array ---

// genArrayLit lowers `[a, b, c]`: allocate a handle, set each element var and
// the length. Returns the id atom.
func (g *gen) genArrayLit(n *ast.ArrayLit) atom {
	elems := make([]atom, len(n.Elems))
	for i, e := range n.Elems {
		elems[i] = g.genExpr(e)
	}
	id := g.allocHandle()
	for i, a := range elems {
		g.setHandleVar(g.arrayElemNameConst(id, i), a)
	}
	g.line("eval \"__wisp_a_${%s}_len=%d\"", id, len(n.Elems))
	return varAtom(id)
}

// arrayElemNameDyn builds the backing-variable name text for a RUNTIME index
// held in idxTemp: `__wisp_a_${idTemp}_${idxTemp}`. The index is int-valid so the
// constructed name is a safe identifier.
func (g *gen) arrayElemNameDyn(idTemp, idxTemp string) string {
	return "__wisp_a_${" + idTemp + "}_${" + idxTemp + "}"
}

// arrayElemNameConst builds the backing-variable name text for a COMPILE-TIME
// constant index i (an array-literal position): `__wisp_a_${idTemp}_<i>`.
func (g *gen) arrayElemNameConst(idTemp string, i int) string {
	return "__wisp_a_${" + idTemp + "}_" + strconv.Itoa(i)
}

// genIndexExpr lowers `x[i]`. For an array it is a bounds-checked element read:
// the bounds check (non-negative int < len) runs BEFORE the backing name is
// built (spec 4.3); a negative or >= len index is a located abort. For a dict it
// is a key lookup: encode the key to a token (BEFORE the backing name), abort
// located via __wisp_dict_miss if the key is not in the dict, else read the
// entry var.
func (g *gen) genIndexExpr(n *ast.IndexExpr) atom {
	if types.IsDict(g.info.Types[n.X]) {
		return g.genDictLookup(n)
	}
	// Tuple: checker proved n.Index is a constant *ast.IntLit in range;
	// read the backing field directly with no bounds check.
	if types.IsTuple(g.info.Types[n.X]) {
		return g.genTupleIndex(n)
	}
	// Array: bounds-checked element read (existing path unchanged).
	id := g.genExpr(n.X)
	idx := g.genExpr(n.Index)
	idxTemp := g.spillToTemp(idx)
	lenTemp := g.arrayLen(id.name)
	g.emitBoundsCheck(idxTemp, lenTemp, n.LBrkPos)
	// A bounds fault set pending (mode-aware __wisp_fail); skip the rest of the
	// statement so a consumer (e.g. print) does not run on the faulted read (M5).
	g.guardAfterSpill()
	return g.readHandleVar(g.arrayElemNameDyn(id.name, idxTemp))
}

// genIndexAssign lowers `target[i] = value`. Array: same up-front bounds check.
// Dict: encode the key, set the entry var, and append the token to the
// insertion-order key list only if the key is new (overwrite keeps its position).
func (g *gen) genIndexAssign(n *ast.IndexAssignStmt) {
	if types.IsDict(g.info.Types[n.Target]) {
		g.genDictSet(n)
		return
	}
	id := g.genExpr(n.Target)
	idx := g.genExpr(n.Index)
	v := g.genExpr(n.Value)
	idxTemp := g.spillToTemp(idx)
	lenTemp := g.arrayLen(id.name)
	g.emitBoundsCheck(idxTemp, lenTemp, n.LBrkPos)
	// Skip the write when the bounds check faulted (M5 fail-at-first-fault).
	g.guardAfterSpill()
	g.setHandleVar(g.arrayElemNameDyn(id.name, idxTemp), v)
}

// arrayLen reads __wisp_a_<id>_len into a fresh temp and returns its name.
func (g *gen) arrayLen(idTemp string) string {
	g.line("eval \"__ret=%s\"", "\\$__wisp_a_${"+idTemp+"}_len")
	t := g.newTemp()
	g.line("%s=\"$__ret\"", t)
	return t
}

// emitBoundsCheck aborts located when idxTemp is negative or >= lenTemp. idxTemp
// is int-valid ([+-]?[0-9]+), so the negative test is a `case` glob (no
// arithmetic on the candidate) and the upper-bound test is a numeric `[ ]`
// against the trusted length, both BEFORE any backing name is built (spec 4.3).
func (g *gen) emitBoundsCheck(idxTemp, lenTemp string, pos SourcePos) {
	g.use(runtime.BoundsFail)
	g.line("case \"$%s\" in -*) __wisp_bounds_fail %s \"$%s\" \"$%s\" ;; esac", idxTemp, g.posLiteral(pos), idxTemp, lenTemp)
	g.line("if [ \"$%s\" -ge \"$%s\" ]; then __wisp_bounds_fail %s \"$%s\" \"$%s\"; fi", idxTemp, lenTemp, g.posLiteral(pos), idxTemp, lenTemp)
}

// genArrayLength lowers length(xs) for an array argument: read its _len var.
func (g *gen) genArrayLength(arg ast.Expr) atom {
	id := g.genExpr(arg)
	return varAtom(g.arrayLen(id.name))
}

// genPush lowers push(xs, v): set element [len] then bump _len. len is read
// once, the element written at that index, and the length incremented.
func (g *gen) genPush(args []ast.Expr) atom {
	id := g.genExpr(args[0])
	v := g.genExpr(args[1])
	lenTemp := g.arrayLen(id.name)
	g.setHandleVar(g.arrayElemNameDyn(id.name, lenTemp), v)
	newLen := g.newTemp()
	g.line("%s=$(( $%s + 1 ))", newLen, lenTemp)
	g.line("eval \"__wisp_a_${%s}_len=\\$%s\"", id.name, newLen)
	return litAtom("''")
}

// --- higher-order builtins map/filter/each (M4) ---
//
// All three lower to a counted loop 0..len-1 over the source array (reusing the
// array machinery above) that indirect-calls the function reference f for each
// element. f is spilled to a stable temp before the loop; the element is read
// each iteration into a temp via the dynamic backing-name read. map pushes
// f(x) into a fresh result U[]; filter pushes x when f(x) is true; each calls
// f(x) and does NOT read __ret (the callee is void). An empty source array makes
// the loop body run zero times: map/filter return a fresh empty array, each is a
// no-op.

// genMap lowers map(xs, f) -> U[]: a fresh result array of f(x) for each x.
func (g *gen) genMap(args []ast.Expr) atom {
	id := g.genExpr(args[0])
	f := g.spillToTemp(g.genExpr(args[1]))
	out := g.allocHandle()
	outLen := g.newTemp()
	g.line("%s=0", outLen)
	idxTemp, _ := g.beginArrayLoop(id.name)
	elem := g.readHandleVar(g.arrayElemNameDyn(id.name, idxTemp))
	g.line("\"$%s\" %s", f, g.word(elem))
	res := g.newTemp()
	g.line("%s=\"$__ret\"", res)
	g.setHandleVar(g.arrayElemNameDyn(out, outLen), varAtom(res))
	g.line("%s=$(( $%s + 1 ))", outLen, outLen)
	g.endArrayLoop(idxTemp)
	g.line("eval \"__wisp_a_${%s}_len=\\$%s\"", out, outLen)
	return varAtom(out)
}

// genFilter lowers filter(xs, f) -> T[]: a fresh array of the x where f(x) is true.
func (g *gen) genFilter(args []ast.Expr) atom {
	id := g.genExpr(args[0])
	f := g.spillToTemp(g.genExpr(args[1]))
	out := g.allocHandle()
	outLen := g.newTemp()
	g.line("%s=0", outLen)
	idxTemp, _ := g.beginArrayLoop(id.name)
	elemTemp := g.spillToTemp(g.readHandleVar(g.arrayElemNameDyn(id.name, idxTemp)))
	g.line("\"$%s\" \"$%s\"", f, elemTemp)
	g.line("if [ \"$__ret\" = true ]; then")
	g.indent++
	g.setHandleVar(g.arrayElemNameDyn(out, outLen), varAtom(elemTemp))
	g.line("%s=$(( $%s + 1 ))", outLen, outLen)
	g.indent--
	g.line("fi")
	g.endArrayLoop(idxTemp)
	g.line("eval \"__wisp_a_${%s}_len=\\$%s\"", out, outLen)
	return varAtom(out)
}

// genEach lowers each(xs, f) -> void: call f(x) for each x; __ret is not read
// (the callee returns void).
func (g *gen) genEach(args []ast.Expr) atom {
	id := g.genExpr(args[0])
	f := g.spillToTemp(g.genExpr(args[1]))
	idxTemp, _ := g.beginArrayLoop(id.name)
	elem := g.readHandleVar(g.arrayElemNameDyn(id.name, idxTemp))
	g.line("\"$%s\" %s", f, g.word(elem))
	g.endArrayLoop(idxTemp)
	return litAtom("''")
}

// --- core stdlib array builtins (M6 PR-A) ---

// genSplit lowers split(s, sep) -> string[]: allocate a fresh array handle and
// let __wisp_split fill its element/_len backing vars (the scan + literal match
// live in the prelude). The call site position is forwarded so an empty
// separator aborts located.
func (g *gen) genSplit(n *ast.CallExpr, args []ast.Expr) atom {
	s := g.genExpr(args[0])
	sep := g.genExpr(args[1])
	id := g.allocHandle()
	g.use(runtime.Split)
	g.line("__wisp_split %s \"$%s\" %s %s", g.posLiteral(n.Pos()), id, g.word(s), g.word(sep))
	// An empty-separator fault set pending (mode-aware __wisp_fail); skip the rest
	// of the statement so a consumer does not run on the faulted handle (M5).
	g.guardAfterSpill()
	return varAtom(id)
}

// genListDir lowers list_dir(path) -> string[]: alloc a fresh array handle and
// let __wisp_list_dir fill its element/_len backing vars from a quoted glob loop
// (the loop + base-name strip + ./.. filter + existence guard live in the
// prelude). The call-site position is forwarded so a missing/non-dir path aborts
// located.
func (g *gen) genListDir(n *ast.CallExpr, args []ast.Expr) atom {
	path := g.genExpr(args[0])
	id := g.allocHandle()
	g.use(runtime.ListDir)
	g.line("__wisp_list_dir %s \"$%s\" %s", g.posLiteral(n.Pos()), id, g.word(path))
	g.guardAfterSpill()
	return varAtom(id)
}

// genGlob lowers glob(pattern) -> string[]: alloc a fresh array handle and let
// __wisp_glob fill its element/_len backing vars from an UNQUOTED for-in over the
// pattern (shell pathname expansion; the unmatched-literal case is dropped by the
// helper's existence guard, the matched names eval-stored inert -- the list_dir
// mechanism, but over a user pattern). The call-site position is forwarded for
// call-shape parity with __wisp_list_dir; glob is TOTAL (no located abort), so
// guardAfterSpill never fires here, but it is emitted for structural symmetry
// with the other array-from-shell builtins.
func (g *gen) genGlob(n *ast.CallExpr, args []ast.Expr) atom {
	pattern := g.genExpr(args[0])
	id := g.allocHandle()
	g.use(runtime.Glob)
	g.line("__wisp_glob %s \"$%s\" %s", g.posLiteral(n.Pos()), id, g.word(pattern))
	g.guardAfterSpill()
	return varAtom(id)
}

// genRegexFindAll lowers regex_find_all(s, pattern) -> string[]: alloc a fresh
// array handle and let __wisp_regex_find_all fill its element/_len backing vars
// (the awk match loop + the mandatory zero-width-safe advance + the shell array
// fill live in the prelude). The call-site position is forwarded so a malformed
// pattern aborts located (leaving _len=0). s and pattern are evaluated in source
// order before the alloc, like genSplit.
func (g *gen) genRegexFindAll(n *ast.CallExpr, args []ast.Expr) atom {
	s := g.genExpr(args[0])
	pat := g.genExpr(args[1])
	id := g.allocHandle()
	g.use(runtime.RegexFindAll)
	g.line("__wisp_regex_find_all %s \"$%s\" %s %s", g.posLiteral(n.Pos()), id, g.word(s), g.word(pat))
	g.guardAfterSpill()
	return varAtom(id)
}

// genJoin lowers join(parts, sep) -> string: read the array id and let
// __wisp_join concatenate its elements with sep into __ret.
func (g *gen) genJoin(args []ast.Expr) atom {
	id := g.spillToTemp(g.genExpr(args[0]))
	sep := g.genExpr(args[1])
	g.use(runtime.Join)
	g.line("__wisp_join \"$%s\" %s", id, g.word(sep))
	t := g.newTemp()
	g.line("%s=\"$__ret\"", t)
	return varAtom(t)
}

// genArrayContains lowers contains(xs, x) -> bool for an array: a counted loop
// comparing each element to the target by its text representation (sound for the
// comparable element types int/bool/string the checker admits). Short-circuits
// to true on the first match. An empty array yields false.
func (g *gen) genArrayContains(args []ast.Expr) atom {
	id := g.spillToTemp(g.genExpr(args[0]))
	target := g.spillToTemp(g.genExpr(args[1]))
	return g.genArrayContainsAtoms(varAtom(id), varAtom(target))
}

// genReverse lowers reverse(xs) -> T[]: a fresh array with the elements of xs in
// reverse order. out[len-1-i] = xs[i] for each i (any element type; values are
// copied, never inspected).
func (g *gen) genReverse(args []ast.Expr) atom {
	id := g.genExpr(args[0])
	out := g.allocHandle()
	idxTemp, lenTemp := g.beginArrayLoop(id.name)
	elem := g.readHandleVar(g.arrayElemNameDyn(id.name, idxTemp))
	dst := g.newTemp()
	// dst = len - 1 - idx
	g.line("%s=$(( $%s - 1 - $%s ))", dst, lenTemp, idxTemp)
	g.setHandleVar(g.arrayElemNameDyn(out, dst), elem)
	g.endArrayLoop(idxTemp)
	g.line("eval \"__wisp_a_${%s}_len=\\$%s\"", out, lenTemp)
	return varAtom(out)
}

// genReduce lowers reduce(xs, init, f) -> U: a left fold. The accumulator starts
// at init and is replaced by f(acc, elem) for each element in order (reusing the
// array loop + indirect-call machinery, like map). The accumulator atom is read
// back from __ret each iteration.
func (g *gen) genReduce(args []ast.Expr) atom {
	id := g.genExpr(args[0])
	acc := g.spillToTemp(g.genExpr(args[1]))
	f := g.spillToTemp(g.genExpr(args[2]))
	idxTemp, _ := g.beginArrayLoop(id.name)
	elem := g.readHandleVar(g.arrayElemNameDyn(id.name, idxTemp))
	g.line("\"$%s\" \"$%s\" %s", f, acc, g.word(elem))
	g.line("%s=\"$__ret\"", acc)
	g.endArrayLoop(idxTemp)
	return varAtom(acc)
}

// beginArrayLoop opens a counted `while` loop 0..len-1 over the array whose id
// is in idTemp, returning the index temp (held at the current iteration) and the
// length temp. The caller emits the body, then calls endArrayLoop. The loop body
// runs zero times for an empty array.
func (g *gen) beginArrayLoop(idTemp string) (idxTemp, lenTemp string) {
	lenTemp = g.arrayLen(idTemp)
	idxTemp = g.newTemp()
	g.line("%s=0", idxTemp)
	g.line("while [ \"$%s\" -lt \"$%s\" ]; do", idxTemp, lenTemp)
	g.indent++
	g.shellDepth++
	g.loopPendingBreak()
	return idxTemp, lenTemp
}

// endArrayLoop bumps the index and closes a loop opened by beginArrayLoop.
func (g *gen) endArrayLoop(idxTemp string) {
	g.line("%s=$(( $%s + 1 ))", idxTemp, idxTemp)
	g.shellDepth--
	g.indent--
	g.line("done")
}

// --- for-in over arrays ---

// genForIn lowers `for (x in xs) { body }` to a counted loop 0..len-1 binding x
// (block-scoped, M1 rule 11). The element variable is read each iteration into
// the loop var's mangled name. Break/continue map like the C-style for (the body
// runs inside a once-wrapper nested in a real while loop).
func (g *gen) genForIn(n *ast.ForInStmt) {
	if types.IsDict(g.info.Types[n.Coll]) {
		g.genDictForIn(n)
		return
	}
	g.pushScope()
	v := g.info.ForInVars[n]
	if v != nil {
		g.declareVar(v)
	}

	id := g.genExpr(n.Coll)
	lenTemp := g.arrayLen(id.name)
	idxTemp := g.newTemp()
	g.line("%s=0", idxTemp)
	g.line("while :; do")
	g.indent++
	g.shellDepth++
	g.loopPendingBreak()
	g.line("if [ \"$%s\" -ge \"$%s\" ]; then break; fi", idxTemp, lenTemp)
	// bind the element: x = xs[idx] (skipped for a blank loop variable)
	if v != nil {
		g.line("eval \"%s=%s\"", v.Mangled, "\\$"+g.arrayElemNameDyn(id.name, idxTemp))
	}
	g.onceCount++
	g.line("for __wisp_once%d in 0; do", g.onceCount)
	g.indent++
	g.shellDepth++
	g.loops = append(g.loops, loopCtx{isFor: true})
	g.genBlock(n.Body)
	g.loops = g.loops[:len(g.loops)-1]
	g.shellDepth--
	g.indent--
	g.line("done")
	g.line("%s=$(( $%s + 1 ))", idxTemp, idxTemp)
	g.shellDepth--
	g.indent--
	g.line("done")
	g.popScope()
}

// --- helpers ---

// spillToTemp ensures a is held in a shell variable and returns its name. A
// variable atom is returned as-is; a literal is copied into a fresh temp so it
// can be expanded as "$name" by the bounds check and name builders.
func (g *gen) spillToTemp(a atom) string {
	if !a.lit {
		return a.name
	}
	t := g.newTemp()
	g.line("%s=%s", t, a.name)
	return t
}
