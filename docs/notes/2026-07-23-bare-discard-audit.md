# Bare-discard audit (FR-009, SC-011 fallback)

Part of the fallible-value-handling rationalization (2026-07-23). Enumerates
every bare-statement discard of an `Optional`/`Result` value across the
compilable corpus, using the checker's own "discarded ... value" warning
(added in the must-use-warning task) as the type-aware enumerator instead of
a name grep.

## Scope

Audited: every `.wisp` file under `testdata/golden/` and `examples/` (804
files), compiled through `driver.Compile` (lexer -> parser -> checker),
collecting every diagnostic whose message contains `"discarded"`.

Not audited by this harness: docs code-fences and Go-embedded wisp source
strings. That surface is covered separately by
`internal/doctest/doctest_test.go` (`TestDocBlocksLex`,
`TestDocProgramsCompileAndRun`), which compiles and runs every doc example;
none of those emit a discard warning as of this audit.

The harness itself was a temporary, uncommitted `_test.go` under
`internal/driver/`, deleted after running.

## Enumerated bare-discard sites (4 total)

| Site | Type | Disposition |
|---|---|---|
| `testdata/golden/fs_injection.wisp:28` (`fs.which("-v")`) | `Optional[string]` | Leave. Fixture exercises argument-injection-safety of `fs.which`'s input, not its return value. |
| `testdata/golden/fs_injection.wisp:29` (`fs.which("$(touch pwned4)")`) | `Optional[string]` | Leave. Same as above. |
| `testdata/golden/io_tail_read_stdin_after_read_line.wisp:3` (`read_line()`) | `Optional[string]` | Leave. Fixture intentionally discards the first line to test that a subsequent `read_stdin()` still reads correctly. |
| `testdata/golden/mustuse_warn.wisp:2` (`parse_int("x")`) | `Optional[int]` | Leave. This is the fixture that tests the must-use warning itself; the file's second line (`_ = parse_int("y")`) demonstrates the deliberate-discard escape hatch alongside it. |

Disposition rule applied (per plan Step 2): default is leave-it (the
non-gating warning is acceptable for fixtures that intentionally exercise
behavior via a bare call); convert to `_ = ...` only if the discard were
clearly a mistake asserting nothing. None of the 4 sites met that bar, so
none were rewritten to `_ = ...`.

## SC-014 zero-references sweep — Category A (kept: intentional documentation)

These reference the removed names `int_or`/`float_or`/`get_or`/`env_or` in
non-call, documentary contexts and are intentionally kept:

- `testdata/golden/sugar_parity.wisp:1,7,15,20,24` — parity comments
  (`// int_or(s, fb) == unwrap_or(...)`) documenting the migration.
- `internal/types/collections_test.go:44`,
  `internal/types/fs_test.go:14,77`,
  `internal/codegen/fs_test.go:13`,
  `internal/golden/treeshake_test.go:143-144` — explanatory comments.
- `internal/golden/treeshake_test.go:159` — the
  `[]string{"__wisp_int_or","__wisp_float_or","__wisp_env_or"}` list, which
  asserts these generated helpers are *absent* from tree-shaken output
  (SC-015). This is a negative assertion, not a live reference.

## Category B — fixed

- `examples/numeric.wisp:6-7` — printed labels said `int_or:`/`float_or:`
  though the calls had already migrated to
  `unwrap_or(parse_int(...), ...)`/`unwrap_or(parse_float(...), ...)`.
  Relabeled to `parse_int:`/`parse_float:`.
- `internal/doctest/doctest_test.go:38` — updated the pinned expected stdout
  for `numeric.wisp` to match the relabeled output byte-for-byte.
