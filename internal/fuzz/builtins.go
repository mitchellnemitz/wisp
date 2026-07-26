// internal/fuzz/builtins.go
package fuzz

import (
	"sort"
	"strings"
)

// effectfulDenylist: FR-006 effectful/nondeterministic families + non-value/abort
// builtins the generator never emits (print is the witness mechanism, emitted
// structurally; exit/assert*/skip abort the process). A whole-namespace token
// ("process"/"fs"/"env") excludes every "<token>." member -- see isExcluded.
// math.random is listed explicitly (not the bare token "math", whose other
// members -- abs/min/max/clamp/sign/gcd/lcm/sqrt/pow/exp/ln/log10/log2/floor/
// ceil/round/trunc/pi/int_max/int_min -- are deterministic and ARE admitted):
// random() is NONDETERMINISTIC (LC_ALL=C awk srand() seeded from $$ + a per-call
// counter -- internal/runtime/prelude.go), so every program calling it would
// differ run-to-run and per shell (different PIDs), the exact
// nondeterminism-masquerading-as-divergence FR-006/FR-016 forbid. There is no
// bare flat "random" in the real catalog (types.BuiltinNames() omits it --
// random was moved to the math module home, isRemovableBuiltin), only
// "math.random", so only that form is listed.
var effectfulDenylist = []string{
	"now", "sleep",
	"read_line", "read_stdin", "read_secret", "set_stdin",
	"on_exit", "on_signal",
	"process", "fs", "env",
	"math.random",
	"print", "exit",
	"assert", "assert_eq", "assert_ne", "assert_contains",
	"assert_some", "assert_none", "assert_ok", "assert_err",
	"skip", "test_tmpdir",
}

// nonGenerableDenylist: pure builtins the v1 generator structurally cannot emit
// a witnessed value for -- a documented, source-cited frozen-snapshot
// exclusion. This package's own Type (node.go) has exactly six Kinds
// (KInt/KFloat/KBool/KString/KArray/KDict); builtinSig's only accommodation for
// a non-printable handle is the RetHandle bolt-on for Optional. Anything that
// needs a Kind this package does not have -- a function reference, a composite
// tuple, an ErrorType handle, or a Result[T,E] handle -- cannot be constructed
// or captured by the generator, independent of whether real wisp itself could
// build or print the value. Four structural reasons:
//
//  1. FUNCTION-REFERENCE args (a `f` param). The v1 generation IR has no
//     function-definition node and emits no function definitions (FR-002
//     pure-core scope), so a funcref argument can never be synthesized. These
//     are the higher-order builtins map/filter/each/reduce/sort_by/find/any/
//     all/count_where and the combinators and_then/or_else/map_err
//     (internal/types/builtins.go:144-211,301-308,493; internal/types/
//     collections.go higher-order handlers -- each has a `f` param the fixed
//     sig table cannot express). The flat forms of the array-family ones
//     (map/filter/each/...) are NOT in the real catalog at all -- they were
//     moved to the array module home (types.BuiltinNames() omits them,
//     isRemovableBuiltin) -- so only the array.-namespaced form is listed for
//     those; and_then/or_else/map_err have no namespaced duplicate and stay
//     flat-only.
//  2. COMPOSITE results the IR cannot represent or print: parse_args ->
//     ({string:string}, string[], string[]) (a bare 3-tuple; builtins.go:
//     182-188, result Invalid, hand-built shape) and array.zip -> (T,U)[] (a
//     tuple array; builtins.go:169-175, core_members.go array.zip). The flat
//     "zip" is likewise not in the real catalog (moved to array), so only
//     array.zip is listed.
//  3. ErrorType handle. error/error_with construct an ErrorType value
//     (builtins.go:48-59); cause(err) consumes AND returns one
//     (Optional[error], builtins.go:71-76); wrap(err,msg) consumes and returns
//     one (builtins.go:62-68). ErrorType has no Kind in this package's Type,
//     and builtinSig has no ErrorHandle field parallel to RetHandle, so the
//     generator can neither construct an ErrorType argument nor declare a
//     LetStmt of that type for a return -- independent of whether real wisp's
//     debug() could render one. (These are not "abort" builtins -- error()/
//     error_with() never abort -- the exclusion reason is structural
//     unconstructability, not effect.)
//  4. Result[T,E] handle, WHOLE-FAMILY. is_ok/is_err (builtins.go:609-610) and
//     unwrap_err (builtins.go:612, also reason 3 via its ErrorType return) all
//     consume a Result[T,E] value. Result[T,E] is constructible ONLY via the
//     Ok(x)/Err(e) reserved-constant syntax (reservedConstants map,
//     builtins.go:715-718; special-cased in internal/types/call.go
//     checkOkCall/checkErrCall) -- Ok/Err are NOT catalog builtins (absent
//     from types.BuiltinNames() and every types.CoreMembers(ns)), so no
//     admitted builtin in this snapshot produces a Result value, and
//     builtinSig has no Result-handle bolt-on (unlike Optional's RetHandle).
//     No admitted builtin can ever supply is_ok/is_err a well-typed argument,
//     so (parallel to json, Task 4's FR-004 reconciliation) "result" is a
//     WHOLE-FAMILY exclusion, out of v1 SC-009 scope entirely -- not a
//     per-member carve, since it has zero generable members.
//  5. HANDLE type the IR has no Kind for, WHOLE NAMESPACE: json. Every json.*
//     member consumes or produces jsonValueType (core_members.go:63-83), so
//     none of json.encode/decode/from_*/null/array/object/type_of/get/at/as_*
//     can be built or witnessed; the family has zero generable members and is
//     out of v1 scope (see the Task 4 FR-004 reconciliation in the plan). The
//     single token "json" excludes all json.* members via the
//     namespace-prefix rule in isExcluded.
//
// This snapshot is frozen and kept honest by
// TestAdmittedSetIsExactlyCatalogMinusExcluded: a new catalog builtin forces a
// conscious admit-or-deny decision here.
var nonGenerableDenylist = []string{
	// higher-order (funcref arg), flat (no namespaced duplicate in the catalog)
	"and_then", "or_else", "map_err",
	// higher-order (funcref arg), array.-namespaced (the flat forms are not in
	// the real catalog -- moved to the array module home)
	"array.map", "array.filter", "array.each", "array.reduce", "array.sort_by",
	"array.find", "array.any", "array.all", "array.count_where",
	// composite results the IR cannot represent/print
	"parse_args", "array.zip",
	// ErrorType handle, unconstructable/uncapturable by this package's Type
	"error", "error_with", "cause", "wrap",
	// Result[T,E] handle: whole-family exclusion (reason 4 above)
	"is_ok", "is_err", "unwrap_err",
	// whole namespace: every member touches the jsonValueType handle (reason 5)
	"json",
}

func isExcluded(name string) bool {
	for _, d := range append(append([]string{}, effectfulDenylist...), nonGenerableDenylist...) {
		if name == d || strings.HasPrefix(name, d+".") {
			return true
		}
	}
	return false
}

// builtinSig is one admitted, generable builtin. Family groups it for SC-009
// (namespace name, or a specific flat sub-family -- never one coarse "core"
// bucket). RetHandle marks a non-printable handle return (Optional) wrapped by
// Task 6 to reach stdout. Special marks a builtin whose call shape is
// special-cased in the checker (is_some/is_none/unwrap/unwrap_or/dict.get):
// its Params are recorded as nil because the fixed table cannot express them,
// so the GENERIC call paths MUST NOT synthesize it -- only a family-specific
// witness path in Task 6 builds it in a hand-verified good shape. Without this
// flag a nil-Params entry with the zero-value Ret (Type{}.Kind == KInt, since
// KInt is the iota zero) would be misread as a zero-arg int builtin.
//
// This package's Type has no Void kind. A handful of admitted mutating
// builtins (array.push/insert_at/remove_at, dict.clear/remove) genuinely
// return void in the real catalog; their Ret field here is a placeholder
// (marked "Void" in a trailing comment) that Task 6 MUST NOT print directly --
// they are witnessed by mutating their container argument and printing THAT
// afterward (e.g. array.push(xs, 5); print(length(xs))), not by treating Ret
// as the call's own printable value.
type builtinSig struct {
	Name      string
	Family    string
	Params    []Type
	Ret       Type
	RetHandle bool
	Special   bool
	Import    string
}

var (
	intT    = Type{Kind: KInt}
	floatT  = Type{Kind: KFloat}
	boolT   = Type{Kind: KBool}
	stringT = Type{Kind: KString}
	intArrT = Type{Kind: KArray, Elem: &Type{Kind: KInt}}
	strArrT = Type{Kind: KArray, Elem: &Type{Kind: KString}}
	intDctT = Type{Kind: KDict, Elem: &Type{Kind: KInt}}
)

func intArrArrT() Type {
	e := intArrT
	return Type{Kind: KArray, Elem: &e}
}

// admittedBuiltins is the v1 snapshot: catalog minus effectfulDenylist minus
// nonGenerableDenylist, enforced exactly by TestAdmittedSetIsExactlyCatalog
// MinusExcluded. Every entry's Params/Ret are read from the real signature in
// internal/types/builtins.go / core_members.go / the dedicated checker
// handlers in internal/types/collections.go (not guessed): where a builtin is
// generic (e.g. array.reverse: T[]->T[]), the int/int[] instantiation is
// recorded here (Task 6 witnesses with concrete int payloads).
var admittedBuiltins = []builtinSig{
	// --- convert (scalar constructors; abort on bad input, NOT Optional) ---
	{Name: "to_string", Family: "convert", Params: []Type{intT}, Ret: stringT},
	{Name: "to_int", Family: "convert", Params: []Type{stringT}, Ret: intT},
	{Name: "to_float", Family: "convert", Params: []Type{stringT}, Ret: floatT},
	{Name: "to_bool", Family: "convert", Params: []Type{stringT}, Ret: boolT},

	// --- parse (Optional-returning scalar parsers; never abort) ---
	{Name: "parse_int", Family: "parse", Params: []Type{stringT}, Ret: intT, RetHandle: true},
	{Name: "parse_float", Family: "parse", Params: []Type{stringT}, Ret: floatT, RetHandle: true},
	{Name: "parse_bool", Family: "parse", Params: []Type{stringT}, Ret: boolT, RetHandle: true},

	// --- collection ---
	{Name: "length", Family: "collection", Params: []Type{stringT}, Ret: intT},

	// --- debug (structural renderer, S4) ---
	{Name: "debug", Family: "debug", Params: []Type{intT}, Ret: stringT},

	// --- option (Optional access; special-cased call shape in the checker) ---
	{Name: "is_some", Family: "option", Params: nil, Ret: Type{Kind: KBool}, Special: true},
	{Name: "is_none", Family: "option", Params: nil, Ret: Type{Kind: KBool}, Special: true},
	{Name: "unwrap", Family: "option", Params: nil, Ret: Type{}, Special: true},
	{Name: "unwrap_or", Family: "option", Params: nil, Ret: Type{}, Special: true},

	// --- math (namespace "math"; abs/min/max/clamp/sign admit the int
	// overload -- all five are int/float-overloaded and special-cased) ---
	{Name: "math.abs", Family: "math", Params: []Type{intT}, Ret: intT, Import: "math"},
	{Name: "math.min", Family: "math", Params: []Type{intT, intT}, Ret: intT, Import: "math"},
	{Name: "math.max", Family: "math", Params: []Type{intT, intT}, Ret: intT, Import: "math"},
	{Name: "math.clamp", Family: "math", Params: []Type{intT, intT, intT}, Ret: intT, Import: "math"},
	{Name: "math.sign", Family: "math", Params: []Type{intT}, Ret: intT, Import: "math"},
	{Name: "math.gcd", Family: "math", Params: []Type{intT, intT}, Ret: intT, Import: "math"},
	{Name: "math.lcm", Family: "math", Params: []Type{intT, intT}, Ret: intT, Import: "math"},
	{Name: "math.sqrt", Family: "math", Params: []Type{floatT}, Ret: floatT, Import: "math"},
	{Name: "math.pow", Family: "math", Params: []Type{floatT, floatT}, Ret: floatT, Import: "math"},
	{Name: "math.exp", Family: "math", Params: []Type{floatT}, Ret: floatT, Import: "math"},
	{Name: "math.ln", Family: "math", Params: []Type{floatT}, Ret: floatT, Import: "math"},
	{Name: "math.log10", Family: "math", Params: []Type{floatT}, Ret: floatT, Import: "math"},
	{Name: "math.log2", Family: "math", Params: []Type{floatT}, Ret: floatT, Import: "math"},
	{Name: "math.floor", Family: "math", Params: []Type{floatT}, Ret: intT, Import: "math"},
	{Name: "math.ceil", Family: "math", Params: []Type{floatT}, Ret: intT, Import: "math"},
	{Name: "math.round", Family: "math", Params: []Type{floatT}, Ret: intT, Import: "math"},
	{Name: "math.trunc", Family: "math", Params: []Type{floatT}, Ret: intT, Import: "math"},
	{Name: "math.pi", Family: "math", Params: nil, Ret: floatT, Import: "math"},
	{Name: "math.int_max", Family: "math", Params: nil, Ret: intT, Import: "math"},
	{Name: "math.int_min", Family: "math", Params: nil, Ret: intT, Import: "math"},

	// --- array (namespace "array"; generic members instantiated at int/int[]) ---
	{Name: "array.concat", Family: "array", Params: []Type{intArrT, intArrT}, Ret: intArrT, Import: "array"},
	{Name: "array.contains", Family: "array", Params: []Type{intArrT, intT}, Ret: boolT, Import: "array"},
	{Name: "array.drop", Family: "array", Params: []Type{intArrT, intT}, Ret: intArrT, Import: "array"},
	{Name: "array.first", Family: "array", Params: []Type{intArrT}, Ret: intT, Import: "array"},
	{Name: "array.flatten", Family: "array", Params: []Type{intArrArrT()}, Ret: intArrT, Import: "array"},
	{Name: "array.index_of", Family: "array", Params: []Type{intArrT, intT}, Ret: intT, RetHandle: true, Import: "array"},
	{Name: "array.insert_at", Family: "array", Params: []Type{intArrT, intT, intT}, Ret: Type{Kind: KInt}, Import: "array"}, // Void
	{Name: "array.is_empty", Family: "array", Params: []Type{intArrT}, Ret: boolT, Import: "array"},
	{Name: "array.last", Family: "array", Params: []Type{intArrT}, Ret: intT, Import: "array"},
	{Name: "array.pop", Family: "array", Params: []Type{intArrT}, Ret: intT, Import: "array"},
	{Name: "array.push", Family: "array", Params: []Type{intArrT, intT}, Ret: Type{Kind: KInt}, Import: "array"}, // Void
	{Name: "array.range", Family: "array", Params: []Type{intT}, Ret: intArrT, Import: "array"},
	{Name: "array.remove_at", Family: "array", Params: []Type{intArrT, intT}, Ret: Type{Kind: KInt}, Import: "array"}, // Void
	{Name: "array.reverse", Family: "array", Params: []Type{intArrT}, Ret: intArrT, Import: "array"},
	{Name: "array.slice", Family: "array", Params: []Type{intArrT, intT, intT}, Ret: intArrT, Import: "array"},
	{Name: "array.sort", Family: "array", Params: []Type{intArrT}, Ret: intArrT, Import: "array"},
	{Name: "array.sum", Family: "array", Params: []Type{intArrT}, Ret: intT, Import: "array"},
	{Name: "array.take", Family: "array", Params: []Type{intArrT, intT}, Ret: intArrT, Import: "array"},
	{Name: "array.unique", Family: "array", Params: []Type{intArrT}, Ret: intArrT, Import: "array"},

	// --- dict (namespace "dict"; K is always string in v1, V instantiated at int) ---
	{Name: "dict.clear", Family: "dict", Params: []Type{intDctT}, Ret: Type{Kind: KInt}, Import: "dict"}, // Void
	{Name: "dict.get", Family: "dict", Params: nil, Ret: intT, RetHandle: true, Special: true, Import: "dict"},
	{Name: "dict.has", Family: "dict", Params: []Type{intDctT, stringT}, Ret: boolT, Import: "dict"},
	{Name: "dict.is_empty", Family: "dict", Params: []Type{intDctT}, Ret: boolT, Import: "dict"},
	{Name: "dict.keys", Family: "dict", Params: []Type{intDctT}, Ret: strArrT, Import: "dict"},
	{Name: "dict.merge", Family: "dict", Params: []Type{intDctT, intDctT}, Ret: intDctT, Import: "dict"},
	{Name: "dict.remove", Family: "dict", Params: []Type{intDctT, stringT}, Ret: Type{Kind: KInt}, Import: "dict"}, // Void
	{Name: "dict.size", Family: "dict", Params: []Type{intDctT}, Ret: intT, Import: "dict"},
	{Name: "dict.values", Family: "dict", Params: []Type{intDctT}, Ret: intArrT, Import: "dict"},

	// --- string (namespace "string"; all 28 members are fixed-signature and
	// generable -- no funcref, no composite result, no handle type) ---
	{Name: "string.char_at", Family: "string", Params: []Type{stringT, intT}, Ret: stringT, Import: "string"},
	{Name: "string.chr", Family: "string", Params: []Type{intT}, Ret: stringT, Import: "string"},
	{Name: "string.contains", Family: "string", Params: []Type{stringT, stringT}, Ret: boolT, Import: "string"},
	{Name: "string.count", Family: "string", Params: []Type{stringT, stringT}, Ret: intT, Import: "string"},
	{Name: "string.ends_with", Family: "string", Params: []Type{stringT, stringT}, Ret: boolT, Import: "string"},
	{Name: "string.format_float", Family: "string", Params: []Type{floatT, intT}, Ret: stringT, Import: "string"},
	{Name: "string.index_of", Family: "string", Params: []Type{stringT, stringT}, Ret: intT, RetHandle: true, Import: "string"},
	{Name: "string.is_empty", Family: "string", Params: []Type{stringT}, Ret: boolT, Import: "string"},
	{Name: "string.join", Family: "string", Params: []Type{strArrT, stringT}, Ret: stringT, Import: "string"},
	{Name: "string.last_index_of", Family: "string", Params: []Type{stringT, stringT}, Ret: intT, RetHandle: true, Import: "string"},
	{Name: "string.lines", Family: "string", Params: []Type{stringT}, Ret: strArrT, Import: "string"},
	{Name: "string.lower", Family: "string", Params: []Type{stringT}, Ret: stringT, Import: "string"},
	{Name: "string.ord", Family: "string", Params: []Type{stringT}, Ret: intT, Import: "string"},
	{Name: "string.pad_end", Family: "string", Params: []Type{stringT, intT, stringT}, Ret: stringT, Import: "string"},
	{Name: "string.pad_start", Family: "string", Params: []Type{stringT, intT, stringT}, Ret: stringT, Import: "string"},
	{Name: "string.repeat", Family: "string", Params: []Type{stringT, intT}, Ret: stringT, Import: "string"},
	{Name: "string.replace", Family: "string", Params: []Type{stringT, stringT, stringT}, Ret: stringT, Import: "string"},
	{Name: "string.replace_first", Family: "string", Params: []Type{stringT, stringT, stringT}, Ret: stringT, Import: "string"},
	{Name: "string.reverse", Family: "string", Params: []Type{stringT}, Ret: stringT, Import: "string"},
	{Name: "string.split", Family: "string", Params: []Type{stringT, stringT}, Ret: strArrT, Import: "string"},
	{Name: "string.starts_with", Family: "string", Params: []Type{stringT, stringT}, Ret: boolT, Import: "string"},
	{Name: "string.substring", Family: "string", Params: []Type{stringT, intT, intT}, Ret: stringT, Import: "string"},
	{Name: "string.trim", Family: "string", Params: []Type{stringT}, Ret: stringT, Import: "string"},
	{Name: "string.trim_end", Family: "string", Params: []Type{stringT}, Ret: stringT, Import: "string"},
	{Name: "string.trim_prefix", Family: "string", Params: []Type{stringT, stringT}, Ret: stringT, Import: "string"},
	{Name: "string.trim_start", Family: "string", Params: []Type{stringT}, Ret: stringT, Import: "string"},
	{Name: "string.trim_suffix", Family: "string", Params: []Type{stringT, stringT}, Ret: stringT, Import: "string"},
	{Name: "string.upper", Family: "string", Params: []Type{stringT}, Ret: stringT, Import: "string"},

	// --- regex (namespace "regex"; POSIX ERE, fixed-signature, fallible on a
	// malformed pattern but otherwise pure/deterministic) ---
	{Name: "regex.matches", Family: "regex", Params: []Type{stringT, stringT}, Ret: boolT, Import: "regex"},
	{Name: "regex.find", Family: "regex", Params: []Type{stringT, stringT}, Ret: stringT, RetHandle: true, Import: "regex"},
	{Name: "regex.find_all", Family: "regex", Params: []Type{stringT, stringT}, Ret: strArrT, Import: "regex"},
	{Name: "regex.replace", Family: "regex", Params: []Type{stringT, stringT, stringT}, Ret: stringT, Import: "regex"},
}

// AdmittedFamilies returns the sorted, de-duplicated set of families across all
// admitted builtins -- the full expected family set the SC-009 coverage report
// checks witnessed families against, so a family with zero witnesses is
// reported MISSING rather than silently absent. Exported for cmd/wisp-fuzz.
func AdmittedFamilies() []string {
	seen := map[string]bool{}
	var out []string
	for _, b := range admittedBuiltins {
		if b.Family == "" || seen[b.Family] {
			continue
		}
		seen[b.Family] = true
		out = append(out, b.Family)
	}
	sort.Strings(out)
	return out
}
