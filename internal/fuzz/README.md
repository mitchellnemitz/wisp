# Cross-shell differential fuzzer (local only)

`internal/fuzz` (driver `cmd/wisp-fuzz`) is a local-only cross-shell differential
fuzzer for the wisp compiler. It generates well-typed wisp programs from a seed,
compiles each once via `internal/driver`, runs the single compiled artifact under
every shell discovered by `internal/testrunner` (dash, busybox `ash`, bash, zsh),
and reports any cross-shell divergence (raw stdout bytes or exit status) or
recompile instability (the same source compiled twice yields different bytes).

## Not wired into CI (FR-014)

This fuzzer NEVER runs in per-PR CI. Every test that executes real shells is behind
the `fuzzshell` build tag, so a plain `go test ./...` compiles the package but runs
no shell. Run the real-shell suite deliberately, locally:

```
go test -tags fuzzshell ./internal/fuzz/
```

All four shells must be installed; a missing shell fails the preflight
(`RequirePrereqs`) rather than silently running a reduced matrix (FR-019). Where a
shell is absent, the `fuzzshell` tests SKIP.

## Commands

```
go run ./cmd/wisp-fuzz seeded            # run the embedded canonical manifest
go run ./cmd/wisp-fuzz seeded -seed N -bound-value K   # one ad-hoc (seed,bound)
go run ./cmd/wisp-fuzz soak  -seed N      # generate until Ctrl-C
go run ./cmd/wisp-fuzz replay             # replay the curated regression corpus
go run ./cmd/wisp-fuzz regen -seed N -index I   # regenerate one program by provenance
```

`seeded` and `replay` exit nonzero on any finding. A divergence is persisted
(source, per-shell outputs, provenance, carve decision) under
`internal/fuzz/corpus/findings`; each finding is shrunk to a 1-minimal program and
re-oracled so the stored outputs match the stored source.

Coverage is reported per builtin FAMILY and per payload/language CATEGORY. The ten
admitted families are: `array`, `collection`, `convert`, `debug`, `dict`, `math`,
`option`, `parse`, `regex`, `string`.

## The embedded, digest-guarded canonical manifest (FR-021)

The canonical baseline manifest is `internal/fuzz/corpus/manifest.json`, embedded
into the binary via `go:embed` and read through `LoadCanonicalManifest`. Editing it
has no effect until the package is rebuilt -- the embedded bytes are what run, and
`CanonicalManifestDigest` / `loadManifestWithDigest` refuse an on-disk manifest whose
bytes do not hash to the embedded digest. Changing the baseline is therefore a
deliberate, reviewed edit-and-rebuild, not an accidental file swap. The `-manifest`
flag on `seeded` is an exploration escape hatch only; it is bannered NON-CANONICAL
and never counts as the clean baseline.

## The int64-boundary carve-out (FR-015, extended 2026-08-17)

zsh's `$(( ))` engine converts an operand's value text with its own number parser,
which truncates any magnitude above 2^63-1 after 18 digits ("number truncated after
18 digits") and continues with the truncated value, exit unchanged -- whereas dash,
busybox, and bash convert the same value text to a signed 64-bit integer and are
correct. This is a documented pre-existing zsh limitation (design-decisions.md), not
a wisp bug, so the oracle carves zsh out of the comparison for the narrow construct
that hits it: a boundary-tainted value reaching a source-level `$(( ))` arithmetic
operand. A value is boundary-tainted if it derives from any of the four boundary
sources -- the INT64_MIN/INT64_MAX literals or `math.int_min()`/`math.int_max()` --
through the IR-visible data flow: variable bindings, arithmetic composition,
array/dict literals, and the `dict.get`/`unwrap_or`/index carriers. The carve is
decided STRUCTURALLY from the IR (`programReachesBoundaryArith`), is logged when it
fires, and excludes ONLY zsh -- the other three shells must still agree. The
original Design-B carve (`programReachesIntMinArith`, min-side direct) is retained
as the SC-008 done-signal predicate and is subsumed by the general one. Nothing else
is carved: the INT_MIN magnitude into `abs`/`gcd`/`lcm` aborts uniformly on all four
shells, `min`/`max`/`clamp`/`sign` lower to integer `[ -lt ]` tests that zsh
evaluates correctly, and comparison-only operands never reach `$(( ))` at all (they
lower to `[ ]` integer tests, where zsh reads the value text without the truncation)
-- so carving those shapes would mask a real regression. The empirical
record behind the 2026-08-17 extension (six first-baseline divergences, two
detector gaps) is source-cited in `internal/fuzz/intmin.go`. The compiler side of
the ORIGINAL min-side contract lives in `internal/codegen/expr.go` (`arith()`),
which references INT_MIN operands BARE inside `$(( ))` precisely so dash/busybox/bash
read the stored value correctly.

## Done signal (SC-001) and the SC-008 regression direction

1. In `internal/codegen/expr.go`, in `arith()`, change BOTH INT_MIN branches to the
   dollar form: the literal-spill (return `"$"+t` instead of bare `t`) and the
   variable branch (return `"$"+a.name` instead of bare `a.name`). This reintroduces
   the dash off-by-one on any INT_MIN operand in `$(( ))`.
2. Generator direction (SC-001): run `go run ./cmd/wisp-fuzz seeded`. Expect a
   nonzero exit and a `DIVERGENCE` line for a program putting INT_MIN into arithmetic
   -- a dash-vs-bash divergence (zsh carved out per FR-015).
   `TestDoneSignalReachableViaVar` guarantees the manifest contains such a program.
3. Replay/regression direction (SC-008 fail half): run
   `go run ./cmd/wisp-fuzz replay`. Expect a nonzero exit and a `REGRESSION` line for
   the curated `corpus/regressions/intmin-via-var.json` entry -- its stored, carve-
   clean source now diverges dash-vs-bash under the reverted compiler.
4. Revert both edits; re-run both commands; expect `clean: no divergences` and
   `corpus clean: N entries pass` respectively.

The done signal requires all four shells installed (busybox included); it cannot be
exercised end-to-end without them, since a genuine cross-shell divergence cannot be
produced on a correct compiler.

## The curated regression entry

`corpus/regressions/intmin-via-var.json` is CURATED, not discovered: on the correct
(carve-clean) compiler the fuzzer never diverges on this construct, so it is
hand-authored (its source generated once via `Print` for byte-exact fidelity). Its
provenance (`seed 0, bound_value 1, program_index 0`) is a deliberate PLACEHOLDER,
not a pointer to a real pre-shrink run -- replay runs the STORED `source` and stored
`carved_zsh` decision, never a regeneration from provenance (FR-017; enforced by
`TestReplayUsesStoredSourceNotProvenance`), so the placeholder is inert.
`TestCuratedReplayPasses` (fuzzshell) asserts it replays clean on the correct
compiler; the done signal above covers the fail direction.
