package codegen

import (
	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/runtime"
	"github.com/mitchellnemitz/wisp/internal/token"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// Dict lowering (M3 PR-C). A dict is a reference handle (spec 4.1) like array/
// struct: an id from __wisp_alloc. Its entries live in __wisp_d_<id>_<token>
// vars and an insertion-ordered list of tokens in __wisp_d_<id>_keys, where
// <token> is the injection-safe, reversible "k<hex>" encoding of the runtime key
// bytes (__wisp_dkey_enc). The encoding is ALWAYS computed (and, for int keys,
// the value canonicalized through __wisp_int) BEFORE the backing variable name
// is built, so the constructed name is a safe identifier and numerically-equal
// int keys collapse to one entry (spec 4.1 abort-before-name ordering). Set/get/
// has/append/iterate are emitted inline here (no generic helper layer, matching
// the PR-B struct/array style).

// dictKeysName returns the shell text for a dict's insertion-order key-list var,
// `__wisp_d_${idTemp}_keys`. The "keys" suffix lives in the same namespace as a
// "k<hex>" entry token only if some key encodes to exactly "eys"-prefixed... it
// cannot: every entry token starts with "k" then hex, and "keys" is not of that
// shape, so the list var never collides with an entry var.
func (g *gen) dictKeysName(idTemp string) string {
	return "__wisp_d_${" + idTemp + "}_keys"
}

// dictEntryName returns the shell text for a dict entry var given the id temp and
// the token temp (holding "k<hex>"): `__wisp_d_${idTemp}_${tokTemp}`.
func (g *gen) dictEntryName(idTemp, tokTemp string) string {
	return "__wisp_d_${" + idTemp + "}_${" + tokTemp + "}"
}

// encodeKey lowers a dict key expression to a fresh temp holding its "k<hex>"
// token. For an int-keyed dict the value is first canonicalized through the
// existing __wisp_int validate-and-abort (so 5, 05, +5 collapse to one token and
// the int-validity invariant is reasserted at use); for a string-keyed dict the
// raw bytes are encoded directly. keyType is the dict's key type K.
func (g *gen) encodeKey(keyType types.Type, key atom, pos token.Position) string {
	src := key
	if keyType == types.Int {
		g.use(runtime.Int)
		g.line("__wisp_int %s %s", g.posLiteral(pos), g.word(key))
		nt := g.newTemp()
		g.line("%s=\"$__ret\"", nt)
		src = varAtom(nt)
	}
	g.use(runtime.DictEnc)
	g.line("__wisp_dkey_enc %s", g.word(src))
	tok := g.newTemp()
	g.line("%s=\"$__ret\"", tok)
	return tok
}

// emitDictAppendIfNew appends tokTemp to the dict's key list only when it is not
// already present, preserving insertion order on overwrite (spec 4.4). The
// membership test is a `case` over the space-bounded list with the token matched
// LITERALLY (quoted inside the pattern); a token is [0-9a-fk]* so it carries no
// glob metacharacters regardless.
func (g *gen) emitDictAppendIfNew(idTemp, tokTemp string) {
	keysRead := g.readHandleVar(g.dictKeysName(idTemp)) // current list -> temp
	g.line("case \" $%s \" in", keysRead.name)
	g.indent++
	g.line("*\" $%s \"*) : ;;", tokTemp)
	g.line("*) eval \"%s=\\\"\\$%s \\$%s\\\"\" ;;", g.dictKeysName(idTemp), keysRead.name, tokTemp)
	g.indent--
	g.line("esac")
}

// genDictLit lowers `{ k: v, ... }`: allocate a handle, init the key list empty,
// then for each entry (source order) encode the key, set the entry var, and
// append the token if new. A runtime-duplicate key (only possible with non-
// constant keys; constant dups are a compile error) overwrites the value and
// keeps the first occurrence's position, matching d[k]=v semantics.
func (g *gen) genDictLit(n *ast.DictLit) atom {
	dt := g.info.Types[n]
	keyType := types.DictKeyType(dt)

	id := g.allocHandle()
	g.line("eval \"%s=''\"", g.dictKeysName(id))
	for _, e := range n.Entries {
		key := g.genExpr(e.Key)
		val := g.genExpr(e.Value)
		tok := g.encodeKey(keyType, key, e.Colon)
		g.setHandleVar(g.dictEntryName(id, tok), val)
		g.emitDictAppendIfNew(id, tok)
	}
	return varAtom(id)
}

// genDictLookup lowers `d[k]` (a dict read): encode the key, abort located via
// __wisp_dict_miss when the key is absent (spec 4.4), else read the entry var.
// The original key text is preserved for the abort message.
func (g *gen) genDictLookup(n *ast.IndexExpr) atom {
	dt := g.info.Types[n.X]
	keyType := types.DictKeyType(dt)

	id := g.genExpr(n.X)
	key := g.genExpr(n.Index)
	keyWord := g.spillToTemp(key) // keep the original key text for the abort message
	tok := g.encodeKey(keyType, varAtom(keyWord), n.LBrkPos)
	keysRead := g.readHandleVar(g.dictKeysName(id.name))
	g.use(runtime.DictMiss)
	g.line("case \" $%s \" in", keysRead.name)
	g.indent++
	g.line("*\" $%s \"*) : ;;", tok)
	g.line("*) __wisp_dict_miss %s \"$%s\" ;;", g.posLiteral(n.LBrkPos), keyWord)
	g.indent--
	g.line("esac")
	// A missing-key fault set pending; skip the rest of the statement so the
	// consumer does not run on the faulted read (M5).
	g.guardAfterSpill()
	return g.readHandleVar(g.dictEntryName(id.name, tok))
}

// genDictSet lowers `d[k] = v`: encode the key, set the entry var, append the
// token if new (overwrite keeps insertion position).
func (g *gen) genDictSet(n *ast.IndexAssignStmt) {
	dt := g.info.Types[n.Target]
	keyType := types.DictKeyType(dt)

	id := g.genExpr(n.Target)
	key := g.genExpr(n.Index)
	val := g.genExpr(n.Value)
	tok := g.encodeKey(keyType, key, n.LBrkPos)
	g.setHandleVar(g.dictEntryName(id.name, tok), val)
	g.emitDictAppendIfNew(id.name, tok)
}

// genHas lowers `has(d, k) -> bool`: encode the key and test membership in the
// key list, capturing true/false.
func (g *gen) genHas(n *ast.CallExpr, args []ast.Expr) atom {
	dt := g.info.Types[args[0]]
	keyType := types.DictKeyType(dt)

	id := g.genExpr(args[0])
	key := g.genExpr(args[1])
	tok := g.encodeKey(keyType, key, n.Pos())
	keysRead := g.readHandleVar(g.dictKeysName(id.name))
	t := g.newTemp()
	g.line("case \" $%s \" in", keysRead.name)
	g.indent++
	g.line("*\" $%s \"*) %s=true ;;", tok, t)
	g.line("*) %s=false ;;", t)
	g.indent--
	g.line("esac")
	return varAtom(t)
}

// genKeys lowers `keys(d) -> K[]`: allocate a fresh array handle and, walking the
// dict's insertion-ordered token list, decode each token back to its key bytes
// (and, for an int-keyed dict, re-run the existing __wisp_int validate-and-abort
// per spec 4.1 -- not trusting the stored validity), appending each to the array.
func (g *gen) genKeys(args []ast.Expr) atom {
	dt := g.info.Types[args[0]]
	keyType := types.DictKeyType(dt)

	id := g.genExpr(args[0])
	keysRead := g.readHandleVar(g.dictKeysName(id.name))

	arr := g.allocHandle()
	idxTemp := g.newTemp()
	g.line("%s=0", idxTemp)
	// `for tok in $list`: tokens are [0-9a-fk]* (no spaces/globs), space-separated,
	// so word-splitting yields exactly the insertion-ordered tokens.
	tokVar := g.newTemp()
	g.line("for %s in $%s; do", tokVar, keysRead.name)
	g.indent++
	g.shellDepth++
	g.loopPendingBreak()
	g.use(runtime.DictDec)
	g.line("__wisp_dkey_dec \"$%s\"", tokVar)
	decoded := g.newTemp()
	g.line("%s=\"$__ret\"", decoded)
	if keyType == types.Int {
		g.use(runtime.Int)
		g.line("__wisp_int %s \"$%s\"", g.posLiteral(args[0].Pos()), decoded)
		g.line("%s=\"$__ret\"", decoded)
	}
	g.setHandleVar(g.arrayElemNameDyn(arr, idxTemp), varAtom(decoded))
	g.line("%s=$(( $%s + 1 ))", idxTemp, idxTemp)
	g.shellDepth--
	g.indent--
	g.line("done")
	g.line("eval \"__wisp_a_${%s}_len=\\$%s\"", arr, idxTemp)
	return varAtom(arr)
}

// genDictForIn lowers `for (k in d) { body }`: iterate the insertion-ordered
// token list, decode each token into the (block-scoped, M1 rule 11) loop key var
// -- re-running __wisp_int for an int-keyed dict (spec 4.1) -- and run the body.
// Break/continue map like the array for-in (a once-wrapper nested in a while).
func (g *gen) genDictForIn(n *ast.ForInStmt) {
	dt := g.info.Types[n.Coll]
	keyType := types.DictKeyType(dt)

	g.pushScope()
	v := g.info.ForInVars[n]
	if v != nil {
		g.declareVar(v)
	}

	id := g.genExpr(n.Coll)
	keysRead := g.readHandleVar(g.dictKeysName(id.name))
	tokVar := g.newTemp()
	g.line("for %s in $%s; do", tokVar, keysRead.name)
	g.indent++
	g.shellDepth++
	g.loopPendingBreak()
	if v != nil {
		g.use(runtime.DictDec)
		g.line("__wisp_dkey_dec \"$%s\"", tokVar)
		g.line("%s=\"$__ret\"", v.Mangled)
		if keyType == types.Int {
			g.use(runtime.Int)
			g.line("__wisp_int %s \"$%s\"", g.posLiteral(n.VarPos), v.Mangled)
			g.line("%s=\"$__ret\"", v.Mangled)
		}
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
	g.shellDepth--
	g.indent--
	g.line("done")
	g.popScope()
}
