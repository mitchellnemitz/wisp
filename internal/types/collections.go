package types

import "github.com/mitchellnemitz/wisp/internal/ast"

// Collections-core checker handlers. Each has an array/dict/funcref-aware
// signature the fixed builtinSigs table cannot express, so it is special-cased
// here (like map/keys/contains) and records its own CallInfo. The builtinSigs
// entries only reserve the names.

// isOrderedElem reports whether t is an element type with a total order usable by
// `sort` (int, float, string).
func isOrderedElem(t Type) bool {
	return t == Int || t == Float || t == String
}

// arrayBuiltinArg checks that arg is an array and returns its (type, elem, ok).
func (c *checker) arrayBuiltinArg(arg ast.Expr, name string) (Type, Type, bool) {
	at := c.checkExpr(arg)
	if at == Invalid {
		return Invalid, Invalid, false
	}
	if !isArray(at) {
		c.errf(arg.Pos(), "argument 1 of %s must be an array, got %s", name, at)
		return Invalid, Invalid, false
	}
	return at, elemType(at), true
}

func (c *checker) checkSortCall(n *ast.CallExpr, dispName string) Type {
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		c.typeArgs(n.Args)
		return Invalid
	}
	at, et, ok := c.arrayBuiltinArg(n.Args[0], dispName)
	if !ok {
		return Invalid
	}
	if c.rejectTypeVar(n.Args[0].Pos(), et, "sort") {
		return Invalid
	}
	if !isOrderedElem(et) {
		c.errf(n.Args[0].Pos(), "%s requires an array of int, float, or string, got %s", dispName, at)
		return Invalid
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "sort", Args: []ast.Expr{n.Args[0]}, Result: at}
	return at
}

func (c *checker) checkSortByCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", dispName, len(n.Args))
		return Invalid
	}
	at := c.info.Types[n.Args[0]]
	ft := c.info.Types[n.Args[1]]
	if at == Invalid || ft == Invalid {
		return Invalid
	}
	if !isArray(at) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be an array, got %s", dispName, at)
		return Invalid
	}
	et := elemType(at)
	want := funcType([]Type{et, et}, Bool)
	if ft != want {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must be %s, got %s", dispName, want, ft)
		return Invalid
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "sort_by", Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: at}
	return at
}

// checkFindAnyAllCall handles the shared (xs: T[], f: fn(T) -> bool) shape of
// find/any/all; result is the caller-supplied type (Int for find, Bool for any/all).
func (c *checker) checkFindAnyAllCall(n *ast.CallExpr, name, dispName string, result Type) Type {
	_, _, ok := c.higherOrderArgs(n, dispName)
	if !ok {
		return result
	}
	// higherOrderArgs already required f to be fn(T)->U with the array element type;
	// for find/any/all U must be bool.
	ft := c.info.Types[n.Args[1]]
	if funcRetType(ft) != Bool {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must return bool, got %s", dispName, funcRetType(ft))
		return result
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: name, Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: result}
	return result
}

func (c *checker) checkSliceCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 3 {
		c.errf(n.CalleePos, "%s expects 3 arguments, got %d", dispName, len(n.Args))
		return Invalid
	}
	at := c.info.Types[n.Args[0]]
	if at == Invalid {
		return Invalid
	}
	if !isArray(at) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be an array, got %s", dispName, at)
		return Invalid
	}
	for i := 1; i <= 2; i++ {
		if t := c.info.Types[n.Args[i]]; t != Invalid && t != Int {
			c.errf(n.Args[i].Pos(), "argument %d of %s must be int, got %s", i+1, dispName, t)
		}
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "slice", Args: []ast.Expr{n.Args[0], n.Args[1], n.Args[2]}, Result: at}
	return at
}

func (c *checker) checkConcatCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", dispName, len(n.Args))
		return Invalid
	}
	a := c.info.Types[n.Args[0]]
	b := c.info.Types[n.Args[1]]
	if a == Invalid || b == Invalid {
		return Invalid
	}
	if !isArray(a) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be an array, got %s", dispName, a)
		return Invalid
	}
	if !isArray(b) {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must be an array, got %s", dispName, b)
		return Invalid
	}
	if a != b {
		c.errf(n.CalleePos, "%s requires arrays of the same element type, got %s and %s", dispName, a, b)
		return Invalid
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "concat", Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: a}
	return a
}

func (c *checker) checkSumCall(n *ast.CallExpr, dispName string) Type {
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		c.typeArgs(n.Args)
		return Invalid
	}
	at, et, ok := c.arrayBuiltinArg(n.Args[0], dispName)
	if !ok {
		return Invalid
	}
	if c.rejectTypeVar(n.Args[0].Pos(), et, "sum") {
		return Invalid
	}
	if et != Int && et != Float {
		c.errf(n.Args[0].Pos(), "%s requires an array of int or float, got %s", dispName, at)
		return Invalid
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "sum", Args: []ast.Expr{n.Args[0]}, Result: et}
	return et
}

func (c *checker) checkRangeCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		return arrayType(Int)
	}
	if t := c.info.Types[n.Args[0]]; t != Invalid && t != Int {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be int, got %s", dispName, t)
	}
	res := arrayType(Int)
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "range", Args: []ast.Expr{n.Args[0]}, Result: res}
	return res
}

func (c *checker) checkFirstLastCall(n *ast.CallExpr, name, dispName string) Type {
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		c.typeArgs(n.Args)
		return Invalid
	}
	_, et, ok := c.arrayBuiltinArg(n.Args[0], dispName)
	if !ok {
		return Invalid
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: name, Args: []ast.Expr{n.Args[0]}, Result: et}
	return et
}

// dictBuiltinArg checks that arg is a dict and returns (type, key, val, ok).
func (c *checker) dictBuiltinArg(arg ast.Expr, name string) (Type, Type, Type, bool) {
	dt := c.checkExpr(arg)
	if dt == Invalid {
		return Invalid, Invalid, Invalid, false
	}
	if !isDict(dt) {
		c.errf(arg.Pos(), "argument 1 of %s must be a dict, got %s", name, dt)
		return Invalid, Invalid, Invalid, false
	}
	return dt, dictKeyType(dt), dictValType(dt), true
}

func (c *checker) checkValuesCall(n *ast.CallExpr, dispName string) Type {
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		c.typeArgs(n.Args)
		return Invalid
	}
	_, _, vt, ok := c.dictBuiltinArg(n.Args[0], dispName)
	if !ok {
		return Invalid
	}
	res := arrayType(vt)
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "values", Args: []ast.Expr{n.Args[0]}, Result: res}
	return res
}

func (c *checker) checkGetOrCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 3 {
		c.errf(n.CalleePos, "%s expects 3 arguments, got %d", dispName, len(n.Args))
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
	kt, vt := dictKeyType(dt), dictValType(dt)
	if t := c.info.Types[n.Args[1]]; t != Invalid && t != kt {
		c.errf(n.Args[1].Pos(), "argument 2 of %s has type %s, want %s (the dict key type)", dispName, t, kt)
	}
	if t := c.info.Types[n.Args[2]]; t != Invalid && t != vt {
		c.errf(n.Args[2].Pos(), "argument 3 of %s has type %s, want %s (the dict value type)", dispName, t, vt)
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "get_or", Args: []ast.Expr{n.Args[0], n.Args[1], n.Args[2]}, Result: vt}
	return vt
}

// checkGetCall handles get(d, k) -> Optional[V] (spec 7): the value wrapped in
// Some if k is present, else None. V is decoded from the dict's value type.
func (c *checker) checkGetCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", dispName, len(n.Args))
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
	kt, vt := dictKeyType(dt), dictValType(dt)
	if t := c.info.Types[n.Args[1]]; t != Invalid && t != kt {
		c.errf(n.Args[1].Pos(), "argument 2 of %s has type %s, want %s (the dict key type)", dispName, t, kt)
	}
	res := optionalType(vt)
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "get", Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: res}
	return res
}

func (c *checker) checkRemoveCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", dispName, len(n.Args))
		return Void
	}
	dt := c.info.Types[n.Args[0]]
	if dt == Invalid {
		return Void
	}
	if !isDict(dt) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be a dict, got %s", dispName, dt)
		return Void
	}
	kt := dictKeyType(dt)
	if t := c.info.Types[n.Args[1]]; t != Invalid && t != kt {
		c.errf(n.Args[1].Pos(), "argument 2 of %s has type %s, want %s (the dict key type)", dispName, t, kt)
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "remove", Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: Void}
	return Void
}

func (c *checker) checkMergeCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", dispName, len(n.Args))
		return Invalid
	}
	a := c.info.Types[n.Args[0]]
	b := c.info.Types[n.Args[1]]
	if a == Invalid || b == Invalid {
		return Invalid
	}
	if !isDict(a) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be a dict, got %s", dispName, a)
		return Invalid
	}
	if !isDict(b) {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must be a dict, got %s", dispName, b)
		return Invalid
	}
	if a != b {
		c.errf(n.CalleePos, "%s requires dicts of the same type, got %s and %s", dispName, a, b)
		return Invalid
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "merge", Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: a}
	return a
}

// checkClampCall handles clamp(x, lo, hi): all three the same numeric type
// (int or float), result that type.
func (c *checker) checkClampCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 3 {
		c.errf(n.CalleePos, "%s expects 3 arguments, got %d", dispName, len(n.Args))
		return Invalid
	}
	t := c.info.Types[n.Args[0]]
	if t == Invalid {
		return Invalid
	}
	if t != Int && t != Float {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be int or float, got %s", dispName, t)
		return Invalid
	}
	for i := 1; i <= 2; i++ {
		at := c.info.Types[n.Args[i]]
		if at != Invalid && at != t {
			c.errf(n.Args[i].Pos(), "%s requires all arguments to be the same numeric type, got %s and %s", dispName, t, at)
			return Invalid
		}
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "clamp", Args: []ast.Expr{n.Args[0], n.Args[1], n.Args[2]}, Result: t}
	return t
}

// isComparableElem reports whether t is one of int/bool/string - the element types
// valid for contains (array branch), index_of (array branch), and unique.
func isComparableElem(t Type) bool { return t == Int || t == Bool || t == String }

// checkIndexOfCall resolves the overloaded index_of builtin (spec I4).
// The overload is chosen by the first argument's type (n.Args[0]): a string
// first arg is the substring search (string, string) -> Optional[int]; an array
// first arg is the element search (T[], T) -> Optional[int] with T restricted to
// int/bool/string (same guard as contains). Same dispatch pattern as
// checkContainsCall (which dispatches on the same Args[0] slot).
func (c *checker) checkIndexOfCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	res := optionalType(Int)
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", dispName, len(n.Args))
		return res
	}
	a1 := c.info.Types[n.Args[0]]
	if a1 == Invalid {
		return res
	}

	switch {
	case a1 == String:
		a2 := c.info.Types[n.Args[1]]
		if a2 != Invalid && a2 != String {
			c.errf(n.Args[1].Pos(), "argument 2 of %s must be string (substring search), got %s", dispName, a2)
		}
	case isArray(a1):
		et := elemType(a1)
		if !isComparableElem(et) && !c.isEnumType(et) {
			c.errf(n.Args[0].Pos(), "%s on an array is defined only for comparable element types int/bool/string/enum, got [%s]", dispName, et)
			return res
		}
		a2 := c.info.Types[n.Args[1]]
		if a2 != Invalid && a2 != et {
			c.errf(n.Args[1].Pos(), "argument 2 of %s has type %s, want %s (the array element type)", dispName, a2, et)
		}
	default:
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be a string or an array, got %s", dispName, a1)
		return res
	}

	c.info.Calls[n] = &CallInfo{
		Kind:    CallBuiltin,
		Builtin: "index_of",
		Args:    []ast.Expr{n.Args[0], n.Args[1]},
		Result:  res,
	}
	return res
}

// checkCountWhereCall handles count_where(xs: T[], f: fn(T)->bool) -> int.
func (c *checker) checkCountWhereCall(n *ast.CallExpr, dispName string) Type {
	_, u, ok := c.higherOrderArgs(n, dispName)
	if !ok {
		return Int
	}
	if u != Bool {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must return bool, got %s", dispName, u)
		return Int
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "count_where", Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: Int}
	return Int
}

// checkFlattenCall handles flatten(xs: T[][]) -> T[].
func (c *checker) checkFlattenCall(n *ast.CallExpr, dispName string) Type {
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		c.typeArgs(n.Args)
		return Invalid
	}
	at := c.checkExpr(n.Args[0])
	if at == Invalid {
		return Invalid
	}
	if !isArray(at) {
		c.errf(n.Args[0].Pos(), "%s requires an array of arrays, got %s", dispName, at)
		return Invalid
	}
	et := elemType(at)
	if !isArray(et) {
		c.errf(n.Args[0].Pos(), "%s requires an array of arrays, got %s", dispName, at)
		return Invalid
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "flatten", Args: []ast.Expr{n.Args[0]}, Result: et}
	return et
}

// checkUniqueCall handles unique(xs: T[]) -> T[]. T must be int/bool/string.
func (c *checker) checkUniqueCall(n *ast.CallExpr, dispName string) Type {
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		c.typeArgs(n.Args)
		return Invalid
	}
	at := c.checkExpr(n.Args[0])
	if at == Invalid {
		return Invalid
	}
	if !isArray(at) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be an array, got %s", dispName, at)
		return Invalid
	}
	et := elemType(at)
	if !isComparableElem(et) && !c.isEnumType(et) {
		c.errf(n.Args[0].Pos(), "%s on an array is defined only for comparable element types int/bool/string/enum, got [%s]", dispName, et)
		return Invalid
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "unique", Args: []ast.Expr{n.Args[0]}, Result: at}
	return at
}

// checkTakeDropCall handles take/drop(xs: T[], n: int) -> T[].
func (c *checker) checkTakeDropCall(n *ast.CallExpr, name, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", dispName, len(n.Args))
		return Invalid
	}
	at := c.info.Types[n.Args[0]]
	if at == Invalid {
		return Invalid
	}
	if !isArray(at) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be an array, got %s", dispName, at)
		return Invalid
	}
	if nt := c.info.Types[n.Args[1]]; nt != Invalid && nt != Int {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must be int, got %s", dispName, nt)
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: name, Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: at}
	return at
}

// checkPopCall handles pop(xs: T[]) -> T.
func (c *checker) checkPopCall(n *ast.CallExpr, dispName string) Type {
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		c.typeArgs(n.Args)
		return Invalid
	}
	_, et, ok := c.arrayBuiltinArg(n.Args[0], dispName)
	if !ok {
		return Invalid
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "pop", Args: []ast.Expr{n.Args[0]}, Result: et}
	return et
}

// checkRemoveAtCall handles remove_at(xs: T[], i: int) -> void.
func (c *checker) checkRemoveAtCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 2 {
		c.errf(n.CalleePos, "%s expects 2 arguments, got %d", dispName, len(n.Args))
		return Void
	}
	at := c.info.Types[n.Args[0]]
	if at != Invalid && !isArray(at) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be an array, got %s", dispName, at)
		return Void
	}
	if it := c.info.Types[n.Args[1]]; it != Invalid && it != Int {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must be int, got %s", dispName, it)
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "remove_at", Args: []ast.Expr{n.Args[0], n.Args[1]}, Result: Void}
	return Void
}

// checkInsertAtCall handles insert_at(xs: T[], i: int, v: T) -> void.
func (c *checker) checkInsertAtCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	if len(n.Args) != 3 {
		c.errf(n.CalleePos, "%s expects 3 arguments, got %d", dispName, len(n.Args))
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
	if it := c.info.Types[n.Args[1]]; it != Invalid && it != Int {
		c.errf(n.Args[1].Pos(), "argument 2 of %s must be int, got %s", dispName, it)
	}
	if vt := c.info.Types[n.Args[2]]; vt != Invalid && et != Invalid && vt != et {
		c.errf(n.Args[2].Pos(), "argument 3 of %s has type %s, want %s (the array element type)", dispName, vt, et)
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "insert_at", Args: []ast.Expr{n.Args[0], n.Args[1], n.Args[2]}, Result: Void}
	return Void
}

// checkSizeCall handles size(d: {K:V}) -> int.
func (c *checker) checkSizeCall(n *ast.CallExpr, dispName string) Type {
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		c.typeArgs(n.Args)
		return Int
	}
	dt := c.checkExpr(n.Args[0])
	if dt != Invalid && !isDict(dt) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be a dict, got %s", dispName, dt)
		return Int
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "size", Args: []ast.Expr{n.Args[0]}, Result: Int}
	return Int
}

// checkClearCall handles clear(d: {K:V}) -> void.
func (c *checker) checkClearCall(n *ast.CallExpr, dispName string) Type {
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		c.typeArgs(n.Args)
		return Void
	}
	dt := c.checkExpr(n.Args[0])
	if dt != Invalid && !isDict(dt) {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be a dict, got %s", dispName, dt)
		return Void
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "clear", Args: []ast.Expr{n.Args[0]}, Result: Void}
	return Void
}

// checkSignCall handles sign(x): x int or float, result always int.
func (c *checker) checkSignCall(n *ast.CallExpr, dispName string) Type {
	for _, a := range n.Args {
		c.checkExpr(a)
	}
	// On any arity/type error return Invalid (not Int), matching abs/min/max, so
	// follow-on checking does not proceed as if sign() were well-typed.
	if len(n.Args) != 1 {
		c.errf(n.CalleePos, "%s expects 1 argument, got %d", dispName, len(n.Args))
		return Invalid
	}
	t := c.info.Types[n.Args[0]]
	if t == Invalid {
		return Invalid
	}
	if t != Int && t != Float {
		c.errf(n.Args[0].Pos(), "argument 1 of %s must be int or float, got %s", dispName, t)
		return Invalid
	}
	c.info.Calls[n] = &CallInfo{Kind: CallBuiltin, Builtin: "sign", Args: []ast.Expr{n.Args[0]}, Result: Int}
	return Int
}
