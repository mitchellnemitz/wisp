package types

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/ast"
	"github.com/mitchellnemitz/wisp/internal/token"
)

// checkExpr resolves the type of e, records it in Info.Types, and returns it.
// On a type error it records the diagnostic and returns Invalid (or the most
// useful type for continued checking).
func (c *checker) checkExpr(e ast.Expr) Type {
	return c.checkExprExpecting(e, Invalid)
}

// checkExprExpecting is checkExpr with a contextual expected type. An empty
// array literal `[]` consults it for its element type, which cannot otherwise
// be inferred. A bare identifier naming an overloaded or generic higher-order
// builtin (checkIdent) also consults it to select an overload/instantiation
// from context (a funcref-typed annotation, an argument position, or a
// higher-order builtin's expected parameter type). All other expressions
// ignore it.
func (c *checker) checkExprExpecting(e ast.Expr, want Type) Type {
	t := c.exprType(e, want)
	c.info.Types[e] = t
	return t
}

func (c *checker) exprType(e ast.Expr, want Type) Type {
	switch n := e.(type) {
	case *ast.IntLit:
		// Validate magnitude against the int64 range, same mechanism as the const
		// path (foldConst). Without this, codegen emits out-of-range digits
		// verbatim into $(( )) and the shell result diverges. Raw holds the
		// unsigned magnitude; INT_MIN reaches the checker as unary minus over Raw
		// and is validated in checkUnary (which parses the signed magnitude).
		if _, err := parseWispInt(n.Raw, false); err != nil {
			c.errf(n.LitPos, "integer literal out of range: %q", n.Raw)
			return Invalid
		}
		return Int
	case *ast.FloatLit:
		if err := floatLitInDomain(n.Raw); err != nil {
			c.errf(n.LitPos, "float literal out of domain: %q", n.Raw)
			return Invalid
		}
		return Float
	case *ast.BoolLit:
		return Bool
	case *ast.StringLit:
		return c.checkStringLit(n)
	case *ast.Ident:
		return c.checkIdent(n, want)
	case *ast.UnaryExpr:
		return c.checkUnary(n)
	case *ast.BinaryExpr:
		return c.checkBinary(n)
	case *ast.CallExpr:
		return c.checkCall(n)
	case *ast.StructLit:
		return c.checkStructLit(n, want)
	case *ast.ArrayLit:
		return c.checkArrayLit(n, want)
	case *ast.DictLit:
		return c.checkDictLit(n, want)
	case *ast.FieldAccess:
		return c.checkFieldAccess(n, want)
	case *ast.TupleLit:
		return c.checkTupleLit(n)
	case *ast.IndexExpr:
		return c.checkIndexExpr(n)
	default:
		c.errf(e.Pos(), "internal: unhandled expression node %T (checker drift)", e)
		return Invalid
	}
}

func (c *checker) checkStringLit(n *ast.StringLit) Type {
	// Interpolation parts may be any value type: ${} is the explicit render
	// marker and auto-stringifies (spec 5.1). We still type-check each embedded
	// expression so codegen sees its type; void is not renderable.
	for _, part := range n.Parts {
		if part.IsText() {
			continue
		}
		t := c.checkExpr(part.Expr)
		if c.rejectTypeVar(part.Expr.Pos(), t, "string interpolation") {
			continue
		} else if t == Void {
			c.errf(part.Expr.Pos(), "cannot interpolate a void value")
		} else if isFuncref(t) {
			// Function references are opaque (spec 2.4): no conversion to string,
			// including the auto-stringify of ${} interpolation.
			c.errf(part.Expr.Pos(), "cannot interpolate a function reference (it is opaque)")
		} else if isErrorType(t) {
			// The error handle is opaque (spec 4.1): no string conversion. Its
			// message field is renderable; the handle itself is not.
			c.errf(part.Expr.Pos(), "cannot interpolate an error value (it is opaque); use e.message")
		} else if isOptional(t) {
			c.errf(part.Expr.Pos(), "cannot interpolate an Optional value (it is opaque); use is_some/unwrap")
		} else if isRunResultType(t) {
			c.errf(part.Expr.Pos(), "cannot interpolate a RunResult value (it is opaque); use .stdout/.stderr/.code")
		} else if isProcessType(t) {
			c.errf(part.Expr.Pos(), "cannot interpolate a Process value (it is opaque); use .pid")
		} else if t == jsonValueType {
			c.errf(part.Expr.Pos(), "cannot interpolate a json.Value (it is opaque); use json.encode or an accessor")
		} else if isArray(t) {
			c.errf(part.Expr.Pos(), "cannot interpolate an array value (it is opaque); use debug(xs) or join")
		} else if isDict(t) {
			c.errf(part.Expr.Pos(), "cannot interpolate a dict value (it is opaque); use debug(d)")
		} else if isTuple(t) {
			c.errf(part.Expr.Pos(), "cannot interpolate a tuple value (it is opaque); use debug(t)")
		} else if isResult(t) {
			c.errf(part.Expr.Pos(), "cannot interpolate a Result value (it is opaque); use debug(r) or match")
		} else if c.isStructType(t) {
			c.errf(part.Expr.Pos(), "cannot interpolate a struct value (it is opaque); use debug(v)")
		}
	}
	return String
}

func (c *checker) checkIdent(n *ast.Ident, want Type) Type {
	if n.Name == "_" {
		c.errf(n.NamePos, "cannot use _ as a value")
		return Invalid
	}
	if n.Name == "None" {
		c.noneNeedsContext(n.NamePos)
		return Invalid
	}
	if n.Name == "Some" {
		c.errf(n.NamePos, "Some is a constructor and must be called: Some(x)")
		return Invalid
	}
	if isReservedConstant(n.Name) {
		// stdout/stderr usable as int values (their primary use is print's `to`,
		// validated separately, but they are int constants).
		return reservedConstants[n.Name]
	}
	if v := c.lookup(n.Name); v != nil {
		v.Used = true
		c.info.Uses[n] = v
		return v.Type
	}
	// A top-level const name is not in the scope stack, but it is a valid
	// value of its declared type. Record a Uses entry pointing at the top-level
	// const Var so codegen and LSP can find the declaration.
	if tv := c.cur.topConsts[n.Name]; tv != nil {
		c.info.Uses[n] = tv
		return tv.Type
	}
	// A bare name in value position that resolves to a declared function is a
	// function reference (M4, spec 2.2): record the target's mangled name and the
	// full-arity funcref type so codegen emits the name. The no-shadow rule
	// guarantees no local can share a function's name, so the order (local first,
	// then function) is unambiguous.
	if fn, ok := c.cur.funcs[n.Name]; ok {
		if len(fn.TypeParams) > 0 {
			c.errf(n.NamePos, "generic function %q cannot be used as a function reference; call it directly", n.Name)
			return Invalid
		}
		ft := c.funcRefType(fn)
		c.info.FuncRefs[n] = &FuncRef{Mangled: mangleFunc(c.cur.id, fn.Name), Type: ft}
		return ft
	}
	// A module namespace alias is not a value; only `alias.member` is valid (M8).
	if _, ok := c.cur.namespaces[n.Name]; ok {
		c.errf(n.NamePos, "%q is a module namespace, not a value", n.Name)
		return Invalid
	}
	// Referencing a builtin name as a value: allowlisted builtins record a FuncRef
	// (eta-expansion to __wisp_builtin_<name>); all others are a compile error with
	// a reason-branched message (spec 2.2 + builtin-funcrefs spec).
	if isBuiltin(n.Name) {
		// A modularized builtin's bare value-reference was removed along with its
		// bare call: suggest the module home instead of synthesizing a funcref or
		// emitting the "cannot be referenced" message. User bindings (local var,
		// top const, user func) are resolved above, so this fires only for an
		// otherwise-unbound removable name.
		if hint, ok := removedHint(n.Name); ok {
			ns, _, _ := strings.Cut(hint, ".")
			c.errf(n.NamePos, "%q was moved to a module; import %q and call it as %s(...)", n.Name, ns, hint)
			return Invalid
		}
		if builtinFuncrefGeneratable[n.Name] {
			ft := builtinFuncrefType(n.Name)
			c.info.FuncRefs[n] = &FuncRef{Mangled: builtinFuncrefMangled(n.Name), Type: ft}
			return ft
		}
		// Overloaded builtin (abs/min/max/clamp/sign): pick the int or float arm
		// from the expected funcref type. No context (want == Invalid), or a want
		// that matches no arm, is a compile error naming the ambiguity.
		if _, ok := overloadedFuncrefArms[n.Name]; ok {
			if ft, mangled, ok := resolveOverloadedFuncref(n.Name, want); ok {
				c.info.FuncRefs[n] = &FuncRef{Mangled: mangled, Type: ft}
				return ft
			} else if want != Invalid && isFuncref(want) && funcRetType(want) != Invalid {
				// F9 misfire fix: a usable annotation was present but matched no arm.
				c.errf(n.NamePos, "%q has no function-reference form matching %s; supported: %s",
					n.Name, disp(want), joinedOverloadedArms(n.Name))
				return Invalid
			} else {
				c.errf(n.NamePos, "reference to overloaded builtin %q needs a function-reference type annotation to select an overload", n.Name)
				return Invalid
			}
		}
		// Generic higher-order builtin (map/filter/each/reduce/sort_by/find/any/
		// all/count_where/and_then/or_else/map_err): pick the container-shape axis
		// (array/Optional/Result) from the expected funcref type. No context, or a
		// want that matches no axis, is a compile error naming the ambiguity.
		if _, ok := genericFuncrefAxes[n.Name]; ok {
			if ft, mangled, ok := resolveGenericFuncref(n.Name, want); ok {
				c.info.FuncRefs[n] = &FuncRef{Mangled: mangled, Type: ft}
				return ft
			} else if want != Invalid && isFuncref(want) && funcRetType(want) != Invalid {
				c.errf(n.NamePos, "%q has no function-reference form matching %s; supported containers: %s",
					n.Name, disp(want), joinedGenericAxisNames(n.Name))
				return Invalid
			} else {
				c.errf(n.NamePos, "reference to generic builtin %q needs a function-reference type annotation to select a container overload", n.Name)
				return Invalid
			}
		}
		// reason-branched message for non-referenceable builtins (see
		// funcrefRejectReason, funcref_class.go, shared with the :819
		// qualified-member path).
		reason := funcrefRejectReason(n.Name)
		c.errf(n.NamePos, "builtin %q cannot be referenced as a function value (%s); wrap it in a fn", n.Name, reason)
		return Invalid
	}
	if isCoreModule(n.Name) {
		c.errf(n.NamePos, "module %q is not imported; add import %q", n.Name, n.Name)
		return Invalid
	}
	c.errf(n.NamePos, "undeclared name %q%s", n.Name, suggestSuffix(n.Name, c.varNamesInScope()))
	return Invalid
}

// rejectTypeVar reports a bare type variable at an operation needing a concrete
// type, naming the parameter, and returns true if it fired.
func (c *checker) rejectTypeVar(pos token.Position, t Type, op string) bool {
	if isTypeVar(t) {
		if c.typeParamBounds[typeVarName(t)] != "" {
			c.errf(pos, "%s is not yet supported for a bounded type parameter %s", op, typeVarName(t))
		} else {
			c.errf(pos, "%s is not allowed on type parameter %s (it has no concrete type)", op, typeVarName(t))
		}
		return true
	}
	return false
}

func (c *checker) checkUnary(n *ast.UnaryExpr) Type {
	// Special case: unary minus directly over an integer literal. Raw holds the
	// unsigned magnitude, so for INT_MIN (-9223372036854775808) the magnitude
	// alone is one past INT_MAX and only the negated form is in range. Parse the
	// magnitude WITH the sign applied (mirroring foldUnary) so INT_MIN is accepted
	// while any other out-of-range magnitude is rejected, and skip the bare-IntLit
	// check that would otherwise reject the valid INT_MIN magnitude.
	if n.Op == token.Minus {
		if il, ok := n.X.(*ast.IntLit); ok {
			if _, err := parseWispInt(il.Raw, true); err != nil {
				c.errf(il.LitPos, "integer literal out of range: %q", "-"+il.Raw)
				c.info.Types[n.X] = Invalid
				return Invalid
			}
			c.info.Types[n.X] = Int
			return Int
		}
	}
	t := c.checkExpr(n.X)
	switch n.Op {
	case token.Minus:
		if c.isNumericTypeVar(t) {
			return t
		}
		if c.rejectTypeVar(n.OpPos, t, "unary -") {
			return Invalid
		}
		if t == Int {
			return Int
		}
		if t == Float {
			return Float
		}
		if t != Invalid {
			c.errf(n.OpPos, "unary - requires int or float, got %s", t)
		}
		// Recover as int (the most common case) so checking continues.
		return Invalid
	case token.Bang:
		if t != Invalid && t != Bool {
			c.errf(n.OpPos, "unary ! requires bool, got %s", t)
			return Invalid
		}
		return Bool
	default:
		c.errf(n.OpPos, "internal: unhandled unary operator %s (checker drift)", n.Op)
		return Invalid
	}
}

func (c *checker) checkBinary(n *ast.BinaryExpr) Type {
	lt := c.checkExpr(n.L)
	rt := c.checkExpr(n.R)
	if lt == Invalid || rt == Invalid {
		// Recover with the operator's natural result type where unambiguous.
		return binaryResultRecover(n.Op)
	}
	// Bitwise ops require concrete int -- reject ALL type-var operands before the
	// numeric-typevar branch (which would otherwise accept a numeric T that could
	// be instantiated as float, and would return T instead of Int).
	switch n.Op {
	case token.Amp, token.Pipe, token.Caret, token.Shl, token.Shr:
		if c.rejectTypeVar(n.L.Pos(), lt, n.Op.String()+" (bitwise)") ||
			c.rejectTypeVar(n.R.Pos(), rt, n.Op.String()+" (bitwise)") {
			return Int
		}
	}
	switch n.Op {
	case token.Plus, token.Minus, token.Star, token.Slash, token.Percent,
		token.Lt, token.Lte, token.Gt, token.Gte:
		if c.isNumericTypeVar(lt) && lt == rt {
			if n.Op == token.Percent {
				c.errf(n.OpPos, "%% is not allowed on numeric type parameter %s (modulo is undefined for float)", typeVarName(lt))
				return Invalid
			}
			if n.Op == token.Lt || n.Op == token.Lte || n.Op == token.Gt || n.Op == token.Gte {
				return Bool
			}
			return lt
		}
		if c.rejectTypeVar(n.OpPos, lt, n.Op.String()+" arithmetic/comparison") ||
			c.rejectTypeVar(n.OpPos, rt, n.Op.String()+" arithmetic/comparison") {
			return binaryResultRecover(n.Op)
		}
	case token.Eq, token.Neq:
		if (c.isComparableTypeVar(lt) || c.isNumericTypeVar(lt)) && lt == rt {
			return Bool
		}
		if c.rejectTypeVar(n.OpPos, lt, n.Op.String()+" equality") ||
			c.rejectTypeVar(n.OpPos, rt, n.Op.String()+" equality") {
			return Bool
		}
	}
	switch n.Op {
	case token.Plus:
		// rule 7 (+ float): int+int=int, float+float=float, string+string=string,
		// mixed=error. int/float mixing requires an explicit conversion.
		if lt == Int && rt == Int {
			return Int
		}
		if lt == Float && rt == Float {
			return Float
		}
		if lt == String && rt == String {
			return String
		}
		c.errf(n.OpPos, "+ requires int+int, float+float, or string+string, got %s and %s", lt, rt)
		return Invalid
	case token.Minus, token.Star, token.Slash:
		// int/int and float/float are both allowed; mixing is an error.
		if lt == Int && rt == Int {
			if n.Op == token.Slash {
				c.rejectConstDivByZero(n.R)
			}
			return Int
		}
		if lt == Float && rt == Float {
			return Float
		}
		c.errf(n.OpPos, "%s requires int+int or float+float, got %s and %s", n.Op, lt, rt)
		return Invalid
	case token.Percent:
		// modulo is int-only; it is undefined for float (compile error).
		if lt == Int && rt == Int {
			c.rejectConstDivByZero(n.R)
			return Int
		}
		c.errf(n.OpPos, "%% requires int operands (modulo is undefined for float), got %s and %s", lt, rt)
		return Invalid
	case token.Amp, token.Pipe, token.Caret, token.Shl, token.Shr:
		// bitwise ops are int-only; float/string/bool are errors.
		for _, side := range []struct {
			t Type
			e ast.Expr
		}{{lt, n.L}, {rt, n.R}} {
			if side.t != Int {
				if side.t == Bool && (n.Op == token.Amp || n.Op == token.Pipe) {
					logical := "&&"
					if n.Op == token.Pipe {
						logical = "||"
					}
					c.errf(side.e.Pos(), "bitwise %s requires int operands, got bool; did you mean %s (logical)?", n.Op.String(), logical)
				} else {
					c.errf(side.e.Pos(), "bitwise %s requires int operands, got %s", n.Op.String(), side.t)
				}
			}
		}
		return Int
	case token.Lt, token.Lte, token.Gt, token.Gte:
		if lt == Int && rt == Int {
			return Bool
		}
		if lt == Float && rt == Float {
			return Bool
		}
		c.errf(n.OpPos, "%s requires int+int or float+float operands, got %s and %s", n.Op, lt, rt)
		return Bool
	case token.Eq, token.Neq:
		// == != defined for matching int/float/bool/string (no implicit
		// coercion). Handle types (array/struct) are opaque: no comparison (spec
		// 4.1 handle soundness). Function references are likewise opaque (spec 2.4):
		// isHandle does not cover fn types, so guard them explicitly.
		if isFuncref(lt) || isFuncref(rt) {
			c.errf(n.OpPos, "%s is not defined for function references (they are opaque and cannot be compared)", n.Op)
			return Bool
		}
		if isOptional(lt) || isOptional(rt) {
			if lt == rt && comparableOptional(lt) {
				return Bool
			}
			c.errf(n.OpPos, "%s is not defined for Optional values; use is_some/is_none and unwrap", n.Op)
			return Bool
		}
		if c.isHandle(lt) || c.isHandle(rt) {
			c.errf(n.OpPos, "%s is not defined for %s (aggregate handles are opaque and cannot be compared)", n.Op, handleNoun(lt, rt))
			return Bool
		}
		if lt != rt {
			c.errf(n.OpPos, "%s requires operands of the same type, got %s and %s", n.Op, lt, rt)
			return Bool
		}
		if lt == Void {
			c.errf(n.OpPos, "%s cannot compare void", n.Op)
		}
		return Bool
	case token.AndAnd, token.OrOr:
		if lt != Bool {
			c.errf(n.L.Pos(), "%s requires bool operands, got %s", n.Op, lt)
		}
		if rt != Bool {
			c.errf(n.R.Pos(), "%s requires bool operands, got %s", n.Op, rt)
		}
		return Bool
	default:
		c.errf(n.Pos(), "internal: unhandled binary operator %s (checker drift)", n.Op)
		return Invalid
	}
}

// checkStructLit type-checks a struct construction `Name { f: v, ... }`: the
// named struct must exist, every field must be set exactly once with the right
// type, and no unknown field may appear. It returns the struct type.
func (c *checker) checkStructLit(n *ast.StructLit, want Type) Type {
	// Resolve the (possibly qualified) struct name to its internal token, then look
	// it up in the global table.
	var tok Type
	if n.Namespace != "" {
		tok = c.resolveNamedType(n.Namespace+"."+n.Name, n.NamePos)
	} else if _, ok := c.cur.structs[n.Name]; ok {
		tok = internalStructName(n.Name, c.cur.id)
	} else {
		tok = Invalid
	}
	si, ok := c.info.Structs[string(tok)]
	if tok == Invalid || !ok {
		if n.Namespace == "" {
			// Only emit the generic message when resolveNamedType did not already
			// report a qualified-resolution error.
			c.errf(n.NamePos, "unknown struct type %q%s", n.Name, suggestSuffix(n.Name, c.structNames()))
		} else if tok != Invalid {
			// resolveNamedType resolved a valid but non-struct type (e.g. the core
			// type json.Value); it emitted no error, so name it here.
			c.errf(n.NamePos, "%s is not a struct type", disp(tok))
		}
		for _, f := range n.Fields {
			c.checkExpr(f.Value)
		}
		return Invalid
	}

	// Generic struct: determine the concrete instantiation before checking fields.
	if len(si.TypeParams) > 0 {
		var concreteTok Type
		// Prefer the contextual want type when it is a registered instantiation of
		// this base struct (the common case: an explicit let annotation).
		if want != Invalid {
			if _, wok := c.info.Structs[string(want)]; wok && isInstantiationOf(want, tok) {
				concreteTok = want
			}
		}
		if concreteTok == "" || concreteTok == Invalid {
			// Infer type arguments from field values via unification.
			subst := map[string]Type{}
			for _, f := range n.Fields {
				baseFT, known := si.fieldType(f.Name)
				if !known {
					continue
				}
				vt := c.checkExpr(f.Value)
				if vt != Invalid {
					var cf conflict
					c.unify(baseFT, vt, subst, &cf)
				}
			}
			for _, tp := range si.TypeParams {
				if _, ok := subst[tp]; !ok {
					c.errf(n.NamePos, "cannot infer type arguments for generic struct %q; add a type annotation", n.Name)
					return Invalid
				}
			}
			concreteTok = c.registerConcreteStructInst(si, n.Name, subst)
		}
		concreteSI := c.info.Structs[string(concreteTok)]
		set := map[string]bool{}
		for _, f := range n.Fields {
			ft, known := concreteSI.fieldType(f.Name)
			if !known {
				c.errf(f.NamePos, "struct %q has no field %q", n.Name, f.Name)
				c.checkExpr(f.Value)
				continue
			}
			if set[f.Name] {
				c.errf(f.NamePos, "field %q is set more than once", f.Name)
			}
			set[f.Name] = true
			got := c.checkExprExpecting(f.Value, ft)
			if got != Invalid && ft != Invalid && got != ft {
				c.errf(f.Value.Pos(), "field %q has type %s, want %s", f.Name, got, ft)
			}
		}
		for _, f := range concreteSI.Fields {
			if !set[f.Name] {
				c.errf(n.NamePos, "struct %q is missing field %q", n.Name, f.Name)
			}
		}
		return concreteTok
	}

	// Non-generic struct: original path unchanged.
	set := map[string]bool{}
	for _, f := range n.Fields {
		ft, known := si.fieldType(f.Name)
		if !known {
			c.errf(f.NamePos, "struct %q has no field %q", n.Name, f.Name)
			c.checkExpr(f.Value)
			continue
		}
		if set[f.Name] {
			c.errf(f.NamePos, "field %q is set more than once", f.Name)
		}
		set[f.Name] = true
		got := c.checkExprExpecting(f.Value, ft)
		if got != Invalid && ft != Invalid && got != ft {
			c.errf(f.Value.Pos(), "field %q has type %s, want %s", f.Name, got, ft)
		}
	}
	// completeness: every declared field must be provided.
	for _, f := range si.Fields {
		if !set[f.Name] {
			c.errf(n.NamePos, "struct %q is missing field %q", n.Name, f.Name)
		}
	}
	return tok
}

// checkTupleLit type-checks a tuple literal (e1, e2, ..., en), n >= 2.
// Each element is checked bottom-up (no expected-type threading -- a tuple
// literal infers its type from its elements, like an explicit-element array).
// Void-typed elements are errors. info.Types is recorded by checkExprExpecting.
func (c *checker) checkTupleLit(n *ast.TupleLit) Type {
	// Defensive arity guard (belt-and-suspenders): the parser already rejects a
	// one-element literal, but reject < 2 here too with the same message.
	if len(n.Elems) < 2 {
		c.errf(n.Pos(), "tuple literal requires at least two elements")
		return Invalid
	}
	elems := make([]Type, 0, len(n.Elems))
	for _, e := range n.Elems {
		et := c.checkExpr(e)
		// Invalid-before-Void, matching the resolveType ordering (spec 5.1).
		if et == Invalid {
			return Invalid
		}
		if et == Void {
			c.errf(e.Pos(), "tuple element has no value type (void)")
			return Invalid
		}
		elems = append(elems, et)
	}
	return tupleType(elems)
}

// checkArrayLit type-checks an array literal `[a, b, c]`. Every element must
// share one type; that type becomes the array's element type. An empty literal
// `[]` takes its element type from the contextual want (the surrounding
// annotation); without one it is an error.
func (c *checker) checkArrayLit(n *ast.ArrayLit, want Type) Type {
	if len(n.Elems) == 0 {
		if isArray(want) {
			return want
		}
		c.errf(n.LBrkPos, "cannot infer the element type of an empty array literal; annotate it (e.g. let xs: int[] = [])")
		return Invalid
	}
	var wantElem Type = Invalid
	if isArray(want) {
		wantElem = elemType(want)
	}
	elem := c.checkExprExpecting(n.Elems[0], wantElem)
	for i := 1; i < len(n.Elems); i++ {
		et := c.checkExprExpecting(n.Elems[i], wantElem)
		if et != Invalid && elem != Invalid && et != elem {
			c.errf(n.Elems[i].Pos(), "array element %d has type %s, but earlier elements are %s", i+1, et, elem)
		}
	}
	if elem == Invalid {
		return Invalid
	}
	return arrayType(elem)
}

// checkDictLit type-checks a dict literal `{ k: v, ... }`. Every key must share
// one type (int or string); every value must share one type; those become the
// dict's key/value types. A duplicate key that is STATICALLY determinable (a
// constant int/string key literal) is a compile error (spec 4.4). An empty
// literal `{}` takes its type from the contextual want.
func (c *checker) checkDictLit(n *ast.DictLit, want Type) Type {
	if len(n.Entries) == 0 {
		if isDict(want) {
			return want
		}
		c.errf(n.LBrace, "cannot infer the type of an empty dict literal; annotate it (e.g. let m: {string: int} = {})")
		return Invalid
	}
	var wantKey, wantVal Type = Invalid, Invalid
	if isDict(want) {
		wantKey = dictKeyType(want)
		wantVal = dictValType(want)
	}

	keyT := c.checkExprExpecting(n.Entries[0].Key, wantKey)
	valT := c.checkExprExpecting(n.Entries[0].Value, wantVal)
	// The key type must be a comparable scalar (int/string/bool/float) or a value
	// enum (keyed by its backing scalar).
	if keyT != Invalid && keyT != Int && keyT != String && keyT != Bool && keyT != Float && !c.isValueEnum(keyT) {
		c.errf(n.Entries[0].Key.Pos(), "dict key must be int, string, bool, float, or a value enum, got %s", keyT)
		keyT = Invalid
	}

	seen := map[string]bool{} // static const keys, for duplicate detection
	if lit, ok := c.dictKeyDedupToken(n.Entries[0].Key); ok {
		seen[lit] = true
	}

	for i := 1; i < len(n.Entries); i++ {
		kt := c.checkExprExpecting(n.Entries[i].Key, wantKey)
		if kt != Invalid && kt != Int && kt != String && kt != Bool && kt != Float && !c.isValueEnum(kt) {
			c.errf(n.Entries[i].Key.Pos(), "dict key must be int, string, bool, float, or a value enum, got %s", kt)
		} else if kt != Invalid && keyT != Invalid && kt != keyT {
			c.errf(n.Entries[i].Key.Pos(), "dict key %d has type %s, but earlier keys are %s", i+1, kt, keyT)
		}
		vt := c.checkExprExpecting(n.Entries[i].Value, wantVal)
		if vt != Invalid && valT != Invalid && vt != valT {
			c.errf(n.Entries[i].Value.Pos(), "dict value %d has type %s, but earlier values are %s", i+1, vt, valT)
		}
		if lit, ok := c.dictKeyDedupToken(n.Entries[i].Key); ok {
			if seen[lit] {
				c.errf(n.Entries[i].Key.Pos(), "duplicate key %s in dict literal", lit)
			}
			seen[lit] = true
		}
	}
	if keyT == Invalid || valT == Invalid {
		return Invalid
	}
	if valT == Void {
		c.errf(n.LBrace, "dict value type cannot be void")
		return Invalid
	}
	return dictType(keyT, valT)
}

// dictKeyDedupToken returns a canonical compile-time dedup token for a
// statically-constant dict key, and true; ("", false) for a non-constant key
// (so duplicate detection only fires when equality is provable at compile time).
// It reads the checker's folded value, so it covers int/string/bool/float
// literals and value-enum variant keys (which fold to their backing const).
// A dict literal is homogeneous -- every key shares one type, enforced by the
// key-type gate -- so no cross-kind token clash is possible and the raw value
// string is a sufficient token. Deliberately UNPREFIXED: the token is used
// verbatim in the "duplicate key %s in dict literal" diagnostic, and a type tag
// like "f:" would leak into that user-facing text as junk (e.g. "f:1").
// Float keys are keyed by the parsed float64 with -0.0 folded to +0.0, so
// 1.0/1.00 and 0.0/-0.0 collide. This token is Go shortest-form and does NOT
// byte-equal the runtime %.17g key text; what coincides -- and all FR-022
// requires -- is the equivalence RELATION (equal float64 <=> equal runtime
// %.17g key), so a compile-time duplicate is exactly a runtime collision.
func (c *checker) dictKeyDedupToken(key ast.Expr) (string, bool) {
	// Value-enum variant keys and const references fold during checkExpr and land
	// in FoldedValues, so consult it first.
	if fv, ok := c.info.FoldedValues[key]; ok {
		return dedupTokenFromFolded(fv)
	}
	// Plain literals are NOT folded into FoldedValues by ordinary checkExpr, so
	// read them straight off the AST (mirroring the old staticKey, extended to
	// bool/float). A negated int/float literal is a UnaryExpr over the magnitude.
	switch n := key.(type) {
	case *ast.IntLit:
		if v, err := parseWispInt(n.Raw, false); err == nil {
			return strconv.FormatInt(v, 10), true
		}
	case *ast.FloatLit:
		return floatDedupToken(n.Raw)
	case *ast.BoolLit:
		return dedupTokenFromFolded(n.Value)
	case *ast.StringLit:
		s := ""
		for _, p := range n.Parts {
			if !p.IsText() {
				return "", false // interpolated; not statically known
			}
			s += p.Text
		}
		return s, true
	case *ast.UnaryExpr:
		if n.Op == token.Minus {
			switch x := n.X.(type) {
			case *ast.IntLit:
				if v, err := parseWispInt(x.Raw, true); err == nil {
					return strconv.FormatInt(v, 10), true
				}
			case *ast.FloatLit:
				return floatDedupToken("-" + x.Raw)
			}
		}
	}
	return "", false
}

// dedupTokenFromFolded maps a folded constant value to its canonical dedup
// token, matching dictKeyDedupToken's per-kind rules.
func dedupTokenFromFolded(fv interface{}) (string, bool) {
	switch v := fv.(type) {
	case int64:
		return strconv.FormatInt(v, 10), true
	case string:
		return v, true
	case bool:
		if v {
			return "true", true
		}
		return "false", true
	case FoldedFloat:
		return floatDedupToken(v.Raw)
	}
	return "", false
}

// floatDedupToken canonicalizes a float literal's raw text to the parsed float64
// shortest form with -0.0 folded to +0.0, so 1.0/1.00 and 0.0/-0.0 collide --
// the same equivalence relation as the runtime %.17g key (FR-013/FR-022).
func floatDedupToken(raw string) (string, bool) {
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return "", false
	}
	if f == 0 {
		f = 0 // fold -0.0 to +0.0
	}
	return strconv.FormatFloat(f, 'g', -1, 64), true
}

// resolveQualifiedConst applies the R6 decision rule to a `ns.NAME` FieldAccess
// against the module bound to the namespace (target modid), using TWO pieces of
// target metadata: its own top-level const set (tctx.constTable, own declarations
// only) and its exported-const set (tctx.exported). It is the single resolution
// path shared by the function-body value position (checkFieldAccess) and the
// default-argument context (foldConst).
//
// PRECONDITION: the caller has already confirmed via qualifiedNsTarget that
// the base IS an in-scope namespace alias not shadowed by a local; so inside this
// helper the base is always a namespace (never a struct value).
//
// The returned bool, `handled`, means "this namespace-qualified reference has
// been fully dealt with -- resolved to a const value OR an appropriate diagnostic
// was emitted." Because the base is always a namespace, EVERY branch returns
// handled == true; the caller must therefore never fall through to struct-field
// handling for a namespace-qualified base, even when the returned Type is Invalid.
//
//   - NAME in tctx.constTable AND tctx.exported -> resolve: record info.Types[n]
//     and info.FoldedValues[n] (the carrier, R4/R5); return (entry.Type, entry.Value, true).
//   - NAME in tctx.constTable but NOT exported -> "constant %q is not exported
//     from %q"; return (Invalid, nil, true). (AC4)
//   - NAME is an exported fn or struct of the target -> wrong-kind diagnostic;
//     return (Invalid, nil, true). It is NOT a const and does NOT fall through. (AC16)
//   - NAME absent from tctx.constTable and not a fn/struct -> "namespace %q has no
//     exported constant %q"; return (Invalid, nil, true). An imported const is not
//     in the target's OWN constTable, so a re-export attempt lands here. (AC5/AC7)
func (c *checker) resolveQualifiedConst(n *ast.FieldAccess, field string, modid int, want Type) (Type, interface{}, bool) {
	nsName := n.X.(*ast.Ident).Name
	tctx := c.modCtx[modid]
	// A reserved core module resolves its members through the core catalog. A
	// coreConst is a value; a coreFunc/coreType used in value position gets a
	// wrong-kind diagnostic (never a silent struct-field fallthrough).
	if tctx.core != "" {
		if m, ok := coreCatalog[tctx.core][field]; ok {
			switch m.kind {
			case coreConst:
				c.info.Types[n] = m.constVal.Type
				c.info.FoldedValues[n] = m.constVal.Value
				return m.constVal.Type, m.constVal.Value, true
			case coreFunc:
				// Part 3: a core-module function member is referenceable as a
				// funcref VALUE iff its underlying flat builtin is in the
				// generatable allowlist AND it does not take type arguments. It
				// mints the SAME __wisp_builtin_<name> wrapper and funcref type as
				// the bare-ident path, so ns.member and bare member share one
				// wrapper. A type-argument member (json.decode) can never be a
				// value; the error is gated on the flag, not on an empty builtin
				// string. Any other coreFunc (overloaded/composite/handle/nullary
				// with no uniform helper) keeps the wrong-kind diagnostic.
				if m.takesTypeArgs {
					c.errf(n.DotPos, "%s.%s takes type arguments and cannot be used as a function value", nsName, field)
					return Invalid, nil, true
				}
				if builtinFuncrefGeneratable[m.builtin] {
					ft := builtinFuncrefType(m.builtin)
					c.info.Types[n] = ft
					c.info.MemberFuncRefs[n] = &FuncRef{Mangled: builtinFuncrefMangled(m.builtin), Type: ft}
					return ft, nil, true
				}
				// Overloaded builtin delegate (abs/min/max/clamp/sign): the arm is
				// selected the SAME way as the bare-ident path, from the expected
				// funcref type; see resolveOverloadedFuncref.
				if _, ok := overloadedFuncrefArms[m.builtin]; ok {
					if ft, mangled, ok := resolveOverloadedFuncref(m.builtin, want); ok {
						c.info.Types[n] = ft
						c.info.MemberFuncRefs[n] = &FuncRef{Mangled: mangled, Type: ft}
						return ft, nil, true
					} else if want != Invalid && isFuncref(want) && funcRetType(want) != Invalid {
						c.errf(n.DotPos, "%q has no function-reference form matching %s; supported: %s",
							field, disp(want), joinedOverloadedArms(m.builtin))
						return Invalid, nil, true
					} else {
						c.errf(n.DotPos, "reference to overloaded builtin %q needs a function-reference type annotation to select an overload", field)
						return Invalid, nil, true
					}
				}
				// Generic higher-order builtin delegate (array.map/filter/each/
				// reduce/sort_by/find/any/all/count_where): the axis is selected the
				// SAME way as the bare-ident path, from the expected funcref type;
				// see resolveGenericFuncref.
				if _, ok := genericFuncrefAxes[m.builtin]; ok {
					if ft, mangled, ok := resolveGenericFuncref(m.builtin, want); ok {
						c.info.Types[n] = ft
						c.info.MemberFuncRefs[n] = &FuncRef{Mangled: mangled, Type: ft}
						return ft, nil, true
					} else if want != Invalid && isFuncref(want) && funcRetType(want) != Invalid {
						c.errf(n.DotPos, "%q has no function-reference form matching %s; supported containers: %s",
							field, disp(want), joinedGenericAxisNames(m.builtin))
						return Invalid, nil, true
					} else {
						c.errf(n.DotPos, "reference to generic builtin %q needs a function-reference type annotation to select a container overload", field)
						return Invalid, nil, true
					}
				}
				reason := funcrefRejectReason(m.builtin)
				c.errf(n.DotPos, "%q of module %q cannot be referenced as a function value (%s); wrap it in a fn", field, nsName, reason)
				return Invalid, nil, true
			case coreType:
				c.errf(n.DotPos, "%q is a type of module %q, not a value", field, nsName)
				return Invalid, nil, true
			}
		}
		c.errf(n.DotPos, "module %q has no member %q", nsName, field)
		return Invalid, nil, true
	}
	if entry, inOwn := tctx.constTable[field]; inOwn {
		if !tctx.exported[field] {
			c.errf(n.DotPos, "constant %q is not exported from %q", field, nsName)
			return Invalid, nil, true
		}
		// Resolve: record the type and the producing module's folded value on the
		// FieldAccess node (the carrier, R4). FoldedValues is ast.Expr-keyed, so a
		// FieldAccess is a valid key; info.Uses is *ast.Ident-keyed and is NOT used.
		c.info.Types[n] = entry.Type
		c.info.FoldedValues[n] = entry.Value
		return entry.Type, entry.Value, true
	}
	// A qualified reference to an exported, non-generic user function in value
	// position (never called) mints a FuncRef, mirroring the bare-same-module
	// path (checkExprIdent) and the qualified-CALL path (checkQualifiedCall).
	// AC16 still holds for the const-initializer / default-argument context
	// (foldAllowsQualified): a function is never a foldable constant, regardless
	// of exportedness or genericity.
	if fn, isFn := tctx.funcs[field]; isFn {
		if !tctx.exported[field] {
			c.errf(n.DotPos, "%q is not exported by %q", field, nsName)
			return Invalid, nil, true
		}
		if c.foldAllowsQualified {
			// Checked BEFORE the generic-function check: a generic function in a
			// default-argument position must get this const-context message, not
			// the funcref-vocabulary "generic function ... cannot be used as a
			// function reference" message below, since the latter implies a
			// funcref was otherwise a candidate at all, which is never true here
			// regardless of genericity.
			c.errf(n.DotPos, "%q is a function, not a constant expression", field)
			return Invalid, nil, true
		}
		if len(fn.TypeParams) > 0 {
			c.errf(n.DotPos, "generic function %q cannot be used as a function reference; call it directly", field)
			return Invalid, nil, true
		}
		// Resolve fn's signature types in ITS OWN module's context (matching
		// checkUserCallIn's identical swap for the call-position case), not the
		// caller's -- otherwise a struct name in fn's own signature resolves
		// against the wrong module's struct table. Safe to leave c.typeParams
		// unswapped: the TypeParams check above already guarantees fn is
		// non-generic, so its annotations can never contain a type variable.
		caller := c.cur
		c.cur = tctx
		ft := c.funcRefType(fn)
		c.cur = caller
		c.info.Types[n] = ft
		c.info.MemberFuncRefs[n] = &FuncRef{Mangled: mangleFunc(modid, fn.Name), Type: ft}
		return ft, nil, true
	}
	if _, isStruct := tctx.structs[field]; isStruct {
		c.errf(n.DotPos, "%q is a struct of %q, not a constant", field, nsName)
		return Invalid, nil, true
	}
	// Absent from the target's OWN source. A re-export attempt lands here (an
	// imported const is not in the target's own constTable, R2/AC7), never a silent
	// resolution to the original module's value.
	c.errf(n.DotPos, "namespace %q has no exported constant %q", nsName, field)
	return Invalid, nil, true
}

// checkQualifiedEnumVariantAccess handles `ns.Enum.Variant`: a FieldAccess
// whose base X is ITSELF a FieldAccess matching the `ns.NAME` shape
// (qualifiedNsTarget), where NAME names an enum declared (and exported) in
// the target module. It mirrors checkEnumVariantAccess's fold, but resolves
// the enum in the TARGET module's tables rather than c.cur's. Returns
// (_, false) when n.X is not a namespace-qualified enum reference at all (so
// the caller falls through to the existing const/funcref/struct path, which
// also handles the re-export misuse case: an enum only reaches tctx.enums
// when THAT module declared it, so a merely-imported enum is invisible here
// exactly as for consts, R2/AC7 mirrored).
func (c *checker) checkQualifiedEnumVariantAccess(n *ast.FieldAccess) (Type, bool) {
	inner, isFA := n.X.(*ast.FieldAccess)
	if !isFA {
		return Invalid, false
	}
	enumName, modid, ok := c.qualifiedNsTarget(inner)
	if !ok {
		return Invalid, false
	}
	tctx := c.modCtx[modid]
	ei, isEnum := tctx.enums[enumName]
	if !isEnum {
		return Invalid, false
	}
	if ei.Kind == EnumTagged {
		// A bare `Expr.Unit` / `Expr.IntLit` reference is construction-or-error;
		// handled by checkFieldAccess (Task 7). Signal "not a value-enum fold".
		return Invalid, false
	}
	nsName := inner.X.(*ast.Ident).Name
	if !tctx.exportedEnums[enumName] {
		// Gate on the dedicated exportedEnums set (Task 2), NOT the name-shared
		// `exported` map, so a same-named exported fn/const cannot make a
		// non-exported enum visible here (FR-002/FR-006). Preposition "by"
		// matches the existing struct "not exported by %q" message and Task 2's
		// enum-type branch, so all three "not exported" sites read identically.
		c.errf(inner.DotPos, "enum %q is not exported by %q", enumName, nsName)
		c.info.Types[n] = Invalid
		return Invalid, true
	}
	cv, found := ei.constValue(n.Field)
	if !found {
		c.errf(n.DotPos, "enum %q has no variant %q", enumName, n.Field)
		c.info.Types[n] = Invalid
		return Invalid, true
	}
	tok := internalEnumName(enumName, modid)
	c.info.Types[n] = tok
	c.info.FoldedValues[n] = cv
	return tok, true
}

// checkFieldAccess type-checks `x.field`: x must be a struct value and field
// must exist; the result is the field's type.
func (c *checker) checkFieldAccess(n *ast.FieldAccess, want Type) Type {
	// A qualified cross-module const reference `ns.NAME` in value position (R3): the
	// base is an in-scope namespace alias not shadowed by a local. Recognized here,
	// before checkExpr(n.X) would reject the namespace alias as a non-value. When
	// qualifiedNsTarget reports a namespace-qualified base, resolveQualifiedConst
	// fully handles it (handled is always true for a namespace base -- resolve or
	// emit the right diagnostic), so we RETURN its type unconditionally and never
	// fall through to the struct-field path for such a base (AC4/AC5/AC7/AC16).
	if field, modid, ok := c.qualifiedNsTarget(n); ok {
		t, _, _ := c.resolveQualifiedConst(n, field, modid, want)
		return t
	}
	// Enum variant access `Color.Red` (R3): X names an enum type iff its base is a
	// bare identifier that is an enum name AND is NOT shadowed by an in-scope local
	// AND is NOT a module namespace alias (locals/namespaces keep higher precedence,
	// matching the qualifiedNsTarget order above). `.Field` must be a declared
	// variant; the access types to the enum and FOLDS to the variant's int value
	// (recorded in info.FoldedValues -- the codegen precondition for inlining).
	if t, ok := c.checkEnumVariantAccess(n); ok {
		return t
	}
	if t, ok := c.checkQualifiedEnumVariantAccess(n); ok {
		return t
	}
	// A bare (no-call) reference to a tagged-union variant: `Expr.Unit` constructs
	// a no-payload value; `Expr.IntLit` (payload variant, no call) is an error --
	// the SC-020 "function reference" diagnostic when `want` is a funcref, else the
	// SC-030 "has a payload" diagnostic. `want` is threaded in so the helper can
	// distinguish the two contexts.
	if t, ok := c.checkBareTaggedVariant(n, want); ok {
		return t
	}
	xt := c.checkExpr(n.X)
	if xt == Invalid {
		return Invalid
	}
	if c.rejectTypeVar(n.DotPos, xt, "field access") {
		return Invalid
	}
	if isErrorType(xt) {
		switch n.Field {
		case "message":
			return String
		case "code":
			return Int
		default:
			c.errf(n.DotPos, "error has no field %q (only message, code)", n.Field)
			return Invalid
		}
	}
	if isRunResultType(xt) {
		switch n.Field {
		case "stdout", "stderr":
			return String
		case "code":
			return Int
		default:
			c.errf(n.DotPos, "RunResult has no field %q (only stdout, stderr, code)", n.Field)
			return Invalid
		}
	}
	if isProcessType(xt) {
		switch n.Field {
		case "pid":
			return Int
		default:
			c.errf(n.DotPos, "Process has no field %q (only pid)", n.Field)
			return Invalid
		}
	}
	if !c.isStructType(xt) {
		c.errf(n.DotPos, "cannot access field %q of non-struct type %s", n.Field, xt)
		return Invalid
	}
	si := c.info.Structs[string(xt)]
	ft, ok := si.fieldType(n.Field)
	if !ok {
		c.errf(n.DotPos, "struct %q has no field %q", xt, n.Field)
		return Invalid
	}
	return ft
}

// checkIndexExpr type-checks `x[i]`: x must be an array, dict, or tuple.
// For tuples the index MUST be a constant *ast.IntLit; a non-literal or
// out-of-range index is a compile error.
func (c *checker) checkIndexExpr(n *ast.IndexExpr) Type {
	xt := c.checkExpr(n.X)
	it := c.checkExpr(n.Index)
	if xt == Invalid {
		return Invalid
	}
	if isDict(xt) {
		kt := dictKeyType(xt)
		if it != Invalid && it != kt {
			c.errf(n.Index.Pos(), "dict key must be %s, got %s", kt, it)
		}
		return dictValType(xt)
	}
	// Tuple: index must be a constant *ast.IntLit; bounds-checked at compile time.
	if isTuple(xt) {
		lit, ok := n.Index.(*ast.IntLit)
		if !ok {
			c.errf(n.Index.Pos(), "tuple index must be a constant integer literal")
			return Invalid
		}
		elems := tupleElemTypes(xt)
		// strconv.Atoi errors on overflow (the lexer accepts arbitrarily long
		// integer literals), so an out-of-int index never wraps negative and
		// panics elems[k]; treat any unparseable/negative/too-large index as out
		// of range. The < 0 guard is defensive (an IntLit.Raw is unsigned digits).
		k, err := strconv.Atoi(lit.Raw)
		if err != nil || k < 0 || k >= len(elems) {
			c.errf(n.Index.Pos(), "tuple index %s out of range for %s (arity %d)", lit.Raw, xt, len(elems))
			return Invalid
		}
		return elems[k]
	}
	if !isArray(xt) {
		c.errf(n.LBrkPos, "cannot index non-array, non-dict type %s", xt)
		return Invalid
	}
	if it != Invalid && it != Int {
		c.errf(n.Index.Pos(), "array index must be int, got %s", it)
	}
	// Negative constant index is statically invalid (spec construct #11). The
	// dynamic upper bound depends on the runtime length and remains a runtime
	// abort (out of scope). No exact runtime message mirror; checker-specific.
	if v, ok := constIntProbe(n.Index); ok && v < 0 {
		c.errf(n.Index.Pos(), "array index out of bounds: negative constant index")
	}
	return elemType(xt)
}

// handleNoun renders a short noun for an aggregate-handle operand error,
// preferring whichever operand is the handle.
func handleNoun(lt, rt Type) string {
	if isArray(lt) || isArray(rt) {
		return "array values"
	}
	return "struct values"
}

// binaryResultRecover gives the result type an operator would have produced, so
// checking can continue after an operand error without cascading.
func binaryResultRecover(op token.Kind) Type {
	switch op {
	case token.Plus, token.Minus, token.Star, token.Slash, token.Percent:
		return Invalid // could be int or string; unknown
	case token.Lt, token.Lte, token.Gt, token.Gte, token.Eq, token.Neq, token.AndAnd, token.OrOr:
		return Bool
	case token.Amp, token.Pipe, token.Caret, token.Shl, token.Shr:
		return Int
	default:
		panic(fmt.Sprintf("binaryResultRecover: no case for operator %s (parser/checker drift)", op))
	}
}
