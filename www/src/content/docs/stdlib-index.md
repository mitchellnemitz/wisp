---
title: Standard library index
---


A terse, complete index of the wisp standard library: what exists and what is
proposed. It is the working list for future stdlib milestones, not a promise of
final signatures. Every name is `snake_case` (the permanent convention; see
[the stdlib guide](/guide/stdlib/#naming-convention)).

Legend: `[x]` implemented, `[ ]` proposed. Implemented functions are documented
in the [standard library reference](/guide/stdlib/). Signatures use `T`/`U` for
generic element types and `K`/`V` for dict key/value types.

A `[ref]` tag below marks a builtin that is also referenceable as a first-class
`fn`-value (eta-expansion via a thin wrapper). Referenceable builtins fall into
three classes: monomorphic (no annotation needed), overloaded (annotation
selects the arm), and generic (annotation selects the container shape). All
other builtins are call-only; wrap them in a `fn` to pass them as a value. See
the [function references](/guide/language/#function-references) section for
the full rules, per-class examples, and the fallible-funcref position-
degradation note.

## Conversions

- `[x] to_string(x: int | float | bool | string) -> string`
- `[x] to_int(x: string | float) -> int`
- `[x] to_float(x: int | string) -> float`
- `[x] to_bool(x: int | float | string) -> bool`
- `[x] parse_int(s: string) -> Optional[int]` -- parse, or None on bad input (no abort); fold with `unwrap_or`
- `[x] parse_float(s: string) -> Optional[float]`
- `[x] parse_bool(s: string) -> Optional[bool]`

## Strings

- `[x] length(s: string) -> int`
- `[x] [ref] string.lower(s: string) -> string`
- `[x] [ref] string.upper(s: string) -> string`
- `[x] [ref] string.trim(s: string) -> string`
- `[x] string.replace(s: string, search: string, with: string) -> string` -- all occurrences
- `[x] string.split(s: string, sep: string) -> string[]`
- `[x] string.join(parts: string[], sep: string) -> string`
- `[x] string.contains(s: string, sub: string) -> bool`
- `[x] string.starts_with(s: string, pre: string) -> bool`
- `[x] string.ends_with(s: string, suf: string) -> bool`
- `[x] string.index_of(s: string, sub: string) -> Optional[int]` -- Some(first byte index) or None; also `string.index_of(xs: T[], x: T) -> Optional[int]` for arrays of comparable element types
- `[x] string.repeat(s: string, n: int) -> string`
- `[x] string.substring(s: string, start: int, end: int) -> string` -- byte range [start, end)
- `[x] string.char_at(s: string, i: int) -> string` -- the one-byte (or one-rune) slice at i
- `[x] string.last_index_of(s: string, sub: string) -> Optional[int]` -- Some(rightmost byte index) or None
- `[x] string.count(s: string, sub: string) -> int` -- non-overlapping occurrences
- `[x] string.replace_first(s: string, search: string, with: string) -> string`
- `[x] string.trim_start(s: string) -> string`
- `[x] string.trim_end(s: string) -> string`
- `[x] string.trim_prefix(s: string, pre: string) -> string` -- remove pre if present
- `[x] string.trim_suffix(s: string, suf: string) -> string`
- `[x] string.pad_start(s: string, width: int, fill: string) -> string`
- `[x] string.pad_end(s: string, width: int, fill: string) -> string`
- `[x] string.lines(s: string) -> string[]` -- split on newlines
- `[x] string.is_empty(s: string) -> bool`
- `[x] string.reverse(s: string) -> string` -- (array `reverse` already exists; strings differ)
- `[x] string.ord(s: string) -> int` / `[x] string.chr(code: int) -> string` -- byte/codepoint conversion

Decisions to settle for strings: byte vs rune (UTF-8) semantics for `length`,
`substring`, `char_at`, indexing. Today `length` is bytes; a rune-aware set may
warrant separate `byte_*` vs `char_*` names.

## Numbers

- `[x] math.abs(x: int) -> int` / `math.abs(x: float) -> float`
- `[x] math.min(a, b)` / `[x] math.max(a, b)` -- same ordered scalar type (int/float/bool/string/value-enum; funcref value form int/float only)
- `[x] math.clamp(x, lo, hi)` -- same numeric type
- `[x] math.sign(x: int) -> int` / `math.sign(x: float) -> int` -- -1, 0, 1
- `[x] math.pow(base: float, exp: float) -> float`
- `[x] [ref] math.sqrt(x: float) -> float` -- Newton's method (basic arithmetic, no awk math); exact on perfect squares, within ~1 ulp otherwise; domain x >= 0 (negative aborts located)
- `[x] math.floor(x: float) -> int` / `[x] math.ceil(x: float) -> int` / `[x] math.round(x: float) -> int`
- `[x] math.trunc(x: float) -> int` -- toward zero (to_int(float) already truncates)
- `[x] math.gcd(a: int, b: int) -> int` / `[x] math.lcm(a: int, b: int) -> int`
- `[x] math.int_min() -> int` / `[x] math.int_max() -> int` -- platform limits, computed at runtime by detecting shell integer width
- `[x] [ref] math.exp(x: float) -> float` -- e^x, pure-arithmetic Taylor series (no awk libm); a result outside the representable float range aborts located; use `exp(1.0)` for Euler's number -- there is no `e` constant, so `e` stays free as the conventional error-binding name in `catch (e)`
- `[x] [ref] math.ln(x: float) -> float` -- natural log; domain x > 0 (non-positive aborts located, no hang)
- `[x] math.log10(x: float) -> float` -- base-10 log; domain x > 0
- `[x] math.log2(x: float) -> float` -- base-2 log; domain x > 0
- `[x] math.pi() -> float` -- compile-time constant 3.141592653589793 (no awk call)
- `[x] string.format_float(x: float, decimals: int) -> string` -- render x to exactly `decimals` decimal places via awk `printf` (f-conversion at the given precision); decimals must be >= 0 (negative aborts located); rounding follows platform printf (round-half-to-even on most libc, NOT guaranteed half-up); contrast with `to_string(float)` (full %.17g precision)

Trigonometry (sin/cos/tan/asin/acos/atan/atan2) is deferred: awk trig is absent on
no-libm busybox builds, so each function needs a new pure-arithmetic implementation.
All other math ops above are pure-arithmetic (basic +,-,*,/ only).

## Arrays

- `[x] length(xs: T[]) -> int`
- `[x] array.is_empty(xs: T[]) -> bool`
- `[x] array.push(xs: T[], v: T) -> void`
- `[x] string.contains(xs: T[], x: T) -> bool` -- `contains` has one compiler
  entry, under `string`; the array form delegates through the same overload
  resolver and works identically for arrays of comparable elements
- `[x] array.reverse(xs: T[]) -> T[]`
- `[x] array.map(xs: T[], f: fn(T) -> U) -> U[]`
- `[x] array.filter(xs: T[], f: fn(T) -> bool) -> T[]`
- `[x] array.each(xs: T[], f: fn(T) -> void) -> void`
- `[x] array.reduce(xs: T[], init: U, f: fn(U, T) -> U) -> U`
- `[x] array.sort(xs: T[]) -> T[]` -- ascending, any ordered scalar element type (int/float/bool/string/value-enum)
- `[x] array.sort_by(xs: T[], less: fn(T, T) -> bool) -> T[]`
- `[x] array.find(xs: T[], f: fn(T) -> bool) -> Optional[int]` -- Some(first matching index) or None
- `[x] array.any(xs: T[], f: fn(T) -> bool) -> bool` / `[x] array.all(xs: T[], f: fn(T) -> bool) -> bool`
- `[x] array.count_where(xs: T[], f: fn(T) -> bool) -> int`
- `[x] array.slice(xs: T[], start: int, end: int) -> T[]`
- `[x] array.concat(a: T[], b: T[]) -> T[]`
- `[x] array.flatten(xs: T[][]) -> T[]`
- `[x] array.unique(xs: T[]) -> T[]` -- comparable element types, order-preserving
- `[x] array.first(xs: T[]) -> T` / `[x] array.last(xs: T[]) -> T` -- abort on empty (catchable)
- `[x] array.take(xs: T[], n: int) -> T[]` / `[x] array.drop(xs: T[], n: int) -> T[]`
- `[x] array.pop(xs: T[]) -> T` -- remove and return last; abort on empty
- `[x] array.remove_at(xs: T[], i: int) -> void` / `[x] array.insert_at(xs: T[], i: int, v: T) -> void`
- `[x] array.sum(xs: int[]) -> int` / `array.sum(xs: float[]) -> float`
- `[x] array.range(n: int) -> int[]` -- 0..n-1; and/or `range_from(start, end, step)`
- `[x] array.zip(a: T[], b: U[]) -> (T, U)[]` -- IMPLEMENTED (tuples milestone)

## Dicts

- `[x] dict.has(d: {K: V}, k: K) -> bool`
- `[x] dict.is_empty(d: {K: V}) -> bool`
- `[x] dict.keys(d: {K: V}) -> K[]`
- `[x] dict.values(d: {K: V}) -> V[]` -- insertion order
- `[x] dict.get(d: {K: V}, k: K) -> Optional[V]` -- Some(value) if present, else None; fold with `unwrap_or` for a value-or-fallback
- `[x] dict.remove(d: {K: V}, k: K) -> void`
- `[x] dict.size(d: {K: V}) -> int`
- `[x] dict.merge(a: {K: V}, b: {K: V}) -> {K: V}` -- b wins on conflict
- `[x] dict.clear(d: {K: V}) -> void`

## I/O and system

- `[x] print(msg: string, to: int = stdout) -> void`
- `[x] env.get(name: string) -> Optional[string]` / `[x] env.has(name: string) -> bool`
- `[x] fs.read_file(path: string) -> string`
- `[x] fs.write_file(path: string, content: string) -> void`
- `[x] fs.append_file(path: string, content: string) -> void`
- `[x] process.run(argv: string[]) -> string` -- stdout; aborts on nonzero exit
- `[x] exit(code: int) -> void`
- `[x] env.set(name: string, value: string) -> void` -- export name=value into the process env; all later children inherit it; name must match `[A-Za-z_][A-Za-z0-9_]*` (invalid aborts located); mutates global state (contrast with `process.run_env`)
- `[x] env.unset(name: string) -> void` -- remove name from the process env; no-op success if not set; same name validation as `env.set`
- `[x] read_line() -> Optional[string]` -- one line from stdin; None on EOF
- `[x] read_stdin() -> string` -- all of stdin
- `[x] read_secret(prompt: string) -> Optional[string]` -- print prompt to stderr, read one line with echo suppressed (stty -echo, best-effort; no-op when stdin is not a TTY); Some(line) or None on EOF
- `[x] [ref] fs.file_exists(path: string) -> bool` / `[x] [ref] fs.is_dir(path: string) -> bool`
- `[x] [ref] fs.is_file(path: string) -> bool` -- true for a regular file or symlink to one (follows symlinks, like `test -f`); total
- `[x] [ref] fs.is_symlink(path: string) -> bool` -- true iff `path` itself is a symlink (does not follow); total
- `[x] fs.file_size(path: string) -> int` -- byte count via `wc -c`; leading whitespace stripped; aborts located on missing/unreadable/dir
- `[x] fs.read_link(path: string) -> Optional[string]` -- immediate symlink target via `readlink -- "$path"`; non-symlink or any failure -> None (never aborts); single-level, not canonicalized; `readlink` not strict POSIX but present on every target
- `[x] fs.chmod(path: string, mode: string) -> void` -- `chmod -- "$mode" "$path"`; single `--` guards leading-dash path; located abort on failure; statement-only (Void)
- `[x] fs.symlink(target: string, link_path: string) -> void` -- `ln -s -- "$target" "$link_path"`; target stored verbatim (dangling links allowed); located abort if link_path exists; statement-only (Void)
- `[x] fs.symlink_force(target: string, link_path: string) -> void` -- create-or-replace symlink; removes and re-creates link_path if it is a symlink or regular file; located abort if link_path is a real directory; NOT atomic, DESTRUCTIVE-ON-FAILURE (removes old link_path first); statement-only (Void)
- `[x] fs.glob(pattern: string) -> string[]` -- shell pathname expansion; no match -> empty array (not the literal pattern); matched names are inert data; pattern word-splits (a pattern with a space is two globs); total; zsh parity via emulate-sh banner
- `[x] fs.temp_file() -> string` -- `mktemp`; located abort on failure; caller owns cleanup via `remove_file`; `mktemp` not strict POSIX but present on every target
- `[x] fs.temp_dir() -> string` -- `mktemp -d`; located abort on failure; caller owns cleanup via `remove_dir`
- `[x] fs.list_dir(path: string) -> string[]`
- `[x] fs.make_dir(path: string) -> void` -- mkdir -p semantics
- `[x] fs.remove_file(path: string) -> void` / `[x] fs.remove_dir(path: string) -> void`
- `[x] fs.rename(from: string, to: string) -> void` -- enables atomic write-via-rename
- `[x] fs.cwd() -> string` / `[x] fs.change_dir(path: string) -> void`
- `[x] fs.program_path() -> string` -- the script's invocation path ($0 captured at top level before main; same value from any call depth; meaningful for built artifacts)
- `[x] [ref] fs.dir_name(path: string) -> string` -- directory portion, POSIX dirname semantics, pure string (no external process)
- `[x] [ref] fs.base_name(path: string) -> string` -- final component, POSIX basename semantics, pure string (no external process)
- `[x] fs.which(name: string) -> Optional[string]` -- resolved path, or None if not found
- `[x] process.run_status(argv: string[]) -> int` -- run, return exit code, do not abort
- `[x] process.run_full(argv: string[]) -> RunResult` -- stdout + stderr + code; stdin=/dev/null
- `[x] process.run_input(argv: string[], stdin: string) -> string` -- run argv with stdin fed as exact bytes via printf %s (no added newline); returns stdout (trailing newlines stripped); aborts on nonzero child exit; empty argv aborts
- `[x] process.run_input_full(argv: string[], stdin: string) -> RunResult` -- same stdin feed; returns RunResult{stdout,stderr,code} without aborting; empty argv aborts
- `[x] process.run_env(argv: string[], env: {string: string}) -> string` -- run with augmented child env (adds/overrides; does not clear); aborts on nonzero exit like `run`; env NAME must match `[A-Za-z_][A-Za-z0-9_]*` (invalid name aborts located); VALUES are arbitrary; for a supported argv[0], `run_env(argv, {})` == `run(argv)`; argv[0] limitation: a name starting with `-` or containing `=` is unsupported (use a path-qualified form); uses `env` (POSIX); non-goals: no clear-slate, no unset
- `[x] process.run_env_status(argv: string[], env: {string: string}) -> int` -- run with augmented child env, return exit code, do not abort; same NAME validation and empty-argv abort as `run_env`
- `[x] process.run_env_full(argv: string[], env: {string: string}) -> RunResult` -- run with augmented child env, return RunResult (stdout+stderr+code), do not abort; same NAME validation and empty-argv abort as `run_env`
- `[x] process.exec_command(argv: string[]) -> void` -- replace the shell process with argv via POSIX exec (same PID, inherits all fds/signals); never returns on success; empty argv aborts located; exec failure exits 127/126 (not a wisp abort)
- Non-goal: `mtime` (no POSIX-portable epoch source; use `run(["stat", ...])` platform-specifically)

## Signals and traps

Signal and exit HOOKS for cleanup and graceful shutdown. Both lower to the POSIX
`trap` builtin. The handler must be a user-declared `fn() -> void` (nullary, void
return); a builtin or wrong-type handler is a compile error. Both are
statement-only (void, not usable as a value). Replace semantics: a later call for
the same signal/EXIT replaces the first (POSIX trap). To run multiple steps,
compose one handler fn. Not full process supervision (forwarding/reaping a
long-running child is a future feature).

- `[x] on_exit(handler: fn() -> void) -> void` -- register `handler` to run when
  the process exits (normal completion, `exit(n)`, or located abort); the
  original exit code is preserved (`__wisp_ec=$?; handler; exit "$__wisp_ec"`);
  does NOT fire on an uncaught terminating signal (use `on_signal` + `exit` for
  that); statement-only; total (cannot fail)
- `[x] on_signal(sig: string, handler: fn() -> void) -> void` -- register
  `handler` to run when the signal named `sig` is received; execution RESUMES
  after the handler returns (a caught signal does not terminate); to terminate,
  the handler calls `exit(n)`; `sig` must be a STRING LITERAL from
  `{INT, TERM, HUP, QUIT, USR1, USR2}` (compile-time validated; EXIT/KILL/STOP
  and any non-literal are compile errors); statement-only; total (no runtime
  validation); `exit()` from an `on_signal` handler triggers the `on_exit`
  handler (the canonical shared-cleanup shutdown pattern)
- Non-goal: no handler accumulation/stacking (replace-only); no `EXIT` via
  `on_signal` (use `on_exit`); no `KILL`/`STOP` (untrappable); no handler
  arguments; no `untrap`/ignore/default; no re-entrancy protection

## Background processes

Explicit async: launch a command in the background, then poll or block on it.
No promises, no event loop, no callbacks. `Process` is an opaque handle (only
`.pid: int` is readable; no compare/interpolate/arithmetic). REAPING CONTRACT:
the caller must `wait` every spawned process to reclaim its temp files and reap
the zombie; a never-waited Process leaks until the shell exits. `wait_any`
returns ONE process -- wait the others too. wisp manages the spawned command,
not grandchildren the command itself forks (same boundary as `run`).

- `[x] process.spawn(argv: string[]) -> Process` -- launch `argv` in the background;
  stdout/stderr captured to temp files, stdin `/dev/null`. Empty argv aborts.
- `[x] process.wait(p: Process) -> RunResult` -- block until `p` finishes; returns its
  RunResult. IDEMPOTENT (cached; reclaims temps on the first call).
- `[x] process.is_done(p: Process) -> bool` -- non-blocking, non-reaping liveness check
  (true once finished, normal or signaled).
- `[x] process.pid_alive(pid: int) -> bool` -- total liveness probe: true iff a process with `pid` exists and is signalable by the caller (POSIX `kill -0`); nonexistent PID and EPERM both return false; never aborts. For a process wisp itself spawned, prefer `is_done` (PID-reuse-resistant).
- `[x] process.signal(p: Process, sig: string) -> void` -- send `sig` to the spawned
  command; `sig` must be a STRING LITERAL from
  `{INT, TERM, HUP, QUIT, USR1, USR2, KILL, STOP, CONT}`. Best-effort; no-op once
  finished. PID-reuse-RESISTANT (a documented microsecond residual race,
  unclosable without process groups). Observes the command's CURRENT signal
  disposition (a not-yet-installed handler yields the default action, so an exact
  trapped exit code is not guaranteed from spawn-then-signal).
- `[x] process.wait_any(ps: Process[], poll_secs: int) -> Process` -- poll `ps` in LIST
  order, return the first observed done; `poll_secs` is WHOLE SECONDS (sub-second
  sleep is non-portable), must be `>= 0` (0 = tight poll). Empty list / negative
  poll abort.
- `[x] process.make_fifo(path: string) -> void` -- create a named pipe (`mkfifo`) for
  IPC; aborts if the path exists.

## Pipelines

- `[x] process.pipe(stages: string[][]) -> RunResult` -- run a pipeline `stage1 | stage2 | ...`, capturing the last stage's stdout/stderr/code. Last-stage status only (no PIPESTATUS; intermediate failures invisible). First stage's stdin is `/dev/null`; only the last stage's stderr is captured (earlier stages' stderr inherits to the script's stderr). An empty outer array aborts located; an empty inner argv yields 127 from that stage. argv is injection-safe (inert).

## Time and randomness

- `[x] now() -> int` -- current Unix epoch in whole seconds via `date +%s`; `date +%s` is a near-universal extension present on every dash/busybox ash/bash/zsh target; wall clock (not monotonic -- can move backward under NTP/adjustment)
- `[x] sleep(secs: int) -> void` -- pause for `secs` whole seconds via the external `sleep`; `secs >= 0` required (negative aborts located: `sleep: negative duration`); statement-only (Void, not usable as a value); sub-second/fractional sleep is not supported (not portable)
- `[x] math.random(max: int) -> int` -- non-cryptographic random int in `[0, max)`; `max > 0` required (non-positive aborts located: `random: max must be positive`); uses awk `srand`/`rand` (works on dash, which has no `$RANDOM`; present in busybox awk); seed varied per call via wall-clock time + process id + monotonic per-process counter; not for security/tokens; two runs in the same second with the same PID reproduce the sequence (documented non-crypto limitation)

## Argument parsing

- `[x] parse_args(args: string[], value_flags: string[]) -> ({string: string}, string[], string[])` -- pure; splits args into (valued-flag dict, boolean-switch array, positional array); `value_flags` names which flags consume the next token; space form `F VALUE` and equals form `F=VALUE` both supported; `--` terminates (space-form value-flag consumes a following `--` as its value); empty `F=` yields present empty string; last occurrence wins; lone `-` is a positional; non-goals: no short bundling, no alias resolution, no type coercion, no help gen

## Errors

- `[x] error(msg: string) -> error`
- `[x] throw <error>` (keyword) / `[x] try { } catch (e) { } finally { }`
- `[x] error_with(code: int, msg: string) -> error`
- `[x] wrap(err: error, msg: string) -> error` -- new error with `message=msg`, `code=0`, carrying `err` as its cause; `err` is unchanged; no message concatenation (reach the inner via `cause`); cause survives value paths (`throw`/`catch`, `unwrap_err`, `map_err`/`or_else`) but not the fault path (`unwrap` clears it)
- `[x] cause(err: error) -> Optional[error]` -- `Some(inner)` if `err` was produced by `wrap`, else `None`; walk the chain with `match (cause(cur)) { case Some(inner) {...} case None {...} }`

## Debugging

- `[x] debug(x) -> string` -- structural render of ANY value (scalars, `T[]`, `Optional`/`Result`, dict, struct, tuple, `error`, funcref, `RunResult`); does not print. Idiom: `print(debug(x), stderr)`

## Optional and Result access

- `[x] is_some(o: Optional[T]) -> bool` / `[x] is_none(o: Optional[T]) -> bool`
- `[x] is_ok(r: Result[T]) -> bool` / `[x] is_err(r: Result[T]) -> bool`
- `[x] unwrap(o: Optional[T]) -> T` / `unwrap(r: Result[T]) -> T` -- abort on None/Err (catchable)
- `[x] unwrap_or(o: Optional[T], fallback: T) -> T` / `unwrap_or(r: Result[T], fallback: T) -> T` -- total; fallback evaluated eagerly
- `[x] unwrap_err(r: Result[T]) -> error` -- the carried error, or abort on Ok (catchable)

`Optional[T]` supports `==`/`!=` when `T` is comparable (`int`/`bool`/`string`/
`float`, an enum, or nested comparable `Optional`); `Result` and aggregate
carriers stay non-comparable.

## Combinators

Overloaded over Optional and/or Result; the function runs only on the activating branch.

- `[x] array.map(o: Optional[T], f: fn(T) -> U) -> Optional[U]` / `array.map(r: Result[T], f: fn(T) -> U) -> Result[U]` (U != void) -- success-side transform (array form above)
- `[x] and_then(o: Optional[T], f: fn(T) -> Optional[U]) -> Optional[U]` / `and_then(r: Result[T], f: fn(T) -> Result[U]) -> Result[U]` -- flat-map
- `[x] array.filter(o: Optional[T], f: fn(T) -> bool) -> Optional[T]` -- Optional-only (array form above); not defined over Result

`map` and `filter` here are the same `array.map`/`array.filter` builtins
applied through `Optional`'s value-or-empty representation, not a separate
combinator.

- `[x] or_else(o: Optional[T], f: fn() -> Optional[T]) -> Optional[T]` / `or_else(r: Result[T], f: fn(error) -> Result[T]) -> Result[T]` -- lazy fallback; Err passes the error handle to f
- `[x] map_err(r: Result[T], f: fn(error) -> error) -> Result[T]` -- Result-only; transform the Err side

## Regular expressions

POSIX ERE, whole-match only, byte-based under `LC_ALL=C`. A malformed pattern is a catchable located abort.

- `[x] regex.matches(s: string, pattern: string) -> bool`
- `[x] regex.find(s: string, pattern: string) -> Optional[string]` -- first whole match, or None
- `[x] regex.find_all(s: string, pattern: string) -> string[]` -- every non-empty whole match, left to right
- `[x] regex.replace(s: string, pattern: string, replacement: string) -> string` -- all matches; `&` is the whole match, `\&` a literal `&`; no group backrefs

## JSON (`import "json"`)

The first core module: a reserved namespace opted into with `import "json"`.
`json.Value` is an opaque handle storing canonical JSON text (numbers/strings
verbatim, so no precision loss). See the [stdlib guide](/guide/stdlib/#json-import-json).

- `[x] json.decode[T](s: string) -> T` -- validate + parse; `T` defaults to `json.Value`, or `string`/`int`/`float`/`bool`; malformed aborts located
- `[x] json.encode(v: json.Value) -> string` -- canonical text
- `[x] json.from_int/from_float/from_bool/from_string(x) -> json.Value`, `json.null() -> json.Value`
- `[x] json.array(json.Value[]) -> json.Value`, `json.object(dict[string, json.Value]) -> json.Value`
- `[x] json.type_of(v) -> string`, `json.get(v, key) -> Optional[json.Value]`, `json.at(v, i) -> Optional[json.Value]`
- `[x] json.as_string/as_int/as_float/as_bool(v)` -- native value; type mismatch aborts located

## Deferred / needs a language feature first

- [x] User-defined generic functions (`fn f[T, U](...)`) with local, one-pass
  type inference at call sites -- IMPLEMENTED (generics slice 1, "M-gen-1"). Type
  parameters are usable wherever a type annotation appears and a bare type
  parameter is barred from every operation that needs a concrete type. Codegen is
  unchanged: a generic function emits one type-erased shell function. The generics
  program continues in further slices, each its own milestone:
  - [x] M-gen-2: the `comparable` bound (`fn f[T: comparable](...)`) -- IMPLEMENTED.
    A `comparable` `T` unlocks `==`/`!=`, with the inferred type restricted to
    `int`/`bool`/`string`/`float` (and value enums) and checked at the call site. Codegen
    unchanged. Ordered comparison and arithmetic on a `T` remain errors under any
    bound (they need monomorphization).
  - [x] M-gen-3: the `numeric` bound (`fn f[T: numeric](...)`) -- IMPLEMENTED.
    A `numeric` `T` unlocks `+`,`-`,`*`,`/`,`<`,`<=`,`>`,`>=`,`==`,`!=`, and
    unary `-`; `%` (modulo) is rejected because it is undefined for float. Only
    `int` or `float` satisfies the bound at a call site. Codegen monomorphizes:
    a numeric-bounded function emits one shell function per concrete type
    (`__int`, `__float`), so the arithmetic dispatches correctly.
  - [x] M-gen-4: generic structs (`struct Box[T]`) -- IMPLEMENTED (R7-generics-mgen4).
  Explicit type arguments at call sites (`f[int](...)`) remain unsupported; the
  postfix `[` is array indexing, so that syntax is decided in a later slice.
- [x] Tuples (`(T1, T2, ...)`, n >= 2): fixed-arity, immutable, opaque -- IMPLEMENTED. `zip`, `process.run_full`, and tuple destructuring (`let (a, b) = ...`) are also implemented; only a destructuring `match` remains deferred.
- [x] An optional/maybe type (`Optional[T]` = `Some(T) | None`) -- IMPLEMENTED.
  `find`/`index_of`/`last_index_of` now return `Optional[int]` and the dict
  accessor `get(d, k) -> Optional[V]` was added, alongside the access builtins
  `is_some`/`is_none`/`unwrap`/`unwrap_or`. `Optional` is opaque (no
  `==`/`to_string()`/switch); `None` concretizes at `let`/`return`/assignment only.
  `first`/`last`/`char_at` keep their catchable aborts.
- [x] `match` binding + `Result[T]` -- IMPLEMENTED. `match (o) { case Some(x) { }
  case None { } }` (and `Ok(x)`/`Err(e)`) binds the matched-variant payload into
  the matching arm's block only. `Result[T]` is a tagged
  `Ok(T) | Err(error)` value (single type parameter; the error payload is always
  the built-in `error` handle, conceptually `Result[T, error]`), with access
  builtins `is_ok`/`is_err`/`unwrap`/`unwrap_err`/`unwrap_or` -- `unwrap`/`unwrap_or`
  are overloaded across `Optional` and `Result`. `unwrap` of an `Err` performs a
  catchable located abort carrying the error's `.message`; `Ok(x)` infers fully
  like `Some(x)`, while `Err(e)` (like `None`) concretizes its `T` at
  `let`/`return`/assignment only. `Result` is opaque (no `==`/`to_string()`/switch).
  A destructuring `match` remains deferred.
- Rune-aware (UTF-8) string operations as a parallel `char_*` family.
- [x] Regular expressions (POSIX ERE, whole-match only) -- IMPLEMENTED.
  `matches`/`regex_find`/`regex_find_all`/`regex_replace`, byte-based under
  `LC_ALL=C`, backed by `awk` with the pattern/subject/replacement passed as
  inert data via `ENVIRON`. Capture groups, flags, compiled patterns, and
  `regex_split` are deferred (busybox/mawk lack `match(s, re, arr)`); `regex_split`
  is the clean fast-follow.
- [DONE] Broader shell targets (bash, zsh). The `#!/bin/sh` output now runs under
  busybox ash, dash, bash, and zsh, all tested in CI. bash was free (POSIX
  superset that word-splits unquoted `$var`). zsh needed a guarded header shim
  `if [ -n "${ZSH_VERSION:-}" ]; then emulate sh 2>/dev/null || setopt shwordsplit; fi`
  (in the current shell, not a subshell), because native zsh disables the
  unquoted-`$var` word-splitting that the dict/array `for x in $list` loops depend
  on; the shim is a no-op under the other three. A self-contained tooling
  milestone: it touched the codegen header and CI, not the language.
- [x] Time (`now`/`sleep`) and randomness (`random`) -- IMPLEMENTED. Wall-clock epoch seconds via `date +%s`, whole-second sleep, and non-crypto `[0, max)` random int via awk `srand`/`rand` with per-call-varied seed. See the Time and randomness section above.

## Suggested grouping into milestones

These are a starting point, to be turned into specs one at a time.

- Collections core: `sort`, `sort_by`, `find`, `any`, `all`, `slice`, `concat`,
  `first`/`last`, `sum`, `range`, plus dict `values`/`get`/`remove`/`merge`.
- String round-out: `substring`, `char_at`, `last_index_of`, `count`,
  `trim_start`/`trim_end`/`trim_prefix`/`trim_suffix`, `pad_start`/`pad_end`,
  `lines`, `is_empty`.
- Numeric/math: `clamp`, `sign`, `floor`/`ceil`/`round`, `sqrt`, `gcd` (`pow`
  deferred to a portable-math milestone; see Numbers above).
- [x] Filesystem metadata: `is_file`, `is_symlink`, `file_size`, `read_link`, `chmod`, `symlink`, `glob`, `temp_file`, `temp_dir` -- IMPLEMENTED. See the I/O and system section above.
- [x] Optional type (`Optional[T]`), unlocking `find`/`index_of`/`last_index_of`
  and dict `get` without sentinels -- shipped.
- [x] `match` binding form + `Result[T]` (`Ok`/`Err`,
  `is_ok`/`is_err`/`unwrap_err`, overloaded `unwrap`/`unwrap_or`) -- shipped.
- [x] Optional/Result chaining combinators (`map`/`and_then`/`filter`/`or_else`/`map_err`) -- IMPLEMENTED.
  Next: a destructuring `match` and a richer `run`.

## Testing

These builtins are only meaningful inside `*_test.wisp` files (test bodies, `setup`, `teardown`).
Documented in [the stdlib guide](/guide/stdlib/#testing) and the [testing guide](/guide/testing/).

- `[x] assert(cond: bool, msg: string = "") -> void`
- `[x] assert_eq[T: comparable](got: T, want: T) -> void`
- `[x] assert_ne[T: comparable](got: T, want: T) -> void`
- `[x] assert_some(o: Optional[T]) -> void`
- `[x] assert_none(o: Optional[T]) -> void`
- `[x] assert_ok(r: Result[T]) -> void`
- `[x] assert_err(r: Result[T]) -> void`
- `[x] assert_contains(s: string, sub: string) -> void` / `assert_contains(xs: T[], x: T) -> void`
- `[x] skip(reason: string) -> void`
- `[x] test_tmpdir() -> string`
