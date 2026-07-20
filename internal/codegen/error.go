package codegen

import (
	"strconv"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/runtime"
)

// Error handling (M5). The lowering is flag-based and runs in the current shell
// (no subshell, no mktemp, no trap), so try-body mutations persist (spec
// section 1). Five runtime variables drive it (program-conditional, emitted
// only when errMode):
//
//   __wisp_try_depth  -- >0 while anywhere inside a try construct
//   __wisp_err_pending -- non-empty while a fault is in flight
//   __wisp_err_msg     -- the in-flight error's message
//   __wisp_err_code    -- the in-flight error's int code (e.code on catch)
//   __wisp_err_pos     -- a thrown error's source position; drives the located
//                         "wisp: <pos>: <msg>" abort when a throw escapes
//                         uncaught (empty for a fault, whose msg embeds its pos)
//
// All four error vars (pending/msg/code/pos) are saved and restored across every
// try boundary so a nested fault cannot clobber a suspended outer error.
//
// __wisp_fail (mode-aware) and __wisp_throw set pending+msg and RETURN at
// depth>0 instead of exiting. Every statement in a function body (and in a try/
// catch/finally body) is wrapped in a skip-guard `if [ -z "$__wisp_err_pending" ];
// then <stmt> fi`, so once a fault is pending the rest of the body is skipped
// and control flows to the function's end (unwind) or to a try's post-body
// handler check. A nested skip-guard is also opened after each call/value-
// producer spill so the REST of a faulting statement's evaluation is skipped
// (fail-at-first-fault mid-statement, spec invariant 2/14).

// programUsesErrorHandling reports whether prog contains any try or throw, which
// turns on the M5 guard scaffolding. A program with neither is byte-for-byte the
// M4 shape (zero overhead, spec section 5).
func programUsesErrorHandling(prog *ast.Program) bool {
	for _, fn := range prog.Funcs {
		if blockUsesErrorHandling(fn.Body) {
			return true
		}
	}
	// In test mode the test bodies are part of the program too: a try/throw in a
	// `test (...)` block must enable error mode, exactly as one in a function.
	for _, td := range prog.Tests {
		if blockUsesErrorHandling(td.Body) {
			return true
		}
	}
	return false
}

func blockUsesErrorHandling(stmts []ast.Stmt) bool {
	for _, s := range stmts {
		if stmtUsesErrorHandling(s) {
			return true
		}
	}
	return false
}

func stmtUsesErrorHandling(s ast.Stmt) bool {
	switch n := s.(type) {
	case *ast.ThrowStmt, *ast.TryStmt:
		return true
	case *ast.IfStmt:
		if blockUsesErrorHandling(n.Then) || blockUsesErrorHandling(n.Else) {
			return true
		}
		for _, ei := range n.ElseIfs {
			if blockUsesErrorHandling(ei.Body) {
				return true
			}
		}
	case *ast.WhileStmt:
		return blockUsesErrorHandling(n.Body)
	case *ast.ForStmt:
		return blockUsesErrorHandling(n.Body)
	case *ast.ForInStmt:
		return blockUsesErrorHandling(n.Body)
	case *ast.SwitchStmt:
		for _, cs := range n.Cases {
			if blockUsesErrorHandling(cs.Body) {
				return true
			}
		}
		return blockUsesErrorHandling(n.Default)
	}
	return false
}

// programUsesWrap reports whether prog calls the `wrap` builtin anywhere. It
// gates the __wisp_err_cause throw-path threading (the four sites genThrow/
// bindCatchVar/__wisp_fail/genTry): `wrap` is the only producer of an error
// cause, so a program that never calls it cannot carry a cause and the threading
// would be dead code. Emitting the threading unconditionally would change the
// .sh of every error-handling program, breaking the no-use byte-identity
// invariant (AC6), hence the gate.
//
// Unlike programUsesErrorHandling (which walks STATEMENT structure only and
// treats throw/try as opaque triggers), `wrap` is a CALL EXPRESSION that appears
// in expression positions -- `let x = wrap(...)`, a return value, `throw
// wrap(...)`, a call argument, an assignment RHS, etc. So this predicate must
// descend into EVERY expression position across every function body, every
// function/param default, every top-level const, and every test body; a
// statement-only scan would miss `let x = wrap(...)` and silently drop the
// threading. It scans the whole program (not reachability-pruned): the gate is a
// program-wide property, and an unreachable `wrap` is harmless to enable for.
func programUsesWrap(prog *ast.Program) bool {
	for _, fn := range prog.Funcs {
		for i := range fn.Params {
			if exprUsesWrap(fn.Params[i].Default) {
				return true
			}
		}
		if blockUsesWrap(fn.Body) {
			return true
		}
	}
	for _, c := range prog.Consts {
		if exprUsesWrap(c.Value) {
			return true
		}
	}
	for _, td := range prog.Tests {
		if blockUsesWrap(td.Body) {
			return true
		}
	}
	return false
}

func blockUsesWrap(stmts []ast.Stmt) bool {
	for _, s := range stmts {
		if stmtUsesWrap(s) {
			return true
		}
	}
	return false
}

func stmtUsesWrap(s ast.Stmt) bool {
	switch n := s.(type) {
	case *ast.ConstStmt:
		return exprUsesWrap(n.Value)
	case *ast.FinalStmt:
		return exprUsesWrap(n.Value)
	case *ast.LetStmt:
		return exprUsesWrap(n.Value)
	case *ast.TupleBindStmt:
		return exprUsesWrap(n.Value)
	case *ast.AssignStmt:
		return exprUsesWrap(n.Value)
	case *ast.FieldAssignStmt:
		return exprUsesWrap(n.Target) || exprUsesWrap(n.Value)
	case *ast.IndexAssignStmt:
		return exprUsesWrap(n.Target) || exprUsesWrap(n.Index) || exprUsesWrap(n.Value)
	case *ast.ReturnStmt:
		return exprUsesWrap(n.Value)
	case *ast.ExprStmt:
		return exprUsesWrap(n.X)
	case *ast.ThrowStmt:
		return exprUsesWrap(n.X)
	case *ast.IfStmt:
		if exprUsesWrap(n.Cond) || blockUsesWrap(n.Then) || blockUsesWrap(n.Else) {
			return true
		}
		for _, ei := range n.ElseIfs {
			if exprUsesWrap(ei.Cond) || blockUsesWrap(ei.Body) {
				return true
			}
		}
	case *ast.WhileStmt:
		return exprUsesWrap(n.Cond) || blockUsesWrap(n.Body)
	case *ast.ForStmt:
		return stmtUsesWrap(n.Init) || exprUsesWrap(n.Cond) || stmtUsesWrap(n.Post) || blockUsesWrap(n.Body)
	case *ast.ForInStmt:
		return exprUsesWrap(n.Coll) || blockUsesWrap(n.Body)
	case *ast.MatchStmt:
		if exprUsesWrap(n.Scrutinee) {
			return true
		}
		for _, arm := range n.Arms {
			if blockUsesWrap(arm.Body) {
				return true
			}
		}
	case *ast.SwitchStmt:
		if exprUsesWrap(n.Subject) {
			return true
		}
		for _, cs := range n.Cases {
			for _, v := range cs.Values {
				if exprUsesWrap(v) {
					return true
				}
			}
			if blockUsesWrap(cs.Body) {
				return true
			}
		}
		return blockUsesWrap(n.Default)
	case *ast.TryStmt:
		return blockUsesWrap(n.Body) || blockUsesWrap(n.Catch) || blockUsesWrap(n.Finally)
	}
	return false
}

func exprUsesWrap(e ast.Expr) bool {
	switch n := e.(type) {
	case nil:
		return false
	case *ast.CallExpr:
		if n.CalleeName == "wrap" {
			return true
		}
		if exprUsesWrap(n.Callee) {
			return true
		}
		for _, a := range n.Args {
			if exprUsesWrap(a) {
				return true
			}
		}
	case *ast.UnaryExpr:
		return exprUsesWrap(n.X)
	case *ast.BinaryExpr:
		return exprUsesWrap(n.L) || exprUsesWrap(n.R)
	case *ast.FieldAccess:
		return exprUsesWrap(n.X)
	case *ast.IndexExpr:
		return exprUsesWrap(n.X) || exprUsesWrap(n.Index)
	case *ast.StringLit:
		for _, p := range n.Parts {
			if !p.IsText() && exprUsesWrap(p.Expr) {
				return true
			}
		}
	case *ast.StructLit:
		for _, f := range n.Fields {
			if exprUsesWrap(f.Value) {
				return true
			}
		}
	case *ast.ArrayLit:
		for _, el := range n.Elems {
			if exprUsesWrap(el) {
				return true
			}
		}
	case *ast.TupleLit:
		for _, el := range n.Elems {
			if exprUsesWrap(el) {
				return true
			}
		}
	case *ast.DictLit:
		for _, en := range n.Entries {
			if exprUsesWrap(en.Key) || exprUsesWrap(en.Value) {
				return true
			}
		}
	}
	return false
}

// --- guard primitives ---

// openGuard emits one skip-guard `if [ -z "$__wisp_err_pending" ]; then`,
// raising the indent and the open-guard count. The output line count at open is
// recorded so closeGuardsTo can detect an empty body (e.g. a trailing guard
// opened after a void call with nothing left to skip) and fill it with a `:`
// no-op, keeping the script ShellCheck-clean (no empty `then`). A no-op outside
// errMode.
func (g *gen) openGuard() {
	if !g.errMode {
		return
	}
	g.line("if [ -z \"$__wisp_err_pending\" ]; then")
	g.indent++
	g.guardDepth++
	g.guardOpenLines = append(g.guardOpenLines, len(g.bodyMap))
}

// closeGuardsTo closes every skip-guard opened since guardDepth was n, emitting
// one `fi` and dropping the indent for each. If a guard's body emitted no lines
// (an empty `then`), a `:` no-op is written first so the output stays valid
// POSIX sh and ShellCheck-clean. A no-op outside errMode.
func (g *gen) closeGuardsTo(n int) {
	if !g.errMode {
		return
	}
	for g.guardDepth > n {
		openedAt := g.guardOpenLines[len(g.guardOpenLines)-1]
		g.guardOpenLines = g.guardOpenLines[:len(g.guardOpenLines)-1]
		if len(g.bodyMap) == openedAt {
			g.line(":")
		}
		g.indent--
		g.guardDepth--
		g.line("fi")
	}
}

// guardAfterSpill opens a nested skip-guard immediately after a call/value-
// producer spill point so the REST of the current statement's evaluation is
// skipped when the call faulted (fail-at-first-fault, mid-statement). The guard
// closes at the enclosing statement/condition boundary via closeGuardsTo. A
// no-op outside errMode.
func (g *gen) guardAfterSpill() {
	g.openGuard()
}

// loopPendingBreak emits, at the top of a loop body in errMode, a `break` when a
// fault is pending so a loop whose body skips (and whose condition may keep
// faulting) does not spin: it exits the loop and lets the unwind continue. The
// break targets the innermost shell loop, which is this loop's own `while`.
func (g *gen) loopPendingBreak() {
	if !g.errMode {
		return
	}
	g.line("if [ -n \"$__wisp_err_pending\" ]; then break; fi")
}

// --- error value codegen ---

// genErrorLit lowers error(msg) to a fresh error handle with message=msg and
// code=0. An error handle reuses the M3 handle machinery: an id from __wisp_alloc
// with the message stored in __wisp_s_<id>_message and code in __wisp_s_<id>_code.
func (g *gen) genErrorLit(args []ast.Expr) atom {
	msg := g.genExpr(args[0])
	id := g.allocHandle()
	g.setHandleVar("__wisp_s_${"+id+"}_message", msg)
	g.setHandleVar("__wisp_s_${"+id+"}_code", litAtom("0"))
	return varAtom(id)
}

// genErrorWithLit lowers error_with(code, msg) to a fresh error handle with both
// fields set. The code is an int and the message is a string.
func (g *gen) genErrorWithLit(args []ast.Expr) atom {
	code := g.genExpr(args[0])
	msg := g.genExpr(args[1])
	id := g.allocHandle()
	g.setHandleVar("__wisp_s_${"+id+"}_code", code)
	g.setHandleVar("__wisp_s_${"+id+"}_message", msg)
	return varAtom(id)
}

// genWrap lowers wrap(err, msg): allocate a fresh error handle with message=msg,
// code=0, and _cause set to the inner error's handle id. The inner error is
// unchanged. msg flows as inert data via setHandleVar (injection-safe). The cause
// is an integer handle id written without re-evaluation.
func (g *gen) genWrap(args []ast.Expr) atom {
	inner := g.genExpr(args[0])
	msg := g.genExpr(args[1])
	id := g.allocHandle()
	g.setHandleVar("__wisp_s_${"+id+"}_message", msg)
	g.setHandleVar("__wisp_s_${"+id+"}_code", litAtom("0"))
	// _cause holds the inner error's handle id (a positive decimal int, inert).
	g.setHandleVar("__wisp_s_${"+id+"}_cause", varAtom(inner.name))
	return varAtom(id)
}

// genCause lowers cause(err): read __wisp_s_<id>_cause; if non-empty return
// Some(inner_id) else return None. The Optional handle is built inline (like
// genIntSentinelToOptional). cause never aborts.
func (g *gen) genCause(args []ast.Expr) atom {
	e := g.genExpr(args[0])
	causeVal := g.readHandleVar("__wisp_s_${" + e.name + "}_cause")
	out := g.allocHandle()
	g.line("if [ -n \"$%s\" ]; then", causeVal.name)
	g.indent++
	g.setHandleVar(tagFieldName(out), litAtom("some"))
	g.setHandleVar(tagValueName(out), varAtom(causeVal.name))
	g.indent--
	g.line("else")
	g.indent++
	g.setHandleVar(tagFieldName(out), litAtom("none"))
	g.indent--
	g.line("fi")
	return varAtom(out)
}

// genThrow lowers `throw <expr>`: evaluate the error handle, read its .message
// and .code, set __wisp_err_code for the catch side, and invoke the throw path
// (__wisp_throw <pos> <msg>), which at depth>0 sets pending + returns and at
// depth 0 aborts located.
func (g *gen) genThrow(n *ast.ThrowStmt) {
	e := g.genExpr(n.X)
	msg := g.readHandleVar("__wisp_s_${" + e.name + "}_message")
	code := g.readHandleVar("__wisp_s_${" + e.name + "}_code")
	g.line("__wisp_err_code=%s", g.word(code))
	// Propagate the thrown error's cause UNCONDITIONALLY -- including the empty
	// string for a non-wrapped throw -- exactly as __wisp_err_code is written
	// unconditionally above. The empty write prevents a prior in-flight wrapped
	// error's cause from leaking onto a later plain throw (AC4a). Gated on
	// usesWrap so a program without `wrap` is byte-identical (AC6).
	if g.usesWrap {
		cause := g.readHandleVar("__wisp_s_${" + e.name + "}_cause")
		g.line("__wisp_err_cause=%s", g.word(cause))
	}
	g.use(runtime.Throw)
	g.line("__wisp_throw %s %s", g.posLiteral(n.KwPos), g.word(msg))
}

// genTry lowers a try/catch/finally construct, emitting the spec section 4.2
// skeleton. All scaffolding is UNGUARDED; only the body, handler, and finally
// STATEMENTS carry per-statement skip-guards.
func (g *gen) genTry(n *ast.TryStmt) {
	g.tryID++
	id := g.tryID
	sp := tryVar("sp", id)
	sm := tryVar("sm", id)
	sc := tryVar("sc", id)
	ss := tryVar("ss", id)
	sx := tryVar("sx", id) // body save-slot for __wisp_err_cause (usesWrap only)

	// Save outer in-flight error; clear pending; enter the try (depth++). The
	// body save-slot captures ALL FOUR error variables -- pending (sp), msg (sm),
	// code (sc), pos (ss) -- so an outer error suspended across this try is
	// restored intact. Saving only pending+msg (the old shape) left a restored
	// outer error with the WRONG code (H6: a try whose body/handler raised its own
	// error overwrote __wisp_err_code) and an unlocated uncaught abort (M4: the
	// throw position in __wisp_err_pos was lost).
	g.line("%s=\"$__wisp_err_pending\"; %s=\"$__wisp_err_msg\"", sp, sm)
	g.line("%s=\"$__wisp_err_code\"; %s=\"$__wisp_err_pos\"", sc, ss)
	// Save __wisp_err_cause across the body too (parallel to _code), so an outer
	// pending wrapped error suspended across this try is restored with ITS OWN
	// cause, never the inner try's and never empty (AC4b). Gated on usesWrap.
	if g.usesWrap {
		g.line("%s=\"$__wisp_err_cause\"", sx)
	}
	g.line("__wisp_err_pending=")
	g.line("__wisp_try_depth=$(( __wisp_try_depth + 1 ))")

	// Guarded body.
	g.pushScope()
	g.genBlock(n.Body)
	g.popScope()

	// Handler: runs iff a fault is pending. Bind e (a fresh error handle whose
	// .message is __wisp_err_msg), clear pending FIRST (a handler fault is not
	// self-caught), then run the guarded handler.
	g.line("if [ -n \"$__wisp_err_pending\" ]; then")
	g.indent++
	g.pushScope()
	g.bindCatchVar(n)
	g.line("__wisp_err_pending=")
	g.genBlock(n.Catch)
	g.popScope()
	g.indent--
	g.line("fi")

	// Finally (optional) runs with the post-handler error SUSPENDED: save+clear
	// pending around the finally body so its own guarded statements run; a finally
	// fault wins (its pending/msg/code/pos stay live), else the suspended error is
	// restored. The finally save-slot also carries all four variables (fp/fm +
	// code fc + pos fs) so restoring after a finally that threw-and-caught its own
	// error reinstates the ORIGINAL code and position, not the finally error's
	// (H6/M4): without fc/fs the restored error kept its message but inherited the
	// finally error's code, and an uncaught restore lost its location.
	if n.HasFinally {
		fp := tryVar("fp", id)
		fm := tryVar("fm", id)
		fc := tryVar("fc", id)
		fs := tryVar("fs", id)
		fx := tryVar("fx", id) // finally save-slot for __wisp_err_cause (usesWrap only)
		g.line("%s=\"$__wisp_err_pending\"; %s=\"$__wisp_err_msg\"", fp, fm)
		g.line("%s=\"$__wisp_err_code\"; %s=\"$__wisp_err_pos\"", fc, fs)
		if g.usesWrap {
			g.line("%s=\"$__wisp_err_cause\"", fx)
		}
		g.line("__wisp_err_pending=")
		g.pushScope()
		g.genBlock(n.Finally)
		g.popScope()
		if g.usesWrap {
			g.line("if [ -z \"$__wisp_err_pending\" ]; then __wisp_err_pending=\"$%s\"; __wisp_err_msg=\"$%s\"; __wisp_err_code=\"$%s\"; __wisp_err_pos=\"$%s\"; __wisp_err_cause=\"$%s\"; fi", fp, fm, fc, fs, fx)
		} else {
			g.line("if [ -z \"$__wisp_err_pending\" ]; then __wisp_err_pending=\"$%s\"; __wisp_err_msg=\"$%s\"; __wisp_err_code=\"$%s\"; __wisp_err_pos=\"$%s\"; fi", fp, fm, fc, fs)
		}
	}

	// Leave the try (depth--), then the epilogue. An UNCAUGHT error that reaches
	// depth 0 aborts located (M4): __wisp_err_pos is the throw position for a
	// thrown error (empty for a fault, whose location is already inside the msg),
	// so the print reproduces the depth-0 "wisp: <pos>: <msg>" form when a position
	// is present and "wisp: <msg>" otherwise -- a fault's msg already begins with
	// its "<pos>: " prefix, so either branch is located.
	g.line("__wisp_try_depth=$(( __wisp_try_depth - 1 ))")
	g.line("if [ -n \"$__wisp_err_pending\" ]; then")
	g.indent++
	g.line("if [ \"$__wisp_try_depth\" -eq 0 ]; then")
	g.indent++
	g.line("%s", "if [ -n \"$__wisp_err_pos\" ]; then printf 'wisp: %s: %s\\n' \"$__wisp_err_pos\" \"$__wisp_err_msg\" >&2; else printf 'wisp: %s\\n' \"$__wisp_err_msg\" >&2; fi")
	g.line("exit 1")
	g.indent--
	g.line("fi")
	g.indent--
	g.line("else")
	g.indent++
	g.line("__wisp_err_pending=\"$%s\"; __wisp_err_msg=\"$%s\"", sp, sm)
	g.line("__wisp_err_code=\"$%s\"; __wisp_err_pos=\"$%s\"", sc, ss)
	if g.usesWrap {
		g.line("__wisp_err_cause=\"$%s\"", sx)
	}
	g.indent--
	g.line("fi")
}

// bindCatchVar binds the catch variable e to a fresh error handle whose message
// field is the in-flight __wisp_err_msg and whose code field is __wisp_err_code
// (defaulting to 0 via :-0 for faults from __wisp_fail which don't set it).
func (g *gen) bindCatchVar(n *ast.TryStmt) {
	v := g.info.CatchVars[n]
	if v == nil {
		return // blank catch: error is caught, handler runs, no error var bound
	}
	g.declareVar(v)
	id := g.allocHandle()
	g.setHandleVar("__wisp_s_${"+id+"}_message", varAtom("__wisp_err_msg"))
	codeT := g.newTemp()
	g.line("%s=\"${__wisp_err_code:-0}\"", codeT)
	g.setHandleVar("__wisp_s_${"+id+"}_code", varAtom(codeT))
	// Restore the in-flight cause onto the freshly-bound handle (the original
	// inner error's handle id, or empty for a non-wrapped error), so a caught
	// wrapped error answers cause(e) with the original inner error -- deeper
	// chain intact -- and a non-wrapped one answers None. Gated on usesWrap.
	if g.usesWrap {
		g.setHandleVar("__wisp_s_${"+id+"}_cause", varAtom("__wisp_err_cause"))
	}
	g.line("%s=\"$%s\"", v.Mangled, id)
}

// tryVar builds a unique per-try save-slot variable name, e.g. __wisp_sp_3.
func tryVar(kind string, id int) string {
	return "__wisp_" + kind + "_" + strconv.Itoa(id)
}
