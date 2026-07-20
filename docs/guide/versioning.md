# Versioning and compatibility

## Policy

wisp is pre-1.0, and under Semantic Versioning
(https://semver.org/spec/v2.0.0.html) a pre-1.0 project (`0.y.z`) carries no
stability guarantee: any release,
including a minor version bump, may contain a breaking language or CLI
change. Concretely, this is the semver-0.x break-on-minor convention: while
the major version stays `0`, a bump in the minor position (`0.x` to
`0.x+1`) is where a breaking change may land, unlike post-1.0 semver, where
only a major bump may break compatibility. Treat every wisp release,
including a minor one, as a potential breaking change until the project
reaches `1.0.0`.

## Reading the CHANGELOG

[`CHANGELOG.md`](../../CHANGELOG.md) at the repository root lists every
notable change, grouped under Keep a Changelog
(https://keepachangelog.com/en/1.1.0/) categories:

- **Added**: a new feature or flag.
- **Changed**: a change in existing behavior.
- **Fixed**: a bug fix.
- **Security**: a fix for a vulnerability.

Before upgrading, check the `[Unreleased]` section and every version entry
between your current pin and the version you're upgrading to, not just the
latest one -- a breaking change may have landed in an intermediate release.

## How `wisp --version` reports its version

The string `wisp --version` prints comes from `internal/version.Number`. In
a local `go build`, this defaults to `"0.0.0-dev"` (`internal/version/version.go:6`).
A binary reporting `0.0.0-dev` is a locally built, unreleased binary, not a
release.

At release time, goreleaser overrides this default with the git tag: pushing
a `v*`-tagged commit triggers the release workflow, and goreleaser's build
config bakes in the tag via `-ldflags -X .../internal/version.Number={{
.Version }}` (`.goreleaser.yaml:19,28`). The git tag, not the source default,
is the source of truth for a released binary's version.

A future release may record the compiler version that produced a lockfile in
`wisp.lock`; today `wisp.lock` does not record which compiler version
produced it.
