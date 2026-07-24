package types

import (
	"github.com/mitchellnemitz/wisp/internal/ast"
)

// M6 core stdlib builtins whose signatures the fixed builtin table cannot
// express: join (array arg-1), the contains overload (string vs array), the
// numeric overloads abs/min/max, and the generic reverse/reduce. split,
// starts_with, ends_with, index_of, and repeat have exact fixed signatures and
// use the generic checkBuiltinCall path.

// isNumeric reports whether t is int or float. Used by abs/min/max.
func isNumeric(t Type) bool { return t == Int || t == Float }

// checkJoinCall handles join(parts: string[], sep: string) -> string. parts
// must be a string[]; sep must be a string.
func (c *checker) checkJoinCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", dispName, len(n.Args))
		return String
	}
	pt := c.info.Types[n.Args[0]]
	if pt != Invalid {
		if !isArray(pt) || elemType(pt) != String {
			c.errf(n.Args[0].Pos(), "argument 1 of %s must be string[], got %s", dispName, ast.CanonicalType(ast.TypeName(pt)))
		}
	}
	st := c.info.Types[n.Args[1]]
	if st != Invalid && st != String {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must be string, got %s", dispName, st)
	}
	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: "join",
		Args:    []ast.Expr{n.Args[0], n.Args[1]},
		Result:  String,
	}
	return String
}

// checkRunCall handles run(argv: string[]) -> string (M7). argv must be a
// string[], the same shape as join's arg-1, which the fixed builtin table
// cannot express. A non-string[] argument is a compile error.
func (c *checker) checkRunCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		return String
	}
	at := c.info.Types[n.Args[0]]
	if at != Invalid {
		if !isArray(at) || elemType(at) != String {
			c.errf(n.Args[0].Pos(), "argument 1 of %s must be string[], got %s", dispName, ast.CanonicalType(ast.TypeName(at)))
		}
	}
	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: "run",
		Args:    []ast.Expr{n.Args[0]},
		Result:  String,
	}
	return String
}

// checkExecCommandCall handles exec_command(argv: string[]) -> void. Mirrors
// checkRunCall (argv must be a string[], which the fixed builtin table cannot
// express) but the result is Void: exec_command replaces the shell process and
// never returns on success, so it produces no value. It is NOT a terminator --
// the checker has no exit()-style unreachable analysis to apply (see the plan's
// global constraints), so a statement after exec_command(...) is valid.
func (c *checker) checkExecCommandCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		return Void
	}
	at := c.info.Types[n.Args[0]]
	if at != Invalid {
		if !isArray(at) || elemType(at) != String {
			c.errf(n.Args[0].Pos(), "argument 1 of %s must be string[], got %s", dispName, ast.CanonicalType(ast.TypeName(at)))
		}
	}
	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: "exec_command",
		Args:    []ast.Expr{n.Args[0]},
		Result:  Void,
	}
	return Void
}

// checkRunStatusCall handles run_status(argv: string[]) -> int. Mirrors
// checkRunCall: argv must be a string[], which the fixed builtin table cannot
// express. The result is the child's exit code (int), and run_status does NOT
// abort on a nonzero exit (only an empty argv aborts, at runtime).
func (c *checker) checkRunStatusCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		return Int
	}
	at := c.info.Types[n.Args[0]]
	if at != Invalid {
		if !isArray(at) || elemType(at) != String {
			c.errf(n.Args[0].Pos(), "argument 1 of %s must be string[], got %s", dispName, ast.CanonicalType(ast.TypeName(at)))
		}
	}
	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: "run_status",
		Args:    []ast.Expr{n.Args[0]},
		Result:  Int,
	}
	return Int
}

// checkRunFullCall: argv must be a string[]; returns RunResult.
func (c *checker) checkRunFullCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		return RunResult
	}
	at := c.info.Types[n.Args[0]]
	if at != Invalid {
		if !isArray(at) || elemType(at) != String {
			c.errf(n.Args[0].Pos(), "argument 1 of %s must be string[], got %s", dispName, ast.CanonicalType(ast.TypeName(at)))
		}
	}
	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: "run_full",
		Args:    []ast.Expr{n.Args[0]},
		Result:  RunResult,
	}
	return RunResult
}

// checkRunInputCall handles run_input(argv: string[], stdin: string) -> string.
// Mirrors checkRunCall but takes a second string arg (stdin fed to the child).
func (c *checker) checkRunInputCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", dispName, len(n.Args))
		return String
	}
	if at := c.info.Types[n.Args[0]]; at != Invalid {
		if !isArray(at) || elemType(at) != String {
			c.errf(n.Args[0].Pos(), "argument 1 of %s must be string[], got %s", dispName, ast.CanonicalType(ast.TypeName(at)))
		}
	}
	if st := c.info.Types[n.Args[1]]; st != Invalid && st != String {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must be string, got %s", dispName, st)
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "run_input", Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: String}
	return String
}

// checkRunInputFullCall handles run_input_full(argv: string[], stdin: string) -> RunResult.
func (c *checker) checkRunInputFullCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", dispName, len(n.Args))
		return RunResult
	}
	if at := c.info.Types[n.Args[0]]; at != Invalid {
		if !isArray(at) || elemType(at) != String {
			c.errf(n.Args[0].Pos(), "argument 1 of %s must be string[], got %s", dispName, ast.CanonicalType(ast.TypeName(at)))
		}
	}
	if st := c.info.Types[n.Args[1]]; st != Invalid && st != String {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must be string, got %s", dispName, st)
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "run_input_full", Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: RunResult}
	return RunResult
}

// checkPipeCall handles pipe(stages: string[][]) -> RunResult. stages must be an
// array whose element type is string[] (i.e. string[][]); stage count is
// runtime-valued (no array-literal requirement).
func (c *checker) checkPipeCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		return RunResult
	}
	at := c.info.Types[n.Args[0]]
	if at != Invalid && (!isArray(at) || !isArray(elemType(at)) || elemType(elemType(at)) != String) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be string[][], got %s", dispName, ast.CanonicalType(ast.TypeName(at)))
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "pipe", Args: []ast.Expr{n.Args[0]}, Result: RunResult}
	return RunResult
}

// checkRunEnvCall handles run_env(argv: string[], env: {string:string}) -> string.
// argv is a string[] like checkRunCall's single arg; env must be exactly
// {string:string} -- modeled on the dictType(String, String) comparison used by
// parse_args/zip, NOT on checkRunCall (which validates only a single string[]).
// A {int:int} dict IS constructible and must be rejected as the wrong dict type.
func (c *checker) checkRunEnvCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", dispName, len(n.Args))
		return String
	}
	at := c.info.Types[n.Args[0]]
	if at != Invalid {
		if !isArray(at) || elemType(at) != String {
			c.errf(n.Args[0].Pos(), "argument 1 of %s must be string[], got %s", dispName, ast.CanonicalType(ast.TypeName(at)))
		}
	}
	et := c.info.Types[n.Args[1]]
	if et != Invalid && et != dictType(String, String) {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must be {string: string}, got %s", dispName, et)
	}
	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: "run_env",
		Args:    []ast.Expr{n.Args[0], n.Args[1]},
		Result:  String,
	}
	return String
}

// checkRunEnvStatusCall handles run_env_status(argv: string[], env: {string:string}) -> int.
// Mirrors checkRunEnvCall but returns Int (no abort on nonzero, like run_status).
func (c *checker) checkRunEnvStatusCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", dispName, len(n.Args))
		return Int
	}
	at := c.info.Types[n.Args[0]]
	if at != Invalid {
		if !isArray(at) || elemType(at) != String {
			c.errf(n.Args[0].Pos(), "argument 1 of %s must be string[], got %s", dispName, ast.CanonicalType(ast.TypeName(at)))
		}
	}
	et := c.info.Types[n.Args[1]]
	if et != Invalid && et != dictType(String, String) {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must be {string: string}, got %s", dispName, et)
	}
	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: "run_env_status",
		Args:    []ast.Expr{n.Args[0], n.Args[1]},
		Result:  Int,
	}
	return Int
}

// checkRunEnvFullCall handles run_env_full(argv: string[], env: {string:string}) -> RunResult.
// Mirrors checkRunEnvCall but returns RunResult (no abort on nonzero, like run_full).
func (c *checker) checkRunEnvFullCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", dispName, len(n.Args))
		return RunResult
	}
	at := c.info.Types[n.Args[0]]
	if at != Invalid {
		if !isArray(at) || elemType(at) != String {
			c.errf(n.Args[0].Pos(), "argument 1 of %s must be string[], got %s", dispName, ast.CanonicalType(ast.TypeName(at)))
		}
	}
	et := c.info.Types[n.Args[1]]
	if et != Invalid && et != dictType(String, String) {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must be {string: string}, got %s", dispName, et)
	}
	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: "run_env_full",
		Args:    []ast.Expr{n.Args[0], n.Args[1]},
		Result:  RunResult,
	}
	return RunResult
}

// checkContainsCall resolves the overloaded contains builtin (spec 2.1/2.3).
// The overload is chosen by ARG-1 type: a string arg-1 is the substring test
// (string, string) -> bool; an array arg-1 is the membership test (T[], T) ->
// bool with T restricted to the comparable scalar types int/bool/string/float
// and value enums (handle/fn element types are excluded).
// THEN arg-2 is checked against the chosen signature, so a wrong arg-2 yields a
// clear type error rather than a confusing "no matching overload". If arg-1
// itself has no resolved type, that error is already reported and no overload is
// attempted.
func (c *checker) checkContainsCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", dispName, len(n.Args))
		return Bool
	}
	a1 := c.info.Types[n.Args[0]]
	if a1 == Invalid {
		// Arg-1's own error was already reported; do not attempt overload dispatch.
		return Bool
	}

	// Codegen dispatches the string/array variant on the static arg-1 type (like
	// length), so no overload tag needs to be recorded here.
	switch {
	case a1 == String:
		a2 := c.info.Types[n.Args[1]]
		if a2 != Invalid && a2 != String {
			c.errf(n.Args[1].Pos(), "argument 2 of %s must be string (substring test), got %s", dispName, a2)
		}
	case isArray(a1):
		et := elemType(a1)
		if !c.isComparableScalar(et) {
			c.errf(n.Args[0].Pos(), "%s on an array is defined only for comparable element types int/bool/string/float/enum, got [%s]", dispName, et)
			return Bool
		}
		a2 := c.info.Types[n.Args[1]]
		if a2 != Invalid && a2 != et {
			c.errf(n.Args[1].Pos(), "argument 2 of %s has type %s, want %s (the array element type)", dispName, a2, et)
		}
	default:
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be a string or an array, got %s", dispName, a1)
		return Bool
	}

	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: "contains",
		Args:    []ast.Expr{n.Args[0], n.Args[1]},
		Result:  Bool,
	}
	return Bool
}

// checkAssertEqNeCall handles assert_eq/assert_ne [T: comparable](got, want).
// Both operands must share a comparable type: a concrete int/bool/string, or a
// nested comparable Optional (the same rule the == operator enforces). On
// failure codegen renders both via debug(). Returns void.
func (c *checker) checkAssertEqNeCall(n *ast.CallExpr, name string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", name, len(n.Args))
		return Void
	}
	a1 := c.info.Types[n.Args[0]]
	a2 := c.info.Types[n.Args[1]]
	if a1 == Invalid || a2 == Invalid {
		return Void
	}
	if a1 != a2 {
		c.errf(n.Args[1].Pos(), "argument 2 of %s has type %s, want %s (both operands must share one comparable type)", name, a2, a1)
		return Void
	}
	if !c.isComparableScalar(a1) && !comparableOptional(a1) {
		c.errf(n.Args[0].Pos(), "%s requires a comparable value (int, bool, string, float, an enum type, or a nested comparable Optional), got %s", name, a1)
		return Void
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: name, Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: Void}
	return Void
}

// checkAssertOptionalCall handles assert_some/assert_none(o: Optional[T]). On
// failure codegen renders the actual value via debug(). Returns void.
func (c *checker) checkAssertOptionalCall(n *ast.CallExpr, name string) Type {
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", name, len(n.Args))
		c.typeArgs(n.Args)
		return Void
	}
	if _, _, ok := c.optionalArg(n.Args[0], name); !ok {
		return Void
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: name, Args: []ast.Expr{n.Args[0]}, Result: Void}
	return Void
}

// checkAssertResultCall handles assert_ok/assert_err(r: Result[T]). On failure
// codegen renders the actual value via debug(). Returns void.
func (c *checker) checkAssertResultCall(n *ast.CallExpr, name string) Type {
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", name, len(n.Args))
		c.typeArgs(n.Args)
		return Void
	}
	if _, _, ok := c.resultArg(n.Args[0], name); !ok {
		return Void
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: name, Args: []ast.Expr{n.Args[0]}, Result: Void}
	return Void
}

// checkAssertContainsCall mirrors checkContainsCall's arg-0 overload: (string,
// string) substring or (T[], T) element with T comparable (int/bool/string).
// On failure codegen renders the operands via debug(). Returns void.
func (c *checker) checkAssertContainsCall(n *ast.CallExpr) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "assert_contains expects 2 arguments, got %d", len(n.Args))
		return Void
	}
	a1 := c.info.Types[n.Args[0]]
	if a1 == Invalid {
		return Void
	}
	switch {
	case a1 == String:
		a2 := c.info.Types[n.Args[1]]
		if a2 != Invalid && a2 != String {
			c.errf(n.Args[1].Pos(), "argument 2 of assert_contains must be string (substring test), got %s", a2)
		}
	case isArray(a1):
		et := elemType(a1)
		if !c.isComparableScalar(et) {
			c.errf(n.Args[0].Pos(), "assert_contains on an array is defined only for comparable element types int/bool/string/float/enum, got [%s]", et)
			return Void
		}
		a2 := c.info.Types[n.Args[1]]
		if a2 != Invalid && a2 != et {
			c.errf(n.Args[1].Pos(), "argument 2 of assert_contains has type %s, want %s (the array element type)", a2, et)
		}
	default:
		c.errf(n.Args[0].Pos(), "argument 1 of assert_contains must be a string or an array, got %s", a1)
		return Void
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "assert_contains", Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: Void}
	return Void
}

// checkAbsCall handles abs(int) -> int / abs(float) -> float. The result type
// equals the (numeric) argument type.
func (c *checker) checkAbsCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		return Invalid
	}
	at := c.info.Types[n.Args[0]]
	if at == Invalid {
		return Invalid
	}
	if !isNumeric(at) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be int or float, got %s", dispName, at)
		return Invalid
	}
	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: "abs",
		Args:    []ast.Expr{n.Args[0]},
		Result:  at,
	}
	return at
}

// checkMinMaxCall handles min/max(a, b): both args the same ordered scalar type
// (int/float/bool/string/value-enum), result = that shared type. Mixing distinct
// types (int/float, or two different value enums) is a compile error (no implicit
// coercion). The funcref forms of min/max are int/float-only and handled
// elsewhere (overloadedFuncrefArms); this is the direct-call path.
func (c *checker) checkMinMaxCall(n *ast.CallExpr, name, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", dispName, len(n.Args))
		return Invalid
	}
	a1 := c.info.Types[n.Args[0]]
	a2 := c.info.Types[n.Args[1]]
	if a1 == Invalid || a2 == Invalid {
		return Invalid
	}
	if !c.isComparableScalar(a1) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be an ordered scalar type (int, float, bool, string, or a value enum), got %s", dispName, a1)
		return Invalid
	}
	if !c.isComparableScalar(a2) {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must be an ordered scalar type (int, float, bool, string, or a value enum), got %s", dispName, a2)
		return Invalid
	}
	if a1 != a2 {
		c.errf(n.CalleePos, "%s requires both arguments to be the same ordered scalar type, got %s and %s", dispName, a1, a2)
		return Invalid
	}
	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: name,
		Args:    []ast.Expr{n.Args[0], n.Args[1]},
		Result:  a1,
	}
	return a1
}

// checkReverseCall handles reverse(xs: T[]) -> T[] for any element type.
func (c *checker) checkReverseCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		return Invalid
	}
	xt := c.info.Types[n.Args[0]]
	if xt == Invalid {
		return Invalid
	}
	if !isArray(xt) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be an array, got %s", dispName, xt)
		return Invalid
	}
	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: "reverse",
		Args:    []ast.Expr{n.Args[0]},
		Result:  xt,
	}
	return xt
}

// checkReduceCall handles reduce(xs: T[], init: U, f: fn(U, T) -> U) -> U, a
// left fold. U is the type of init; f must take (U, T) and return U. The result
// type is U.
func (c *checker) checkReduceCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 3 {
		c.errf(n.CalleePos, "%s expects 3 arguments, got %d", dispName, len(n.Args))
		return Invalid
	}
	xt := c.info.Types[n.Args[0]]
	u := c.info.Types[n.Args[1]]
	ft := c.info.Types[n.Args[2]]
	if xt == Invalid || u == Invalid || ft == Invalid {
		return Invalid
	}
	if !isArray(xt) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be an array, got %s", dispName, xt)
		return Invalid
	}
	et := elemType(xt)
	if !isFuncref(ft) {
		c.errf(n.Args[2].Pos(), "argument 3 of %s must be a function reference, got %s", dispName, ft)
		return Invalid
	}
	params := funcParamTypes(ft)
	if len(params) != 2 {
		c.errf(n.Args[2].Pos(), "argument 3 of %s must take exactly two arguments (accumulator, element), got %s", dispName, ft)
		return Invalid
	}
	if params[0] != u {
		c.errf(n.Args[2].Pos(), "argument 3 of %s takes accumulator %s but the initial value has type %s", dispName, params[0], u)
		return Invalid
	}
	if params[1] != et {
		c.errf(n.Args[2].Pos(), "argument 3 of %s takes element %s but the array element type is %s", dispName, params[1], et)
		return Invalid
	}
	if funcRetType(ft) != u {
		c.errf(n.Args[2].Pos(), "argument 3 of %s must return %s (the accumulator type), got %s", dispName, u, funcRetType(ft))
		return Invalid
	}
	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: "reduce",
		Args:    []ast.Expr{n.Args[0], n.Args[1], n.Args[2]},
		Result:  u,
	}
	return u
}

// checkOnExitCall handles on_exit(handler: fn()->void) -> void.
// The handler must be a funcref of type EXACTLY fn()->void (nullary, void
// return). A builtin-as-funcref is already a compile error (expr.go:152);
// a wrong-type/wrong-arity handler is a located checker error.
func (c *checker) checkOnExitCall(n *ast.CallExpr) Type {
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "on_exit expects 1 argument, got %d", len(n.Args))
		c.typeArgs(n.Args)
		return Void
	}
	ft := c.checkExpr(n.Args[0])
	if ft == Invalid {
		return Void
	}
	if !isFuncref(ft) {
		c.errf(n.Args[0].Pos(), "on_exit: handler must be a function reference, got %s", ft)
		return Void
	}
	params := funcParamTypes(ft)
	if len(params) != 0 {
		c.errf(n.Args[0].Pos(), "on_exit: handler must take no parameters, got %s", ft)
		return Void
	}
	if funcRetType(ft) != Void {
		c.errf(n.Args[0].Pos(), "on_exit: handler must return void, got %s", ft)
		return Void
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "on_exit", Args: []ast.Expr{n.Args[0]}, Result: Void}
	return Void
}

// onSignalAllowed is the fixed portable allowlist of trappable signal names
// (case-sensitive, no SIG prefix). EXIT is excluded (use on_exit); KILL/STOP are
// untrappable by POSIX. on_signal's sig literal is validated against this set at
// COMPILE time, which keeps the builtin TOTAL (no runtime validation).
var onSignalAllowed = map[string]bool{
	"INT": true, "TERM": true, "HUP": true, "QUIT": true, "USR1": true, "USR2": true,
}

// signalSendAllowed is the allowlist for signal(p, sig): the signals you may
// SEND. It is a SEPARATE map from onSignalAllowed (the TRAP allowlist) and
// deliberately INCLUDES KILL/STOP/CONT, which cannot be trapped but can be sent.
// onSignalAllowed MUST NOT be mutated or widened.
var signalSendAllowed = map[string]bool{
	"INT": true, "TERM": true, "HUP": true, "QUIT": true, "USR1": true, "USR2": true,
	"KILL": true, "STOP": true, "CONT": true,
}

// stringLitText returns the constant text of a string literal that has NO
// interpolation, and ok=true. A constant single- or double-quoted string is a
// StringLit whose parts are all text: one text part (single-quoted, or a
// non-empty double-quoted constant) or zero parts (an empty double-quoted "").
// A non-StringLit node, or a StringLit with any interpolation part, yields
// ok=false. Used by checkOnSignalCall to enforce the literal-sig rule and
// extract the validated value.
func stringLitText(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.StringLit)
	if !ok {
		return "", false
	}
	var text string
	for _, p := range lit.Parts {
		if !p.IsText() {
			return "", false
		}
		text += p.Text
	}
	return text, true
}

// checkOnSignalCall handles on_signal(sig: string, handler: fn()->void) -> void.
// arg0 MUST be a string LITERAL (a single-text-part StringLit, no interpolation);
// its text is validated against onSignalAllowed at COMPILE time. arg1 is a
// funcref of type EXACTLY fn()->void (the same check as checkOnExitCall). All
// failures are located checker errors; the builtin is TOTAL (no runtime path).
func (c *checker) checkOnSignalCall(n *ast.CallExpr) Type {
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "on_signal expects 2 arguments, got %d", len(n.Args))
		c.typeArgs(n.Args)
		return Void
	}
	// arg0: a string literal with a single text part, validated against the allowlist.
	c.checkExpr(n.Args[0])
	sig, ok := stringLitText(n.Args[0])
	if !ok {
		c.errf(n.Args[0].Pos(), "on_signal: signal name must be a string literal")
		c.checkExpr(n.Args[1])
		return Void
	}
	if !onSignalAllowed[sig] {
		c.errf(n.Args[0].Pos(), "on_signal: unsupported signal: %s", sig)
		c.checkExpr(n.Args[1])
		return Void
	}
	// arg1: a funcref of type EXACTLY fn()->void.
	ft := c.checkExpr(n.Args[1])
	if ft == Invalid {
		return Void
	}
	if !isFuncref(ft) {
		c.errf(n.Args[1].Pos(), "on_signal: handler must be a function reference, got %s", ft)
		return Void
	}
	params := funcParamTypes(ft)
	if len(params) != 0 {
		c.errf(n.Args[1].Pos(), "on_signal: handler must take no parameters, got %s", ft)
		return Void
	}
	if funcRetType(ft) != Void {
		c.errf(n.Args[1].Pos(), "on_signal: handler must return void, got %s", ft)
		return Void
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "on_signal", Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: Void}
	return Void
}

// checkSpawnCall handles spawn(argv: string[]) -> Process. argv must be a
// string[]; returns a Process handle.
func (c *checker) checkSpawnCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		return Process
	}
	at := c.info.Types[n.Args[0]]
	if at != Invalid && (!isArray(at) || elemType(at) != String) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be string[], got %s", dispName, ast.CanonicalType(ast.TypeName(at)))
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "spawn", Args: []ast.Expr{n.Args[0]}, Result: Process}
	return Process
}

// checkWaitCall handles wait(p: Process) -> RunResult.
func (c *checker) checkWaitCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		return RunResult
	}
	if at := c.info.Types[n.Args[0]]; at != Invalid && !isProcessType(at) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be Process, got %s", dispName, at)
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "wait", Args: []ast.Expr{n.Args[0]}, Result: RunResult}
	return RunResult
}

// checkIsDoneCall handles is_done(p: Process) -> bool.
func (c *checker) checkIsDoneCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		return Bool
	}
	if at := c.info.Types[n.Args[0]]; at != Invalid && !isProcessType(at) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be Process, got %s", dispName, at)
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "is_done", Args: []ast.Expr{n.Args[0]}, Result: Bool}
	return Bool
}

// checkSignalCall handles signal(p: Process, sig: string-literal) -> void.
// sig MUST be a string literal in signalSendAllowed (mirrors checkOnSignalCall,
// but with the SEND allowlist incl KILL/STOP/CONT).
func (c *checker) checkSignalCall(n *ast.CallExpr, dispName string) Type {
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", dispName, len(n.Args))
		c.typeArgs(n.Args)
		return Void
	}
	if at := c.checkExpr(n.Args[0]); at != Invalid && !isProcessType(at) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be Process, got %s", dispName, at)
	}
	c.checkExpr(n.Args[1])
	sig, ok := stringLitText(n.Args[1])
	if !ok {
		c.errf(n.Args[1].Pos(), "signal: signal name must be a string literal")
		return Void
	}
	if !signalSendAllowed[sig] {
		c.errf(n.Args[1].Pos(), "signal: unsupported signal: %s", sig)
		return Void
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "signal", Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: Void}
	return Void
}

// checkWaitAnyCall handles wait_any(ps: Process[], poll_secs: int) -> Process.
func (c *checker) checkWaitAnyCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", dispName, len(n.Args))
		return Process
	}
	if at := c.info.Types[n.Args[0]]; at != Invalid && (!isArray(at) || !isProcessType(elemType(at))) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be [Process], got %s", dispName, at)
	}
	if pt := c.info.Types[n.Args[1]]; pt != Invalid && pt != Int {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must be int, got %s", dispName, pt)
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "wait_any", Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: Process}
	return Process
}

// checkMakeFifoCall handles make_fifo(path: string) -> void.
func (c *checker) checkMakeFifoCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		return Void
	}
	if pt := c.info.Types[n.Args[0]]; pt != Invalid && pt != String {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be string, got %s", dispName, pt)
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "make_fifo", Args: []ast.Expr{n.Args[0]}, Result: Void}
	return Void
}
