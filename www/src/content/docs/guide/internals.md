---
title: Internals
---


This describes how the compiler is built and how the generated shell works. It
is for people changing the compiler and for tools reasoning about its output.
For how to use the language, read the [language reference](/guide/language/).

## Compile pipeline

A compile runs five stages, wired together by `internal/driver`:

```
lexer  ->  parser  ->  module linker  ->  checker (types)  ->  codegen
```

`driver.Compile(filename, src)` loads and links the module graph via
`module.Load`, runs `types.CheckLinked`, and then `codegen.GenerateLinked`. It
returns the script bytes, a per-line source map, and diagnostics. A parse or
link error becomes a diagnostic. Any checker error stops the build before
codegen, so codegen only ever runs on a program that type-checks.
(`codegen.GenerateWithMap` is a lower-level single-program entry point that the
driver does not call directly.)

## Packages

- `internal/token`: token kinds, the keyword table, and `Position`, which
  renders as `file:line:col`.
- `internal/lexer`: turns source into tokens. It keeps `//` comments on a side
  channel for the formatter, so they never reach the parser or codegen. Errors
  are `*lexer.Error` and carry a position.
- `internal/parser`: builds the AST. `Parse` and `ParseWithComments` are the
  entry points. Errors are `*parser.Error` and carry a position.
- `internal/ast`: the AST node types.
- `internal/module`: the module loader/linker. Given the root file it discovers
  the project root, parses every imported/included module, assigns each a module
  id, dedups by canonical path, detects cycles, and reads `wisp.json`. It does no
  type-checking; it imports only the front-end packages.
- `internal/types`: the checker. `CheckLinked(linked)` checks the whole linked
  program into one `Info` (and `Check(prog)` is the single-module wrapper over
  it). `Info` holds every expression's type, each identifier's binding, call
  resolutions, function data, the resolved root `main`, and the errors and
  warnings. The builtin catalog and reserved names live here, along with the
  did-you-mean suggestions.
- `internal/runtime`: the POSIX-sh prelude. Each helper is a snippet with a
  dependency list, and the package tree-shakes them.
- `internal/codegen`: lowers the checked program to one `.sh` plus a line map.
- `internal/doc`: the `///` doc-comment extractor and Markdown renderer behind
  `wisp doc`.
- `internal/format`: the `wisp fmt` formatter.
- `internal/driver`: the shared build, run, and check entry points, plus
  diagnostic rendering, source maps, and exit codes.
- `internal/lsp`: the language server.
- `internal/editors`: holds the drift-guard test that keeps the editor grammars
  in sync with the compiler's name tables.

## Modules (linking)

Modules are linked at compile time into one program; there is no runtime module
system. Each module gets a small integer module id (modid), the root being 0,
assigned once per canonical file path so a file reached several ways is compiled
once. The modid makes shell names collision-free: a function mangles to
`__wisp_f_m<modid>_<name>`, so two modules may define a function of the same name
without clashing. The discriminator is always the compiler-assigned integer,
never any `wisp.json` name, alias, or `owner/repo` text, so no package-controlled
data reaches a shell identifier (the injection invariant holds).

A struct's cross-module identity is the internal token `Name@modid`. Because `@`
cannot appear in a source identifier, two modules' same-named structs are
distinct types that never collide, and the token never reaches the shell (struct
handles are keyed by a runtime id, not a name). Diagnostics render the readable
`Name`. Only functions reachable from the root `main` are emitted, so an unused
import contributes nothing to the output.

## Core modules

A **core module** is a reserved namespace an author opts into with `import
"json"` (no filesystem file). The mechanism is generic and table-driven, so
adding another core module (regex, env, fs, ...) is data plus its own codegen and
prelude, not new control flow.

The moving parts:

- **Registry** (`internal/core`): a leaf package listing the reserved namespace
  names. A leaf so `internal/module` can depend on it without a cycle.
- **Loader hook** (`internal/module`): `resolveImport` checks the registry
  first; a reserved name interns a synthetic module (path `"\x00core/" + name`,
  a NUL a real path can't contain) and binds the alias. No json-specific branch.
- **Checker bridge** (`internal/types/core_members.go`): `coreCatalog[namespace]
  [member]` describes each member as a function, a type, or a constant. A
  function member records a `CallBuiltin` `CallInfo` with a `Builtin` string, so
  the checker delegates to the ordinary builtin machinery. Four resolution sites
  consult the catalog: qualified calls, qualified type references, qualified
  value/const references, and the type-argument guard.
- **Codegen**: dispatches purely on `CallInfo.Builtin`, so the namespaced
  spelling needs **zero** awareness -- a `json.encode` call lowers exactly like a
  flat builtin named `json_encode`. Handle-returning members allocate a reference
  handle like any aggregate.
- **Prelude**: the runtime helpers, tree-shaken like every other helper, so a
  program that imports a core module but uses only part of it pays only for what
  it calls.

To add a core module: append its name to the registry, add a `coreCatalog`
block, add the `genBuiltinCall` cases, and register the prelude helpers. The
loader, the four bridge sites, and the dispatch switch are untouched.

## Codegen conventions

The generated script uses a fixed naming scheme so user names can never collide
with compiler-generated ones. Everything lives in the reserved `__` namespace.

- Functions are mangled to `__wisp_f_m<modid>_<name>`, carrying the module id so
  same-named functions in different modules never collide. The root module is 0,
  so `main` becomes `__wisp_f_m0_main`.
- Variables are mangled to `__wisp_v_<n>` with a per-function counter, so two
  sibling-scope variables with the same source name get distinct shell names.
  Codegen reads the mangled name the checker assigned rather than deriving it.
- Every value-producing expression writes a single global return register,
  `__ret`. The result is spilled to a fresh temporary before the next thing
  that writes `__ret`, which is how strict left-to-right evaluation is kept
  without clobbering. Temporaries and condition spills use their own prefixes,
  distinct from the function and variable prefixes.

## Reference handles

Structs, arrays, dicts, and errors are reference handles. A monotonic counter
issues a fresh integer id, and the instance's data lives in namespaced backing
variables keyed by that id:

- A struct field is `__wisp_s_<id>_<field>`.
- An array element is `__wisp_a_<id>_<i>`, with the length in
  `__wisp_a_<id>_len`.
- A dict entry is `__wisp_d_<id>_<token>`, with the insertion-ordered keys in
  `__wisp_d_<id>_keys`.
- An error is a handle whose message is the struct field
  `__wisp_s_<id>_message`.

Dict keys are encoded to a reversible, variable-name-safe token before being
used in a backing-variable name, so a key with arbitrary bytes cannot collide
with the length or keys slots.

## Value representations

- bool is the literal string `true` or `false`. A comparison emits
  `[ "$x" = true ]`.
- Floats are handled with awk. Each float operation runs a constant awk program
  with the operands passed as `-v` variables and prints with `%.17g`, and the
  result is checked for finiteness. There is no exponent, infinity, or NaN.

## Injection safety

Every expansion in the output is double-quoted. `printf` formats are constants
with data supplied only through `%s`; the generated script never uses `echo` for
data. String literals are re-encoded as POSIX single-quoted tokens. Runtime
search and separator strings are matched literally with quoted parameter
expansions, so glob metacharacters in them are inert. No runtime string is ever
re-evaluated or used as a pattern. This is what makes string values inert data.

## Tree-shaken prelude

The runtime prelude is a library of helper snippets. Codegen records which
helpers a program uses, the runtime package resolves the transitive
dependencies, and only those helpers are emitted, in a deterministic order. An
unused helper never appears in the output.

## Source maps

`GenerateWithMap` records, as it emits, the wisp position for each generated
line. The table has one entry per output line, so it cannot drift from the
script. Lines with no source, such as the shebang, the prelude, and blank
separators, map to nothing. `wisp build --source-map` writes this table as
JSON next to the `.sh`.

## Generated-shell error model

By default the script fails fast. A located abort calls a fail helper that
prints `wisp: file:line:col: message` to stderr and exits 1. This covers
out-of-bounds access, division by zero, bad conversions, an empty split
separator, a negative repeat count, and a missing dict key.

When a program contains a `try` or a `throw`, codegen emits an error-handling
mode instead, with no overhead when neither is present. In this mode the fail
helper is aware of a try-depth counter. At depth 0 it prints and exits as
before. Inside a `try` it converts the fault into a catchable error: it stores
the message, sets a pending-error flag, and returns rather than exiting. The
first fault wins; a later one does not overwrite it. A `throw` works the same
way but stores the thrown error's message, so the caught `e.message` is exactly
the message of the thrown error.

Each guarded statement opens a short-circuit guard that skips the rest of the
body once an error is pending, and the guards close in last-in-first-out order
at statement and condition boundaries. This is how the body stops at the first
fault while still running in the current shell, which is what lets mutations
made before a fault persist. In the non-error mode these guard lines are
stripped, so a program without `try` or `throw` produces the same output it
would have before error handling existed.

## CI

The GitHub Actions workflow runs three jobs on every push and pull request:

- build-test: a gofmt check, `go vet`, `go build`, and `go test` over the whole
  module.
- golden: installs dash, busybox, zsh (bash is preinstalled), and a pinned
  ShellCheck, then runs the test suite. The golden harness compiles and runs
  fixtures under dash, busybox ash, bash, and zsh and ShellChecks the generated
  output. zsh runs the same `#!/bin/sh` script as the others: a one-line header
  shim restores the unquoted-`$var` word splitting that the array/dict
  `for x in $list` loops rely on, since native zsh disables it, and raises the
  zsh function-nesting ceiling so deep recursion matches the other shells (the
  shim is a no-op under the other three shells):

  ```sh
  if [ -n "${ZSH_VERSION:-}" ]; then emulate sh 2>/dev/null || setopt shwordsplit; FUNCNEST=1000000; fi
  ```

  zsh caps function-call nesting at the `FUNCNEST` parameter (default 700), so
  without the shim deep wisp recursion aborts under zsh ("maximum nested function
  level reached") while dash and bash bound it only by the OS stack. zsh has no
  unlimited setting: unsetting `FUNCNEST` reverts to the 700 default, and
  `FUNCNEST=0` means zero nesting (the opposite of bash, where 0 is unlimited), so
  the shim sets a large fixed ceiling (1000000) under zsh only. Past that, every
  shell is bounded by the C stack and all four crash in the same multi-thousand-
  frame range; 1000000 is well clear of any realistic recursion depth.
- static-binary: builds static `wisp` and `wisp-lsp` binaries and runs the
  hello example inside a busybox container, asserting the output.
