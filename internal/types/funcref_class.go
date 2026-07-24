package types

import "strings"

// Funcref classification: the single source of truth for which builtins are
// referenceable as first-class fn-values (eta-expansion) and, for those that are
// not, why. It is queried by the checker (checkIdent + the Part 3 member-funcref
// resolver) AND by the exhaustive surface test, and it is kept in agreement with
// the codegen/runtime wrapper-synthesis table by the cross-package consistency
// test (internal/codegen/builtinref_consistency_test.go).
//
// Funcref-ability is a CODEGEN property, not a type-signature property: a builtin
// is referenceable only when its lowering is one of the three uniform prelude
// helper shapes (total pass-through, located name-injected, void located) over
// scalar (int/float/bool/string) arguments. Builtins with bespoke inline
// lowering, arg-type dispatch, composite/handle args or results, or higher-order
// generic shapes are NOT referenceable and are rejected at compile time with a
// reason. The set below is therefore an explicit allowlist transcribed from the
// genBuiltinCall switch, not derived from builtinSigs.

// BuiltinFuncrefClass labels a builtin's funcref-ability. The first three are
// generatable (a wrapper is synthesized, the value-position use records a
// FuncRef); the rest are rejected in value position.
type BuiltinFuncrefClass string

const (
	// Generatable categories (a __wisp_builtin_<name> wrapper exists).
	FuncrefMonomorphic BuiltinFuncrefClass = "monomorphic" // >=1 scalar param, scalar result
	FuncrefVoid        BuiltinFuncrefClass = "void"        // scalar params, void result
	FuncrefNullary     BuiltinFuncrefClass = "nullary"     // no params, scalar result

	// Rejected categories (deferred; value position is a compile error).
	FuncrefOverloaded BuiltinFuncrefClass = "overloaded" // arg-type dispatch / multi-arm
	FuncrefGeneric    BuiltinFuncrefClass = "generic"    // type-variable generic
	FuncrefMapFilter  BuiltinFuncrefClass = "mapfilter"  // map/filter dual-axis
	FuncrefStatement  BuiltinFuncrefClass = "statement"  // statement-only / void combinator
	FuncrefBespoke    BuiltinFuncrefClass = "bespoke"    // scalar-shaped but no uniform lowering
)

// builtinFuncrefGeneratable is the explicit allowlist of builtins that ARE
// referenceable as function values. The underlying prelude helper name, its
// "located" (fallible, name-as-$1) vs "total" (pass-through) shape, and the
// prelude dependency live in internal/runtime (builtinWrapperSpecs), pinned to
// this set by the consistency test. The funcref TYPE is derived from builtinSigs.
//
// Transcribed from the genBuiltinCall switch in internal/codegen/expr.go:
// total => genHelperCall, located => genLocatedHelperCall, void => the
// genVoidLocatedHelperCall cases (also located, void result).
var builtinFuncrefGeneratable = map[string]bool{
	// --- total (pass-through) string/bool + path builtins ---
	"lower": true, "upper": true, "trim": true,
	"trim_start": true, "trim_end": true, "trim_prefix": true, "trim_suffix": true,
	"is_empty": true, "reverse_string": true,
	"starts_with": true, "ends_with": true,
	"has_env":     true,
	"pid_alive":   true,
	"file_exists": true, "is_dir": true, "is_file": true, "is_symlink": true,
	"dir_name": true, "base_name": true,
	// total nullary
	"cwd": true, "read_stdin": true, "now": true, "int_max": true, "int_min": true,
	"pi": true, "program_path": true,

	// --- located (fallible; name injected as $1) scalar builtins ---
	"replace": true, "replace_first": true, "matches": true, "regex_replace": true,
	"repeat": true, "count": true, "substring": true, "char_at": true,
	"pad_start": true, "pad_end": true, "ord": true, "chr": true,
	"read_file": true, "file_size": true,
	"sqrt": true, "exp": true, "ln": true, "log10": true, "log2": true, "pow": true,
	"floor": true, "ceil": true, "round": true, "trunc": true,
	"format_float": true, "gcd": true, "lcm": true, "random": true,
	// located nullary
	"temp_file": true, "temp_dir": true,

	// --- void located (side-effecting; name injected as $1, no result) ---
	"write_file": true, "append_file": true, "set_env": true, "unset_env": true,
	"set_stdin": true, "chmod": true, "symlink": true, "symlink_force": true,
	"make_fifo": true, "make_dir": true, "remove_file": true, "remove_dir": true,
	"rename": true, "sleep": true, "change_dir": true,
}

// funcrefArm describes one int/float overload arm of an overloaded builtin
// referenceable as a funcref value.
type funcrefArm struct {
	suffix string // mangled-name suffix, e.g. "int"/"float"
	params []Type
	result Type
}

// overloadedFuncrefArms lists, for each int/float-overloaded builtin that IS
// referenceable as a funcref value, its concrete arms. Each arm gets its own
// standalone prelude helper and wrapper (__wisp_builtin_<name>_<suffix>),
// because a funcref value is a single compile-time-chosen shell function name:
// there is no runtime dispatch on argument type. The arm is selected by the
// checker from the expected funcref type (a `let f: fn(...)->... = name`
// annotation or other contextual expected type); see resolveOverloadedFuncref.
// Transcribed from the checkAbsCall/checkMinMaxCall/checkClampCall/
// checkSignCall special-case checkers in internal/types/call.go, which permit
// exactly int or float (never a mix) for each of these builtins.
var overloadedFuncrefArms = map[string][]funcrefArm{
	"abs":   {{"int", []Type{Int}, Int}, {"float", []Type{Float}, Float}},
	"min":   {{"int", []Type{Int, Int}, Int}, {"float", []Type{Float, Float}, Float}},
	"max":   {{"int", []Type{Int, Int}, Int}, {"float", []Type{Float, Float}, Float}},
	"clamp": {{"int", []Type{Int, Int, Int}, Int}, {"float", []Type{Float, Float, Float}, Float}},
	// sign's result is Int in BOTH arms (it never returns the operand, only
	// -1/0/1), so only the parameter type distinguishes the two arms.
	"sign": {{"int", []Type{Int}, Int}, {"float", []Type{Float}, Int}},

	// contains/index_of are arg-1-type-dispatched (string vs array), transcribed
	// from checkContainsCall/checkIndexOfCall. The array arm is pinned to a
	// single concrete element type (int[]) rather than supporting unbounded
	// generic element types (plan.md PR B scope): a funcref value is a single
	// compile-time-chosen shell function name, and int[] is the only element
	// type this arm needs to prove pinned per the Tests-minimum list.
	"contains": {{"string", []Type{String, String}, Bool}, {"array_int", []Type{arrayType(Int), Int}, Bool}},
	"index_of": {{"string", []Type{String, String}, optionalType(Int)}, {"array_int", []Type{arrayType(Int), Int}, optionalType(Int)}},
}

// overloadedFuncrefMangled is the eta-expansion wrapper name one arm of an
// overloaded builtin funcref records. Both the bare-ident path (checkIdent)
// and the namespaced-member path (Part 3, e.g. math.min) mint the same name
// for the same (name, arm) pair, matching the plain builtinFuncrefMangled
// convention used by the unambiguous-shape builtins.
func overloadedFuncrefMangled(name, suffix string) string {
	return "__wisp_builtin_" + name + "_" + suffix
}

// resolveOverloadedFuncref selects the arm of an overloaded builtin whose
// funcref type is IDENTICAL to want. It returns ok == false when want is
// Invalid (no annotation/context to disambiguate) or matches no arm; the
// caller reports a "needs a funcref type annotation" diagnostic in that case.
func resolveOverloadedFuncref(name string, want Type) (Type, string, bool) {
	if want == Invalid {
		return Invalid, "", false
	}
	for _, a := range overloadedFuncrefArms[name] {
		ft := funcType(a.params, a.result)
		if ft == want {
			return ft, overloadedFuncrefMangled(name, a.suffix), true
		}
	}
	// Params-only fallback: a higher-order builtin argument position (e.g.
	// map(xs, abs), where xs: int[]) knows the funcref's expected parameter
	// types from the array element type but not its result type -- that is
	// what the caller (map) is solving for. Such callers signal this by
	// passing a func type whose result is the Invalid sentinel (never a real
	// annotation: "invalid" is not a parseable type name), matching arms on
	// parameter types alone in that case. See higherOrderArgs.
	if isFuncref(want) && funcRetType(want) == Invalid {
		wantParams := funcParamTypes(want)
		for _, a := range overloadedFuncrefArms[name] {
			if len(a.params) != len(wantParams) {
				continue
			}
			match := true
			for i := range wantParams {
				if a.params[i] != wantParams[i] {
					match = false
					break
				}
			}
			if match {
				return funcType(a.params, a.result), overloadedFuncrefMangled(name, a.suffix), true
			}
		}
	}
	return Invalid, "", false
}

// joinedOverloadedArms renders the comma-", "-joined disp(funcType(...)) of
// every arm of an overloaded builtin, in overloadedFuncrefArms' declared
// slice order, for the "%q has no function-reference form matching %s;
// supported: %s" diagnostic (expr.go :203/:803).
func joinedOverloadedArms(name string) string {
	arms := overloadedFuncrefArms[name]
	parts := make([]string, len(arms))
	for i, a := range arms {
		parts[i] = disp(funcType(a.params, a.result))
	}
	return strings.Join(parts, ", ")
}

// OverloadedFuncrefWrapperIDs returns the funcref mangled wrapper ids for every
// arm of every overloaded builtin referenceable as a funcref value (e.g.
// "__wisp_builtin_abs_int", "__wisp_builtin_abs_float"). Exported for the
// cross-package consistency test, which checks these ids the same way it checks
// the plain generatable allowlist's ids.
func OverloadedFuncrefWrapperIDs() []string {
	var out []string
	for name, arms := range overloadedFuncrefArms {
		for _, a := range arms {
			out = append(out, overloadedFuncrefMangled(name, a.suffix))
		}
	}
	return out
}

// genericFuncrefAxes lists, for each higher-order builtin referenceable as a
// funcref value, the container-shape axes it supports (e.g. map supports
// array/optional/result; filter supports only array/optional; map_err supports
// only result). Unlike overloadedFuncrefArms, these builtins are not finite-arm
// dispatched on a fixed scalar type: the shell lowering they share with the
// direct-call path (genMap/genFilter/genEach/... in internal/codegen) does not
// vary with the element/result SCALAR type, only with the CONTAINER shape. So a
// single wrapper per (builtin, axis) pair correctly serves any element/result
// type -- there is no monomorphization-per-scalar-type step, unlike contains/
// index_of's array arm (which is pinned to one concrete element type because its
// bespoke wrapper body performs a scalar EQUALITY comparison, not a pass-through
// indirect call). validate reports whether want (a fn(...)->... funcref type) has
// the shape this axis requires, for ANY element/result type.
type genericFuncrefAxis struct {
	suffix   string // mangled-name suffix, e.g. "array"/"optional"/"result"
	validate func(want Type) bool
}

var genericFuncrefAxes = map[string][]genericFuncrefAxis{
	// map(xs: T[], f: fn(T)->U) -> U[]
	// map(o: Optional[T], f: fn(T)->U) -> Optional[U]
	// map(r: Result[T], f: fn(T)->U) -> Result[U]
	"map": {
		{"array", func(want Type) bool {
			ps := funcParamTypes(want)
			if len(ps) != 2 || !isArray(ps[0]) || !isFuncref(ps[1]) {
				return false
			}
			pps := funcParamTypes(ps[1])
			if len(pps) != 1 || pps[0] != elemType(ps[0]) {
				return false
			}
			u := funcRetType(ps[1])
			return u != Void && funcRetType(want) == arrayType(u)
		}},
		{"optional", func(want Type) bool {
			ps := funcParamTypes(want)
			if len(ps) != 2 || !isOptional(ps[0]) || !isFuncref(ps[1]) || len(funcParamTypes(ps[1])) != 1 {
				return false
			}
			if funcParamTypes(ps[1])[0] != optionalElemType(ps[0]) {
				return false
			}
			u := funcRetType(ps[1])
			return u != Void && funcRetType(want) == optionalType(u)
		}},
		{"result", func(want Type) bool {
			ps := funcParamTypes(want)
			if len(ps) != 2 || !isResult(ps[0]) || !isFuncref(ps[1]) || len(funcParamTypes(ps[1])) != 1 {
				return false
			}
			if funcParamTypes(ps[1])[0] != resultElemType(ps[0]) {
				return false
			}
			u := funcRetType(ps[1])
			return u != Void && funcRetType(want) == resultType(u)
		}},
	},
	// filter(xs: T[], f: fn(T)->bool) -> T[]
	// filter(o: Optional[T], f: fn(T)->bool) -> Optional[T]
	"filter": {
		{"array", func(want Type) bool {
			ps := funcParamTypes(want)
			if len(ps) != 2 || !isArray(ps[0]) || !isFuncref(ps[1]) || len(funcParamTypes(ps[1])) != 1 {
				return false
			}
			return funcParamTypes(ps[1])[0] == elemType(ps[0]) && funcRetType(ps[1]) == Bool && funcRetType(want) == ps[0]
		}},
		{"optional", func(want Type) bool {
			ps := funcParamTypes(want)
			if len(ps) != 2 || !isOptional(ps[0]) || !isFuncref(ps[1]) || len(funcParamTypes(ps[1])) != 1 {
				return false
			}
			return funcParamTypes(ps[1])[0] == optionalElemType(ps[0]) && funcRetType(ps[1]) == Bool && funcRetType(want) == ps[0]
		}},
	},
	// each(xs: T[], f: fn(T)->void) -> void
	"each": {
		{"array", func(want Type) bool {
			ps := funcParamTypes(want)
			if len(ps) != 2 || !isArray(ps[0]) || !isFuncref(ps[1]) || len(funcParamTypes(ps[1])) != 1 {
				return false
			}
			return funcParamTypes(ps[1])[0] == elemType(ps[0]) && funcRetType(ps[1]) == Void && funcRetType(want) == Void
		}},
	},
	// reduce(xs: T[], init: U, f: fn(U,T)->U) -> U
	"reduce": {
		{"array", func(want Type) bool {
			ps := funcParamTypes(want)
			if len(ps) != 3 || !isArray(ps[0]) || !isFuncref(ps[2]) {
				return false
			}
			fp := funcParamTypes(ps[2])
			if len(fp) != 2 || fp[0] != ps[1] || fp[1] != elemType(ps[0]) {
				return false
			}
			return funcRetType(ps[2]) == ps[1] && funcRetType(want) == ps[1]
		}},
	},
	// sort_by(xs: T[], f: fn(T,T)->bool) -> T[]
	"sort_by": {
		{"array", func(want Type) bool {
			ps := funcParamTypes(want)
			if len(ps) != 2 || !isArray(ps[0]) || !isFuncref(ps[1]) {
				return false
			}
			et := elemType(ps[0])
			fp := funcParamTypes(ps[1])
			return len(fp) == 2 && fp[0] == et && fp[1] == et && funcRetType(ps[1]) == Bool && funcRetType(want) == ps[0]
		}},
	},
	// find(xs: T[], f: fn(T)->bool) -> Optional[int]
	"find": {
		{"array", func(want Type) bool {
			ps := funcParamTypes(want)
			if len(ps) != 2 || !isArray(ps[0]) || !isFuncref(ps[1]) || len(funcParamTypes(ps[1])) != 1 {
				return false
			}
			return funcParamTypes(ps[1])[0] == elemType(ps[0]) && funcRetType(ps[1]) == Bool && funcRetType(want) == optionalType(Int)
		}},
	},
	// any/all(xs: T[], f: fn(T)->bool) -> bool
	"any": {
		{"array", func(want Type) bool { return anyAllShape(want) }},
	},
	"all": {
		{"array", func(want Type) bool { return anyAllShape(want) }},
	},
	// count_where(xs: T[], f: fn(T)->bool) -> int
	"count_where": {
		{"array", func(want Type) bool {
			ps := funcParamTypes(want)
			if len(ps) != 2 || !isArray(ps[0]) || !isFuncref(ps[1]) || len(funcParamTypes(ps[1])) != 1 {
				return false
			}
			return funcParamTypes(ps[1])[0] == elemType(ps[0]) && funcRetType(ps[1]) == Bool && funcRetType(want) == Int
		}},
	},
	// and_then(Optional[T], fn(T)->Optional[U]) -> Optional[U]
	// and_then(Result[T],   fn(T)->Result[U])   -> Result[U]
	"and_then": {
		{"optional", func(want Type) bool {
			ps := funcParamTypes(want)
			if len(ps) != 2 || !isOptional(ps[0]) || !isFuncref(ps[1]) || len(funcParamTypes(ps[1])) != 1 {
				return false
			}
			ret := funcRetType(ps[1])
			return funcParamTypes(ps[1])[0] == optionalElemType(ps[0]) && isOptional(ret) && funcRetType(want) == ret
		}},
		{"result", func(want Type) bool {
			ps := funcParamTypes(want)
			if len(ps) != 2 || !isResult(ps[0]) || !isFuncref(ps[1]) || len(funcParamTypes(ps[1])) != 1 {
				return false
			}
			ret := funcRetType(ps[1])
			return funcParamTypes(ps[1])[0] == resultElemType(ps[0]) && isResult(ret) && funcRetType(want) == ret
		}},
	},
	// or_else(Optional[T], fn()->Optional[T])    -> Optional[T]
	// or_else(Result[T],   fn(error)->Result[T]) -> Result[T]
	"or_else": {
		{"optional", func(want Type) bool {
			ps := funcParamTypes(want)
			if len(ps) != 2 || !isOptional(ps[0]) || !isFuncref(ps[1]) || len(funcParamTypes(ps[1])) != 0 {
				return false
			}
			ret := funcRetType(ps[1])
			return isOptional(ret) && optionalElemType(ret) == optionalElemType(ps[0]) && funcRetType(want) == ps[0]
		}},
		{"result", func(want Type) bool {
			ps := funcParamTypes(want)
			if len(ps) != 2 || !isResult(ps[0]) || !isFuncref(ps[1]) {
				return false
			}
			fp := funcParamTypes(ps[1])
			if len(fp) != 1 || !isErrorType(fp[0]) {
				return false
			}
			ret := funcRetType(ps[1])
			return isResult(ret) && resultElemType(ret) == resultElemType(ps[0]) && funcRetType(want) == ps[0]
		}},
	},
	// map_err(Result[T], fn(error)->error) -> Result[T]
	"map_err": {
		{"result", func(want Type) bool {
			ps := funcParamTypes(want)
			if len(ps) != 2 || !isResult(ps[0]) || !isFuncref(ps[1]) {
				return false
			}
			fp := funcParamTypes(ps[1])
			return len(fp) == 1 && isErrorType(fp[0]) && isErrorType(funcRetType(ps[1])) && funcRetType(want) == ps[0]
		}},
	},
}

// anyAllShape validates the shared any/all funcref shape:
// fn(T[], fn(T)->bool) -> bool.
func anyAllShape(want Type) bool {
	ps := funcParamTypes(want)
	if len(ps) != 2 || !isArray(ps[0]) || !isFuncref(ps[1]) || len(funcParamTypes(ps[1])) != 1 {
		return false
	}
	return funcParamTypes(ps[1])[0] == elemType(ps[0]) && funcRetType(ps[1]) == Bool && funcRetType(want) == Bool
}

// genericFuncrefMangled is the eta-expansion wrapper name one axis of a generic
// higher-order builtin funcref records: "__wisp_builtin_<name>_<axis>". Both the
// bare-ident path (checkIdent) and the namespaced-member path (Part 3, e.g.
// array.map) mint the same name for the same (name, axis) pair.
func genericFuncrefMangled(name, suffix string) string {
	return "__wisp_builtin_" + name + "_" + suffix
}

// resolveGenericFuncref selects the axis of a generic higher-order builtin whose
// SHAPE (not exact type identity, since the element/result types are free
// variables) matches want. It returns ok == false when want is Invalid, not a
// funcref type, or matches no axis; the caller reports a "needs a funcref type
// annotation" diagnostic in that case.
func resolveGenericFuncref(name string, want Type) (Type, string, bool) {
	if want == Invalid || !isFuncref(want) {
		return Invalid, "", false
	}
	for _, ax := range genericFuncrefAxes[name] {
		if ax.validate(want) {
			return want, genericFuncrefMangled(name, ax.suffix), true
		}
	}
	return Invalid, "", false
}

// GenericFuncrefWrapperIDs returns the funcref mangled wrapper ids for every
// axis of every generic higher-order builtin referenceable as a funcref value
// (e.g. "__wisp_builtin_map_array", "__wisp_builtin_map_optional"). Exported for
// the cross-package consistency test.
func GenericFuncrefWrapperIDs() []string {
	var out []string
	for name, axes := range genericFuncrefAxes {
		for _, ax := range axes {
			out = append(out, genericFuncrefMangled(name, ax.suffix))
		}
	}
	return out
}

// joinedGenericAxisNames renders the comma-", "-joined ax.suffix of every
// axis of a generic higher-order builtin, in genericFuncrefAxes' declared
// slice order, for the "%q has no function-reference form matching %s;
// supported containers: %s" diagnostic (expr.go :215/:816). Concrete arm
// types are not enumerable for a generic axis (element/result types are free
// variables, see genericFuncrefAxis.validate's doc comment,
// funcref_class.go:196-199), so the container shapes (array/optional/result)
// are named instead.
func joinedGenericAxisNames(name string) string {
	axes := genericFuncrefAxes[name]
	parts := make([]string, len(axes))
	for i, ax := range axes {
		parts[i] = ax.suffix
	}
	return strings.Join(parts, ", ")
}

// OverloadedFuncrefNames returns the bare builtin names covered by
// overloadedFuncrefArms. Exported for the consistency test.
func OverloadedFuncrefNames() []string {
	out := make([]string, 0, len(overloadedFuncrefArms))
	for name := range overloadedFuncrefArms {
		out = append(out, name)
	}
	return out
}

// GenericFuncrefNames returns the bare builtin names covered by
// genericFuncrefAxes. Exported for the consistency test.
func GenericFuncrefNames() []string {
	out := make([]string, 0, len(genericFuncrefAxes))
	for name := range genericFuncrefAxes {
		out = append(out, name)
	}
	return out
}

// isScalarFuncType reports whether t is one of the four scalar value types that a
// synthesized funcref wrapper can carry through the shell calling convention.
func isScalarFuncType(t Type) bool {
	return t == Int || t == Float || t == Bool || t == String
}

// builtinFuncrefGeneratableSig reports whether a builtin's signature is
// uniform-wrapper-shaped: every parameter has exactly one accepted type and that
// type is scalar, and the result is scalar or void. This is a defensive invariant
// over the allowlist above (checked exhaustively by a test), NOT the membership
// test -- membership is the explicit allowlist, because a scalar-shaped signature
// (e.g. to_int) does not by itself imply a uniform lowering.
func builtinFuncrefGeneratableSig(name string) bool {
	sig, ok := builtinSigs[name]
	if !ok {
		return false
	}
	for _, p := range sig.params {
		if len(p.types) != 1 || !isScalarFuncType(p.types[0]) {
			return false
		}
	}
	return sig.result == Void || isScalarFuncType(sig.result)
}

// BuiltinFuncrefGeneratable reports whether a builtin is referenceable as a
// function value (a wrapper is synthesized). Exported for the consistency test.
func BuiltinFuncrefGeneratable(name string) bool {
	return builtinFuncrefGeneratable[name]
}

// GeneratableBuiltinFuncrefs returns a copy of the generatable allowlist.
// Exported for the consistency test.
func GeneratableBuiltinFuncrefs() map[string]bool {
	out := make(map[string]bool, len(builtinFuncrefGeneratable))
	for k := range builtinFuncrefGeneratable {
		out[k] = true
	}
	return out
}

// builtinFuncrefType returns the funcref Type of a generatable builtin, derived
// from its flat builtinSigs entry (each param's single accepted type, plus the
// result). It is the one place funcref types are minted for builtins, shared by
// the bare-ident path (checkIdent) and the namespaced-member path (Part 3).
func builtinFuncrefType(name string) Type {
	sig := builtinSigs[name]
	params := make([]Type, len(sig.params))
	for i, p := range sig.params {
		params[i] = p.types[0]
	}
	return funcType(params, sig.result)
}

// builtinFuncrefMangled is the eta-expansion wrapper name a funcref to a builtin
// records. Both the bare-ident and namespaced-member paths mint the same name,
// so a wrapper is emitted (and tree-shaken) once per builtin regardless of how it
// is referenced.
func builtinFuncrefMangled(name string) string {
	return "__wisp_builtin_" + name
}

// funcrefRejectReason computes the parenthetical reason clause shared by the
// two non-referenceable-builtin diagnostics (expr.go:231 bare-ident, :819
// qualified member): why `name` has no funcref-shaped scalar lowering. Both
// call sites must produce byte-identical reason text for the same builtin.
func funcrefRejectReason(name string) string {
	sig := builtinSigs[name]
	switch {
	case sig.result == Void:
		return "it is statement-only"
	case sig.result == Invalid:
		return "it is overloaded or generic"
	case builtinOverloaded[name]:
		return "it is overloaded or generic"
	}
	return "it has no single funcref-shaped scalar lowering"
}

// BuiltinFuncrefClassOf classifies a builtin for the exhaustive surface test and
// diagnostics. Generatable builtins are split by shape; the rest get a
// best-effort rejection label. Panics for a non-builtin name (callers pass only
// keys of builtinSigs).
func BuiltinFuncrefClassOf(name string) BuiltinFuncrefClass {
	sig, ok := builtinSigs[name]
	if !ok {
		panic("BuiltinFuncrefClassOf: not a builtin: " + name)
	}
	if builtinFuncrefGeneratable[name] {
		switch {
		case len(sig.params) == 0:
			return FuncrefNullary
		case sig.result == Void:
			return FuncrefVoid
		default:
			return FuncrefMonomorphic
		}
	}
	if _, ok := overloadedFuncrefArms[name]; ok {
		return FuncrefOverloaded
	}
	if _, ok := genericFuncrefAxes[name]; ok {
		switch name {
		case "map", "filter":
			return FuncrefMapFilter
		default:
			return FuncrefGeneric
		}
	}
	switch {
	case sig.result == Void:
		return FuncrefStatement
	default:
		return FuncrefBespoke
	}
}
