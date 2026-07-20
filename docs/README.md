# wisp documentation

wisp is a small, explicit, C-like language that compiles to POSIX shell. The
output targets dash and busybox ash and depends only on POSIX sh plus the applets
busybox guarantees, so the scripts it produces run anywhere a minimal shell is
present. The compiler is written in Go and ships as a static binary.

## Guide

Start here if you want to write wisp.

- [Getting started](guide/getting-started.md): install the compiler, write and run your first program.
- [Testing and coverage](guide/testing.md): write tests with the `test` construct, run them cross-shell with `wisp test`, and measure coverage with `--coverage`. Core feature.
- [Language reference](guide/language.md): types, expressions, control flow, functions, structs, arrays, dicts, error handling, modules.
- [Standard library](guide/stdlib.md): every builtin, with signatures, semantics, and edge cases.
- [CLI reference](guide/cli.md): `wisp build`, `run`, `check`, `test`, `fmt`, and `doc`, plus exit codes and source maps.
- [Editor tooling](guide/tooling.md): the `wisp-lsp` language server and the VSCode and Vim integrations.
- [Internals](guide/internals.md): the compile pipeline, codegen conventions, and the generated-shell runtime model.

For what is planned next, see the [standard library index](stdlib-index.md): a terse, status-marked list of current and proposed builtins.

## Contributing

- [CONTRIBUTING.md](../CONTRIBUTING.md): build, test, and the steps to add a builtin.
- [AGENTS.md](../AGENTS.md): orientation for AI coding agents working in this repo.

## Design

The rationale behind the language's design -- the load-bearing decisions and the
alternatives considered -- is recorded in [design-decisions.md](design-decisions.md).
