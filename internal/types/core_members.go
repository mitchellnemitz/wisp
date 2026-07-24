package types

import (
	"sort"
	"strconv"
	"strings"

	"github.com/mitchellnemitz/wisp/internal/ast"
)

// This file is the reserved core-module bridge: the single, json-blind mechanism
// that resolves `ns.member` for a synthetic core module (json now;
// regex/env/fs/... in Units 12-19) against a table, delegating function members
// to the ordinary builtin machinery so codegen and the runtime prelude need no
// awareness of namespaces. A follow-on core module is added by appending a block
// to coreCatalog (plus its codegen cases + prelude helpers) -- no new control
// flow here.

// jsonValueType is the internal Type token for json.Value, the boxed dynamic JSON
// handle. The '.' is illegal in a source identifier, so the token never collides
// with a user type or a struct token (Name@modid). It is opaque per spec 4.1;
// the opaque-classification plumbing lives in composite.go/expr.go.
const jsonValueType Type = "json.Value"

// IsJSONValue reports whether t is the json.Value handle type. Exported for
// codegen, which dispatches the json builtins' boxing/handle lowering on it (and
// picks the decode[T] variant from CallInfo.Result).
func IsJSONValue(t Type) bool { return t == jsonValueType }

type coreMemberKind int

const (
	coreFunc coreMemberKind = iota
	coreType
	coreConst
)

// coreMember describes one member of a core module.
type coreMember struct {
	kind coreMemberKind

	// coreFunc:
	builtin       string                                 // CallInfo.Builtin key codegen dispatches on
	check         func(c *checker, n *ast.CallExpr) Type // custom checker; nil -> generic sig path
	sig           builtinSig                             // used when check == nil && !delegate
	delegate      bool                                   // route to the full flat dispatch (overloads, composite args, arg-domain checks)
	takesTypeArgs bool                                   // true only for json.decode

	// coreType:
	typ Type

	// coreConst:
	constVal ConstEntry
}

// coreCatalog maps namespace -> member name -> descriptor. It is a package var
// (not a const) so tests can inject a second reserved namespace to prove the
// bridge is generic; production code only reads it. Test-injected namespaces
// are always named with a "__" prefix (e.g. "__probe", "__dprobe") --
// crossNamespaceSuffix relies on this convention to skip them so a sentinel
// namespace never leaks into a real "did you mean" diagnostic.
var coreCatalog = map[string]map[string]coreMember{
	"json": {
		"Value":  {kind: coreType, typ: jsonValueType},
		"encode": {kind: coreFunc, builtin: "json_encode", sig: coreSig1(jsonValueType, String)},
		"decode": {kind: coreFunc, builtin: "json_decode", takesTypeArgs: true}, // .check set in init (avoids a static init cycle)
		// Scalar constructors are from_* (not int/string/float/bool): the bare type
		// names are keyword tokens the parser cannot use as a `.member` selector.
		"from_string": {kind: coreFunc, builtin: "json_from_string", sig: coreSig1(String, jsonValueType)},
		"from_int":    {kind: coreFunc, builtin: "json_from_int", sig: coreSig1(Int, jsonValueType)},
		"from_float":  {kind: coreFunc, builtin: "json_from_float", sig: coreSig1(Float, jsonValueType)},
		"from_bool":   {kind: coreFunc, builtin: "json_from_bool", sig: coreSig1(Bool, jsonValueType)},
		"null":        {kind: coreFunc, builtin: "json_null", sig: coreSig0(jsonValueType)},
		"array":       {kind: coreFunc, builtin: "json_array", sig: coreSig1(arrayType(jsonValueType), jsonValueType)},
		"object":      {kind: coreFunc, builtin: "json_object", sig: coreSig1(dictType(String, jsonValueType), jsonValueType)},
		"type_of":     {kind: coreFunc, builtin: "json_type_of", sig: coreSig1(jsonValueType, String)},
		"get":         {kind: coreFunc, builtin: "json_get", sig: coreSig2(jsonValueType, String, optionalType(jsonValueType))},
		"at":          {kind: coreFunc, builtin: "json_at", sig: coreSig2(jsonValueType, Int, optionalType(jsonValueType))},
		"as_string":   {kind: coreFunc, builtin: "json_as_string", sig: coreSig1(jsonValueType, String)},
		"as_int":      {kind: coreFunc, builtin: "json_as_int", sig: coreSig1(jsonValueType, Int)},
		"as_float":    {kind: coreFunc, builtin: "json_as_float", sig: coreSig1(jsonValueType, Float)},
		"as_bool":     {kind: coreFunc, builtin: "json_as_bool", sig: coreSig1(jsonValueType, Bool)},
	},
	// regex (Unit 12): pure aliases of the existing flat regex builtins. No custom
	// checkers, no codegen/prelude changes -- each member delegates to its builtin
	// key, so a namespaced call lowers byte-identically to the flat spelling.
	"regex": {
		"matches":  {kind: coreFunc, builtin: "matches", sig: coreSig2(String, String, Bool)},
		"find":     {kind: coreFunc, builtin: "regex_find", sig: coreSig2(String, String, optionalType(String))},
		"find_all": {kind: coreFunc, builtin: "regex_find_all", sig: coreSig2(String, String, arrayType(String))},
		"replace":  {kind: coreFunc, builtin: "regex_replace", sig: coreSig3(String, String, String, String)},
	},
	// env (Unit 13): pure aliases of the flat env builtins. Member names drop the
	// module-name affix (env/has_env/set_env/unset_env/env_or -> get/has/set/unset/
	// get_or). None is overloaded or arg-domain-checked, so all use coreSig. The
	// namespace name "env" equals the flat builtin "env"; the bare spelling is a
	// removed flat builtin (moved-to-module diagnostic); env.get(x) resolves via
	// the namespace. get's own coreSig is independent of builtinSigs["env"] (not a
	// delegate member), so it must be kept in sync by hand: env(name) returns
	// Optional[string] (FR-004), so get does too.
	"env": {
		"get":    {kind: coreFunc, builtin: "env", sig: coreSig1(String, optionalType(String))},
		"has":    {kind: coreFunc, builtin: "has_env", sig: coreSig1(String, Bool)},
		"set":    {kind: coreFunc, builtin: "set_env", sig: coreSig2(String, String, Void)},
		"unset":  {kind: coreFunc, builtin: "unset_env", sig: coreSig1(String, Void)},
		"get_or": {kind: coreFunc, builtin: "env_or", sig: coreSig2(String, String, String)},
	},
	// math (Unit 16): member name == flat builtin name verbatim. Overloaded
	// (abs/min/max/clamp/sign) and arg-domain-checked (gcd/random) members
	// DELEGATE to the flat dispatch so overload resolution and the compile-time
	// domain diagnostics are preserved; the rest are fixed-signature coreSig.
	// pi/int_max/int_min are NULLARY builtins today (called pi()), not compile-time
	// constants, so they are exposed as nullary function members (coreSig0) to stay
	// byte-identical to the flat spelling.
	"math": {
		"abs":     {kind: coreFunc, builtin: "abs", delegate: true},
		"min":     {kind: coreFunc, builtin: "min", delegate: true},
		"max":     {kind: coreFunc, builtin: "max", delegate: true},
		"clamp":   {kind: coreFunc, builtin: "clamp", delegate: true},
		"sign":    {kind: coreFunc, builtin: "sign", delegate: true},
		"gcd":     {kind: coreFunc, builtin: "gcd", delegate: true},
		"random":  {kind: coreFunc, builtin: "random", delegate: true},
		"sqrt":    {kind: coreFunc, builtin: "sqrt", sig: coreSig1(Float, Float)},
		"pow":     {kind: coreFunc, builtin: "pow", sig: coreSig2(Float, Float, Float)},
		"exp":     {kind: coreFunc, builtin: "exp", sig: coreSig1(Float, Float)},
		"ln":      {kind: coreFunc, builtin: "ln", sig: coreSig1(Float, Float)},
		"log10":   {kind: coreFunc, builtin: "log10", sig: coreSig1(Float, Float)},
		"log2":    {kind: coreFunc, builtin: "log2", sig: coreSig1(Float, Float)},
		"floor":   {kind: coreFunc, builtin: "floor", sig: coreSig1(Float, Int)},
		"ceil":    {kind: coreFunc, builtin: "ceil", sig: coreSig1(Float, Int)},
		"round":   {kind: coreFunc, builtin: "round", sig: coreSig1(Float, Int)},
		"trunc":   {kind: coreFunc, builtin: "trunc", sig: coreSig1(Float, Int)},
		"lcm":     {kind: coreFunc, builtin: "lcm", sig: coreSig2(Int, Int, Int)},
		"pi":      {kind: coreFunc, builtin: "pi", sig: coreSig0(Float)},
		"int_max": {kind: coreFunc, builtin: "int_max", sig: coreSig0(Int)},
		"int_min": {kind: coreFunc, builtin: "int_min", sig: coreSig0(Int)},
	},
	// fs (Unit 14): all members are fixed-signature flat builtins (none overloaded,
	// none arg-domain-checked, none special-cased in checkBuiltinCall), so all use
	// coreSig; member name == flat builtin name verbatim. The funcref-VALUE form
	// (map(xs, fs.is_file)) is out of scope: the value-position resolver rejects a
	// coreFunc as a value, so only the CALL form fs.is_file(x) is supported.
	"fs": {
		"file_exists":   {kind: coreFunc, builtin: "file_exists", sig: coreSig1(String, Bool)},
		"is_dir":        {kind: coreFunc, builtin: "is_dir", sig: coreSig1(String, Bool)},
		"is_file":       {kind: coreFunc, builtin: "is_file", sig: coreSig1(String, Bool)},
		"is_symlink":    {kind: coreFunc, builtin: "is_symlink", sig: coreSig1(String, Bool)},
		"chmod":         {kind: coreFunc, builtin: "chmod", sig: coreSig2(String, String, Void)},
		"symlink":       {kind: coreFunc, builtin: "symlink", sig: coreSig2(String, String, Void)},
		"symlink_force": {kind: coreFunc, builtin: "symlink_force", sig: coreSig2(String, String, Void)},
		"file_size":     {kind: coreFunc, builtin: "file_size", sig: coreSig1(String, Int)},
		"cwd":           {kind: coreFunc, builtin: "cwd", sig: coreSig0(String)},
		"program_path":  {kind: coreFunc, builtin: "program_path", sig: coreSig0(String)},
		"dir_name":      {kind: coreFunc, builtin: "dir_name", sig: coreSig1(String, String)},
		"base_name":     {kind: coreFunc, builtin: "base_name", sig: coreSig1(String, String)},
		"make_dir":      {kind: coreFunc, builtin: "make_dir", sig: coreSig1(String, Void)},
		"remove_file":   {kind: coreFunc, builtin: "remove_file", sig: coreSig1(String, Void)},
		"remove_dir":    {kind: coreFunc, builtin: "remove_dir", sig: coreSig1(String, Void)},
		"rename":        {kind: coreFunc, builtin: "rename", sig: coreSig2(String, String, Void)},
		"which":         {kind: coreFunc, builtin: "which", sig: coreSig1(String, optionalType(String))},
		"read_link":     {kind: coreFunc, builtin: "read_link", sig: coreSig1(String, optionalType(String))},
		"temp_file":     {kind: coreFunc, builtin: "temp_file", sig: coreSig0(String)},
		"temp_dir":      {kind: coreFunc, builtin: "temp_dir", sig: coreSig0(String)},
		"list_dir":      {kind: coreFunc, builtin: "list_dir", sig: coreSig1(String, arrayType(String))},
		"glob":          {kind: coreFunc, builtin: "glob", sig: coreSig1(String, arrayType(String))},
		"read_file":     {kind: coreFunc, builtin: "read_file", sig: coreSig1(String, String)},
		"write_file":    {kind: coreFunc, builtin: "write_file", sig: coreSig2(String, String, Void)},
		"append_file":   {kind: coreFunc, builtin: "append_file", sig: coreSig2(String, String, Void)},
		"change_dir":    {kind: coreFunc, builtin: "change_dir", sig: coreSig1(String, Void)},
	},
	// process (Unit 15): member name == flat builtin name verbatim. Every member
	// except pid_alive is special-cased in checkBuiltinCall (composite args like
	// string[]/{string:string}/string[][], special return types RunResult/Process,
	// or a Process-handle arg), so they DELEGATE to the flat dispatch; pid_alive is
	// a plain (int)->bool coreSig. RunResult and Process stay GLOBAL type names
	// (used bare), so no qualified type members are added.
	"process": {
		"run":            {kind: coreFunc, builtin: "run", delegate: true},
		"run_env":        {kind: coreFunc, builtin: "run_env", delegate: true},
		"run_env_status": {kind: coreFunc, builtin: "run_env_status", delegate: true},
		"run_env_full":   {kind: coreFunc, builtin: "run_env_full", delegate: true},
		"run_status":     {kind: coreFunc, builtin: "run_status", delegate: true},
		"run_full":       {kind: coreFunc, builtin: "run_full", delegate: true},
		"run_input":      {kind: coreFunc, builtin: "run_input", delegate: true},
		"run_input_full": {kind: coreFunc, builtin: "run_input_full", delegate: true},
		"exec_command":   {kind: coreFunc, builtin: "exec_command", delegate: true},
		"pipe":           {kind: coreFunc, builtin: "pipe", delegate: true},
		"spawn":          {kind: coreFunc, builtin: "spawn", delegate: true},
		"wait":           {kind: coreFunc, builtin: "wait", delegate: true},
		"is_done":        {kind: coreFunc, builtin: "is_done", delegate: true},
		"signal":         {kind: coreFunc, builtin: "signal", delegate: true},
		"wait_any":       {kind: coreFunc, builtin: "wait_any", delegate: true},
		"make_fifo":      {kind: coreFunc, builtin: "make_fifo", delegate: true},
		"pid_alive":      {kind: coreFunc, builtin: "pid_alive", sig: coreSig1(Int, Bool)},
	},
	// string (Batch B): the string builtin group. Most members are fixed-signature
	// (coreSig). Six DELEGATE to the flat dispatch: join (its arg-0 is string[], a
	// shape the fixed table cannot express); contains/index_of (string|array
	// OVERLOADED -- delegation runs the flat overload resolver, so the array form
	// also type-checks and lowers identically, documented and harmless); and
	// repeat/chr/format_float (each has a compile-time arg-domain check in
	// builtinIntArgDomains that checkBuiltinSig does NOT run -- delegating preserves
	// the flat-named domain diagnostic, exactly as math.gcd/random do). The one
	// rename: reverse_string -> string.reverse (the `_string` affix is redundant in
	// the string namespace; parallel to array.reverse, a distinct builtin key).
	// The funcref-VALUE form (map(xs, string.trim)) is out of scope, consistent
	// with fs/process: the value-position resolver rejects a coreFunc as a value.
	"string": {
		"lower":         {kind: coreFunc, builtin: "lower", sig: coreSig1(String, String)},
		"upper":         {kind: coreFunc, builtin: "upper", sig: coreSig1(String, String)},
		"trim":          {kind: coreFunc, builtin: "trim", sig: coreSig1(String, String)},
		"trim_start":    {kind: coreFunc, builtin: "trim_start", sig: coreSig1(String, String)},
		"trim_end":      {kind: coreFunc, builtin: "trim_end", sig: coreSig1(String, String)},
		"trim_prefix":   {kind: coreFunc, builtin: "trim_prefix", sig: coreSig2(String, String, String)},
		"trim_suffix":   {kind: coreFunc, builtin: "trim_suffix", sig: coreSig2(String, String, String)},
		"replace":       {kind: coreFunc, builtin: "replace", sig: coreSig3(String, String, String, String)},
		"replace_first": {kind: coreFunc, builtin: "replace_first", sig: coreSig3(String, String, String, String)},
		"split":         {kind: coreFunc, builtin: "split", sig: coreSig2(String, String, arrayType(String))},
		"join":          {kind: coreFunc, builtin: "join", delegate: true},
		"contains":      {kind: coreFunc, builtin: "contains", delegate: true},
		"starts_with":   {kind: coreFunc, builtin: "starts_with", sig: coreSig2(String, String, Bool)},
		"ends_with":     {kind: coreFunc, builtin: "ends_with", sig: coreSig2(String, String, Bool)},
		"index_of":      {kind: coreFunc, builtin: "index_of", delegate: true},
		"repeat":        {kind: coreFunc, builtin: "repeat", delegate: true},
		"substring":     {kind: coreFunc, builtin: "substring", sig: coreSig3(String, Int, Int, String)},
		"char_at":       {kind: coreFunc, builtin: "char_at", sig: coreSig2(String, Int, String)},
		"last_index_of": {kind: coreFunc, builtin: "last_index_of", sig: coreSig2(String, String, optionalType(Int))},
		"count":         {kind: coreFunc, builtin: "count", sig: coreSig2(String, String, Int)},
		"pad_start":     {kind: coreFunc, builtin: "pad_start", sig: coreSig3(String, Int, String, String)},
		"pad_end":       {kind: coreFunc, builtin: "pad_end", sig: coreSig3(String, Int, String, String)},
		"lines":         {kind: coreFunc, builtin: "lines", sig: coreSig1(String, arrayType(String))},
		"is_empty":      {kind: coreFunc, builtin: "is_empty", sig: coreSig1(String, Bool)},
		"reverse":       {kind: coreFunc, builtin: "reverse_string", sig: coreSig1(String, String)},
		"ord":           {kind: coreFunc, builtin: "ord", sig: coreSig1(String, Int)},
		"chr":           {kind: coreFunc, builtin: "chr", delegate: true},
		"int_or":        {kind: coreFunc, builtin: "int_or", sig: coreSig2(String, Int, Int)},
		"float_or":      {kind: coreFunc, builtin: "float_or", sig: coreSig2(String, Float, Float)},
		"format_float":  {kind: coreFunc, builtin: "format_float", delegate: true},
	},
	// dict (Batch B): the dict builtin group. Every member is generic over the
	// dict's K/V and is special-cased in checkBuiltinNamed (the fixed table cannot
	// express the inference), so ALL nine DELEGATE -- the flat generic-inference
	// handlers run unchanged and codegen is byte-identical. dict.get delegates to
	// the flat "get" (dict get, checkGetCall), a DISTINCT builtin key from json's
	// json_get. Member name == builtin key. Funcref-VALUE form out of scope (as fs).
	"dict": {
		"has":    {kind: coreFunc, builtin: "has", delegate: true},
		"keys":   {kind: coreFunc, builtin: "keys", delegate: true},
		"get":    {kind: coreFunc, builtin: "get", delegate: true},
		"get_or": {kind: coreFunc, builtin: "get_or", delegate: true},
		"remove": {kind: coreFunc, builtin: "remove", delegate: true},
		"merge":  {kind: coreFunc, builtin: "merge", delegate: true},
		"values": {kind: coreFunc, builtin: "values", delegate: true},
		"size":   {kind: coreFunc, builtin: "size", delegate: true},
		"clear":  {kind: coreFunc, builtin: "clear", delegate: true},
	},
	// array (Batch B): the array builtin group, including the funcref-heavy
	// higher-order builtins. Every member is special-cased in checkBuiltinNamed
	// (generic element inference, funcref-arg inference, composite results, or
	// arg-domain-checked indices), so ALL 28 DELEGATE. A funcref ARGUMENT passed
	// into a namespaced call (array.map(xs, f)) infers exactly as the flat map via
	// delegation and lowers byte-identically. The funcref-VALUE form
	// (map(xs, array.reverse)) is out of scope, consistent with fs/process: the
	// value-position resolver rejects a coreFunc as a value. insert_at/remove_at
	// keep their flat-named compile-time index domain diagnostics (delegation runs
	// checkBuiltinArgDomains first). contains/index_of mirror string's identical
	// entries (core_members.go:212,215) to make array membership/search reachable
	// under array., not just string. -- same builtin keys, byte-identical codegen.
	// Member name == builtin key; contains/index_of additionally share their
	// builtin keys with string's identical entries (core_members.go:212,215).
	"array": {
		"contains":    {kind: coreFunc, builtin: "contains", delegate: true},
		"index_of":    {kind: coreFunc, builtin: "index_of", delegate: true},
		"push":        {kind: coreFunc, builtin: "push", delegate: true},
		"pop":         {kind: coreFunc, builtin: "pop", delegate: true},
		"map":         {kind: coreFunc, builtin: "map", delegate: true},
		"filter":      {kind: coreFunc, builtin: "filter", delegate: true},
		"each":        {kind: coreFunc, builtin: "each", delegate: true},
		"zip":         {kind: coreFunc, builtin: "zip", delegate: true},
		"reverse":     {kind: coreFunc, builtin: "reverse", delegate: true},
		"reduce":      {kind: coreFunc, builtin: "reduce", delegate: true},
		"sort":        {kind: coreFunc, builtin: "sort", delegate: true},
		"sort_by":     {kind: coreFunc, builtin: "sort_by", delegate: true},
		"find":        {kind: coreFunc, builtin: "find", delegate: true},
		"any":         {kind: coreFunc, builtin: "any", delegate: true},
		"all":         {kind: coreFunc, builtin: "all", delegate: true},
		"slice":       {kind: coreFunc, builtin: "slice", delegate: true},
		"concat":      {kind: coreFunc, builtin: "concat", delegate: true},
		"sum":         {kind: coreFunc, builtin: "sum", delegate: true},
		"range":       {kind: coreFunc, builtin: "range", delegate: true},
		"first":       {kind: coreFunc, builtin: "first", delegate: true},
		"last":        {kind: coreFunc, builtin: "last", delegate: true},
		"count_where": {kind: coreFunc, builtin: "count_where", delegate: true},
		"flatten":     {kind: coreFunc, builtin: "flatten", delegate: true},
		"unique":      {kind: coreFunc, builtin: "unique", delegate: true},
		"take":        {kind: coreFunc, builtin: "take", delegate: true},
		"drop":        {kind: coreFunc, builtin: "drop", delegate: true},
		"insert_at":   {kind: coreFunc, builtin: "insert_at", delegate: true},
		"remove_at":   {kind: coreFunc, builtin: "remove_at", delegate: true},
	},
}

// Custom member checkers are wired here rather than in the coreCatalog literal:
// a method value in the package-level var initializer would form a static
// initialization cycle (the method's call graph reaches coreCatalog).
func init() {
	m := coreCatalog["json"]["decode"]
	m.check = (*checker).checkJSONDecode
	coreCatalog["json"]["decode"] = m
}

// coreSig0/1/2 build fixed builtin signatures for core members.
func coreSig0(result Type) builtinSig { return builtinSig{result: result} }
func coreSig1(p0, result Type) builtinSig {
	return builtinSig{params: []builtinParam{{name: "a", types: []Type{p0}}}, result: result}
}
func coreSig2(p0, p1, result Type) builtinSig {
	return builtinSig{params: []builtinParam{
		{name: "a", types: []Type{p0}},
		{name: "b", types: []Type{p1}},
	}, result: result}
}
func coreSig3(p0, p1, p2, result Type) builtinSig {
	return builtinSig{params: []builtinParam{
		{name: "a", types: []Type{p0}},
		{name: "b", types: []Type{p1}},
		{name: "c", types: []Type{p2}},
	}, result: result}
}

// isCoreModule reports whether name is a registered core-module namespace
// (a coreCatalog key), independent of whether it is currently imported.
func isCoreModule(name string) bool {
	_, ok := coreCatalog[name]
	return ok
}

// coreMemberNames returns the sorted member names of a namespace (for
// did-you-mean suggestions).
func coreMemberNames(ns string) []string {
	members := coreCatalog[ns]
	out := make([]string, 0, len(members))
	for name := range members {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// crossNamespaceSuffix returns `; did you mean "otherNs.member"?` when member
// exists verbatim (exact match, not fuzzy -- a "right name, wrong namespace"
// hint, not a typo hint) on EXACTLY ONE other core namespace, matching on the
// member NAME (the coreCatalog map key), not the coreMember.builtin field --
// string.reverse and array.reverse share the member name "reverse" but route
// to different builtin keys (reverse_string vs reverse), so a builtin-key
// match would misclassify this case. Returns "" when zero or 2+ other
// namespaces have the member (ambiguous, matching suggestSuffix's own
// tie-breaks-to-nothing rule). Skips any coreCatalog key starting with "__"
// (see the coreCatalog doc comment above for why). Collects every matching
// namespace before deciding -- Go map iteration order is randomized, so an
// early-return implementation would be non-deterministic whenever 2+
// namespaces share the member.
func crossNamespaceSuffix(ns, member string) string {
	var matches []string
	for otherNs, members := range coreCatalog {
		if otherNs == ns || strings.HasPrefix(otherNs, "__") {
			continue
		}
		if _, ok := members[member]; ok {
			matches = append(matches, otherNs)
		}
	}
	if len(matches) != 1 {
		return ""
	}
	return `; did you mean ` + strconv.Quote(matches[0]+"."+member) + `?`
}

// checkCoreCall resolves `ns.member(args)` for a reserved core module. It is
// keyed purely on (namespace, member) table lookups; nothing here is
// json-specific. A function member records a CallBuiltin CallInfo (via its custom
// checker or the shared generic sig path) so codegen lowers it exactly like a
// flat builtin.
func (c *checker) checkCoreCall(n *ast.CallExpr, ns, member string) Type {
	members, ok := coreCatalog[ns]
	if !ok {
		// Unreachable: a core namespace alias exists only because the loader
		// created a synthetic module for a catalog-backed name.
		c.errf(n.CalleePos, "internal: unknown core module %q", ns)
		c.typeArgs(n.Args)
		return Invalid
	}
	m, ok := members[member]
	if !ok {
		sugg := suggestSuffix(member, coreMemberNames(ns))
		if sugg == "" {
			sugg = crossNamespaceSuffix(ns, member)
		}
		c.errf(n.CalleePos, "module %q has no member %q%s", ns, member, sugg)
		c.typeArgs(n.Args)
		return Invalid
	}
	switch m.kind {
	case coreType:
		c.errf(n.CalleePos, "%q is a type of module %q, not callable", member, ns)
		c.typeArgs(n.Args)
		return Invalid
	case coreConst:
		c.errf(n.CalleePos, "%q is a constant of module %q, not callable", member, ns)
		c.typeArgs(n.Args)
		return Invalid
	}
	// coreFunc. The pre-dispatch type-arg guard (checkCall) is skipped for a
	// qualified callee, so this branch owns type-arg validation, mirroring
	// checkUserCallIn's not-generic rejection.
	if len(n.TypeArgs) > 0 && !m.takesTypeArgs {
		c.errf(n.TypeArgs[0].Pos, "%s.%s does not take type arguments", ns, member)
		for _, ta := range n.TypeArgs {
			c.resolveType(ta.Name, ta.Pos) // still surface unknown-type diagnostics
		}
	}
	if m.check != nil {
		return m.check(c, n)
	}
	// A delegating member reuses the full flat-builtin dispatch keyed on its
	// builtin name: overloads, composite-arg validation, special return types, and
	// compile-time arg-domain checks all run exactly as for the flat spelling, so
	// codegen lowers byte-identically. Diagnostics name ns.member via dispName.
	if m.delegate {
		return c.checkBuiltinNamed(n, m.builtin, ns+"."+member)
	}
	return c.checkBuiltinSig(n, ns+"."+member, m.builtin, m.sig)
}

// isJSONDecodeTarget reports whether t is a type json.decode[T] can project into
// (spec 4.4). Recursive projection into structs/arrays is out of scope for Unit 5.
func isJSONDecodeTarget(t Type) bool {
	return t == jsonValueType || t == String || t == Int || t == Float || t == Bool
}

// checkJSONDecode resolves json.decode[T](s). T comes from the explicit call-site
// type argument (Unit 4); when omitted it defaults to json.Value. The result type
// is T, which codegen reads from CallInfo.Result to pick the boxing vs scalar
// lowering. T is compared as a RESOLVED Type, so a transparent alias of a
// supported type (e.g. `type MyInt = int`) works.
func (c *checker) checkJSONDecode(n *ast.CallExpr) Type {
	target := jsonValueType
	if len(n.TypeArgs) >= 1 {
		if len(n.TypeArgs) > 1 {
			c.errf(n.TypeArgs[1].Pos, "json.decode takes at most one type argument")
		}
		target = c.resolveType(n.TypeArgs[0].Name, n.TypeArgs[0].Pos)
		if target == Invalid {
			c.typeArgs(n.Args)
			return Invalid
		}
		if !isJSONDecodeTarget(target) {
			c.errf(n.TypeArgs[0].Pos, "json.decode does not support target type %s (supported: json.Value, string, int, float, bool)", disp(target))
			c.typeArgs(n.Args)
			return target
		}
	}
	return c.checkBuiltinSig(n, "json.decode", "json_decode", coreSig1(String, target))
}

// coreTypeMember resolves a qualified type reference `ns.T` for a reserved core
// module: a coreType member returns its Type, anything else is an error. Returns
// (Invalid, false) if ns is not a core namespace in the catalog.
func coreTypeMember(ns, member string) (Type, bool) {
	members, ok := coreCatalog[ns]
	if !ok {
		return Invalid, false
	}
	m, ok := members[member]
	if !ok || m.kind != coreType {
		return Invalid, false
	}
	return m.typ, true
}
