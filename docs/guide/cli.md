# CLI reference

```
usage: wisp <build|run|check|fmt|test|doc|add|install|remove> [args]
```

## Exit codes

The codes are consistent across commands:

- `2`: a usage or argument error, such as an unknown command, a missing or
  unexpected argument, an unknown flag, or an unreadable source file.
- `1`: a compile error. A gating error diagnostic was reported and no artifact
  was produced.
- `0`: success. Warnings never change the exit code.

`wisp run` is the exception: it forwards the exit status of the program it ran.

## wisp build

```sh
wisp build <src> [-o <out>] [--source-map]
```

Compiles a source file to a single self-contained `.sh` script.

- `-o <out>`: the output path. Without it, the output is the source path with
  its extension replaced by `.sh`. The script is written with mode 0755; a
  rebuild over an existing output resets its mode to 0755. `-o` requires a path
  argument, so a following flag-shaped token (such as `--source-map`) is a usage
  error rather than a filename.
- `--source-map`: also write `<out>.map`, a JSON map from each generated line
  back to its wisp source position. The `.sh` bytes are identical whether or
  not this flag is set.

If the resolved output path is the source file itself -- for example `wisp build
foo.sh`, whose default output would be `foo.sh` -- the build refuses and exits 2
without touching the source; pass `-o` to choose a different path. An unknown
flag (a leading-`-` token that is not `-o`/`--source-map`) is a usage error,
consistent with `check`, `run`, and `fmt`.

A compile error writes diagnostics to stderr, produces no file, and exits 1.

## wisp run

```sh
wisp run <src> [args...]
```

Compiles to a temporary script, runs it under `/bin/sh`, removes the temporary,
and exits with the script's status. Any tokens after the source path are passed
to the program as its own arguments, readable through `main(args: string[])`.

`run` defines no compiler flags. A token starting with `-` in the source
position is a usage error.

## wisp check

```sh
wisp check <src>
```

Type-checks only. It writes every diagnostic to stderr and writes no output
file. It exits 1 if there is any error, otherwise 0. Warnings, such as an
unused local variable, are reported but do not change the exit code.

## wisp fmt

```sh
wisp fmt <src>                     # print formatted source to stdout
wisp fmt -w <src>                  # rewrite the file in place
wisp fmt --check <src>             # verify formatting; write nothing
wisp fmt -w <path>...              # multiple files and/or directories
wisp fmt --check <path>...         # same, for --check
```

The formatter parses the source and prints it in the one canonical style. It is
deterministic and idempotent: formatting already-formatted source is a no-op.

- With no flag, it prints the formatted source to stdout. This requires
  exactly one resolved file; a directory, or more than one path, with no `-w`
  or `--check` is a usage error.
- `-w` rewrites each resolved file in place. It skips the write for a file
  that is already canonical, so it does not touch the modification time
  needlessly.
- `--check` writes nothing to stdout except one path per line, in lexical
  order, for each file that is not already canonical, and exits 1 if any file
  is non-canonical (0 if all are). This is the form to use in CI.
- `-w` and `--check` cannot be combined.

A path argument may be a single `.wisp` file or a directory. A directory is
walked recursively for `*.wisp` files, excluding any `.wisp/` subdirectory (the
compiler's package/module cache). Multiple paths may be given, mixing files and
directories; the resolved set is deduped and processed in lexical order by
path, regardless of argument or filesystem walk order.

`fmt` only formats valid source. A parse error on any file is reported (the
located error, to stderr) and processing continues with the rest; a file with
a parse error, like a non-canonical file, makes the overall exit code
non-zero.

The canonical style is fixed: four-space indentation, one statement per line
with no semicolons, one space around binary operators and after commas and
colons, no space just inside brackets, the opening brace on the same line as
its construct, `else` and `else if` and `catch` and `finally` on the same line
as the preceding brace, one blank line between top-level declarations, and a
single trailing newline. Blank lines within a block are removed. Comments are
preserved (a full-line comment stays inside the block it was written in).

Adjacent module directives (`import`/`include`, in any combination) are an
exception to the one-blank-line rule: no blank line is printed between two
directives that are already adjacent in the source. A comment written between
two directives suppresses this collapse, so the blank line before the comment
is kept. Directives are never reordered or grouped; only the blank line
between an already-adjacent pair collapses.

### Multi-line collection layout

`wisp fmt` preserves the user's layout choice for array, dict, and struct
literals, as well as struct declarations. A literal written on a single line
stays single-line with no trailing comma. A literal written across multiple
lines is kept multi-line: one item per line indented one level deeper than the
construct, a trailing comma after each item including the last, and the closing
bracket or brace on its own line at the construct's indent. The formatter never
auto-wraps a single-line literal and never collapses a multi-line one.

**Comment-placement limitation.** A full-line comment between items of a
multi-line literal is never lost: it is preserved in the output. However, its
exact position may shift to the enclosing block indent rather than staying at
the element indent. Faithful element-indent placement for in-literal comments
is a planned improvement.

## wisp doc

```sh
wisp doc <path>...
```

Extracts `///` doc-comments from one or more `.wisp` files (or directories) and
writes canonical Markdown to stdout.

### The `///` convention

A doc-comment is a `///` line placed directly above a top-level `fn`, `struct`,
`enum`, or `const` declaration with no blank line or regular `//` comment
between them:

```wisp
/// Returns the absolute value of n.
fn abs(n: int) -> int {
    if (n < 0) { return -n }
    return n
}
```

Multiple consecutive `///` lines form a single doc-comment block:

```wisp
/// Parses a decimal integer from s.
/// Returns none when s is not a valid integer.
fn parse_int(s: string) -> Optional[int] {
    return int_of_string(s)
}
```

A `///` comment that appears on the same line as code (trailing comment) never
attaches to anything. Only full-line `///` comments attach.

`///` is an ordinary comment to the wisp compiler: it is ignored by the
type-checker and code generator, and preserved verbatim by `wisp fmt`. It is
interpreted only by `wisp doc`.

### Markdown output shape

For each file, `wisp doc` writes a `## <path>` section. Each documentable
declaration gets a `### <name>` block containing a fenced signature and, when a
doc-comment is attached, the prose below the fence:

```
## path/to/file.wisp

### abs

```
fn abs(n: int) -> int
```

Returns the absolute value of n.

### parse_int

```
fn parse_int(s: string) -> Optional[int]
```

Parses a decimal integer from s.
Returns none when s is not a valid integer.
```

Files with no documentable declarations contribute no section. The output ends
with a single trailing newline.

### Path expansion

- A `.wisp` file argument is documented as-is.
- A directory argument contributes its immediate (non-recursive) `.wisp` files,
  sorted by filename.
- An empty directory (no `.wisp` files) contributes nothing and is not an error.
- Multiple arguments are processed in order; a file named twice is documented twice.

Exit codes for `wisp doc`:

- `0`: success (output written, or no documentable content found).
- `1`: an IO or parse error, or a non-`.wisp` file argument. Nothing is written to stdout.
- `2`: a usage error (no arguments).

## wisp test

```sh
wisp test [path] [--tap] [--filter <regex>] [--shell <name>] [--coverage]
```

Discovers and runs `*_test.wisp` files. `path` is a directory or a single `*_test.wisp`
file; the default is `.` (the current directory). When given a directory, `wisp test`
walks it recursively and runs all `*_test.wisp` files in sorted order.

Each test file is compiled once and executed under every available shell (dash, busybox-sh,
bash, zsh -f). A shell not installed on the current machine is noted and skipped. At
least one shell must be available; if none are, the command exits 1. A cross-shell
divergence -- a test that passes under one shell and fails under another -- is a failure.

Exit codes for `wisp test`:

- `0`: all selected tests passed or were skipped on every shell. A path with no
  `*_test.wisp` files also exits 0, after printing `wisp test: no *_test.wisp files found`.
- `1`: a test failed, a compile error prevented a file from running, or no shells are available.
- `2`: a usage error (bad `--filter` regex, unreadable path, unknown flag).

**`--tap`**

Emit raw TAP version 13 output instead of the human-readable summary. The plan line
(`1..N`) comes first, followed by result lines (`ok N - name [shell]` or
`not ok N - name [shell]`), with `# ...` diagnostic lines after failures, and
`# SKIP <reason>` on the result line for skipped tests.

**`--filter <regex>`**

Select which tests appear in the reported results by the Go regular expression. All tests
still execute under each shell; the filter is applied to the captured TAP output afterward,
so it narrows what is reported, not what runs. The regex is applied to the test name as
written in the source; the shell label (`[dash]` etc.) is not part of the name. A bad regex
is a usage error (exit 2).

**`--shell <name>`**

Restrict execution to one named shell (`dash`, `bash`, `zsh`, `busybox-sh`). The name must
match the shell label exactly as it appears in output (the busybox shell is `busybox-sh`,
not `busybox`). If the named shell is not available on the current machine, the command
exits 1.

**`--coverage`**

After the normal pass/fail summary, print a per-source-file coverage report:

```
--- coverage ---
calc.wisp: 6/6 (100%)
calc_test.wisp: 15/15 (100%)
```

Coverage is line-level against the original source, not a tree-shaken artifact. A
function that no test calls appears in the universe and is reported as uncovered; it does
not silently disappear. Lines with no executions are listed as `uncovered: <line-numbers>`.

`--filter` narrows which test results are displayed but does not change what is measured.
Coverage hits are the union across all shells.

See the [testing guide](testing.md) for the full test framework documentation.

## wisp add, install, remove

These manage git dependencies recorded in `wisp.json` and pinned in `wisp.lock`.
They run from anywhere inside a project; the root is the nearest ancestor
directory containing a `wisp.json`, the same root the compiler resolves imports
against. See [Modules and the project root](#modules-and-the-project-root).

```sh
wisp add <owner/repo> [--git <url>] [--ref <ref>]
wisp install
wisp remove <owner/repo>
```

`wisp add` records a dependency under the `owner/repo` key and then installs.

- The first add of a key requires `--git <url>`. A later add of the same key
  re-pins it and may omit `--git`, reusing the recorded URL.
- `--ref <ref>` pins a branch, tag, or commit. On a first add without it, the
  remote's default branch is discovered and recorded. On a re-pin without it,
  the existing recorded ref is kept.
- The git URL transport must be one of `https://`, `ssh://`, `git://`,
  `file://`, or the scp-like `user@host:path` form.

`wisp install` fetches and verifies every dependency from `wisp.json` and
`wisp.lock`, populating `<root>/.wisp/modules/`. It takes no arguments. A
consistent lock reproduces the same module tree without re-resolving.

`wisp remove <owner/repo>` drops the key from `wisp.json`, re-resolves, prunes
the removed module's tree from `.wisp/modules/`, and updates `wisp.lock`.

`add` and `remove` are atomic: if anything fails, `wisp.json` and `wisp.lock`
are restored to their pre-command bytes. A bad `owner/repo` key, a missing
`--git` on a first add, or extra arguments to `install` are usage errors
(exit 2); a failed fetch, an unresolvable dependency, or removing a key that is
not a direct dependency exits 1.

## Diagnostics

Errors and warnings are located:

```
path/to/file.wisp:12:7: message
```

Compile diagnostics also print the offending source line and a caret under the
column. The caret aligns by display width, so a multibyte UTF-8 character before
the error column does not shift it, and a leading tab is preserved verbatim so
the caret shares the tab stop. Errors anchored at end of input (and the no-main
error on an empty file) still carry the filename and a real position, never
`0:0`. Warnings carry a `warning:` prefix after the location. When the checker
reports an unknown name and there is a single close match, it appends a
suggestion, for example `call to undeclared function "lenght"; did you mean
"length"?`. A suggestion is only offered when one candidate is the unique
closest within a small edit distance. The `wisp:` prefix is reserved for usage
and I/O errors, such as a missing argument or an unreadable file; located
compile diagnostics do not carry it.

## Modules and the project root

When a source file uses `import` or `include`, the compiler resolves the project
root by searching upward from that file for the nearest directory containing a
`wisp.json`; if none is found, the file's own directory is the root. `import
"owner/repo"` resolves against `<root>/.wisp/modules/owner/repo/`. The compiler
only reads what is already present under `.wisp/`; populating it is the job of
the [package tool](#wisp-add-install-remove) (`wisp add`/`install`/`remove`),
which clones repositories into `.wisp/modules/` and writes `wisp.lock`. The
file you pass on the command line is always the build target, regardless of any
`wisp.json` `main` field (which names a package's entry file, not the program
entry). See the [language reference](language.md#modules).
