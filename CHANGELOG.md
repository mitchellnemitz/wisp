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

### Changed

- `wisp fmt` accepts multiple files and/or directories in one invocation.

### Fixed

- Diagnostic rendering sanitizes C0 control bytes and DEL so a hostile source
  line or message cannot inject terminal escape sequences.

## [0.1.0] - 2026-06-23

First tagged release. Feature-complete for v1 (M1-M6) plus the M7 I/O
builtins, with an LSP, VSCode/Vim support, prebuilt binaries, and full
documentation.

> Note: several language forms were changed or removed during pre-0.1.0
> development (for example an early `if let` binding form and older match-arm,
> array-type, and conversion-builtin spellings). These predate the first
> tagged release, so they were never part of a published version; a `.wisp`
> file written against that early syntax may fail to parse on 0.1.0 or later.
