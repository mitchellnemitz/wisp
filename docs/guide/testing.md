# Testing

wisp ships a native test framework integrated into the compiler. Tests are first-class:
the `test` construct, the assertion builtins, and the `wisp test` runner are part of the
language and the CLI, not a bolt-on library.

## Your first test

Create `math_test.wisp`:

```wisp
test ("addition works") {
    assert_eq(1 + 1, 2)
}

test ("subtraction works") {
    assert_eq(10 - 3, 7)
}
```

Run it:

```sh
wisp test math_test.wisp
# ok    math_test.wisp [dash]: 2 passed, 0 failed, 0 skipped
# ok    math_test.wisp [bash]: 2 passed, 0 failed, 0 skipped
# ok    math_test.wisp [zsh]: 2 passed, 0 failed, 0 skipped
# ---
# 6 passed, 0 failed, 0 skipped
# (this machine has dash, bash, and zsh installed; with busybox also present a
#  [busybox-sh] line appears and the totals rise accordingly)
```

Run with coverage:

```sh
wisp test --coverage math_test.wisp
```

## Test files

A test file must be named `*_test.wisp`. The `test` construct is only valid in files with
that suffix; using it in a regular `.wisp` file is a compile error.

A test file has no `fn main`. The runner generates its own entry point. You may define
helper functions, constants, `setup`, and `teardown` at the top level alongside your
`test` blocks.

## The test construct

```
test ("<name>") { <body> }
```

- `"<name>"` is a string literal. Duplicate names in one file are a compile error.
- The body is a sequence of statements, the same as a function body.
- A test has no return type; it exits by completing normally (pass), calling an assertion
  that fails (fail), or calling `skip(reason)` (skip).

Example showing all three outcomes (lex-only; not runnable as a standalone program):

```wisp
test ("this test passes") {
    assert(true)
    assert_eq(2 + 3, 5)
}

test ("this test is skipped") {
    skip("not implemented yet")
}

test ("this test fails") {
    assert_eq(1, 2)
}
```

## Assertions

All assertions are builtins available everywhere inside a test body. On failure they
immediately fail the current test with a located message showing the file, line, and
column; the runner moves on to the next test.

### assert

```
assert(cond: bool, msg: string = "") -> void
```

Fails if `cond` is false. The optional `msg` is appended to the failure message.

```wisp
test ("assert examples") {
    assert(true)
    assert(1 < 2, "ordering")
}
```

### assert_eq, assert_ne

```
assert_eq[T: comparable](got: T, want: T) -> void
assert_ne[T: comparable](got: T, want: T) -> void
```

`assert_eq` fails if `got != want`; `assert_ne` fails if `got == want`. Both render the
actual and expected values in the failure message via `debug()`.

```wisp
test ("assert_eq and assert_ne examples") {
    assert_eq(1 + 1, 2)
    assert_ne("hello", "world")
}
```

### assert_some, assert_none

```
assert_some(o: Optional[T]) -> void
assert_none(o: Optional[T]) -> void
```

`assert_some` fails if `o` is `None`; `assert_none` fails if `o` is `Some(x)`.

```wisp
test ("optional assertions") {
    let found: Optional[int] = Some(42)
    assert_some(found)
    let missing: Optional[int] = None
    assert_none(missing)
}
```

### assert_ok, assert_err

```
assert_ok(r: Result[T]) -> void
assert_err(r: Result[T]) -> void
```

`assert_ok` fails if `r` is `Err`; `assert_err` fails if `r` is `Ok`.

```wisp
test ("result assertions") {
    let ok: Result[int] = Ok(1)
    assert_ok(ok)
}
```

### assert_contains

```
assert_contains(s: string, sub: string) -> void
assert_contains(xs: T[], x: T) -> void   // T: comparable
```

Overloaded: when the first argument is a string it asserts `sub` is a substring; when it
is an array it asserts `x` is a member.

```wisp
test ("assert_contains examples") {
    assert_contains("hello, world", "world")
    let nums: int[] = [1, 2, 3]
    assert_contains(nums, 2)
}
```

## skip

```
skip(reason: string) -> void
```

Marks the current test skipped and stops executing its body. The runner records the skip
reason and moves on. The test counts as neither passed nor failed.

```wisp
test ("skipped test") {
    skip("waiting for feature X")
    assert_eq(1, 2)   // never reached
}
```

## Lifecycle: setup and teardown

Define zero-argument `setup` and `teardown` functions at the top level of a test file to
run code around each test:

- `setup()` runs before each test body.
- `teardown()` runs after each test body, whether it passed, failed, or was skipped.

State is isolated between tests: each test runs in its own subshell, so variables set in
one body are not visible in another.

Example:

```wisp
fn setup() -> void {
    fs.write_file("state.txt", "fresh")
}

fn teardown() -> void {
    fs.remove_file("state.txt")
}

test ("setup ran before body") {
    assert_eq(fs.read_file("state.txt"), "fresh")
}
```

Both functions are optional. If only one is needed, define only that one.

## test_tmpdir

```
test_tmpdir() -> string
```

Returns the path to a fresh temporary directory created before the current test and
removed after it completes (teardown still runs while the directory exists). The
directory is unique per test, so tests cannot interfere with each other through it.

```wisp
test ("write and read a temp file") {
    let dir: string = test_tmpdir()
    fs.write_file("${dir}/result.txt", "hello")
    assert_eq(fs.read_file("${dir}/result.txt"), "hello")
}
```

## Testing code that runs external programs

Use `process.run_full` to invoke programs and assert on their typed result:

```wisp
test ("run_full captures stdout and exit code") {
    let r: RunResult = process.run_full(["printf", "%s", "hello"])
    assert_eq(r.code, 0)
    assert_eq(r.stdout, "hello")
}
```

`process.run_full` returns a `RunResult` struct with fields `code: int`, `stdout: string`, and
`stderr: string`. This is useful for testing shell-level behavior of the program under
test.

## Testing a library (include + export)

Expose the code under test with `export fn` and pull it into the test file with `include`:

`calc.wisp`:

```wisp
export fn add(a: int, b: int) -> int {
    return a + b
}
```

`calc_test.wisp`:

```wisp
include "./calc.wisp" as calc

test ("add") {
    assert_eq(calc.add(2, 3), 5)
}
```

Run with:

```sh
wisp test calc_test.wisp
```

For a multi-file project, place all files in a directory with a `wisp.json` and point
`wisp test` at the directory: it discovers and runs all `*_test.wisp` files recursively.

## wisp test reference

```
wisp test [path] [--tap] [--filter <regex>] [--shell <name>] [--coverage]
```

`path` is a directory or a single `*_test.wisp` file. When it is a directory, `wisp test`
walks it recursively, collects all `*_test.wisp` files in sorted order, and runs them.
The default path is `.` (the current directory).

Each test file is compiled once and executed under every available shell (dash, busybox-sh,
bash, zsh -f). Shells not installed on the current machine are noted and skipped.
At least one shell must be available; if none are, the command exits 1.

When the path contains no `*_test.wisp` files, `wisp test` prints `wisp test: no *_test.wisp
files found` and exits 0 -- an empty test set is not an error.

### Flags

**`--tap`**

Emit raw TAP-13 output instead of the human summary. TAP is useful for CI systems and
test-result aggregators that consume the standard protocol.

```
TAP version 13
1..6
ok 1 - add: basic arithmetic [dash]
ok 2 - subtract: basic arithmetic [dash]
...
```

**`--filter <regex>`**

Select which tests appear in the reported results, by the Go regular expression. All tests
still execute under each shell; the filter is applied to the captured TAP output afterward,
so it narrows what is reported, not what runs (a side-effectful or slow test still runs even
when filtered out of the display). The regex is matched against the test name as written in
the source; shell label suffixes are not part of the name for matching purposes.

```sh
wisp test --filter "add"
wisp test --filter "^divide"
```

**`--shell <name>`**

Restrict execution to a single named shell (`dash`, `bash`, `zsh`, `busybox-sh`). The name
must match the shell label exactly as it appears in output; the busybox shell is labeled
`busybox-sh`, not `busybox`.

```sh
wisp test --shell dash
wisp test --shell busybox-sh
```

**`--coverage`**

Report code coverage after the normal pass/fail summary. See [Coverage](#coverage) below.

### Exit codes

- `0`: all selected tests passed or were skipped on every shell.
- `1`: at least one test failed, a compile error prevented a file from running, or no shell was available (a requested `--shell` target is not installed, or no supported shell is present at all).
- `2`: a usage error (bad `--filter` regex, unreadable path, unknown flag).

### Cross-shell divergence

If a test passes under one shell and fails under another, `wisp test` marks it a failure
and names the shells where it failed. This ensures your tests are portable, not just
passing on the shell you happen to have installed locally.

## TAP-13 output format

wisp test emits TAP version 13. The plan line comes first, followed by one result per
test per shell:

```
TAP version 13
1..3
ok 1 - a passes [dash]
not ok 2 - b fails an assertion [dash]
# calc_test.wisp:6:3: assertion failed: 1 != 2
ok 3 - c is skipped [dash] # SKIP not ready yet
```

A skipped test carries `# SKIP <reason>` on the result line. A failing test is followed
by `# ...` diagnostic lines with the located failure message.

A file that fails to compile is reported as a single synthetic failure rather than as its
individual tests:

```
not ok 1 - calc_test.wisp (compile error)
```

Infrastructure failures of the runner itself are also surfaced as synthetic `not ok`
lines, so a crashed or truncated run never silently passes:

- `not ok N - <file> [<shell>] (<message>)` when the TAP stream is incomplete or
  unparseable.
- `not ok N - <file> [<shell>] (runner exit code disagrees with TAP)` when the runner's
  exit status contradicts the TAP results it printed.

## Coverage

`wisp test --coverage` measures how many lines of source were executed during the test
run. The report appears after the normal summary:

```
--- coverage ---
calc.wisp: 6/6 (100%)
calc_test.wisp: 15/15 (100%)
```

Each line shows `covered/total (pct%)`. Lines with no executions follow as
`uncovered: <line-numbers>`. When a function in the source file is never called by any
test, its lines appear in the uncovered list.

**Honest coverage semantics:** The coverage universe comes from the original source, not
a tree-shaken artifact. A function that no test references still appears in the universe
and counts as uncovered (0% of its lines). Coverage does not disappear uncalled code --
it reports exactly which source lines were and were not reached.

`--filter` narrows which test results are displayed, but not what coverage measures.
The coverage report always reflects the full test run.

Coverage hits are the union across all shells: a line executed under any shell is counted
as covered. This means 100% coverage in the report means every instrumented line ran on
at least one shell.

## Complete example

See [examples/testing/](../../examples/testing/) for a working example: `calc.wisp`
(library), `calc_test.wisp` (test file), and `wisp.json` (project root marker).

Run it from the repository root:

```sh
wisp test examples/testing/
wisp test --coverage examples/testing/
```
