// Package runtime holds the wisp runtime prelude: a set of POSIX-sh helper
// snippets that back the language builtins and the fail-fast path (spec
// sections 9.5, 10).
//
// Each helper is a named snippet with a declared dependency list. Codegen calls
// Emit with the set of helpers a program actually uses; Emit resolves the
// transitive dependencies and returns only those helpers, in dependency order
// (a helper always appears after everything it depends on). This is the
// tree-shaking required by spec section 9.1 / acceptance criterion 5: an unused
// helper's text never appears in the output for a program that does not need it.
//
// Naming: helpers live in the reserved "__" namespace (spec section 5/9.6). The
// fail helper is __wisp_fail; the builtin helpers are __wisp_<builtin>, except
// bool which splits by static argument type into __wisp_bool_int and
// __wisp_bool_str (the int->bool and string->bool rules differ, spec section
// 10.1, and the runtime cannot tell the two apart since both store as text).
//
// Every snippet writes its result to the global return register __ret, matching
// the calling convention in spec section 9.2, except print (void) and
// __wisp_fail (exits). All snippets satisfy the safety invariants of section
// 9.6: every expansion is double-quoted, printf formats are constants with data
// passed only as %s, no runtime string is ever re-evaluated or used as a glob
// pattern, and the int range check never feeds the candidate to $(( )).
package runtime

import (
	"sort"
	"strings"
)

// Helper IDs. These are the names codegen and tests request from Emit.
const (
	Fail = "__wisp_fail"
	// Throw is the user-throw path (M5). It is mode-aware like the error-mode
	// __wisp_fail, but stores the RAW thrown message (no position prefix) so
	// e.message == the thrown text exactly; at depth 0 it aborts located. Only
	// emitted in error mode (a program that uses try/throw).
	Throw = "__wisp_throw"
	// IDivOvf is the integer division-overflow guard: INT_MIN / -1 has no
	// representable quotient (|INT_MIN| > INT_MAX) and is a fatal $(( )) exception
	// on some shells, so codegen calls it before the `/` arithmetic. It aborts
	// located (the same model as division by zero) when num == INT_MIN && den ==
	// -1; otherwise it is a no-op. `%` has the representable result 0 for
	// INT_MIN % -1, but x86 idiv traps (SIGFPE) computing the overflowing
	// quotient alongside the remainder, so codegen guards `%` separately with
	// IMin and substitutes 0 rather than calling this.
	IDivOvf = "__wisp_idiv_ovf"
	// IMin sets __ret to INT_MIN (-2^63), computed at runtime by bit-doubling
	// because zsh cannot parse the 19-digit literal. Used by the `%` overflow
	// guard to detect an INT_MIN dividend without embedding the literal.
	IMin = "__wisp_imin"
	// AssertFail / Skip back the assertion + skip builtins (test framework). Unlike
	// __wisp_fail they are NEVER mode-aware: an assertion failure / skip EXITS the
	// current (sub)shell DIRECTLY with a reserved code (122 fail / 121 skip), never
	// routed through the errMode pending-guard unwind. Both codes sit in the safe
	// 0..125 band (>128 is the shells' signal range) and behave identically on
	// dash/busybox/bash/zsh. The message reaches stderr only as inert data (%s).
	AssertFail = "__wisp_assert_fail"
	Skip       = "__wisp_skip"
	Print      = "__wisp_print"
	String     = "string"
	Int        = "int"
	BoolInt    = "__wisp_bool_int"
	BoolStr    = "__wisp_bool_str"
	Length     = "length"
	Lower      = "lower"
	Upper      = "upper"
	Trim       = "trim"
	Replace    = "replace"

	// Float helper family (M3 PR-A). All awk calls use a CONSTANT program text
	// with operands passed only via -v, deterministic %.17g output, and every
	// float-producing/consuming helper validates the value is finite (spec 3.6).
	FFinite     = "__wisp_ffinite" // validate a candidate is a finite decimal, abort located otherwise
	FAdd        = "__wisp_fadd"
	FSub        = "__wisp_fsub"
	FMul        = "__wisp_fmul"
	FDiv        = "__wisp_fdiv"
	FCmp        = "__wisp_fcmp"
	FBool       = "__wisp_fbool"
	FStr        = "__wisp_fstr"         // string(float): canonical %.17g
	FKey        = "__wisp_fkey"         // canonical float dict-key: %.17g with sign-of-zero folded to "0"
	FFloatI     = "__wisp_ffloat_i"     // float(int): validate-and-canonicalize
	FFloatS     = "__wisp_ffloat_s"     // float(string): case-glob validate then canonicalize
	FIntT       = "__wisp_fint"         // int(float): awk truncate toward zero, then __wisp_int magnitude check
	FormatFloat = "__wisp_format_float" // <pos> <x> <decimals>: awk printf "%." d "f" (not %.*f -- busybox); decimals<0 + non-finite abort located

	// Aggregate handle runtime (M3 PR-B). Alloc issues fresh reference ids from a
	// monotonic counter; fields/elements are read/written by codegen directly as
	// __wisp_s_<id>_<field> / __wisp_a_<id>_<i> / __wisp_a_<id>_len, with no
	// generic get/set helper layer (spec 4.1).
	Alloc      = "__wisp_alloc"       // returns a fresh handle id in __ret
	BoundsFail = "__wisp_bounds_fail" // located out-of-bounds / bad-index abort

	// Dict runtime (M3 PR-C). A dict key (runtime data) is encoded into a
	// variable-name-safe, reversible, collision-free token "k<hex>" (hex of the
	// key's bytes) BEFORE any backing variable name is built; the entry lives in
	// __wisp_d_<id>_<token> and the insertion-ordered token list in
	// __wisp_d_<id>_keys. The "k" prefix isolates the key namespace from the
	// _keys ordering var and from array _len/element vars (spec 4.1). Set/get/has/
	// append/iterate are emitted inline by codegen (no generic layer, like PR-B);
	// these helpers are the byte-level encode/decode and the located missing-key
	// abort. Int-key decode re-runs __wisp_int at the keys()/for-in site.
	DictHexDig = "__wisp_hexdig"    // map 0..15 -> a hex digit in __ret
	DictEnc    = "__wisp_dkey_enc"  // key bytes -> token "k<hex>" in __ret
	DictDec    = "__wisp_dkey_dec"  // token "k<hex>" -> original bytes in __ret
	DictMiss   = "__wisp_dict_miss" // located missing-key abort

	// --- core stdlib (M6 PR-A) ---
	//
	// String ops are pure-sh and LITERAL: a runtime search/separator is matched by
	// quoting it inside ${var#"$x"} / ${var%"$x"} (a quoted operand is a literal,
	// never a glob -- the same model as __wisp_replace), so * ? [ ] \ are inert and
	// no value is ever re-evaluated (spec 9.6). split/join take the destination /
	// source array's runtime id from codegen (the alloc id is a safe decimal) and
	// read/write its __wisp_a_<id>_* backing vars via the array machinery.
	Split      = "__wisp_split"       // <pos> <id> <s> <sep>: fill array id; empty sep aborts located
	Join       = "__wisp_join"        // <id> <sep>: concat elements with sep into __ret
	Contains   = "__wisp_contains"    // <s> <sub>: literal substring test -> true/false
	StartsWith = "__wisp_starts_with" // <s> <pre> -> true/false
	EndsWith   = "__wisp_ends_with"   // <s> <suf> -> true/false
	IndexOf    = "__wisp_index_of"    // <s> <sub>: byte index, empty->0, absent->-1
	Repeat     = "__wisp_repeat"      // <pos> <s> <n>: s*n; n<0 aborts located; n==0 -> ""
	FAbs       = "__wisp_fabs"        // <pos> <x>: |x| for a float (awk magnitude, finiteness-checked)
	AbsInt     = "__wisp_abs_int"     // <pos> <x>: |x| for an int; x == INT_MIN aborts located (no positive counterpart)

	// --- I/O (M7) ---
	//
	// env/has_env read the environment via awk ENVIRON (POSIX-mandated, present in
	// busybox awk); the name is passed via -v and never interpolated into the awk
	// program, and the membership test (name in ENVIRON) distinguishes set-empty
	// (present) from unset (absent). read_file/write_file/append_file use cat,
	// printf, and quoted redirection; run rebuilds argv into its own positional
	// parameters and executes "$@", never an assembled command string. Every
	// fallible helper routes faults through the mode-aware __wisp_fail.
	Env         = "__wisp_env"          // <pos> <name>: ENVIRON[name] or located abort if unset
	HasEnv      = "__wisp_has_env"      // <name>: true/false via (name in ENVIRON)
	ReadFile    = "__wisp_read_file"    // <pos> <path>: exact file bytes; missing/unreadable/NUL abort
	WriteFile   = "__wisp_write_file"   // <pos> <path> <content>: truncate-write; failure aborts
	AppendFile  = "__wisp_append_file"  // <pos> <path> <content>: append; failure aborts
	SetEnv      = "__wisp_set_env"      // <pos> <name> <value>: export NAME=VALUE; invalid name aborts located
	UnsetEnv    = "__wisp_unset_env"    // <pos> <name>: unset NAME; invalid name aborts located
	EnvName     = "__wisp_envname"      // <pos> <name> <caller>: validate env var name [A-Za-z_][A-Za-z0-9_]*; invalid aborts located
	Run         = "__wisp_run"          // <pos> <handle-id>: run argv, capture stdout; empty/nonzero abort
	RunInput    = "__wisp_run_input"    // <pos> <argv-id> <stdin>: printf %s stdin | argv; capture stdout; empty/nonzero abort
	RunEnv      = "__wisp_run_env_argv" // <pos> <new-id> <argv-id> <env-id>: build `env NAME=VALUE... argv` into new-id
	ExecCommand = "__wisp_exec_command" // <pos> <handle-id>: replace the shell process via exec "$@"; empty argv -> located abort
	SetStdin    = "__wisp_set_stdin"    // <pos> <content>: replace fd 0 with content bytes

	// --- Collections core ---
	//
	// scmp is the byte-lexicographic string less-than used by sort(string[]). The
	// operands flow through the ENVIRON array (NOT awk -v, which would interpret C
	// escape sequences in a value containing a backslash) under an LC_ALL=C prefix
	// on the awk process so the comparison is byte order regardless of locale.
	Scmp = "__wisp_scmp" // <a> <b>: byte-lexicographic a<b -> true/false

	// b2i maps a bool text value (the type system guarantees exactly "true" or
	// "false") to 1/0 so bool ordering (< <= > >=, min/max, sort) reuses the numeric
	// integer compare uniformly. The operand is double-quoted; no user-controlled
	// payload can reach it beyond that two-element domain.
	B2i = "__wisp_b2i" // <b>: bool true/false -> 1/0

	// parse_args classifies a CLI argument list into values/switches/positionals
	// (getopts milestone). Codegen pre-allocates the result dict + two array
	// handles (and inits the dict's _keys list); this helper scans the args array
	// and fills their backing vars. Flag NAMES (value_flags membership, switch
	// dedup, value dict keys) flow through the reversible "k<hex>" token of
	// __wisp_dkey_enc so a flag name carrying any bytes is name-safe and
	// space/glob-inert; flag VALUES and positionals are stored verbatim via a
	// deferred `\$var` eval (N1: never re-evaluated or globbed). Pure, total,
	// cross-shell.
	ParseArgs = "__wisp_parse_args" // <dictId> <swId> <posId> <argsId> <vfId>: fill the three result handles

	// --- coverage instrumentation (test framework, spec R15-R17) ---
	//
	// __wisp_cov <record>: append one newline-terminated coverage hit record
	// `<file>:<line>` to $COVFILE. Emitted ONLY in coverage mode (tree-shaken
	// otherwise) and is the single statement-level marker codegen inserts. The
	// record is a double-quoted INERT literal built by the compiler (N1: never
	// re-evaluated or globbed). One short `printf >> $COVFILE` append: POSIX
	// O_APPEND makes each record atomic and tests run strictly sequentially, so
	// accumulation across per-test subshells and across shells is correct.
	// ROBUSTNESS: when $COVFILE is unset/empty the helper no-ops (returns) so a
	// coverage-compiled script run outside `wisp test --coverage` does not crash.
	Cov = "__wisp_cov"

	// --- String round-out ---
	//
	// All byte-based and pure-sh, modeled on the M6 string helpers: search/affix
	// strings are matched/stripped via quoted parameter expansion (literal, never a
	// glob). The fallible ones (substring/char_at/count/replace_first/pad_*) take a
	// leading <pos> and route faults through __wisp_fail.
	Substring     = "__wisp_substring"      // <pos> <s> <start> <end>: byte range [start,end); range abort
	CharAt        = "__wisp_char_at"        // <pos> <s> <i>: one byte at i; range abort
	LastIndexOf   = "__wisp_last_index_of"  // <s> <sub>: rightmost byte index, -1, empty->len
	Count         = "__wisp_count"          // <pos> <s> <sub>: non-overlapping count; empty sub abort
	ReplaceFirst  = "__wisp_replace_first"  // <pos> <s> <search> <with>: first match only; empty search abort
	TrimStart     = "__wisp_trim_start"     // <s>: strip leading whitespace
	TrimEnd       = "__wisp_trim_end"       // <s>: strip trailing whitespace
	TrimPrefix    = "__wisp_trim_prefix"    // <s> <pre>: drop pre if present
	TrimSuffix    = "__wisp_trim_suffix"    // <s> <suf>: drop suf if present
	PadStart      = "__wisp_pad_start"      // <pos> <s> <width> <fill>: left-pad to width; empty fill abort if needed
	PadEnd        = "__wisp_pad_end"        // <pos> <s> <width> <fill>: right-pad to width; empty fill abort if needed
	Lines         = "__wisp_lines"          // <id> <s>: fill array with newline-separated lines
	IsEmpty       = "__wisp_is_empty"       // <s>: length(s)==0 -> true/false
	ReverseString = "__wisp_reverse_string" // <s>: byte-reversed string in __ret; total
	Ord           = "__wisp_ord"            // <pos> <s>: byte value of first byte; empty aborts located
	Chr           = "__wisp_chr"            // <pos> <code>: single-byte string for code 1-255; 0/out-of-range aborts located

	// --- I/O tail ---
	//
	// read_line/read_stdin use stdin of the running script. change_dir persists
	// the cwd for the life of the script (single flat-shell model). run_full
	// captures stdout/stderr via mktemp + x-sentinel; its three output globals
	// are copied to per-handle vars by codegen before any next call can overwrite.
	ReadLine     = "__wisp_read_line"      // no args: __ret=line | __wisp_rl_eof=1 on EOF; total
	ReadSecret   = "__wisp_read_secret"    // <prompt>: prompt to stderr, no-echo line read (stty -echo best-effort), EOF->none; total
	ReadStdin    = "__wisp_read_stdin"     // no args: __ret=all-stdin content (x-sentinel); total
	ChangeDir    = "__wisp_change_dir"     // <pos> <path>: cd --; bad path aborts located
	RunFull      = "__wisp_run_full"       // <pos> <argv-handle-id>: run argv, set __wisp_rf_stdout/stderr/code; empty argv or mktemp failure aborts located
	RunInputFull = "__wisp_run_input_full" // <pos> <argv-id> <stdin>: printf %s stdin | argv; capture stdout/stderr/code; no abort

	// --- Numeric / math ---
	//
	// Float ops use awk (%.17g) and an op-named located abort on non-finite/undefined
	// (sqrt) or reuse __wisp_int's range check (floor/ceil/round). gcd/lcm are
	// shell arithmetic that abort located on an INT_MIN operand (2^63 overflow).
	// int_or/float_or are NON-aborting validators returning a
	// canonical value or the fallback. sqrt uses Newton's method (basic arithmetic
	// only) because minimal busybox awk has no math support (no sqrt()/^).
	Sqrt       = "__wisp_sqrt"        // <pos> <x>
	Floor      = "__wisp_floor"       // <pos> <x> -> int (toward -inf)
	Ceil       = "__wisp_ceil"        // <pos> <x> -> int (toward +inf)
	Round      = "__wisp_round"       // <pos> <x> -> int (floor(x+0.5))
	Gcd        = "__wisp_gcd"         // <pos> <a> <b> -> gcd(|a|,|b|); an INT_MIN operand aborts located (2^63 overflow)
	Lcm        = "__wisp_lcm"         // <pos> <a> <b> -> lcm(|a|,|b|), 0 if either 0; propagates gcd's INT_MIN abort
	Pow        = "__wisp_pow"         // <pos> <base> <exp> -> float; awk-only, no ^ or math library
	Exp        = "__wisp_exp"         // <pos> <x> -> e^x float; pure-arithmetic Taylor (no awk exp); overflow aborts located
	Ln         = "__wisp_ln"          // <pos> <x> -> ln(x) float; pure-arithmetic atanh series (no awk log); x<=0 aborts located
	Log10      = "__wisp_log10"       // <pos> <x> -> ln(x)/ln(10) float; composes Ln + FDiv by a baked constant
	Log2       = "__wisp_log2"        // <pos> <x> -> ln(x)/ln(2) float; composes Ln + FDiv by a baked constant
	Pi         = "__wisp_pi"          // -> the float literal 3.141592653589793; total, no args
	MinInt     = "__wisp_min_int"     // <a> <b> -> the smaller int, unchanged; total
	MinFloat   = "__wisp_min_float"   // <a> <b> -> the smaller float, unchanged (via __wisp_fcmp); total
	MaxInt     = "__wisp_max_int"     // <a> <b> -> the larger int, unchanged; total
	MaxFloat   = "__wisp_max_float"   // <a> <b> -> the larger float, unchanged (via __wisp_fcmp); total
	ClampInt   = "__wisp_clamp_int"   // <x> <lo> <hi> -> max(lo, min(x, hi)) for int; total
	ClampFloat = "__wisp_clamp_float" // <x> <lo> <hi> -> max(lo, min(x, hi)) for float (via __wisp_fcmp); total
	SignInt    = "__wisp_sign_int"    // <x> -> -1/0/1 for int; total
	SignFloat  = "__wisp_sign_float"  // <x> -> -1/0/1 for float (via __wisp_fcmp); total
	IntMax     = "__wisp_int_max"     // -> int (shell POSIX max; determined at runtime by doubling)
	IntMin     = "__wisp_int_min"     // -> int (shell POSIX min; -int_max - 1)
	IntOr      = "__wisp_int_or"      // <s> <fallback> -> canonical int or fallback
	FloatOr    = "__wisp_float_or"    // <s> <fallback> -> canonical float or fallback
	ParseInt   = "__wisp_parse_int"   // <s>: canonical int in __ret + exit 0 if valid, else exit 1 (-> None)
	ParseFloat = "__wisp_parse_float" // <s>: canonical float in __ret + exit 0 if valid, else exit 1 (-> None)

	// --- Filesystem + process (fs milestone) ---
	//
	// Predicates and cwd/env_or are total (never abort). The mutating ops and
	// list_dir/run_status are fallible and route faults through __wisp_fail with a
	// leading <pos>. Every path/name flows as a double-quoted expansion; -- is
	// mandatory on the option-parsing positional-path commands (mkdir/rm/rmdir/mv,
	// and command -v for which); env_or's name flows only via awk -v / ENVIRON[n].
	FileExists = "__wisp_file_exists" // <path>: [ -e ] -> true/false; total
	PidAlive   = "__wisp_pid_alive"   // <pid>: kill -0 pid -> true/false; total (ESRCH/EPERM -> false)
	IsDir      = "__wisp_is_dir"      // <path>: [ -d ] -> true/false; total
	Cwd        = "__wisp_cwd"         // pwd into __ret
	// Arg0 is a SENTINEL, not a prelude helper. program_path() has no function
	// body: it lowers to a read of the global $__wisp_arg0, captured ONCE at top
	// level (`__wisp_arg0="$0"`) before main runs. The lowering registers this id
	// in the codegen `used` set so the footer-assembly step knows to emit the
	// capture line; a program that never calls program_path() emits no capture and
	// is byte-identical to before (spec P2). It is intentionally ABSENT from the
	// registry so EmitMode/usedHelpers ignore it (it has no body to emit).
	Arg0        = "__wisp_arg0"         // $0 capture global (sentinel; no helper body)
	ProgramPath = "__wisp_program_path" // -> $__wisp_arg0; total, no args; callers must also g.use(Arg0)
	DirName     = "__wisp_dir_name"     // <path>: directory portion (pure POSIX-style)
	BaseName    = "__wisp_base_name"    // <path>: final component (pure POSIX-style)
	EnvOr       = "__wisp_env_or"       // <name> <fallback>: ENVIRON[name] if set else fallback
	MakeDir     = "__wisp_make_dir"     // <pos> <path>: mkdir -p -- ; failure aborts located
	RemoveFile  = "__wisp_remove_file"  // <pos> <path>: rm -f -- ; real failure aborts located
	RemoveDir   = "__wisp_remove_dir"   // <pos> <path>: rmdir -- ; non-empty/missing aborts located
	Rename      = "__wisp_rename"       // <pos> <from> <to>: mv -- ; failure aborts located
	Which       = "__wisp_which"        // <name>: __ret=command -v path; returns command -v's exit status
	ListDir     = "__wisp_list_dir"     // <pos> <id> <dir>: fill array id with base names; missing/non-dir aborts
	RunStatus   = "__wisp_run_status"   // <pos> <handle-id>: run argv bare, __ret=$?; empty argv aborts

	// --- Filesystem metadata (fs-metadata milestone) ---
	//
	// is_file/is_symlink mirror the predicate helpers (total, no position arg).
	// file_size is fallible (located; $1=pos, $2=path).
	// chmod/symlink are Void fallible ($1=pos, $2/$3=user args); chmod transposes
	// the operands ($3=mode, $2=path) because chmod's CLI is mode-then-path.
	IsFile       = "__wisp_is_file"       // <path>: [ -f ] -> true/false; total
	IsSymlink    = "__wisp_is_symlink"    // <path>: [ -L ] -> true/false; total
	FileSize     = "__wisp_file_size"     // <pos> <path>: wc -c < path, strip leading spaces; failure aborts located
	Chmod        = "__wisp_chmod"         // <pos> <path> <mode>: chmod -- mode path; failure aborts located
	Symlink      = "__wisp_symlink"       // <pos> <target> <link_path>: ln -s -- target link_path; failure aborts located
	SymlinkForce = "__wisp_symlink_force" // <pos> <target> <link_path>: rm -f link_path then ln -s target link_path; failure aborts located
	ReadLink     = "__wisp_read_link"     // <path>: readlink -- path; __ret=target, returns exit status; nonzero->None
	TempFile     = "__wisp_temp_file"     // <pos>: mktemp, __ret=path; failure aborts located
	TempDir      = "__wisp_temp_dir"      // <pos>: mktemp -d, __ret=path; failure aborts located
	Glob         = "__wisp_glob"          // <pos> <id> <pattern>: fill array id with shell-expanded matches; total (no-match -> empty)

	// --- Regular expressions (regex milestone) ---
	//
	// POSIX ERE via awk. The pattern, subject, and replacement flow ONLY through
	// per-process ENVIRON assignment (never -v, which C-unescapes a value and
	// corrupts backslashes) under LC_ALL=C, with a CONSTANT awk program (the
	// __wisp_scmp shape). Every helper uses the unified result protocol: awk prints
	// its payload then a sentinel byte x (BOTH printfs inside awk so the command
	// substitution status stays awk's), the helper snapshots rc=$? immediately, and
	// rc != 0 is a uniform malformed-pattern located abort (spec section 5). All
	// four are fallible and take a leading <pos>.
	Matches      = "__wisp_matches"        // <pos> <s> <p>: (s~re) -> true/false; bad pattern aborts
	RegexFind    = "__wisp_regex_find"     // <pos> <s> <p>: token 1<match> / 0 in __ret; bad pattern aborts
	RegexFindAll = "__wisp_regex_find_all" // <pos> <id> <s> <p>: fill array id with non-empty matches; bad pattern aborts
	RegexReplace = "__wisp_regex_replace"  // <pos> <s> <p> <repl>: gsub-all into __ret; bad pattern aborts

	// --- Time / delay ---
	Now    = "__wisp_now"    // -> int: Unix epoch seconds via date +%s
	Sleep  = "__wisp_sleep"  // <pos> <secs>: whole-second pause; secs>=0 else located abort
	Random = "__wisp_random" // <pos> <max>: int in [0,max) via per-call-seeded awk; max>0 else located abort

	// --- Signals / traps ---
	//
	// on_exit registers a user fn as an EXIT trap handler. The handler name is
	// a bare [A-Za-z0-9_] mangled word (safe for $1 expansion). $? and
	// $__wisp_ec are escaped so they expand at TRAP-FIRE time: capture the real
	// exit code first, run the handler, then restore and exit with the original
	// code. exit inside an EXIT trap does NOT re-fire the trap (POSIX). Total
	// void: no __wisp_fail dependency.
	OnExit = "__wisp_on_exit" // <handler>: install exit-code-preserving EXIT trap

	// on_signal registers a user fn as the trap action for a named signal. Both
	// operands are inert compile-time words: $1 is the mangled handler name (an
	// [A-Za-z0-9_] word) and $2 is the allowlist-validated signal name. The saved
	// action is the bare handler name -- on_signal RESUMES after the handler (no
	// $? capture, no exit), so to terminate the handler must call exit() itself.
	// Total: the sig is compile-time-validated, no runtime check, no __wisp_fail.
	OnSignal = "__wisp_on_signal" // <handler> <sig>: install a named-signal trap

	// --- Background processes ---
	//
	// spawn launches a wrapper subshell that backgrounds the command, captures
	// stdout/stderr to mktemp files, publishes the command pid via a
	// newline-sentinel pidfile, and records the exit/death code to a done-marker.
	// wait blocks on the wrapper, reads the marker + temp files into a cached
	// RunResult. is_done/signal/wait_any read the marker without reaping.
	// make_fifo creates a named pipe for IPC.
	Spawn    = "__wisp_spawn"     // <pos> <argv-id>: launch wrapper, publish pid via newline sentinel; located
	Wait     = "__wisp_wait"      // <proc-id>: block on wrapper, build+cache RunResult; idempotent
	IsDone   = "__wisp_is_done"   // <proc-id>: marker non-empty -> true/false; total
	Signal   = "__wisp_signal"    // <proc-id> <sig>: guarded best-effort kill; total
	WaitAny  = "__wisp_wait_any"  // <pos> <array-id> <poll>: poll markers, first-in-list-order done; located
	MakeFifo = "__wisp_make_fifo" // <pos> <path>: mkfifo -- "$path"; located

	// --- Pipelines ---
	//
	// pipe(stages: string[][]) -> RunResult. __wisp_pipe_exec rebuilds one stage's
	// argv (run-family pattern) and exec's it inside its own ( ) segment.
	// __wisp_pipe walks the outer array, builds a per-segment-redirect pipeline
	// string of ( __wisp_pipe_exec <id> [redirs] ) segments joined by |, evals it,
	// and copies stdout/stderr/code into the __wisp_rf_* globals (like run_full).
	Pipe     = "__wisp_pipe"      // <pos> <outer-id>: build+eval a pipeline string of stage-exec segments; located
	PipeExec = "__wisp_pipe_exec" // <stage-id>: rebuild that stage's argv and exec "$@"; runs in a ( ) segment

	// Builtin funcref wrappers (__wisp_builtin_<name>) are synthesized from the
	// spec table in builtinref.go, which owns their ids, deps, and shell source.
)

// failModeSrc is the mode-aware __wisp_fail body emitted in error mode (a
// program that uses try/throw). At depth 0 it behaves like M1 (located print +
// exit 1). At depth > 0 it converts the fault to a catchable error: it stores
// the POSITION-PREFIXED located text ("<pos>: <msg>") as __wisp_err_msg so a
// caught fault's e.message is the located message, sets __wisp_err_pending, and
// RETURNS (no exit) so the per-statement guards can unwind to the nearest try.
// FIRST-FAULT-WINS: if a fault is already pending it does NOT overwrite the
// message (a helper that keeps running after a fault cannot clobber the
// original). The depth/pending/msg vars are compiler-controlled. __wisp_err_pos
// is CLEARED here: a fault's location is already baked into __wisp_err_msg (the
// "<pos>: <msg>" form), so the try epilogue prints it verbatim (M4); only the
// throw path, whose message is raw, carries a separate position.
const failModeSrc = `__wisp_fail() {
	if [ "${__wisp_try_depth:-0}" -gt 0 ]; then
		if [ -z "$__wisp_err_pending" ]; then
			__wisp_err_msg="$1: $2"
			__wisp_err_code=
			__wisp_err_pos=
			__wisp_err_pending=1
		fi
		return
	fi
	printf 'wisp: %s: %s\n' "$1" "$2" >&2
	exit 1
}`

// failModeCauseSrc is failModeSrc with the __wisp_err_cause clear added inside
// the depth>0 first-fault-wins guard, immediately alongside the __wisp_err_code=
// clear -- NOT unconditionally and NOT on the depth-0 exit branch -- so it keeps
// first-fault-wins parity with _code (a fault has no cause, and a stale cause
// from a prior in-flight error must not leak onto it, AC4). Emitted in place of
// failModeSrc ONLY when the program uses `wrap` (the throw-path threading is
// gated), so a non-`wrap` faulting program stays byte-identical (AC6).
const failModeCauseSrc = `__wisp_fail() {
	if [ "${__wisp_try_depth:-0}" -gt 0 ]; then
		if [ -z "$__wisp_err_pending" ]; then
			__wisp_err_msg="$1: $2"
			__wisp_err_code=
			__wisp_err_cause=
			__wisp_err_pos=
			__wisp_err_pending=1
		fi
		return
	fi
	printf 'wisp: %s: %s\n' "$1" "$2" >&2
	exit 1
}`

// throwModeSrc is the user-throw path (M5). Like the mode-aware fail, but it
// stores the RAW thrown message (no position prefix) so e.message equals the
// thrown text exactly. At depth 0 it aborts located with the raw message. At
// depth > 0 it records the throw position in __wisp_err_pos (M4) so an UNCAUGHT
// throw escaping a try epilogue still aborts with the located
// "wisp: <pos>: <msg>" form, matching a direct depth-0 abort; the raw message
// alone reaches e.message in a catch.
const throwModeSrc = `__wisp_throw() {
	if [ "${__wisp_try_depth:-0}" -gt 0 ]; then
		if [ -z "$__wisp_err_pending" ]; then
			__wisp_err_msg="$2"
			__wisp_err_pos="$1"
			__wisp_err_pending=1
		fi
		return
	fi
	printf 'wisp: %s: %s\n' "$1" "$2" >&2
	exit 1
}`

// Shared pure-arithmetic awk series fragments (no awk libm: busybox awk is
// commonly built WITHOUT math support, so awk exp/log/^ are unavailable). These
// const strings are the SOURCE-level fragments that __wisp_pow's float-exponent
// path already used inline; they are now also assembled into the standalone
// __wisp_exp/__wisp_ln helpers. Sharing is at the awk-SOURCE-fragment level, not
// the shell-call level: __wisp_pow stays a single awk process whose text is
// reassembled BYTE-IDENTICALLY from these consts (no extra %.17g round-trips, so
// pow's runtime value and emitted .sh are unchanged). The fragments carry pow's
// original two-tab indentation; awk ignores indentation, so reusing them at a
// different nesting depth in the standalone helpers is functionally identical.
//
// lnSeriesAwk: given x (already x = input + 0) and k = 0, range-reduces x into
// [0.6, 1.2), runs the atanh series, and leaves the natural log in ln_b. The
// `while (x < 0.6) x = x*2` loop does NOT terminate for x == 0 and diverges for
// x < 0, so any standalone caller MUST guard x <= 0 BEFORE this fragment.
const lnSeriesAwk = `		while (x >= 1.2) { x = x / 2; k = k + 1 }
		while (x < 0.6)  { x = x * 2; k = k - 1 }
		t = (x - 1) / (x + 1)
		t2 = t * t; s = t; p = t; n = 1
		while (n < 200) {
			n = n + 2; p = p * t2; s = s + p / n
			if (p / n < 1e-17 && p / n > -1e-17) break
		}
		ln_b = 2 * s + k * 0.6931471805599453
`

// expSeriesAwk: given lp, runs the exp Taylor series e^lp = 1 + lp + lp^2/2! +
// ... and leaves the result in acc.
const expSeriesAwk = `		# exp via Taylor: e^x = 1 + x + x^2/2! + ...
		acc = 1; term = 1; n = 0
		while (n < 300) {
			n = n + 1; term = term * lp / n; acc = acc + term
			if (term < 1e-17 && term > -1e-17) break
		}
`

// helper is one prelude snippet plus the IDs it depends on.
type helper struct {
	id   string
	deps []string
	// order is a stable rank used as a secondary sort key so Emit output is
	// deterministic regardless of request order, while still honoring deps.
	order int
	src   string
}

// registry is the catalog of all prelude helpers, keyed by ID.
var registry = map[string]helper{
	// __wisp_fail <pos> <msg>: report a located fail-fast abort and exit 1 (spec
	// section 4). The format string is a compiler constant; both the position and
	// the message flow only through %s, so neither can be reinterpreted as a
	// format (M1 section 9.6 invariant). Resulting stderr:
	// `wisp: <file>:<line>:<col>: <msg>`.
	Fail: {
		id:    Fail,
		order: 0,
		src: `__wisp_fail() {
	printf 'wisp: %s: %s\n' "$1" "$2" >&2
	exit 1
}`,
	},

	// __wisp_throw <pos> <msg>: the user-throw path (M5). Mode-aware like
	// __wisp_fail but stores the RAW message. Only emitted in error mode; its
	// registered src is the mode-aware body (EmitMode substitutes nothing for it).
	Throw: {
		id:    Throw,
		order: 0,
		src:   throwModeSrc,
	},

	// __wisp_assert_fail <pos> <message>: an assertion failure. Prints the located
	// message to stderr and EXITS the current (sub)shell with the reserved code
	// 122. It does NOT use the `wisp:`-prefixed program-abort form (an assertion is
	// not a program crash); the Task-3 test runner keys off the exit CODE primarily
	// and parses this message secondarily. STDERR SHAPE (stable, runner-parsed):
	//	<pos>: <message>
	// where <message> is built by codegen and always begins with "assertion
	// failed", e.g. `assertion failed`, `assertion failed: <user msg>`, or
	// `assertion failed: 1 != 2` (assert_eq). The format string is a compiler
	// constant; <pos> and <message> flow only through %s, so neither can be
	// reinterpreted as a format or a glob (section 9.6 invariant). 122 is in the
	// safe 0..125 band and is reported identically on all four shells.
	AssertFail: {
		id:    AssertFail,
		order: 0,
		src: `__wisp_assert_fail() {
	printf '%s: %s\n' "$1" "$2" >&2
	exit 122
}`,
	},

	// __wisp_skip <pos> <reason>: a skip. Prints the located reason to stderr and
	// EXITS the current (sub)shell with the reserved code 121, so the rest of the
	// body does not run. NOT mode-aware (no errMode unwind). STDERR SHAPE (stable,
	// runner-parsed):
	//	<pos>: SKIP: <reason>
	// <pos> and <reason> flow only through %s (inert data). 121 is in the safe
	// 0..125 band and is reported identically on all four shells.
	Skip: {
		id:    Skip,
		order: 0,
		src: `__wisp_skip() {
	printf '%s: SKIP: %s\n' "$1" "$2" >&2
	exit 121
}`,
	},

	// print(msg, fd): fd is the literal 1 (stdout) or 2 (stderr), resolved at
	// compile time from print's `to` (spec section 9.4). printf, never echo.
	Print: {
		id:    Print,
		order: 1,
		src: `__wisp_print() {
	if [ "$2" = 2 ]; then
		printf '%s\n' "$1" >&2
	else
		printf '%s\n' "$1"
	fi
}`,
	},

	// string(x): identity. int and bool already store as their text form, and a
	// string is itself, so string() just copies into the return register.
	String: {
		id:    String,
		order: 2,
		src: `__wisp_string() {
	__ret="$1"
}`,
	},

	// length(s): byte length, shell-independent. `${#1}` counts codepoints (not
	// bytes) under bash/zsh in a UTF-8 locale, and busybox ash counts codepoints
	// regardless of a runtime LC_ALL assignment, so the documented byte semantics
	// hold only via an LC_ALL=C awk length() (the reverse_string model): awk's
	// length is a byte count under LC_ALL=C on all four targets.
	Length: {
		id:    Length,
		order: 3,
		src: `__wisp_length() {
	__ret="$(__wisp_len_s="$1" LC_ALL=C awk 'BEGIN{ print length(ENVIRON["__wisp_len_s"]) }')"
}`,
	},

	// bool(int): 0 -> false, any nonzero (incl. negative) -> true. The argument
	// is type int, so it is syntactically [+-]?[0-9]+ (the int-validity
	// invariant), making the -eq comparison injection-safe.
	BoolInt: {
		id:    BoolInt,
		order: 4,
		src: `__wisp_bool_int() {
	if [ "$1" -eq 0 ]; then
		__ret=false
	else
		__ret=true
	fi
}`,
	},

	// bool(string): canonical only. Exactly "true"/"false"; anything else aborts
	// (spec section 10.1). The subject is single-quoted-pattern-free: a plain
	// literal compare via case, with the metachar-free arms being constants.
	// $1 is the located source position (forwarded to __wisp_fail); $2 is the
	// subject (M2 section 4).
	BoolStr: {
		id:    BoolStr,
		deps:  []string{Fail},
		order: 5,
		src: `__wisp_bool_str() {
	case "$2" in
		true) __ret=true ;;
		false) __ret=false ;;
		*) __wisp_fail "$1" "to_bool(): not a bool: \"$2\"" ;;
	esac
}`,
	},

	// int(s): validate [+-]?[0-9]+ with no surrounding whitespace, AND verify the
	// magnitude fits signed 64-bit by a length comparison plus a digit-by-digit
	// (single-digit arithmetic) compare against the width bound -- never by
	// feeding the whole candidate to $(( )), which would itself wrap (spec
	// section 9.6 invariant 5). Normalizes leading zeros and the sign of zero.
	Int: {
		id:    Int,
		deps:  []string{Fail},
		order: 6,
		src: `__wisp_int() {
	__i_s="$2"
	__i_body="$__i_s"
	__i_neg=0
	case "$__i_s" in
		-*) __i_body="${__i_s#-}"; __i_neg=1 ;;
		+*) __i_body="${__i_s#+}" ;;
	esac
	case "$__i_body" in
		'' | *[!0-9]*) __wisp_fail "$1" "to_int(): not an integer: \"$__i_s\"" ;;
	esac
	[ -n "$__wisp_err_pending" ] && return
	__i_mag="$__i_body"
	while :; do
		case "$__i_mag" in
			0?*) __i_mag="${__i_mag#0}" ;;
			*) break ;;
		esac
	done
	if [ "$__i_neg" -eq 1 ]; then
		__i_bound=9223372036854775808
	else
		__i_bound=9223372036854775807
	fi
	__i_len="${#__i_mag}"
	__i_blen="${#__i_bound}"
	if [ "$__i_len" -gt "$__i_blen" ]; then
		__wisp_fail "$1" "to_int(): out of range: \"$__i_s\""
		[ -n "$__wisp_err_pending" ] && return
	fi
	if [ "$__i_len" -eq "$__i_blen" ]; then
		# Equal length: lexical (= numeric) compare digit by digit. Single-digit
		# arithmetic only, so the candidate is never fed whole to $(( )).
		__i_a="$__i_mag"
		__i_b="$__i_bound"
		while [ -n "$__i_a" ]; do
			__i_da="${__i_a%"${__i_a#?}"}"
			__i_db="${__i_b%"${__i_b#?}"}"
			if [ "$__i_da" -gt "$__i_db" ]; then
				__wisp_fail "$1" "to_int(): out of range: \"$__i_s\""
				[ -n "$__wisp_err_pending" ] && return
			fi
			if [ "$__i_da" -lt "$__i_db" ]; then
				break
			fi
			__i_a="${__i_a#?}"
			__i_b="${__i_b#?}"
		done
	fi
	if [ "$__i_neg" -eq 1 ] && [ "$__i_mag" != 0 ]; then
		__ret="-$__i_mag"
	else
		__ret="$__i_mag"
	fi
}`,
	},

	// lower/upper: ASCII case fold via tr, fed with printf '%s' (never echo, which
	// mangles leading '-'/backslashes). A trailing sentinel '_' is appended before
	// capture and stripped after, so $( ) does not eat trailing newlines -- the
	// "other bytes unchanged" contract (spec section 9.5). '_' is unaffected by
	// the A-Z<->a-z translation.
	Lower: {
		id:    Lower,
		order: 7,
		src: `__wisp_lower() {
	__ret="$(printf '%s_' "$1" | tr 'A-Z' 'a-z')"
	__ret="${__ret%_}"
}`,
	},
	Upper: {
		id:    Upper,
		order: 8,
		src: `__wisp_upper() {
	__ret="$(printf '%s_' "$1" | tr 'a-z' 'A-Z')"
	__ret="${__ret%_}"
}`,
	},

	// trim(s): strip leading/trailing bytes in {space, tab, \n, \r}; interior
	// untouched. Pure-sh, literal: it inspects one boundary byte at a time via a
	// case over the four literal characters. The \r is precomputed once (printf)
	// rather than per iteration.
	Trim: {
		id:    Trim,
		order: 9,
		src: `__wisp_trim() {
	__t_s="$1"
	__t_cr="$(printf '\r')"
	while [ -n "$__t_s" ]; do
		__t_c="${__t_s%"${__t_s#?}"}"
		case "$__t_c" in
			' ' | '	' | '
' | "$__t_cr") __t_s="${__t_s#?}" ;;
			*) break ;;
		esac
	done
	while [ -n "$__t_s" ]; do
		__t_c="${__t_s#"${__t_s%?}"}"
		case "$__t_c" in
			' ' | '	' | '
' | "$__t_cr") __t_s="${__t_s%?}" ;;
			*) break ;;
		esac
	done
	__ret="$__t_s"
}`,
	},

	// replace(s, search, replacement): LITERAL replace-all, left-to-right,
	// non-overlapping; aborts on empty search; replacement may be empty (spec
	// section 9.5). Byte-oriented via LC_ALL=C awk index()+substr() advancing
	// loop -- identical on all four shells; a shell ${rest#"$search"} scan
	// would step codepoints under bash/zsh/busybox in a UTF-8 locale. Operands
	// flow via ENVIRON[] only (never interpolated into the awk program text).
	// The trailing-x sentinel preserves any trailing bytes through $(...).
	Replace: {
		id:    Replace,
		deps:  []string{Fail},
		order: 10,
		src: `__wisp_replace() {
	if [ -z "$3" ]; then
		__wisp_fail "$1" "replace(): empty search string"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__r_r="$(__wisp_r_s="$2" __wisp_r_n="$3" __wisp_r_w="$4" LC_ALL=C awk 'BEGIN{
	s=ENVIRON["__wisp_r_s"]; nd=ENVIRON["__wisp_r_n"]; w=ENVIRON["__wisp_r_w"]
	out=""; sl=length(nd)
	while ((p=index(s,nd))>0) { out=out substr(s,1,p-1) w; s=substr(s,p+sl) }
	printf "%sx", out s
}')"
	__ret="${__r_r%x}"
}`,
	},

	// --- float helper family (M3 PR-A) ---
	//
	// Every awk call below uses a CONSTANT program text; operand VALUES flow only
	// through `-v` assignments (never interpolated into the program), so a
	// float-looking string cannot inject awk/shell (spec 3.3). The output format
	// is pinned to %.17g so a double round-trips and results are deterministic
	// across busybox awk and the host awk (spec 3.3 / risks). `(x+0)` forces
	// numeric coercion of the -v string operand.

	// __wisp_ffinite <pos> <value>: enforce the float-validity invariant (spec
	// 3.6). A finite wisp float matches [+-]?[0-9]+(\.[0-9]+)? -- NO exponent, no
	// inf/nan. The check is a POSIX `case` glob (the M1 safe-match mechanism, not
	// an interpolated regex): reject anything with a non [0-9.] body byte, an
	// empty body, a leading/trailing dot, or more than one dot. So awk results
	// like `inf`, `nan`, or an exponent form (`1e+17`, which %.17g emits for
	// large magnitudes) all abort located rather than entering the float space
	// and re-feeding `-v`. On success __ret is the (sign-preserved) value.
	FFinite: {
		id:    FFinite,
		deps:  []string{Fail},
		order: 20,
		src: `__wisp_ffinite() {
	__f_body="$2"
	case "$2" in
		-* | +*) __f_body="${2#?}" ;;
	esac
	case "$__f_body" in
		'' | *[!0-9.]* | .* | *. | *.*.*)
			__wisp_fail "$1" "float: non-finite or out-of-domain result: \"$2\"" ;;
	esac
	[ -n "$__wisp_err_pending" ] && return
	__ret="$2"
}`,
	},

	// __wisp_fadd/fsub/fmul <pos> <a> <b>: compute a OP b via a constant awk
	// program, then validate the result is finite (abort located on inf/nan/
	// exponent). a and b are already invariant-valid floats, passed via -v.
	FAdd: {
		id:    FAdd,
		deps:  []string{FFinite},
		order: 21,
		src: `__wisp_fadd() {
	__f_r="$(LC_ALL=C awk -v a="$2" -v b="$3" 'BEGIN{ printf "%.17g", (a+0)+(b+0) }')"
	__wisp_ffinite "$1" "$__f_r"
}`,
	},
	FSub: {
		id:    FSub,
		deps:  []string{FFinite},
		order: 22,
		src: `__wisp_fsub() {
	__f_r="$(LC_ALL=C awk -v a="$2" -v b="$3" 'BEGIN{ printf "%.17g", (a+0)-(b+0) }')"
	__wisp_ffinite "$1" "$__f_r"
}`,
	},
	FMul: {
		id:    FMul,
		deps:  []string{FFinite},
		order: 23,
		src: `__wisp_fmul() {
	__f_r="$(LC_ALL=C awk -v a="$2" -v b="$3" 'BEGIN{ printf "%.17g", (a+0)*(b+0) }')"
	__wisp_ffinite "$1" "$__f_r"
}`,
	},

	// __wisp_fdiv <pos> <a> <b>: guard a numeric-zero divisor FIRST (located
	// abort, matching int division), so awk never yields inf/nan from /0; then
	// compute and validate finiteness. The zero test is itself a constant awk
	// program with the divisor passed via -v.
	FDiv: {
		id:    FDiv,
		deps:  []string{FFinite, Fail},
		order: 24,
		src: `__wisp_fdiv() {
	if awk -v b="$3" 'BEGIN{ exit !((b+0)==0) }'; then
		__wisp_fail "$1" "division by zero"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__f_r="$(LC_ALL=C awk -v a="$2" -v b="$3" 'BEGIN{ printf "%.17g", (a+0)/(b+0) }')"
	__wisp_ffinite "$1" "$__f_r"
}`,
	},

	// __wisp_fcmp <pos> <op> <a> <b>: float comparison via awk exit status. op is
	// a compiler-chosen fixed token (lt/le/gt/ge/eq/ne) passed via -v, so the awk
	// program text stays constant; operands pass via -v too. Result is captured
	// into the true/false bool repr.
	Scmp: {
		id:    Scmp,
		order: 26,
		src: `__wisp_scmp() {
	if LC_ALL=C __wisp_scmp_a="$1" __wisp_scmp_b="$2" awk 'BEGIN { exit !(ENVIRON["__wisp_scmp_a"] < ENVIRON["__wisp_scmp_b"]) }'; then
		__ret=true
	else
		__ret=false
	fi
}`,
	},

	// __wisp_b2i <b>: map bool text (true/false, type-guaranteed) to 1/0 into __ret,
	// so bool ordering reuses the numeric integer compare. Pure POSIX test, no
	// bashisms; byte-identical across dash, busybox ash, bash, and zsh.
	B2i: {
		id:    B2i,
		order: 26,
		src: `__wisp_b2i() {
	if [ "$1" = true ]; then
		__ret=1
	else
		__ret=0
	fi
}`,
	},

	// __wisp_parse_args <dictId> <swId> <posId> <argsId> <vfId>: scan the args
	// array, classify each token, and fill the three pre-allocated result handles
	// (the values dict, the switches array, the positionals array). Codegen has
	// already alloc'd the handles and initialized __wisp_d_<dictId>_keys to ''.
	//
	// Parse rules (spec R1-R5 + R3 precedence):
	//   * a token is a FLAG iff it starts with '-' and is longer than one char
	//     (a lone '-' is a positional);
	//   * '--' terminates option parsing (dropped; the rest are positionals)
	//     UNLESS it is consumed as a preceding space-form value-flag's value;
	//   * a flag whose name is in value_flags takes `name=value` (split on the
	//     FIRST '='; empty 'name=' -> present empty value) or, bare, consumes the
	//     NEXT token unconditionally (even '--'/flag-shaped); at end-of-args it is
	//     omitted; LAST occurrence wins (the entry var is overwritten);
	//   * any other flag is a switch (deduped first-seen by the WHOLE token
	//     string; bare and '='-form are distinct);
	//   * everything else is a positional, order preserved.
	//
	// SAFETY: flag NAMES route through __wisp_dkey_enc -> "k<hex>" tokens for the
	// value_flags membership test, the switch dedup list, and the dict entry/key
	// names, so a flag name of any bytes is a safe identifier and the
	// space-bounded `case` membership idiom never mis-splits. Flag VALUES and
	// positionals are stored verbatim through a deferred `\$var` eval, never
	// re-evaluated or globbed (N1). Total: no fault path.
	ParseArgs: {
		id:    ParseArgs,
		deps:  []string{DictEnc},
		order: 95,
		src: `__wisp_parse_args() {
	__pa_d="$1"
	__pa_sw="$2"
	__pa_po="$3"
	__pa_args="$4"
	__pa_vf="$5"
	eval "__ret=\$__wisp_a_${__pa_vf}_len"
	__pa_vlen="$__ret"
	__pa_vlist=" "
	__pa_i=0
	while [ "$__pa_i" -lt "$__pa_vlen" ]; do
		eval "__ret=\$__wisp_a_${__pa_vf}_${__pa_i}"
		__wisp_dkey_enc "$__ret"
		__pa_vlist="$__pa_vlist$__ret "
		__pa_i=$(( __pa_i + 1 ))
	done
	__pa_swlist=" "
	__pa_swn=0
	__pa_pon=0
	eval "__ret=\$__wisp_a_${__pa_args}_len"
	__pa_alen="$__ret"
	__pa_term=0
	__pa_i=0
	while [ "$__pa_i" -lt "$__pa_alen" ]; do
		eval "__ret=\$__wisp_a_${__pa_args}_${__pa_i}"
		__pa_tok="$__ret"
		if [ "$__pa_term" -eq 1 ]; then
			eval "__wisp_a_${__pa_po}_${__pa_pon}=\$__pa_tok"
			__pa_pon=$(( __pa_pon + 1 ))
			__pa_i=$(( __pa_i + 1 ))
			continue
		fi
		case "$__pa_tok" in
			-?*) : ;;
			*)
				eval "__wisp_a_${__pa_po}_${__pa_pon}=\$__pa_tok"
				__pa_pon=$(( __pa_pon + 1 ))
				__pa_i=$(( __pa_i + 1 ))
				continue ;;
		esac
		if [ "$__pa_tok" = "--" ]; then
			__pa_term=1
			__pa_i=$(( __pa_i + 1 ))
			continue
		fi
		__pa_name="${__pa_tok%%=*}"
		case "$__pa_tok" in
			*=*) __pa_haseq=1 ;;
			*) __pa_haseq=0 ;;
		esac
		__wisp_dkey_enc "$__pa_name"
		__pa_nt="$__ret"
		case "$__pa_vlist" in
			*" $__pa_nt "*) __pa_isval=1 ;;
			*) __pa_isval=0 ;;
		esac
		if [ "$__pa_isval" -eq 1 ]; then
			if [ "$__pa_haseq" -eq 1 ]; then
				__pa_val="${__pa_tok#*=}"
			else
				__pa_j=$(( __pa_i + 1 ))
				if [ "$__pa_j" -ge "$__pa_alen" ]; then
					__pa_i="$__pa_j"
					continue
				fi
				eval "__ret=\$__wisp_a_${__pa_args}_${__pa_j}"
				__pa_val="$__ret"
				__pa_i="$__pa_j"
			fi
			eval "__wisp_d_${__pa_d}_${__pa_nt}=\$__pa_val"
			eval "__ret=\$__wisp_d_${__pa_d}_keys"
			case " $__ret " in
				*" $__pa_nt "*) : ;;
				*) eval "__wisp_d_${__pa_d}_keys=\"\$__ret $__pa_nt\"" ;;
			esac
			__pa_i=$(( __pa_i + 1 ))
			continue
		fi
		__wisp_dkey_enc "$__pa_tok"
		__pa_st="$__ret"
		case "$__pa_swlist" in
			*" $__pa_st "*) : ;;
			*)
				eval "__wisp_a_${__pa_sw}_${__pa_swn}=\$__pa_tok"
				__pa_swn=$(( __pa_swn + 1 ))
				__pa_swlist="$__pa_swlist$__pa_st " ;;
		esac
		__pa_i=$(( __pa_i + 1 ))
	done
	eval "__wisp_a_${__pa_sw}_len=\$__pa_swn"
	eval "__wisp_a_${__pa_po}_len=\$__pa_pon"
}`,
	},

	TrimStart: {
		id:    TrimStart,
		order: 67,
		src: `__wisp_trim_start() {
	__ts_s="$1"
	__ts_tab="$(printf '\t')"
	__ts_cr="$(printf '\r')"
	while [ -n "$__ts_s" ]; do
		__ts_c="${__ts_s%"${__ts_s#?}"}"
		case "$__ts_c" in
			' ' | "$__ts_tab" | '
' | "$__ts_cr") __ts_s="${__ts_s#?}" ;;
			*) break ;;
		esac
	done
	__ret="$__ts_s"
}`,
	},

	TrimEnd: {
		id:    TrimEnd,
		order: 68,
		src: `__wisp_trim_end() {
	__te_s="$1"
	__te_tab="$(printf '\t')"
	__te_cr="$(printf '\r')"
	while [ -n "$__te_s" ]; do
		__te_c="${__te_s#"${__te_s%?}"}"
		case "$__te_c" in
			' ' | "$__te_tab" | '
' | "$__te_cr") __te_s="${__te_s%?}" ;;
			*) break ;;
		esac
	done
	__ret="$__te_s"
}`,
	},

	TrimPrefix: {
		id:    TrimPrefix,
		order: 69,
		src: `__wisp_trim_prefix() {
	__ret="${1#"$2"}"
}`,
	},

	TrimSuffix: {
		id:    TrimSuffix,
		order: 70,
		src: `__wisp_trim_suffix() {
	__ret="${1%"$2"}"
}`,
	},

	IsEmpty: {
		id:    IsEmpty,
		order: 71,
		src: `__wisp_is_empty() {
	if [ -z "$1" ]; then __ret=true; else __ret=false; fi
}`,
	},

	// last_index_of <s> <sub>: byte index of the rightmost literal occurrence, or
	// -1; empty sub -> byte length(s). Byte-oriented via LC_ALL=C awk (a left-to-
	// right index() scan that records the last match offset), identical on all
	// four shells; a shell byte scan would step codepoints under a UTF-8 locale.
	LastIndexOf: {
		id:    LastIndexOf,
		order: 72,
		src: `__wisp_last_index_of() {
	__ret="$(__wisp_li_s="$1" __wisp_li_n="$2" LC_ALL=C awk 'BEGIN{
	s=ENVIRON["__wisp_li_s"]; nd=ENVIRON["__wisp_li_n"]
	if (nd=="") { print length(s); exit }
	last=-1; off=1
	while ((p=index(substr(s,off),nd))>0) { last=off+p-2; off=off+p }
	print last
}')"
}`,
	},

	// count <pos> <s> <sub>: non-overlapping byte-literal occurrence count; empty
	// sub aborts located. The empty-sub abort stays in shell (the located
	// __wisp_fail); the counting is byte-oriented via LC_ALL=C awk index()/substr,
	// so the result is identical on all four shells (a shell byte scan would step
	// codepoints under a UTF-8 locale).
	Count: {
		id:    Count,
		deps:  []string{Fail},
		order: 73,
		src: `__wisp_count() {
	if [ -z "$3" ]; then
		__wisp_fail "$1" "count(): empty search string"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__ret="$(__wisp_ct_s="$2" __wisp_ct_n="$3" LC_ALL=C awk 'BEGIN{
	s=ENVIRON["__wisp_ct_s"]; nd=ENVIRON["__wisp_ct_n"]
	n=0; off=1; sl=length(nd)
	while ((p=index(substr(s,off),nd))>0) { n++; off=off+p-1+sl }
	print n
}')"
}`,
	},

	ReplaceFirst: {
		id:    ReplaceFirst,
		deps:  []string{Fail},
		order: 74,
		src: `__wisp_replace_first() {
	if [ -z "$3" ]; then
		__wisp_fail "$1" "replace_first(): empty search string"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__rf_r="$(__wisp_rf_s="$2" __wisp_rf_n="$3" __wisp_rf_w="$4" LC_ALL=C awk 'BEGIN{
	s=ENVIRON["__wisp_rf_s"]; nd=ENVIRON["__wisp_rf_n"]; w=ENVIRON["__wisp_rf_w"]
	p=index(s,nd)
	if (p==0) { printf "%sx", s } else { printf "%sx", substr(s,1,p-1) w substr(s,p+length(nd)) }
}')"
	__ret="${__rf_r%x}"
}`,
	},

	Substring: {
		id:    Substring,
		deps:  []string{Fail},
		order: 75,
		src: `__wisp_substring() {
	__sb_len="$(__wisp_sb_s="$2" LC_ALL=C awk 'BEGIN{ print length(ENVIRON["__wisp_sb_s"]) }')"
	if [ "$3" -lt 0 ] || [ "$4" -lt "$3" ] || [ "$4" -gt "$__sb_len" ]; then
		__wisp_fail "$1" "substring(): range out of bounds"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__sb_r="$(__wisp_sb_s="$2" __wisp_sb_st="$3" __wisp_sb_en="$4" LC_ALL=C awk 'BEGIN{
	s=ENVIRON["__wisp_sb_s"]; st=ENVIRON["__wisp_sb_st"]+0; en=ENVIRON["__wisp_sb_en"]+0
	printf "%sx", substr(s, st+1, en-st)
}')"
	__ret="${__sb_r%x}"
}`,
	},

	CharAt: {
		id:    CharAt,
		deps:  []string{Fail},
		order: 76,
		src: `__wisp_char_at() {
	__ca_len="$(__wisp_ca_s="$2" LC_ALL=C awk 'BEGIN{ print length(ENVIRON["__wisp_ca_s"]) }')"
	if [ "$3" -lt 0 ] || [ "$3" -ge "$__ca_len" ]; then
		__wisp_fail "$1" "char_at(): index out of bounds"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__ca_r="$(__wisp_ca_s="$2" __wisp_ca_i="$3" LC_ALL=C awk 'BEGIN{
	printf "%sx", substr(ENVIRON["__wisp_ca_s"], ENVIRON["__wisp_ca_i"]+1, 1)
}')"
	__ret="${__ca_r%x}"
}`,
	},

	// pad_start <pos> <s> <width> <fill>: left-pad s with copies of fill (final
	// copy truncated at a BYTE boundary) to exactly <width> bytes; width <= byte
	// length(s) returns s unchanged; empty fill aborts located. Width, the gap, and
	// the fill truncation are byte-oriented via LC_ALL=C awk, so the result (and the
	// byte boundary the final fill copy is cut at) is identical on all four shells.
	// A shell `${#}`/`${pad#?}` scan would measure and step codepoints under bash/
	// zsh/busybox in a UTF-8 locale, diverging and possibly cutting a fill mid-rune
	// differently per shell. The empty-fill abort stays in shell; the trailing-`x`
	// sentinel preserves any trailing bytes through the `$(...)` capture.
	PadStart: {
		id:    PadStart,
		deps:  []string{Fail},
		order: 77,
		src: `__wisp_pad_start() {
	if [ -z "$4" ]; then
		__ps_len="$(__wisp_ps_s="$2" LC_ALL=C awk 'BEGIN{ print length(ENVIRON["__wisp_ps_s"]) }')"
		if [ "$3" -gt "$__ps_len" ]; then
			__wisp_fail "$1" "pad_start(): empty fill"
			[ -n "$__wisp_err_pending" ] && return
		fi
	fi
	__ps_r="$(__wisp_ps_s="$2" __wisp_ps_w="$3" __wisp_ps_f="$4" LC_ALL=C awk 'BEGIN{
	s=ENVIRON["__wisp_ps_s"]; w=ENVIRON["__wisp_ps_w"]+0; f=ENVIRON["__wisp_ps_f"]
	sl=length(s)
	if (sl>=w||w<=0) { printf "%sx", s; exit }
	gap=w-sl; pad=""
	while (length(pad)<gap) pad=pad f
	printf "%sx", substr(pad,1,gap) s
}')"
	__ret="${__ps_r%x}"
}`,
	},

	// pad_end: the right-pad mirror of pad_start; same byte-oriented LC_ALL=C awk
	// model, same empty-fill abort and trailing-`x` sentinel.
	PadEnd: {
		id:    PadEnd,
		deps:  []string{Fail},
		order: 78,
		src: `__wisp_pad_end() {
	if [ -z "$4" ]; then
		__pe_len="$(__wisp_pe_s="$2" LC_ALL=C awk 'BEGIN{ print length(ENVIRON["__wisp_pe_s"]) }')"
		if [ "$3" -gt "$__pe_len" ]; then
			__wisp_fail "$1" "pad_end(): empty fill"
			[ -n "$__wisp_err_pending" ] && return
		fi
	fi
	__pe_r="$(__wisp_pe_s="$2" __wisp_pe_w="$3" __wisp_pe_f="$4" LC_ALL=C awk 'BEGIN{
	s=ENVIRON["__wisp_pe_s"]; w=ENVIRON["__wisp_pe_w"]+0; f=ENVIRON["__wisp_pe_f"]
	sl=length(s)
	if (sl>=w||w<=0) { printf "%sx", s; exit }
	gap=w-sl; pad=""
	while (length(pad)<gap) pad=pad f
	printf "%sx", s substr(pad,1,gap)
}')"
	__ret="${__pe_r%x}"
}`,
	},

	Lines: {
		id:    Lines,
		order: 79,
		src: `__wisp_lines() {
	__ln_id="$1"
	__ln_s="$2"
	if [ -z "$__ln_s" ]; then eval "__wisp_a_${__ln_id}_len=0"; return; fi
	__ln_nl='
'
	__ln_rest="$__ln_s"
	__ln_cur=""
	__ln_n=0
	while [ -n "$__ln_rest" ]; do
		__ln_after="${__ln_rest#"$__ln_nl"}"
		if [ "$__ln_after" != "$__ln_rest" ]; then
			eval "__wisp_a_${__ln_id}_${__ln_n}=\$__ln_cur"
			__ln_n=$(( __ln_n + 1 ))
			__ln_cur=""
			__ln_rest="$__ln_after"
		else
			__ln_first="${__ln_rest%"${__ln_rest#?}"}"
			__ln_cur="$__ln_cur$__ln_first"
			__ln_rest="${__ln_rest#?}"
		fi
	done
	eval "__wisp_a_${__ln_id}_${__ln_n}=\$__ln_cur"
	__ln_n=$(( __ln_n + 1 ))
	if [ "${__ln_s%"$__ln_nl"}" != "$__ln_s" ]; then __ln_n=$(( __ln_n - 1 )); fi
	eval "__wisp_a_${__ln_id}_len=\$__ln_n"
}`,
	},

	Sqrt: {
		id:    Sqrt,
		deps:  []string{Fail},
		order: 81,
		// Newton's method using ONLY +,-,*,/ (busybox awk is commonly built WITHOUT
		// math support, so awk's sqrt()/^ are unavailable). A negative input prints
		// "nan", which the shell case-glob below turns into a located abort. The seed
		// is magnitude-scaled (5 * 10^k where x ~ [1,100)*100^k) so a fixed iteration
		// count converges across the whole float range; Newton then iterates on the
		// EXACT x, so a perfect square (e.g. sqrt(144)=12) returns exactly.
		src: `__wisp_sqrt() {
	__sq_r="$(LC_ALL=C awk -v a="$2" 'BEGIN{
		x = a + 0
		if (x < 0) { print "nan"; exit }
		if (x == 0) { printf "%.17g", 0; exit }
		# +inf guard: a float literal with a huge integer part overflows to +inf
		# in awk. For x > 0, only inf satisfies x == x/2; without this the
		# normalization loop below (y = y / 100) would never terminate. Print a
		# non-numeric token so the case-glob aborts located instead of hanging.
		if (x == x / 2) { print "inf"; exit }
		y = x; k = 0
		while (y >= 100) { y = y / 100; k = k + 1 }
		while (y < 1)    { y = y * 100; k = k - 1 }
		g = 5
		j = k
		while (j > 0) { g = g * 10; j = j - 1 }
		while (j < 0) { g = g / 10; j = j + 1 }
		i = 0
		while (i < 80) { g = (g + x / g) / 2; i = i + 1 }
		printf "%.17g", g
	}')"
	__sq_b="$__sq_r"
	case "$__sq_r" in -* | +*) __sq_b="${__sq_r#?}" ;; esac
	case "$__sq_b" in
		'' | *[!0-9.]* | .* | *. | *.*.*)
			__wisp_fail "$1" "sqrt(): non-finite or undefined result"
			[ -n "$__wisp_err_pending" ] && return ;;
	esac
	__ret="$__sq_r"
}`,
	},

	// __wisp_format_float <pos> <x> <decimals>: render x to `decimals` digits via
	// awk printf, building the conversion as `"%." d "f"` (awk string concat).
	// $1=pos, $2=x, $3=decimals. NOTE: the `*`-precision form `printf "%.*f", d, x`
	// is NOT portable -- busybox awk rejects it ("%*x formats are not supported"),
	// caught by the act golden; the d-concatenated literal conversion works on
	// gawk/mawk/busybox/BSD. x/decimals reach awk via -v (the program is constant).
	// A negative decimals aborts located before awk. A non-finite x (reachable via a
	// huge plain-decimal literal that awk overflows to inf) is caught by REUSING
	// __wisp_ffinite on the printf output. Rounding is the platform printf's
	// (round-half-to-even on most libc), not guaranteed half-up.
	FormatFloat: {
		id:    FormatFloat,
		deps:  []string{Fail, FFinite},
		order: 82,
		src: `__wisp_format_float() {
	if [ "$3" -lt 0 ]; then
		__wisp_fail "$1" "format_float: decimals must be >= 0"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__wisp_ff_r="$(LC_ALL=C awk -v x="$2" -v d="$3" 'BEGIN{ printf "%." d "f", (x+0) }')"
	__wisp_ffinite "$1" "$__wisp_ff_r"
	[ -n "$__wisp_err_pending" ] && return
	__ret="$__wisp_ff_r"
}`,
	},

	Floor: {
		id:    Floor,
		deps:  []string{Int},
		order: 82,
		src: `__wisp_floor() {
	__fl_t="$(awk -v a="$2" 'BEGIN{ x=a+0; r=int(x); if (x<0 && x!=r) r=r-1; printf "%d", r }')"
	__wisp_int "$1" "$__fl_t"
}`,
	},

	Ceil: {
		id:    Ceil,
		deps:  []string{Int},
		order: 83,
		src: `__wisp_ceil() {
	__cl_t="$(awk -v a="$2" 'BEGIN{ x=a+0; r=int(x); if (x>0 && x!=r) r=r+1; printf "%d", r }')"
	__wisp_int "$1" "$__cl_t"
}`,
	},

	Round: {
		id:    Round,
		deps:  []string{Int},
		order: 84,
		src: `__wisp_round() {
	__rd_t="$(awk -v a="$2" 'BEGIN{ y=(a+0)+0.5; r=int(y); if (y<0 && y!=r) r=r-1; printf "%d", r }')"
	__wisp_int "$1" "$__rd_t"
}`,
	},

	// __wisp_abs_int <pos> <x>: |x| for an int. INT_MIN (the most-negative value)
	// has no representable positive counterpart, so `$(( 0 - x ))` would overflow
	// (and zsh silently mis-evaluates a literal/value of that magnitude); abs of it
	// aborts located, mirroring division by zero. INT_MIN is detected by comparing
	// against the runtime-computed minimum (the int_min doubling model) rather than
	// a hardcoded literal, which zsh cannot parse. Any other x negates in-range.
	AbsInt: {
		id:    AbsInt,
		deps:  []string{Fail},
		order: 84,
		src: `__wisp_abs_int() {
	if [ "$2" -ge 0 ]; then __ret="$2"; return; fi
	__ai_m=1
	while [ $(( __ai_m + __ai_m )) -gt 0 ]; do __ai_m=$(( __ai_m + __ai_m )); done
	__ai_min=$(( - (__ai_m - 1) - __ai_m - 1 ))
	if [ "$2" -eq "$__ai_min" ]; then
		__wisp_fail "$1" "abs(): integer overflow"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__ret=$(( 0 - $2 ))
}`,
	},

	// __wisp_idiv_ovf <pos> <num> <den>: located abort when num/den would overflow,
	// i.e. num == INT_MIN && den == -1 (quotient 2^63 is unrepresentable, and the
	// $(( )) itself is a fatal exception on some shells). Otherwise a no-op. Called
	// by codegen before the `/` arithmetic, after the zero guard. INT_MIN is the
	// runtime-computed minimum (zsh cannot parse the literal). `%` uses IMin +
	// codegen's mod guard instead (INT_MIN % -1 is representable as 0 but x86
	// idiv still traps on it).
	IDivOvf: {
		id:    IDivOvf,
		deps:  []string{Fail},
		order: 84,
		src: `__wisp_idiv_ovf() {
	[ "$3" -eq -1 ] || return
	__do_m=1
	while [ $(( __do_m + __do_m )) -gt 0 ]; do __do_m=$(( __do_m + __do_m )); done
	__do_min=$(( - (__do_m - 1) - __do_m - 1 ))
	if [ "$2" -eq "$__do_min" ]; then
		__wisp_fail "$1" "division overflow"
		[ -n "$__wisp_err_pending" ] && return
	fi
}`,
	},

	// __wisp_imin: set __ret to INT_MIN (-2^63). Computed by bit-doubling up to
	// the largest positive power of two (2^62), then -(m-1)-m-1 = -2^63, so no
	// 19-digit literal is ever parsed (zsh cannot). Used by codegen's `%` overflow
	// guard to compare a dividend against INT_MIN. No deps; sets __ret only.
	IMin: {
		id:    IMin,
		order: 84,
		src: `__wisp_imin() {
	__im_m=1
	while [ $(( __im_m + __im_m )) -gt 0 ]; do __im_m=$(( __im_m + __im_m )); done
	__ret=$(( - (__im_m - 1) - __im_m - 1 ))
}`,
	},

	// __wisp_gcd <pos> <a> <b>: gcd(|a|, |b|), non-negative; gcd(0,0)=0. INT_MIN
	// poisons the result: |INT_MIN| = 2^63 is not representable, and zsh cannot do
	// 64-bit arithmetic on a value of that magnitude inside $(( )) (it truncates and
	// diverges from dash/busybox/bash). So either operand equal to INT_MIN aborts
	// located, the same defined behavior as abs(INT_MIN). This replaces the old
	// silent negate that wrapped to a wrong/negative result.
	Gcd: {
		id:    Gcd,
		deps:  []string{Fail},
		order: 85,
		src: `__wisp_gcd() {
	__gc_m=1
	while [ $(( __gc_m + __gc_m )) -gt 0 ]; do __gc_m=$(( __gc_m + __gc_m )); done
	__gc_min=$(( - (__gc_m - 1) - __gc_m - 1 ))
	if [ "$2" -eq "$__gc_min" ] || [ "$3" -eq "$__gc_min" ]; then
		__wisp_fail "$1" "gcd(): integer overflow"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__gc_a="$2"
	[ "$__gc_a" -lt 0 ] && __gc_a=$(( - __gc_a ))
	__gc_b="$3"
	[ "$__gc_b" -lt 0 ] && __gc_b=$(( - __gc_b ))
	while [ "$__gc_b" -ne 0 ]; do
		__gc_t=$(( __gc_a % __gc_b ))
		__gc_a="$__gc_b"
		__gc_b="$__gc_t"
	done
	__ret="$__gc_a"
}`,
	},

	// __wisp_lcm <pos> <a> <b>: lcm = |a / gcd(a,b) * b|, 0 if either operand is 0.
	// gcd carries the located <pos> and aborts on an INT_MIN operand (its 2^63
	// overflow), so lcm propagates that abort via the pending guard. The remaining
	// product overflow at the int boundary is the general unspecified-overflow
	// contract (see the Numbers note in the stdlib guide), not a located abort.
	Lcm: {
		id:    Lcm,
		deps:  []string{Gcd},
		order: 86,
		src: `__wisp_lcm() {
	if [ "$2" -eq 0 ] || [ "$3" -eq 0 ]; then __ret=0; return; fi
	__wisp_gcd "$1" "$2" "$3"
	[ -n "$__wisp_err_pending" ] && return
	__lc="$(( $2 / __ret * $3 ))"
	[ "$__lc" -lt 0 ] && __lc=$(( - __lc ))
	__ret="$__lc"
}`,
	},

	Pow: {
		id:    Pow,
		deps:  []string{Fail},
		order: 87,
		// Portable pow(base, exp) using only +,-,*,/ (no awk ^ or math library).
		// Special cases handled first: b^0=1, 1^e=1, 0^pos=0, 0^neg=abort.
		// Integer exponent path: repeated squaring (exact, handles negative exponent via 1/result).
		// Float exponent path: exp(e*ln(b)) via atanh-series ln and Taylor-series exp,
		// each needing only basic arithmetic. Negative base + non-integer exponent -> abort.
		// Result "nan"/"inf"/non-numeric -> located abort.
		src: `__wisp_pow() {
	__pw_r="$(LC_ALL=C awk -v pos="$1" -v b="$2" -v e="$3" 'BEGIN{
		b = b + 0; e = e + 0
		if (b == 1 || e == 0) { printf "%.17g", 1; exit }
		if (b == 0) {
			if (e > 0) { printf "%.17g", 0; exit }
			print "nan"; exit
		}
		# check if exponent is integer-valued
		ei = int(e); is_int = (e == ei)
		if (b < 0 && !is_int) { print "nan"; exit }
		# integer exponent via repeated squaring
		if (is_int) {
			neg = 0; en = ei
			if (en < 0) { neg = 1; en = -en }
			acc = 1; base = b
			while (en > 0) {
				if (en % 2 == 1) acc = acc * base
				base = base * base; en = int(en / 2)
			}
			if (neg) acc = 1 / acc
			printf "%.17g", acc; exit
		}
		# float exponent: exp(e * ln(b))
		# ln via atanh series: ln(x) = 2*atanh((x-1)/(x+1))
		# atanh(t) = t + t^3/3 + t^5/5 + ... (|t|<1); use range reduction to keep |t|<0.3
		x = b; k = 0
` + lnSeriesAwk + `		lp = e * ln_b
` + expSeriesAwk + `		printf "%.17g", acc
	}')"
	__pw_b="$__pw_r"
	case "$__pw_r" in -* | +*) __pw_b="${__pw_r#?}" ;; esac
	case "$__pw_b" in
		'' | *[!0-9.]* | .* | *. | *.*.*)
			__wisp_fail "$1" "pow(): non-finite or undefined result"
			[ -n "$__wisp_err_pending" ] && return ;;
	esac
	__ret="$__pw_r"
}`,
	},

	// __wisp_ln <pos> <x> -> ln(x). Pure-arithmetic atanh series (the shared
	// lnSeriesAwk fragment also used by __wisp_pow); NO awk log (busybox awk has
	// no libm). A non-positive input is guarded BEFORE the range-reduction loop:
	// while (x < 0.6) x = x*2 never terminates for x == 0 (hang) and diverges for
	// x < 0, so x <= 0 prints "nan" and exits the awk immediately -- the case-glob
	// below then aborts located, exactly like sqrt(-1), without ever entering the
	// loop. ln(1.0) is exactly 0 (t == 0 -> series == 0, k == 0).
	Ln: {
		id:    Ln,
		deps:  []string{Fail},
		order: 87,
		src: `__wisp_ln() {
	__ln_r="$(LC_ALL=C awk -v a="$2" 'BEGIN{
		x = a + 0
		if (x <= 0) { print "nan"; exit }
		k = 0
` + lnSeriesAwk + `		printf "%.17g", ln_b
	}')"
	__ln_b="$__ln_r"
	case "$__ln_r" in -* | +*) __ln_b="${__ln_r#?}" ;; esac
	case "$__ln_b" in
		'' | *[!0-9.]* | .* | *. | *.*.*)
			__wisp_fail "$1" "ln(): non-finite or undefined result"
			[ -n "$__wisp_err_pending" ] && return ;;
	esac
	__ret="$__ln_r"
}`,
	},

	// __wisp_exp <pos> <x> -> e^x. Pure-arithmetic Taylor series (the shared
	// expSeriesAwk fragment also used by __wisp_pow); NO awk exp (busybox awk has
	// no libm). NEGATIVE-x reciprocal: the alternating Taylor series suffers
	// catastrophic cancellation for large-negative x (the terms alternate sign and
	// grow before they decay, so e.g. exp(-40) returns a wrong, even negative,
	// finite-looking value). So negative x is computed as 1/exp(|x|): run the
	// series on |x| (all-positive terms, well-conditioned), then reciprocate.
	// OVERFLOW guard: a too-large |x| drives acc to +inf during the accumulation;
	// the post-series acc == acc/2 test (true ONLY for +inf, mirroring
	// __wisp_sqrt's inf-detection) prints the token "inf" so the case-glob aborts
	// located reliably across all four awks. A finite-but-non-representable result
	// (e.g. exp(-40) ~= 4.2e-18, or exp(100) ~= 2.7e43) renders in exponent
	// notation, which the case-glob also rejects -> located abort, preserving the
	// always-finite-and-representable float invariant. exp(0.0) == 1 (the series'
	// first term, no reciprocal).
	Exp: {
		id:    Exp,
		deps:  []string{Fail},
		order: 87,
		src: `__wisp_exp() {
	__ex_r="$(LC_ALL=C awk -v a="$2" 'BEGIN{
		lp = a + 0
		neg = 0
		if (lp < 0) { neg = 1; lp = -lp }
` + expSeriesAwk + `		if (acc == acc / 2 && acc != 0) { print "inf"; exit }
		if (neg) acc = 1 / acc
		printf "%.17g", acc
	}')"
	__ex_b="$__ex_r"
	case "$__ex_r" in -* | +*) __ex_b="${__ex_r#?}" ;; esac
	case "$__ex_b" in
		'' | *[!0-9.]* | .* | *. | *.*.*)
			__wisp_fail "$1" "exp(): non-finite or undefined result"
			[ -n "$__wisp_err_pending" ] && return ;;
	esac
	__ret="$__ex_r"
}`,
	},

	// __wisp_log10 <pos> <x> -> ln(x)/ln(10); __wisp_log2 <pos> <x> -> ln(x)/ln(2).
	// Standalone helpers (not just inline codegen) so log10/log2 are referenceable
	// as funcref values; the direct-call codegen path (genLogBase) also routes
	// through these, so there is exactly one lowering per builtin. ln(10) and
	// ln(2) are the same baked float literals genLogBase used inline.
	Log10: {
		id:    Log10,
		deps:  []string{Ln, FDiv},
		order: 87,
		src: `__wisp_log10() {
	__wisp_ln "$1" "$2"
	[ -n "$__wisp_err_pending" ] && return
	__l10_x="$__ret"
	__wisp_fdiv "$1" "$__l10_x" "2.302585092994046"
}`,
	},

	Log2: {
		id:    Log2,
		deps:  []string{Ln, FDiv},
		order: 87,
		src: `__wisp_log2() {
	__wisp_ln "$1" "$2"
	[ -n "$__wisp_err_pending" ] && return
	__l2_x="$__ret"
	__wisp_fdiv "$1" "$__l2_x" "0.6931471805599453"
}`,
	},

	// __wisp_min_int/__wisp_max_int and __wisp_min_float/__wisp_max_float pick an
	// operand UNCHANGED (no arithmetic/awk reformat), mirroring genMinMax. Both
	// are total: min/max never abort. Standalone (not just inline codegen) so
	// min/max are referenceable as funcref values (one arm per numeric type).
	MinInt: {
		id:    MinInt,
		order: 87,
		src: `__wisp_min_int() {
	if [ "$1" -le "$2" ]; then __ret="$1"; else __ret="$2"; fi
}`,
	},

	MaxInt: {
		id:    MaxInt,
		order: 87,
		src: `__wisp_max_int() {
	if [ "$1" -ge "$2" ]; then __ret="$1"; else __ret="$2"; fi
}`,
	},

	MinFloat: {
		id:    MinFloat,
		deps:  []string{FCmp},
		order: 87,
		src: `__wisp_min_float() {
	__wisp_fcmp - le "$1" "$2"
	if [ "$__ret" = true ]; then __ret="$1"; else __ret="$2"; fi
}`,
	},

	MaxFloat: {
		id:    MaxFloat,
		deps:  []string{FCmp},
		order: 87,
		src: `__wisp_max_float() {
	__wisp_fcmp - ge "$1" "$2"
	if [ "$__ret" = true ]; then __ret="$1"; else __ret="$2"; fi
}`,
	},

	// __wisp_clamp_int/__wisp_clamp_float lower clamp(x, lo, hi) = max(lo,
	// min(x, hi)), mirroring genClamp. The contract requires lo <= hi (a
	// documented precondition, not a runtime check); both are total.
	ClampInt: {
		id:    ClampInt,
		order: 87,
		src: `__wisp_clamp_int() {
	if [ "$1" -le "$3" ]; then __ci_m="$1"; else __ci_m="$3"; fi
	if [ "$2" -ge "$__ci_m" ]; then __ret="$2"; else __ret="$__ci_m"; fi
}`,
	},

	ClampFloat: {
		id:    ClampFloat,
		deps:  []string{FCmp},
		order: 87,
		src: `__wisp_clamp_float() {
	__wisp_fcmp - le "$1" "$3"
	if [ "$__ret" = true ]; then __cf_m="$1"; else __cf_m="$3"; fi
	__wisp_fcmp - ge "$2" "$__cf_m"
	if [ "$__ret" = true ]; then __ret="$2"; else __ret="$__cf_m"; fi
}`,
	},

	// __wisp_sign_int/__wisp_sign_float lower sign(x) -> -1/0/1, mirroring
	// genSign. Both are total.
	SignInt: {
		id:    SignInt,
		order: 87,
		src: `__wisp_sign_int() {
	if [ "$1" -lt 0 ]; then __ret=-1; elif [ "$1" -gt 0 ]; then __ret=1; else __ret=0; fi
}`,
	},

	SignFloat: {
		id:    SignFloat,
		deps:  []string{FCmp},
		order: 87,
		src: `__wisp_sign_float() {
	__wisp_fcmp - lt "$1" 0
	__sf_lt="$__ret"
	__wisp_fcmp - gt "$1" 0
	if [ "$__sf_lt" = true ]; then __ret=-1; elif [ "$__ret" = true ]; then __ret=1; else __ret=0; fi
}`,
	},

	IntMax: {
		id:    IntMax,
		order: 88,
		// Detect the shell's max signed integer at runtime by doubling until overflow.
		// POSIX guarantees at least 32-bit; in practice 64-bit. No dependencies.
		src: `__wisp_int_max() {
	__im_m=1
	while [ $(( __im_m + __im_m )) -gt 0 ]; do __im_m=$(( __im_m + __im_m )); done
	__im_r=$(( __im_m - 1 ))
	__ret=$(( __im_r + __im_m ))
}`,
	},

	IntMin: {
		id:    IntMin,
		order: 89,
		src: `__wisp_int_min() {
	__imn_m=1
	while [ $(( __imn_m + __imn_m )) -gt 0 ]; do __imn_m=$(( __imn_m + __imn_m )); done
	__imn_r=$(( __imn_m - 1 ))
	__imn_x=$(( __imn_r + __imn_m ))
	__ret=$(( - __imn_x - 1 ))
}`,
	},

	IntOr: {
		id:    IntOr,
		order: 87,
		src: `__wisp_int_or() {
	__io_s="$1"
	__io_body="$__io_s"
	__io_neg=0
	case "$__io_s" in
		-*) __io_body="${__io_s#-}"; __io_neg=1 ;;
		+*) __io_body="${__io_s#+}" ;;
	esac
	case "$__io_body" in
		'' | *[!0-9]*) __ret="$2"; return ;;
	esac
	__io_mag="$__io_body"
	while :; do
		case "$__io_mag" in
			0?*) __io_mag="${__io_mag#0}" ;;
			*) break ;;
		esac
	done
	if [ "$__io_neg" -eq 1 ]; then
		__io_bound=9223372036854775808
	else
		__io_bound=9223372036854775807
	fi
	if [ "${#__io_mag}" -gt "${#__io_bound}" ]; then __ret="$2"; return; fi
	if [ "${#__io_mag}" -eq "${#__io_bound}" ]; then
		__io_x="$__io_mag"
		__io_y="$__io_bound"
		while [ -n "$__io_x" ]; do
			__io_dx="${__io_x%"${__io_x#?}"}"
			__io_dy="${__io_y%"${__io_y#?}"}"
			if [ "$__io_dx" -gt "$__io_dy" ]; then __ret="$2"; return; fi
			if [ "$__io_dx" -lt "$__io_dy" ]; then break; fi
			__io_x="${__io_x#?}"
			__io_y="${__io_y#?}"
		done
	fi
	if [ "$__io_neg" -eq 1 ] && [ "$__io_mag" != 0 ]; then
		__ret="-$__io_mag"
	else
		__ret="$__io_mag"
	fi
}`,
	},

	ParseInt: {
		id:    ParseInt,
		order: 89,
		src: `__wisp_parse_int() {
	__pi_s="$1"
	__pi_body="$__pi_s"
	__pi_neg=0
	case "$__pi_s" in
		-*) __pi_body="${__pi_s#-}"; __pi_neg=1 ;;
		+*) __pi_body="${__pi_s#+}" ;;
	esac
	case "$__pi_body" in
		'' | *[!0-9]*) return 1 ;;
	esac
	__pi_mag="$__pi_body"
	while :; do
		case "$__pi_mag" in
			0?*) __pi_mag="${__pi_mag#0}" ;;
			*) break ;;
		esac
	done
	if [ "$__pi_neg" -eq 1 ]; then
		__pi_bound=9223372036854775808
	else
		__pi_bound=9223372036854775807
	fi
	if [ "${#__pi_mag}" -gt "${#__pi_bound}" ]; then return 1; fi
	if [ "${#__pi_mag}" -eq "${#__pi_bound}" ]; then
		__pi_x="$__pi_mag"
		__pi_y="$__pi_bound"
		while [ -n "$__pi_x" ]; do
			__pi_dx="${__pi_x%"${__pi_x#?}"}"
			__pi_dy="${__pi_y%"${__pi_y#?}"}"
			if [ "$__pi_dx" -gt "$__pi_dy" ]; then return 1; fi
			if [ "$__pi_dx" -lt "$__pi_dy" ]; then break; fi
			__pi_x="${__pi_x#?}"
			__pi_y="${__pi_y#?}"
		done
	fi
	if [ "$__pi_neg" -eq 1 ] && [ "$__pi_mag" != 0 ]; then
		__ret="-$__pi_mag"
	else
		__ret="$__pi_mag"
	fi
	return 0
}`,
	},

	ParseFloat: {
		id:    ParseFloat,
		order: 90,
		src: `__wisp_parse_float() {
	__pf_body="$1"
	case "$1" in -* | +*) __pf_body="${1#?}" ;; esac
	case "$__pf_body" in
		'' | *[!0-9.]* | .* | *. | *.*.*) return 1 ;;
	esac
	__pf_c="$(LC_ALL=C awk -v a="$1" 'BEGIN{ printf "%.17g", a+0 }')"
	__pf_cb="$__pf_c"
	case "$__pf_c" in -* | +*) __pf_cb="${__pf_c#?}" ;; esac
	case "$__pf_cb" in
		'' | *[!0-9.]* | .* | *. | *.*.*) return 1 ;;
	esac
	__ret="$__pf_c"
	return 0
}`,
	},

	FloatOr: {
		id:    FloatOr,
		order: 88,
		src: `__wisp_float_or() {
	__fo_body="$1"
	case "$1" in -* | +*) __fo_body="${1#?}" ;; esac
	case "$__fo_body" in
		'' | *[!0-9.]* | .* | *. | *.*.*) __ret="$2"; return ;;
	esac
	__fo_c="$(LC_ALL=C awk -v a="$1" 'BEGIN{ printf "%.17g", a+0 }')"
	__fo_cb="$__fo_c"
	case "$__fo_c" in -* | +*) __fo_cb="${__fo_c#?}" ;; esac
	case "$__fo_cb" in
		'' | *[!0-9.]* | .* | *. | *.*.*) __ret="$2"; return ;;
	esac
	__ret="$__fo_c"
}`,
	},

	FCmp: {
		id:    FCmp,
		order: 25,
		src: `__wisp_fcmp() {
	if awk -v op="$2" -v a="$3" -v b="$4" 'BEGIN{
		x=a+0; y=b+0;
		if (op=="lt") r=(x<y);
		else if (op=="le") r=(x<=y);
		else if (op=="gt") r=(x>y);
		else if (op=="ge") r=(x>=y);
		else if (op=="eq") r=(x==y);
		else r=(x!=y);
		exit !r
	}'; then
		__ret=true
	else
		__ret=false
	fi
}`,
	},

	// __wisp_fbool <pos> <value>: bool(float) tests numeric zero via awk so a
	// computed -0.0 / 0.000 is correctly false (NOT a string match against "0").
	FBool: {
		id:    FBool,
		order: 26,
		src: `__wisp_fbool() {
	if awk -v a="$2" 'BEGIN{ exit !((a+0)==0) }'; then
		__ret=false
	else
		__ret=true
	fi
}`,
	},

	// __wisp_fstr <pos> <value>: string(float) -> the canonical %.17g decimal
	// (spec 3.3/3.4). A float value already satisfies the invariant, so the
	// canonicalization never produces inf/nan; a magnitude needing exponent form
	// is impossible here because such a value could not have entered the float
	// space (it would have aborted at its producing op).
	FStr: {
		id:    FStr,
		order: 27,
		src: `__wisp_fstr() {
	__ret="$(LC_ALL=C awk -v a="$2" 'BEGIN{ printf "%.17g", (a+0) }')"
}`,
	},

	// __wisp_ffloat_i <pos> <intval>: float(int). The int is invariant-valid by
	// construction; canonicalize to %.17g and validate finiteness (a huge int
	// whose %.17g needs exponent notation aborts located -- it is outside the
	// representable float domain per 3.6).
	FFloatI: {
		id:    FFloatI,
		deps:  []string{FFinite},
		order: 28,
		src: `__wisp_ffloat_i() {
	__f_r="$(LC_ALL=C awk -v a="$2" 'BEGIN{ printf "%.17g", (a+0) }')"
	__wisp_ffinite "$1" "$__f_r"
}`,
	},

	// __wisp_ffloat_s <pos> <str>: float(string). VALIDATE the raw string via a
	// `case` glob (the M1 safe-match mechanism) BEFORE it is passed to awk via
	// -v, so a bad string aborts located first and only an invariant-valid value
	// ever reaches awk; then canonicalize to %.17g (and re-validate finiteness,
	// catching an in-range-looking-but-huge magnitude). The accepted domain is
	// [+-]?[0-9]+(\.[0-9]+)? (spec 3.4).
	FFloatS: {
		id:    FFloatS,
		deps:  []string{FFinite, Fail},
		order: 29,
		src: `__wisp_ffloat_s() {
	__f_body="$2"
	case "$2" in
		-* | +*) __f_body="${2#?}" ;;
	esac
	case "$__f_body" in
		'' | *[!0-9.]* | .* | *. | *.*.*)
			__wisp_fail "$1" "to_float(): not a float: \"$2\"" ;;
	esac
	[ -n "$__wisp_err_pending" ] && return
	__f_r="$(LC_ALL=C awk -v a="$2" 'BEGIN{ printf "%.17g", (a+0) }')"
	__wisp_ffinite "$1" "$__f_r"
}`,
	},

	// __wisp_fint <pos> <value>: int(float). Truncate toward zero awk-side
	// (int(x) in awk truncates toward zero), formatted as a plain integer via
	// %d, then REUSE the existing __wisp_int validate-and-abort (lexical
	// magnitude, never $(( ))) so an out-of-range result aborts located and no
	// out-of-range digit string ever reaches arithmetic. awk %d on a magnitude
	// beyond its integer range can itself misformat, so __wisp_int is the
	// authoritative range gate.
	FIntT: {
		id:    FIntT,
		deps:  []string{Int},
		order: 30,
		src: `__wisp_fint() {
	__f_t="$(awk -v a="$2" 'BEGIN{ printf "%d", int(a+0) }')"
	__wisp_int "$1" "$__f_t"
}`,
	},

	// __wisp_fkey <value>: the canonical float DICT-KEY string. Like __wisp_fstr
	// it renders %.17g (always a plain decimal for an in-domain float), but it
	// folds the sign of zero NUMERICALLY -- a value testing equal to zero maps to
	// the single token "0" that %.17g emits for +0.0 -- so -0.0 and 0.0 produce a
	// byte-identical key on every shell regardless of how that awk renders -0.0.
	// The LC_ALL=C pin is mandatory: the %.17g emission is locale-sensitive (a
	// comma-decimal locale would emit "2,5"), and the key token must match every
	// other %.17g site byte-for-byte. Distinct from __wisp_fstr, which omits the
	// zero-fold and would split -0.0/0.0. order 31 is an unused stable-sort slot
	// (see the placement note above); a tie at 28 would sort nondeterministically
	// in the test-only IDs() helper.
	FKey: {
		id:    FKey,
		order: 31,
		src: `__wisp_fkey() {
	__ret="$(LC_ALL=C awk -v a="$1" 'BEGIN{ x=a+0; if (x==0) printf "0"; else printf "%.17g", x }')"
}`,
	},

	// --- aggregate handle runtime (M3 PR-B) ---
	//
	// __wisp_alloc: issue a fresh reference id from a monotonic counter and
	// return it in __ret. The counter defaults to 0 on first use (no separate
	// initializer line is emitted). Ids are plain non-negative decimal integers,
	// so a backing variable name __wisp_s_<id>_<field> / __wisp_a_<id>_<i> built
	// from an id is a safe identifier. Handle ids are opaque (the checker forbids
	// any int<->handle operation), so the integer shape never leaks to user math.
	Alloc: {
		id:    Alloc,
		order: 40,
		src: `__wisp_alloc() {
	__wisp_next_id=$(( ${__wisp_next_id:-0} + 1 ))
	__ret="$__wisp_next_id"
}`,
	},

	// __wisp_bounds_fail <pos> <index> <len>: a located out-of-bounds / bad-index
	// abort for array access. The index and length flow only through %s, so a
	// crafted value cannot be reinterpreted as a format (M1 9.6 invariant).
	BoundsFail: {
		id:    BoundsFail,
		deps:  []string{Fail},
		order: 41,
		src: `__wisp_bounds_fail() {
	__wisp_fail "$1" "array index out of bounds: index $2, length $3"
}`,
	},

	// --- dict runtime (M3 PR-C) ---
	//
	// __wisp_hexdig <n>: n is 0..15 (single-digit arithmetic from the encoder);
	// __ret is its lowercase hex digit. A constant `case`, never arithmetic on
	// untrusted data.
	DictHexDig: {
		id:    DictHexDig,
		order: 50,
		src: `__wisp_hexdig() {
	case "$1" in
		0) __ret=0 ;; 1) __ret=1 ;; 2) __ret=2 ;; 3) __ret=3 ;;
		4) __ret=4 ;; 5) __ret=5 ;; 6) __ret=6 ;; 7) __ret=7 ;;
		8) __ret=8 ;; 9) __ret=9 ;; 10) __ret=a ;; 11) __ret=b ;;
		12) __ret=c ;; 13) __ret=d ;; 14) __ret=e ;; 15) __ret=f ;;
	esac
}`,
	},

	// __wisp_dkey_enc <key>: encode the key's BYTES to the token "k<hex>" (two
	// lowercase hex digits per byte), returned in __ret. This is the reversible,
	// injection-safe, collision-free variable-name fragment of spec 4.1: the
	// token contains only [0-9a-fk], so __wisp_d_<id>_<token> is always a safe
	// identifier and two distinct keys never collide (distinct byte strings have
	// distinct hex). The leading "k" namespaces keys away from the _keys/_len/
	// element vars (a key "len" or "0" encodes to k6c656e / k30, never "len"/"0").
	// LC_ALL=C is scoped via `local` so the byte-at-a-time `?` expansion and the
	// printf ordinal trick operate on bytes, not locale characters.
	DictEnc: {
		id:    DictEnc,
		deps:  []string{DictHexDig},
		order: 51,
		src: `__wisp_dkey_enc() {
	local LC_ALL
	LC_ALL=C
	__de_s="$1"
	__de_o="k"
	while [ -n "$__de_s" ]; do
		__de_c="${__de_s%"${__de_s#?}"}"
		__de_s="${__de_s#?}"
		__de_n=$(printf '%d' "'$__de_c")
		__de_n=$(( __de_n & 255 ))
		__wisp_hexdig $(( __de_n / 16 ))
		__de_hi="$__ret"
		__wisp_hexdig $(( __de_n % 16 ))
		__de_o="$__de_o$__de_hi$__ret"
	done
	__ret="$__de_o"
}`,
	},

	// __wisp_dkey_dec <token>: inverse of __wisp_dkey_enc. Strip the leading "k",
	// then turn each two-hex-digit pair back into its byte via the octal printf
	// idiom (`printf "\NNN"`), returning the original bytes in __ret. The token is
	// compiler/encoder-produced ([0-9a-fk]*), so "0x$pair" is always a valid hex
	// literal. LC_ALL=C is scoped via `local` for byte-correct reconstruction.
	DictDec: {
		id:    DictDec,
		order: 52,
		src: `__wisp_dkey_dec() {
	local LC_ALL
	LC_ALL=C
	__dd_h="${1#k}"
	__dd_esc=""
	while [ -n "$__dd_h" ]; do
		__dd_p="${__dd_h%"${__dd_h#??}"}"
		__dd_h="${__dd_h#??}"
		__dd_esc="$__dd_esc\\0$(printf '%03o' "0x$__dd_p")"
	done
	__dd_o="$(printf '%b.' "$__dd_esc")"
	__ret="${__dd_o%.}"
}`,
	},

	// __wisp_dict_miss <pos> <key>: a located missing-key abort for a d[k] lookup
	// of an absent key (spec 4.4). The key flows only through %s, so it cannot be
	// reinterpreted as a format (M1 9.6 invariant).
	DictMiss: {
		id:    DictMiss,
		deps:  []string{Fail},
		order: 53,
		src: `__wisp_dict_miss() {
	__wisp_fail "$1" "dict key not found: \"$2\""
}`,
	},

	// --- core stdlib (M6 PR-A) ---

	// __wisp_split <pos> <id> <s> <sep>: split s on each non-overlapping literal
	// occurrence of sep, storing the parts as the array backing vars of handle id
	// (__wisp_a_<id>_<i>) and setting _len. Empty sep aborts located. Splitting ""
	// yields a single empty element; a trailing sep yields a trailing "". The scan
	// reuses the __wisp_replace model: ${rest#"$sep"} matches sep LITERALLY (a
	// quoted operand is never a glob), so a sep of glob metacharacters is inert.
	// id is a safe decimal alloc id, so __wisp_a_${id}_<i> is a safe identifier;
	// each element VALUE is carried through a variable and written via a deferred
	// `\$var` inside the single double-quoted eval argument (spec 9.6 invariant 7).
	Split: {
		id:    Split,
		deps:  []string{Fail},
		order: 60,
		src: `__wisp_split() {
	__sp_id="$2"
	__sp_sep="$4"
	if [ -z "$__sp_sep" ]; then
		__wisp_fail "$1" "split(): empty separator"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__sp_rest="$3"
	__sp_cur=""
	__sp_n=0
	while [ -n "$__sp_rest" ]; do
		__sp_after="${__sp_rest#"$__sp_sep"}"
		if [ "$__sp_after" != "$__sp_rest" ]; then
			eval "__wisp_a_${__sp_id}_${__sp_n}=\$__sp_cur"
			__sp_n=$(( __sp_n + 1 ))
			__sp_cur=""
			__sp_rest="$__sp_after"
		else
			__sp_first="${__sp_rest%"${__sp_rest#?}"}"
			__sp_cur="$__sp_cur$__sp_first"
			__sp_rest="${__sp_rest#?}"
		fi
	done
	eval "__wisp_a_${__sp_id}_${__sp_n}=\$__sp_cur"
	__sp_n=$(( __sp_n + 1 ))
	eval "__wisp_a_${__sp_id}_len=\$__sp_n"
}`,
	},

	// __wisp_join <id> <sep>: concatenate the elements of array handle id with sep
	// between them into __ret. join of an empty array is "". Element values flow
	// only through "$..." expansions (read via a deferred `\$name` eval), so they
	// are inert data.
	Join: {
		id:    Join,
		order: 61,
		src: `__wisp_join() {
	__jn_id="$1"
	__jn_sep="$2"
	eval "__ret=\$__wisp_a_${__jn_id}_len"
	__jn_len="$__ret"
	__jn_out=""
	__jn_i=0
	while [ "$__jn_i" -lt "$__jn_len" ]; do
		eval "__ret=\$__wisp_a_${__jn_id}_${__jn_i}"
		if [ "$__jn_i" -eq 0 ]; then
			__jn_out="$__ret"
		else
			__jn_out="$__jn_out$__jn_sep$__ret"
		fi
		__jn_i=$(( __jn_i + 1 ))
	done
	__ret="$__jn_out"
}`,
	},

	// __wisp_contains <s> <sub>: literal substring test. Empty sub -> true.
	// Byte-oriented via LC_ALL=C awk index(); a shell ${rest#"$sub"} scan would
	// step codepoints under bash/zsh/busybox in a UTF-8 locale.
	Contains: {
		id:    Contains,
		order: 62,
		src: `__wisp_contains() {
	if [ -z "$2" ]; then
		__ret=true
		return
	fi
	__ret="$(__wisp_cn_s="$1" __wisp_cn_n="$2" LC_ALL=C awk 'BEGIN{
	s=ENVIRON["__wisp_cn_s"]; nd=ENVIRON["__wisp_cn_n"]
	print (index(s,nd)>0)?"true":"false"
}')"
}`,
	},

	// __wisp_starts_with <s> <pre>: true iff s begins with pre (literal). Empty pre
	// -> true. ${s#"$pre"} removes a literal leading pre; if the result differs
	// from s, the prefix was present (or pre was empty, handled first).
	StartsWith: {
		id:    StartsWith,
		order: 63,
		src: `__wisp_starts_with() {
	if [ -z "$2" ]; then
		__ret=true
		return
	fi
	__sw_after="${1#"$2"}"
	if [ "$__sw_after" != "$1" ]; then
		__ret=true
	else
		__ret=false
	fi
}`,
	},

	// __wisp_ends_with <s> <suf>: true iff s ends with suf (literal). Empty suf ->
	// true. Byte-oriented via LC_ALL=C awk length()+substr(); a shell ${s%"$suf"}
	// trim would measure codepoints under bash/zsh/busybox in a UTF-8 locale.
	EndsWith: {
		id:    EndsWith,
		order: 64,
		src: `__wisp_ends_with() {
	if [ -z "$2" ]; then
		__ret=true
		return
	fi
	__ret="$(__wisp_ew_s="$1" __wisp_ew_n="$2" LC_ALL=C awk 'BEGIN{
	s=ENVIRON["__wisp_ew_s"]; nd=ENVIRON["__wisp_ew_n"]; n=length(nd)
	print (length(s)>=n && substr(s,length(s)-n+1)==nd)?"true":"false"
}')"
}`,
	},

	// __wisp_index_of <s> <sub>: byte index of the first literal occurrence of sub
	// in s, or -1 if absent; empty sub -> 0. Byte-oriented via LC_ALL=C awk index()
	// (1-based, 0 when absent), so the documented byte index is identical on all
	// four shells; a shell `${rest#?}` byte scan would step codepoints under
	// bash/zsh/busybox in a UTF-8 locale.
	IndexOf: {
		id:    IndexOf,
		order: 65,
		src: `__wisp_index_of() {
	__ret="$(__wisp_io_s="$1" __wisp_io_n="$2" LC_ALL=C awk 'BEGIN{
	s=ENVIRON["__wisp_io_s"]; nd=ENVIRON["__wisp_io_n"]
	if (nd=="") { print 0; exit }
	p=index(s,nd); print (p==0?-1:p-1)
}')"
}`,
	},

	// __wisp_repeat <pos> <s> <n>: s repeated n times. n < 0 aborts located; n == 0
	// -> "". n is int-valid ([+-]?[0-9]+), so the loop counter is trusted. The
	// negative check is a numeric `[ ]` against the trusted n.
	Repeat: {
		id:    Repeat,
		deps:  []string{Fail},
		order: 66,
		src: `__wisp_repeat() {
	__rp_n="$3"
	if [ "$__rp_n" -lt 0 ]; then
		__wisp_fail "$1" "repeat(): negative count"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__rp_out=""
	__rp_i=0
	while [ "$__rp_i" -lt "$__rp_n" ]; do
		__rp_out="$__rp_out$2"
		__rp_i=$(( __rp_i + 1 ))
	done
	__ret="$__rp_out"
}`,
	},

	// __wisp_fabs <pos> <x>: |x| for a float, via a constant awk program (operand
	// passed only through -v), then finiteness-validated like the other float ops.
	FAbs: {
		id:    FAbs,
		deps:  []string{FFinite},
		order: 67,
		src: `__wisp_fabs() {
	__f_r="$(LC_ALL=C awk -v a="$2" 'BEGIN{ x=(a+0); if (x<0) x=-x; printf "%.17g", x }')"
	__wisp_ffinite "$1" "$__f_r"
}`,
	},

	// --- I/O (M7) ---

	// __wisp_env <pos> <name>: return ENVIRON[name] in __ret, or abort located
	// naming the variable when unset. The name is passed to awk via -v (never
	// interpolated into the program text), so a name with shell- or awk-active
	// bytes is inert. The membership test `(name in ENVIRON)` is true for a
	// set-but-empty variable and false only when unset, so set-empty correctly
	// returns "". The value is read through an awk command substitution, which
	// strips trailing newlines (the documented env/run distinction from
	// read_file's exact bytes); interior bytes are preserved.
	Env: {
		id:    Env,
		deps:  []string{Fail},
		order: 70,
		src: `__wisp_env() {
	if awk -v n="$2" 'BEGIN{ exit !(n in ENVIRON) }'; then
		__ret="$(awk -v n="$2" 'BEGIN{ printf "%s", ENVIRON[n] }')"
	else
		__wisp_fail "$1" "env: $2 is not set"
		[ -n "$__wisp_err_pending" ] && return
	fi
}`,
	},

	// __wisp_has_env <name>: true/false on whether name is set (including set to
	// the empty string). The name passes via -v only.
	HasEnv: {
		id:    HasEnv,
		order: 71,
		src: `__wisp_has_env() {
	if awk -v n="$1" 'BEGIN{ exit !(n in ENVIRON) }'; then
		__ret=true
	else
		__ret=false
	fi
}`,
	},

	// __wisp_read_file <pos> <path>: return the file's EXACT bytes in __ret. A
	// trailing sentinel x is appended inside the command substitution so $( )
	// strips none of the file's own trailing newlines, then stripped after. The
	// capture is attempted FIRST (it also establishes readability): a failed cat
	// aborts located and short-circuits before the NUL guard, so the guard cannot
	// misfire on a missing file. The NUL guard then re-reads the now-known-readable
	// file: `tr -d '\000'` removes NUL bytes and `cmp -s` compares to the original;
	// they differ iff the file held a NUL, which wisp strings cannot hold (M1). The
	// pipe form is used (dash/busybox have no process substitution). The path is a
	// quoted, --terminated argument so a leading-`-` path is not option-parsed.
	ReadFile: {
		id:    ReadFile,
		deps:  []string{Fail},
		order: 72,
		src: `__wisp_read_file() {
	__rf_out="$(cat -- "$2" && printf x)" || __wisp_fail "$1" "read_file: cannot read $2"
	[ -n "$__wisp_err_pending" ] && return
	tr -d '\000' < "$2" | cmp -s - "$2" || __wisp_fail "$1" "read_file: $2 contains a NUL byte (unsupported)"
	[ -n "$__wisp_err_pending" ] && return
	__ret="${__rf_out%x}"
}`,
	},

	// __wisp_write_file <pos> <path> <content>: write content to path, truncating
	// or creating. content and path are quoted expansions; the redirection target
	// is not option-parsed, so a leading-`-` path is safe. printf '%s' adds no
	// trailing newline (exact-bytes write). A write failure aborts located naming
	// the path. Writes are not atomic (ordinary redirection).
	WriteFile: {
		id:    WriteFile,
		deps:  []string{Fail},
		order: 73,
		src: `__wisp_write_file() {
	printf '%s' "$3" > "$2" || __wisp_fail "$1" "write_file: cannot write $2"
	[ -n "$__wisp_err_pending" ] && return
}`,
	},

	// __wisp_append_file <pos> <path> <content>: like write_file but appends (>>),
	// creating the file if absent.
	AppendFile: {
		id:    AppendFile,
		deps:  []string{Fail},
		order: 74,
		src: `__wisp_append_file() {
	printf '%s' "$3" >> "$2" || __wisp_fail "$1" "append_file: cannot write $2"
	[ -n "$__wisp_err_pending" ] && return
}`,
	},

	// __wisp_envname <pos> <name> <caller>: validate that <name> ($2) is a POSIX
	// env var name [A-Za-z_][A-Za-z0-9_]* (first byte then rest), byte-exact under
	// LC_ALL=C. On a violation aborts located "<caller>: invalid environment
	// variable name: <name>" ($3 = caller, e.g. set_env / unset_env). Shared by
	// __wisp_set_env and __wisp_unset_env. Duplicates the __wisp_run_env_argv
	// byte-walk; run_env itself is NOT modified. `local LC_ALL` restores the locale
	// on return (mirrors __wisp_run_env_argv). An empty name is rejected by the
	// first-byte case (empty first byte does not match [A-Za-z_]).
	EnvName: {
		id:    EnvName,
		deps:  []string{Fail},
		order: 73,
		src: `__wisp_envname() {
	local LC_ALL __wisp_en_rest __wisp_en_first __wisp_en_c
	LC_ALL=C
	__wisp_en_rest="$2"
	__wisp_en_first="${__wisp_en_rest%"${__wisp_en_rest#?}"}"
	case "$__wisp_en_first" in
		[A-Za-z_]) ;;
		*)
			__wisp_fail "$1" "$3: invalid environment variable name: $2"
			return ;;
	esac
	__wisp_en_rest="${__wisp_en_rest#?}"
	while [ -n "$__wisp_en_rest" ]; do
		__wisp_en_c="${__wisp_en_rest%"${__wisp_en_rest#?}"}"
		__wisp_en_rest="${__wisp_en_rest#?}"
		case "$__wisp_en_c" in
			[A-Za-z0-9_]) ;;
			*)
				__wisp_fail "$1" "$3: invalid environment variable name: $2"
				return ;;
		esac
	done
}`,
	},

	// __wisp_set_env <pos> <name> <value>: validate the name, then export NAME=VALUE
	// into the current shell. The value ($3) is inert: the NAME=VALUE word is one
	// double-quoted expansion, so export assigns it verbatim (no command
	// substitution / glob / word-split / re-eval). $1=pos, $2=name, $3=value.
	// SC2163: `export "$var"` is INTENTIONAL here -- it exports the variable whose
	// NAME=VALUE pair we built (the dynamically-named, already-validated env var),
	// not the literal `__wisp_se_pair`. ShellCheck's SC2163 is a false positive for
	// this idiom; disabled per-line (the file-level disable does NOT cover SC2163,
	// and run_env has no precedent since it uses `env NAME=VALUE`, not `export`).
	SetEnv: {
		id:    SetEnv,
		deps:  []string{Fail, EnvName},
		order: 74,
		src: `__wisp_set_env() {
	local __wisp_se_pair
	__wisp_envname "$1" "$2" set_env
	[ -n "$__wisp_err_pending" ] && return
	__wisp_se_pair="$2=$3"
	# shellcheck disable=SC2163
	export "$__wisp_se_pair"
}`,
	},

	// __wisp_unset_env <pos> <name>: validate the name, then unset it. Unsetting an
	// unset name is a POSIX no-op success. $1=pos, $2=name.
	UnsetEnv: {
		id:    UnsetEnv,
		deps:  []string{Fail, EnvName},
		order: 74,
		src: `__wisp_unset_env() {
	__wisp_envname "$1" "$2" unset_env
	[ -n "$__wisp_err_pending" ] && return
	unset "$2"
}`,
	},

	// __wisp_set_stdin <pos> <content>: replace fd 0 with content's exact bytes
	// (test stdin injection, Gap 1). Writes content to a mktemp scratch file via
	// the write_file inert-write idiom (printf '%s', never re-evaluated), points
	// fd 0 at it with exec 0<, then unlinks it -- POSIX keeps an open fd valid
	// after its path is unlinked, so reads still work and no temp file is left
	// behind. exec 0< rebinds fd 0 for the rest of the CURRENT shell/subshell
	// only, which is what makes per-test isolation work (the test runner forks a
	// subshell per test). __ret is used as scratch for the mktemp path; safe
	// because the void-located call path never reads __ret afterward. $1=pos,
	// $2=content.
	SetStdin: {
		id:    SetStdin,
		deps:  []string{Fail},
		order: 74,
		src: `__wisp_set_stdin() {
	local __wisp_ss_tmp
	__ret="$(mktemp)" || __wisp_fail "$1" "set_stdin: mktemp failed"
	[ -n "$__wisp_err_pending" ] && return
	__wisp_ss_tmp="$__ret"
	printf '%s' "$2" > "$__wisp_ss_tmp" || __wisp_fail "$1" "set_stdin: cannot write stdin buffer"
	[ -n "$__wisp_err_pending" ] && return
	exec 0< "$__wisp_ss_tmp" || __wisp_fail "$1" "set_stdin: cannot reopen stdin"
	[ -n "$__wisp_err_pending" ] && return
	rm -f "$__wisp_ss_tmp"
}`,
	},

	// __wisp_run <pos> <handle-id>: run the command whose argv lives in the array
	// handle, capturing stdout into __ret. The helper is a POSIX function with its
	// OWN positional parameters, isolating the caller's $1.. -- but its params
	// START as (<pos>, <id>), so it saves them into named vars and clears its
	// positionals BEFORE rebuilding argv, or "$@" would still carry pos/id. Each
	// element is appended via a deferred `\$var` expansion inside a double-quoted
	// eval argument (the element value is never re-parsed; the eval string is built
	// from the numeric id and index only). Empty argv aborts before anything runs.
	// stdout is captured in a command substitution (stderr passes through); the
	// exit status is snapshotted on the very next line BEFORE the short-circuit
	// guard (which would clobber $?), and a nonzero status aborts located naming
	// argv[0] and the status (a non-executable command surfaces as 127).
	Run: {
		id:    Run,
		deps:  []string{Fail},
		order: 75,
		src: `__wisp_run() {
	__wisp_run_pos="$1"
	__wisp_run_id="$2"
	eval "__ret=\$__wisp_a_${__wisp_run_id}_len"
	__wisp_run_len="$__ret"
	if [ "$__wisp_run_len" -eq 0 ]; then
		__wisp_fail "$__wisp_run_pos" "run: empty argv"
		[ -n "$__wisp_err_pending" ] && return
	fi
	set --
	__wisp_run_i=0
	while [ "$__wisp_run_i" -lt "$__wisp_run_len" ]; do
		eval "set -- \"\$@\" \"\$__wisp_a_${__wisp_run_id}_${__wisp_run_i}\""
		__wisp_run_i=$(( __wisp_run_i + 1 ))
	done
	__ret="$( "$@" )"
	__wisp_run_rc=$?
	[ -n "$__wisp_err_pending" ] && return
	if [ "$__wisp_run_rc" -ne 0 ]; then
		__wisp_fail "$__wisp_run_pos" "run: $1 exited with status $__wisp_run_rc"
		[ -n "$__wisp_err_pending" ] && return
	fi
}`,
	},

	RunInput: {
		id:    RunInput,
		deps:  []string{Fail},
		order: 76,
		src: `__wisp_run_input() {
	__wisp_ri_pos="$1"
	__wisp_ri_id="$2"
	__wisp_ri_stdin="$3"
	eval "__ret=\$__wisp_a_${__wisp_ri_id}_len"
	__wisp_ri_len="$__ret"
	if [ "$__wisp_ri_len" -eq 0 ]; then
		__wisp_fail "$__wisp_ri_pos" "run_input: empty argv"
		[ -n "$__wisp_err_pending" ] && return
	fi
	set --
	__wisp_ri_i=0
	while [ "$__wisp_ri_i" -lt "$__wisp_ri_len" ]; do
		eval "set -- \"\$@\" \"\$__wisp_a_${__wisp_ri_id}_${__wisp_ri_i}\""
		__wisp_ri_i=$(( __wisp_ri_i + 1 ))
	done
	__ret="$(printf '%s' "$__wisp_ri_stdin" | "$@")"
	__wisp_ri_rc=$?
	[ -n "$__wisp_err_pending" ] && return
	if [ "$__wisp_ri_rc" -ne 0 ]; then
		__wisp_fail "$__wisp_ri_pos" "run_input: $1 exited with status $__wisp_ri_rc"
		[ -n "$__wisp_err_pending" ] && return
	fi
}`,
	},

	// __wisp_exec_command <pos> <handle-id>: replace the current shell process
	// with the argv stored under <handle-id>. $1 = call position (injected by
	// genVoidLocatedHelperCall), $2 = the argv array handle id. Empty argv aborts
	// located BEFORE exec (reachable). Otherwise rebuild argv into the positionals
	// (run-family pattern, exactly like __wisp_run) and `exec "$@"`. On success the
	// shell process is REPLACED -- exec never returns; the helper ends at exec "$@".
	// A FAILED exec (command not found / not executable) makes a non-interactive
	// POSIX shell EXIT 127/126 with its own diagnostic on every target shell
	// (dash/busybox ash/bash/zsh) -- it does NOT return, so emitting a post-exec
	// located abort would be unreachable dead code (and ShellCheck would flag it);
	// the helper therefore ends at `exec "$@"`. LOAD-BEARING: capture pos + id into
	// named vars BEFORE `set --` (which clears the positionals), mirroring
	// __wisp_run / __wisp_pipe_exec.
	ExecCommand: {
		id:    ExecCommand,
		deps:  []string{Fail},
		order: 76,
		src: `__wisp_exec_command() {
	__wisp_ec_pos="$1"
	__wisp_ec_id="$2"
	eval "__ret=\$__wisp_a_${__wisp_ec_id}_len"
	__wisp_ec_len="$__ret"
	if [ "$__wisp_ec_len" -eq 0 ]; then
		__wisp_fail "$__wisp_ec_pos" "exec_command: empty argv"
		[ -n "$__wisp_err_pending" ] && return
	fi
	set --
	__wisp_ec_i=0
	while [ "$__wisp_ec_i" -lt "$__wisp_ec_len" ]; do
		eval "set -- \"\$@\" \"\$__wisp_a_${__wisp_ec_id}_${__wisp_ec_i}\""
		__wisp_ec_i=$(( __wisp_ec_i + 1 ))
	done
	exec "$@"
}`,
	},

	// __wisp_run_env_argv <pos> <new-id> <argv-id> <env-id>: build a NEW argv array
	// of the form `env NAME=VALUE... <original argv>` (NO --) into handle <new-id>,
	// for the run_env family; the caller then reuses the plain run/run_status/
	// run_full path over <new-id>. STEPS:
	// (a) The ORIGINAL argv must be non-empty -- the built argv always carries `env`
	//     (and >=0 pairs), so it is never empty and the reused helper's own
	//     empty-argv abort cannot fire; this pre-check is the only place a
	//     run_env([], ...) call aborts located ("empty argv"), mirroring run([]).
	// (b) Clear positionals, then seed `env` as argv[0].
	// (c) For each env-dict key token (insertion order), DECODE the token to the
	//     original NAME bytes (validation MUST run on the decoded bytes, not the
	//     [0-9a-fk] token), then VALIDATE the name under LC_ALL=C against the POSIX
	//     env-name grammar [A-Za-z_][A-Za-z0-9_]* via a byte-at-a-time case-glob
	//     walk (first byte a letter/_; every remaining byte alnum/_; an EMPTY name
	//     or ANY other byte -- =, whitespace, newline, control, non-ASCII -- is
	//     rejected). A violation aborts located with the shared run_env message. The
	//     value is read from __wisp_d_<env-id>_<token>, the NAME=VALUE word is built
	//     with printf '%s=%s', and appended via the DEFERRED escaped-\$ eval
	//     (set -- "$@" "$word"): expansion is deferred to eval-time as a plain
	//     double-quoted parameter expansion that does NOT re-execute an embedded
	//     $(...) in the value -- the run-family injection-safety mechanism (verbatim
	//     __wisp_run's eval form). VALUES are never validated (inert after the =).
	// (d) Append each original argv element via the same deferred escaped-\$ eval.
	// (e) Store the rebuilt "$@" into the new handle's __wisp_a_<new-id>_<n> backing
	//     vars + _len (the list_dir/keys eval-store pattern). LC_ALL=C is scoped via
	//     `local` for byte-correct validation, matching __wisp_dkey_enc/_dec.
	RunEnv: {
		id:    RunEnv,
		deps:  []string{Fail, DictDec},
		order: 75,
		src: `__wisp_run_env_argv() {
	local LC_ALL
	LC_ALL=C
	__wisp_rev_pos="$1"
	__wisp_rev_new="$2"
	__wisp_rev_argv="$3"
	__wisp_rev_env="$4"
	eval "__ret=\$__wisp_a_${__wisp_rev_argv}_len"
	__wisp_rev_len="$__ret"
	if [ "$__wisp_rev_len" -eq 0 ]; then
		__wisp_fail "$__wisp_rev_pos" "run_env: empty argv"
		[ -n "$__wisp_err_pending" ] && return
	fi
	set -- env
	eval "__ret=\$__wisp_d_${__wisp_rev_env}_keys"
	for __wisp_rev_tok in $__ret; do
		__wisp_dkey_dec "$__wisp_rev_tok"
		__wisp_rev_name="$__ret"
		__wisp_rev_rest="$__wisp_rev_name"
		__wisp_rev_first="${__wisp_rev_rest%"${__wisp_rev_rest#?}"}"
		case "$__wisp_rev_first" in
			[A-Za-z_]) ;;
			*)
				__wisp_fail "$__wisp_rev_pos" "run_env: invalid environment variable name: $__wisp_rev_name"
				[ -n "$__wisp_err_pending" ] && return ;;
		esac
		__wisp_rev_rest="${__wisp_rev_rest#?}"
		while [ -n "$__wisp_rev_rest" ]; do
			__wisp_rev_c="${__wisp_rev_rest%"${__wisp_rev_rest#?}"}"
			__wisp_rev_rest="${__wisp_rev_rest#?}"
			case "$__wisp_rev_c" in
				[A-Za-z0-9_]) ;;
				*)
					__wisp_fail "$__wisp_rev_pos" "run_env: invalid environment variable name: $__wisp_rev_name"
					[ -n "$__wisp_err_pending" ] && return ;;
			esac
		done
		eval "__ret=\$__wisp_d_${__wisp_rev_env}_${__wisp_rev_tok}"
		__wisp_rev_val="$__ret"
		__wisp_rev_pair="$__wisp_rev_name=$__wisp_rev_val"
		eval "set -- \"\$@\" \"\$__wisp_rev_pair\""
	done
	__wisp_rev_i=0
	while [ "$__wisp_rev_i" -lt "$__wisp_rev_len" ]; do
		eval "set -- \"\$@\" \"\$__wisp_a_${__wisp_rev_argv}_${__wisp_rev_i}\""
		__wisp_rev_i=$(( __wisp_rev_i + 1 ))
	done
	__wisp_rev_n=0
	while [ "$#" -gt 0 ]; do
		eval "__wisp_a_${__wisp_rev_new}_${__wisp_rev_n}=\$1"
		shift
		__wisp_rev_n=$(( __wisp_rev_n + 1 ))
	done
	eval "__wisp_a_${__wisp_rev_new}_len=\$__wisp_rev_n"
}`,
	},

	// --- Filesystem + process (fs milestone) ---

	// __wisp_pid_alive <pid>: true iff `kill -0 pid` succeeds (process/group exists and is
	// signalable by the caller). Total: ESRCH (no such process) and EPERM (not permitted) both
	// read false; never aborts. pid is an inert quoted "$1" int after the fixed -0 selector.
	PidAlive: {
		id:    PidAlive,
		order: 76,
		src: `__wisp_pid_alive() {
	if kill -0 "$1" 2>/dev/null; then __ret=true; else __ret=false; fi
}`,
	},

	// __wisp_file_exists <path>: true iff path exists (any file type, follows a
	// symlink). [ takes the operand as data, so no -- is needed. Total.
	FileExists: {
		id:    FileExists,
		order: 76,
		src: `__wisp_file_exists() {
	if [ -e "$1" ]; then __ret=true; else __ret=false; fi
}`,
	},

	// __wisp_is_dir <path>: true iff path exists and is a directory (follows a
	// symlink to a directory). Total.
	IsDir: {
		id:    IsDir,
		order: 77,
		src: `__wisp_is_dir() {
	if [ -d "$1" ]; then __ret=true; else __ret=false; fi
}`,
	},

	// __wisp_is_file <path>: true iff path is a regular file ([ -f ], follows
	// symlinks). Total.
	IsFile: {
		id:    IsFile,
		order: 77,
		src: `__wisp_is_file() {
	if [ -f "$1" ]; then __ret=true; else __ret=false; fi
}`,
	},

	// __wisp_is_symlink <path>: true iff path itself is a symlink ([ -L ]). Total.
	IsSymlink: {
		id:    IsSymlink,
		order: 77,
		src: `__wisp_is_symlink() {
	if [ -L "$1" ]; then __ret=true; else __ret=false; fi
}`,
	},

	// __wisp_file_size <pos> <path>: byte count via wc -c redirected from path.
	// The if-assignment form checks wc's exit status directly (no pipeline that
	// would lose the status). BSD/macOS wc -c emits leading whitespace ("   3");
	// the while loop strips leading spaces via parameter expansion (NOT arithmetic,
	// to avoid the float/int-invariant machinery). A missing path, an unreadable
	// file, or a directory all cause wc -c to fail -> located abort.
	FileSize: {
		id:    FileSize,
		deps:  []string{Fail},
		order: 80,
		src: `__wisp_file_size() {
	if __ret="$(wc -c < "$2")"; then
		while case "$__ret" in ' '*) true;; *) false;; esac; do __ret="${__ret# }"; done
	else
		__wisp_fail "$1" "file_size: cannot read $2"
		[ -n "$__wisp_err_pending" ] && return
	fi
}`,
	},

	// __wisp_chmod <pos> <path> <mode>: change permissions via chmod. The CLI order
	// is mode-then-path, so $3 (mode) precedes $2 (path) in the command. A SINGLE
	// -- terminates option parsing, guarding a leading-dash path. A missing path or
	// bad mode aborts located.
	Chmod: {
		id:    Chmod,
		deps:  []string{Fail},
		order: 85,
		src: `__wisp_chmod() {
	chmod -- "$3" "$2" || { __wisp_fail "$1" "chmod: failed: $2"; }
	[ -n "$__wisp_err_pending" ] && return
}`,
	},

	// __wisp_symlink <pos> <target> <link_path>: create a symlink via ln -s. The
	// wisp arg order (target, link_path) matches ln's CLI order, so $2=target and
	// $3=link_path. A SINGLE -- terminates option parsing, guarding a leading-dash
	// target or link_path. If link_path already exists, ln fails -> located abort.
	Symlink: {
		id:    Symlink,
		deps:  []string{Fail},
		order: 86,
		src: `__wisp_symlink() {
	ln -s -- "$2" "$3" || { __wisp_fail "$1" "symlink: failed"; }
	[ -n "$__wisp_err_pending" ] && return
}`,
	},

	// __wisp_symlink_force <pos> <target> <link_path>: create-or-replace a symlink.
	// rm -f removes an existing symlink (the LINK, never its target) or a regular
	// file (or other non-directory); ln -s then creates the new link. A REAL
	// directory at link_path makes rm -f fail ("is a directory"), the && short-
	// circuits, and the located abort fires -- never clobbers or links inside a real
	// dir. Both args inert double-quoted with -- guards. NOT temp+mv: BSD mv follows
	// a symlink-to-dir destination (the ln -sf botch); rm+ln with POSIX-only flags is
	// correct + portable on BSD + GNU. $1=pos, $2=target, $3=link_path.
	SymlinkForce: {
		id:    SymlinkForce,
		deps:  []string{Fail},
		order: 86,
		src: `__wisp_symlink_force() {
	rm -f -- "$3" && ln -s -- "$2" "$3" || { __wisp_fail "$1" "symlink_force: failed"; }
	[ -n "$__wisp_err_pending" ] && return
}`,
	},

	// __wisp_read_link <path>: resolve the immediate target of a symlink via
	// readlink -- "$1". The exit status of the command substitution assignment IS
	// readlink's status (POSIX: a simple assignment with a $() takes the
	// substitution's status). Codegen wraps the result via genStrSentinelToOptional:
	// exit 0 -> Some(__ret), nonzero -> None. A non-symlink, missing path, or
	// permission failure all exit nonzero -> None. Single-level (no -f). Never aborts.
	ReadLink: {
		id:    ReadLink,
		order: 87,
		src: `__wisp_read_link() {
	__ret="$(readlink -- "$1")"
}`,
	},

	// __wisp_temp_file <pos>: create a temporary file via mktemp and return its path
	// in __ret. Fallible: located abort if mktemp fails (e.g. $TMPDIR unwritable).
	// Uses genLocatedHelperCall (position $1); no user path arg.
	TempFile: {
		id:    TempFile,
		deps:  []string{Fail},
		order: 88,
		src: `__wisp_temp_file() {
	if __ret="$(mktemp)"; then
		:
	else
		__wisp_fail "$1" "temp_file: mktemp failed"
		[ -n "$__wisp_err_pending" ] && return
	fi
}`,
	},

	// __wisp_temp_dir <pos>: create a temporary directory via mktemp -d and return
	// its path in __ret. Fallible: located abort if mktemp -d fails.
	// Uses genLocatedHelperCall (position $1); no user path arg.
	TempDir: {
		id:    TempDir,
		deps:  []string{Fail},
		order: 89,
		src: `__wisp_temp_dir() {
	if __ret="$(mktemp -d)"; then
		:
	else
		__wisp_fail "$1" "temp_dir: mktemp failed"
		[ -n "$__wisp_err_pending" ] && return
	fi
}`,
	},

	// __wisp_cwd: the current working directory. The trailing newline that command
	// substitution strips is correct (a cwd never legitimately ends in newline).
	Cwd: {
		id:    Cwd,
		order: 78,
		src: `__wisp_cwd() {
	__ret="$(pwd)"
}`,
	},

	// __wisp_pi: the float literal pi. A real helper (not just a codegen literal)
	// so pi is referenceable as a funcref value; the direct-call codegen path
	// also routes through this, so there is exactly one lowering.
	Pi: {
		id:    Pi,
		order: 1,
		src: `__wisp_pi() {
	__ret=3.141592653589793
}`,
	},

	// __wisp_program_path: reads the $__wisp_arg0 capture global. Callers (both
	// the direct-call codegen path and the funcref-wrapper-use path) MUST also
	// g.use(Arg0) so the `__wisp_arg0="$0"` capture line is emitted; Arg0 is a
	// codegen sentinel, not a registry dependency, so it cannot be listed in deps.
	ProgramPath: {
		id:    ProgramPath,
		order: 1,
		src: `__wisp_program_path() {
	__ret="$__wisp_arg0"
}`,
	},

	// __wisp_dir_name <path>: the directory portion, POSIX dirname-style, as a
	// PURE shell string function (no external dirname, no I/O). Special cases per
	// spec P4: "" -> ".", an all-slash path ("/" or "//") -> "/", a path with no
	// slash after trailing-slash stripping -> ".", and stripping the last
	// component to empty (e.g. "/a") -> "/". The value flows ONLY through
	// double-quoted expansion and quoted `case` patterns (literal, never a glob),
	// so a path with $(...)/backticks/;/|/*/space/newline is inert (spec P5). Total.
	DirName: {
		id:    DirName,
		order: 78,
		src: `__wisp_dir_name() {
	__dn_p="$1"
	if [ -z "$__dn_p" ]; then __ret="."; return; fi
	case "$__dn_p" in
		*[!/]*) ;;
		*) __ret="/"; return ;;
	esac
	while :; do
		case "$__dn_p" in
			?*/) __dn_p="${__dn_p%/}" ;;
			*) break ;;
		esac
	done
	case "$__dn_p" in
		*/*) ;;
		*) __ret="."; return ;;
	esac
	__dn_d="${__dn_p%/*}"
	if [ -z "$__dn_d" ]; then __ret="/"; else __ret="$__dn_d"; fi
}`,
	},

	// __wisp_base_name <path>: the final path component, POSIX basename-style, as
	// a PURE shell string function. Special cases per spec P4: "" -> "", an
	// all-slash path -> "/", otherwise strip trailing slashes then take the
	// component after the last slash. Same injection-inert handling as dir_name
	// (quoted expansion + quoted case patterns). Total.
	BaseName: {
		id:    BaseName,
		order: 78,
		src: `__wisp_base_name() {
	__bn_p="$1"
	if [ -z "$__bn_p" ]; then __ret=""; return; fi
	case "$__bn_p" in
		*[!/]*) ;;
		*) __ret="/"; return ;;
	esac
	while :; do
		case "$__bn_p" in
			?*/) __bn_p="${__bn_p%/}" ;;
			*) break ;;
		esac
	done
	__ret="${__bn_p##*/}"
}`,
	},

	// __wisp_env_or <name> <fallback>: the value of environment variable name if
	// SET (including set-but-empty -> ""), else fallback. The name flows only via
	// awk -v (never interpolated into the program); the membership test
	// (n in ENVIRON) distinguishes set-empty from unset, exactly like env/has_env.
	EnvOr: {
		id:    EnvOr,
		order: 79,
		src: `__wisp_env_or() {
	if awk -v n="$1" 'BEGIN{ exit !(n in ENVIRON) }'; then
		__ret="$(awk -v n="$1" 'BEGIN{ printf "%s", ENVIRON[n] }')"
	else
		__ret="$2"
	fi
}`,
	},

	// __wisp_make_dir <pos> <path>: mkdir -p -- path. Creating an existing
	// directory is a no-op success (-p); a genuine failure (a path component is a
	// non-directory, permission denied) aborts located. -- so a leading-`-` path is
	// data. Not transactional (mkdir -p may create ancestors before failing).
	MakeDir: {
		id:    MakeDir,
		deps:  []string{Fail},
		order: 80,
		src: `__wisp_make_dir() {
	mkdir -p -- "$2" || __wisp_fail "$1" "make_dir: cannot create $2"
	[ -n "$__wisp_err_pending" ] && return
}`,
	},

	// __wisp_remove_file <pos> <path>: rm -f -- path. Removing a missing file is a
	// no-op success (-f); a real failure (path is a directory, unwritable parent)
	// aborts located -- -f does not mask a genuine error.
	RemoveFile: {
		id:    RemoveFile,
		deps:  []string{Fail},
		order: 81,
		src: `__wisp_remove_file() {
	rm -f -- "$2" || __wisp_fail "$1" "remove_file: cannot remove $2"
	[ -n "$__wisp_err_pending" ] && return
}`,
	},

	// __wisp_remove_dir <pos> <path>: rmdir -- path (empty directory only). A
	// missing path, a non-directory, or a non-empty directory aborts located.
	RemoveDir: {
		id:    RemoveDir,
		deps:  []string{Fail},
		order: 82,
		src: `__wisp_remove_dir() {
	rmdir -- "$2" || __wisp_fail "$1" "remove_dir: cannot remove $2"
	[ -n "$__wisp_err_pending" ] && return
}`,
	},

	// __wisp_rename <pos> <from> <to>: mv -- from to. A failure (source missing,
	// cross-device unsupported, permission) aborts located. -- terminates options
	// for BOTH operands.
	Rename: {
		id:    Rename,
		deps:  []string{Fail},
		order: 83,
		src: `__wisp_rename() {
	mv -- "$2" "$3" || __wisp_fail "$1" "rename: cannot rename $2 to $3"
	[ -n "$__wisp_err_pending" ] && return
}`,
	},

	// __wisp_which <name>: resolve a command name to a path. __ret is assigned in a
	// command substitution, so the function's exit status IS command -v's status
	// (POSIX: a simple assignment with a command substitution takes the
	// substitution's status). The codegen wrapper maps zero -> Some(__ret),
	// nonzero -> None. On not-found, command -v exits nonzero and __ret is empty.
	// -- is defensive (a read with no command-execution surface); droppable for
	// which only if the busybox under test rejects `command -v --` (spec 3.4). Never
	// aborts.
	Which: {
		id:    Which,
		order: 84,
		src: `__wisp_which() {
	__ret="$(command -v -- "$1")"
}`,
	},

	// __wisp_list_dir <pos> <id> <dir>: fill array handle id with the base names of
	// dir's entries (including hidden, excluding . and ..), in unspecified order. A
	// missing path or a non-directory aborts located BEFORE the loop. The loop is a
	// quoted glob (never ls-parsing): an UNMATCHED pattern stays literal and is
	// discarded by the [ -e ] || [ -L ] guard; the .* arm always yields ./.. which
	// the case skips on the BASE NAME (never $entry). Base name via parameter
	// expansion only (byte-exact; $() would strip a trailing newline). A broken
	// symlink fails -e but passes -L and is kept. EMPTY-DIR CONTRACT: zero kept
	// entries writes _len=0 and no element (NOT split's append-final-element body).
	ListDir: {
		id:    ListDir,
		deps:  []string{Fail},
		order: 85,
		src: `__wisp_list_dir() {
	__ld_pos="$1"
	__ld_id="$2"
	__ld_dir="$3"
	if [ ! -d "$__ld_dir" ]; then
		__wisp_fail "$__ld_pos" "list_dir: not a directory: $__ld_dir"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__ld_n=0
	for __ld_entry in "$__ld_dir"/* "$__ld_dir"/.*; do
		__ld_name="${__ld_entry##*/}"
		case "$__ld_name" in
		. | ..) continue ;;
		esac
		[ -e "$__ld_entry" ] || [ -L "$__ld_entry" ] || continue
		eval "__wisp_a_${__ld_id}_${__ld_n}=\$__ld_name"
		__ld_n=$(( __ld_n + 1 ))
	done
	eval "__wisp_a_${__ld_id}_len=\$__ld_n"
}`,
	},

	// __wisp_glob <pos> <id> <pattern>: fill array handle id with the paths matching
	// pattern via shell pathname expansion (the list_dir array-from-shell mechanism,
	// but over a USER pattern instead of "$dir"/*). Total -- no located abort; an
	// empty result is success (the <pos> arg is accepted for call-shape parity with
	// list_dir but never used to fail). The for-in iterates $3 UNQUOTED so the shell
	// expands the pattern (quoting it would make the glob a literal). dash/busybox
	// lack nullglob, so an UNMATCHED pattern stays literal and is dropped by the
	// [ -e ] || [ -L ] guard (-> empty array); the emitted emulate-sh banner puts zsh
	// in sh-mode (disabling NOMATCH, restoring word-splitting) so zsh passes the
	// literal through to the same guard. Matched names are stored with deferred \$
	// (eval), so a filename containing $(...)/backticks/;/*/space/newline is INERT,
	// never re-evaluated -- the list_dir injection-safety guarantee. A broken symlink
	// fails -e but passes -L and is kept.
	Glob: {
		id:    Glob,
		order: 85,
		src: `__wisp_glob() {
	__gl_id="$2"
	__gl_n=0
	for __gl_x in $3; do
		[ -e "$__gl_x" ] || [ -L "$__gl_x" ] || continue
		eval "__wisp_a_${__gl_id}_${__gl_n}=\$__gl_x"
		__gl_n=$(( __gl_n + 1 ))
	done
	eval "__wisp_a_${__gl_id}_len=\$__gl_n"
}`,
	},

	// __wisp_run_status <pos> <handle-id>: run the argv in the array handle BARE
	// (NOT in a $() capture), so the child's stdout/stderr pass through to the
	// script, and return the child's exit status in __ret. The argv rebuild is
	// verbatim run's (own positionals, deferred \$var eval, no command string). The
	// ONLY abort is empty argv; a nonzero child status is RETURNED, not aborted.
	// __ret=$? is captured on the very next line before anything clobbers $?. The
	// private __wisp_rs_* var names avoid colliding with __wisp_run_*.
	RunStatus: {
		id:    RunStatus,
		deps:  []string{Fail},
		order: 86,
		src: `__wisp_run_status() {
	__wisp_rs_pos="$1"
	__wisp_rs_id="$2"
	eval "__ret=\$__wisp_a_${__wisp_rs_id}_len"
	__wisp_rs_len="$__ret"
	if [ "$__wisp_rs_len" -eq 0 ]; then
		__wisp_fail "$__wisp_rs_pos" "run_status: empty argv"
		[ -n "$__wisp_err_pending" ] && return
	fi
	set --
	__wisp_rs_i=0
	while [ "$__wisp_rs_i" -lt "$__wisp_rs_len" ]; do
		eval "set -- \"\$@\" \"\$__wisp_a_${__wisp_rs_id}_${__wisp_rs_i}\""
		__wisp_rs_i=$(( __wisp_rs_i + 1 ))
	done
	"$@"
	__ret=$?
}`,
	},

	// --- Regular expressions (regex milestone) ---
	//
	// Malformed-pattern detection is exactly "awk exited nonzero" (rc != 0). It is
	// faithful to the engine, never stricter: busybox awk is lenient and accepts
	// some malformed ERE (notably an unbalanced '(') with rc 0, yielding an
	// unspecified result instead of aborting; an unbalanced '[' is rejected by every
	// target awk including busybox. wisp does not reimplement ERE validation to mask
	// this, so the abort fixtures use '[' (universally rejected) -- never '(', which
	// is not portably rejected. See the regex spec's malformed-pattern section.
	//
	// __wisp_matches <pos> <s> <p>: true iff p matches anywhere in s. The subject
	// and pattern flow via ENVIRON (never -v); the awk program is a constant under
	// LC_ALL=C; awk prints the 0/1 token then the sentinel x with BOTH printfs
	// inside awk, so the command-substitution status stays awk's. rc != 0 is the
	// malformed-pattern located abort; the token rule is CONJUNCTIVE (rc == 0 AND
	// the stripped output is exactly 0 or 1) so a malformed pattern can never read
	// as false.
	Matches: {
		id:    Matches,
		deps:  []string{Fail},
		order: 87,
		src: `__wisp_matches() {
	__re_out="$(__wisp_re_s="$2" __wisp_re_p="$3" LC_ALL=C awk 'BEGIN{ s=ENVIRON["__wisp_re_s"]; re=ENVIRON["__wisp_re_p"]; printf "%s", (s~re)?"1":"0"; printf "x" }' 2>/dev/null)"
	__re_rc=$?
	if [ "$__re_rc" -ne 0 ]; then
		__wisp_fail "$1" "matches: regex failed (likely an invalid pattern): \"$3\""
		[ -n "$__wisp_err_pending" ] && return
	fi
	__re_out="${__re_out%x}"
	case "$__re_out" in
		1) __ret=true ;;
		0) __ret=false ;;
		*)
			__wisp_fail "$1" "matches: regex failed (likely an invalid pattern): \"$3\""
			[ -n "$__wisp_err_pending" ] && return ;;
	esac
}`,
	},

	// __wisp_regex_find <pos> <s> <p>: the first whole match of p in s. awk prints a
	// one-byte STATUS TOKEN (1 for a match, 0 for none) then the match bytes (if
	// any) then the sentinel x, all inside awk so the command-substitution status
	// stays awk's. rc != 0 is the malformed-pattern located abort. The helper leaves
	// the TOKEN-prefixed value (1<match> or 0) in __ret; codegen peels the leading
	// byte. The in-awk sentinel preserves a match that ends in a newline.
	RegexFind: {
		id:    RegexFind,
		deps:  []string{Fail},
		order: 88,
		src: `__wisp_regex_find() {
	__re_out="$(__wisp_re_s="$2" __wisp_re_p="$3" LC_ALL=C awk 'BEGIN{ s=ENVIRON["__wisp_re_s"]; re=ENVIRON["__wisp_re_p"]; if (match(s, re)) { printf "1%s", substr(s, RSTART, RLENGTH) } else { printf "0" }; printf "x" }' 2>/dev/null)"
	__re_rc=$?
	if [ "$__re_rc" -ne 0 ]; then
		__wisp_fail "$1" "regex_find: regex failed (likely an invalid pattern): \"$3\""
		[ -n "$__wisp_err_pending" ] && return
	fi
	__ret="${__re_out%x}"
}`,
	},

	// __wisp_regex_find_all <pos> <id> <s> <p>: fill array handle id with every
	// NON-EMPTY whole match of p in s, left to right. awk does the match loop and
	// the MANDATORY zero-width-safe advance; the shell decodes the result and writes
	// the array. The transport is a length-prefixed, delimiter-free stream: per
	// non-empty match awk emits "<len>:<bytes>\n" where <len> is the byte length
	// (LC_ALL=C makes length() a byte count, so the shell byte-slice agrees). The
	// shell-side decode loop runs under a function-local LC_ALL=C (scoped via
	// `local`, like the dict key codec) so its byte-at-a-time `?` peel counts bytes,
	// not locale characters -- otherwise a non-C caller locale (e.g. bash in UTF-8)
	// would peel multibyte characters and desync the count from awk's byte length.
	// The decode reads <len> up to the first ':', then consumes EXACTLY <len> bytes
	// for the match (a per-byte peel, so a match's own newline is data, never a
	// delimiter), then drops the single framing newline. The advance formula
	// substr(rest, RSTART + (RLENGTH > 0 ? RLENGTH : 1)) advances by one byte on a
	// zero-width match so x*/a? cannot loop forever. On a malformed pattern awk
	// emits zero records before it fatals, so there is no partial stream to
	// mis-decode: _len=0 is set before the located abort and the decode loop is
	// never reached on that path.
	RegexFindAll: {
		id:    RegexFindAll,
		deps:  []string{Fail},
		order: 89,
		src: `__wisp_regex_find_all() {
	local LC_ALL
	LC_ALL=C
	__re_id="$2"
	__re_out="$(__wisp_re_s="$3" __wisp_re_p="$4" LC_ALL=C awk 'BEGIN{
		s = ENVIRON["__wisp_re_s"]; re = ENVIRON["__wisp_re_p"]; rest = s
		while (match(rest, re)) {
			if (RLENGTH > 0) { m = substr(rest, RSTART, RLENGTH); printf "%d:%s\n", length(m), m }
			rest = substr(rest, RSTART + (RLENGTH > 0 ? RLENGTH : 1))
			if (rest == "") break
		}
		printf "x"
	}' 2>/dev/null)"
	__re_rc=$?
	if [ "$__re_rc" -ne 0 ]; then
		eval "__wisp_a_${__re_id}_len=0"
		__wisp_fail "$1" "regex_find_all: regex failed (likely an invalid pattern): \"$4\""
		[ -n "$__wisp_err_pending" ] && return
	fi
	__re_out="${__re_out%x}"
	__re_n=0
	while [ -n "$__re_out" ]; do
		__re_len="${__re_out%%:*}"
		__re_out="${__re_out#*:}"
		__re_m=""
		__re_i=0
		while [ "$__re_i" -lt "$__re_len" ]; do
			__re_c="${__re_out%"${__re_out#?}"}"
			__re_m="$__re_m$__re_c"
			__re_out="${__re_out#?}"
			__re_i=$(( __re_i + 1 ))
		done
		__re_out="${__re_out#?}"
		eval "__wisp_a_${__re_id}_${__re_n}=\$__re_m"
		__re_n=$(( __re_n + 1 ))
	done
	eval "__wisp_a_${__re_id}_len=\$__re_n"
}`,
	},

	// __wisp_regex_replace <pos> <s> <p> <repl>: replace ALL matches of p in s with
	// repl (awk gsub). repl follows awk's replacement semantics: & = whole match and
	// \& = literal & are POSIX-defined and portable; group backrefs are unsupported.
	// A literal backslash (\\) is engine-dependent (BSD awk vs gawk/mawk/busybox
	// disagree), so wisp documents it rather than guaranteeing it. No status token
	// (the whole output is payload), so the malformed signal is rc != 0 only.
	// Trailing newlines survive via the in-awk sentinel x.
	RegexReplace: {
		id:    RegexReplace,
		deps:  []string{Fail},
		order: 90,
		src: `__wisp_regex_replace() {
	__re_out="$(__wisp_re_s="$2" __wisp_re_p="$3" __wisp_re_r="$4" LC_ALL=C awk 'BEGIN{ s=ENVIRON["__wisp_re_s"]; re=ENVIRON["__wisp_re_p"]; repl=ENVIRON["__wisp_re_r"]; gsub(re, repl, s); printf "%s", s; printf "x" }' 2>/dev/null)"
	__re_rc=$?
	if [ "$__re_rc" -ne 0 ]; then
		__wisp_fail "$1" "regex_replace: regex failed (likely an invalid pattern): \"$3\""
		[ -n "$__wisp_err_pending" ] && return
	fi
	__ret="${__re_out%x}"
}`,
	},
	ReverseString: {
		id:    ReverseString,
		order: 91,
		src: `__wisp_reverse_string() {
	__rs_out="$(__wisp_rvs_s="$1" LC_ALL=C awk 'BEGIN{
	s=ENVIRON["__wisp_rvs_s"]; n=length(s); out=""
	for(i=n;i>=1;i--) out=out substr(s,i,1)
	printf "%sx", out
}')"
	__ret="${__rs_out%x}"
}`,
	},
	Ord: {
		id:    Ord,
		deps:  []string{Fail},
		order: 92,
		src: `__wisp_ord() {
	if [ -z "$2" ]; then
		__wisp_fail "$1" "ord(): empty string"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__ret="$(__wisp_ord_s="$2" LC_ALL=C awk 'BEGIN{
	for(i=1;i<=255;i++) __ch[sprintf("%c",i)]=i
	printf "%d", __ch[substr(ENVIRON["__wisp_ord_s"],1,1)]
}')"
}`,
	},
	Chr: {
		id:    Chr,
		deps:  []string{Fail},
		order: 93,
		src: `__wisp_chr() {
	if [ "$2" -lt 1 ] || [ "$2" -gt 255 ]; then
		__wisp_fail "$1" "chr(): code out of range 1-255"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__chr_out="$(printf "\\$(printf '%03o' "$2")x")"
	__ret="${__chr_out%x}"
}`,
	},
	ReadLine: {
		id:    ReadLine,
		order: 94,
		src: `__wisp_read_line() {
	__wisp_rl_line=""
	IFS= read -r __wisp_rl_line
	__wisp_rl_rc=$?
	if [ "$__wisp_rl_rc" -ne 0 ] && [ -z "$__wisp_rl_line" ]; then
		__wisp_rl_eof=1
		__ret=""
	else
		__wisp_rl_eof=
		__ret="$__wisp_rl_line"
	fi
}`,
	},
	// __wisp_read_secret <prompt>: print prompt to stderr, read one line with echo
	// suppressed (best-effort stty), return via __ret + __wisp_rl_eof (the SHARED
	// read_line EOF flag -- safe: genReadSecret copies it immediately, mirroring
	// genReadLine). $1=prompt. On a non-TTY (pipe), stty -g/-echo fail silently and
	// read still reads the line. CAPTURE rc IMMEDIATELY after read, before the stty
	// restore (else rc would be the stty exit -> EOF misclassified). Restore + trailing
	// newline run before the EOF branch so a normal/EOF path never leaves echo off.
	// NO trap (would clobber the user's on_signal/on_exit; not portably saveable) --
	// a SIGINT during read may leave echo off (documented; recover via blind `stty echo`).
	// Total: never aborts.
	ReadSecret: {
		id:    ReadSecret,
		order: 94,
		src: `__wisp_read_secret() {
	printf '%s' "$1" >&2
	__wisp_rs_saved="$(stty -g 2>/dev/null)"
	stty -echo 2>/dev/null
	__wisp_rs_line=""
	IFS= read -r __wisp_rs_line
	__wisp_rs_rc=$?
	[ -n "$__wisp_rs_saved" ] && stty "$__wisp_rs_saved" 2>/dev/null
	printf '\n' >&2
	if [ "$__wisp_rs_rc" -ne 0 ] && [ -z "$__wisp_rs_line" ]; then
		__wisp_rl_eof=1
		__ret=""
	else
		__wisp_rl_eof=
		__ret="$__wisp_rs_line"
	fi
}`,
	},
	ReadStdin: {
		id:    ReadStdin,
		order: 95,
		src: `__wisp_read_stdin() {
	__wisp_rs_out="$(cat; printf x)"
	__ret="${__wisp_rs_out%x}"
}`,
	},
	ChangeDir: {
		id:    ChangeDir,
		deps:  []string{Fail},
		order: 96,
		src: `__wisp_change_dir() {
	cd -- "$2" || { __wisp_fail "$1" "change_dir: failed"; [ -n "$__wisp_err_pending" ] && return; }
}`,
	},
	RunFull: {
		id:    RunFull,
		deps:  []string{Fail},
		order: 97,
		src: `__wisp_run_full() {
	__wisp_rf_id="$2"
	eval "__wisp_rf_n=\$__wisp_a_${__wisp_rf_id}_len"
	if [ -z "$__wisp_rf_n" ] || [ "$__wisp_rf_n" -eq 0 ]; then
		__wisp_fail "$1" "run_full: empty argv"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__wisp_rf_t1="$(mktemp)" || { __wisp_fail "$1" "run_full: mktemp failed"; [ -n "$__wisp_err_pending" ] && return; }
	__wisp_rf_t2="$(mktemp)" || { rm -f "$__wisp_rf_t1"; __wisp_fail "$1" "run_full: mktemp failed"; [ -n "$__wisp_err_pending" ] && return; }
	set --
	__wisp_rf_i=0
	while [ "$__wisp_rf_i" -lt "$__wisp_rf_n" ]; do
		eval "set -- \"\$@\" \"\$__wisp_a_${__wisp_rf_id}_${__wisp_rf_i}\""
		__wisp_rf_i=$(( __wisp_rf_i + 1 ))
	done
	"$@" < /dev/null > "$__wisp_rf_t1" 2> "$__wisp_rf_t2"
	__wisp_rf_code=$?
	__wisp_rf_sout="$(cat "$__wisp_rf_t1"; printf x)"
	__wisp_rf_serr="$(cat "$__wisp_rf_t2"; printf x)"
	__wisp_rf_stdout="${__wisp_rf_sout%x}"
	__wisp_rf_stderr="${__wisp_rf_serr%x}"
	rm -f "$__wisp_rf_t1" "$__wisp_rf_t2"
}`,
	},

	RunInputFull: {
		id:    RunInputFull,
		deps:  []string{Fail},
		order: 98,
		src: `__wisp_run_input_full() {
	__wisp_rif_id="$2"
	__wisp_rif_stdin="$3"
	eval "__wisp_rif_n=\$__wisp_a_${__wisp_rif_id}_len"
	if [ -z "$__wisp_rif_n" ] || [ "$__wisp_rif_n" -eq 0 ]; then
		__wisp_fail "$1" "run_input_full: empty argv"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__wisp_rif_t1="$(mktemp)" || { __wisp_fail "$1" "run_input_full: mktemp failed"; [ -n "$__wisp_err_pending" ] && return; }
	__wisp_rif_t2="$(mktemp)" || { rm -f "$__wisp_rif_t1"; __wisp_fail "$1" "run_input_full: mktemp failed"; [ -n "$__wisp_err_pending" ] && return; }
	set --
	__wisp_rif_i=0
	while [ "$__wisp_rif_i" -lt "$__wisp_rif_n" ]; do
		eval "set -- \"\$@\" \"\$__wisp_a_${__wisp_rif_id}_${__wisp_rif_i}\""
		__wisp_rif_i=$(( __wisp_rif_i + 1 ))
	done
	printf '%s' "$__wisp_rif_stdin" | "$@" > "$__wisp_rif_t1" 2> "$__wisp_rif_t2"
	__wisp_rf_code=$?
	__wisp_rf_sout="$(cat "$__wisp_rif_t1"; printf x)"
	__wisp_rf_serr="$(cat "$__wisp_rif_t2"; printf x)"
	__wisp_rf_stdout="${__wisp_rf_sout%x}"
	__wisp_rf_stderr="${__wisp_rf_serr%x}"
	rm -f "$__wisp_rif_t1" "$__wisp_rif_t2"
}`,
	},

	// __wisp_cov <record>: coverage hit-marker (spec R15-R17). One append of a
	// single newline-terminated record to $COVFILE; no-op when COVFILE is
	// unset/empty so a coverage-compiled script is safe to run on its own. The
	// record reaches printf only through %s (inert). Emitted only in coverage mode.
	Cov: {
		id:    Cov,
		order: 98,
		src: `__wisp_cov() {
	[ -n "${COVFILE:-}" ] || return 0
	printf '%s\n' "$1" >> "$COVFILE"
}`,
	},

	// __wisp_now: Unix epoch seconds via `date +%s`. No arguments; total.
	// `date +%s` is a GNU/BSD/busybox extension, not POSIX.1, but is present on
	// every real dash/busybox ash/bash/zsh target (documented non-goal: pure
	// POSIX.1-only portability). The trailing newline that command substitution
	// strips is harmless (epoch is a plain integer; no embedded newlines).
	Now: {
		id:    Now,
		order: 99,
		src: `__wisp_now() {
	__ret="$(date +%s)"
}`,
	},

	// __wisp_sleep <pos> <secs>: whole-second pause via the external `sleep`
	// command. $1 = call position (injected by genVoidLocatedHelperCall), $2 =
	// secs. Guards secs >= 0; a negative value aborts located so the malformed
	// token never reaches the external `sleep`. secs flows as inert "$2" (never
	// interpolated). Sub-second sleep is a non-goal (not portable to POSIX sleep).
	Sleep: {
		id:    Sleep,
		deps:  []string{Fail},
		order: 99,
		src: `__wisp_sleep() {
	if [ "$2" -lt 0 ]; then
		__wisp_fail "$1" "sleep: negative duration"
		[ -n "$__wisp_err_pending" ] && return
	fi
	sleep "$2"
}`,
	},

	// __wisp_random <pos> <max>: a non-cryptographic random integer in [0, max).
	// $1 = call position (injected by genLocatedHelperCall), $2 = max. Guards
	// max > 0; a non-positive value aborts located so a malformed/empty bound
	// never reaches awk. The PER-CALL seed is varied so two calls in the same
	// second (each a fresh awk process; srand() resolution is one second) are not
	// forced to the same seed and thus the same first rand():
	//   - a monotonic per-process counter __wisp_rand_ctr (the ${...:-0} default
	//     makes first use work without any prologue init, preserving the no-use
	//     byte-identity invariant -- this var is NOT added to the prologue);
	//   - awk-semantics fact: bare srand() SEEDS from the wall clock but RETURNS
	//     the PREVIOUS seed; calling srand() a SECOND time returns the time seed
	//     the first call installed, so `t = srand()` reads the time. We then
	//     reseed srand(t + p + c*7919) -- varying per second (t), per process (p
	//     = $$), and per call (c = counter); 7919 is a fixed prime multiplier.
	// p, c, and max cross the boundary as NAMED -v vars, NEVER interpolated into
	// the awk program text. int(rand()*m) maps to [0, max) without %-modulo bias.
	// Non-cryptographic; overall uniformity is bounded by awk rand() quality.
	Random: {
		id:    Random,
		deps:  []string{Fail},
		order: 99,
		src: `__wisp_random() {
	if [ "$2" -le 0 ]; then
		__wisp_fail "$1" "random: max must be positive"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__wisp_rand_ctr=$(( ${__wisp_rand_ctr:-0} + 1 ))
	__ret="$(awk -v p="$$" -v c="$__wisp_rand_ctr" -v m="$2" 'BEGIN{ srand(); t = srand(); srand(t + p + c*7919); printf "%d", int(rand() * m) }')"
}`,
	},

	// __wisp_on_exit <handler>: install an exit-code-preserving EXIT trap.
	// $1 is the mangled handler name (an inert [A-Za-z0-9_] word, already
	// expanded at call time -- NOT escaped -- so it becomes a literal word in
	// the saved trap action). \$? and \"\$__wisp_ec\" are escaped to expand at
	// TRAP-FIRE time: capture the real exit code first, run the handler, then
	// restore and exit with the original code. exit inside an EXIT trap does
	// NOT re-fire the trap (POSIX). Total void: no __wisp_fail dependency.
	OnExit: {
		id:    OnExit,
		order: 200,
		src: `__wisp_on_exit() {
	# shellcheck disable=SC2154
	trap '__wisp_ec=$?; '"$1"'; exit "$__wisp_ec"' EXIT
}`,
	},

	// __wisp_on_signal <handler> <sig>: install a named-signal trap. Both $1
	// (the mangled handler name) and $2 (the allowlist-validated signal name)
	// are inert compile-time words expanded at call time, so the saved action
	// is the bare handler name and the trap is registered for the literal
	// signal. on_signal RESUMES after the handler -- no $? capture, no exit --
	// so the handler must call exit() to terminate. Total: no __wisp_fail.
	OnSignal: {
		id:    OnSignal,
		order: 201,
		src: `__wisp_on_signal() {
	# shellcheck disable=SC2064  # $1/$2 are inert compile-time words; expand NOW (at call time) to bake in the handler name + literal signal
	trap "$1" "$2"
}`,
	},

	// __wisp_spawn <pos> <argv-id>: launch argv in a wrapper subshell that
	// backgrounds the command, captures stdout/stderr to temp files, publishes
	// the COMMAND pid (newline-terminated, so spawn's read is sentinel-gated and
	// robust to write fragmentation), then records the exit/death code to the
	// done-marker as its last action. Sets globals __wisp_sp_{pid,wrap,out,err,
	// pidf,done} for genSpawn to store into the Process handle. Located (empty
	// argv / mktemp / pid-capture cap). The pid-capture spin cannot rely on
	// write() atomicity; it accepts the pid only when `read` succeeds (the
	// newline is present) AND the content is all digits.
	Spawn: {
		id:    Spawn,
		deps:  []string{Fail},
		order: 210,
		src: `__wisp_spawn() {
	__wisp_sp_pos="$1"
	__wisp_sp_id="$2"
	eval "__wisp_sp_n=\$__wisp_a_${__wisp_sp_id}_len"
	if [ -z "$__wisp_sp_n" ] || [ "$__wisp_sp_n" -eq 0 ]; then
		__wisp_fail "$__wisp_sp_pos" "spawn: empty argv"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__wisp_sp_out="$(mktemp)" || { __wisp_fail "$__wisp_sp_pos" "spawn: mktemp failed"; [ -n "$__wisp_err_pending" ] && return; }
	__wisp_sp_err="$(mktemp)" || { rm -f "$__wisp_sp_out"; __wisp_fail "$__wisp_sp_pos" "spawn: mktemp failed"; [ -n "$__wisp_err_pending" ] && return; }
	__wisp_sp_pidf="$(mktemp)" || { rm -f "$__wisp_sp_out" "$__wisp_sp_err"; __wisp_fail "$__wisp_sp_pos" "spawn: mktemp failed"; [ -n "$__wisp_err_pending" ] && return; }
	__wisp_sp_done="$(mktemp)" || { rm -f "$__wisp_sp_out" "$__wisp_sp_err" "$__wisp_sp_pidf"; __wisp_fail "$__wisp_sp_pos" "spawn: mktemp failed"; [ -n "$__wisp_err_pending" ] && return; }
	set --
	__wisp_sp_i=0
	while [ "$__wisp_sp_i" -lt "$__wisp_sp_n" ]; do
		eval "set -- \"\$@\" \"\$__wisp_a_${__wisp_sp_id}_${__wisp_sp_i}\""
		__wisp_sp_i=$(( __wisp_sp_i + 1 ))
	done
	( "$@" >"$__wisp_sp_out" 2>"$__wisp_sp_err" </dev/null & __wisp_sp_c=$!; printf '%s\n' "$__wisp_sp_c" >"$__wisp_sp_pidf"; wait "$__wisp_sp_c"; printf '%s' "$?" >"$__wisp_sp_done" ) &
	__wisp_sp_wrap=$!
	__wisp_sp_k=0
	__wisp_sp_pid=''
	while [ "$__wisp_sp_k" -lt 10000000 ]; do
		if IFS= read -r __wisp_sp_pid < "$__wisp_sp_pidf"; then
			case "$__wisp_sp_pid" in
				''|*[!0-9]*) : ;;
				*) break ;;
			esac
		fi
		__wisp_sp_k=$(( __wisp_sp_k + 1 ))
	done
	if [ "$__wisp_sp_k" -ge 10000000 ]; then
		rm -f "$__wisp_sp_out" "$__wisp_sp_err" "$__wisp_sp_pidf" "$__wisp_sp_done"
		__wisp_fail "$__wisp_sp_pos" "spawn: failed to capture child pid"
		[ -n "$__wisp_err_pending" ] && return
	fi
}`,
	},

	// __wisp_wait <proc-id>: block until the wrapper exits (command reaped +
	// marker written by then; the wait-once 127 on a redundant join is ignored),
	// read the code from the marker and stdout/stderr from the temp files
	// (run_full trailing-newline-safe read), build a RunResult, CACHE it on the
	// Process (state=done, result=<rid>), delete the four temps, return the
	// RunResult id in __ret. Idempotent: a second call returns the cache.
	Wait: {
		id:    Wait,
		deps:  []string{Alloc},
		order: 211,
		src: `__wisp_wait() {
	__wisp_wt_id="$1"
	eval "__wisp_wt_state=\$__wisp_s_${__wisp_wt_id}_state"
	# shellcheck disable=SC2154,SC1010  # __wisp_wt_state set by eval above; 'done' here is a string operand, not a loop keyword
	if [ "$__wisp_wt_state" = done ]; then
		eval "__ret=\$__wisp_s_${__wisp_wt_id}_result"
		return
	fi
	eval "__wisp_wt_wrap=\$__wisp_s_${__wisp_wt_id}_wrap"
	eval "__wisp_wt_out=\$__wisp_s_${__wisp_wt_id}_out"
	eval "__wisp_wt_err=\$__wisp_s_${__wisp_wt_id}_err"
	eval "__wisp_wt_donef=\$__wisp_s_${__wisp_wt_id}_done"
	eval "__wisp_wt_pidf=\$__wisp_s_${__wisp_wt_id}_pidf"
	# shellcheck disable=SC2154  # __wisp_wt_wrap set by eval above
	wait "$__wisp_wt_wrap" 2>/dev/null
	# shellcheck disable=SC2154  # __wisp_wt_donef set by eval above
	__wisp_wt_code="$(cat "$__wisp_wt_donef")"
	# shellcheck disable=SC2154  # __wisp_wt_out set by eval above
	__wisp_wt_sout="$(cat "$__wisp_wt_out"; printf x)"
	# shellcheck disable=SC2154  # __wisp_wt_err set by eval above
	__wisp_wt_serr="$(cat "$__wisp_wt_err"; printf x)"
	__wisp_wt_stdout="${__wisp_wt_sout%x}"
	__wisp_wt_stderr="${__wisp_wt_serr%x}"
	__wisp_alloc
	__wisp_wt_rid="$__ret"
	eval "__wisp_s_${__wisp_wt_rid}_stdout=\$__wisp_wt_stdout"
	eval "__wisp_s_${__wisp_wt_rid}_stderr=\$__wisp_wt_stderr"
	eval "__wisp_s_${__wisp_wt_rid}_code=\$__wisp_wt_code"
	eval "__wisp_s_${__wisp_wt_id}_state=done"
	eval "__wisp_s_${__wisp_wt_id}_result=\$__wisp_wt_rid"
	# shellcheck disable=SC2154  # __wisp_wt_out/err/donef/pidf set by eval above
	rm -f "$__wisp_wt_out" "$__wisp_wt_err" "$__wisp_wt_donef" "$__wisp_wt_pidf"
	__ret="$__wisp_wt_rid"
}`,
	},

	// __wisp_is_done <proc-id>: true if state=done OR the done-marker is NON-EMPTY
	// ([ -s ], NOT [ -e ] -- the marker is mktemp'd empty). No reap, no kill -0.
	// Total.
	IsDone: {
		id:    IsDone,
		order: 212,
		src: `__wisp_is_done() {
	__wisp_id_id="$1"
	eval "__wisp_id_state=\$__wisp_s_${__wisp_id_id}_state"
	# shellcheck disable=SC2154,SC1010  # __wisp_id_state set by eval above; 'done' here is a string operand, not a loop keyword
	if [ "$__wisp_id_state" = done ]; then __ret=true; return; fi
	eval "__wisp_id_done=\$__wisp_s_${__wisp_id_id}_done"
	# shellcheck disable=SC2154  # __wisp_id_done set by eval above
	if [ -s "$__wisp_id_done" ]; then __ret=true; else __ret=false; fi
}`,
	},

	// __wisp_signal <proc-id> <sig>: guarded best-effort kill of the COMMAND pid.
	// No-op if finished (state=done OR marker NON-EMPTY) -- PID-reuse-resistant.
	// kill's status is ignored (|| :). TOTAL: never aborts. sig is a checker-
	// validated literal from signalSendAllowed.
	Signal: {
		id:    Signal,
		order: 213,
		src: `__wisp_signal() {
	__wisp_sg_id="$1"
	eval "__wisp_sg_state=\$__wisp_s_${__wisp_sg_id}_state"
	# shellcheck disable=SC2154,SC1010  # __wisp_sg_state set by eval above; 'done' here is a string operand, not a loop keyword
	if [ "$__wisp_sg_state" = done ]; then return; fi
	eval "__wisp_sg_done=\$__wisp_s_${__wisp_sg_id}_done"
	# shellcheck disable=SC2154  # __wisp_sg_done set by eval above
	if [ -s "$__wisp_sg_done" ]; then return; fi
	eval "__wisp_sg_pid=\$__wisp_s_${__wisp_sg_id}_pid"
	# shellcheck disable=SC2154  # __wisp_sg_pid set by eval above
	kill -"$2" "$__wisp_sg_pid" 2>/dev/null || :
}`,
	},

	// __wisp_wait_any <pos> <array-id> <poll>: poll the Process[] array in LIST
	// ORDER, returning (in __ret) the first whose state=done OR marker NON-EMPTY,
	// checking immediately before any sleep. poll>0 sleeps poll whole seconds
	// between rounds; poll==0 is a tight loop. Located: empty array / negative
	// poll abort (attributed to wait_any, BEFORE any sleep). Reads markers only;
	// no reap, so it composes with a later wait.
	WaitAny: {
		id:    WaitAny,
		deps:  []string{Fail},
		order: 214,
		src: `__wisp_wait_any() {
	__wisp_wa_id="$2"
	__wisp_wa_poll="$3"
	eval "__wisp_wa_n=\$__wisp_a_${__wisp_wa_id}_len"
	if [ -z "$__wisp_wa_n" ] || [ "$__wisp_wa_n" -eq 0 ]; then
		__wisp_fail "$1" "wait_any: empty process list"
		[ -n "$__wisp_err_pending" ] && return
	fi
	if [ "$__wisp_wa_poll" -lt 0 ]; then
		__wisp_fail "$1" "wait_any: poll_secs must be >= 0"
		[ -n "$__wisp_err_pending" ] && return
	fi
	while :; do
		__wisp_wa_i=0
		while [ "$__wisp_wa_i" -lt "$__wisp_wa_n" ]; do
			eval "__wisp_wa_p=\$__wisp_a_${__wisp_wa_id}_${__wisp_wa_i}"
			# shellcheck disable=SC2154  # __wisp_wa_p set by eval above
			eval "__wisp_wa_state=\$__wisp_s_${__wisp_wa_p}_state"
			# shellcheck disable=SC2154,SC1010  # __wisp_wa_state set by eval above; 'done' here is a string operand, not a loop keyword
			if [ "$__wisp_wa_state" = done ]; then __ret="$__wisp_wa_p"; return; fi
			# shellcheck disable=SC2154  # __wisp_wa_p set by eval above
			eval "__wisp_wa_done=\$__wisp_s_${__wisp_wa_p}_done"
			# shellcheck disable=SC2154  # __wisp_wa_done set by eval above
			if [ -s "$__wisp_wa_done" ]; then __ret="$__wisp_wa_p"; return; fi
			__wisp_wa_i=$(( __wisp_wa_i + 1 ))
		done
		if [ "$__wisp_wa_poll" -gt 0 ]; then sleep "$__wisp_wa_poll"; fi
	done
}`,
	},

	// __wisp_make_fifo <pos> <path>: create a FIFO via `mkfifo -- "$path"`. The
	// -- guards a leading-dash path; path flows as inert "$2". Located abort on
	// failure (e.g. path already exists).
	MakeFifo: {
		id:    MakeFifo,
		deps:  []string{Fail},
		order: 215,
		src: `__wisp_make_fifo() {
	mkfifo -- "$2" || __wisp_fail "$1" "make_fifo: cannot create fifo at $2"
	[ -n "$__wisp_err_pending" ] && return
}`,
	},

	// __wisp_pipe_exec <stage-id>: rebuild that stage's argv into the positionals
	// (run-family pattern) and exec it. Runs inside its own ( ) pipeline segment, so
	// exec replaces only that subshell. Empty inner argv -> exit 127 (command-not-found
	// semantics; a segment cannot drive a located abort of the outer pipe call).
	// LOAD-BEARING: capture the stage id into __wisp_pe_id BEFORE `set --` (which clears
	// the positionals) -- the rebuild loop must reference ${__wisp_pe_id}, NOT ${1},
	// because after `set --` $1 is empty and the loop would expand the empty handle id.
	// This mirrors the merged __wisp_spawn (__wisp_sp_id="$2" captured first).
	PipeExec: {
		id:    PipeExec,
		order: 216,
		src: `__wisp_pipe_exec() {
	__wisp_pe_id="$1"
	eval "__wisp_pe_n=\$__wisp_a_${__wisp_pe_id}_len"
	if [ -z "$__wisp_pe_n" ] || [ "$__wisp_pe_n" -eq 0 ]; then exit 127; fi
	set --
	__wisp_pe_i=0
	while [ "$__wisp_pe_i" -lt "$__wisp_pe_n" ]; do
		eval "set -- \"\$@\" \"\$__wisp_a_${__wisp_pe_id}_${__wisp_pe_i}\""
		__wisp_pe_i=$(( __wisp_pe_i + 1 ))
	done
	exec "$@"
}`,
	},

	// __wisp_pipe <pos> <outer-id>: walk the outer string[][] array and BUILD a pipeline
	// string of ( __wisp_pipe_exec <stage-id> [redirs] ) segments joined by |, with
	// PER-SEGMENT redirects (first stage </dev/null, last stage >t1 2>t2 -- NEVER a
	// trailing whole-pipeline redirect, which binds non-portably). Only compiler-controlled
	// tokens + digit-guarded integer stage ids enter the string; argv data never does.
	// eval the string; $? is the last stage's status; read the temps run_full-style.
	Pipe: {
		id:    Pipe,
		deps:  []string{Fail, PipeExec},
		order: 217,
		src: `__wisp_pipe() {
	__wisp_pp_pos="$1"
	__wisp_pp_outer="$2"
	eval "__wisp_pp_tot=\$__wisp_a_${__wisp_pp_outer}_len"
	if [ -z "$__wisp_pp_tot" ] || [ "$__wisp_pp_tot" -eq 0 ]; then
		__wisp_fail "$__wisp_pp_pos" "pipe: empty stages"
		[ -n "$__wisp_err_pending" ] && return
	fi
	__wisp_pp_t1="$(mktemp)" || { __wisp_fail "$__wisp_pp_pos" "pipe: mktemp failed"; [ -n "$__wisp_err_pending" ] && return; }
	__wisp_pp_t2="$(mktemp)" || { rm -f "$__wisp_pp_t1"; __wisp_fail "$__wisp_pp_pos" "pipe: mktemp failed"; [ -n "$__wisp_err_pending" ] && return; }
	__wisp_pp_str=""
	__wisp_pp_k=0
	while [ "$__wisp_pp_k" -lt "$__wisp_pp_tot" ]; do
		eval "__wisp_pp_sid=\$__wisp_a_${__wisp_pp_outer}_${__wisp_pp_k}"
		# shellcheck disable=SC2154
		case "$__wisp_pp_sid" in
			''|*[!0-9]*) rm -f "$__wisp_pp_t1" "$__wisp_pp_t2"; __wisp_fail "$__wisp_pp_pos" "pipe: bad stage id"; return ;;
		esac
		__wisp_pp_seg="( __wisp_pipe_exec $__wisp_pp_sid"
		[ "$__wisp_pp_k" -eq 0 ] && __wisp_pp_seg="$__wisp_pp_seg </dev/null"
		[ "$(( __wisp_pp_k + 1 ))" -eq "$__wisp_pp_tot" ] && __wisp_pp_seg="$__wisp_pp_seg >\"\$__wisp_pp_t1\" 2>\"\$__wisp_pp_t2\""
		__wisp_pp_seg="$__wisp_pp_seg )"
		if [ -z "$__wisp_pp_str" ]; then __wisp_pp_str="$__wisp_pp_seg"; else __wisp_pp_str="$__wisp_pp_str | $__wisp_pp_seg"; fi
		__wisp_pp_k=$(( __wisp_pp_k + 1 ))
	done
	eval "$__wisp_pp_str"
	__wisp_rf_code=$?
	__wisp_pp_so="$(cat "$__wisp_pp_t1"; printf x)"
	__wisp_pp_se="$(cat "$__wisp_pp_t2"; printf x)"
	__wisp_rf_stdout="${__wisp_pp_so%x}"
	__wisp_rf_stderr="${__wisp_pp_se%x}"
	rm -f "$__wisp_pp_t1" "$__wisp_pp_t2"
}`,
	},

	// Builtin funcref wrappers (__wisp_builtin_<name>) are added to this registry
	// by builtinref.go's init from the wrapper spec table.
}

// Emit returns the POSIX-sh source for the requested helpers and their
// transitive dependencies, in dependency order (each helper after the ones it
// depends on) and deterministically (independent of request order). Unknown IDs
// are ignored. Emit(nil) returns "".
func Emit(ids []string) string {
	return EmitMode(ids, false, false)
}

// EmitMode is Emit with an error-handling mode switch (M5). When errMode is
// true the program uses try/throw, so __wisp_fail is emitted as the mode-aware
// body (depth 0 -> located exit; depth > 0 -> set pending + return) instead of
// the M1 unconditional exit. The function NAME is unchanged, so every helper
// that calls __wisp_fail keeps working. When errMode is false the M1 fail body
// is emitted and the output is byte-for-byte the M4 shape (zero overhead).
func EmitMode(ids []string, errMode, usesWrap bool) string {
	selected := map[string]bool{}
	var visit func(id string)
	visit = func(id string) {
		if selected[id] {
			return
		}
		h, ok := registry[id]
		if !ok {
			return
		}
		selected[id] = true
		for _, d := range h.deps {
			visit(d)
		}
	}
	for _, id := range ids {
		visit(id)
	}
	if len(selected) == 0 {
		return ""
	}

	// Deterministic order: by the helper's stable `order` rank, with the ID as a
	// tiebreaker so equal-rank helpers (fail/throw at rank 0) emit in a fixed
	// order regardless of map iteration. Because every dependency has a
	// strictly-lower-or-equal order than its dependents, ordering by rank also
	// satisfies the dependency constraint.
	chosen := make([]helper, 0, len(selected))
	for id := range selected {
		chosen = append(chosen, registry[id])
	}
	sort.Slice(chosen, func(i, j int) bool { return helperLess(chosen[i], chosen[j]) })

	var b strings.Builder
	for i, h := range chosen {
		if i > 0 {
			b.WriteString("\n")
		}
		src := h.src
		if errMode && h.id == Fail {
			src = failModeSrc // mode-aware fail in error mode
			if usesWrap {
				src = failModeCauseSrc // also clear __wisp_err_cause on a fault
			}
		} else if !errMode {
			// Strip the error-mode short-circuit lines from fallible helpers: in M1
			// mode __wisp_fail exits before reaching them (dead code), so removing
			// them keeps the output byte-for-byte the M4 shape (zero-overhead, no
			// __wisp_err_pending leak). EmitMode is the single rewrite point.
			src = stripShortCircuits(src)
		}
		b.WriteString(src)
		b.WriteString("\n")
	}
	return b.String()
}

// shortCircuitLine is the exact (untrimmed-of-indent) error-mode guard line
// added after a __wisp_fail call inside a fallible helper. In M1 mode it is
// dead code (fail exits first); stripShortCircuits removes it so a non-error
// program's output is byte-for-byte the M4 shape.
const shortCircuitLine = `[ -n "$__wisp_err_pending" ] && return`

// loopPendingBreakLine is the loop-scoped counterpart of shortCircuitLine: at
// the top of a counted while-loop body in a bespoke generic-funcref wrapper
// (map_array, filter_array, each_array, reduce_array, sort_by_array,
// find_array, any_array, all_array, count_where_array), it stops further
// callback invocations once a fault raised by a prior callback call is
// pending, mirroring codegen's loopPendingBreak() (internal/codegen/error.go)
// used by every direct-call array loop via beginArrayLoop. Without this, a
// funcref-invoked wrapper would keep iterating and re-invoking the callback
// after a fault, diverging from the direct-call lowering's fail-fast
// semantics. In M1 mode it is dead code (no pending flag is ever set);
// stripShortCircuits removes it for the same byte-for-byte M4 reason as
// shortCircuitLine.
const loopPendingBreakLine = `[ -n "$__wisp_err_pending" ] && break`

// stripShortCircuits removes every short-circuit guard line from a helper body
// (a line whose content after leading tabs is exactly shortCircuitLine or
// loopPendingBreakLine). Used in non-error mode only.
func stripShortCircuits(src string) string {
	lines := strings.Split(src, "\n")
	out := lines[:0]
	for _, ln := range lines {
		trimmed := strings.TrimLeft(ln, "\t")
		if trimmed == shortCircuitLine || trimmed == loopPendingBreakLine {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// helperLess orders helpers by rank, then by ID, for deterministic emission.
func helperLess(a, b helper) bool {
	if a.order != b.order {
		return a.order < b.order
	}
	return a.id < b.id
}

// IDs returns every helper ID in the registry, sorted by emission order. Used by
// codegen tests to assert tree-shaking (that a minimal program omits helpers).
func IDs() []string {
	out := make([]string, 0, len(registry))
	for id := range registry {
		out = append(out, id)
	}
	// sort by order for determinism (stable: tie order among equal-order ids is
	// the map-iteration order, matching the prior insertion sort)
	sort.SliceStable(out, func(i, j int) bool { return registry[out[i]].order < registry[out[j]].order })
	return out
}
