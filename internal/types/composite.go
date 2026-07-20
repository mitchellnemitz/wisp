package types

import "strings"

// Composite types (M3 PR-B) reuse the flat string Type representation so that
// structural equality is plain `==` and `%s` formatting still prints the type.
// An array type is encoded "[" + elem + "]" (e.g. "[int]", "[[int]]",
// "[Point]"); a named struct type is the struct's name (e.g. "Point"). The
// primitive Int/Float/Bool/String/Void/Invalid constants are unchanged.
//
// A "handle" type is any aggregate stored as a reference id at runtime: an array
// or a named struct. Handle types are OPAQUE (spec 4.1): no conversion to/from
// int, no arithmetic, no comparison.

// arrayType builds the array Type with element type elem.
func arrayType(elem Type) Type { return Type("[" + string(elem) + "]") }

// isArray reports whether t is an array type.
func isArray(t Type) bool {
	return strings.HasPrefix(string(t), "[") && strings.HasSuffix(string(t), "]")
}

// IsArray reports whether t is an array type. Exported for codegen, which
// dispatches length()/push() and rendering on the static argument type.
func IsArray(t Type) bool { return isArray(t) }

// ElemType returns the element type of an array type. Exported for codegen
// (sort/sum dispatch on the element type). Must only be called when IsArray(t).
func ElemType(t Type) Type { return elemType(t) }

// elemType returns the element type of an array type. It must only be called
// when isArray(t) is true.
func elemType(t Type) Type {
	return Type(string(t)[1 : len(t)-1])
}

// optionalType builds the Optional Type with element type elem.
func optionalType(elem Type) Type { return Type("Optional[" + string(elem) + "]") }

// isOptional reports whether t is an Optional type. Every Optional Type the
// checker produces is concrete (e.g. "Optional[int]"); there is no "Optional[?]"
// sentinel (None is handled structurally at its AST node, never as a type).
func isOptional(t Type) bool {
	return strings.HasPrefix(string(t), "Optional[") && strings.HasSuffix(string(t), "]")
}

// optionalElemType returns the element type of an Optional type. Must only be
// called when isOptional(t).
func optionalElemType(t Type) Type {
	return Type(string(t)[len("Optional[") : len(t)-1])
}

// IsOptional / OptionalElemType / ComparableOptional are exported wrappers for codegen.
func IsOptional(t Type) bool         { return isOptional(t) }
func OptionalElemType(t Type) Type   { return optionalElemType(t) }
func ComparableOptional(t Type) bool { return comparableOptional(t) }

// resultType builds the Result Type with success type elem. The error payload is
// always the built-in error handle, so it is not encoded (single type parameter).
func resultType(elem Type) Type { return Type("Result[" + string(elem) + "]") }

// isResult reports whether t is a Result type. Like Optional, every Result Type
// the checker produces is concrete; there is no "Result[?]" sentinel.
func isResult(t Type) bool {
	return strings.HasPrefix(string(t), "Result[") && strings.HasSuffix(string(t), "]")
}

// resultElemType returns the success type of a Result. Must only be called when
// isResult(t). Single-bracket unwrap, identical to optionalElemType.
func resultElemType(t Type) Type {
	return Type(string(t)[len("Result[") : len(t)-1])
}

// IsResult / ResultElemType are exported wrappers for codegen.
func IsResult(t Type) bool       { return isResult(t) }
func ResultElemType(t Type) Type { return resultElemType(t) }

// variantsOf returns the ordered constructor names for a sum type, or nil.
func variantsOf(t Type) []string {
	if isOptional(t) {
		return []string{"Some", "None"}
	}
	if isResult(t) {
		return []string{"Ok", "Err"}
	}
	return nil
}

// matchArmBoundType returns the payload type for a variant arm and whether
// a binding exists. Returns (Invalid, false) for payload-less variants (None).
func matchArmBoundType(scrut Type, variant string) (Type, bool) {
	if isOptional(scrut) {
		switch variant {
		case "Some":
			return optionalElemType(scrut), true
		case "None":
			return Invalid, false
		}
	}
	if isResult(scrut) {
		switch variant {
		case "Ok":
			return resultElemType(scrut), true
		case "Err":
			return ErrorType, true
		}
	}
	return Invalid, false
}

// A dict type is encoded "{" + K + ":" + V + "}" (e.g. "{string:int}",
// "{int:[int]}"). K is "int" or "string" (the only hashable key types, spec
// 4.4); V is any non-void type. The encoding is structural, so equal Type
// strings denote the same dict and the key/value split is recoverable. Because
// V may itself be a composite containing ':' (e.g. a nested dict), the split is
// at the FIRST ':' (K never contains ':').

// dictType builds the dict Type with key type k and value type v.
func dictType(k, v Type) Type { return Type("{" + string(k) + ":" + string(v) + "}") }

// isDict reports whether t is a dict type.
func isDict(t Type) bool {
	s := string(t)
	return strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") && strings.Contains(s, ":")
}

// IsDict reports whether t is a dict type. Exported for codegen, which
// dispatches has()/keys()/for-in/index on the static argument type.
func IsDict(t Type) bool { return isDict(t) }

// dictKeyType returns the key type K of a dict type. Must only be called when
// isDict(t) is true. The split is at the first ':'.
func dictKeyType(t Type) Type {
	s := string(t)[1 : len(t)-1] // strip { }
	i := strings.IndexByte(s, ':')
	return Type(s[:i])
}

// dictValType returns the value type V of a dict type. Must only be called when
// isDict(t) is true. The split is at the first ':'.
func dictValType(t Type) Type {
	s := string(t)[1 : len(t)-1]
	i := strings.IndexByte(s, ':')
	return Type(s[i+1:])
}

// DictKeyType / DictValType are exported wrappers for codegen.
func DictKeyType(t Type) Type { return dictKeyType(t) }
func DictValType(t Type) Type { return dictValType(t) }

// A function-reference type (M4) is encoded "fn(" + join(params, ",") + ")->" +
// R, mirroring the array/dict encodings (e.g. "fn(int,int)->int",
// "fn()->void", "fn([int])->bool"). The encoding is structural, so equal Type
// strings denote the same function type and compile-time type equality is plain
// ==. Function refs are OPAQUE (spec 2.4): like handles, no conversion, no
// arithmetic, no comparison -- but they are NOT covered by isHandle (which is
// array/dict/struct only), so isFuncref guards the == / != path separately.

// isFuncref reports whether t is a function-reference type.
func isFuncref(t Type) bool {
	return strings.HasPrefix(string(t), "fn(") && strings.Contains(string(t), ")->")
}

// IsFuncref reports whether t is a function-reference type. Exported for
// codegen, which dispatches indirect-call lowering on the static callee type.
func IsFuncref(t Type) bool { return isFuncref(t) }

// funcRetType returns the return type R of a function-reference type. Must only
// be called when isFuncref(t) is true. The split is at the LAST ")->" so a
// nested function-typed parameter (which itself contains ")->") does not
// confuse the split: the top-level params are balanced-paren matched, then "->"
// follows the matching ')'.
func funcRetType(t Type) Type {
	_, ret := splitFuncType(t)
	return ret
}

// funcParamTypes returns the parameter types of a function-reference type. Must
// only be called when isFuncref(t) is true.
func funcParamTypes(t Type) []Type {
	params, _ := splitFuncType(t)
	return params
}

// splitFuncType decomposes "fn(P1,P2,...)->R" into its parameter types and
// return type. The parameter list is split at top-level commas (commas nested
// inside a composite param -- a nested fn/dict/array -- are not separators), and
// the "->R" follows the ')' that matches the opening '(' after "fn". A param of
// function type contains its own "(...)->..." which the depth counter skips.
func splitFuncType(t Type) ([]Type, Type) {
	s := string(t)[2:] // drop "fn"; s begins with "("
	// Find the ')' matching s[0]=='('.
	depth := 0
	close := -1
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				close = i
			}
		}
		if close >= 0 {
			break
		}
	}
	inner := s[1:close]      // the comma-separated param list
	ret := Type(s[close+3:]) // skip ")->"
	var params []Type
	if inner != "" {
		params = splitTopLevel(inner)
	}
	return params, ret
}

// splitTopLevel splits a composite-type parameter list at commas that are not
// nested inside (), [], or {} -- so a function-typed, array, or dict parameter
// (which contains its own separators) stays intact.
func splitTopLevel(s string) []Type {
	var out []Type
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, Type(s[start:i]))
				start = i + 1
			}
		}
	}
	out = append(out, Type(s[start:]))
	return out
}

// tupleType builds the tuple Type encoding "(T1,T2,...,Tn)".
func tupleType(elems []Type) Type {
	s := "("
	for i, e := range elems {
		if i > 0 {
			s += ","
		}
		s += string(e)
	}
	return Type(s + ")")
}

// isTuple reports whether t is a tuple type. A tuple both starts with "(" and
// ends with ")"; a funcref starts with "fn(" and ends with its return type (not
// necessarily ")"), so the prefix/suffix pair is unambiguous.
func isTuple(t Type) bool {
	return strings.HasPrefix(string(t), "(") && strings.HasSuffix(string(t), ")")
}

// tupleElemTypes returns the element types of a tuple. Must only be called when
// isTuple(t). Uses splitTopLevel on the inner text so nested composites
// (including nested tuples) are not split at their own commas.
func tupleElemTypes(t Type) []Type {
	inner := string(t)[1 : len(t)-1]
	return splitTopLevel(inner)
}

// IsTuple / TupleElemTypes are exported wrappers for codegen.
func IsTuple(t Type) bool          { return isTuple(t) }
func TupleElemTypes(t Type) []Type { return tupleElemTypes(t) }

// isStructType reports whether t names a declared struct. The checker's struct
// table makes this decidable.
func (c *checker) isStructType(t Type) bool {
	_, ok := c.info.Structs[string(t)]
	return ok
}

// isErrorType reports whether t is the built-in error handle type (M5).
func isErrorType(t Type) bool { return t == ErrorType }

// isRunResultType reports whether t is the built-in RunResult handle type (R3).
func isRunResultType(t Type) bool { return t == RunResult }

// isProcessType reports whether t is the built-in Process handle type.
func isProcessType(t Type) bool { return t == Process }

// isHandle reports whether t is a reference-handle (aggregate) type: an array, a
// dict, a declared struct, the built-in error type, an Optional, a Result, or a
// tuple. Handle types are opaque per spec 4.1 (no int/arith/compare).
func (c *checker) isHandle(t Type) bool {
	if t == Invalid {
		return false
	}
	if isArray(t) || isDict(t) || isErrorType(t) || isRunResultType(t) || isProcessType(t) || isOptional(t) || isResult(t) || isTuple(t) || t == jsonValueType {
		return true
	}
	return c.isStructType(t)
}

// isPrimitive reports whether t is one of the scalar value types.
func isPrimitive(t Type) bool {
	switch t {
	case Int, Float, Bool, String:
		return true
	}
	return false
}

// fieldType returns the declared type of struct field name and whether it
// exists.
func (s *StructInfo) fieldType(name string) (Type, bool) {
	t, ok := s.byName[name]
	return t, ok
}
