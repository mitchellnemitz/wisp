# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Before 1.0.0, wisp makes no stability guarantee: any release, including a minor
version bump, may contain a breaking language or CLI change. Consult this file
before upgrading. See [the versioning guide](https://mitchellnemitz.github.io/wisp/guide/versioning/) for the
full policy.

## [Unreleased]

### Added

- `wisp --help` and `wisp --version` flags.
- Namespaced core-module support in the LSP (hover/completion for `string.*`,
  `dict.*`, `array.*`, `env.*`, `fs.*`, `process.*`, `math.*`, `json.*`, `regex.*`).
- `parse_int`/`parse_float`/`parse_bool`, returning `Optional[T]` instead of
  aborting on invalid input.
- A must-use warning when an `Optional` or `Result` is discarded as a bare
  statement.
- User-defined enums in two modes: value enums (`enum Name: <int|string|bool>`,
  comparable, usable as a `switch` subject, projected to their backing with
  `to_int()`/`to_string()`/`to_bool()`) and tagged-union enums (payload-carrying
  variants consumed through `match`). Both work across modules via `export enum`.
- Float is comparable everywhere the comparable set is consulted (`==`/`!=`,
  `contains`/`index_of`/`unique`, the `assert_eq`/`assert_ne` family,
  `Optional[float]` equality, the generic `comparable` bound, and as a `switch`
  subject), is a valid value-enum backing, and is a valid dict key. Dict keys
  widen to every comparable scalar plus value enums.
- `<`, `<=`, `>`, `>=`, `math.min`/`math.max`, and `array.sort` accept the full
  comparable scalar set (int, float, bool, string, value enums), matching `==`.
  Bool orders `false < true`; value enums order by backing value.

### Changed

- `wisp fmt` accepts multiple files and/or directories in one invocation.
- `env(name)` and `env.get(name)` now return `Optional[string]` instead of
  aborting on an unset variable (BREAKING).

### Removed

- `int_or`, `float_or`, `get_or`, `env_or`. Use `unwrap_or(parse_int(s), fb)`,
  `unwrap_or(dict.get(d, k), fb)`, and `unwrap_or(env.get(n), fb)` instead.

### Fixed

- Diagnostic rendering sanitizes C0 control bytes and DEL so a hostile source
  line or message cannot inject terminal escape sequences.
- INT_MIN (`-9223372036854775808`) cross-shell arithmetic divergence: value
  operands are now referenced bare inside `$(( ))` so dash/busybox ash/bash read
  the stored value correctly, and `INT_MIN % -1` is folded to 0 to avoid an x86
  `idiv` SIGFPE. (zsh's `$(( ))` still cannot represent 2^63; that residual is a
  loud error, documented in the guide.)
