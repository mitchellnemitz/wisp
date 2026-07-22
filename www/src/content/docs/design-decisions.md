---
title: Design decisions
---


wisp compiles a small, explicit, C-like language to POSIX shell. This document
explains why it is shaped the way it is: the invariants it will not trade away,
the alternatives it considered and rejected, and the features it deliberately
leaves out. It condenses roughly 140 per-feature design specs into one durable
record, organized by theme rather than by milestone. For syntax and semantics,
see the [language reference](/guide/language/); for how the compiler is built,
see [internals](/guide/internals/).

## Why compile to shell at all

Hand-written shell scripts accumulate three recurring classes of bug: missing
quotes that let word-splitting and globbing corrupt data, implicit truthiness
and coercion that make `[ "$x" ]` mean something different depending on what
`$x` happens to hold, and silent injection when untrusted data reaches a
context the shell re-parses. These bugs are not exotic; they are the normal
failure mode of scripts that grow past a few dozen lines, and they show up
constantly in container entrypoints, CI glue, and small ops tooling that has
to run in a minimal environment with no runtime to install.

wisp's premise is that a compiler can structurally rule these out: insert
every quote correctly, require every conversion to be explicit, and never let
a data value be re-interpreted as code. The output is still a POSIX shell
script with no runtime dependency, so it drops into exactly the places
hand-written shell already runs. The rest of this document is about the
consequences of taking that premise seriously.

## Injection safety as a hard invariant

This is the one invariant every other decision defers to. The rule: every
expansion in generated output is double-quoted, data reaches `printf` only
through `%s` (the generated script never uses `echo` for data), string
literals are re-encoded as POSIX single-quoted tokens, and no runtime string
is ever re-evaluated or used as a glob pattern or command. A wisp string value
that contains `$(...)`, backticks, or semicolons is inserted as inert data and
is never executed by the generated shell, no matter what the value came from
(user input, a file, an argument).

This shows up in several concrete places:

- Float arithmetic shells out to `awk`, but the awk *program* is always a
  compile-time constant string; operand values flow in only through `-v`
  assignments, never interpolated into the program text. A value that looks
  like awk syntax cannot become awk syntax.
- Search and separator strings (`string.split`, `string.contains`, and
  friends) are matched literally through quoted parameter expansions, so glob
  metacharacters in them are inert rather than pattern syntax.
- Dict keys are encoded to a reversible, variable-name-safe token before they
  become part of a backing-variable name, so a key containing arbitrary bytes
  cannot collide with a reserved slot like the length or keys list.
- Trap actions and other constructs that need a shell-level literal use only
  the compiler-assigned mangled function name, never a user-controlled string.

Because this invariant is easy to violate one call site at a time as new
builtins are added, it is backed by a machine gate rather than a convention:
a scan test fails the build if any builtin name never appears in a compiled
test fixture, which closes the gap where a builtin is wired into the checker
and codegen but never actually exercised end to end before merge. The
philosophy generalizes past injection safety: several internal switches over
closed sets (AST node kinds, operator tokens) were swept from "silently
return a default on an unhandled case" to "panic loudly on checker/codegen
drift," on the reasoning that a silent fallback in a transpiler is a
correctness bug wearing a UI, not a graceful degradation.

## Explicitness: no truthiness, no implicit coercion

wisp has no implicit coercion and no implicit truthiness. A condition must
already be a `bool`; there is no shell-like "any nonempty string is true."
`int` and `float` never mix in one expression; convert explicitly with
`to_float()`/`to_int()`. The one exception is string interpolation
(`"${expr}"`), which is the single place a value becomes a string implicitly
- everywhere else you convert explicitly. This exception is deliberate and
scoped: interpolation is the language's most common operation (nearly every
script prints something), and gating it behind an explicit `to_string()` call
at every use would fight the language's own ergonomics for no safety benefit,
since interpolation already goes through the same double-quoted, `%s`-only
path as everything else.

The `bool` conversion is strict on purpose: from a string, only the exact
literals `"true"` and `"false"` convert; `"True"`, `" true "`, `"1"`, and `""`
all abort. There is no case-folding, whitespace-trimming, or catch-all
default. The rationale is that a permissive `to_bool` silently accepts
malformed input as `false` (or as an arbitrary default), which turns a data
bug into a wrong-answer bug that never surfaces. If you need looser input,
you normalize it explicitly first (`to_bool(string.lower(string.trim(raw)))`),
which puts the leniency in your code, visibly, rather than in the compiler's.

The same reasoning extends to constant-argument checking: when a builtin or
`/`/`%` receives a compile-time-constant argument that can never satisfy the
operation's precondition (`string.chr(0)`, `5 / 0`), the compiler reports it
as a compile error at the argument rather than deferring to a runtime abort.
A non-constant argument (a variable, a runtime-computed value) still goes
through the runtime guard. Catching the error earlier, when it is knowable
earlier, is a strict improvement with no cost to programs that pass the
check.

## snake_case builtins and the drift-guard

The standard library has exactly one casing convention: every builtin is
`snake_case` (`read_file`, not `readFile` or `ReadFile`), and this never
changes. A new builtin is `snake_case` by construction, not by review
discipline.

The reason this needs enforcement rather than just a style guide: wisp ships
editor grammars (VS Code, Vim) that highlight keywords, types, builtins, and
constants by name, and a language server that offers completions from the
same name tables. If the compiler's builtin catalog and the editor grammars
drift apart, silently, the failure mode is invisible until a user hits a
builtin that highlights as a plain identifier or doesn't autocomplete. So
`internal/editors` holds a drift-guard test that reconciles the editor
grammars against `internal/types/accessors.go`'s `ReservedNames()` -
the sorted, deduped union of keywords, type names, reserved constants and sum
constructors, and builtins. Adding a builtin is documented as touching five
surfaces (checker, codegen, runtime prelude, editor grammars, golden
fixtures) precisely because skipping the editor-grammar update is silent
without the guard.

`ReservedNames()` deliberately excludes two bound words, `comparable` and
`numeric`: they are keywords only in type-parameter bound position
(`fn f[T: comparable](...)`), so a user program may still declare a function
or variable named `comparable` or `numeric`. Making them fully reserved would
have cost real vocabulary for a restriction that only matters in one syntactic
slot. Any identifier beginning with `__` is reserved as a non-enumerable
prefix rule (the compiler's own namespace), rather than as a name on any list.

## Opaque handles and the single return register

Structs, arrays, dicts, errors, `Optional`, `Result`, tuples, and function
references are all reference handles: assigning one or passing it to a
function shares the same instance, exactly like a pointer. Each is backed by
a monotonic integer id and namespaced shell variables keyed by that id (a
struct field is `__wisp_s_<id>_<field>`, an array element is
`__wisp_a_<id>_<i>` with its length in `__wisp_a_<id>_len`, and so on).
Handles are opaque: you cannot convert one with `to_string()`, compare it,
or do arithmetic on it. `debug(x)` renders any value structurally for
inspection without pretending a handle is a primitive.

Opacity is not an oversight to be relaxed later; it is a direct consequence
of what these values actually are. A struct is not one shell value, it is a
constellation of variables reachable through an id - there is no single
string that "is" the struct, so there is nothing coherent for `to_string()`
or `==` to compare. The same logic makes `Optional` and `Result` opaque
(their internal encoding is a tag plus a value slot, not a renderable
scalar), and makes a function reference opaque (it is the target's mangled
shell name, and comparing or printing that name would leak an internal
implementation detail as if it were program-visible state).

The single return register, `__ret`, is the codegen-level counterpart of the
same idea. Every value-producing expression writes its result to one global
shell variable rather than each call site inventing its own convention. The
result is spilled to a fresh temporary before the next thing that writes
`__ret`, which is what keeps strict left-to-right evaluation order without
one write clobbering an earlier one still in flight. Having exactly one
convention for "how does a value get out of an expression" is what lets every
later feature (aggregates, funcrefs, error handling) reuse the same lowering
without inventing a parallel mechanism per feature.

## Match-only destructuring for Optional and Result

`Optional[T]` and `Result[T]` model absence and fallibility as first-class,
type-checked values - no bare `null`, no `-1` sentinel, no exception-only error
channel. Access is only through builtins (`is_some`, `unwrap`, `unwrap_or`,
`is_ok`, `unwrap_err`, ...) or through `match`, never through an implicit
dereference.

wisp's `match` requires the scrutinee in parentheses - `match (expr) { case
Pattern { body } ... }` - and is exhaustive at compile time: every variant of
`Optional` or `Result` must be covered, either explicitly or by a trailing `_`
wildcard arm. A payload-carrying variant arm must bind or discard its payload
explicitly (`Some(name)` or `Some(_)`); a bare `Some` with no parentheses is a
compile error, so there is no way to accidentally ignore a value you meant to
use.

`match` is the destructurer for variants rather than a conditional-binding
form like `if let Some(x) = expr { ... } else { ... }`, for two concrete
reasons. First, syntax consistency: every conditional form in the language
wraps its test in parentheses (`if (cond)`, `while (cond)`, `switch
(subject)`); a construct where the scrutinee was unparenthesized and `=` was
not the assignment operator would force every reader to special-case one
construct's grammar. Second, ergonomics: unwrapping a `Result` needs both
arms simultaneously, and a single-arm binding form has no dual-arm shape - you
would either evaluate the scrutinee twice or manually spill it to a temporary
yourself. `match` avoids both: the scrutinee is parenthesized like everything
else, and exhaustiveness means both arms live in one statement, evaluating
the scrutinee exactly once. `match` is the sole destructurer for `Optional`
and `Result`; this is also why the paren-keyword principle (below) is stated
as an absolute rather than a preference.

## The float domain: finite decimals via awk

wisp deliberately does not adopt IEEE 754 semantics. `float` is a finite
decimal in a specific runtime domain: a value whose `%.17g` representation is
a plain decimal with no exponent character, which in practice means
magnitudes roughly in `[1e-4, 1e17)`. There is no infinity, no NaN, and no
exponent form, at either the literal level or the runtime level.

The mechanism: each float operation shells out to a constant `awk` program
with operands passed as `-v` variables, printing the result with `%.17g`, and
the formatted result is then checked for finiteness. Two things follow from
this. First, the dependency is deliberate and singular - `awk` is the one
external applet the compiler's output relies on, chosen because it is a
POSIX-mandated tool busybox guarantees, and because it is the only
practical way to do decimal arithmetic in a portable shell script without
inventing a bignum library in shell itself. Second, the domain rule is not
arbitrary conservatism: it exists because the runtime already refuses
`inf`/`nan`/exponent forms (a fault the shell has no other way to represent
safely as a plain value), and the literal-level check simply catches at
compile time what the runtime would otherwise catch late and less
helpfully - a float literal whose `%.17g` form would be out of range is a
compile error, parallel to the integer literal range rule, rather than a
silent runtime surprise. A float literal requires at least one digit on each
side of the decimal point (`3.14`, `2.0`); `3.`, `.5`, and `1e9` are not valid
literals, so there is no ambiguity about whether a token is an int or a
float at parse time.

Modulo on `float` is a compile error, not a runtime one: `%` is defined only
for `int`, because modulo is not a numerically sound operation for a decimal
approximation. This is decided at the type level rather than deferred to
`awk`, because there is no runtime behavior to fall back to that would be
correct.

## Integer overflow and division by zero

`int` is a signed 64-bit decimal integer, `-9223372036854775808` to
`9223372036854775807`. A literal outside this range is a compile error in
every context, mirroring the float-literal domain rule. Runtime arithmetic
that overflows this range is *unspecified* rather than defined: it wraps
two's-complement where the underlying shell supports it, and behavior at the
exact 64-bit boundary is shell-dependent. This is an accepted limitation, not
an oversight - wisp targets four shells (dash, busybox ash, bash, zsh) whose
native `$(( ))` arithmetic disagree at the extreme boundary (`INT_MIN`
specifically: zsh cannot represent `2^63` at all, and dash is off-by-one for
an `INT_MIN` value stored in a variable and then used in arithmetic). Rather
than emulate 64-bit wraparound in shell arithmetic by hand for the sake of one
boundary value - which would add real complexity and its own bug surface for
a case realistic programs rarely hit at the exact edge - wisp documents the
divergence and treats it as a shell-portability limit, the same way it treats
a shift amount at or beyond the integer width as platform-defined.

Where overflow is *not* left unspecified is where it is unambiguous and
dangerous to get silently wrong: negating or taking the absolute value of the
minimum `int`, and dividing it by `-1`, are both genuinely unrepresentable in
64-bit two's-complement (there is no positive counterpart to `INT_MIN`). Both
are a catchable, located abort rather than a silently wrong value. Ordinary
division and modulo by zero abort the same way. The dividing line is
consistent throughout the language: where a result is representable but its
exact value depends on the underlying shell, wisp documents the divergence;
where a result is not representable at all, wisp aborts rather than emit a
number that only looks correct.

## Fail-fast by default, try/catch without unwinding

The default error model is fail-fast: any runtime fault (an out-of-bounds
index, a bad conversion, division by zero, a missing dict key) calls a fail
helper that prints `wisp: file:line:col: message` to stderr and exits 1. A
program with no `try` and no `throw` produces exactly this and nothing more -
no overhead, no extra codegen.

`try`/`catch`/`finally` layers recoverable error handling on top without
introducing stack unwinding, because shell has no such mechanism to layer on.
The implementation runs the guarded body in the *current* shell - not a
subshell - and tracks a try-depth counter and a pending-error flag. Inside a
`try`, a fault stores its message, sets the flag, and returns rather than
exiting; the first fault wins and a later one does not overwrite it. Each
guarded statement wraps a short-circuit check that skips the rest of the body
once an error is pending, and these guards nest in last-in-first-out order at
statement and block boundaries. `throw` works identically, storing the
thrown error's message so the caught `e.message` is exactly what was thrown.

A subshell-based design was considered and rejected for one concrete reason:
it would lose mutations made inside the `try` body. The canonical idiom
`try { n = to_int(x) } catch (e) { n = -1 }` requires that a partial parse or
any variable written before the fault remain visible after the `try` - a
subshell would isolate those writes in a child process and they would vanish
silently when it exited. Running in the current shell with a flag-based
short-circuit is the only design that preserves this, at the cost of somewhat
more intricate codegen and no new external dependency (no `mktemp`, no
subshell, no `trap`).

Because there is no unwinding, `return`, `break`, and `continue` that would
escape a `try`/`catch`/`finally` body are compile errors, not runtime
behavior to get right. Modeling "the user wants to return through a pending
error" correctly would need a full control-flow-disposition state machine
distinguishing "error in flight" from "user wants to leave normally," judged
not worth the complexity for what it would buy; instead you structure code so
the `try` is the last thing before the exit you want, then return after it.
`break`/`continue` remain legal when they target a loop entirely inside the
`try` body.

Error wrapping (`wrap`/`cause`) lets a handler attach context to a fault while
preserving the original cause, Go-`%w`-style, without adding a new field to
`error` or a new named type - `cause(err)` returns `Optional[error]` rather
than exposing `e.cause` directly. One non-obvious constraint drove the design:
the persistent error handle is *reconstructed* on `catch` (the throw path
carries only the message and code across the flag-based mechanism), so a
cause has to be threaded through that same reconstruction path deliberately,
or wrapping context would silently vanish across exactly the
catch-wrap-rethrow pattern it exists to support.

## Compile-time module linking, tree-shaking, and always-qualified access

Modules are a compile-time concept only; there is no module system in the
generated shell. The compiler links every reachable module into one script,
assigns each a small integer module id (the root is 0), and only functions
reachable from `main` are emitted - an unused import contributes nothing to
the output.

Access is always qualified (`geo.Point`, `geo.manhattan(p)`) with no
unqualified bring-into-scope form, and visibility is not transitive
(importing a module does not expose its own imports). This is a legibility
decision as much as a technical one: in a language whose whole value
proposition is that the compiler, not the reader, tracks what a name refers
to, an unqualified import would reintroduce exactly the "where did this name
come from" ambiguity the type system elsewhere eliminates.

The module id is also the mechanism that keeps the injection-safety invariant
intact under multi-file programs. A function mangles to
`__wisp_f_m<modid>_<name>`, so two modules may define a function of the same
name without colliding - and the discriminator is always the
compiler-assigned integer, never any package-controlled string (a
`wisp.json` name, an import alias, or an `owner/repo` string). No
package-controlled data reaches a shell identifier. A struct's cross-module
identity is likewise the internal token `Name@modid` (`@` cannot appear in a
source identifier), and that token never reaches the shell either - struct
handles are keyed by a runtime id, not a name, and diagnostics render the
readable `Name` while the compiler tracks the qualified token internally.

Core modules (`json`, `regex`, `env`, `fs`, `process`, `math`, `string`,
`array`, `dict`, ...) are a reserved namespace a program opts into with
`import "json"` and no filesystem file behind it. The mechanism is
table-driven rather than hardcoded per module: a registry lists reserved
names, a checker-side catalog describes each member as a function, type, or
constant (delegating function members to the ordinary builtin-checking
machinery), and codegen dispatches purely on the underlying builtin
identifier - so `json.encode` lowers exactly like a flat builtin named
`json_encode`, and codegen needs zero awareness that namespacing exists.
Adding a new core module is data plus its own prelude helpers, not new control
flow, which is what let the language later collapse *every* builtin into a
module home (removing the historical flat/qualified dual-spelling that early
core modules briefly kept for compatibility) without a rewrite of the
dispatch mechanism.

The package tool that fetches external packages is deliberately isolated
from this: the compiler itself never touches the network. Fetching shells out
to the user's own `git` via `os/exec` - already a compiler dependency for
`wisp run` - so no new Go import reaches the static binary, and the compiler
stays git-free and offline while the separate tool step is the one that goes
online. Installed packages land in a compiler-owned `.wisp/` directory that
`include` may not resolve into, keeping the local-file and installed-package
namespaces from colliding.

## The paren-keyword principle

Every construct keyword in wisp takes a parenthesized head: `if (...)`,
`while (...)`, `for (...)`, `match (...)`, `test ("name")`. There is never a
non-parenthesized function-like keyword in wisp, under any circumstances.
This is stated as an absolute, not a style preference: a construct with an
unparenthesized scrutinee and `=` doing something other than assignment would
break the pattern, and that inconsistency is one of the two concrete reasons
variant destructuring is done with `match`. The principle exists to make the
grammar predictable from a small number of rules rather than requiring a
reader to remember which constructs are exceptions.

## Cross-shell determinism

Generated output targets four shells with one script: dash, busybox ash,
bash, and zsh. The `#!/bin/sh` shebang and script body never change between
targets - there is no per-shell conditional codegen beyond one guarded
exception described below. Achieving this required finding and closing real
divergences rather than assuming POSIX compliance was sufficient:

- **zsh word-splitting.** wisp lowers array and dict iteration to
  space-joined strings iterated with an unquoted `for x in $list` expansion,
  relying on the fact that dash, busybox ash, and bash all word-split an
  unquoted `$var` on `$IFS`. Native zsh does not do this by default, which
  would silently iterate once over the whole joined string instead of per
  element. The fix is a one-line guarded shim in the generated header,
  `emulate sh 2>/dev/null || setopt shwordsplit`, that runs only when
  `$ZSH_VERSION` is set and is a no-op everywhere else - restoring the
  behavior the other three shells already have natively rather than
  reimplementing iteration to avoid depending on it.
- **zsh function-nesting depth.** zsh caps recursive function calls at
  `FUNCNEST` (default 700), while dash and bash are bounded only by the OS
  stack. Deep wisp recursion would abort under zsh alone with no equivalent
  failure on the other three shells. The same header shim raises `FUNCNEST`
  to a large fixed ceiling under zsh only, restoring parity rather than
  asking programs to avoid recursion depths the other shells tolerate fine.
- **The `INT_MIN` boundary**, discussed above: an unrepresentable case at the
  extreme of the integer range, documented rather than silently patched over
  with per-shell branching that would reintroduce the "one script, all
  shells" violation the language otherwise avoids.
- **CI enforces this directly**, not by inspection: the golden test harness
  compiles and runs every fixture under all four shells and ShellChecks the
  generated output on every push, so a shell-specific regression fails the
  build rather than surfacing later as a user bug report.

The guiding rule when a genuine shell divergence is found: fix it with one
guarded, shell-detecting shim that is a no-op elsewhere, rather than
generating different code per target. The one-script invariant is treated as
worth the extra shim complexity, not something to trade away for a simpler
compiler.

## POSIX-only output, awk as the sole dependency

The generated script's only runtime dependency is POSIX `sh` plus the applets
busybox guarantees. `awk` (for float arithmetic) is the sole applet dependency
beyond that floor, chosen deliberately rather than accumulated - the
dependency budget is treated as a fixed constraint each new feature has to
fit inside, not a number to renegotiate per feature. Output is ShellCheck-clean
by construction and is checked as such in CI, not just aimed for: readable
generated shell, with source-position comments per function and a source-map
option that ties every emitted line back to the wisp line that produced it,
is treated as a first-class deliverable rather than an afterthought, on the
reasoning that symbol mangling and block-scope renaming already make
generated output harder to read back to source than typical hand-written
shell, so the toolchain has to actively compensate rather than assume
debuggability comes for free.

## A stdlib-only, static Go compiler binary

The compiler itself is Go, stdlib only - no third-party Go modules - and
builds as a single static binary (`CGO_ENABLED=0`) small enough to run inside
a busybox container. This is a hard constraint applied to every new feature,
not just the initial architecture: when the external package tool needed to
fetch remote code, the design explicitly chose to shell out to the user's
own `git` via `os/exec` (already a compiler dependency for `wisp run`) rather
than vendor a Go git library, specifically so no new Go import would reach
the static binary. The result is a compiler with the same "runs anywhere a
minimal shell is present" property as its own output - the two halves of the
project share one operating constraint.

## Deliberately excluded: closures, dynamic dispatch, struct embedding

Some features that are common in general-purpose languages are a poor fit
for wisp's compilation target and are excluded on purpose rather than
deferred as future work:

- **Closures and captured variables.** Function references are C-style
  function pointers: a bare function name in value position decays to a
  reference (the function's mangled shell name), storable in variables,
  parameters, struct fields, and collections, and callable indirectly. There
  is deliberately no lambda syntax and no captured environment. A closure
  would need the compiler to synthesize and manage an environment record
  behind the scenes in a language whose entire value proposition is that
  every runtime value is a transparent, traceable shell variable or handle -
  a captured environment is exactly the kind of implicit, invisible state
  the rest of the design works to avoid. Where a value needs to be threaded
  into a callback, it is passed as an explicit extra argument to a named
  top-level function; there is no inline lambda syntax at all, so even a
  builtin that cannot be referenced directly (because it is overloaded or
  has no single funcref-shaped lowering) is wrapped in an ordinary named
  `fn`, not a closure.
- **Interfaces and dynamic dispatch.** wisp has no interface types and no
  runtime polymorphism over user types. Structs are concrete, opaque
  handles with a fixed field set. A bare `enum Name { ... }` with at least
  one payload variant is a general user-declared tagged-union sum type; a
  backed `enum Name: int|string|bool { ... }` is a separate, comparable
  value-enum mode. `Optional`/`Result` are the same idea, built in and made
  generic over their payload type: `Some`/`None` and `Ok`/`Err` are
  constructors dispatched by `match`, the same as any user tagged-union
  enum's variants. Dispatch over any of them, built-in or user-declared, is the
  compiler-verified exhaustive `match`, not virtual method calls. Runtime
  dispatch would require a vtable-like indirection mechanism lowered to
  shell - itself a nontrivial and injection-adjacent piece of machinery - to
  buy polymorphism that a small, explicit scripting language aimed at
  entrypoints and CI glue rarely needs; generics with `comparable`/`numeric`
  bounds cover the realistic cases (a function that needs to compare or do
  arithmetic on a type it doesn't know yet) without it.
- **Struct embedding or inheritance.** A struct is a flat, named record with
  no supertype and no embedded-field promotion. Adding embedding would mean
  deciding how field access, construction, and identity compose across a
  hierarchy of handles, each already a nontrivial reference-handle scheme
  in its own right - real complexity for a feature real wisp programs
  (small scripts, entrypoints, CLI tools) have little use for. Composition
  is done explicitly, with a named field of the composed type, which is
  consistent with the rest of the language's preference for visible
  structure over implicit mechanism.

These are not gaps the project intends to fill later; each one is excluded
because it would require the compiler to manage implicit runtime state
(a captured environment, a vtable, a promoted field set) in a language whose
central bet is that all state is explicit and traceable to a shell variable a
human can read.
