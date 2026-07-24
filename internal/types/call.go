package types

import (
	"fmt"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/token"
)

// arityWant formats the expected-argument-count phrase for an arity diagnostic:
// "N argument(s)" with correct singular/plural when the count is fixed
// (min == max, i.e. no optional parameters), else the "N to M arguments" range
// form for a callee with optional/defaulted parameters.
func arityWant(min, max int) string {
	if min == max {
		if min == 1 {
			return "1 argument"
		}
		return fmt.Sprintf("%d arguments", min)
	}
	return fmt.Sprintf("%d to %d arguments", min, max)
}

// checkCall resolves a call. The callee is an expression (M4). A bare-identifier
// callee resolves by precedence: a local funcref variable -> indirect call; else
// a declared function -> direct call; else a builtin; else error. A non-ident
// callee expression of function type -> indirect call; of any non-function type
// -> compile error. It validates arguments, records a CallInfo, and returns the
// result type.
func (c *checker) checkCall(n *ast.CallExpr) Type {
	// Explicit call-site type arguments (M9) are consumed ONLY by a direct or
	// qualified user-function call. Reject them on every other callee form at this
	// single choke point, BEFORE dispatch. The guard does not return: the call then
	// resolves through its normal handler (which ignores TypeArgs), so a single
	// located error results with no cascade and no silent-ignore.
	if len(n.TypeArgs) > 0 && !c.calleeConsumesTypeArgs(n) {
		c.errf(n.TypeArgs[0].Pos, "%s does not take type arguments", c.uncallableTypeArgForm(n))
	}
	if n.CalleeName != "" {
		return c.checkNamedCall(n)
	}
	// A tagged-union construction call `Enum.Variant(arg)`: the callee is a
	// FieldAccess whose base names an enum type (checked before the namespace path,
	// since an enum-variant callee and a namespace-member callee are both
	// FieldAccess and the enum check must win when the base names an enum).
	if t, ok := c.checkEnumConstruct(n); ok {
		return t
	}
	// A qualified cross-module call `ns.fn(...)` (M8): the callee is a FieldAccess
	// whose base is an in-scope namespace alias not shadowed by a local variable.
	if field, modid, ok := c.qualifiedNsTarget(n.Callee); ok {
		return c.checkQualifiedCall(n, field, modid)
	}
	return c.checkIndirectCall(n, c.checkExpr(n.Callee))
}

// calleeConsumesTypeArgs reports whether the call's callee is the one form that
// accepts explicit type arguments: a direct user-function call (bare identifier
// naming a declared function, not shadowed by a local variable) or a qualified
// cross-module user-function call. It mirrors checkNamedCall's routing precedence
// exactly (a local variable shadows a function name), using read-only lookups, so
// its decision can never diverge from where the call actually routes.
func (c *checker) calleeConsumesTypeArgs(n *ast.CallExpr) bool {
	if n.CalleeName != "" {
		if c.lookup(n.CalleeName) != nil {
			return false // a local variable (funcref or not) -> indirect / not-callable
		}
		_, ok := c.cur.funcs[n.CalleeName]
		return ok
	}
	_, _, ok := c.qualifiedNsTarget(n.Callee)
	return ok
}

// uncallableTypeArgForm names the callee form for the type-args rejection message,
// for a callee that does not consume type arguments.
func (c *checker) uncallableTypeArgForm(n *ast.CallExpr) string {
	if name := n.CalleeName; name != "" {
		switch name {
		case "Some", "Ok", "Err":
			return fmt.Sprintf("the constructor %q", name)
		}
		if isReservedConstant(name) {
			return fmt.Sprintf("%q", name)
		}
		if isBuiltin(name) {
			return fmt.Sprintf("builtin %q", name)
		}
		return fmt.Sprintf("%q", name)
	}
	return "a function-reference call"
}

// checkQualifiedCall resolves `ns.fn(args)` against the exported function `field`
// of the module bound to the namespace (target modid). It type-checks exactly like
// a direct user call and records a CallUser CallInfo whose Func is the target decl
// (so reachability tree-shaking follows the cross-module edge) and whose Mangled
// uses the target module's modid.
func (c *checker) checkQualifiedCall(n *ast.CallExpr, field string, modid int) Type {
	// Resolve the namespace alias name for diagnostics.
	nsName := n.Callee.(*ast.FieldAccess).X.(*ast.Ident).Name
	// A reserved core module (json, ...) resolves its members through the core
	// catalog, not Prog-derived symbol tables (its Prog is empty).
	if coreNS := c.modCtx[modid].core; coreNS != "" {
		return c.checkCoreCall(n, coreNS, field)
	}
	tctx := c.modCtx[modid]
	fn, ok := tctx.funcs[field]
	if !ok {
		c.errf(n.CalleePos, "module %q has no function %q", nsName, field)
		c.typeArgs(n.Args)
		return Invalid
	}
	if !tctx.exported[field] {
		c.errf(n.CalleePos, "%q is not exported by %q", field, nsName)
		c.typeArgs(n.Args)
		return Invalid
	}
	return c.checkUserCallIn(n, fn, modid)
}

// checkNamedCall resolves a call whose callee is a bare identifier.
func (c *checker) checkNamedCall(n *ast.CallExpr) Type {
	name := n.CalleeName
	if name == "Some" {
		return c.checkSomeCall(n)
	}
	if name == "Ok" {
		return c.checkOkCall(n)
	}
	if name == "Err" {
		return c.checkErrCall(n)
	}
	if isReservedConstant(name) {
		c.errf(n.CalleePos, "%q is not callable", name)
		c.typeArgs(n.Args)
		return Invalid
	}
	// A local variable shadows nothing of these (the no-shadow rule keeps a
	// variable from sharing a function/builtin name), but a funcref-typed local is
	// the indirect-call case and takes precedence.
	if v := c.lookup(name); v != nil {
		v.Used = true
		c.info.Uses[n.Callee.(*ast.Ident)] = v
		if isFuncref(v.Type) {
			return c.checkIndirectCallType(n, v.Type)
		}
		c.errf(n.CalleePos, "%q is not callable (it has type %s)", name, v.Type)
		c.typeArgs(n.Args)
		return Invalid
	}
	if fn, ok := c.cur.funcs[name]; ok {
		return c.checkUserCallIn(n, fn, c.cur.id)
	}
	if isBuiltin(name) {
		// ERGO-6: map/filter are removable (array namespace) but their
		// Optional/Result combinator overload is a flat carrier-family member.
		// When arg-0 is statically Optional/Result, dispatch to the combinator
		// overload under the bare spelling instead of firing the module hint.
		// Array arg-0 and the zero/one-arg forms fall through to the hint below.
		if (name == "map" || name == "filter") && len(n.Args) == 2 {
			if t := c.checkExpr(n.Args[0]); isOptional(t) || isResult(t) {
				if name == "map" {
					return c.checkMapCall(n, name)
				}
				return c.checkFilterCall(n, name)
			}
		}
		// A modularized builtin's bare flat call was removed: it is reachable only
		// through its module home now. Emit the module-hint error instead of
		// resolving it as a flat builtin. User-bound names are caught above (local
		// var / user func), so this fires only when the name is not user-bound.
		if hint, ok := removedHint(name); ok {
			ns, _, _ := strings.Cut(hint, ".")
			c.errf(n.CalleePos, "%q was moved to a module; import %q and call it as %s(...)", name, ns, hint)
			c.typeArgs(n.Args)
			return Invalid
		}
		return c.checkBuiltinCall(n)
	}
	c.errf(n.CalleePos, "call to undeclared function %q%s", name, suggestSuffix(name, c.funcAndBuiltinNames()))
	c.typeArgs(n.Args)
	return Invalid
}

// checkIndirectCall resolves a call whose callee is a non-identifier expression
// whose already-resolved type is calleeType. A non-function type is an error.
func (c *checker) checkIndirectCall(n *ast.CallExpr, calleeType Type) Type {
	if calleeType == Invalid {
		c.typeArgs(n.Args)
		return Invalid
	}
	if !isFuncref(calleeType) {
		c.errf(n.CalleePos, "cannot call a value of type %s", calleeType)
		c.typeArgs(n.Args)
		return Invalid
	}
	return c.checkIndirectCallType(n, calleeType)
}

// checkIndirectCallType validates the arguments of an indirect call against the
// funcref signature ft (exact arity, no defaults -- spec 2.2), records the
// CallInfo, and returns the result type.
func (c *checker) checkIndirectCallType(n *ast.CallExpr, ft Type) Type {
	params := funcParamTypes(ft)
	ret := funcRetType(ft)
	argTypes := make([]Type, len(n.Args))
	for i, a := range n.Args {
		argTypes[i] = c.checkExpr(a)
	}
	if len(n.Args) != len(params) {
		c.errf(n.CalleePos, "function reference expects %s, got %d", arityWant(len(params), len(params)), len(n.Args))
		return ret
	}
	for i, want := range params {
		at := argTypes[i]
		if at != Invalid && want != Invalid && at != want {
			c.errf(n.Args[i].Pos(), "argument %d has type %s, want %s", i+1, at, want)
		}
	}
	c.info.Calls[n] = &CallInfo{
		Kind:   CallIndirect,
		Args:   n.Args,
		Result: ret,
	}
	return ret
}

func (c *checker) typeArgs(args []ast.Expr) {
	for _, a := range args {
		c.checkExpr(a)
	}
}

// checkSomeCall types Some(x) -> Optional[typeof x] (spec 3.2). x must be a
// non-void value; Some(None) is the bottom-up None error; Some(<void>) is the
// void-argument error. Invalid x suppresses.
func (c *checker) checkSomeCall(n *ast.CallExpr) Type {
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "Some expects 1 argument, got %d", len(n.Args))
		c.typeArgs(n.Args)
		return Invalid
	}
	xt := c.checkExpr(n.Args[0]) // Some(None): None is bottom-up here -> noneNeedsContext + Invalid
	if xt == Invalid {
		return Invalid
	}
	if xt == Void {
		c.errf(n.Args[0].Pos(), "Some requires a value, got void")
		return Invalid
	}
	res := optionalType(xt)
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "Some", Args: []ast.Expr{n.Args[0]}, Result: res}
	return res
}

// checkOkCall types Ok(x) -> Result[typeof x] (spec 3.2), fully inferred like
// Some. It does NOT special-case a context-dependent argument: Ok(None)/Ok(Err)
// self-error bottom-up (the inner node emits its own needs-context error and
// Invalid), and Ok(Invalid) suppresses to Invalid.
func (c *checker) checkOkCall(n *ast.CallExpr) Type {
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "Ok expects 1 argument, got %d", len(n.Args))
		c.typeArgs(n.Args)
		return Invalid
	}
	xt := c.checkExpr(n.Args[0])
	if xt == Invalid {
		return Invalid
	}
	if xt == Void {
		c.errf(n.Args[0].Pos(), "Ok requires a value, got void")
		return Invalid
	}
	res := resultType(xt)
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "Ok", Args: []ast.Expr{n.Args[0]}, Result: res}
	return res
}

// checkErrCall handles the BOTTOM-UP Err(e) path: no expected Result is threaded
// here, so the success type T is unknown and this is the spec-5.3 needs-context
// error (parallel to a bare None in checkExpr). The argument is still checked and
// must be the built-in error type. The blessed-site path that supplies T and
// records info.Calls is the Err branch of checkValueAgainst.
func (c *checker) checkErrCall(n *ast.CallExpr) Type {
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "Err expects 1 argument, got %d", len(n.Args))
		c.typeArgs(n.Args)
		return Invalid
	}
	c.requireErrorArg(n)
	c.errNeedsContext(n.CalleePos)
	return Invalid
}

// requireErrorArg checks that an Err call's single argument is the built-in error
// type (a non-error argument is its own type error). Used by both the bottom-up
// and blessed-site Err paths.
func (c *checker) requireErrorArg(n *ast.CallExpr) {
	at := c.checkExpr(n.Args[0])
	if at != Invalid && at != ErrorType {
		c.errf(n.Args[0].Pos(), "Err requires an error value, got %s", at)
	}
}

// errNeedsContext is the spec-5.3 diagnostic for an Err with no expected Result
// type to concretize its success type T.
func (c *checker) errNeedsContext(pos token.Position) {
	c.errf(pos, "Err requires an expected Result type here; annotate the binding or use it in a Result context")
}

// isErrCall reports whether e is an Err(...) constructor call. The callee name is
// the CalleeName string on a bare-ident call.
func isErrCall(e ast.Expr) bool {
	ce, ok := e.(*ast.CallExpr)
	return ok && ce.CalleeName == "Err"
}

// optionalArg checks that arg is a CONCRETE Optional[T] and returns (type, elem,
// ok). A non-Optional arg is an error. A None argument is already an error from
// checkExpr (bottom-up noneNeedsContext) and surfaces here as Invalid.
func (c *checker) optionalArg(arg ast.Expr, name string) (Type, Type, bool) {
	ot := c.checkExpr(arg)
	if ot == Invalid {
		return Invalid, Invalid, false
	}
	if !isOptional(ot) {
		c.errf(arg.Pos(), "%s requires an Optional value, got %s", name, ot)
		return Invalid, Invalid, false
	}
	return ot, optionalElemType(ot), true
}

func (c *checker) checkIsSomeNoneCall(n *ast.CallExpr, name string) Type {
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", name, len(n.Args))
		c.typeArgs(n.Args)
		return Bool
	}
	if _, _, ok := c.optionalArg(n.Args[0], name); !ok {
		return Bool
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: name, Args: []ast.Expr{n.Args[0]}, Result: Bool}
	return Bool
}

// resultArg checks that arg is a CONCRETE Result[T] and returns (type, elem, ok).
// A non-Result arg is an error; mirrors optionalArg.
func (c *checker) resultArg(arg ast.Expr, name string) (Type, Type, bool) {
	rt := c.checkExpr(arg)
	if rt == Invalid {
		return Invalid, Invalid, false
	}
	if !isResult(rt) {
		c.errf(arg.Pos(), "%s requires a Result value, got %s", name, rt)
		return Invalid, Invalid, false
	}
	return rt, resultElemType(rt), true
}

func (c *checker) checkIsOkErrCall(n *ast.CallExpr, name string) Type {
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", name, len(n.Args))
		c.typeArgs(n.Args)
		return Bool
	}
	if _, _, ok := c.resultArg(n.Args[0], name); !ok {
		return Bool
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: name, Args: []ast.Expr{n.Args[0]}, Result: Bool}
	return Bool
}

func (c *checker) checkUnwrapErrCall(n *ast.CallExpr) Type {
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "unwrap_err expects 1 argument, got %d", len(n.Args))
		c.typeArgs(n.Args)
		return ErrorType
	}
	if _, _, ok := c.resultArg(n.Args[0], "unwrap_err"); !ok {
		return ErrorType
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "unwrap_err", Args: []ast.Expr{n.Args[0]}, Result: ErrorType}
	return ErrorType
}

// unwrapArg resolves the element type for the overloaded unwrap/unwrap_or, which
// accept EITHER an Optional[T] or a Result[T] (dispatched on the static arg type).
// Returns (elem, ok); a non-Optional/non-Result arg is an error.
func (c *checker) unwrapArg(arg ast.Expr, name string) (Type, bool) {
	t := c.checkExpr(arg)
	if t == Invalid {
		return Invalid, false
	}
	if isOptional(t) {
		return optionalElemType(t), true
	}
	if isResult(t) {
		return resultElemType(t), true
	}
	c.errf(arg.Pos(), "%s requires an Optional or Result value, got %s", name, t)
	return Invalid, false
}

func (c *checker) checkUnwrapCall(n *ast.CallExpr) Type {
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "unwrap expects 1 argument, got %d", len(n.Args))
		c.typeArgs(n.Args)
		return Invalid
	}
	et, ok := c.unwrapArg(n.Args[0], "unwrap")
	if !ok {
		return Invalid
	}
	// Builtin stays "unwrap"; codegen re-dispatches on the operand's static type.
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "unwrap", Args: []ast.Expr{n.Args[0]}, Result: et}
	return et
}

func (c *checker) checkUnwrapOrCall(n *ast.CallExpr) Type {
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "unwrap_or expects 2 arguments, got %d", len(n.Args))
		c.typeArgs(n.Args)
		return Invalid
	}
	et, ok := c.unwrapArg(n.Args[0], "unwrap_or")
	// The fallback is an ARGUMENT position, NOT a concretization site. Type it
	// BOTTOM-UP: a None/Err fallback self-errors (needs-context), and any other
	// fallback is compared to et with plain ==.
	got := c.checkExpr(n.Args[1])
	if !ok {
		return Invalid // first arg already reported; et is meaningless
	}
	if got != Invalid && et != Invalid && got != et {
		c.errf(n.Args[1].Pos(), "argument 2 of unwrap_or has type %s, want %s (the Optional/Result element type)", got, et)
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "unwrap_or", Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: et}
	return et
}

func (c *checker) checkBuiltinCall(n *ast.CallExpr) Type {
	return c.checkBuiltinNamed(n, n.CalleeName, n.CalleeName)
}

// checkBuiltinNamed runs the full flat-builtin dispatch keyed on `name` (the
// builtin key, decides which handler runs and what CallInfo.Builtin records),
// independent of n.CalleeName. dispName is the separate, display-only spelling
// used in every diagnostic the handler produces. A flat call passes
// dispName == n.CalleeName; the reserved core-module bridge passes a delegating
// member's ns.member spelling, so a namespaced call (e.g. math.clamp) reuses the
// exact overload / composite-arg / special-return / arg-domain dispatch of the
// flat builtin and lowers byte-identically, while its diagnostics name the
// namespaced spelling instead of the dead flat key.
func (c *checker) checkBuiltinNamed(n *ast.CallExpr, name, dispName string) Type {
	// length and push have array-aware signatures the fixed builtin table cannot
	// express; handle them before the generic path.
	c.checkBuiltinArgDomains(n, name, dispName)

	switch name {
	case "length":
		if t := c.checkLengthCall(n); t != noSpecial {
			return t
		}
	case "push":
		return c.checkPushCall(n, dispName)
	case "has":
		return c.checkHasCall(n, dispName)
	case "keys":
		return c.checkKeysCall(n, dispName)
	case "map":
		return c.checkMapCall(n, dispName)
	case "filter":
		return c.checkFilterCall(n, dispName)
	case "each":
		return c.checkEachCall(n, dispName)
	case "zip":
		return c.checkZipCall(n, dispName)
	case "parse_args":
		return c.checkParseArgsCall(n)
	case "and_then":
		return c.checkAndThenCall(n)
	case "or_else":
		return c.checkOrElseCall(n)
	case "map_err":
		return c.checkMapErrCall(n)
	case "join":
		return c.checkJoinCall(n, dispName)
	case "contains":
		return c.checkContainsCall(n, dispName)
	case "index_of":
		return c.checkIndexOfCall(n, dispName)
	case "abs":
		return c.checkAbsCall(n, dispName)
	case "min":
		return c.checkMinMaxCall(n, "min", dispName)
	case "max":
		return c.checkMinMaxCall(n, "max", dispName)
	case "reverse":
		return c.checkReverseCall(n, dispName)
	case "reduce":
		return c.checkReduceCall(n, dispName)
	case "run":
		return c.checkRunCall(n, dispName)
	case "exec_command":
		return c.checkExecCommandCall(n, dispName)
	case "run_env":
		return c.checkRunEnvCall(n, dispName)
	case "run_env_status":
		return c.checkRunEnvStatusCall(n, dispName)
	case "run_env_full":
		return c.checkRunEnvFullCall(n, dispName)
	case "run_status":
		return c.checkRunStatusCall(n, dispName)
	case "run_full":
		return c.checkRunFullCall(n, dispName)
	case "run_input":
		return c.checkRunInputCall(n, dispName)
	case "run_input_full":
		return c.checkRunInputFullCall(n, dispName)
	case "sort":
		return c.checkSortCall(n, dispName)
	case "sort_by":
		return c.checkSortByCall(n, dispName)
	case "find":
		return c.checkFindAnyAllCall(n, "find", dispName, optionalType(Int))
	case "any":
		return c.checkFindAnyAllCall(n, "any", dispName, Bool)
	case "all":
		return c.checkFindAnyAllCall(n, "all", dispName, Bool)
	case "slice":
		return c.checkSliceCall(n, dispName)
	case "concat":
		return c.checkConcatCall(n, dispName)
	case "sum":
		return c.checkSumCall(n, dispName)
	case "range":
		return c.checkRangeCall(n, dispName)
	case "first":
		return c.checkFirstLastCall(n, "first", dispName)
	case "last":
		return c.checkFirstLastCall(n, "last", dispName)
	case "values":
		return c.checkValuesCall(n, dispName)
	case "remove":
		return c.checkRemoveCall(n, dispName)
	case "merge":
		return c.checkMergeCall(n, dispName)
	case "clamp":
		return c.checkClampCall(n, dispName)
	case "sign":
		return c.checkSignCall(n, dispName)
	case "count_where":
		return c.checkCountWhereCall(n, dispName)
	case "flatten":
		return c.checkFlattenCall(n, dispName)
	case "unique":
		return c.checkUniqueCall(n, dispName)
	case "take":
		return c.checkTakeDropCall(n, "take", dispName)
	case "drop":
		return c.checkTakeDropCall(n, "drop", dispName)
	case "pop":
		return c.checkPopCall(n, dispName)
	case "remove_at":
		return c.checkRemoveAtCall(n, dispName)
	case "insert_at":
		return c.checkInsertAtCall(n, dispName)
	case "size":
		return c.checkSizeCall(n, dispName)
	case "clear":
		return c.checkClearCall(n, dispName)
	case "array_is_empty":
		return c.checkArrayIsEmptyCall(n, dispName)
	case "dict_is_empty":
		return c.checkDictIsEmptyCall(n, dispName)
	case "to_int":
		if t, ok := c.checkIntEnumCall(n); ok {
			return t
		}
	case "to_string":
		if t, ok := c.checkStringEnumCall(n); ok {
			return t
		}
	case "to_bool":
		if t, ok := c.checkBoolEnumCall(n); ok {
			return t
		}
	case "to_float":
		if t, ok := c.checkFloatEnumCall(n); ok {
			return t
		}
	case "debug":
		return c.checkDebugCall(n)
	case "is_some":
		return c.checkIsSomeNoneCall(n, "is_some")
	case "is_none":
		return c.checkIsSomeNoneCall(n, "is_none")
	case "is_ok":
		return c.checkIsOkErrCall(n, "is_ok")
	case "is_err":
		return c.checkIsOkErrCall(n, "is_err")
	case "unwrap":
		return c.checkUnwrapCall(n)
	case "unwrap_err":
		return c.checkUnwrapErrCall(n)
	case "unwrap_or":
		return c.checkUnwrapOrCall(n)
	case "get":
		return c.checkGetCall(n, dispName)
	case "assert_eq":
		return c.checkAssertEqNeCall(n, "assert_eq")
	case "assert_ne":
		return c.checkAssertEqNeCall(n, "assert_ne")
	case "assert_some":
		return c.checkAssertOptionalCall(n, "assert_some")
	case "assert_none":
		return c.checkAssertOptionalCall(n, "assert_none")
	case "assert_ok":
		return c.checkAssertResultCall(n, "assert_ok")
	case "assert_err":
		return c.checkAssertResultCall(n, "assert_err")
	case "assert_contains":
		return c.checkAssertContainsCall(n)
	case "on_exit":
		return c.checkOnExitCall(n)
	case "on_signal":
		return c.checkOnSignalCall(n)
	case "pipe":
		return c.checkPipeCall(n, dispName)
	case "spawn":
		return c.checkSpawnCall(n, dispName)
	case "wait":
		return c.checkWaitCall(n, dispName)
	case "is_done":
		return c.checkIsDoneCall(n, dispName)
	case "signal":
		return c.checkSignalCall(n, dispName)
	case "wait_any":
		return c.checkWaitAnyCall(n, dispName)
	case "make_fifo":
		return c.checkMakeFifoCall(n, dispName)
	}

	return c.checkBuiltinSig(n, dispName, name, builtinSigs[name])
}

// checkBuiltinSig runs the generic builtin-signature path -- arity, per-argument
// type-set check, defaults fill, and CallInfo record -- for a call with signature
// `sig`. It is the shared seam reused by the reserved core-module bridge
// (checkCoreCall). dispName is the name used in diagnostics (e.g. "json.encode");
// builtin is the CallInfo.Builtin key codegen dispatches on (e.g. "json_encode").
// For a flat builtin the two are equal (== n.CalleeName), so behavior is
// byte-identical to the pre-extraction code.
func (c *checker) checkBuiltinSig(n *ast.CallExpr, dispName, builtin string, sig builtinSig) Type {
	argTypes := make([]Type, len(n.Args))
	for i, a := range n.Args {
		argTypes[i] = c.checkExpr(a)
	}

	// arity: count required (non-defaulted) params.
	required := 0
	for _, p := range sig.params {
		if !p.hasDefault {
			required++
		}
	}
	if len(n.Args) < required || len(n.Args) > len(sig.params) {
		c.errf(n.CalleePos, "%s expects %s, got %d", dispName, arityWant(required, len(sig.params)), len(n.Args))
		return sig.result
	}

	for i, p := range sig.params {
		if i >= len(n.Args) {
			break
		}
		at := argTypes[i]
		if at == Invalid {
			continue
		}
		if !typeInSet(at, p.types) {
			c.errf(n.Args[i].Pos(), "argument %d of %s has type %s, want %s", i+1, dispName, at, joinTypes(p.types))
		}
	}

	// rule 9: print's `to` must be the reserved constant stdout or stderr.
	if builtin == "print" && len(n.Args) >= 2 {
		c.requireStdoutStderr(n.Args[1])
	}

	// Build the defaults-filled argument list for codegen.
	full := make([]ast.Expr, len(sig.params))
	copy(full, n.Args)
	for i := len(n.Args); i < len(sig.params); i++ {
		// only print has a default in M1: `to = stdout`.
		full[i] = defaultArgFor(builtin, i)
	}

	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: builtin,
		Args:    full,
		Result:  sig.result,
	}
	return sig.result
}

// checkUserCallIn resolves a direct user call to fn, defined in module modid (the
// current module for a local call, or the target module for a qualified call).
// Arguments are checked in the CALLER's context; the callee's parameter and return
// type annotations are resolved in the DEFINING module's context (so a parameter
// typed as one of that module's own structs resolves to that module's token, which
// the caller's same-struct argument also resolves to -- cross-module type equality
// holds). The recorded CallInfo's Mangled carries the defining module's modid.
func (c *checker) checkUserCallIn(n *ast.CallExpr, fn *ast.FuncDecl, modid int) Type {
	callName := n.CalleeName
	if callName == "" {
		callName = fn.Name
	}

	// Resolve the callee's signature types in its DEFINING module's context, with
	// the CALLEE's type parameters active so a generic callee's annotations resolve
	// to type variables (the caller's typeParams are unrelated), BEFORE checking the
	// arguments. This lets a concrete parameter's type flow down as the argument's
	// expected type, so an overloaded/generic builtin funcref reference (`abs`,
	// `map`, ...) can be disambiguated by "the expected parameter type at the
	// reference site" (design spec Part 2), not only by a `let` annotation. A
	// generic callee's OTHER parameters may still contain unresolved type
	// variables ("$"-prefixed, illegal in source and in every composite type --
	// see typeVarType) at this point; those individual arguments are checked
	// without a `want` hint, unchanged from before, and checkGenericUserCall
	// performs its own structural inference below.
	savedTP := c.typeParams
	if len(fn.TypeParams) > 0 {
		c.typeParams = map[string]bool{}
		for _, p := range fn.TypeParams {
			c.typeParams[p] = true
		}
	} else {
		c.typeParams = nil
	}
	caller := c.cur
	c.cur = c.modCtx[modid]
	ret := c.resolveType(fn.RetType, fn.KwPos)
	paramTypes := make([]Type, len(fn.Params))
	for i := range fn.Params {
		paramTypes[i] = c.resolveType(fn.Params[i].Type, fn.Params[i].NamePos)
	}
	c.cur = caller
	c.typeParams = savedTP

	argTypes := make([]Type, len(n.Args))
	for i, a := range n.Args {
		if i < len(paramTypes) && !strings.Contains(string(paramTypes[i]), "$") {
			argTypes[i] = c.checkExprExpecting(a, paramTypes[i])
		} else {
			argTypes[i] = c.checkExpr(a)
		}
	}

	// Explicit call-site type arguments (M9). Resolve them in the CALLER's context
	// NOW -- before the swap to the defining module below -- so aliases and the
	// caller's own type parameters resolve correctly and a struct-typed argument
	// gets the caller's module identity (cross-module equality with a same-struct
	// value argument). seeded/typeArgPos are threaded into the generic path.
	var explicit []Type
	var typeArgPos map[string]token.Position
	var seeded map[string]bool
	// preSuppressed: type params whose cannot-infer sweep must stay silent because
	// the type-arg list itself was already diagnosed (an unresolvable arg, or a
	// count mismatch). Without this a return-only generic on those error paths
	// piles a spurious cannot-infer on top of the primary error (spec error 10).
	var preSuppressed map[string]bool
	if len(n.TypeArgs) > 0 {
		if len(fn.TypeParams) == 0 {
			c.errf(n.TypeArgs[0].Pos, "%s is not generic and takes no type arguments", callName)
			for _, ta := range n.TypeArgs {
				c.resolveType(ta.Name, ta.Pos) // still surface unknown-type diagnostics
			}
		} else if len(n.TypeArgs) != len(fn.TypeParams) {
			c.errf(n.TypeArgs[0].Pos, "%s expects %d type argument(s), got %d", callName, len(fn.TypeParams), len(n.TypeArgs))
			for _, ta := range n.TypeArgs {
				c.resolveType(ta.Name, ta.Pos)
			}
			// Run unseeded inference below (explicit stays nil), but suppress the
			// cannot-infer cascade for every param: the arity error is the diagnostic.
			preSuppressed = map[string]bool{}
			for _, tp := range fn.TypeParams {
				preSuppressed[tp] = true
			}
		} else {
			explicit = make([]Type, len(n.TypeArgs))
			typeArgPos = map[string]token.Position{}
			seeded = map[string]bool{}
			for i, ta := range n.TypeArgs {
				explicit[i] = c.resolveType(ta.Name, ta.Pos)
				tp := fn.TypeParams[i]
				typeArgPos[tp] = ta.Pos
				seeded[tp] = true
				// An unresolvable arg (already diagnosed) leaves its param unbound;
				// keep the cannot-infer sweep quiet for it.
				if explicit[i] == Invalid {
					if preSuppressed == nil {
						preSuppressed = map[string]bool{}
					}
					preSuppressed[tp] = true
				}
			}
		}
	}

	required := 0
	for _, p := range fn.Params {
		if p.Default == nil {
			required++
		}
	}

	if len(n.Args) < required || len(n.Args) > len(fn.Params) {
		c.errf(n.CalleePos, "%s expects %s, got %d", callName, arityWant(required, len(fn.Params)), len(n.Args))
		// For a generic callee, ret may still contain type variables (no inference
		// ran on this error path); return Invalid so a "$T" never reaches the call
		// node's recorded type and triggers a cascade.
		if len(fn.TypeParams) > 0 {
			return Invalid
		}
		return ret
	}

	if len(fn.TypeParams) > 0 {
		return c.checkGenericUserCall(n, fn, modid, callName, argTypes, paramTypes, ret, explicit, seeded, typeArgPos, preSuppressed)
	}

	for i := range fn.Params {
		if i >= len(n.Args) {
			break
		}
		at := argTypes[i]
		want := paramTypes[i]
		if at != Invalid && want != Invalid && at != want {
			c.errf(n.Args[i].Pos(), "argument %d of %s has type %s, want %s", i+1, callName, at, want)
		}
	}

	// defaults-filled argument list: omitted trailing args take the parameter's
	// default constant-expression node.
	full := make([]ast.Expr, len(fn.Params))
	copy(full, n.Args)
	for i := len(n.Args); i < len(fn.Params); i++ {
		full[i] = fn.Params[i].Default
	}

	c.info.Calls[n] = &CallInfo{
		Kind:    CallUser,
		Func:    fn,
		Mangled: mangleFunc(modid, fn.Name),
		Args:    full,
		Result:  ret,
	}
	return ret
}

// checkGenericUserCall infers the type arguments of a generic callee by one-pass
// structural unification of each declared parameter type against the actual
// argument type, then substitutes the bindings into the return type (an unbound
// parameter -> Invalid). Conflicts, structural mismatches, and un-inferable
// parameters are non-cascading, suppression-aware diagnostics (spec 4.2-4.4).
//
// explicit (optional, len == len(fn.TypeParams) when present) seeds subst with
// call-site type arguments; seeded marks which params came from explicit args and
// typeArgPos maps each to its source position for located bound diagnostics.
// preSuppressed marks params whose cannot-infer sweep must stay silent because the
// type-arg list was already diagnosed (unresolvable arg / arity mismatch).
func (c *checker) checkGenericUserCall(n *ast.CallExpr, fn *ast.FuncDecl, modid int,
	callName string, argTypes, paramTypes []Type, ret Type,
	explicit []Type, seeded map[string]bool, typeArgPos map[string]token.Position,
	preSuppressed map[string]bool) Type {
	subst := map[string]Type{}
	// Seed explicit call-site type arguments before unifying value args, so the
	// same engine that infers also validates the explicit bindings (a value arg
	// contradicting a seed surfaces via the unify conflict path). An Invalid seed
	// (unresolvable type arg, already diagnosed) is skipped.
	for i, tp := range fn.TypeParams {
		if explicit != nil && explicit[i] != Invalid {
			subst[tp] = explicit[i]
		}
	}
	// origin: type-param name -> index of the argument that FIRST introduced a
	// binding for it in subst (the bound type may itself be another type variable,
	// e.g. $U), for first-binder blame on a bound-satisfaction failure.
	origin := map[string]int{}
	// suppressed: type-param names left unbound SOLELY by an argument that already
	// produced its own error. The cannot-infer sweep stays silent only for these.
	suppressed := map[string]bool{}
	// Params whose type-arg list was already diagnosed (unresolvable arg / arity
	// mismatch) start suppressed so the cannot-infer sweep does not cascade.
	for tp := range preSuppressed {
		suppressed[tp] = true
	}
	// anyArgErrored: some argument (or param type) was already Invalid, so the call
	// is fundamentally broken; the bound-satisfaction sweep is skipped entirely to
	// avoid piling a "does not satisfy comparable" on top, even for a bounded param
	// the errored argument did not mention.
	anyArgErrored := false
	for i := range fn.Params {
		if i >= len(n.Args) {
			break
		}
		at := argTypes[i]
		want := paramTypes[i]
		if at == Invalid || want == Invalid {
			anyArgErrored = true
			for tp := range typeVarsIn(want, fn.TypeParams) {
				suppressed[tp] = true
			}
			continue
		}
		// All-or-nothing per top-level argument: unify into a COPY of subst and
		// commit only on full success (spec 4.2 "never bound from a failed branch").
		tentative := cloneSubst(subst)
		var cf conflict
		if c.unify(want, at, tentative, &cf) {
			for k := range tentative {
				if _, had := subst[k]; !had {
					if _, seen := origin[k]; !seen {
						origin[k] = i // first argument to bind k
					}
				}
			}
			subst = tentative
			continue
		}
		if cf.found {
			if seeded[cf.param] {
				c.errf(n.Args[i].Pos(), "explicit type argument %s = %s conflicts with argument %d of type %s",
					cf.param, disp(cf.prior), i+1, disp(cf.next))
			} else {
				c.errf(n.Args[i].Pos(), "cannot infer type parameter %s of %s: bound to %s but also %s",
					cf.param, callName, cf.prior, cf.next)
			}
		} else {
			c.errf(n.Args[i].Pos(), "argument %d of %s has type %s, which does not match %s",
				i+1, callName, at, want)
		}
		// This argument errored at the call: any type param it constrains must not
		// also pile on a redundant cannot-infer (spec 4.4, no cascade).
		for tp := range typeVarsIn(want, fn.TypeParams) {
			suppressed[tp] = true
		}
		ret = Invalid
	}
	for _, tp := range fn.TypeParams {
		if _, ok := subst[tp]; !ok {
			if suppressed[tp] {
				ret = Invalid
				continue
			}
			c.errf(n.CalleePos, "cannot infer type parameter %s of %s; give the argument a concrete type", tp, callName)
			ret = Invalid
		}
	}
	// Bound satisfaction: a comparable T must be bound to int, bool, or string.
	// Runs after unification commits; skips unbound/Invalid bindings (those are
	// already diagnosed by the cannot-infer / suppression paths). It is also
	// skipped entirely when the call is already Invalid from an earlier argument
	// mismatch / unification / cannot-infer error, so it never piles a second
	// "does not satisfy comparable" on top of the primary diagnostic (no cascade).
	for _, tp := range fn.TypeParams {
		if ret == Invalid || anyArgErrored {
			break
		}
		if fn.TypeParamBounds[tp] != "comparable" {
			continue
		}
		// Skip a param whose constraining argument already errored (its type was
		// Invalid): the primary error stands; no second "does not satisfy" cascade
		// even when ret is not yet Invalid (e.g. an undefined-variable argument).
		if suppressed[tp] {
			continue
		}
		ct, ok := subst[tp]
		if !ok || ct == Invalid {
			continue
		}
		// Accept a concrete int/bool/string, OR a type variable that is itself
		// comparable-bounded in the CALLER's scope: when a comparable generic is
		// called from inside another generic, unification binds the callee's T to
		// the caller's $U, and the bound propagates when U: comparable.
		if !c.isComparableScalar(ct) && !c.isComparableTypeVar(ct) {
			pos := boundErrPos(n, tp, origin, typeArgPos)
			shown := disp(ct) // strip struct @modid / typevar $ from the message
			if isTypeVar(ct) {
				shown = "type parameter " + typeVarName(ct) // never show the "$U" encoding
			}
			c.errf(pos, "cannot use %s as type parameter %s: it does not satisfy comparable (which requires int, bool, string, float, an enum type, or another comparable-bounded type parameter)", shown, tp)
			ret = Invalid
		}
	}
	// Numeric bound satisfaction: numeric T must bind to int or float (or another
	// numeric-bounded type variable in the caller's scope).
	for _, tp := range fn.TypeParams {
		if ret == Invalid || anyArgErrored {
			break
		}
		if fn.TypeParamBounds[tp] != "numeric" {
			continue
		}
		if suppressed[tp] {
			continue
		}
		ct, ok := subst[tp]
		if !ok || ct == Invalid {
			continue
		}
		if ct != Int && ct != Float && !c.isNumericTypeVar(ct) {
			pos := boundErrPos(n, tp, origin, typeArgPos)
			shown := disp(ct)
			if isTypeVar(ct) {
				shown = "type parameter " + typeVarName(ct)
			}
			c.errf(pos, "cannot use %s as type parameter %s: it does not satisfy numeric (which requires int or float)", shown, tp)
			ret = Invalid
		}
	}
	// Build TypeSubst for numeric-bounded params, plus a comparable param when
	// bound to float or to a type variable (the type-variable case lets float
	// specialization reach THROUGH a nested comparable generic; a comparable
	// param bound to a concrete non-float stays unrecorded / type-erased).
	var typeSubst map[string]Type
	for _, tp := range fn.TypeParams {
		ct, ok := subst[tp]
		if !ok || ct == Invalid {
			continue
		}
		bound := fn.TypeParamBounds[tp]
		record := bound == "numeric" ||
			(bound == "comparable" && (ct == Float || isTypeVar(ct)))
		if !record {
			continue
		}
		if typeSubst == nil {
			typeSubst = map[string]Type{}
		}
		typeSubst[tp] = ct
	}
	result := ret
	if result != Invalid {
		result = c.applySubst(ret, subst)
	}
	full := make([]ast.Expr, len(fn.Params))
	copy(full, n.Args)
	for i := len(n.Args); i < len(fn.Params); i++ {
		full[i] = fn.Params[i].Default
	}
	c.info.Calls[n] = &CallInfo{
		Kind:      CallUser,
		Func:      fn,
		Mangled:   mangleFunc(modid, fn.Name),
		Args:      full,
		Result:    result,
		TypeSubst: typeSubst,
	}
	return result
}

// boundErrPos picks the source position for a bound-satisfaction diagnostic on
// type parameter tp. An explicit call-site type argument (typeArgPos) wins so the
// error anchors at the argument the user wrote; otherwise the first value argument
// that bound tp (origin); otherwise the callee. origin is never populated for a
// seeded param, so typeArgPos is the only correct anchor for those.
func boundErrPos(n *ast.CallExpr, tp string, origin map[string]int, typeArgPos map[string]token.Position) token.Position {
	if tap, ok := typeArgPos[tp]; ok {
		return tap
	}
	if i, ok := origin[tp]; ok && i < len(n.Args) {
		return n.Args[i].Pos()
	}
	return n.CalleePos
}

// isComparableScalar reports whether t is a comparable, totally-ordered scalar:
// int, bool, string, float, or a value enum. This is the single source of truth
// for scalar comparability across ==, dict keys, switch, contains/index_of/unique,
// and the ordering operators / min / max / sort.
func (c *checker) isComparableScalar(t Type) bool {
	return t == Int || t == Bool || t == String || t == Float || c.isValueEnum(t)
}

// comparableOptional reports whether t is an Optional whose element type supports
// structural ==: int/bool/string, float (numeric identity, codegen/optional.go), or
// a nested comparable Optional. Optional[error], Optional[value-enum], and
// Optional[<aggregate>] are NOT comparable. This deliberately does NOT route through
// isComparableScalar: the codegen-side gate (types.ComparableOptional) is enum-blind,
// so widening the element rule to value enums here would split checker from codegen.
func comparableOptional(t Type) bool {
	if !isOptional(t) {
		return false
	}
	elem := optionalElemType(t)
	return elem == Int || elem == Bool || elem == String || elem == Float || comparableOptional(elem)
}

// noSpecial signals that a special-cased builtin handler did not apply and the
// generic signature path should run. It is a sentinel distinct from any real
// result type.
const noSpecial Type = "\x00noSpecial"

// checkLengthCall handles length(xs) on an array. When the single argument is an
// array it resolves the call to a count (int) and records the CallInfo; codegen
// dispatches on the static argument type. When the argument is not an array it
// returns noSpecial so the generic string-only signature applies.
func (c *checker) checkLengthCall(n *ast.CallExpr) Type {
	if len(n.Args) != 1 {
		return noSpecial
	}
	at := c.checkExpr(n.Args[0])
	if !isArray(at) {
		return noSpecial // string length, or a type error, via the generic path
	}
	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: "length",
		Args:    []ast.Expr{n.Args[0]},
		Result:  Int,
	}
	return Int
}

// checkPushCall handles push(xs, v): xs must be an array, v must match its
// element type. push grows the array in place and returns void (spec 4.3).
func (c *checker) checkPushCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", dispName, len(n.Args))
		return Void
	}
	at := c.info.Types[n.Args[0]]
	if at == Invalid {
		return Void
	}
	if !isArray(at) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be an array, got %s", dispName, at)
		return Void
	}
	et := elemType(at)
	vt := c.info.Types[n.Args[1]]
	if vt != Invalid && et != Invalid && vt != et {
		c.errf(n.Args[1].Pos(), "argument 2 of %s has type %s, want %s (the array element type)", dispName, vt, et)
	}
	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: "push",
		Args:    []ast.Expr{n.Args[0], n.Args[1]},
		Result:  Void,
	}
	return Void
}

// checkHasCall handles has(d, k): d must be a dict, k must match its key type;
// the result is bool (spec 4.4).
func (c *checker) checkHasCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", dispName, len(n.Args))
		return Bool
	}
	dt := c.info.Types[n.Args[0]]
	if dt == Invalid {
		return Bool
	}
	if !isDict(dt) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be a dict, got %s", dispName, dt)
		return Bool
	}
	kt := dictKeyType(dt)
	at := c.info.Types[n.Args[1]]
	if at != Invalid && kt != Invalid && at != kt {
		c.errf(n.Args[1].Pos(), "argument 2 of %s has type %s, want %s (the dict key type)", dispName, at, kt)
	}
	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: "has",
		Args:    []ast.Expr{n.Args[0], n.Args[1]},
		Result:  Bool,
	}
	return Bool
}

// checkKeysCall handles keys(d): d must be a dict; the result is K[], an array
// of its keys in insertion order (spec 4.4).
func (c *checker) checkKeysCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		return Invalid
	}
	dt := c.info.Types[n.Args[0]]
	if dt == Invalid {
		return Invalid
	}
	if !isDict(dt) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be a dict, got %s", dispName, dt)
		return Invalid
	}
	res := arrayType(dictKeyType(dt))
	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: "keys",
		Args:    []ast.Expr{n.Args[0]},
		Result:  res,
	}
	return res
}

// higherOrderArgs type-checks the shared `(xs: T[], f: fn(T) -> U)` shape of
// map/filter/each (M4). It returns the array element type T, the function's
// return type U, and ok. On any structural error it reports it and returns
// ok=false. f must be a function reference of exactly one parameter whose type
// is T (the array element type).
func (c *checker) higherOrderArgs(n *ast.CallExpr, name string) (elem, ret Type, ok bool) {
	if len(n.Args) != 2 {
		for _, a := range n.Args {
			c.checkExpr(a)
		}
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", name, len(n.Args))
		return Invalid, Invalid, false
	}
	xt := c.checkExpr(n.Args[0])
	if xt == Invalid {
		c.checkExpr(n.Args[1])
		return Invalid, Invalid, false
	}
	if !isArray(xt) {
		c.checkExpr(n.Args[1])
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be an array, got %s", name, xt)
		return Invalid, Invalid, false
	}
	et := elemType(xt)
	// Expected-type context for the funcref argument: an overloaded builtin
	// (e.g. abs) referenced bare here is disambiguated from the array element
	// type, matching the design spec's "expected parameter type at the
	// reference site" context rule (mirrors checkUserCallIn's per-param
	// checkExprExpecting for ordinary function calls). The result type is
	// unconstrained here (that is what map/filter/each solve for), so want
	// carries the Invalid-result sentinel; see resolveOverloadedFuncref.
	ft := c.checkExprExpecting(n.Args[1], funcType([]Type{et}, Invalid))
	if ft == Invalid {
		return Invalid, Invalid, false
	}
	if !isFuncref(ft) {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must be a function reference, got %s", name, ft)
		return Invalid, Invalid, false
	}
	params := funcParamTypes(ft)
	if len(params) != 1 {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must take exactly one argument, got %s", name, ft)
		return Invalid, Invalid, false
	}
	if params[0] != et {
		c.errf(n.Args[1].Pos(), "argument 2 of %s takes %s but the array element type is %s", name, params[0], et)
		return Invalid, Invalid, false
	}
	return et, funcRetType(ft), true
}

// checkMapCall handles map(xs: T[], f: fn(T)->U) -> U[] (array form) and
// map(o: Optional[T], f: fn(T)->U) -> Optional[U] /
// map(r: Result[T], f: fn(T)->U) -> Result[U] (combinator forms).
func (c *checker) checkMapCall(n *ast.CallExpr, dispName string) Type {
	// Optional/Result overload: probe arg0 locally BEFORE higherOrderArgs (which
	// errors on non-array arg0). The double checkExpr on the array/error path is
	// intentional and harmless (at most a duplicate diagnostic on an erroring arg0).
	if len(n.Args) == 2 {
		ot := c.checkExpr(n.Args[0])
		if isOptional(ot) || isResult(ot) {
			var et Type
			if isOptional(ot) {
				et = optionalElemType(ot)
			} else {
				et = resultElemType(ot)
			}
			// Expected-type context (see higherOrderArgs): disambiguates an
			// overloaded builtin (e.g. abs) referenced bare as arg2 from the
			// Optional/Result element type; the result type is unconstrained
			// here (map solves for it), hence the Invalid-result sentinel.
			ft := c.checkExprExpecting(n.Args[1], funcType([]Type{et}, Invalid))
			if ft == Invalid {
				return Invalid
			}
			if !isFuncref(ft) {
				c.errf(n.Args[1].Pos(), "argument 2 of %s must be a function reference, got %s", dispName, ft)
				return Invalid
			}
			params := funcParamTypes(ft)
			if len(params) != 1 {
				c.errf(n.Args[1].Pos(), "argument 2 of %s must take exactly one parameter, got %s", dispName, ft)
				return Invalid
			}
			if params[0] != et {
				c.errf(n.Args[1].Pos(), "%s over %s: fn parameter type is %s, want %s", dispName, ot, params[0], et)
				return Invalid
			}
			u := funcRetType(ft)
			if u == Void {
				c.errf(n.Args[1].Pos(), "argument 2 of %s must return a value, got a void function", dispName)
				return Invalid
			}
			var res Type
			if isOptional(ot) {
				res = optionalType(u)
			} else {
				res = resultType(u)
			}
			c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "map", Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: res}
			return res
		}
	}
	// Array path: UNCHANGED from today (higherOrderArgs checkExprs its own args).
	_, u, ok := c.higherOrderArgs(n, dispName)
	if !ok {
		return Invalid
	}
	if u == Void {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must return a value, got a void function", dispName)
		return Invalid
	}
	res := arrayType(u)
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "map", Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: res}
	return res
}

// checkFilterCall handles filter(xs: T[], f: fn(T)->bool) -> T[] (array form) and
// filter(o: Optional[T], f: fn(T)->bool) -> Optional[T] (combinator form).
// filter over Result is explicitly not provided.
func (c *checker) checkFilterCall(n *ast.CallExpr, dispName string) Type {
	// Optional overload: filter(Optional[T], fn(T)->bool) -> Optional[T].
	// filter over Result is explicitly NOT provided (spec section 1). Probe arg0
	// locally; the double checkExpr on the array/error fallthrough is intentional
	// and harmless (see checkMapCall note).
	if len(n.Args) == 2 {
		ot := c.checkExpr(n.Args[0])
		if isOptional(ot) {
			et := optionalElemType(ot)
			// Expected-type context (see higherOrderArgs): filter's result type
			// is always bool, so, unlike map, the full func type is known here
			// and the ordinary exact-match resolution path applies.
			ft := c.checkExprExpecting(n.Args[1], funcType([]Type{et}, Bool))
			if ft == Invalid {
				return Invalid
			}
			if !isFuncref(ft) {
				c.errf(n.Args[1].Pos(), "argument 2 of %s must be a function reference, got %s", dispName, ft)
				return Invalid
			}
			params := funcParamTypes(ft)
			if len(params) != 1 {
				c.errf(n.Args[1].Pos(), "argument 2 of %s must take exactly one parameter, got %s", dispName, ft)
				return Invalid
			}
			if params[0] != et {
				c.errf(n.Args[1].Pos(), "%s over %s: fn parameter type is %s, want %s", dispName, ot, params[0], et)
				return Invalid
			}
			u := funcRetType(ft)
			if u != Bool {
				c.errf(n.Args[1].Pos(), "argument 2 of %s must return bool, got %s", dispName, u)
				return Invalid
			}
			c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "filter", Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: ot}
			return ot
		}
		if isResult(ot) {
			// filter is not defined over Result. Type arg1 too (so info.Types is
			// populated for LSP) before erroring.
			c.checkExpr(n.Args[1])
			c.errf(n.Args[0].Pos(), "%s is not defined over Result (use and_then)", dispName)
			return Invalid
		}
	}
	// Array path: UNCHANGED from today (higherOrderArgs checkExprs its own args).
	t, u, ok := c.higherOrderArgs(n, dispName)
	if !ok {
		return Invalid
	}
	if u != Bool {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must return bool, got %s", dispName, u)
		return Invalid
	}
	res := arrayType(t)
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "filter", Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: res}
	return res
}

// checkEachCall handles each(xs: T[], f: fn(T) -> void) -> void.
func (c *checker) checkEachCall(n *ast.CallExpr, dispName string) Type {
	_, u, ok := c.higherOrderArgs(n, dispName)
	if !ok {
		return Void
	}
	if u != Void {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must return void, got %s", dispName, u)
		return Void
	}
	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: "each",
		Args:    []ast.Expr{n.Args[0], n.Args[1]},
		Result:  Void,
	}
	return Void
}

// checkZipCall handles zip(a: T[], b: U[]) -> (T,U)[].
// zip is a hand-coded generic builtin: its result type is parametric in the
// two input element types. The generic-signature machinery is a future slice.
func (c *checker) checkZipCall(n *ast.CallExpr, dispName string) Type {
	if len(n.Args) != 2 {
		c.errf(n.Pos(), "%s expects two arrays, got %d argument(s)", dispName, len(n.Args))
		c.typeArgs(n.Args)
		return Invalid
	}
	at := c.checkExpr(n.Args[0])
	bt := c.checkExpr(n.Args[1])
	if at == Invalid || bt == Invalid {
		return Invalid
	}
	if !isArray(at) || !isArray(bt) {
		c.errf(n.Pos(), "%s requires two arrays, got %s, %s", dispName, at, bt)
		return Invalid
	}
	res := arrayType(tupleType([]Type{elemType(at), elemType(bt)}))
	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: "zip",
		Args:    []ast.Expr{n.Args[0], n.Args[1]},
		Result:  res,
	}
	return res
}

// checkParseArgsCall handles
// parse_args(args: string[], value_flags: string[]) ->
// ({string:string}, string[], string[]). Like checkZipCall/checkRunFullCall it
// hand-builds a result type the fixed builtin table cannot express (a tuple of
// composites). Both arguments must be string[].
func (c *checker) checkParseArgsCall(n *ast.CallExpr) Type {
	if len(n.Args) != 2 {
		c.errf(n.Pos(), "parse_args expects two arrays, got %d argument(s)", len(n.Args))
		c.typeArgs(n.Args)
		return Invalid
	}
	at := c.checkExpr(n.Args[0])
	bt := c.checkExpr(n.Args[1])
	if at == Invalid || bt == Invalid {
		return Invalid
	}
	strArr := arrayType(String)
	if at != strArr {
		c.errf(n.Args[0].Pos(), "parse_args requires string[] for args, got %s", ast.CanonicalType(ast.TypeName(at)))
		return Invalid
	}
	if bt != strArr {
		c.errf(n.Args[1].Pos(), "parse_args requires string[] for value_flags, got %s", ast.CanonicalType(ast.TypeName(bt)))
		return Invalid
	}
	res := tupleType([]Type{dictType(String, String), arrayType(String), arrayType(String)})
	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: "parse_args",
		Args:    []ast.Expr{n.Args[0], n.Args[1]},
		Result:  res,
	}
	return res
}

// requireStdoutStderr enforces rule 9 for print's `to`. It must be stdout or
// stderr specifically -- not just any reserved constant (Some/None are reserved
// constants too, but are not valid stream targets).
func (c *checker) requireStdoutStderr(e ast.Expr) {
	if id, ok := e.(*ast.Ident); ok && (id.Name == "stdout" || id.Name == "stderr") {
		return
	}
	c.errf(e.Pos(), "print's `to` must be the constant stdout or stderr")
}

func typeInSet(t Type, set []Type) bool {
	for _, s := range set {
		if t == s {
			return true
		}
	}
	return false
}

func joinTypes(ts []Type) string {
	s := ""
	for i, t := range ts {
		if i > 0 {
			s += "|"
		}
		s += string(t)
	}
	return s
}

// defaultArgFor returns the constant default node for a builtin parameter that
// was omitted at the call site. In M1 the only builtin default is print's `to`,
// which defaults to the reserved constant stdout.
func defaultArgFor(builtin string, paramIdx int) ast.Expr {
	if builtin == "print" && paramIdx == 1 {
		return &ast.Ident{Name: "stdout"}
	}
	if builtin == "assert" && paramIdx == 1 {
		// assert(cond, msg = ""): the omitted message is the empty string literal.
		return &ast.StringLit{}
	}
	return nil
}

// checkCombinatorOptResult validates that a 2-arg combinator call has an
// Optional or Result as arg0 and a funcref as arg1. Returns the operand type,
// the funcref type, and true on success; records an error and returns Invalid
// on failure.
func (c *checker) checkCombinatorOptResult(n *ast.CallExpr, name string) (operandType, funcType Type, ok bool) {
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", name, len(n.Args))
		c.typeArgs(n.Args)
		return Invalid, Invalid, false
	}
	ot := c.checkExpr(n.Args[0])
	ft := c.checkExpr(n.Args[1])
	if ot == Invalid || ft == Invalid {
		return Invalid, Invalid, false
	}
	if !isOptional(ot) && !isResult(ot) {
		c.errf(n.Args[0].Pos(), "%s requires an Optional or Result value, got %s", name, ot)
		return Invalid, Invalid, false
	}
	if !isFuncref(ft) {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must be a function reference, got %s", name, ft)
		return Invalid, Invalid, false
	}
	return ot, ft, true
}

// checkAndThenCall handles:
//
//	and_then(Optional[T], fn(T)->Optional[U]) -> Optional[U]
//	and_then(Result[T],   fn(T)->Result[U])   -> Result[U]
func (c *checker) checkAndThenCall(n *ast.CallExpr) Type {
	ot, ft, ok := c.checkCombinatorOptResult(n, "and_then")
	if !ok {
		return Invalid
	}
	params := funcParamTypes(ft)
	ret := funcRetType(ft)
	var et Type
	if isOptional(ot) {
		et = optionalElemType(ot)
	} else {
		et = resultElemType(ot)
	}
	// fn must take exactly one param of element type.
	if len(params) != 1 || params[0] != et {
		c.errf(n.Args[1].Pos(), "and_then: fn must take one parameter of type %s, got %s", et, ft)
		return Invalid
	}
	// fn return type must match the monad constructor (Optional->Optional, Result->Result)
	// with any element type (chain can change element type).
	if isOptional(ot) && !isOptional(ret) {
		c.errf(n.Args[1].Pos(), "and_then over Optional[%s]: fn must have shape fn(%s)->Optional[...], got %s", et, et, ft)
		return Invalid
	}
	if isResult(ot) && !isResult(ret) {
		c.errf(n.Args[1].Pos(), "and_then over Result[%s]: fn must have shape fn(%s)->Result[...], got %s", et, et, ft)
		return Invalid
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "and_then", Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: ret}
	return ret
}

// checkOrElseCall handles:
//
//	or_else(Optional[T], fn()->Optional[T])    -> Optional[T]
//	or_else(Result[T],   fn(error)->Result[T]) -> Result[T]
func (c *checker) checkOrElseCall(n *ast.CallExpr) Type {
	ot, ft, ok := c.checkCombinatorOptResult(n, "or_else")
	if !ok {
		return Invalid
	}
	params := funcParamTypes(ft)
	ret := funcRetType(ft)
	if isOptional(ot) {
		// fn must take no params and return Optional[same elem].
		if len(params) != 0 {
			c.errf(n.Args[1].Pos(), "or_else over Optional: fn must take no parameters, got %s", ft)
			return Invalid
		}
		if !isOptional(ret) {
			c.errf(n.Args[1].Pos(), "or_else over Optional: fn must return Optional[...], got %s", ret)
			return Invalid
		}
		if optionalElemType(ret) != optionalElemType(ot) {
			c.errf(n.Args[1].Pos(), "or_else: fn return element type %s does not match %s", optionalElemType(ret), optionalElemType(ot))
			return Invalid
		}
	} else {
		// Result: fn must take one error param and return Result[same elem].
		if len(params) != 1 || !isErrorType(params[0]) {
			c.errf(n.Args[1].Pos(), "or_else over Result: fn must take one error parameter, got %s", ft)
			return Invalid
		}
		if !isResult(ret) {
			c.errf(n.Args[1].Pos(), "or_else over Result: fn must return Result[...], got %s", ret)
			return Invalid
		}
		if resultElemType(ret) != resultElemType(ot) {
			c.errf(n.Args[1].Pos(), "or_else: fn return element type %s does not match %s", resultElemType(ret), resultElemType(ot))
			return Invalid
		}
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "or_else", Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: ot}
	return ot
}

// checkMapErrCall handles:
//
//	map_err(Result[T], fn(error)->error) -> Result[T]
func (c *checker) checkMapErrCall(n *ast.CallExpr) Type {
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "map_err expects 2 arguments, got %d", len(n.Args))
		c.typeArgs(n.Args)
		return Invalid
	}
	ot := c.checkExpr(n.Args[0])
	ft := c.checkExpr(n.Args[1])
	if ot == Invalid || ft == Invalid {
		return Invalid
	}
	if !isResult(ot) {
		c.errf(n.Args[0].Pos(), "map_err requires a Result value, got %s", ot)
		return Invalid
	}
	if !isFuncref(ft) {
		c.errf(n.Args[1].Pos(), "argument 2 of map_err must be a function reference, got %s", ft)
		return Invalid
	}
	params := funcParamTypes(ft)
	ret := funcRetType(ft)
	if len(params) != 1 || !isErrorType(params[0]) {
		c.errf(n.Args[1].Pos(), "map_err: fn must take one error parameter, got %s", ft)
		return Invalid
	}
	if !isErrorType(ret) {
		c.errf(n.Args[1].Pos(), "map_err: fn must return error, got %s", ret)
		return Invalid
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "map_err", Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: ot}
	return ot
}

// debugRenderable reports whether t is a type debug() can render at codegen
// time. It mirrors genDebugValue's dispatch (internal/codegen/debug.go) so
// the checker's accept set can never be broader than what codegen can
// generate: scalars and a fixed set of opaque leaf types render directly;
// array/Optional/Result/dict/tuple/struct recurse into their element or
// field types; enum, Process, a type variable, and any type this switch does
// not recognize are never renderable, at any nesting depth. visiting tracks
// struct type names currently being descended into, so a self-referential or
// mutually recursive struct terminates the recursion instead of overflowing
// the stack; pass nil for a fresh top-level call.
func (c *checker) debugRenderable(t Type, visiting map[Type]bool) bool {
	switch {
	case t == Invalid, t == Void:
		return false
	case isTypeVar(t):
		return false
	case c.isValueEnum(t):
		return false // top-level subject or struct field: no variant-name render
	case c.isTaggedEnum(t):
		if visiting[t] {
			return false // self-referential or mutually recursive: not renderable
		}
		ei := c.info.Enums[string(t)]
		if ei == nil {
			return false
		}
		if visiting == nil {
			visiting = map[Type]bool{}
		}
		visiting[t] = true
		for _, pt := range ei.Payloads {
			if pt == Invalid {
				continue // no-payload variant
			}
			if c.isValueEnum(pt) {
				// A value-enum PAYLOAD renders by its backing scalar (codegen's
				// debugPayloadType rewrites it to Int/String/Bool before dispatch);
				// it is renderable here even though the general isValueEnum arm above
				// rejects a value enum as a top-level subject or struct field. This is
				// exactly the SC-036/SC-036b path (debug_enum_value_payload_string/_bool).
				continue
			}
			if !c.debugRenderable(pt, visiting) {
				delete(visiting, t)
				return false
			}
		}
		delete(visiting, t)
		return true
	case isProcessType(t):
		return false
	case t == Int, t == Float, t == Bool, t == String:
		return true
	case t == ErrorType, t == RunResult, t == jsonValueType:
		return true
	case isFuncref(t):
		return true
	case isArray(t):
		return c.debugRenderable(elemType(t), visiting)
	case isOptional(t):
		return c.debugRenderable(optionalElemType(t), visiting)
	case isResult(t):
		return c.debugRenderable(resultElemType(t), visiting)
	case isDict(t):
		return c.debugRenderable(dictValType(t), visiting)
	case isTuple(t):
		for _, et := range tupleElemTypes(t) {
			if !c.debugRenderable(et, visiting) {
				return false
			}
		}
		return true
	case c.isStructType(t):
		// visiting[t] on a possibly-nil map is safe: a nil map read returns
		// the zero value (false), never panics, so the nil check below only
		// needs to guard the write, not this read.
		if visiting[t] {
			return false
		}
		si := c.info.Structs[string(t)]
		if si == nil {
			return false
		}
		if visiting == nil {
			visiting = map[Type]bool{}
		}
		visiting[t] = true
		for _, f := range si.Fields {
			if !c.debugRenderable(f.Type, visiting) {
				delete(visiting, t)
				return false
			}
		}
		delete(visiting, t)
		return true
	default:
		return false
	}
}

// checkDebugCall types debug(x) -> string (S4). Accepts a value whose full
// (possibly composite) type debugRenderable confirms codegen can actually
// render; Void, Invalid, and Invalid-adjacent inputs are rejected earlier.
func (c *checker) checkDebugCall(n *ast.CallExpr) Type {
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "debug expects 1 argument, got %d", len(n.Args))
		c.typeArgs(n.Args)
		return Invalid
	}
	at := c.checkExpr(n.Args[0])
	if at == Invalid {
		return Invalid
	}
	if at == Void {
		c.errf(n.Args[0].Pos(), "debug requires a value; got void")
		return Invalid
	}
	// A bare type variable (a generic parameter inside a generic body) has no
	// concrete type at codegen time, so the type-directed renderer cannot be
	// selected. Reject it here, consistent with how string() rejects a typevar.
	if isTypeVar(at) {
		c.errf(n.Args[0].Pos(), "debug cannot render a value of generic type %s; it requires a concrete type", at)
		return Invalid
	}
	// A value enum has no v1 name-rendering; debug would leak its backing scalar.
	if c.isValueEnum(at) {
		c.errf(n.Args[0].Pos(), "debug() is not defined for enum %s (variant-name rendering is not supported); use to_int()/to_string()/to_bool() for the backing value", disp(at))
		return Invalid
	}
	if !c.debugRenderable(at, nil) {
		c.errf(n.Args[0].Pos(), "debug() cannot render %s: it contains a type debug() does not support (enum, Process, a self-referential struct, or an unresolved generic type parameter)", disp(at))
		return Invalid
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "debug", Args: n.Args, Result: String}
	return String
}
