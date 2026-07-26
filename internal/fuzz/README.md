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

## The Design-B INT_MIN carve-out (FR-015)

zsh's `$(( ))` arithmetic cannot represent INT_MIN (magnitude 2^63, one past
INT_MAX): it truncates the 19-digit literal, whereas dash, busybox, and bash all
store and evaluate it correctly. This is a documented pre-existing zsh limitation,
not a wisp bug, so the oracle carves zsh out of the comparison for the single narrow
construct that hits it: an INT_MIN source (the literal or `math.int_min()`) reaching
a source-level `$(( ))` arithmetic operand, directly or through a variable bound to
it. The carve is decided STRUCTURALLY from the IR by data flow
(`programReachesIntMinArith`), is logged when it fires, and excludes ONLY zsh -- the
other three shells must still agree. Nothing else is carved: INT_MIN into
`abs`/`gcd`/`lcm` aborts uniformly on all four shells, and `min`/`max`/`clamp`/`sign`
lower to integer `[ -lt ]` tests that zsh evaluates correctly, so carving them would
mask a real regression. The compiler side of this contract lives in
`internal/codegen/expr.go` (`arith()`), which references INT_MIN operands BARE inside
`$(( ))` precisely so dash/busybox/bash read the stored value correctly.

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
