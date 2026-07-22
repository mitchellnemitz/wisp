package types

import (
	"sort"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/token"
)

// checkValueAgainst checks value against an expected type want at a blessed
// expected-type site (let-init / return / assignment). A None literal is
// concretized to want when want is a concrete Optional[...] (recorded in
// info.Types as want, never a sentinel); otherwise it is the spec-5.4 error. Any
// other value is checked bottom-up (preserving the empty-literal contextual
// threading of checkExprExpecting) and compared with plain ==.
func (c *checker) checkValueAgainst(value ast.Expr, want Type, onMismatch func(got Type)) {
	if isNoneLiteral(value) {
		if want == Invalid {
			// An earlier error already poisoned want; suppress the cascade like
			// every other mismatch check rather than adding an unactionable
			// noneNeedsContext on top of it.
			c.info.Types[value] = Invalid
			return
		}
		if isOptional(want) {
			c.info.Types[value] = want // concrete Optional[T]; codegen still lowers via the None node
			return
		}
		c.noneNeedsContext(value.Pos()) // want is non-Optional / Void
		c.info.Types[value] = Invalid
		return
	}
	if isErrCall(value) {
		ce := value.(*ast.CallExpr)
		if len(ce.Args) != 1 {
			c.errf(ce.CalleePos, "Err expects 1 argument, got %d", len(ce.Args))
			c.typeArgs(ce.Args)
			c.info.Types[value] = Invalid
			return
		}
		c.requireErrorArg(ce) // arg must be the built-in error type regardless of want
		if want == Invalid {
			c.info.Types[value] = Invalid // suppress cascade on an already-poisoned want
			return
		}
		if isResult(want) {
			// Concretize T from the expected Result[T]. Err is a CallExpr, so codegen
			// genCall dispatches on info.Calls[ce] -- record it here (the only place
			// Err is accepted with a concrete type).
			c.info.Types[value] = want
			c.info.Calls[ce] = &CallInfo{Kind: CallBuiltin, Builtin: "Err", Args: []ast.Expr{ce.Args[0]}, Result: want}
			return
		}
		c.errNeedsContext(value.Pos()) // want is non-Result / Void
		c.info.Types[value] = Invalid
		return
	}
	got := c.checkExprExpecting(value, want)
	if got != Invalid && want != Invalid && got != want {
		onMismatch(got)
	}
}

// checkBlock checks a sequence of statements in the current scope. Callers that
// introduce a new lexical block push/pop the scope themselves.
func (c *checker) checkBlock(stmts []ast.Stmt) {
	for _, s := range stmts {
		c.checkStmt(s)
	}
}

func (c *checker) checkStmt(s ast.Stmt) {
	switch n := s.(type) {
	case *ast.LetStmt:
		c.checkLet(n)
	case *ast.ConstStmt:
		c.checkConst(n)
	case *ast.FinalStmt:
		c.checkFinal(n)
	case *ast.TupleBindStmt:
		c.checkTupleBind(n)
	case *ast.AssignStmt:
		c.checkAssign(n)
	case *ast.FieldAssignStmt:
		c.checkFieldAssign(n)
	case *ast.IndexAssignStmt:
		c.checkIndexAssign(n)
	case *ast.ReturnStmt:
		c.checkReturn(n)
	case *ast.IfStmt:
		c.checkIf(n)
	case *ast.MatchStmt:
		c.checkMatch(n)
	case *ast.WhileStmt:
		c.checkWhile(n)
	case *ast.ForStmt:
		c.checkFor(n)
	case *ast.ForInStmt:
		c.checkForIn(n)
	case *ast.SwitchStmt:
		c.checkSwitch(n)
	case *ast.BreakStmt:
		if c.loopDepth == 0 {
			c.errf(n.KwPos, "break outside a loop")
		} else if c.crossesTry() {
			c.errf(n.KwPos, "break inside a try/catch/finally body is not allowed")
		}
	case *ast.ContinueStmt:
		if c.loopDepth == 0 {
			c.errf(n.KwPos, "continue outside a loop")
		} else if c.crossesTry() {
			c.errf(n.KwPos, "continue inside a try/catch/finally body is not allowed")
		}
	case *ast.ThrowStmt:
		c.checkThrow(n)
	case *ast.TryStmt:
		c.checkTry(n)
	case *ast.ExprStmt:
		// `exit` inside a try/catch/finally body is a compile error (spec 2.4),
		// reusing the M5 in-try restriction that rejects return/break/continue.
		// The rule is lexical: only a syntactic exit call inside a try body is
		// rejected (exit reached through a called function still terminates). The
		// check runs before typing so the message is the one users see.
		if c.tryDepth > 0 {
			if call, ok := n.X.(*ast.CallExpr); ok && call.CalleeName == "exit" {
				c.errf(call.CalleePos, "exit inside a try/catch/finally body is not allowed")
			}
		}
		// At statement level only a call is valid (parser already restricts the
		// useful cases); just type it so codegen has its type.
		c.checkExpr(n.X)
	}
}

// crossesTry reports whether a break/continue targeting the innermost enclosing
// loop would cross a try boundary: the loop was opened OUTSIDE the current try
// (its recorded tryDepth is less than the current tryDepth). A loop fully inside
// the try keeps its own break/continue legal.
func (c *checker) crossesTry() bool {
	if c.tryDepth == 0 || len(c.loopTryDepth) == 0 {
		return false
	}
	return c.loopTryDepth[len(c.loopTryDepth)-1] < c.tryDepth
}

// enterLoop / exitLoop bracket a loop body, recording the tryDepth at entry so a
// break/continue can tell whether it would escape an enclosing try (M5).
func (c *checker) enterLoop() {
	c.loopDepth++
	c.loopTryDepth = append(c.loopTryDepth, c.tryDepth)
}

func (c *checker) exitLoop() {
	c.loopDepth--
	c.loopTryDepth = c.loopTryDepth[:len(c.loopTryDepth)-1]
}

func (c *checker) checkLet(n *ast.LetStmt) {
	want := c.resolveType(n.Type, n.KwPos) // annotation required by the grammar; never void here
	c.checkValueAgainst(n.Value, want, func(got Type) {
		c.errf(n.Value.Pos(), "initializer of %q has type %s, want %s", n.Name, got, want)
	})
	if n.Name == "_" {
		return // blank: RHS checked above; no binding created
	}
	c.declare(n, want)
}

// checkConst type-checks a local `const NAME: Type = <const-expr>` statement.
// The value must be a constant expression (foldable at compile time). If Name
// is "_", the value is folded for side-effects but no binding is created.
// Otherwise, the name is declared into the current scope with IsConst set, and
// the binding is recorded in info.ConstVars. No unused warning is emitted for
// const bindings (they may be declared for documentation or future use).
func (c *checker) checkConst(n *ast.ConstStmt) {
	annType := c.resolveType(n.Type, n.KwPos)
	foldedType := c.checkConstExpr(n.Value)
	if foldedType == Invalid {
		// Fold failed; an error has already been emitted. Do not declare the
		// binding: a nil-valued const in scope would make a later const that
		// references it panic when the resolver asserts the folded value.
		return
	}
	if annType != Invalid && foldedType != annType {
		c.errf(n.NamePos, "const %q: type mismatch: declared %s but initializer has type %s",
			n.Name, annType, foldedType)
		return
	}
	if n.Name == "_" {
		return // blank: value folded above; no binding created
	}
	c.declareConst(n, annType)
}

// declareConst adds a const binding to the innermost scope, enforcing the same
// no-redeclaration and no-shadowing rules as let. The Var has IsConst set and
// Used pre-set to true (no unused warning). The Var is NOT added to
// curFunc.Decls; codegen inlines the folded value at use sites and emits no
// runtime `local` for const bindings. The binding is added to the current
// scope so a subsequent const-expr in the same body can resolve it (and so
// no-shadowing is enforced), and recorded in info.ConstVars for LSP navigation.
func (c *checker) declareConst(n *ast.ConstStmt, annType Type) {
	if isReservedIdent(n.Name) {
		c.errf(n.KwPos, "%q uses the reserved \"__\" namespace and cannot be a constant name", n.Name)
	} else if isReservedName(n.Name) {
		c.errf(n.KwPos, "%q is a reserved builtin or constant name and cannot be a constant name", n.Name)
	}
	if prev := c.lookup(n.Name); prev != nil {
		c.errf(n.KwPos, "%q is already declared in this or an enclosing scope (no shadowing); previous at %s", n.Name, prev.Pos)
		return
	}
	if tv := c.cur.topConsts[n.Name]; tv != nil {
		c.errf(n.KwPos, "%q is already declared in this or an enclosing scope (no shadowing); previous at %s", n.Name, tv.Pos)
		return
	}
	if _, ok := c.cur.funcs[n.Name]; ok {
		c.errf(n.KwPos, "%q is a declared function and cannot be shadowed by a constant", n.Name)
		return
	}
	fv := c.info.FoldedValues[n.Value]
	v := &Var{
		Name: n.Name,
		// Use the resolved annotation type, not the inferred folded type. They are
		// equal for a well-typed const; when the annotation fails to resolve
		// (annType == Invalid) the const stays Invalid rather than silently
		// becoming an inferred binding, consistent with let/final.
		Type:        annType,
		Pos:         n.NamePos,
		IsConst:     true,
		FoldedValue: fv,
		Used:        true, // consts are exempt from the unused-variable warning
	}
	// Insert into the current scope: this both enforces no-shadowing and lets
	// the const resolver find the binding (with its FoldedValue) for a later
	// const-expr in this body. Scope membership keeps resolution lexically
	// correct -- the binding is gone once this block's scope pops.
	c.scopes[len(c.scopes)-1][n.Name] = v
	c.info.ConstVars[n] = v
}

// checkFinal type-checks a function-body `final NAME: Type = <expr>` statement.
// final is a runtime-immutable local: it behaves like let for scoping, Decls
// membership, and the unused-variable warning, but reassignment is a compile
// error. The annotation is mandatory (enforced by the parser). If Name is "_",
// the RHS is checked for type errors but no binding is created.
func (c *checker) checkFinal(n *ast.FinalStmt) {
	want := c.resolveType(n.Type, n.KwPos)
	c.checkValueAgainst(n.Value, want, func(got Type) {
		c.errf(n.Value.Pos(), "initializer of %q has type %s, want %s", n.Name, got, want)
	})
	if n.Name == "_" {
		return // blank: RHS checked above; no binding created
	}
	c.declareFinal(n, want)
}

// declareFinal adds a final binding to the innermost scope, enforcing the same
// no-redeclaration and no-shadowing rules as let. The Var has Immutable set
// and is added to curFunc.Decls so codegen emits a runtime `local` (like let).
// The binding is also recorded in info.FinalVars, keyed by the FinalStmt node,
// so codegen's genFinal can resolve it back to this Var.
func (c *checker) declareFinal(n *ast.FinalStmt, t Type) {
	if isReservedIdent(n.Name) {
		c.errf(n.KwPos, "%q uses the reserved \"__\" namespace and cannot be a variable name", n.Name)
	} else if isReservedName(n.Name) {
		c.errf(n.KwPos, "%q is a reserved builtin or constant name and cannot be a variable name", n.Name)
	}
	if prev := c.lookup(n.Name); prev != nil {
		c.errf(n.KwPos, "%q is already declared in this or an enclosing scope (no shadowing); previous at %s", n.Name, prev.Pos)
		return
	}
	if tv := c.cur.topConsts[n.Name]; tv != nil {
		c.errf(n.KwPos, "%q is already declared in this or an enclosing scope (no shadowing); previous at %s", n.Name, tv.Pos)
		return
	}
	if _, ok := c.cur.funcs[n.Name]; ok {
		c.errf(n.KwPos, "%q is a declared function and cannot be shadowed by a variable", n.Name)
		return
	}
	v := &Var{
		Name:      n.Name,
		Mangled:   c.mangleVar(),
		Type:      t,
		Pos:       n.NamePos,
		Immutable: true,
	}
	c.scopes[len(c.scopes)-1][n.Name] = v
	c.curFunc.Decls = append(c.curFunc.Decls, v)
	c.info.FinalVars[n] = v
}

// checkTupleBind type-checks a tuple-destructuring `let`/`final` binding
// `let (a: T1, b: T2, ...) = <expr>` (spec R3/R6). The RHS must be a tuple of
// arity k = len(Slots). Each binding slot's annotation (and each annotated
// discard's type) must equal the corresponding element type; a bare `_` slot
// imposes no constraint. Non-`_` slots bind in the current scope at the SLOT's
// position with the keyword's mutability.
//
// Bindings are NOT registered in info.Vars/info.FinalVars (those are keyed by
// the single-binding *ast.LetStmt/*ast.FinalStmt nodes). The ONLY path the
// destructured Vars reach codegen (`local` emission) and the LSP (go-to-def) is
// the curFunc.Decls append performed by declareTupleSlot. Slot-level diagnostics
// (type mismatch, duplicate name, reserved name, shadowing) are emitted at the
// slot's recorded position, not at the keyword.
func (c *checker) checkTupleBind(n *ast.TupleBindStmt) {
	// Resolve each slot's declared type first (bare `_` has none). Resolution at
	// the slot position so an unknown/void element type errors at the slot.
	slotTypes := make([]Type, len(n.Slots))
	for i := range n.Slots {
		s := &n.Slots[i]
		if s.Type == "" {
			slotTypes[i] = Invalid // bare `_`: no constraint
			continue
		}
		slotTypes[i] = c.resolveType(s.Type, s.Pos)
	}

	// Type the RHS. It must be a tuple of arity k.
	rhs := c.checkExpr(n.Value)
	var elems []Type
	if rhs != Invalid {
		if !IsTuple(rhs) {
			c.errf(n.Value.Pos(), "right-hand side of a destructuring binding must be a tuple, got %s", rhs)
		} else {
			elems = TupleElemTypes(rhs)
			if len(elems) != len(n.Slots) {
				c.errf(n.Value.Pos(), "destructuring arity mismatch: pattern has %d slots but %s has arity %d", len(n.Slots), rhs, len(elems))
				elems = nil // do not also stack per-slot mismatches on an arity error
			}
		}
	}

	// seen tracks non-`_` names already bound by THIS pattern, for duplicate
	// detection at the offending slot.
	seen := map[string]token.Position{}
	for i := range n.Slots {
		s := &n.Slots[i]
		// Element-type check: a binding or an annotated discard must equal element i.
		if s.Type != "" && elems != nil && slotTypes[i] != Invalid {
			if slotTypes[i] != elems[i] {
				c.errf(s.Pos, "slot %d has type %s but element %d of %s has type %s", i, slotTypes[i], i, rhs, elems[i])
			}
		}
		if s.Blank {
			continue // discard: no binding, no duplicate check (`_` is not a name)
		}
		if prev, dup := seen[s.Name]; dup {
			c.errf(s.Pos, "duplicate name %q in destructuring pattern; previous slot at %s", s.Name, prev)
			continue
		}
		seen[s.Name] = s.Pos
		c.declareTupleSlot(n, s, slotTypes[i])
	}
}

// declareTupleSlot binds one non-`_` destructuring slot into the innermost scope
// at the SLOT's position, enforcing the same reserved-name and no-shadowing
// rules as declare/declareFinal but reporting at the slot (declare/declareFinal
// report at the keyword). The Var is appended to curFunc.Decls -- the ONLY path
// it reaches codegen (`local`) and the LSP. Immutable mirrors the keyword
// (`final` -> immutable). The slot Var.Pos is the slot position so LSP
// go-to-definition lands on the name in the pattern.
func (c *checker) declareTupleSlot(n *ast.TupleBindStmt, s *ast.TupleBindSlot, t Type) {
	if isReservedIdent(s.Name) {
		c.errf(s.Pos, "%q uses the reserved \"__\" namespace and cannot be a variable name", s.Name)
	} else if isReservedName(s.Name) {
		c.errf(s.Pos, "%q is a reserved builtin or constant name and cannot be a variable name", s.Name)
	}
	if prev := c.lookup(s.Name); prev != nil {
		c.errf(s.Pos, "%q is already declared in this or an enclosing scope (no shadowing); previous at %s", s.Name, prev.Pos)
		return
	}
	if tv := c.cur.topConsts[s.Name]; tv != nil {
		c.errf(s.Pos, "%q is already declared in this or an enclosing scope (no shadowing); previous at %s", s.Name, tv.Pos)
		return
	}
	if _, ok := c.cur.funcs[s.Name]; ok {
		c.errf(s.Pos, "%q is a declared function and cannot be shadowed by a variable", s.Name)
		return
	}
	v := &Var{
		Name:      s.Name,
		Mangled:   c.mangleVar(),
		Type:      t,
		Pos:       s.Pos,
		Immutable: n.Final,
	}
	c.scopes[len(c.scopes)-1][s.Name] = v
	c.curFunc.Decls = append(c.curFunc.Decls, v)
}

func (c *checker) checkAssign(n *ast.AssignStmt) {
	if n.Name == "_" {
		c.checkExpr(n.Value) // blank: evaluate RHS for type errors; no lookup, no write
		return
	}
	v := c.lookup(n.Name)
	if v == nil {
		if _, ok := c.cur.topConsts[n.Name]; ok {
			c.errf(n.NamePos, "cannot assign to constant %q", n.Name)
			c.checkExpr(n.Value)
			return
		}
		if isReservedConstant(n.Name) {
			// stdout/stderr (and the reserved constructor names) are reserved
			// constants, not assignable lvalues; match the const-binding message.
			c.errf(n.NamePos, "cannot assign to constant %q", n.Name)
			c.checkExpr(n.Value)
			return
		}
		c.errf(n.NamePos, "assignment to undeclared variable %q%s", n.Name, suggestSuffix(n.Name, c.varNamesInScope()))
		c.checkExpr(n.Value)
		return
	}
	if v.IsConst {
		c.errf(n.NamePos, "cannot assign to constant %q", n.Name)
		c.checkExpr(n.Value)
		return
	}
	if v.Immutable {
		c.errf(n.NamePos, "cannot assign to final %q", n.Name)
		c.checkExpr(n.Value)
		return
	}
	v.Used = true
	c.checkValueAgainst(n.Value, v.Type, func(got Type) {
		c.errf(n.Value.Pos(), "cannot assign %s to variable %q of type %s", got, n.Name, v.Type)
	})
}

// checkFieldAssign type-checks `target.field = value`.
func (c *checker) checkFieldAssign(n *ast.FieldAssignStmt) {
	xt := c.checkExpr(n.Target)
	if xt == Invalid {
		c.checkExpr(n.Value)
		return
	}
	if !c.isStructType(xt) {
		c.errf(n.DotPos, "cannot assign field %q of non-struct type %s", n.Field, xt)
		c.checkExpr(n.Value)
		return
	}
	si := c.info.Structs[string(xt)]
	ft, ok := si.fieldType(n.Field)
	if !ok {
		c.errf(n.DotPos, "struct %q has no field %q", xt, n.Field)
		c.checkExpr(n.Value)
		return
	}
	got := c.checkExprExpecting(n.Value, ft)
	if got != Invalid && ft != Invalid && got != ft {
		c.errf(n.Value.Pos(), "cannot assign %s to field %q of type %s", got, n.Field, ft)
	}
}

// checkIndexAssign type-checks `target[index] = value` for an array element or
// a dict entry (set/insert; overwrite preserves the key's insertion position).
func (c *checker) checkIndexAssign(n *ast.IndexAssignStmt) {
	xt := c.checkExpr(n.Target)
	it := c.checkExpr(n.Index)
	if xt == Invalid {
		c.checkExpr(n.Value)
		return
	}
	if isDict(xt) {
		kt := dictKeyType(xt)
		if it != Invalid && it != kt {
			c.errf(n.Index.Pos(), "dict key must be %s, got %s", kt, it)
		}
		vt := dictValType(xt)
		got := c.checkExprExpecting(n.Value, vt)
		if got != Invalid && vt != Invalid && got != vt {
			c.errf(n.Value.Pos(), "cannot assign %s to a value of %s", got, xt)
		}
		return
	}
	if !isArray(xt) {
		c.errf(n.LBrkPos, "cannot index-assign non-array, non-dict type %s", xt)
		c.checkExpr(n.Value)
		return
	}
	if it != Invalid && it != Int {
		c.errf(n.Index.Pos(), "array index must be int, got %s", it)
	}
	et := elemType(xt)
	got := c.checkExprExpecting(n.Value, et)
	if got != Invalid && et != Invalid && got != et {
		c.errf(n.Value.Pos(), "cannot assign %s to an element of %s", got, xt)
	}
}

// checkForIn type-checks `for (x in coll) { body }`. coll must be an array (x
// binds the element type, PR-B) or a dict (x binds the KEY type K, in insertion
// order, PR-C). x is block-scoped (M1 rule 11), so it is not visible after the
// loop and a sibling for-in may reuse the name.
func (c *checker) checkForIn(n *ast.ForInStmt) {
	ct := c.checkExpr(n.Coll)
	var bindType Type = Invalid
	if ct != Invalid {
		if isArray(ct) {
			bindType = elemType(ct)
		} else if isDict(ct) {
			bindType = dictKeyType(ct)
		} else {
			c.errf(n.Coll.Pos(), "for-in requires an array or dict, got %s", ct)
		}
	}
	c.pushScope()
	if n.Var != "_" {
		c.declareForInVar(n, bindType)
	}
	c.enterLoop()
	c.checkBlock(n.Body)
	c.exitLoop()
	c.popScopeWarnUnused()
}

// declareForInVar binds the for-in loop variable into the current (loop) scope,
// enforcing the reserved-name and no-shadowing rules like a let binding. The
// var is recorded so codegen can resolve its mangled name.
func (c *checker) declareForInVar(n *ast.ForInStmt, t Type) {
	if isReservedIdent(n.Var) {
		c.errf(n.VarPos, "%q uses the reserved \"__\" namespace and cannot be a loop variable", n.Var)
	} else if isReservedName(n.Var) {
		c.errf(n.VarPos, "%q is a reserved builtin or constant name and cannot be a loop variable", n.Var)
	}
	if prev := c.lookup(n.Var); prev != nil {
		c.errf(n.VarPos, "%q is already declared in this or an enclosing scope (no shadowing); previous at %s", n.Var, prev.Pos)
	}
	if _, ok := c.cur.funcs[n.Var]; ok {
		c.errf(n.VarPos, "%q is a declared function and cannot be shadowed by a loop variable", n.Var)
	}
	v := &Var{Name: n.Var, Mangled: c.mangleVar(), Type: t, Pos: n.VarPos, Used: true}
	c.scopes[len(c.scopes)-1][n.Var] = v
	c.curFunc.Decls = append(c.curFunc.Decls, v)
	c.info.ForInVars[n] = v
}

func (c *checker) checkReturn(n *ast.ReturnStmt) {
	if c.tryDepth > 0 {
		c.errf(n.KwPos, "return inside a try/catch/finally body is not allowed")
	}
	if n.Value == nil {
		if c.curRet != Void {
			c.errf(n.KwPos, "return without a value in a function returning %s", c.curRet)
		}
		return
	}
	if c.curRet == Void {
		if !isNoneLiteral(n.Value) {
			c.checkExpr(n.Value) // still type the value to surface any errors within it
		}
		c.errf(n.Value.Pos(), "return with a value in a void function")
		return
	}
	c.checkValueAgainst(n.Value, c.curRet, func(got Type) {
		c.errf(n.Value.Pos(), "return value has type %s, want %s", got, c.curRet)
	})
}

// checkThrow type-checks `throw <expr>` (M5): the operand must be of type
// error. throw is a terminating statement for all-paths-return (returns.go).
func (c *checker) checkThrow(n *ast.ThrowStmt) {
	t := c.checkExpr(n.X)
	if t != Invalid && !isErrorType(t) {
		c.errf(n.X.Pos(), "throw requires an error value, got %s", t)
	}
}

// checkTry type-checks `try { body } catch (e) { handler } [finally { ... }]`
// (M5). Each of the three blocks is its own lexical scope; `e` is bound as the
// built-in error type, block-scoped to the handler. tryDepth is raised across
// all three blocks so return/break/continue inside them are rejected.
func (c *checker) checkTry(n *ast.TryStmt) {
	c.tryDepth++

	c.pushScope()
	c.checkBlock(n.Body)
	c.popScopeWarnUnused()

	c.pushScope()
	if n.CatchVar != "_" {
		c.declareCatchVar(n)
	}
	c.checkBlock(n.Catch)
	c.popScopeWarnUnused()

	if n.HasFinally {
		c.pushScope()
		c.checkBlock(n.Finally)
		c.popScopeWarnUnused()
	}

	c.tryDepth--
}

// declareCatchVar binds the catch variable `e` into the handler scope as the
// built-in error type, enforcing the reserved-name and no-shadowing rules like
// a let binding. It is recorded so codegen resolves its mangled name.
func (c *checker) declareCatchVar(n *ast.TryStmt) {
	if isReservedIdent(n.CatchVar) {
		c.errf(n.CatchVarPos, "%q uses the reserved \"__\" namespace and cannot be a catch variable", n.CatchVar)
	} else if isReservedName(n.CatchVar) {
		c.errf(n.CatchVarPos, "%q is a reserved builtin or constant name and cannot be a catch variable", n.CatchVar)
	}
	if prev := c.lookup(n.CatchVar); prev != nil {
		c.errf(n.CatchVarPos, "%q is already declared in this or an enclosing scope (no shadowing); previous at %s", n.CatchVar, prev.Pos)
	}
	if _, ok := c.cur.funcs[n.CatchVar]; ok {
		c.errf(n.CatchVarPos, "%q is a declared function and cannot be shadowed by a catch variable", n.CatchVar)
	}
	v := &Var{Name: n.CatchVar, Mangled: c.mangleVar(), Type: ErrorType, Pos: n.CatchVarPos, Used: true}
	c.scopes[len(c.scopes)-1][n.CatchVar] = v
	c.curFunc.Decls = append(c.curFunc.Decls, v)
	c.info.CatchVars[n] = v
}

// requireBool checks a predicate expression must be bool (rule 2).
func (c *checker) requireBool(e ast.Expr, ctx string) {
	t := c.checkExpr(e)
	if t != Invalid && t != Bool {
		c.errf(e.Pos(), "%s condition must be bool, got %s", ctx, t)
	}
}

func (c *checker) checkIf(n *ast.IfStmt) {
	c.requireBool(n.Cond, "if")
	c.pushScope()
	c.checkBlock(n.Then)
	c.popScopeWarnUnused()
	for _, ei := range n.ElseIfs {
		c.requireBool(ei.Cond, "else if")
		c.pushScope()
		c.checkBlock(ei.Body)
		c.popScopeWarnUnused()
	}
	if n.Else != nil {
		c.pushScope()
		c.checkBlock(n.Else)
		c.popScopeWarnUnused()
	}
}

// checkMatch checks `match (scrutinee) { arm... }`. The scrutinee must be a
// concrete Optional or Result. All variants must be covered exactly once, or a
// wildcard `_` arm must appear last and covers any remaining variants.
func (c *checker) checkMatch(n *ast.MatchStmt) {
	st := c.checkExpr(n.Scrutinee)
	if st == Invalid {
		for _, arm := range n.Arms {
			c.pushScope()
			c.checkBlock(arm.Body)
			c.popScopeWarnUnused()
		}
		return
	}
	variants := variantsOf(st, c.info)
	if variants == nil {
		c.errf(n.Scrutinee.Pos(), "match requires an Optional, Result, or tagged-union enum scrutinee, got %s", st)
		for _, arm := range n.Arms {
			c.pushScope()
			c.checkBlock(arm.Body)
			c.popScopeWarnUnused()
		}
		return
	}

	allVariants := map[string]bool{}
	for _, v := range variants {
		allVariants[v] = true
	}
	remaining := map[string]bool{}
	for _, v := range variants {
		remaining[v] = true
	}

	for i, arm := range n.Arms {
		isLast := i == len(n.Arms)-1
		switch pat := arm.Pattern.(type) {
		case *ast.WildcardPat:
			if !isLast {
				c.errf(pat.Pos, "wildcard '_' arm must be the last arm")
			} else {
				for k := range remaining {
					delete(remaining, k)
				}
			}
			c.pushScope()
			c.checkBlock(arm.Body)
			c.popScopeWarnUnused()
		case *ast.ConstructorPat:
			if !remaining[pat.Variant] {
				if allVariants[pat.Variant] {
					c.errf(pat.VariantPos, "duplicate match arm: %s already covered", pat.Variant)
				} else {
					c.errf(pat.VariantPos, "variant %s is not a constructor of %s", pat.Variant, st)
				}
				continue
			}
			delete(remaining, pat.Variant)
			bound, hasPayload := matchArmBoundType(st, pat.Variant, c.info)
			if hasPayload && pat.Name == "" {
				c.errf(pat.VariantPos, "variant %s has a payload; write %s(name) to bind it or %s(_) to discard", pat.Variant, pat.Variant, pat.Variant)
			}
			if !hasPayload && pat.Name != "" {
				c.errf(pat.VariantPos, "variant %s has no payload; write %s with no parentheses", pat.Variant, pat.Variant)
			}
			c.pushScope()
			if hasPayload && pat.Name != "" && pat.Name != "_" {
				c.declareMatchArmVar(arm, pat, bound)
			}
			c.checkBlock(arm.Body)
			c.popScopeWarnUnused()
		}
	}
	if len(remaining) > 0 {
		missing := make([]string, 0, len(remaining))
		for v := range remaining {
			missing = append(missing, v)
		}
		sort.Strings(missing)
		c.errf(n.KwPos, "match is not exhaustive: missing arms for %s", strings.Join(missing, ", "))
	}
}

// declareMatchArmVar binds a match arm's payload variable into the arm's scope.
func (c *checker) declareMatchArmVar(arm *ast.MatchArm, pat *ast.ConstructorPat, t Type) {
	if isReservedIdent(pat.Name) {
		c.errf(pat.NamePos, "%q uses the reserved \"__\" namespace and cannot be a binding", pat.Name)
	} else if isReservedName(pat.Name) {
		c.errf(pat.NamePos, "%q is a reserved builtin or constant name and cannot be a binding", pat.Name)
	}
	if prev := c.lookup(pat.Name); prev != nil {
		c.errf(pat.NamePos, "%q is already declared in this or an enclosing scope (no shadowing); previous at %s", pat.Name, prev.Pos)
	}
	if _, ok := c.cur.funcs[pat.Name]; ok {
		c.errf(pat.NamePos, "%q is a declared function and cannot be shadowed by a binding", pat.Name)
	}
	v := &Var{Name: pat.Name, Mangled: c.mangleVar(), Type: t, Pos: pat.NamePos}
	c.scopes[len(c.scopes)-1][pat.Name] = v
	c.curFunc.Decls = append(c.curFunc.Decls, v)
	c.info.MatchArmVars[arm] = v
}

func (c *checker) checkWhile(n *ast.WhileStmt) {
	c.requireBool(n.Cond, "while")
	c.pushScope()
	c.enterLoop()
	c.checkBlock(n.Body)
	c.exitLoop()
	c.popScopeWarnUnused()
}

func (c *checker) checkFor(n *ast.ForStmt) {
	// The for-init declares into the loop's own scope (spec 8.3): it is not
	// visible after the loop and a sibling loop may reuse the name.
	c.pushScope()
	if n.Init != nil {
		c.checkStmt(n.Init)
	}
	if n.Cond != nil {
		c.requireBool(n.Cond, "for")
	}
	if n.Post != nil {
		c.checkStmt(n.Post)
	}
	c.enterLoop()
	c.checkBlock(n.Body)
	c.exitLoop()
	c.popScopeWarnUnused()
}

func (c *checker) checkSwitch(n *ast.SwitchStmt) {
	subj := c.checkExpr(n.Subject)
	// rule 8: subject must be int or string. A bare type variable gets the
	// dedicated bare-T message instead (and only that one).
	tvSubj := c.rejectTypeVar(n.Subject.Pos(), subj, "switch on")
	// rule 8: subject must be int or string, or (R5) an enum -- an enum subject
	// folds to its int at runtime and enables variant-coverage exhaustiveness.
	isEnumSubj := !tvSubj && subj != Invalid && c.isEnumType(subj)
	if !tvSubj && !isEnumSubj && subj != Invalid && subj != Int && subj != String {
		c.errf(n.Subject.Pos(), "switch subject must be int or string, got %s", subj)
	}
	// For an enum subject, track which variant values remain uncovered so a
	// defaultless switch can be checked for exhaustiveness (mirrors the match
	// machinery at the bottom of checkMatch).
	var remaining map[int64]string
	var subjEnum *EnumInfo
	if isEnumSubj {
		subjEnum = c.info.Enums[string(subj)]
		remaining = map[int64]string{}
		for i, name := range subjEnum.Variants {
			remaining[subjEnum.Consts[i].(int64)] = name
		}
	}
	// seenCaseValues tracks folded values across all cases for duplicate detection.
	seenCaseValues := map[interface{}]bool{}
	for _, cs := range n.Cases {
		for _, v := range cs.Values {
			// R-F11/CI-13: `_` in switch-case-value position is match's
			// wildcard spelling, not switch's -- intercept it here, before
			// the generic constant-expression check, so the diagnostic
			// names the actual mistake instead of misreporting `_` as "a
			// variable" (const.go:125) via foldConst's *ast.Ident branch.
			// continue skips ONLY this case value (checkConstExpr, the
			// type-match branch, and duplicate detection for it); it does
			// not touch the post-loop default-check (stmt.go:844-865),
			// which still fires "switch must have a default clause" for
			// this input -- resolved as STACK, not suppressed, since the
			// two messages are two views of the same one-edit fix.
			if id, ok := v.(*ast.Ident); ok && id.Name == "_" {
				c.errf(id.NamePos, `switch has no "_" wildcard; use "default"`)
				continue
			}
			// case values must be constant expressions of the subject type (rule 8).
			// checkConstExpr already rejects variable reads, calls, and interpolations;
			// the old isLiteralExpr guard is removed to allow const refs and folded
			// operator expressions as case values.
			vt := c.checkConstExpr(v)
			var typeOK bool
			if isEnumSubj {
				// Each case in an enum switch must be a variant of the SUBJECT enum
				// (a raw int or another enum's variant -> located error). The variant
				// access folds to the enum type token and the variant's int value.
				if vt != Invalid && vt != subj {
					c.errf(v.Pos(), "case value must be a variant of enum %s, got %s", subjEnum.Name, disp(vt))
				} else if vt == subj {
					typeOK = true
				}
			} else {
				typeOK = (subj == Int || subj == String) && vt != Invalid && vt == subj
				if subj != Invalid && subj != Int && subj != String {
					// subject already errored; skip value type comparison
				} else if vt != Invalid && subj != Invalid && vt != subj {
					c.errf(v.Pos(), "case value has type %s, but switch subject is %s", vt, subj)
				}
			}
			// Duplicate detection: compare folded values across all cases. Only run
			// it for a case whose value is type-valid for the subject, so a
			// type-mismatched case does not also stack a spurious duplicate error on
			// top of the primary type error.
			if typeOK {
				if fv, ok := c.info.FoldedValues[v]; ok && fv != nil {
					if seenCaseValues[fv] {
						c.errf(v.Pos(), "duplicate switch case: %v", fv)
					} else {
						seenCaseValues[fv] = true
						if iv, ok := fv.(int64); ok {
							delete(remaining, iv)
						}
					}
				}
			}
		}
		c.pushScope()
		c.checkBlock(cs.Body)
		c.popScopeWarnUnused()
	}
	if n.Default != nil {
		c.pushScope()
		c.checkBlock(n.Default)
		c.popScopeWarnUnused()
		return
	}
	// No default. For an enum subject (R5) the mandatory-default rule is lifted:
	// the switch is legal iff every variant is covered; otherwise the missing
	// variants are reported (mirrors match exhaustiveness). For a non-enum subject
	// the default is still mandatory (rule 5).
	if isEnumSubj {
		if len(remaining) > 0 {
			missing := make([]string, 0, len(remaining))
			for _, name := range remaining {
				missing = append(missing, name)
			}
			sort.Strings(missing)
			c.errf(n.KwPos, "switch is not exhaustive: missing %s", strings.Join(missing, ", "))
		}
		return
	}
	c.errf(n.KwPos, "switch must have a default clause")
}
