package codegen

import (
	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// Collections-core dict builtins (values/get/remove/merge), reusing the M3
// dict backing-variable and insertion-order key-list machinery.

// genValues lowers values(d) -> V[]: the values in insertion order, parallel to
// keys(). Walk the insertion-ordered token list and read each entry's value.
func (g *gen) genValues(args []ast.Expr) atom {
	id := g.genExpr(args[0])
	keysRead := g.readHandleVar(g.dictKeysName(id.name))
	arr := g.allocHandle()
	idxTemp := g.newTemp()
	g.line("%s=0", idxTemp)
	tokVar := g.newTemp()
	g.line("for %s in $%s; do", tokVar, keysRead.name)
	g.indent++
	g.shellDepth++
	g.loopPendingBreak()
	val := g.readHandleVar(g.dictEntryName(id.name, tokVar))
	g.setHandleVar(g.arrayElemNameDyn(arr, idxTemp), val)
	g.line("%s=$(( $%s + 1 ))", idxTemp, idxTemp)
	g.shellDepth--
	g.indent--
	g.line("done")
	g.line("eval \"__wisp_a_${%s}_len=\\$%s\"", arr, idxTemp)
	return varAtom(arr)
}

// genGet lowers get(d, k) -> Optional[V]: Some(d[k]) when k is present, else None.
func (g *gen) genGet(args []ast.Expr) atom {
	keyType := types.DictKeyType(g.info.Types[args[0]])
	id := g.genExpr(args[0])
	key := g.genExpr(args[1])
	tok := g.encodeKey(keyType, key, args[1].Pos())
	keysRead := g.readHandleVar(g.dictKeysName(id.name))
	out := g.allocHandle()
	g.line("case \" $%s \" in", keysRead.name)
	g.indent++
	g.line("*\" $%s \"*)", tok)
	g.indent++
	val := g.readHandleVar(g.dictEntryName(id.name, tok))
	g.setHandleVar(tagFieldName(out), litAtom("some"))
	g.setHandleVar(tagValueName(out), val)
	g.line(";;")
	g.indent--
	g.line("*)")
	g.indent++
	g.setHandleVar(tagFieldName(out), litAtom("none"))
	g.line(";;")
	g.indent--
	g.indent--
	g.line("esac")
	return varAtom(out)
}

// genRemove lowers remove(d, k) -> void: delete k in place. Rebuild the key list
// excluding the token (reproducing the exact leading-space canonical form: start
// empty and append " $t" per survivor), then unset the entry var. Absent key is a
// no-op.
func (g *gen) genRemove(args []ast.Expr) atom {
	keyType := types.DictKeyType(g.info.Types[args[0]])
	id := g.genExpr(args[0])
	key := g.genExpr(args[1])
	tok := g.encodeKey(keyType, key, args[1].Pos())
	keysRead := g.readHandleVar(g.dictKeysName(id.name))
	acc := g.newTemp()
	g.line("%s=''", acc)
	tokVar := g.newTemp()
	g.line("for %s in $%s; do", tokVar, keysRead.name)
	g.indent++
	g.shellDepth++
	g.line("if [ \"$%s\" != \"$%s\" ]; then %s=\"$%s $%s\"; fi", tokVar, tok, acc, acc, tokVar)
	g.shellDepth--
	g.indent--
	g.line("done")
	g.line("eval \"%s=\\$%s\"", g.dictKeysName(id.name), acc)
	g.line("eval \"unset %s\"", g.dictEntryName(id.name, tok))
	return litAtom("''")
}

// genMerge lowers merge(a, b) -> {K:V}: a fresh dict with a's entries then b's,
// b winning on a shared key (overwrite value, keep a's position). Tokens are
// directly comparable across dicts of the same {K:V}, so no re-encoding.
func (g *gen) genMerge(args []ast.Expr) atom {
	a := g.spillToTemp(g.genExpr(args[0]))
	b := g.spillToTemp(g.genExpr(args[1]))
	out := g.allocHandle()
	g.line("eval \"%s=''\"", g.dictKeysName(out))
	g.mergeCopy(a, out)
	g.mergeCopy(b, out)
	return varAtom(out)
}

// genSize lowers size(d) -> int: count of keys in the insertion-order list.
// Empty dict yields 0. No abort, no mutation.
func (g *gen) genSize(args []ast.Expr) atom {
	id := g.genExpr(args[0])
	keysRead := g.readHandleVar(g.dictKeysName(id.name))
	n := g.newTemp()
	g.line("%s=0", n)
	tokVar := g.newTemp()
	g.line("for %s in $%s; do %s=$(( $%s + 1 )); done", tokVar, keysRead.name, n, n)
	return varAtom(n)
}

// genClear lowers clear(d) -> void: unset every entry var and reset the key list
// to ”. Injection-safe: key-list tokens are encodeKey-encoded (alphanumeric only).
func (g *gen) genClear(args []ast.Expr) atom {
	id := g.genExpr(args[0])
	keysRead := g.readHandleVar(g.dictKeysName(id.name))
	tokVar := g.newTemp()
	g.line("for %s in $%s; do eval \"unset %s\"; done", tokVar, keysRead.name, g.dictEntryName(id.name, tokVar))
	g.line("eval \"%s=''\"", g.dictKeysName(id.name))
	return litAtom("''")
}

// mergeCopy copies every entry of srcID into out (set value, append token if new),
// preserving srcID's insertion order for tokens not already in out.
func (g *gen) mergeCopy(srcID, out string) {
	keysRead := g.readHandleVar(g.dictKeysName(srcID))
	tokVar := g.newTemp()
	g.line("for %s in $%s; do", tokVar, keysRead.name)
	g.indent++
	g.shellDepth++
	val := g.readHandleVar(g.dictEntryName(srcID, tokVar))
	g.setHandleVar(g.dictEntryName(out, tokVar), val)
	g.emitDictAppendIfNew(out, tokVar)
	g.shellDepth--
	g.indent--
	g.line("done")
}
