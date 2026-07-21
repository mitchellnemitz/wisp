---
title: Getting started
---


## Install the compiler

The quickest path on Linux or macOS is the install script, which downloads the
latest `wisp` and `wisp-lsp` release binaries:

```sh
curl -fsSL https://raw.githubusercontent.com/mitchellnemitz/wisp/main/install.sh | sh
```

It installs to `/usr/local/bin` by default; override with `PREFIX`, and pin a
version with `WISP_VERSION`.

To build from source instead, use the Go toolchain (Go 1.26 or newer). From the
repository root:

```sh
go build -o wisp ./cmd/wisp
```

Move the binary onto your PATH, or run it in place as `./wisp`. The compiler is
pure Go with no cgo, so you can also build a static binary for a minimal
container:

```sh
CGO_ENABLED=0 go build -o wisp ./cmd/wisp
```

## Your first program

Create `hello.wisp`:

```wisp
fn main() -> int {
  print("hello, world")
  return 0
}
```

Every program needs exactly one `main`. Its return value is the process exit
code.

## Run it

`wisp run` compiles to a temporary script, executes it under `/bin/sh`, and
forwards the exit status:

```sh
wisp run hello.wisp
# hello, world
```

## Build a standalone script

`wisp build` writes a self-contained `.sh` file. With no `-o`, the output path
is the source path with a `.sh` extension:

```sh
wisp build hello.wisp          # writes hello.sh
sh hello.sh                    # hello, world
```

The generated script depends only on POSIX sh and the applets busybox
guarantees (awk is used for float math). It runs under dash and busybox ash
with no other runtime.

## Type-check without building

```sh
wisp check hello.wisp
```

This reports type errors and warnings and writes nothing. It exits non-zero if
there is an error.

## Format

```sh
wisp fmt hello.wisp            # print formatted source to stdout
wisp fmt -w hello.wisp         # rewrite the file in place
wisp fmt --check hello.wisp    # exit non-zero if not already formatted
```

## Versioning and compatibility

wisp is pre-1.0 and makes no stability guarantee: a new release, including a
minor version bump, may contain a breaking language change. If a `.wisp` file
that worked with an older `wisp` now fails with an opaque parse error, the
syntax it uses may have been changed or removed -- check
[`CHANGELOG.md`](https://github.com/mitchellnemitz/wisp/blob/main/CHANGELOG.md) for what changed. See
[the versioning guide](/guide/versioning/) for the full policy.

## Write and run your first test (with coverage)

Create `add_test.wisp`:

```wisp
test ("add works") {
    assert_eq(1 + 1, 2)
}

test ("subtraction works") {
    assert_eq(5 - 2, 3)
}
```

Test files are named `*_test.wisp` and have no `fn main`. Run them with `wisp test`:

```sh
wisp test add_test.wisp
```

wisp runs every test under each available shell (dash, busybox-sh, bash, zsh) and reports
the result per shell. Shells that are not installed are silently skipped, so the exact
lines depend on your machine; on one with dash, bash, and zsh installed (no busybox) the
output is:

```
ok    add_test.wisp [dash]: 2 passed, 0 failed, 0 skipped
ok    add_test.wisp [bash]: 2 passed, 0 failed, 0 skipped
ok    add_test.wisp [zsh]: 2 passed, 0 failed, 0 skipped
---
6 passed, 0 failed, 0 skipped
```

With busybox also installed, a fourth `[busybox-sh]` line appears and the total rises
to `8 passed`.

Add `--coverage` to see which lines were executed:

```sh
wisp test --coverage add_test.wisp
```

Point `wisp test` at a directory to run all `*_test.wisp` files inside it recursively:

```sh
wisp test .
wisp test --coverage .
```

See the [testing guide](/guide/testing/) for assertions, lifecycle hooks, `test_tmpdir`, TAP
output, and coverage semantics.

## Next steps

- The [testing guide](/guide/testing/) covers the full test framework.
- The [language reference](/guide/language/) covers the full syntax and type system.
- The [standard library](/guide/stdlib/) lists every builtin.
- The [CLI reference](/guide/cli/) documents every command and flag, including `wisp doc`,
  which extracts `///` doc-comments to Markdown.
- See [examples/](https://github.com/mitchellnemitz/wisp/tree/main/examples/) for complete programs that exercise structs,
  arrays, dicts, errors, and higher-order functions.
