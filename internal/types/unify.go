package types

import "strings"

// Type-variable encoding and the call-site unification used for generic function
// inference (spec 4.2). A type variable for a type parameter named T is the flat
// string "$T"; "$" is illegal in a source identifier and in every composite type
// encoding, so a type variable can never collide with a user-writable type.
//
// typeVarType and isTypeVar live in checker.go (introduced with the type-param
// scope); typeVarName is the inverse of typeVarType.

// typeVarName returns the parameter name of a type variable. Precondition:
// isTypeVar(t).
func typeVarName(t Type) string { return string(t)[1:] }

// conflict captures the spec-4.3 conflict info so the diagnostic can name the
// parameter and BOTH concrete types. found is true once a conflict is recorded.
type conflict struct {
	found       bool
	param       string
	prior, next Type
}

// unify matches pattern pat (which may contain type variables) against the
// concrete actual type, recording bindings in subst. Returns false on a
// structural-kind mismatch, an arity mismatch, a concrete inequality, or a
// type-variable conflict; on a conflict it ALSO fills *cf (param + both types).
// Cases are tried in the fixed spec-4.2 order. unify mutates subst as it recurses;
// the CALLER is responsible for snapshot/commit so a failed top-level argument
// leaves no stale bindings (spec "never bound from a failed branch").
func (c *checker) unify(pat, actual Type, subst map[string]Type, cf *conflict) bool {
	// 1. Invalid short-circuit (error-suppression wildcard; no cascade).
	if pat == Invalid || actual == Invalid {
		return true
	}
	// 2. pat is a type variable: bind or check conflict (BEFORE structural cases).
	if isTypeVar(pat) {
		name := typeVarName(pat)
		if bound, ok := subst[name]; ok {
			if bound == actual {
				return true
			}
			if !cf.found {
				*cf = conflict{found: true, param: name, prior: bound, next: actual}
			}
			return false
		}
		subst[name] = actual
		return true
	}
	// 3. array.
	if isArray(pat) {
		return isArray(actual) && c.unify(elemType(pat), elemType(actual), subst, cf)
	}
	// 4. dict.
	if isDict(pat) {
		if !isDict(actual) {
			return false
		}
		return c.unify(dictKeyType(pat), dictKeyType(actual), subst, cf) &&
			c.unify(dictValType(pat), dictValType(actual), subst, cf)
	}
	// 5. Optional.
	if isOptional(pat) {
		return isOptional(actual) && c.unify(optionalElemType(pat), optionalElemType(actual), subst, cf)
	}
	// 6. Result.
	if isResult(pat) {
		return isResult(actual) && c.unify(resultElemType(pat), resultElemType(actual), subst, cf)
	}
	// 7. tuple.
	if isTuple(pat) {
		if !isTuple(actual) {
			return false
		}
		pe, ae := tupleElemTypes(pat), tupleElemTypes(actual)
		if len(pe) != len(ae) {
			return false
		}
		for i := range pe {
			if !c.unify(pe[i], ae[i], subst, cf) {
				return false
			}
		}
		return true
	}
	// 8. funcref: same arity precondition, then params + return.
	if isFuncref(pat) {
		if !isFuncref(actual) {
			return false
		}
		pp, ap := funcParamTypes(pat), funcParamTypes(actual)
		if len(pp) != len(ap) {
			return false
		}
		for i := range pp {
			if !c.unify(pp[i], ap[i], subst, cf) {
				return false
			}
		}
		return c.unify(funcRetType(pat), funcRetType(actual), subst, cf)
	}
	// 9. generic-struct instantiation.
	if pb, pm, pargs, ok := genericInstParts(pat); ok {
		ab, am, aargs, ok2 := genericInstParts(actual)
		if !ok2 || pb != ab || pm != am {
			return false
		}
		pe, ae := splitTopLevel(pargs), splitTopLevel(aargs)
		if len(pe) != len(ae) {
			return false
		}
		for i := range pe {
			if !c.unify(pe[i], ae[i], subst, cf) {
				return false
			}
		}
		return true
	}
	// 10. both concrete.
	return pat == actual
}

// applySubst replaces every type variable in pat with its binding from subst; a
// type variable with NO binding becomes Invalid (an unbound param never survives
// as "$T"). It recurses through array/dict/Optional/funcref structure.
func (c *checker) applySubst(pat Type, subst map[string]Type) Type {
	if isTypeVar(pat) {
		if b, ok := subst[typeVarName(pat)]; ok {
			return b
		}
		return Invalid
	}
	if isArray(pat) {
		return arrayType(c.applySubst(elemType(pat), subst))
	}
	if isOptional(pat) {
		return optionalType(c.applySubst(optionalElemType(pat), subst))
	}
	if isDict(pat) {
		return dictType(c.applySubst(dictKeyType(pat), subst), c.applySubst(dictValType(pat), subst))
	}
	if isResult(pat) {
		return resultType(c.applySubst(resultElemType(pat), subst))
	}
	if isTuple(pat) {
		elems := tupleElemTypes(pat)
		out := make([]Type, len(elems))
		for i, e := range elems {
			out[i] = c.applySubst(e, subst)
		}
		return tupleType(out)
	}
	if isFuncref(pat) {
		ps := funcParamTypes(pat)
		out := make([]Type, len(ps))
		for i := range ps {
			out[i] = c.applySubst(ps[i], subst)
		}
		return funcType(out, c.applySubst(funcRetType(pat), subst))
	}
	if base, mod, argsText, ok := genericInstParts(pat); ok {
		elems := splitTopLevel(argsText)
		out := make([]string, len(elems))
		for i, e := range elems {
			out[i] = string(c.applySubst(e, subst))
		}
		return Type(base + "[" + strings.Join(out, ",") + "]" + mod)
	}
	return pat
}

// typeVarsIn collects the names from params P that appear as type variables
// anywhere in the flat-string type t, reusing the same decomposers as unify. Used
// for per-parameter cannot-infer suppression: a param left unbound SOLELY by an
// already-errored argument is suppressed.
func typeVarsIn(t Type, params []string) map[string]bool {
	out := map[string]bool{}
	inSet := map[string]bool{}
	for _, p := range params {
		inSet[p] = true
	}
	var walk func(Type)
	walk = func(x Type) {
		if isTypeVar(x) {
			if n := typeVarName(x); inSet[n] {
				out[n] = true
			}
			return
		}
		switch {
		case isArray(x):
			walk(elemType(x))
		case isOptional(x):
			walk(optionalElemType(x))
		case isDict(x):
			walk(dictKeyType(x))
			walk(dictValType(x))
		case isResult(x):
			walk(resultElemType(x))
		case isTuple(x):
			for _, e := range tupleElemTypes(x) {
				walk(e)
			}
		case isFuncref(x):
			for _, p := range funcParamTypes(x) {
				walk(p)
			}
			walk(funcRetType(x))
		default:
			if _, _, argsText, ok := genericInstParts(x); ok {
				for _, e := range splitTopLevel(argsText) {
					walk(e)
				}
			}
		}
	}
	walk(t)
	return out
}

// cloneSubst returns a shallow copy of subst for the all-or-nothing per-argument
// unify commit. subst values are plain Type strings, so a shallow copy suffices.
func cloneSubst(m map[string]Type) map[string]Type {
	out := make(map[string]Type, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
