package codegen

import (
	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/runtime"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// Optional lowering. An Optional is a two-field reference handle (like error):
//   __wisp_s_<id>_tag   = "some" | "none"
//   __wisp_s_<id>_value = the contained value (unset for None)
// The tag field and value field are shared with the Result lowering (one sum-tag
// mechanism). All values flow only through the deferred \$temp store of
// setHandleVar / readHandleVar, so the lowering is injection-safe by construction.

// tagFieldName is the single sum-discriminant field shared by Optional and Result;
// it holds the lowercase variant string (some/none/ok/err).
func tagFieldName(idTemp string) string { return "__wisp_s_${" + idTemp + "}_tag" }

// tagValueName is the single payload field shared by Optional and Result (the
// Optional contained value, or the Result Ok payload / carried error-handle id).
func tagValueName(idTemp string) string { return "__wisp_s_${" + idTemp + "}_value" }

// genSome lowers Some(x): a fresh handle with tag=some and value=x.
func (g *gen) genSome(args []ast.Expr) atom {
	v := g.genExpr(args[0])
	id := g.allocHandle()
	g.setHandleVar(tagFieldName(id), litAtom("some"))
	g.setHandleVar(tagValueName(id), v)
	return varAtom(id)
}

// genNone lowers None: a fresh handle with tag=none and value unset. Each None
// allocs a fresh handle (no shared sentinel); the tag is the sole source of
// truth (spec 4).
func (g *gen) genNone() atom {
	id := g.allocHandle()
	g.setHandleVar(tagFieldName(id), litAtom("none"))
	return varAtom(id)
}

// genIsSome / genIsNone read the tag and emit a bool temp. want is the tag string
// to compare against: "some" for is_some, "none" for is_none.
func (g *gen) genIsSome(args []ast.Expr, want string) atom {
	o := g.genExpr(args[0])
	p := g.readHandleVar(tagFieldName(o.name))
	res := g.newTemp()
	g.line("if [ \"$%s\" = %s ]; then %s=true; else %s=false; fi", p.name, want, res, res)
	return varAtom(res)
}

// genUnwrap reads the tag; on a non-some handle aborts located 'unwrap of None';
// else reads value.
func (g *gen) genUnwrap(n *ast.CallExpr, args []ast.Expr) atom {
	o := g.genExpr(args[0])
	p := g.readHandleVar(tagFieldName(o.name))
	g.use(runtime.Fail)
	g.line("if [ \"$%s\" != some ]; then __wisp_fail %s %s; fi", p.name, g.posLiteral(n.Pos()), shellSingleQuote("unwrap of None"))
	g.guardAfterSpill()
	return g.readHandleVar(tagValueName(o.name))
}

// genIntSentinelToOptional calls a prelude helper that returns an int byte index
// in __ret with -1 meaning "absent", and wraps the result into an Optional[int]
// handle: -1 -> None, otherwise Some(index). Used by the migrated index_of /
// last_index_of (the prelude bodies are unchanged). The call/capture below is a
// verbatim mirror of genHelperCall (non-fallible: no <pos>, no guard).
func (g *gen) genIntSentinelToOptional(id, fn string, n *ast.CallExpr, args []ast.Expr) atom {
	_ = n // call site unused (non-fallible), kept for signature parity with the located form
	words := g.genArgWords(args)
	g.use(id)
	g.line("%s%s", fn, joinWords(words))
	r := g.newTemp()
	g.line("%s=\"$__ret\"", r)
	out := g.allocHandle()
	g.line("if [ \"$%s\" = -1 ]; then", r)
	g.indent++
	g.setHandleVar(tagFieldName(out), litAtom("none"))
	g.indent--
	g.line("else")
	g.indent++
	g.setHandleVar(tagFieldName(out), litAtom("some"))
	g.setHandleVar(tagValueName(out), varAtom(r))
	g.indent--
	g.line("fi")
	return varAtom(out)
}

// genStrSentinelToOptional calls a prelude helper that writes a string to __ret
// and signals absence via a NONZERO exit status (e.g. which's command -v). The
// status MUST be captured on the line immediately after the call, before any
// other command clobbers $?. Maps nonzero -> None, zero -> Some(__ret). The
// call/capture mirrors genHelperCall (non-fallible: no <pos>, no guard) -- the
// helper never aborts; a nonzero status is data (absence), not a fault.
func (g *gen) genStrSentinelToOptional(id, fn string, n *ast.CallExpr, args []ast.Expr) atom {
	_ = n
	words := g.genArgWords(args)
	g.use(id)
	g.line("%s%s", fn, joinWords(words))
	rc := g.newTemp()
	g.line("%s=$?", rc)
	val := g.newTemp()
	g.line("%s=\"$__ret\"", val)
	out := g.allocHandle()
	g.line("if [ \"$%s\" -eq 0 ]; then", rc)
	g.indent++
	g.setHandleVar(tagFieldName(out), litAtom("some"))
	g.setHandleVar(tagValueName(out), varAtom(val))
	g.indent--
	g.line("else")
	g.indent++
	g.setHandleVar(tagFieldName(out), litAtom("none"))
	g.indent--
	g.line("fi")
	return varAtom(out)
}

// genLocatedTokenToOptional calls a FALLIBLE prelude helper that writes a
// TOKEN-prefixed string to __ret: a leading byte 1 means a match (the rest is
// the value) and 0 means no match. Unlike genStrSentinelToOptional, match vs.
// no-match is this PAYLOAD TOKEN, not the exit status -- the exit status is
// reserved for the malformed-pattern located abort (the helper aborts via
// __wisp_fail before returning on rc != 0). So it forwards <pos> and guards
// after the call (like genLocatedHelperCall), then peels the first byte:
// 1 -> Some(rest), 0 -> None. It shares only the Optional-handle construction
// with genStrSentinelToOptional, not the exit-0/1 discrimination.
func (g *gen) genLocatedTokenToOptional(id, fn string, n *ast.CallExpr, args []ast.Expr) atom {
	pos := g.posLiteral(n.Pos())
	words := g.genArgWords(args)
	g.use(id)
	g.line("%s %s%s", fn, pos, joinWords(words))
	tok := g.newTemp()
	g.line("%s=\"$__ret\"", tok)
	// A malformed-pattern fault set pending; skip the rest on a fault (M5).
	g.guardAfterSpill()
	out := g.allocHandle()
	// First byte is the status token; the rest (if present) is the value.
	g.line("case \"$%s\" in", tok)
	g.indent++
	g.line("1*)")
	g.indent++
	g.setHandleVar(tagFieldName(out), litAtom("some"))
	val := g.newTemp()
	g.line("%s=\"${%s#1}\"", val, tok)
	g.setHandleVar(tagValueName(out), varAtom(val))
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

// genOptionalEquality lowers structural == or != for two Optional handles
// a and b of the SAME comparable-Optional type t (ComparableOptional(t)==true).
// negated=true emits != (negate the == result). The element compare recurses
// for a nested Optional element type; for a concrete element it uses the scalar
// string compare. Every expansion is double-quoted (injection-safety invariant).
func (g *gen) genOptionalEquality(a, b atom, t types.Type, negated bool) atom {
	elem := types.OptionalElemType(t)
	// Read both tag fields into temps.
	ta := g.readHandleVar(tagFieldName(a.name))
	tb := g.readHandleVar(tagFieldName(b.name))
	res := g.newTemp()
	// Outer: compare tags.
	g.line("if [ \"$%s\" = \"$%s\" ]; then", ta.name, tb.name)
	g.indent++
	// Same tag: if 'some', compare values; if 'none'+'none', equal.
	g.line("if [ \"$%s\" = some ]; then", ta.name)
	g.indent++
	va := g.readHandleVar(tagValueName(a.name))
	vb := g.readHandleVar(tagValueName(b.name))
	if types.ComparableOptional(elem) {
		// Nested Optional: recurse structurally (never compare inner handle ids).
		inner := g.genOptionalEquality(va, vb, elem, false)
		g.line("%s=%s", res, g.word(inner))
	} else {
		// Concrete (int/bool/string): scalar string compare.
		g.line("if [ \"$%s\" = \"$%s\" ]; then %s=true; else %s=false; fi",
			va.name, vb.name, res, res)
	}
	g.indent--
	g.line("else")
	g.indent++
	g.line("%s=true", res)
	g.indent--
	g.line("fi")
	g.indent--
	g.line("else")
	g.indent++
	g.line("%s=false", res)
	g.indent--
	g.line("fi")
	if negated {
		neg := g.newTemp()
		g.line("if [ \"$%s\" = true ]; then %s=false; else %s=true; fi", res, neg, neg)
		return varAtom(neg)
	}
	return varAtom(res)
}

// genUnwrapOr eagerly evaluates both operands, then tag-selects (no abort).
func (g *gen) genUnwrapOr(args []ast.Expr) atom {
	o := g.genExpr(args[0])
	fb := g.spillToTemp(g.genExpr(args[1])) // eager: fallback evaluated whether or not present (spec 3.3)
	p := g.readHandleVar(tagFieldName(o.name))
	val := g.readHandleVar(tagValueName(o.name))
	res := g.newTemp()
	g.line("if [ \"$%s\" = some ]; then %s=\"$%s\"; else %s=\"$%s\"; fi", p.name, res, val.name, res, fb)
	return varAtom(res)
}
