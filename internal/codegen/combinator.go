package codegen

import "github.com/mitchellnemitz/wisp/internal/ast"

// Combinator lowering. Each combinator emits a single BALANCED if/else/fi
// conditional that reads the operand's tag field, lazily
// invokes the funcref ONLY on the taken branch, and constructs a fresh result
// handle.
//
// `out` is an id alias (a temp holding a handle id), not a fresh per-combinator
// copy: handles are immutable, so a passthrough branch just aliases the operand
// id and a chaining branch aliases f's returned $__ret (spec section 4). Only the
// genuinely-constructing branches (map's Some/Ok, filter's drop-None, map_err's
// Err) allocate a fresh handle.
//
// FAULT PROPAGATION (spec invariant): the funcref call and the subsequent
// alias/field writes all live inside ONE wisp statement, which errMode already
// wraps in a per-statement skip-guard (error.go genStmt). If f faults it sets
// __wisp_err_pending and returns; the rest of this statement still runs and may
// alias `out` to the garbage $__ret, but that value is DEAD: every later statement
// is skip-guarded, so `out` is never read, and the pending fault propagates at the
// function-end unwind (or to an enclosing try). This is exactly the genMap/genFilter
// array grain (aggregate.go:220-257), which likewise calls f indirectly and reads
// $__ret with NO per-call guardAfterSpill.
//
// CRITICAL -- do NOT call g.guardAfterSpill() inside the manually-emitted
// if/else/fi. guardAfterSpill -> openGuard emits an UNCLOSED `if [ -z pending ];
// then` whose matching `fi` is deferred to the statement boundary (error.go
// openGuard/closeGuardsTo). Opening it between a hand-written `then` and its
// `else`/`fi` mis-binds the else and unbalances the fi, producing broken shell
// that ShellCheck and every shell reject -- and errMode is GLOBAL (codegen.go
// programUsesErrorHandling), so this fires for ANY program that also uses
// try/throw, not just the fault fixture. The only sanctioned guardAfterSpill
// placement is genUnwrapResult's: AFTER the fi has fully closed (result.go:64).
// These combinators need no guardAfterSpill at all (dead-handle-never-read).
//
// Injection safety: all values flow through setHandleVar/readHandleVar/g.word();
// the funcref name is a compile-time mangled identifier, spilled to a temp;
// the payload value flows through a quoted shell variable. No value ever forms
// an eval'd variable name.

// genMapTagged lowers map over a single-present-variant sum handle: map(Optional[T],
// fn(T)->U) -> Optional[U] (tag "some") and map(Result[T], fn(T)->U) -> Result[U]
// (tag "ok"). Present branch calls f and wraps its $__ret in a fresh handle with
// the same tag; the absent branch aliases the operand (unchanged handle).
func (g *gen) genMapTagged(args []ast.Expr, tag string) atom {
	o := g.genExpr(args[0])
	f := g.spillToTemp(g.genExpr(args[1]))
	p := g.readHandleVar(tagFieldName(o.name))
	out := g.newTemp()
	g.line("if [ \"$%s\" = %s ]; then", p.name, tag)
	g.indent++
	val := g.readHandleVar(tagValueName(o.name))
	g.line("\"$%s\" %s", f, g.word(val))
	ret := g.newTemp()
	g.line("%s=\"$__ret\"", ret)
	h := g.allocHandle()
	g.setHandleVar(tagFieldName(h), litAtom(tag))
	g.setHandleVar(tagValueName(h), varAtom(ret))
	g.line("%s=\"$%s\"", out, h)
	g.indent--
	g.line("else")
	g.indent++
	g.line("%s=\"$%s\"", out, o.name) // absent branch: alias the operand (unchanged handle)
	g.indent--
	g.line("fi")
	return varAtom(out)
}

// genAndThenTagged lowers and_then over a single-present-variant sum handle:
// and_then(Optional[T], fn(T)->Optional[U]) (tag "some") and and_then(Result[T],
// fn(T)->Result[U]) (tag "ok"). Present branch aliases f's returned handle id
// ($__ret); the absent branch aliases the operand (unchanged handle).
func (g *gen) genAndThenTagged(args []ast.Expr, tag string) atom {
	o := g.genExpr(args[0])
	f := g.spillToTemp(g.genExpr(args[1]))
	p := g.readHandleVar(tagFieldName(o.name))
	out := g.newTemp()
	g.line("if [ \"$%s\" = %s ]; then", p.name, tag)
	g.indent++
	val := g.readHandleVar(tagValueName(o.name))
	g.line("\"$%s\" %s", f, g.word(val))
	g.line("%s=\"$__ret\"", out) // f's $__ret IS the result handle id
	g.indent--
	g.line("else")
	g.indent++
	g.line("%s=\"$%s\"", out, o.name) // absent branch: alias the operand (unchanged handle)
	g.indent--
	g.line("fi")
	return varAtom(out)
}

// genFilterOptional lowers filter(Optional[T], fn(T)->bool) -> Optional[T].
//
//	Some(x) and f(x)=true -> the operand id (Some(x) unchanged), aliased;
//	Some(x) and f(x)=false -> a fresh None; None -> the operand id, aliased.
func (g *gen) genFilterOptional(args []ast.Expr) atom {
	o := g.genExpr(args[0])
	f := g.spillToTemp(g.genExpr(args[1]))
	p := g.readHandleVar(tagFieldName(o.name))
	out := g.newTemp()
	g.line("if [ \"$%s\" = some ]; then", p.name)
	g.indent++
	val := g.spillToTemp(g.readHandleVar(tagValueName(o.name)))
	g.line("\"$%s\" \"$%s\"", f, val)
	keep := g.newTemp()
	g.line("%s=\"$__ret\"", keep)
	// Balanced inner if/else/fi -- no guard opened between then and else.
	g.line("if [ \"$%s\" = true ]; then", keep)
	g.indent++
	g.line("%s=\"$%s\"", out, o.name) // keep: alias the operand (Some(x) unchanged)
	g.indent--
	g.line("else")
	g.indent++
	none := g.allocHandle()
	g.setHandleVar(tagFieldName(none), litAtom("none"))
	g.line("%s=\"$%s\"", out, none) // drop: a fresh None
	g.indent--
	g.line("fi")
	g.indent--
	g.line("else")
	g.indent++
	g.line("%s=\"$%s\"", out, o.name) // None: alias the operand (unchanged None handle)
	g.indent--
	g.line("fi")
	return varAtom(out)
}

// genOrElseOptional lowers or_else(Optional[T], fn()->Optional[T]) -> Optional[T].
// `out` is an id ALIAS (immutable handles): Some(x) -> the operand id (f not
// called); None -> $__ret (f's returned Optional handle id).
func (g *gen) genOrElseOptional(args []ast.Expr) atom {
	o := g.genExpr(args[0])
	f := g.spillToTemp(g.genExpr(args[1]))
	p := g.readHandleVar(tagFieldName(o.name))
	out := g.newTemp()
	g.line("if [ \"$%s\" = some ]; then", p.name)
	g.indent++
	g.line("%s=\"$%s\"", out, o.name) // present: alias the operand handle
	g.indent--
	g.line("else")
	g.indent++
	g.line("\"$%s\"", f)         // absent: call f (zero-argument)
	g.line("%s=\"$__ret\"", out) // $__ret IS the result Optional handle id
	g.indent--
	g.line("fi")
	return varAtom(out)
}

// genOrElseResult lowers or_else(Result[T], fn(error)->Result[T]) -> Result[T].
// `out` is an id ALIAS: Ok(x) -> the operand id (f not called); Err(e) -> $__ret
// (f's returned Result handle id). The raw error-handle id is passed as f's
// argument (like genUnwrapErr), so .message resolves on the bound param.
func (g *gen) genOrElseResult(args []ast.Expr) atom {
	r := g.genExpr(args[0])
	f := g.spillToTemp(g.genExpr(args[1]))
	vrt := g.readHandleVar(tagFieldName(r.name))
	out := g.newTemp()
	g.line("if [ \"$%s\" = ok ]; then", vrt.name)
	g.indent++
	g.line("%s=\"$%s\"", out, r.name) // Ok: alias the operand handle
	g.indent--
	g.line("else")
	g.indent++
	errid := g.readHandleVar(tagValueName(r.name))
	g.line("\"$%s\" %s", f, g.word(varAtom(errid.name)))
	g.line("%s=\"$__ret\"", out) // $__ret IS the result Result handle id
	g.indent--
	g.line("fi")
	return varAtom(out)
}

// genMapErrResult lowers map_err(Result[T], fn(error)->error) -> Result[T].
//
//	Ok(x) -> the operand id (f not called), aliased;
//	Err(e) -> a fresh Err(f(e)) where f receives the raw error-handle id.
func (g *gen) genMapErrResult(args []ast.Expr) atom {
	r := g.genExpr(args[0])
	f := g.spillToTemp(g.genExpr(args[1]))
	vrt := g.readHandleVar(tagFieldName(r.name))
	out := g.newTemp()
	g.line("if [ \"$%s\" = ok ]; then", vrt.name)
	g.indent++
	g.line("%s=\"$%s\"", out, r.name) // Ok: alias the operand (unchanged Ok handle)
	g.indent--
	g.line("else")
	g.indent++
	errid := g.readHandleVar(tagValueName(r.name))
	g.line("\"$%s\" %s", f, g.word(varAtom(errid.name)))
	newErr := g.newTemp()
	g.line("%s=\"$__ret\"", newErr)
	errHandle := g.allocHandle()
	g.setHandleVar(tagFieldName(errHandle), litAtom("err"))
	g.setHandleVar(tagValueName(errHandle), varAtom(newErr))
	g.line("%s=\"$%s\"", out, errHandle) // a fresh Err wrapping the transformed error
	g.indent--
	g.line("fi")
	return varAtom(out)
}
