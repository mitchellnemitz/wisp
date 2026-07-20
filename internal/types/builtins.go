package types

// Builtins are predefined functions whose names are reserved (spec section
// 9.5). They are special-cased by the checker because some accept union input
// types, which the user-facing type system has no way to express.
//
// stdout and stderr are reserved int constants (values 1 and 2) used only for
// print's `to` argument.

// builtinParam is one builtin parameter. Types lists the accepted argument
// types (a union; length 1 for a single type). HasDefault marks a trailing
// parameter with a default; for print's `to` the default is the reserved
// constant stdout.
type builtinParam struct {
	name       string
	types      []Type
	hasDefault bool
}

// builtinSig is a builtin's signature.
type builtinSig struct {
	params []builtinParam
	result Type
}

// builtinSigs is the catalog from spec section 9.5.
var builtinSigs = map[string]builtinSig{
	// print's `to` is the constant-argument case (rule 9): it is validated
	// separately to require exactly stdout or stderr, so its accepted type here
	// is int only and the default is stdout.
	"print": {
		params: []builtinParam{
			{name: "msg", types: []Type{String}},
			{name: "to", types: []Type{Int}, hasDefault: true},
		},
		result: Void,
	},
	"to_string": {
		params: []builtinParam{{name: "x", types: []Type{Int, Float, Bool, String}}},
		result: String,
	},
	"to_int": {
		// to_int(string) parses (integer-only); to_int(float) truncates toward zero.
		params: []builtinParam{{name: "x", types: []Type{String, Float}}},
		result: Int,
	},
	// error(msg) constructs an error handle with message = msg, code = 0 (M5).
	"error": {
		params: []builtinParam{{name: "msg", types: []Type{String}}},
		result: ErrorType,
	},
	// error_with(code, msg) constructs an error handle with both fields set (R4).
	"error_with": {
		params: []builtinParam{
			{name: "code", types: []Type{Int}},
			{name: "msg", types: []Type{String}},
		},
		result: ErrorType,
	},
	// wrap(err, msg) returns a new error handle with message=msg, code=0, and err
	// as its cause. The cause is reached via cause(e), not a field (spec).
	"wrap": {
		params: []builtinParam{
			{name: "err", types: []Type{ErrorType}},
			{name: "msg", types: []Type{String}},
		},
		result: ErrorType,
	},
	// cause(err) returns Some(inner) if err was produced by wrap, else None.
	// The result is Optional[error]; never aborts.
	"cause": {
		params: []builtinParam{
			{name: "err", types: []Type{ErrorType}},
		},
		result: optionalType(ErrorType),
	},
	"to_float": {
		// to_float(int) and to_float(string) per spec 3.4; no to_float(bool).
		params: []builtinParam{{name: "x", types: []Type{Int, String}}},
		result: Float,
	},
	"to_bool": {
		params: []builtinParam{{name: "x", types: []Type{Int, Float, String}}},
		result: Bool,
	},
	"length": {
		// length(string) -> bytes. length(T[]) -> element count is handled
		// specially in checkBuiltinCall (the fixed table cannot express the array
		// case); this entry covers the string form.
		params: []builtinParam{{name: "s", types: []Type{String}}},
		result: Int,
	},
	// push(xs, v) grows array xs in place; validated specially in
	// checkBuiltinCall (array element typing). This entry only reserves the name.
	"push": {
		params: []builtinParam{
			{name: "xs", types: []Type{}},
			{name: "v", types: []Type{}},
		},
		result: Void,
	},
	// has(d, k) -> bool tests dict membership; keys(d) -> K[] returns the keys in
	// insertion order. Both are dict-aware and validated specially in
	// checkBuiltinCall; these entries only reserve the names (spec 4.4).
	"has": {
		params: []builtinParam{
			{name: "d", types: []Type{}},
			{name: "k", types: []Type{}},
		},
		result: Bool,
	},
	"keys": {
		params: []builtinParam{
			{name: "d", types: []Type{}},
		},
		result: Invalid,
	},
	"lower": {
		params: []builtinParam{{name: "s", types: []Type{String}}},
		result: String,
	},
	"upper": {
		params: []builtinParam{{name: "s", types: []Type{String}}},
		result: String,
	},
	"trim": {
		params: []builtinParam{{name: "s", types: []Type{String}}},
		result: String,
	},
	"replace": {
		params: []builtinParam{
			{name: "s", types: []Type{String}},
			{name: "search", types: []Type{String}},
			{name: "replacement", types: []Type{String}},
		},
		result: String,
	},
	// map/filter/each are generic higher-order builtins (M4) special-cased in
	// checkBuiltinCall: their element/return types are inferred from the array and
	// the function reference, which the fixed table cannot express. These entries
	// only RESERVE the names (so `fn map(...)` is a compile error and referencing
	// `map` as a value errors); the params/result here are never used by the
	// generic path.
	"map": {
		params: []builtinParam{
			{name: "xs", types: []Type{}},
			{name: "f", types: []Type{}},
		},
		result: Invalid,
	},
	"filter": {
		params: []builtinParam{
			{name: "xs", types: []Type{}},
			{name: "f", types: []Type{}},
		},
		result: Invalid,
	},
	"each": {
		params: []builtinParam{
			{name: "xs", types: []Type{}},
			{name: "f", types: []Type{}},
		},
		result: Void,
	},
	// zip(a: T[], b: U[]) -> (T,U)[]: pair two arrays element-wise up to the
	// shorter length. Generic in T and U; special-cased in checkBuiltinCall.
	// This entry only RESERVES the name (so `fn zip(...)` and `let zip = ...` are
	// compile errors) and feeds types.BuiltinNames() for the drift-guard.
	"zip": {
		params: []builtinParam{
			{name: "a", types: []Type{}},
			{name: "b", types: []Type{}},
		},
		result: Invalid,
	},

	// parse_args(args: string[], value_flags: string[]) ->
	// ({string:string}, string[], string[]): classify CLI args into values,
	// switches, and positionals. The bare 3-tuple result is hand-built in
	// checkParseArgsCall (the fixed table cannot express a tuple of composites),
	// so this entry only RESERVES the name and feeds the drift-guard.
	"parse_args": {
		params: []builtinParam{
			{name: "args", types: []Type{}},
			{name: "value_flags", types: []Type{}},
		},
		result: Invalid,
	},

	// and_then/or_else/map_err are monadic chaining combinators (combinators milestone)
	// special-cased in checkBuiltinCall. These entries only RESERVE the names.
	"and_then": {
		params: []builtinParam{
			{name: "m", types: []Type{}},
			{name: "f", types: []Type{}},
		},
		result: Invalid,
	},
	"or_else": {
		params: []builtinParam{
			{name: "m", types: []Type{}},
			{name: "f", types: []Type{}},
		},
		result: Invalid,
	},
	"map_err": {
		params: []builtinParam{
			{name: "r", types: []Type{}},
			{name: "f", types: []Type{}},
		},
		result: Invalid,
	},

	// --- M6 core stdlib ---
	//
	// All of these are validated by a dedicated handler in checkBuiltinCall; the
	// entries below only RESERVE the names (so `fn split(...)` and `let abs = ...`
	// are compile errors). The params/result here are illustrative and unused by
	// the special-cased paths, EXCEPT where a simple fixed signature is sufficient.

	"split": { // split(string, string) -> string[]
		params: []builtinParam{
			{name: "s", types: []Type{String}},
			{name: "sep", types: []Type{String}},
		},
		result: arrayType(String),
	},
	"join": { // join(string[], string) -> string; arg 1 typed specially
		params: []builtinParam{
			{name: "parts", types: []Type{}},
			{name: "sep", types: []Type{String}},
		},
		result: String,
	},
	// contains is OVERLOADED (string,string->bool OR T[],T->bool); dispatched by
	// arg-1 type in checkContainsCall. This entry only reserves the name.
	"contains": {
		params: []builtinParam{
			{name: "a", types: []Type{}},
			{name: "b", types: []Type{}},
		},
		result: Bool,
	},
	"starts_with": {
		params: []builtinParam{
			{name: "s", types: []Type{String}},
			{name: "pre", types: []Type{String}},
		},
		result: Bool,
	},
	"ends_with": {
		params: []builtinParam{
			{name: "s", types: []Type{String}},
			{name: "suf", types: []Type{String}},
		},
		result: Bool,
	},
	// index_of is OVERLOADED (string,string->Optional[int] OR T[],T->Optional[int]);
	// dispatched by the first argument's type in checkIndexOfCall. This entry only
	// reserves the name.
	"index_of": {
		params: []builtinParam{
			{name: "a", types: []Type{}},
			{name: "b", types: []Type{}},
		},
		result: optionalType(Int),
	},
	"repeat": {
		params: []builtinParam{
			{name: "s", types: []Type{String}},
			{name: "n", types: []Type{Int}},
		},
		result: String,
	},
	// abs/min/max are OVERLOADED int/float; validated specially. These entries
	// only reserve the names.
	"abs": {
		params: []builtinParam{{name: "x", types: []Type{Int, Float}}},
		result: Invalid,
	},
	"min": {
		params: []builtinParam{
			{name: "a", types: []Type{}},
			{name: "b", types: []Type{}},
		},
		result: Invalid,
	},
	"max": {
		params: []builtinParam{
			{name: "a", types: []Type{}},
			{name: "b", types: []Type{}},
		},
		result: Invalid,
	},
	// reverse(T[]) -> T[] and reduce(T[], U, fn(U,T)->U) -> U are generic;
	// validated specially. These entries only reserve the names.
	"reverse": {
		params: []builtinParam{{name: "xs", types: []Type{}}},
		result: Invalid,
	},
	"reduce": {
		params: []builtinParam{
			{name: "xs", types: []Type{}},
			{name: "init", types: []Type{}},
			{name: "f", types: []Type{}},
		},
		result: Invalid,
	},

	// --- M7 I/O ---
	//
	// Fixed-signature builtins use the generic checkBuiltinCall path directly.
	// run's string[] argument cannot be expressed by the fixed table (like join's
	// arg-1), so it is validated specially in checkRunCall; this entry only
	// reserves the name and feeds types.BuiltinNames() for the drift guard.
	"env": { // env(string) -> string
		params: []builtinParam{{name: "name", types: []Type{String}}},
		result: String,
	},
	"has_env": { // has_env(string) -> bool
		params: []builtinParam{{name: "name", types: []Type{String}}},
		result: Bool,
	},
	"set_env": { // set_env(string, string) -> void
		params: []builtinParam{
			{name: "name", types: []Type{String}},
			{name: "value", types: []Type{String}},
		},
		result: Void,
	},
	"unset_env": { // unset_env(string) -> void
		params: []builtinParam{
			{name: "name", types: []Type{String}},
		},
		result: Void,
	},
	"read_file": { // read_file(string) -> string
		params: []builtinParam{{name: "path", types: []Type{String}}},
		result: String,
	},
	"write_file": { // write_file(string, string) -> void
		params: []builtinParam{
			{name: "path", types: []Type{String}},
			{name: "content", types: []Type{String}},
		},
		result: Void,
	},
	"append_file": { // append_file(string, string) -> void
		params: []builtinParam{
			{name: "path", types: []Type{String}},
			{name: "content", types: []Type{String}},
		},
		result: Void,
	},
	"run": { // run(string[]) -> string; arg 1 typed specially in checkRunCall
		params: []builtinParam{{name: "argv", types: []Type{}}},
		result: String,
	},
	// run_env(string[], {string:string}) -> string: run with an augmented child
	// environment, return stdout, abort located on nonzero (mirrors run). Both args
	// are composite shapes the fixed table cannot express, so they are validated
	// specially in checkRunEnvCall; this entry only reserves the name.
	"run_env": {
		params: []builtinParam{{name: "argv", types: []Type{}}, {name: "env", types: []Type{}}},
		result: String,
	},
	// run_env_status(string[], {string:string}) -> int: mirrors run_status; no abort on
	// nonzero exit, returns the child's exit code. Validated specially in
	// checkRunEnvStatusCall; this entry only reserves the name.
	"run_env_status": {
		params: []builtinParam{{name: "argv", types: []Type{}}, {name: "env", types: []Type{}}},
		result: Int,
	},
	// run_env_full(string[], {string:string}) -> RunResult: mirrors run_full; no abort on
	// nonzero exit, returns RunResult{stdout,stderr,code}. Validated specially in
	// checkRunEnvFullCall; this entry only reserves the name.
	"run_env_full": {
		params: []builtinParam{{name: "argv", types: []Type{}}, {name: "env", types: []Type{}}},
		result: RunResult,
	},
	// exec_command(string[]) -> void: replace the shell process with argv via the
	// POSIX exec builtin (process-replacement handoff). The argv arg is a string[]
	// the fixed table cannot express, so it is validated specially in
	// checkExecCommandCall; this entry only reserves the name. Void result.
	"exec_command": {
		params: []builtinParam{{name: "argv", types: []Type{}}},
		result: Void,
	},
	"exit": { // exit(int) -> void
		params: []builtinParam{{name: "code", types: []Type{Int}}},
		result: Void,
	},

	// --- Filesystem + process (fs milestone) ---
	//
	// Fixed-signature builtins use the generic checkBuiltinCall path. The four
	// mutating ops are void and fallible (located abort via __wisp_fail);
	// file_exists/is_dir/cwd/env_or are total (never abort); which (-> Optional[string])
	// and list_dir (-> string[]) are also fixed-signature here. Only run_status takes
	// an array argument the fixed table cannot express, so it is reserved here and
	// validated specially in call.go (checkRunStatusCall, mirroring run).
	"file_exists": {params: []builtinParam{{name: "path", types: []Type{String}}}, result: Bool},
	"is_dir":      {params: []builtinParam{{name: "path", types: []Type{String}}}, result: Bool},
	// chmod(path: string, mode: string) -> Void: change file permissions via
	// chmod -- mode path (SINGLE --; operand transposition: mode-then-path).
	// Fallible: located abort on failure (missing path, bad mode). Statement-only.
	"chmod": {params: []builtinParam{
		{name: "path", types: []Type{String}},
		{name: "mode", types: []Type{String}},
	}, result: Void},
	// symlink(target: string, link_path: string) -> Void: create a symlink via
	// ln -s -- target link_path (SINGLE --; target stored verbatim). Fallible:
	// located abort on failure (link_path exists). Statement-only.
	"symlink": {params: []builtinParam{
		{name: "target", types: []Type{String}},
		{name: "link_path", types: []Type{String}},
	}, result: Void},
	// symlink_force(target: string, link_path: string) -> Void: create or replace
	// a symlink via `rm -f -- link_path && ln -s -- target link_path` (removes any
	// existing non-directory at link_path first, then creates the new symlink).
	// NOT ln -sf/temp+mv: those botch the symlink-to-dir case non-portably. Refuses
	// a real directory (rm -f fails -> located abort). Fallible. Statement-only.
	"symlink_force": {params: []builtinParam{
		{name: "target", types: []Type{String}},
		{name: "link_path", types: []Type{String}},
	}, result: Void},
	// is_file(path: string) -> bool: true iff path is a regular file ([ -f ];
	// follows symlinks). Total (never aborts).
	"is_file": {params: []builtinParam{{name: "path", types: []Type{String}}}, result: Bool},
	// pid_alive(pid: int) -> bool: total liveness probe; true iff `kill -0 pid`
	// succeeds (the process/group exists AND is signalable by the caller). A
	// nonexistent PID (ESRCH) and a permission failure (EPERM) both -> false. pid
	// 0 / negative target process groups (POSIX kill). Never aborts.
	"pid_alive": {params: []builtinParam{{name: "pid", types: []Type{Int}}}, result: Bool},
	// is_symlink(path: string) -> bool: true iff path itself is a symlink ([ -L ]).
	// Total.
	"is_symlink": {params: []builtinParam{{name: "path", types: []Type{String}}}, result: Bool},
	// file_size(path: string) -> int: byte count via wc -c < path. Fallible:
	// located abort if path is missing, unreadable, or a directory. BSD wc -c
	// emits leading whitespace; stripped via parameter expansion before the int
	// result is returned.
	"file_size": {params: []builtinParam{{name: "path", types: []Type{String}}}, result: Int},
	"cwd":       {params: []builtinParam{}, result: String},
	// program_path() -> string: the program's invocation path (shell $0), captured
	// once at top level. dir_name/base_name are pure POSIX-style path string
	// functions (no external process). All fixed-signature (generic path).
	"program_path": {params: []builtinParam{}, result: String},
	"dir_name":     {params: []builtinParam{{name: "path", types: []Type{String}}}, result: String},
	"base_name":    {params: []builtinParam{{name: "path", types: []Type{String}}}, result: String},
	"env_or": {params: []builtinParam{
		{name: "name", types: []Type{String}},
		{name: "fallback", types: []Type{String}},
	}, result: String},
	"make_dir":    {params: []builtinParam{{name: "path", types: []Type{String}}}, result: Void},
	"remove_file": {params: []builtinParam{{name: "path", types: []Type{String}}}, result: Void},
	"remove_dir":  {params: []builtinParam{{name: "path", types: []Type{String}}}, result: Void},
	"rename": {params: []builtinParam{
		{name: "from", types: []Type{String}},
		{name: "to", types: []Type{String}},
	}, result: Void},
	// which(name) -> Optional[string]: Some(path) on command -v exit 0, None
	// otherwise. Never aborts. Lowered via a string/exit-status sentinel-to-Optional.
	"which": {params: []builtinParam{{name: "name", types: []Type{String}}}, result: optionalType(String)},
	// read_link(path: string) -> Optional[string]: the immediate symlink target via
	// readlink -- path. Some(target) if path is a symlink; None on nonzero exit
	// (non-symlink, missing, or permission error). Never aborts. Same
	// exit-status->Optional lowering as which.
	"read_link": {params: []builtinParam{{name: "path", types: []Type{String}}}, result: optionalType(String)},
	// read_secret(prompt: string) -> Optional[string]: no-echo line reader via
	// stty -echo; Some(line) on success, None on EOF/error. Never aborts.
	"read_secret": {params: []builtinParam{{name: "prompt", types: []Type{String}}}, result: optionalType(String)},
	// temp_file() -> string: create a temp file via mktemp, return its path.
	// Fallible: located abort if mktemp fails. Caller owns cleanup.
	"temp_file": {params: []builtinParam{}, result: String},
	// temp_dir() -> string: create a temp directory via mktemp -d, return its path.
	// Fallible: located abort if mktemp fails. Caller owns cleanup.
	"temp_dir": {params: []builtinParam{}, result: String},
	// list_dir(path) -> string[]: base names (incl hidden, excl . and ..),
	// unspecified order; missing/non-dir aborts located. The RESULT is string[] (a
	// fixed signature expresses this directly; no special handler), unlike run/join
	// whose ARGUMENT is string[].
	"list_dir": {params: []builtinParam{{name: "path", types: []Type{String}}}, result: arrayType(String)},
	// glob(pattern: string) -> string[]: shell pathname expansion of pattern; the
	// matching paths in shell-expansion order, empty array on no match. Total
	// (no-match is success, not an error). The matched names are inert data
	// (eval-deferred store), like list_dir. Fixed string[] result, no special handler.
	"glob": {params: []builtinParam{{name: "pattern", types: []Type{String}}}, result: arrayType(String)},
	// run_status(argv) -> int: argv's string[] type cannot be expressed by the fixed
	// table, so it is validated specially in checkRunStatusCall (like run); this
	// entry only reserves the name and feeds types.BuiltinNames().
	"run_status": {params: []builtinParam{{name: "argv", types: []Type{}}}, result: Int},

	// --- Collections tail (11 new builtins) ---
	//
	// All have array/dict/funcref-aware signatures the fixed table cannot express;
	// these entries only RESERVE the names and feed types.BuiltinNames().
	"count_where": {params: []builtinParam{{name: "xs", types: []Type{}}, {name: "f", types: []Type{}}}, result: Invalid},
	"flatten":     {params: []builtinParam{{name: "xs", types: []Type{}}}, result: Invalid},
	"unique":      {params: []builtinParam{{name: "xs", types: []Type{}}}, result: Invalid},
	"take":        {params: []builtinParam{{name: "xs", types: []Type{}}, {name: "n", types: []Type{Int}}}, result: Invalid},
	"drop":        {params: []builtinParam{{name: "xs", types: []Type{}}, {name: "n", types: []Type{Int}}}, result: Invalid},
	"pop":         {params: []builtinParam{{name: "xs", types: []Type{}}}, result: Invalid},
	"remove_at":   {params: []builtinParam{{name: "xs", types: []Type{}}, {name: "i", types: []Type{Int}}}, result: Void},
	"insert_at":   {params: []builtinParam{{name: "xs", types: []Type{}}, {name: "i", types: []Type{Int}}, {name: "v", types: []Type{}}}, result: Void},
	"size":        {params: []builtinParam{{name: "d", types: []Type{}}}, result: Invalid},
	"clear":       {params: []builtinParam{{name: "d", types: []Type{}}}, result: Void},

	// --- Assertions + skip (test framework) ---
	//
	// NOT file-gated: callable from any wisp code (the test runner that interprets
	// the reserved exit codes is a later task). assert/skip have fixed signatures;
	// the rest are special-cased in call.go (assert_eq/assert_ne over [T: comparable];
	// assert_some/assert_none over Optional[T]; assert_ok/assert_err over Result[T];
	// assert_contains overloaded on arg-0 type like contains). All return void; on
	// the expected case each is a no-op, on failure each EXITS the (sub)shell with a
	// reserved code (122 assert-fail / 121 skip). Entries below only RESERVE the
	// names and feed types.BuiltinNames(); the special-cased ones use illustrative
	// params.
	"assert": {
		params: []builtinParam{
			{name: "cond", types: []Type{Bool}},
			{name: "msg", types: []Type{String}, hasDefault: true},
		},
		result: Void,
	},
	"assert_eq":       {params: []builtinParam{{name: "got", types: []Type{}}, {name: "want", types: []Type{}}}, result: Void},
	"assert_ne":       {params: []builtinParam{{name: "got", types: []Type{}}, {name: "want", types: []Type{}}}, result: Void},
	"assert_some":     {params: []builtinParam{{name: "o", types: []Type{}}}, result: Void},
	"assert_none":     {params: []builtinParam{{name: "o", types: []Type{}}}, result: Void},
	"assert_ok":       {params: []builtinParam{{name: "r", types: []Type{}}}, result: Void},
	"assert_err":      {params: []builtinParam{{name: "r", types: []Type{}}}, result: Void},
	"assert_contains": {params: []builtinParam{{name: "a", types: []Type{}}, {name: "b", types: []Type{}}}, result: Void},
	"skip":            {params: []builtinParam{{name: "reason", types: []Type{String}}}, result: Void},
	// test_tmpdir() -> string (R11): the per-test temporary directory the runner
	// creates before each test and removes after. Zero args, returns string. NOT
	// file-gated (callable anywhere); outside a test runner __wisp_ttmp is empty.
	"test_tmpdir": {params: []builtinParam{}, result: String},

	// --- Collections core ---
	//
	// All of these have array/dict/funcref-aware signatures the fixed table cannot
	// express, so each is validated by a dedicated handler in checkBuiltinCall;
	// these entries only RESERVE the names (so `fn sort(...)` is a compile error and
	// referencing them as values errors) and feed types.BuiltinNames().
	"sort":    {params: []builtinParam{{name: "xs", types: []Type{}}}, result: Invalid},
	"sort_by": {params: []builtinParam{{name: "xs", types: []Type{}}, {name: "less", types: []Type{}}}, result: Invalid},
	"find":    {params: []builtinParam{{name: "xs", types: []Type{}}, {name: "f", types: []Type{}}}, result: optionalType(Int)},
	"any":     {params: []builtinParam{{name: "xs", types: []Type{}}, {name: "f", types: []Type{}}}, result: Bool},
	"all":     {params: []builtinParam{{name: "xs", types: []Type{}}, {name: "f", types: []Type{}}}, result: Bool},
	"slice":   {params: []builtinParam{{name: "xs", types: []Type{}}, {name: "start", types: []Type{Int}}, {name: "end", types: []Type{Int}}}, result: Invalid},
	"concat":  {params: []builtinParam{{name: "a", types: []Type{}}, {name: "b", types: []Type{}}}, result: Invalid},
	"sum":     {params: []builtinParam{{name: "xs", types: []Type{}}}, result: Invalid},
	"range":   {params: []builtinParam{{name: "n", types: []Type{Int}}}, result: arrayType(Int)},
	"first":   {params: []builtinParam{{name: "xs", types: []Type{}}}, result: Invalid},
	"last":    {params: []builtinParam{{name: "xs", types: []Type{}}}, result: Invalid},
	"values":  {params: []builtinParam{{name: "d", types: []Type{}}}, result: Invalid},
	"get_or":  {params: []builtinParam{{name: "d", types: []Type{}}, {name: "k", types: []Type{}}, {name: "fallback", types: []Type{}}}, result: Invalid},
	"remove":  {params: []builtinParam{{name: "d", types: []Type{}}, {name: "k", types: []Type{}}}, result: Void},
	"merge":   {params: []builtinParam{{name: "a", types: []Type{}}, {name: "b", types: []Type{}}}, result: Invalid},

	// --- String round-out ---
	//
	// All fixed-signature, byte-based; the generic checkBuiltinCall path types them.
	"substring":      {params: []builtinParam{{name: "s", types: []Type{String}}, {name: "start", types: []Type{Int}}, {name: "end", types: []Type{Int}}}, result: String},
	"char_at":        {params: []builtinParam{{name: "s", types: []Type{String}}, {name: "i", types: []Type{Int}}}, result: String},
	"last_index_of":  {params: []builtinParam{{name: "s", types: []Type{String}}, {name: "sub", types: []Type{String}}}, result: optionalType(Int)},
	"count":          {params: []builtinParam{{name: "s", types: []Type{String}}, {name: "sub", types: []Type{String}}}, result: Int},
	"replace_first":  {params: []builtinParam{{name: "s", types: []Type{String}}, {name: "search", types: []Type{String}}, {name: "with", types: []Type{String}}}, result: String},
	"trim_start":     {params: []builtinParam{{name: "s", types: []Type{String}}}, result: String},
	"trim_end":       {params: []builtinParam{{name: "s", types: []Type{String}}}, result: String},
	"trim_prefix":    {params: []builtinParam{{name: "s", types: []Type{String}}, {name: "pre", types: []Type{String}}}, result: String},
	"trim_suffix":    {params: []builtinParam{{name: "s", types: []Type{String}}, {name: "suf", types: []Type{String}}}, result: String},
	"pad_start":      {params: []builtinParam{{name: "s", types: []Type{String}}, {name: "width", types: []Type{Int}}, {name: "fill", types: []Type{String}}}, result: String},
	"pad_end":        {params: []builtinParam{{name: "s", types: []Type{String}}, {name: "width", types: []Type{Int}}, {name: "fill", types: []Type{String}}}, result: String},
	"lines":          {params: []builtinParam{{name: "s", types: []Type{String}}}, result: arrayType(String)},
	"is_empty":       {params: []builtinParam{{name: "s", types: []Type{String}}}, result: Bool},
	"reverse_string": {params: []builtinParam{{name: "s", types: []Type{String}}}, result: String},
	"ord":            {params: []builtinParam{{name: "s", types: []Type{String}}}, result: Int},
	"chr":            {params: []builtinParam{{name: "code", types: []Type{Int}}}, result: String},

	// --- Numeric / math ---
	//
	// Fixed-signature (generic path). clamp/sign are overloaded int/float and
	// special-cased in call.go (entries reserve the names only).
	"int_or":   {params: []builtinParam{{name: "s", types: []Type{String}}, {name: "fallback", types: []Type{Int}}}, result: Int},
	"float_or": {params: []builtinParam{{name: "s", types: []Type{String}}, {name: "fallback", types: []Type{Float}}}, result: Float},
	"sqrt":     {params: []builtinParam{{name: "x", types: []Type{Float}}}, result: Float},
	"pow":      {params: []builtinParam{{name: "base", types: []Type{Float}}, {name: "exp", types: []Type{Float}}}, result: Float},
	// Transcendentals (pure-arithmetic, no awk libm). Float-only args (match
	// sqrt: no int->float coercion). exp/ln route through __wisp_exp/__wisp_ln;
	// log10/log2 = ln(x)/ln(c) with a baked constant; pi() lowers to the float
	// literal 3.141592653589793. No `e` constant (use exp(1.0)). Trig deferred.
	"exp":          {params: []builtinParam{{name: "x", types: []Type{Float}}}, result: Float},
	"ln":           {params: []builtinParam{{name: "x", types: []Type{Float}}}, result: Float},
	"log10":        {params: []builtinParam{{name: "x", types: []Type{Float}}}, result: Float},
	"log2":         {params: []builtinParam{{name: "x", types: []Type{Float}}}, result: Float},
	"pi":           {params: []builtinParam{}, result: Float},
	"floor":        {params: []builtinParam{{name: "x", types: []Type{Float}}}, result: Int},
	"format_float": {params: []builtinParam{{name: "x", types: []Type{Float}}, {name: "decimals", types: []Type{Int}}}, result: String},
	"ceil":         {params: []builtinParam{{name: "x", types: []Type{Float}}}, result: Int},
	"round":        {params: []builtinParam{{name: "x", types: []Type{Float}}}, result: Int},
	"trunc":        {params: []builtinParam{{name: "x", types: []Type{Float}}}, result: Int},
	"gcd":          {params: []builtinParam{{name: "a", types: []Type{Int}}, {name: "b", types: []Type{Int}}}, result: Int},
	"lcm":          {params: []builtinParam{{name: "a", types: []Type{Int}}, {name: "b", types: []Type{Int}}}, result: Int},
	"int_max":      {params: []builtinParam{}, result: Int},
	"int_min":      {params: []builtinParam{}, result: Int},
	"clamp":        {params: []builtinParam{{name: "x", types: []Type{}}, {name: "lo", types: []Type{}}, {name: "hi", types: []Type{}}}, result: Invalid},
	"sign":         {params: []builtinParam{{name: "x", types: []Type{}}}, result: Int},

	// --- Optional access (special-cased in call.go; entries reserve the names) ---
	"is_some":    {params: []builtinParam{{name: "o", types: []Type{}}}, result: Bool},
	"is_none":    {params: []builtinParam{{name: "o", types: []Type{}}}, result: Bool},
	"is_ok":      {params: []builtinParam{{name: "r", types: []Type{}}}, result: Bool},
	"is_err":     {params: []builtinParam{{name: "r", types: []Type{}}}, result: Bool},
	"unwrap":     {params: []builtinParam{{name: "o", types: []Type{}}}, result: Invalid},
	"unwrap_err": {params: []builtinParam{{name: "r", types: []Type{}}}, result: ErrorType},
	"unwrap_or":  {params: []builtinParam{{name: "o", types: []Type{}}, {name: "fallback", types: []Type{}}}, result: Invalid},
	"get":        {params: []builtinParam{{name: "d", types: []Type{}}, {name: "k", types: []Type{}}}, result: Invalid},

	// --- Regular expressions ---
	//
	// POSIX ERE, whole-match only, byte-based under LC_ALL=C. All four are
	// fixed-signature (every arg string) and fallible (a malformed pattern is a
	// located abort), backed by awk via ENVIRON. Capture groups/flags/regex_split
	// are deferred (spec sections 2, 3).
	"matches":        {params: []builtinParam{{name: "s", types: []Type{String}}, {name: "pattern", types: []Type{String}}}, result: Bool},
	"regex_find":     {params: []builtinParam{{name: "s", types: []Type{String}}, {name: "pattern", types: []Type{String}}}, result: optionalType(String)},
	"regex_find_all": {params: []builtinParam{{name: "s", types: []Type{String}}, {name: "pattern", types: []Type{String}}}, result: arrayType(String)},
	"regex_replace":  {params: []builtinParam{{name: "s", types: []Type{String}}, {name: "pattern", types: []Type{String}}, {name: "replacement", types: []Type{String}}}, result: String},
	// --- I/O tail (R3) ---
	"read_line":  {params: []builtinParam{}, result: optionalType(String)},
	"read_stdin": {params: []builtinParam{}, result: String},
	"change_dir": {params: []builtinParam{{name: "path", types: []Type{String}}}, result: Void},
	"run_full": { // run_full(string[]) -> RunResult; arg 1 typed specially in checkRunFullCall
		params: []builtinParam{{name: "argv", types: []Type{}}},
		result: RunResult,
	},
	"run_input": { // run_input(string[], string) -> string; args typed specially in checkRunInputCall
		params: []builtinParam{{name: "argv", types: []Type{}}, {name: "stdin", types: []Type{String}}},
		result: String,
	},
	"run_input_full": { // run_input_full(string[], string) -> RunResult; args typed specially in checkRunInputFullCall
		params: []builtinParam{{name: "argv", types: []Type{}}, {name: "stdin", types: []Type{String}}},
		result: RunResult,
	},
	// set_stdin(content: string) -> void: replace fd 0 with content's exact bytes
	// (test stdin injection, Gap 1). Fixed signature -> generic checkBuiltinCall
	// path; no special check function, no call.go dispatch case.
	"set_stdin": {
		params: []builtinParam{{name: "content", types: []Type{String}}},
		result: Void,
	},

	// on_exit(handler: fn()->void) -> void: register a user fn to run on process
	// exit via an exit-code-preserving EXIT trap. Validated specially in
	// checkBuiltinCall (the funcref-type check cannot be expressed in the fixed
	// table). This entry only reserves the name.
	"on_exit": {
		params: []builtinParam{{name: "handler", types: []Type{}}},
		result: Void,
	},

	// on_signal(sig: string, handler: fn()->void) -> void: register a user fn to
	// run when the process receives the named signal. sig MUST be a string literal
	// validated at compile time against {INT,TERM,HUP,QUIT,USR1,USR2}; the handler
	// must be a fn()->void funcref. Validated specially in checkBuiltinCall (the
	// literal-sig + funcref-type checks cannot be expressed in the fixed table).
	// This entry only reserves the name.
	"on_signal": {
		params: []builtinParam{{name: "sig", types: []Type{String}}, {name: "handler", types: []Type{}}},
		result: Void,
	},

	// --- Pipelines ---
	// pipe(stages: string[][]) -> RunResult: run a pipeline stage1 | stage2 | ...,
	// capturing the last stage's stdout/stderr/code. Validated specially in
	// checkPipeCall (the string[][] arg cannot be expressed by the fixed table).
	// This entry only reserves the name and feeds types.BuiltinNames().
	"pipe": {params: []builtinParam{{name: "stages", types: []Type{}}}, result: RunResult},

	// --- Background processes ---
	"spawn":     {params: []builtinParam{{name: "argv", types: []Type{}}}, result: Process},
	"wait":      {params: []builtinParam{{name: "p", types: []Type{}}}, result: RunResult},
	"is_done":   {params: []builtinParam{{name: "p", types: []Type{}}}, result: Bool},
	"signal":    {params: []builtinParam{{name: "p", types: []Type{}}, {name: "sig", types: []Type{String}}}, result: Void},
	"wait_any":  {params: []builtinParam{{name: "ps", types: []Type{}}, {name: "poll_secs", types: []Type{Int}}}, result: Process},
	"make_fifo": {params: []builtinParam{{name: "path", types: []Type{String}}}, result: Void},

	// debug(x) -> string: structural renderer for any value (S4). The fixed table
	// entry only RESERVES the name; the special-case path in checkBuiltinCall
	// handles arity/void and accepts all non-void types (the table cannot express
	// aggregate types). Not in typeInSet -- see checkDebugCall.
	"debug": {
		params: []builtinParam{{name: "x", types: []Type{Int, Float, Bool, String}}},
		result: String,
	},

	// --- Time / delay ---
	//
	// now() -> int: Unix epoch seconds via `date +%s`. Nullary, like int_max.
	// sleep(secs: int) -> Void: whole-second pause via the external `sleep`
	// command. Void result -- same reusable Void carried by print/push/the fs void
	// builtins -- so using sleep() in expression position is a located checker
	// error (void-as-value, not a bespoke typing). secs must be >= 0; a negative
	// argument aborts located at runtime.
	"now":   {params: []builtinParam{}, result: Int},
	"sleep": {params: []builtinParam{{name: "secs", types: []Type{Int}}}, result: Void},
	// random(max: int) -> int: a non-cryptographic random integer in [0, max),
	// via awk srand/rand with a per-call-varied seed. Fixed (int)->int signature,
	// like gcd; max must be > 0 (a non-positive value aborts located at runtime).
	"random": {params: []builtinParam{{name: "max", types: []Type{Int}}}, result: Int},
}

// reservedConstants are the predefined int constants (spec section 9.5). Their
// names cannot be redefined.
var reservedConstants = map[string]Type{
	"stdout": Int,
	"stderr": Int,
	"Some":   Invalid, // special-cased as a constructor before any reservedConstants read
	"None":   Invalid, // special-cased as a literal before any reservedConstants read
	"Ok":     Invalid, // special-cased as a Result constructor before any reservedConstants read
	"Err":    Invalid, // special-cased as a Result constructor before any reservedConstants read
}

// The set of builtins referenceable as first-class fn-values (eta-expansion) is
// builtinFuncrefGeneratable in funcref_class.go, the single classification
// source read by the checker, the exhaustive surface test, and (via the
// consistency test) the codegen/runtime wrapper-synthesis table.

// builtinOverloaded is the set of builtins whose builtinSigs entry looks
// monomorphic (result is a concrete scalar) but which are NOT monomorphic at
// call time -- they are dispatched specially in checkBuiltinCall and accept
// more than one concrete argument type. Members get the "overloaded or generic"
// reason in the funcref rejection message. At minimum: "length" (its
// length(T[]) array form is dispatched in checkLengthCall).
var builtinOverloaded = map[string]bool{
	"length": true,
}

// isBuiltin reports whether name is a reserved builtin function name.
func isBuiltin(name string) bool {
	_, ok := builtinSigs[name]
	return ok
}

// isReservedConstant reports whether name is a reserved constant.
func isReservedConstant(name string) bool {
	_, ok := reservedConstants[name]
	return ok
}

// isReservedName reports whether name is reserved (a builtin, a reserved
// constant, or a built-in type-name word) and so cannot be used for a user
// function, parameter, or variable. Optional is a plain ident (not a lexer
// keyword like int/error), so it must be listed explicitly.
// The isBuiltin arm is gated to EXCLUDE the removable set: a modularized builtin
// whose flat surface was removed (fs.read_file etc.) is a free ordinary
// identifier a user may bind. This is safe because a removable name collides with
// none of the other four arms (it is not a reserved constant and not one of the
// four type-tags), so excluding it from the isBuiltin arm alone fully frees it;
// the delegate bridge still resolves ns.member by builtin key, untouched.
func isReservedName(name string) bool {
	return (isBuiltin(name) && !isRemovableBuiltin(name)) || isReservedConstant(name) || name == "Optional" || name == "Result" || name == "RunResult" || name == "Process"
}
