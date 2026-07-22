package codegen

import (
	"strconv"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/runtime"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// genDebug lowers debug(x) -> string (S4). Evaluates x, resolves its static
// type via resolveType (so numeric type variables in monomorphized bodies map
// to their concrete type), and dispatches to the per-type inline renderer.
// Returns a varAtom holding the rendered string. MUST NOT emit output (print).
func (g *gen) genDebug(arg ast.Expr) atom {
	id := g.genExpr(arg)
	t := g.resolveType(g.info.Types[arg])
	return g.genDebugValue(id, t)
}

// genDebugValue emits inline shell that renders the value in atom id of type t
// to a string and returns a varAtom for the result. All runtime data flows
// through double-quoted expansions; nothing is placed in printf format strings,
// eval operands, or variable-name positions (S4 safety invariant / spec 9.6).
func (g *gen) genDebugValue(id atom, t types.Type) atom {
	switch {
	case t == types.Int:
		// int values are already decimal digit strings.
		return varAtom(g.spillToTemp(id))
	case t == types.Float:
		// __wisp_fstr <pos> <value>: canonicalize to %.17g. Pass "debug" as a
		// dummy position (the function body only uses $2).
		g.use(runtime.FStr)
		idTemp := g.spillToTemp(id)
		g.line(`__wisp_fstr debug "$%s"`, idTemp)
		tmp := g.newTemp()
		g.line(`%s="$__ret"`, tmp)
		return varAtom(tmp)
	case t == types.Bool:
		// bool values are already "true" / "false" text.
		return varAtom(g.spillToTemp(id))
	case t == types.String:
		// Wrap in literal " chars: "value" -- data in double-quoted expansion,
		// literal " are safe ASCII and not part of the data.
		idTemp := g.spillToTemp(id)
		tmp := g.newTemp()
		g.line(`%s='"'"$%s"'"'`, tmp, idTemp)
		return varAtom(tmp)
	case types.IsArray(t):
		return g.genDebugArray(id, t)
	case types.IsOptional(t):
		return g.genDebugOptional(id, t)
	case types.IsResult(t):
		return g.genDebugResult(id, t)
	case types.IsDict(t):
		return g.genDebugDict(id, t)
	case t == types.ErrorType:
		return g.genDebugError(id)
	case t == types.RunResult:
		return g.genDebugRunResult(id)
	case types.IsTuple(t):
		return g.genDebugTuple(id, t)
	case types.IsFuncref(t):
		// The funcref Type string IS exactly "fn(P,...)->R". Wrap in < > as a
		// compile-time literal: no runtime value read needed.
		tmp := g.newTemp()
		g.line(`%s='<%s>'`, tmp, string(t))
		return varAtom(tmp)
	case types.IsJSONValue(t):
		// A json.Value renders as its stored canonical JSON text (spec 4.1): a bare
		// handle-var read, no re-serialization.
		idTemp := g.spillToTemp(id)
		return g.readHandleVar(jsonTextName(idTemp))
	case g.isTaggedEnumType(t):
		return g.genDebugEnum(id, t)
	case g.info.Enums[string(t)] != nil:
		// value enum: unreachable as a debug SUBJECT (checker-rejected); left
		// unchanged. Reached only if a value enum ever slips through -- it then
		// spills its folded backing constant verbatim (the int for an int backing,
		// the raw string for a string backing): wrong-shaped but injection-safe,
		// and dead by construction (the checker rejects a value enum here).
		return varAtom(g.spillToTemp(id))
	default:
		// struct (checker rejected everything else; resolveType gave concrete type)
		return g.genDebugStruct(id, t)
	}
}

// genDebugArray renders T[]: "[e0, e1, ...]" or "[]".
func (g *gen) genDebugArray(id atom, t types.Type) atom {
	elem := types.ElemType(t)
	idTemp := g.spillToTemp(id)
	res := g.newTemp()
	sep := g.newTemp()
	g.line(`%s="["`, res)
	g.line(`%s=""`, sep)
	idxTemp, _ := g.beginArrayLoop(idTemp)
	elemAtom := g.readHandleVar(g.arrayElemNameDyn(idTemp, idxTemp))
	rendered := g.genDebugValue(elemAtom, elem)
	renderedTemp := g.spillToTemp(rendered)
	g.line(`%s="${%s}${%s}${%s}"`, res, res, sep, renderedTemp)
	g.line(`%s=", "`, sep)
	g.endArrayLoop(idxTemp)
	tmp := g.newTemp()
	g.line(`%s="${%s}]"`, tmp, res)
	return varAtom(tmp)
}

// genDebugOptional renders Optional[T]: "Some(v)" or "None".
func (g *gen) genDebugOptional(id atom, t types.Type) atom {
	elem := types.OptionalElemType(t)
	idTemp := g.spillToTemp(id)
	tag := g.readHandleVar(tagFieldName(idTemp))
	res := g.newTemp()
	g.line(`case "$%s" in`, tag.name)
	g.indent++
	g.line(`some)`)
	g.indent++
	valAtom := g.readHandleVar(tagValueName(idTemp))
	rendered := g.genDebugValue(valAtom, elem)
	renderedTemp := g.spillToTemp(rendered)
	g.line(`%s="Some($%s)"`, res, renderedTemp)
	g.indent--
	g.line(`;;`)
	g.line(`*)`)
	g.indent++
	g.line(`%s="None"`, res)
	g.indent--
	g.line(`;;`)
	g.indent--
	g.line(`esac`)
	return varAtom(res)
}

// genDebugResult renders Result[T]: "Ok(v)" or "Err(error("msg", code))".
func (g *gen) genDebugResult(id atom, t types.Type) atom {
	elem := types.ResultElemType(t)
	idTemp := g.spillToTemp(id)
	tag := g.readHandleVar(tagFieldName(idTemp))
	res := g.newTemp()
	g.line(`case "$%s" in`, tag.name)
	g.indent++
	g.line(`ok)`)
	g.indent++
	valAtom := g.readHandleVar(tagValueName(idTemp))
	rendered := g.genDebugValue(valAtom, elem)
	renderedTemp := g.spillToTemp(rendered)
	g.line(`%s="Ok($%s)"`, res, renderedTemp)
	g.indent--
	g.line(`;;`)
	g.line(`err)`)
	g.indent++
	// _value holds the error handle id.
	erridAtom := g.readHandleVar(tagValueName(idTemp))
	errRendered := g.genDebugError(erridAtom)
	errTemp := g.spillToTemp(errRendered)
	g.line(`%s="Err($%s)"`, res, errTemp)
	g.indent--
	g.line(`;;`)
	g.indent--
	g.line(`esac`)
	return varAtom(res)
}

// genDebugError renders error: error("msg", code).
// msg is user-controlled data carried in a double-quoted expansion (not in
// a format string or eval operand) to preserve injection safety.
func (g *gen) genDebugError(id atom) atom {
	idTemp := g.spillToTemp(id)
	msgAtom := g.readHandleVar("__wisp_s_${" + idTemp + "}_message")
	codeAtom := g.readHandleVar("__wisp_s_${" + idTemp + "}_code")
	msgTemp := g.spillToTemp(msgAtom)
	codeTemp := g.spillToTemp(codeAtom)
	tmp := g.newTemp()
	// Construct: error("MSG", CODE)
	// Shell: 'error("' + "$msgTemp" + '", ' + "$codeTemp" + ')'
	g.line(`%s='error("'"$%s"'", '"$%s"')'`, tmp, msgTemp, codeTemp)
	return varAtom(tmp)
}

// genDebugDict renders {K:V}: "{k: v, k: v, ...}" or "{}".
func (g *gen) genDebugDict(id atom, t types.Type) atom {
	keyType := types.DictKeyType(t)
	valType := types.DictValType(t)
	idTemp := g.spillToTemp(id)
	keysRead := g.readHandleVar(g.dictKeysName(idTemp))
	res := g.newTemp()
	sep := g.newTemp()
	g.line(`%s="{"`, res)
	g.line(`%s=""`, sep)
	tokVar := g.newTemp()
	g.line("for %s in $%s; do", tokVar, keysRead.name)
	g.indent++
	g.shellDepth++
	g.loopPendingBreak()
	// Decode key from token.
	g.use(runtime.DictDec)
	g.line(`__wisp_dkey_dec "$%s"`, tokVar)
	decoded := g.newTemp()
	g.line(`%s="$__ret"`, decoded)
	// Render key.
	var keyRendered atom
	if keyType == types.Int {
		keyRendered = varAtom(decoded)
	} else {
		// string key: wrap in quotes
		kTmp := g.newTemp()
		g.line(`%s='"'"$%s"'"'`, kTmp, decoded)
		keyRendered = varAtom(kTmp)
	}
	keyTemp := g.spillToTemp(keyRendered)
	// Read value.
	valAtom := g.readHandleVar(g.dictEntryName(idTemp, tokVar))
	rendered := g.genDebugValue(valAtom, valType)
	renderedTemp := g.spillToTemp(rendered)
	g.line(`%s="${%s}${%s}${%s}: ${%s}"`, res, res, sep, keyTemp, renderedTemp)
	g.line(`%s=", "`, sep)
	g.shellDepth--
	g.indent--
	g.line("done")
	tmp := g.newTemp()
	g.line(`%s="${%s}}"`, tmp, res)
	return varAtom(tmp)
}

// genDebugStruct renders StructName { f: v, ... } or StructName {} for empty.
func (g *gen) genDebugStruct(id atom, t types.Type) atom {
	si := g.info.Structs[string(t)]
	if si == nil {
		// Defense-in-depth: the checker rejects non-concrete/unsupported debug
		// arguments (e.g. a bare type variable), so this branch should only ever
		// see a real struct. A clear internal error beats a nil-deref SIGSEGV.
		panic("codegen: debug() reached a non-struct/unresolved type " + string(t) + " (checker should have rejected it)")
	}
	idTemp := g.spillToTemp(id)
	tmp := g.newTemp()
	if len(si.Fields) == 0 {
		g.line(`%s="%s {}"`, tmp, si.Name)
		return varAtom(tmp)
	}
	res := g.newTemp()
	sep := g.newTemp()
	g.line(`%s="%s { "`, res, si.Name)
	g.line(`%s=""`, sep)
	for _, f := range si.Fields {
		fieldAtom := g.readHandleVar("__wisp_s_${" + idTemp + "}_" + f.Name)
		rendered := g.genDebugValue(fieldAtom, f.Type)
		renderedTemp := g.spillToTemp(rendered)
		g.line(`%s="${%s}${%s}%s: ${%s}"`, res, res, sep, f.Name, renderedTemp)
		g.line(`%s=", "`, sep)
	}
	g.line(`%s="${%s} }"`, tmp, res)
	return varAtom(tmp)
}

// genDebugTuple renders (v0, v1, ..., vn-1).
func (g *gen) genDebugTuple(id atom, t types.Type) atom {
	elems := types.TupleElemTypes(t)
	idTemp := g.spillToTemp(id)
	res := g.newTemp()
	sep := g.newTemp()
	g.line(`%s="("`, res)
	g.line(`%s=""`, sep)
	for i, et := range elems {
		fieldAtom := g.readHandleVar("__wisp_s_${" + idTemp + "}_" + strconv.Itoa(i))
		rendered := g.genDebugValue(fieldAtom, et)
		renderedTemp := g.spillToTemp(rendered)
		g.line(`%s="${%s}${%s}${%s}"`, res, res, sep, renderedTemp)
		g.line(`%s=", "`, sep)
	}
	tmp := g.newTemp()
	g.line(`%s="${%s})"`, tmp, res)
	return varAtom(tmp)
}

// genDebugRunResult renders RunResult { stdout: "q", stderr: "q", code: n }.
// stdout/stderr are user-controlled strings; carried in double-quoted expansions
// only, never in printf format strings or eval operands.
func (g *gen) genDebugRunResult(id atom) atom {
	idTemp := g.spillToTemp(id)
	stdoutAtom := g.readHandleVar("__wisp_s_${" + idTemp + "}_stdout")
	stderrAtom := g.readHandleVar("__wisp_s_${" + idTemp + "}_stderr")
	codeAtom := g.readHandleVar("__wisp_s_${" + idTemp + "}_code")
	stdoutTemp := g.spillToTemp(stdoutAtom)
	stderrTemp := g.spillToTemp(stderrAtom)
	codeTemp := g.spillToTemp(codeAtom)
	// Wrap stdout/stderr in literal " chars (same pattern as string-leaf renderer).
	stdoutQ := g.newTemp()
	stderrQ := g.newTemp()
	g.line(`%s='"'"$%s"'"'`, stdoutQ, stdoutTemp)
	g.line(`%s='"'"$%s"'"'`, stderrQ, stderrTemp)
	tmp := g.newTemp()
	// Construct: RunResult { stdout: "STDOUT", stderr: "STDERR", code: CODE }
	g.line(`%s="RunResult { stdout: ${%s}, stderr: ${%s}, code: $%s }"`,
		tmp, stdoutQ, stderrQ, codeTemp)
	return varAtom(tmp)
}

// isTaggedEnumType mirrors the checker's isTaggedEnum: codegen has g.info.Enums
// but not the checker type itself.
func (g *gen) isTaggedEnumType(t types.Type) bool {
	ei, ok := g.info.Enums[string(t)]
	return ok && ei.Kind == types.EnumTagged
}

// genDebugEnum renders a tagged-union enum: VariantName(<debug payload>) for a
// payload variant, bare VariantName for a no-payload variant. The variant name is
// a compiler-emitted identifier literal; the payload flows only through
// double-quoted expansions and the print()-via-%s discipline genDebug already
// uses, so trailing bytes/metacharacters in a string payload survive byte-exact.
func (g *gen) genDebugEnum(id atom, t types.Type) atom {
	ei := g.info.Enums[string(t)]
	idTemp := g.spillToTemp(id)
	tag := g.readHandleVar(tagFieldName(idTemp))
	res := g.newTemp()
	g.line(`case "$%s" in`, tag.name)
	g.indent++
	for i, name := range ei.Variants {
		g.line(`%s)`, name) // variant-name identifier literal: injection-safe, unquoted
		g.indent++
		payload := ei.Payloads[i]
		if payload == types.Invalid {
			g.line(`%s="%s"`, res, name) // bare variant
		} else {
			valAtom := g.readHandleVar(tagValueName(idTemp))
			rendered := g.genDebugValue(valAtom, g.debugPayloadType(payload))
			renderedTemp := g.spillToTemp(rendered)
			g.line(`%s="%s($%s)"`, res, name, renderedTemp)
		}
		g.indent--
		g.line(`;;`)
	}
	// Defense-in-depth default arm, matching genDebugOptional/genDebugResult:
	// the tag is always a compiler-emitted declared variant, so this cannot fire
	// through the type system, but a `*)` arm avoids leaving res unset (silent
	// empty render) on any future codegen/checker drift.
	g.line(`*)`)
	g.indent++
	g.line(`%s="<?>"`, res)
	g.indent--
	g.line(`;;`)
	g.indent--
	g.line(`esac`)
	return varAtom(res)
}

// debugPayloadType maps a value-enum payload type to its backing scalar so the
// renderer dispatches by backing (FR-020/SC-036/SC-036b) and never reaches the
// dead int-render branch, which would mis-render a string/bool-backed payload. A
// non-value-enum payload type is returned unchanged.
func (g *gen) debugPayloadType(pt types.Type) types.Type {
	if ei, ok := g.info.Enums[string(pt)]; ok && ei.Kind == types.EnumValue {
		return ei.Backing // Int/String/Bool -> the matching genDebugValue leaf case
	}
	return pt
}
