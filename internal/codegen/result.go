package codegen

import (
	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/runtime"
)

// Result lowering. A two-field reference handle, mirroring the Optional shape and
// SHARING its tag/value fields (one sum-tag mechanism):
//   __wisp_s_<id>_tag   = "ok" | "err"
//   __wisp_s_<id>_value = the Ok payload (tag=ok) OR the error handle id
//                         (tag=err); only one meaning is live, per the tag.
// All values flow through the deferred $temp store of setHandleVar/readHandleVar,
// so the lowering is injection-safe by construction (same as Optional).
// tagFieldName/tagValueName are defined in optional.go (shared).

// genOk lowers Ok(x): a fresh handle with tag=ok and value=x.
func (g *gen) genOk(args []ast.Expr) atom {
	v := g.genExpr(args[0])
	id := g.allocHandle()
	g.setHandleVar(tagFieldName(id), litAtom("ok"))
	g.setHandleVar(tagValueName(id), v)
	return varAtom(id)
}

// genErr lowers Err(e): a fresh handle with tag=err and value=<error handle id>.
func (g *gen) genErr(args []ast.Expr) atom {
	e := g.genExpr(args[0]) // an error handle id
	id := g.allocHandle()
	g.setHandleVar(tagFieldName(id), litAtom("err"))
	g.setHandleVar(tagValueName(id), e)
	return varAtom(id)
}

// genIsOk reads the tag and compares it to want ("ok" for is_ok, "err" for
// is_err), emitting a bool temp.
func (g *gen) genIsOk(args []ast.Expr, want string) atom {
	r := g.genExpr(args[0])
	vrt := g.readHandleVar(tagFieldName(r.name))
	res := g.newTemp()
	g.line("if [ \"$%s\" = %s ]; then %s=true; else %s=false; fi", vrt.name, want, res, res)
	return varAtom(res)
}

// genUnwrapResult reads the tag; on a non-ok handle it reads the carried error
// handle's .message and aborts located with it (catchable), so an unwrapped
// failure propagates its original text; otherwise it returns _value.
func (g *gen) genUnwrapResult(n *ast.CallExpr, args []ast.Expr) atom {
	r := g.genExpr(args[0])
	vrt := g.readHandleVar(tagFieldName(r.name))
	g.use(runtime.Fail)
	// Read the carried error handle's id and .message ONLY on the err branch. On
	// the Ok path _value is the (arbitrary) Ok payload; using it to build an
	// eval'd handle-var name would re-parse user data, so the reads stay guarded
	// behind tag != ok.
	g.line("if [ \"$%s\" != ok ]; then", vrt.name)
	g.indent++
	errid := g.readHandleVar(tagValueName(r.name)) // error handle id (err branch)
	msg := g.readHandleVar("__wisp_s_${" + errid.name + "}_message")
	g.line("__wisp_fail %s %s", g.posLiteral(n.Pos()), g.word(msg))
	g.indent--
	g.line("fi")
	g.guardAfterSpill()
	return g.readHandleVar(tagValueName(r.name))
}

// genUnwrapErr reads the tag; on an ok handle it aborts located with the static
// "unwrap_err of Ok"; otherwise it returns the stored error handle id directly
// (aliased, so .message resolves on it like a bound catch variable).
func (g *gen) genUnwrapErr(n *ast.CallExpr, args []ast.Expr) atom {
	r := g.genExpr(args[0])
	vrt := g.readHandleVar(tagFieldName(r.name))
	g.use(runtime.Fail)
	g.line("if [ \"$%s\" != err ]; then __wisp_fail %s %s; fi", vrt.name, g.posLiteral(n.Pos()), shellSingleQuote("unwrap_err of Ok"))
	g.guardAfterSpill()
	return g.readHandleVar(tagValueName(r.name))
}

// genUnwrapOrResult returns _value when tag=ok, else the eagerly-evaluated
// fallback.
func (g *gen) genUnwrapOrResult(args []ast.Expr) atom {
	r := g.genExpr(args[0])
	fb := g.spillToTemp(g.genExpr(args[1])) // eager: evaluated whether or not ok (spec 3.3)
	vrt := g.readHandleVar(tagFieldName(r.name))
	val := g.readHandleVar(tagValueName(r.name))
	res := g.newTemp()
	g.line("if [ \"$%s\" = ok ]; then %s=\"$%s\"; else %s=\"$%s\"; fi", vrt.name, res, val.name, res, fb)
	return varAtom(res)
}
