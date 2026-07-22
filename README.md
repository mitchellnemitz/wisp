# wisp

wisp is a small, explicit, C-like language that compiles to POSIX shell, targeting ash +
busybox. The single `#!/bin/sh` output also runs unmodified under dash, bash, and zsh
(zsh via a one-line word-splitting shim in the header); all four are tested in CI.
Generated scripts run anywhere a minimal shell is present (Alpine,
distroless-with-busybox, container entrypoints, CI glue) with no runtime beyond POSIX sh
and the applets busybox guarantees. The compiler is written in Go and ships as a static
binary that also runs inside those minimal containers.

The goal is a more obvious, consistent, and safe alternative to hand-written shell: the
compiler inserts every quote correctly and enforces an explicit type system, so the
quoting, word-splitting, and implicit-coercion bugs that plague shell scripts become
structurally impossible.

## Status

The full v1 language and its standard library are implemented: the type system,
control flow (including `match`), functions, structs, arrays, dicts, tuples,
`Optional`/`Result`, generics with `comparable`/`numeric` bounds, error handling,
constants (`const`/`final`, including `export const`), multi-file modules
(`include`/`import`), and a broad builtin library spanning strings, collections,
numeric/math, regular expressions, filesystem, and process and I/O operations.
The compiler ships `build`, `run`, `check`, `test`, `fmt`, and `doc`, plus a package
tool (`add`, `install`, `remove`) and a language server.

Testing and coverage are first-class: `wisp test` discovers and runs `*_test.wisp` files
cross-shell (dash, busybox-sh, bash, zsh), and `--coverage` reports line coverage against
the original source -- including uncalled functions -- so coverage is honest about untested
code, not just what ran.

## Install

Prebuilt `wisp` and `wisp-lsp` binaries for Linux, macOS, and Windows are
attached to each [release](https://github.com/mitchellnemitz/wisp/releases). On
Linux or macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/mitchellnemitz/wisp/main/install.sh | sh
```

Or build from source with Go 1.26 or newer:

```sh
go build -o wisp ./cmd/wisp
go build -o wisp-lsp ./cmd/wisp-lsp
```

## Quickstart

```sh
wisp run examples/hello.wisp        # hello, world
wisp build examples/hello.wisp      # writes examples/hello.sh
```

### Write and run your first test

Create `add_test.wisp`:

```wisp
test ("add works") {
    assert_eq(1 + 1, 2)
}
```

Run it cross-shell with coverage:

```sh
wisp test add_test.wisp             # runs under every available shell
wisp test --coverage add_test.wisp  # same, plus line coverage report
```

See [the testing guide](https://mitchellnemitz.github.io/wisp/guide/testing/) for the full guide.

## Documentation

- [Documentation](https://mitchellnemitz.github.io/wisp/)
- [Getting started](https://mitchellnemitz.github.io/wisp/guide/getting-started/)
- [Testing and coverage](https://mitchellnemitz.github.io/wisp/guide/testing/)
- [Language reference](https://mitchellnemitz.github.io/wisp/guide/language/)
- [Standard library](https://mitchellnemitz.github.io/wisp/guide/stdlib/)
- [CLI reference](https://mitchellnemitz.github.io/wisp/guide/cli/)
- [Editor tooling](https://mitchellnemitz.github.io/wisp/guide/tooling/)
- [Internals](https://mitchellnemitz.github.io/wisp/guide/internals/)
- [Versioning](https://mitchellnemitz.github.io/wisp/guide/versioning/)
- [Contributing](CONTRIBUTING.md), and [AGENTS.md](AGENTS.md) for AI agents

Complete programs are in [examples/](examples/). The rationale behind the
language's design is recorded in [design decisions](https://mitchellnemitz.github.io/wisp/design-decisions/).

## Language

- Types: `int`, `bool`, `string`, `float`, `error`, `struct`, arrays `T[]`, dicts
  `{K: V}`, tuples `(T1, T2, ...)`, function references `fn(T) -> R`, and the sum
  types `Optional[T]` and `Result[T]`.
- Control flow: `if`/`else if`/`else`, `while`, C-style and `for`-in loops, `switch`,
  and `match` for exhaustively destructuring `Optional`/`Result`.
- Functions with default arguments and first-class references; generics with optional
  `comparable`/`numeric` bounds; higher-order `array.map`/`array.filter`/`array.each`/`array.reduce` and the
  `array.map`/`and_then`/`array.filter`/`or_else`/`map_err` combinators over `Optional`/`Result`.
- Constants and immutable bindings: compile-time `const` (with `export const` for
  cross-module use) and runtime-immutable `final`. `_` is the blank identifier, a
  write-only sink at any binding site.
- Error handling: `error`, `throw`, `try`/`catch`/`finally` (recoverable, with persisting
  mutations).
- Multi-file programs: `export` symbols, `include "./path.wisp"` for local files, and
  `import "owner/repo"` for installed packages; everything links into one `.sh`.
- A broad stdlib: string, collection, numeric/math, regex, filesystem, process, and I/O
  builtins, plus conversions, `debug(x)`, and `print`.

## Tooling

- `wisp build <src> -o <out>.sh` (optionally `--source-map`), `wisp run <src>`,
  `wisp check <src>`, `wisp fmt <src>` (`-w` / `--check`), `wisp test [path]`
  (`--tap` / `--filter` / `--shell` / `--coverage`), `wisp doc <path>...` (extracts
  `///` doc-comments to Markdown), plus the package commands
  `wisp add <owner/repo>`, `wisp install`, and `wisp remove <owner/repo>`.
- Located runtime aborts (`wisp: file:line:col: message`) and source-context compile
  diagnostics with "did you mean" suggestions.
- `wisp-lsp`, a Language Server (diagnostics, formatting, document symbols, hover,
  completion) that reuses the compiler's own lexer/parser/checker/formatter.
- Editor support under [editors/](editors/): a VSCode extension bundling syntax
  highlighting and the `wisp-lsp` client in one package, plus Vim syntax files. A
  drift-guard test keeps the editors' keyword/type/builtin sets equal to the compiler's.

## Design principles

- Explicit over implicit: no inference where a declaration is possible, no implicit
  coercion or truthiness, no silent fall-through.
- Portable output: runs on busybox ash, dash, bash, and zsh; depends only on POSIX sh plus
  the applets busybox guarantees (`awk` for floats).
- Single self-contained artifact: one `.sh` file, no install step.
- Injection-safe by construction: the compiler quotes every expansion, so string literals,
  interpolation, switch cases, dict keys, and error messages are inert data, never executed.
- Developer experience: clear diagnostics, readable output, and source maps.

## Build history (delivered)

The language was built milestone by milestone; all of the below are implemented and
merged:

- M1 skeleton (lexer/parser/checker/codegen/CLI)
- M2 source maps + located aborts + source-context diagnostics
- M3 aggregates + float (struct, array, dict, `main(args)`)
- M4 first-class function references + `map`/`filter`/`each`
- M5 error handling (`try`/`catch`/`finally` + `throw` + `error`)
- M6 core stdlib + `wisp fmt` formatter + "did you mean" diagnostics
- M7 I/O builtins (`env`, `has_env`, `read_file`, `write_file`, `append_file`, `run`, `exit`)
- M8 multi-file modules (`export`, `include`, `import`, tree-shaken into one `.sh`)
- M9 package tool (`wisp add`/`install`/`remove`, `wisp.json` + `wisp.lock`)
- `Optional[T]`/`Result[T]` with `match`, plus the access and combinator builtins
- Generics with `comparable`/`numeric` bounds, and generic structs
- Tuples, `const`/`final` (and `export const`), the `_` blank identifier, `debug(x)`
- Stdlib expansion: collections, numeric/math, string round-out, regex, filesystem,
  process, and I/O-tail builtins

Each ships with golden fixtures run under busybox `ash`, `dash`, `bash`, and `zsh`, a
pinned ShellCheck gate, and a static-binary-in-busybox check in CI. The supported shell
set is exactly what the CI matrix pins (those four, at the versions CI installs); macOS
system bash/zsh are covered by the same single-`#!/bin/sh`-script contract, with no
minimum sub-version promised beyond a POSIX-mode-capable build.

## License

MIT. See [LICENSE](LICENSE).
