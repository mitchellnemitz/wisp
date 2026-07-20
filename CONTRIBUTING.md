# Contributing

## Prerequisites

- Go 1.26 or newer.
- For the full golden suite locally: `dash`, `busybox`, and `shellcheck` on
  your PATH. The suite still runs without them, but the shell-execution and
  lint checks that need them are skipped.

## Build

```sh
go build ./...                       # build everything
go build -o wisp ./cmd/wisp          # the compiler
go build -o wisp-lsp ./cmd/wisp-lsp  # the language server
```

## Test

```sh
go test ./...        # the whole suite
go vet ./...
gofmt -l .           # must print nothing
```

CI runs the same checks plus a golden job, which compiles and runs the fixtures
under dash, busybox ash, bash, and zsh and ShellChecks the generated output with
a pinned ShellCheck, and a static-binary job, which builds both binaries with
`CGO_ENABLED=0` and runs the hello example in a busybox container. Keep
`gofmt -l` clean; the build-test job fails on any unformatted file.

## Golden fixtures

`testdata/golden/` holds the end-to-end tests. Each case is a `.wisp` program
and a matching `.json` describing the expected compile result, output, and exit
status. They are the source of truth for language behavior, and they are the
best reference for exact syntax. When you change codegen or add a feature, add
or update fixtures and run `go test ./...` to regenerate and check them.

## Adding a builtin

A builtin touches a few layers. Using an existing one as a template is the
fastest path.

1. Reserve the name and its type in `internal/types/builtins.go`. This alone
   makes the name reserved, so a user cannot redeclare it. The name must be
   `snake_case` (all lowercase, underscores between words), like every other
   builtin; `camelCase` and `PascalCase` are never used. For an overloaded or
   generic builtin, add the checker logic next to the existing handlers in
   `internal/types/stdlib.go` and `internal/types/call.go`.
2. Lower the call in `internal/codegen` and add a runtime helper in
   `internal/runtime/prelude.go` with its dependency list. Helpers are
   tree-shaken, so mark the helper used where you emit the call. Keep the
   generated shell injection-safe: quote every expansion and never re-evaluate
   a runtime string.
3. Update the editor grammars in `editors/vscode/syntaxes/wisp.tmLanguage.json`
   and `editors/vim/syntax/wisp.vim`. The drift-guard test in
   `internal/editors` fails until the grammars list the new name, since the
   builtin set is derived from the compiler.
4. Add golden fixtures that cover the behavior and its edge cases.
5. Run `go test ./...`. Make sure the generated output is ShellCheck-clean.

## Design docs

The rationale behind wisp's design is recorded in
[docs/design-decisions.md](docs/design-decisions.md). User-facing documentation
lives in `docs/guide/`.
