package codegen

import (
	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/runtime"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// JSON core-module lowering (Unit 5). A json.Value is a reference handle like the
// other aggregates: an id from __wisp_alloc whose canonical JSON text lives in a
// single backing var __wisp_j_<id> (spec 4.1). Codegen dispatches purely on
// CallInfo.Builtin ("json_*"), so the namespaced spelling needs no awareness
// here; decode[T] additionally reads CallInfo.Result to pick the boxing vs
// scalar variant. Every runtime value flows through the deferred \$temp store of
// setHandleVar / readHandleVar, so the lowering is injection-safe by
// construction (spec 9.6): no json text is ever re-parsed by the shell.

// jsonTextName returns the shell text for a json.Value's canonical-text backing
// var, `__wisp_j_${idTemp}`.
func jsonTextName(idTemp string) string { return "__wisp_j_${" + idTemp + "}" }

// jsonBox allocates a fresh json.Value handle and stores canonical as its text.
// The canonical value is copied into a fresh temp with an explicit assignment
// BEFORE __wisp_alloc (which writes __ret): a bare spillToTemp would NOT copy a
// variable atom, so a canonical of "$__ret" (from a wrapper) would be clobbered
// by the alloc's __ret write.
func (g *gen) jsonBox(canonical atom) atom {
	ct := g.newTemp()
	g.line("%s=%s", ct, g.word(canonical))
	id := g.allocHandle()
	g.setHandleVar(jsonTextName(id), varAtom(ct))
	return varAtom(id)
}

// genJSONEncode lowers json.encode(v) -> string: the canonical text IS the
// stored value, so this is a bare handle-var read (no awk, no engine dep).
func (g *gen) genJSONEncode(args []ast.Expr) atom {
	v := g.genExpr(args[0])
	return g.readHandleVar(jsonTextName(v.name))
}

// genJSONNull lowers json.null() -> json.Value: a handle whose text is "null".
func (g *gen) genJSONNull() atom {
	return g.jsonBox(litAtom("null"))
}

// genJSONFromIdentity lowers from_int/from_bool/from_float: the wisp int/bool/
// float text IS already the canonical JSON number/bool text (float is the
// exponent-free decimal of the float-validity invariant), so box it directly.
func (g *gen) genJSONFromIdentity(args []ast.Expr) atom {
	return g.jsonBox(g.genExpr(args[0]))
}

// genJSONFromString lowers from_string(s): escape s to a JSON string literal via
// the engine, then box it.
func (g *gen) genJSONFromString(args []ast.Expr) atom {
	s := g.genExpr(args[0])
	g.use(runtime.JSONEscape)
	g.line("__wisp_json_escape %s", g.word(s))
	return g.jsonBox(varAtom("__ret"))
}

// genJSONArray lowers json.array(elems) -> json.Value: "[" + child canonical
// texts joined by "," + "]". Each element is itself a json.Value handle whose
// canonical text is read from its backing var.
func (g *gen) genJSONArray(args []ast.Expr) atom {
	id := g.genExpr(args[0])
	acc := g.newTemp()
	g.line("%s='['", acc)
	first := g.newTemp()
	g.line("%s=1", first)
	idxTemp, _ := g.beginArrayLoop(id.name)
	child := g.spillToTemp(g.readHandleVar(g.arrayElemNameDyn(id.name, idxTemp)))
	canon := g.readHandleVar(jsonTextName(child))
	g.line("if [ \"$%s\" -eq 1 ]; then %s=0; else %s=\"$%s,\"; fi", first, first, acc, acc)
	g.line("%s=\"$%s$%s\"", acc, acc, canon.name)
	g.endArrayLoop(idxTemp)
	g.line("%s=\"$%s]\"", acc, acc)
	return g.jsonBox(varAtom(acc))
}

// genJSONObject lowers json.object(entries) -> json.Value: "{" + `"key":value`
// pairs joined by "," + "}", in the dict's insertion order. Each key is escaped
// to a JSON string literal (engine escape op) and each value is a json.Value
// handle whose canonical text is read from its backing var.
func (g *gen) genJSONObject(args []ast.Expr) atom {
	id := g.genExpr(args[0])
	keysRead := g.readHandleVar(g.dictKeysName(id.name))
	acc := g.newTemp()
	g.line("%s='{'", acc)
	first := g.newTemp()
	g.line("%s=1", first)
	tokVar := g.newTemp()
	g.line("for %s in $%s; do", tokVar, keysRead.name)
	g.indent++
	g.shellDepth++
	g.use(runtime.DictDec)
	g.line("__wisp_dkey_dec \"$%s\"", tokVar)
	key := g.newTemp()
	g.line("%s=\"$__ret\"", key)
	g.use(runtime.JSONEscape)
	g.line("__wisp_json_escape \"$%s\"", key)
	esc := g.newTemp()
	g.line("%s=\"$__ret\"", esc)
	child := g.spillToTemp(g.readHandleVar(g.dictEntryName(id.name, tokVar)))
	canon := g.readHandleVar(jsonTextName(child))
	g.line("if [ \"$%s\" -eq 1 ]; then %s=0; else %s=\"$%s,\"; fi", first, first, acc, acc)
	g.line("%s=\"$%s$%s:$%s\"", acc, acc, esc, canon.name)
	g.shellDepth--
	g.indent--
	g.line("done")
	g.line("%s=\"$%s}\"", acc, acc)
	return g.jsonBox(varAtom(acc))
}

// genJSONTypeOf lowers json.type_of(v) -> string: read the canonical text, hand
// it to the (total) type wrapper, capture __ret.
func (g *gen) genJSONTypeOf(args []ast.Expr) atom {
	v := g.genExpr(args[0])
	canon := g.readHandleVar(jsonTextName(v.name))
	g.use(runtime.JSONTypeOf)
	g.line("__wisp_json_type_of %s", g.word(canon))
	t := g.newTemp()
	g.line("%s=\"$__ret\"", t)
	return varAtom(t)
}

// genJSONScalarAccessor lowers as_string/as_int/as_float/as_bool: read the
// canonical text of the json.Value and pass it (with the located <pos>) to a
// fallible wrapper that aborts on a type mismatch. Mirrors genLocatedHelperCall
// but the sole argument is the handle's canonical text, not the raw AST arg.
func (g *gen) genJSONScalarAccessor(id, fn string, n *ast.CallExpr, args []ast.Expr) atom {
	v := g.genExpr(args[0])
	canon := g.readHandleVar(jsonTextName(v.name))
	pos := g.posLiteral(n.Pos())
	g.use(id)
	g.line("%s %s %s", fn, pos, g.word(canon))
	t := g.newTemp()
	g.line("%s=\"$__ret\"", t)
	g.guardAfterSpill()
	return varAtom(t)
}

// genJSONGetAt lowers get(v, key) / at(v, i) -> Optional[json.Value]. The wrapper
// returns 0 (absent) or 1<canonical-slice> in __ret; a 1 boxes the slice into a
// fresh json.Value handle wrapped in Some, a 0 yields None. sel is the arg-1
// expression (the key or index).
func (g *gen) genJSONGetAt(id, fn string, n *ast.CallExpr, args []ast.Expr) atom {
	v := g.genExpr(args[0])
	canon := g.spillToTemp(g.readHandleVar(jsonTextName(v.name)))
	sel := g.genExpr(args[1])
	pos := g.posLiteral(n.Pos())
	g.use(id)
	g.line("%s %s \"$%s\" %s", fn, pos, canon, g.word(sel))
	tok := g.newTemp()
	g.line("%s=\"$__ret\"", tok)
	g.guardAfterSpill()
	out := g.allocHandle()
	g.line("case \"$%s\" in", tok)
	g.indent++
	g.line("1*)")
	g.indent++
	slice := g.newTemp()
	g.line("%s=\"${%s#1}\"", slice, tok)
	jv := g.jsonBox(varAtom(slice))
	g.setHandleVar(tagFieldName(out), litAtom("some"))
	g.setHandleVar(tagValueName(out), jv)
	g.indent--
	g.line(";;")
	g.line("*)")
	g.indent++
	g.setHandleVar(tagFieldName(out), litAtom("none"))
	g.indent--
	g.line(";;")
	g.indent--
	g.line("esac")
	return varAtom(out)
}

// genJSONDecode lowers json.decode[T](s). CallInfo.Result is T: json.Value ->
// validate + box; a scalar -> the matching decode wrapper (validate then
// extract), which reads __ret like any located helper.
func (g *gen) genJSONDecode(n *ast.CallExpr, ci *types.CallInfo, args []ast.Expr) atom {
	if types.IsJSONValue(ci.Result) {
		s := g.genExpr(args[0])
		pos := g.posLiteral(n.Pos())
		g.use(runtime.JSONValidate)
		g.line("__wisp_json_validate %s %s", pos, g.word(s))
		g.guardAfterSpill()
		return g.jsonBox(varAtom("__ret"))
	}
	switch ci.Result {
	case types.String:
		return g.genLocatedHelperCall(runtime.JSONDecodeString, "__wisp_json_decode_string", n, args)
	case types.Int:
		return g.genLocatedHelperCall(runtime.JSONDecodeInt, "__wisp_json_decode_int", n, args)
	case types.Float:
		return g.genLocatedHelperCall(runtime.JSONDecodeFloat, "__wisp_json_decode_float", n, args)
	case types.Bool:
		return g.genLocatedHelperCall(runtime.JSONDecodeBool, "__wisp_json_decode_bool", n, args)
	}
	panic("genJSONDecode: unsupported decode target " + string(ci.Result))
}
