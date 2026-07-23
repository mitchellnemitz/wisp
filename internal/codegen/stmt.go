package codegen

import (
	"fmt"
	"strconv"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/token"
	"github.com/mitchellnemitz/wisp/internal/types"
)

// coverableStmt reports whether a statement is a coverable line: every
// statement EXCEPT a const declaration, which emits no runtime code (the folded
// value is inlined at every use site, so there is nothing to mark hit). This is
// THE coverable-statement predicate; genStmt uses it to decide whether to emit a
// hit-marker, and walkCoverableStmts uses the SAME predicate to enumerate the
// coverage universe, so the universe is a superset of the emitted markers by
// construction.
func coverableStmt(s ast.Stmt) bool {
	_, isConst := s.(*ast.ConstStmt)
	return !isConst
}

// walkCoverableStmts visits every statement in body and, recursively, in every
// block a statement can contain, calling visit(s.Pos()) for each statement where
// coverableStmt is true and the position is real (a synthesized statement with a
// zero position is skipped, mirroring the marker emitter's curPos guard). The
// recursion mirrors genStmtInner exactly: it descends into IfStmt
// Then/ElseIfs/Else, WhileStmt/ForStmt/ForInStmt bodies (plus ForStmt Init/Post,
// which are themselves statements genStmt lowers), SwitchStmt cases + default,
// MatchStmt arm bodies, and TryStmt Body/Catch/Finally.
func walkCoverableStmts(body []ast.Stmt, visit func(pos token.Position)) {
	for _, s := range body {
		if s == nil {
			continue
		}
		if coverableStmt(s) {
			if p := s.Pos(); p.File != "" || p.Line != 0 {
				visit(p)
			}
		}
		switch n := s.(type) {
		case *ast.IfStmt:
			walkCoverableStmts(n.Then, visit)
			for _, ei := range n.ElseIfs {
				walkCoverableStmts(ei.Body, visit)
			}
			walkCoverableStmts(n.Else, visit)
		case *ast.WhileStmt:
			walkCoverableStmts(n.Body, visit)
		case *ast.ForStmt:
			walkCoverableStmts([]ast.Stmt{n.Init, n.Post}, visit)
			walkCoverableStmts(n.Body, visit)
		case *ast.ForInStmt:
			walkCoverableStmts(n.Body, visit)
		case *ast.SwitchStmt:
			for _, cs := range n.Cases {
				walkCoverableStmts(cs.Body, visit)
			}
			walkCoverableStmts(n.Default, visit)
		case *ast.MatchStmt:
			for _, arm := range n.Arms {
				walkCoverableStmts(arm.Body, visit)
			}
		case *ast.TryStmt:
			walkCoverableStmts(n.Body, visit)
			walkCoverableStmts(n.Catch, visit)
			walkCoverableStmts(n.Finally, visit)
		}
	}
}

func (g *gen) genStmt(s ast.Stmt) {
	// Every line emitted while lowering s attributes to s's source position
	// (spec section 3.3): user-statement lines map to the construct, and the
	// lowering-scaffold lines a statement injects (loop wrappers, the once-wrapper,
	// per-iteration predicate re-emission, break/break 2) map to the nearest
	// causing user statement -- here, the enclosing statement -- which is the
	// best-effort attribution the spec specifies for injected control transfers.
	// curPos is restored afterward so a parent's trailing scaffold lines (else/fi/
	// done) re-attribute to the parent, not to the last nested statement.
	prev := g.curPos
	p := s.Pos()
	g.curPos = &p
	// Per-statement skip-guard (M5): in errMode each statement is wrapped in
	// `if [ -z "$__wisp_err_pending" ]; then <stmt> fi`, so once a fault is
	// pending subsequent statements are skipped (no shell loop is introduced, so
	// break/continue counts inside the statement are unaffected). Nested guards
	// opened mid-statement (after a call spill) also close here. Outside errMode
	// this is a no-op and the output is the M4 shape.
	base := g.guardDepth
	g.openGuard()
	// Coverage hit-marker (spec R15-R17): record that this statement was reached.
	// It sits inside the per-statement skip-guard, so a statement skipped after a
	// pending fault is correctly NOT marked hit, and it attributes to the
	// statement's own position. A no-op unless coverage mode is on. ConstStmt emits
	// no runtime code, so it is not a coverable line; skip its marker.
	if coverableStmt(s) {
		g.emitCoverMarker()
	}
	g.genStmtInner(s)
	g.closeGuardsTo(base)
	g.curPos = prev
}

func (g *gen) genStmtInner(s ast.Stmt) {
	switch n := s.(type) {
	case *ast.LetStmt:
		g.genLet(n)
	case *ast.AssignStmt:
		g.genAssign(n)
	case *ast.FieldAssignStmt:
		g.genFieldAssign(n)
	case *ast.IndexAssignStmt:
		g.genIndexAssign(n)
	case *ast.ReturnStmt:
		g.genReturn(n)
	case *ast.IfStmt:
		g.genIf(n)
	case *ast.WhileStmt:
		g.genWhile(n)
	case *ast.ForStmt:
		g.genFor(n)
	case *ast.ForInStmt:
		g.genForIn(n)
	case *ast.SwitchStmt:
		g.genSwitch(n)
	case *ast.BreakStmt:
		g.genBreak()
	case *ast.ContinueStmt:
		g.genContinue()
	case *ast.ThrowStmt:
		g.genThrow(n)
	case *ast.TryStmt:
		g.genTry(n)
	case *ast.MatchStmt:
		g.genMatchStmt(n)
	case *ast.ConstStmt:
		// Const declarations emit no runtime code: the folded value is inlined at
		// every use site by genIdent. The Var is absent from FuncInfo.Decls so
		// emitLocalDecl already emits no 'local'.
	case *ast.FinalStmt:
		g.genFinal(n)
	case *ast.TupleBindStmt:
		g.genTupleBind(n)
	case *ast.ExprStmt:
		// Only a call is valid here; evaluate it for its effect. A non-void
		// result is computed and discarded.
		g.genExpr(n.X)
	}
}

func (g *gen) genLet(n *ast.LetStmt) {
	if n.Name == "_" {
		g.genExpr(n.Value) // blank: evaluate for side effects, discard the word
		return
	}
	v := g.info.Vars[n]
	// Evaluate the initializer before the name is in scope, matching wisp
	// semantics (the initializer cannot reference the variable).
	a := g.genExpr(n.Value)
	g.line("%s=%s", v.Mangled, g.word(a))
	g.declareVar(v)
}

// genFinal lowers a `final NAME: T = expr` statement. final is a runtime-
// immutable local: it emits exactly like let (RHS into the mangled name). The
// 'local' declaration is already emitted by emitLocalDecl because the final Var
// is in FuncInfo.Decls. The Var is looked up via info.FinalVars.
func (g *gen) genFinal(n *ast.FinalStmt) {
	if n.Name == "_" {
		g.genExpr(n.Value) // blank: evaluate for side effects, discard
		return
	}
	v := g.info.FinalVars[n]
	a := g.genExpr(n.Value)
	g.line("%s=%s", v.Mangled, g.word(a))
	g.declareVar(v)
}

// genTupleBind lowers a tuple-destructuring `let`/`final` (spec R4): evaluate the
// RHS EXACTLY ONCE and spill the resulting tuple-handle atom to a stable temp
// (the established spillToTemp pattern -- NOT genTupleIndex's per-index genExpr,
// which would re-evaluate a call per element). Then bind each non-`_` slot's
// mangled name to its element handle var (__wisp_s_${id}_<i>) via an inert
// deferred-expansion eval (mirrors the tuple-element reads, injection-safe).
//
// guardAfterSpill opens a nested skip-guard right after the spill so a faulting
// RHS propagates BEFORE any binding -- the binds land inside the per-statement
// skip-guard, identical to a normal `let x = fallibleCall()`. An all-discard
// pattern still performs the single RHS evaluation (for effects) and binds
// nothing, exactly like `let _ = f()`.
//
// The bound Vars are NOT in any info map; they live only in curFI.Decls (the
// checker's append). Resolve each by its slot position.
func (g *gen) genTupleBind(n *ast.TupleBindStmt) {
	id := g.spillToTemp(g.genExpr(n.Value)) // evaluate once, spill the handle
	g.guardAfterSpill()                     // fault-before-binding
	for i := range n.Slots {
		s := &n.Slots[i]
		if s.Blank {
			continue // discard slot: read nothing, bind nothing
		}
		v := g.tupleSlotVar(s)
		// Bind: copy element i's backing var into the mangled name with a single,
		// inert deferred expansion (\$src inside double quotes expands once under
		// eval; no globbing, no re-evaluation). The `local` for v.Mangled is emitted
		// by emitLocalDecl because the checker appended v to curFI.Decls.
		g.line("eval \"%s=\\$__wisp_s_${%s}_%s\"", v.Mangled, id, strconv.Itoa(i))
		g.declareVar(v)
	}
}

// tupleSlotVar resolves the *types.Var the checker created for a non-`_`
// destructuring slot. The Vars are appended to curFI.Decls with Var.Pos set to
// the slot position, so the slot's recorded position uniquely identifies it.
func (g *gen) tupleSlotVar(s *ast.TupleBindSlot) *types.Var {
	for _, d := range g.curFI.Decls {
		if d.Name == s.Name && d.Pos == s.Pos {
			return d
		}
	}
	return nil // unreachable for a checked program (checker appended every binding slot)
}

func (g *gen) genAssign(n *ast.AssignStmt) {
	if n.Name == "_" {
		g.genExpr(n.Value) // blank: evaluate for side effects, no assignment
		return
	}
	// AssignStmt targets are not in Info.Uses, so resolve the in-scope binding via
	// the codegen scope stack (which mirrors the checker's). Block scoping
	// guarantees exactly one visible binding for a checked program.
	v := g.resolveVar(n.Name)
	a := g.genExpr(n.Value)
	g.line("%s=%s", v.Mangled, g.word(a))
}

func (g *gen) genReturn(n *ast.ReturnStmt) {
	if n.Value == nil {
		// void return: leave __ret untouched (callers of void functions ignore it)
		// and return from the shell function.
		g.line("return")
		return
	}
	a := g.genExpr(n.Value)
	g.line("__ret=%s", g.word(a))
	g.line("return")
}

func (g *gen) genIf(n *ast.IfStmt) {
	c := g.genCond(n.Cond)
	g.line("if [ \"$%s\" = true ]; then", c)
	g.indent++
	g.pushScope()
	g.genBlock(n.Then)
	g.popScope()
	g.indent--
	for _, ei := range n.ElseIfs {
		// The elif predicate must be evaluated only when reached, so its
		// value-producing code is emitted inside an else block, not before the if.
		g.line("else")
		g.indent++
		ec := g.genCond(ei.Cond)
		g.line("if [ \"$%s\" = true ]; then", ec)
		g.indent++
		g.pushScope()
		g.genBlock(ei.Body)
		g.popScope()
		g.indent--
		// close handled after the loop with matching fi's
	}
	if n.Else != nil {
		g.line("else")
		g.indent++
		g.pushScope()
		g.genBlock(n.Else)
		g.popScope()
		g.indent--
	}
	// close the nested elif if-blocks (each opened an extra `if`/indent inside an
	// `else`).
	for range n.ElseIfs {
		g.line("fi")
		g.indent--
	}
	g.line("fi")
}

func (g *gen) genWhile(n *ast.WhileStmt) {
	// Recompute the predicate at the top of each iteration (spec section 9.4):
	// loop forever, evaluate the predicate into a bool temp, and break when it is
	// not true.
	g.line("while :; do")
	g.indent++
	g.shellDepth++
	g.loopPendingBreak()
	c := g.genCond(n.Cond)
	g.line("if [ \"$%s\" != true ]; then break; fi", c)
	g.loops = append(g.loops, loopCtx{isFor: false})
	g.pushScope()
	g.genBlock(n.Body)
	g.popScope()
	g.loops = g.loops[:len(g.loops)-1]
	g.shellDepth--
	g.indent--
	g.line("done")
}

// genFor lowers `for (init; cond; post) { body }`.
//
// Break/continue mechanism (spec section 9.4): the loop is lowered to
//
//	<init>
//	while :; do
//	  <cond eval>; if not true: break
//	  for __wisp_once in 0; do   # single-iteration wrapper
//	    <body>
//	  done
//	  <post>
//	done
//
// A wisp `continue` becomes `break` (count 1): it exits the once-wrapper, so
// <post> still runs and the condition is re-tested. A wisp `break` becomes
// `break 2`: it exits both the once-wrapper and the real `while`, leaving the
// loop entirely. Because shell `break N`/`continue` count from the innermost
// enclosing shell loop outward, and the nearest wisp loop's own wrappers are
// always the innermost shell loops at the binding point, these counts are a
// function of the target loop's own shape only -- they stay correct for loops
// nested inside the `for` (a break/continue there binds to the inner loop and
// uses that inner loop's counts). For a `while`, the body is directly inside one
// shell loop, so its break/continue are the bare `break`/`continue`.
func (g *gen) genFor(n *ast.ForStmt) {
	// The for-init declares into the loop's own scope, visible to cond/post/body.
	g.pushScope()
	if n.Init != nil {
		g.genStmt(n.Init)
	}
	g.line("while :; do")
	g.indent++
	g.shellDepth++
	g.loopPendingBreak()
	if n.Cond != nil {
		c := g.genCond(n.Cond)
		g.line("if [ \"$%s\" != true ]; then break; fi", c)
	}
	// Unique once-wrapper index per for-loop so nested wrappers do not reuse a
	// loop variable (avoids shellcheck SC2165/SC2167).
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
	if n.Post != nil {
		g.genStmt(n.Post)
	}
	g.shellDepth--
	g.indent--
	g.line("done")
	g.popScope()
}

func (g *gen) genBreak() {
	l := g.loops[len(g.loops)-1]
	if l.isFor {
		g.line("break 2")
	} else {
		g.line("break")
	}
}

func (g *gen) genContinue() {
	l := g.loops[len(g.loops)-1]
	if l.isFor {
		// exit the once-wrapper so <post> runs, then the while re-tests.
		g.line("break")
	} else {
		g.line("continue")
	}
}

func (g *gen) genSwitch(n *ast.SwitchStmt) {
	if g.switchSubjectIsFloat(n) {
		g.genFloatSwitch(n)
		return
	}
	subj := g.genExpr(n.Subject)
	g.line("case %s in", g.word(subj))
	g.indent++
	for _, cs := range n.Cases {
		pats := make([]string, len(cs.Values))
		for i, v := range cs.Values {
			pats[i] = g.casePattern(v)
		}
		g.line("%s)", joinPats(pats))
		g.indent++
		g.pushScope()
		g.genBlock(cs.Body)
		g.popScope()
		g.line(";;")
		g.indent--
	}
	// Always emit a `*)` arm (may be empty). For int/string switches the checker
	// requires a default; for an exhaustive defaultless enum switch the empty `*)`
	// is unreachable but keeps the lowering valid POSIX (R5/R6).
	g.line("*)")
	g.indent++
	g.pushScope()
	if n.Default != nil {
		g.genBlock(n.Default)
	}
	g.popScope()
	g.line(";;")
	g.indent--
	g.indent--
	g.line("esac")
}

// switchSubjectIsFloat reports whether the switch subject compares as a float:
// a plain float, or a float-backed value enum (whose runtime value IS its
// backing float text). Both share the numeric if/elif lowering (FR-010). It
// delegates to comparesAsFloat so float-or-float-backed-enum has one
// definition across membership, assert, Optional, and switch.
func (g *gen) switchSubjectIsFloat(n *ast.SwitchStmt) bool {
	return g.comparesAsFloat(g.info.Types[n.Subject])
}

// genFloatSwitch lowers a float (or float-backed-enum) switch to an if/elif
// chain over __wisp_fcmp: a `case ... esac` glob cannot express numeric identity
// or -0.0 == 0.0. The subject is spilled once (single evaluation of a
// side-effecting subject, SC-026); each case's fcmp guards are emitted lazily
// inside the preceding `else` (mirroring genIf), so a later case is only tested
// when no earlier case matched. Cases[0] is the leading `if`; Cases[1..] are
// `else { if ... }`; the default is the trailing `else`.
func (g *gen) genFloatSwitch(n *ast.SwitchStmt) {
	subj := varAtom(g.spillToTemp(g.genExpr(n.Subject)))

	emitCase := func(cs ast.SwitchCase) {
		matched := g.newTemp()
		g.line("%s=false", matched)
		for _, v := range cs.Values {
			cv := g.genExpr(v)
			eq := g.emitFloatCompare("eq", subj, cv, v.Pos())
			g.line("if [ \"$%s\" = true ]; then %s=true; fi", eq.name, matched)
		}
		g.line("if [ \"$%s\" = true ]; then", matched)
		g.indent++
		g.pushScope()
		before := g.out.Len()
		g.genBlock(cs.Body)
		if g.out.Len() == before {
			g.line(":")
		}
		g.popScope()
		g.indent--
	}

	// Cases[0] == leading `if`.
	emitCase(n.Cases[0])
	// Cases[1..] == `else { <fcmp guards> if ... }`.
	for i := 1; i < len(n.Cases); i++ {
		g.line("else")
		g.indent++
		emitCase(n.Cases[i])
	}
	// Default == trailing `else`.
	g.line("else")
	g.indent++
	g.pushScope()
	before := g.out.Len()
	if n.Default != nil {
		g.genBlock(n.Default)
	}
	if g.out.Len() == before {
		g.line(":")
	}
	g.popScope()
	g.indent--
	// Close: one `fi` per case's `if [matched]`, and dedent each `else` opened
	// for Cases[1..] (mirrors genIf's close loop).
	for i := len(n.Cases) - 1; i >= 1; i-- {
		g.line("fi")
		g.indent--
	}
	g.line("fi")
}

func joinPats(pats []string) string {
	out := ""
	for i, p := range pats {
		if i > 0 {
			out += "|"
		}
		out += p
	}
	return out
}

// genMatchStmt lowers `match (scrutinee) { arm... }`.
// The scrutinee is evaluated once and spilled. The tag field is read once
// before the if/elif structure so the eval lines land outside any arm body.
// A trailing wildcard arm becomes the shell else-clause.
func (g *gen) genMatchStmt(n *ast.MatchStmt) {
	id := g.spillToTemp(g.genExpr(n.Scrutinee))

	// Separate constructor arms from an optional trailing wildcard.
	var conArms []*ast.MatchArm
	var wildArm *ast.MatchArm
	for _, arm := range n.Arms {
		if _, ok := arm.Pattern.(*ast.WildcardPat); ok {
			wildArm = arm
		} else {
			conArms = append(conArms, arm)
		}
	}

	// Single wildcard covers all variants: just run the body.
	if len(conArms) == 0 && wildArm != nil {
		g.pushScope()
		g.genBlock(wildArm.Body)
		g.popScope()
		return
	}

	// Read the tag once before the if/elif block (same field for all arms).
	scrutType := g.info.Types[n.Scrutinee]
	isEnum := g.info.Enums[string(scrutType)] != nil
	firstPat := conArms[0].Pattern.(*ast.ConstructorPat)
	tagField, _ := g.matchTagField(id, firstPat.Variant, isEnum)
	tag := g.readHandleVar(tagField)

	for i, arm := range conArms {
		pat := arm.Pattern.(*ast.ConstructorPat)
		_, want := g.matchTagField(id, pat.Variant, isEnum)
		if i == 0 {
			g.line("if [ \"$%s\" = %s ]; then", tag.name, want)
		} else {
			g.line("elif [ \"$%s\" = %s ]; then", tag.name, want)
		}
		g.indent++
		g.pushScope()
		before := g.out.Len()
		if pat.Name != "" && pat.Name != "_" {
			v := g.info.MatchArmVars[arm]
			valField := matchValueField(id)
			g.line("%s=%s", v.Mangled, g.word(g.readHandleVar(valField)))
			g.declareVar(v)
		}
		g.genBlock(arm.Body)
		// A shell `then` clause may not be empty: an arm with no binding and an
		// empty body (e.g. `case None {}`) would otherwise emit `then` immediately
		// followed by `elif`/`fi`, a syntax error. Emit the `:` no-op instead.
		if g.out.Len() == before {
			g.line(":")
		}
		g.popScope()
		g.indent--
	}
	if wildArm != nil {
		g.line("else")
		g.indent++
		g.pushScope()
		before := g.out.Len()
		g.genBlock(wildArm.Body)
		if g.out.Len() == before { // empty `else` body, e.g. trailing `case _ {}`
			g.line(":")
		}
		g.popScope()
		g.indent--
	}
	g.line("fi")
}

// matchTagField returns the single shared tag field and the expected tag word for
// a variant. For an Optional/Result builtin the word is the lowercase tag; for a
// tagged-union enum it is the variant name verbatim (the compiler-emitted
// identifier literal, injection-safe by the lexer charset).
func (g *gen) matchTagField(id, variant string, isEnum bool) (tagField, want string) {
	if isEnum {
		return tagFieldName(id), variant
	}
	switch variant {
	case "Some":
		return tagFieldName(id), "some"
	case "None":
		return tagFieldName(id), "none"
	case "Ok":
		return tagFieldName(id), "ok"
	case "Err":
		return tagFieldName(id), "err"
	default:
		panic(fmt.Sprintf("matchTagField: no codegen case for variant %q (checker/codegen drift)", variant))
	}
}

// matchValueField returns the shell variable that holds the payload. The field is
// the same for every variant now that Optional and Result share one value field,
// so it does not depend on the variant.
func matchValueField(id string) string {
	return tagValueName(id)
}
