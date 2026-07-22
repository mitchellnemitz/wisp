# AGENTS.md

This is the canonical guide for AI coding agents working in this repository. The
repository's [CLAUDE.md](CLAUDE.md) and
[.github/copilot-instructions.md](.github/copilot-instructions.md) both point
here, so this file is the single source of truth. Add any repo-wide agent
guidance here, not in those pointer files. Humans should read
the [documentation site](https://mitchellnemitz.github.io/wisp/) (sources under
www/src/content/docs/).

## What this is

wisp is a compiler, written in Go, for a small C-like language that compiles to
POSIX shell. The output targets dash and busybox ash and depends only on POSIX
sh plus the applets busybox guarantees. The compiler is stdlib-only Go and
builds as a static binary.

## Layout

- `cmd/wisp`: the compiler CLI.
- `cmd/wisp-lsp`: the language server.
- `internal/token`, `lexer`, `parser`, `ast`: the front end.
- `internal/types`: the type checker, the builtin catalog, reserved names, and
  did-you-mean suggestions.
- `internal/codegen`, `internal/runtime`: lowering to shell and the tree-shaken
  runtime prelude.
- `internal/driver`: build, run, and check entry points, diagnostics, source
  maps, exit codes.
- `internal/doc`: the `///` doc-comment extractor and Markdown renderer behind `wisp doc`.
- `internal/format`: the formatter behind `wisp fmt`.
- `internal/lsp`: the language server implementation.
- `internal/editors`: the drift-guard test for the editor grammars.
- `editors/`: the VSCode extension and Vim files.
- `www/src/content/docs/`: canonical user guide, design decisions, and stdlib
  index (the published site). `docs/`: specs and plans (dev artifacts, not
  published).
- `testdata/golden/`: end-to-end fixtures, a `.wisp` and a `.json` per case.
  Regex malformed-pattern abort fixtures must use an unbalanced `[`, never `(`:
  busybox awk silently accepts `(` (rc 0, no abort), so a `(` fixture passes on
  every shell except busybox and breaks CI.

## Commands

```sh
go build ./...
go test ./...
go vet ./...
gofmt -l .          # must print nothing; CI fails otherwise
```

## Authoritative sources

Do not hardcode language facts. Derive them:

- Keywords and operators: `internal/token/token.go`.
- Builtins and reserved constants: `internal/types/builtins.go`, exposed
  through `internal/types/accessors.go` as `TypeNames`, `BuiltinNames`, and
  `ReservedConstants`.
- The canonical list of identifiers a user may NOT define is
  `internal/types/accessors.go` `ReservedNames()` -- the sorted, deduped union of
  keywords, type names (incl `Result`), reserved constants/sum constructors, and
  builtins; the editor drift-guard reconciles against it. ADDITIONALLY any
  identifier beginning with `__` is reserved (a prefix rule, not enumerable). The
  bound words `comparable`/`numeric` are NOT reserved identifiers -- they are
  context-sensitive keywords only in type-parameter bound position, so a user may
  define a function or variable named `comparable`/`numeric` -- and are excluded
  from `ReservedNames()` by design.
- Real, compile-verified syntax for every construct: `testdata/golden/*.wisp`.
  When in doubt about syntax or behavior, read a fixture.

## Invariants to preserve

- Injection safety. Every expansion in generated shell is double-quoted, data
  reaches `printf` only through `%s`, and no runtime string is re-evaluated or
  used as a glob. Do not weaken this.
- No implicit coercion or truthiness in the language. Conversions are explicit.
- Builtins are named in `snake_case`, always. This is the standard library's one
  casing and it never changes: a new builtin is `snake_case` (`read_file`, not
  `readFile` or `ReadFile`). Never add a `camelCase` or `PascalCase` builtin.
- The generated `.sh` must be ShellCheck-clean and run under dash and busybox.
- The editor grammars must list the same keyword, type, builtin, and constant
  sets as the compiler. The `internal/editors` test enforces this; update the
  grammars when you change the tables.
- Adding a builtin spans the checker, codegen, the runtime prelude, the editor
  grammars, and golden fixtures. See [CONTRIBUTING.md](CONTRIBUTING.md).
- Modules link at compile time into one `.sh`. No package-controlled string (a
  `wisp.json` name, an alias, or an `owner/repo`) may reach a shell identifier:
  shell names carry only the compiler-assigned integer module id
  (`__wisp_f_m<modid>_<name>`), and a struct's cross-module identity token
  `Name@modid` never reaches the shell. `<root>/.wisp/` is a compiler-owned
  directory (the installed-package space); `include` may not resolve into it.
- Paren-keyword principle. Every construct keyword in wisp takes a parenthesized
  head: `fn`, `if (...)`, `while (...)`, `for (...)`, `match (...)`, `test ("name")`.
  There is never a non-parenthesized function-like keyword in wisp, under any
  circumstances. Do not add a construct that violates this shape.

## Testing and coverage convention

- Test files are named `*_test.wisp`. They have no `fn main`; the `test` construct
  replaces it. The `test` keyword is only valid in `*_test.wisp` files.
- Run tests with `wisp test [path]`. The default path is `.`; the command walks
  the directory recursively and runs every `*_test.wisp` file under each available
  shell (dash, busybox-sh, bash, zsh -f).
- Measure coverage with `wisp test --coverage [path]`. Coverage is per source line
  against the original source, including uncalled functions. It never hides untested
  code by tree-shaking it out of the universe.
- The full set of assertions is: `assert`, `assert_eq`, `assert_ne`, `assert_some`,
  `assert_none`, `assert_ok`, `assert_err`, `assert_contains`; plus `skip(reason)` and
  `test_tmpdir()`. All are builtins; none require an import.
- Lifecycle: `fn setup() -> void` and `fn teardown() -> void` at the top level of a
  test file run before and after each test body, respectively.
- See [www/src/content/docs/guide/testing.md](www/src/content/docs/guide/testing.md) for the full guide.

## Conventions in generated shell

Names live in the `__` namespace: functions are `__wisp_f_m<modid>_<name>` (e.g.
`__wisp_f_m0_main` for root-module `main`), variables are `__wisp_v_<n>`, and a
single `__ret` register carries return values. Structs,
arrays, dicts, and errors are reference handles with id-keyed backing
variables. bool is the string `true` or `false`. Floats go through awk. See
[www/src/content/docs/guide/internals.md](www/src/content/docs/guide/internals.md) for the full model.

## Disposition-claim convention

- A disposition or status claim in a review note or tracker (e.g. "implemented",
  "resolved", "deferred") cites the verifying commit's short SHA inline, so the
  claim can be checked against the actual change rather than taken on trust.
