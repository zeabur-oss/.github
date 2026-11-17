# Changelog

All notable changes to the shared workflows, actions and `@zeabur-oss/release` of zeabur-oss/.github. Versions follow [Semantic Versioning](https://semver.org/); each section comes from the change files in `.changeset/` (CONTRIBUTING.md, "Releases").

## 0.1.0 (2026-11-17)

### Features

#### ci-setup caches Cargo's build output with kache

With `rust-cache: true`, `actions/ci-setup` installs kache instead of using Swatinem/rust-cache. A caller runs Cargo as `kache cargo -- <command>` in its scripts; a bare `cargo` still builds, but nothing is cached. Only runs of `main` save the cache. The Nix store cache is hestia instead of magic-nix-cache.

#### The shared workflows, actions and release package get versions

Callers still pin by commit; a version names the commit and the release notes list what a caller changes when its pin moves past it. The base shell now has jactionlint instead of actionlint, the shared skills and the Vale style are gone, and `zeabur-oss-release` is called as `bun x zeabur-oss-release`.

### Fixes

#### knope formats the release it writes

A caller's `knope.toml` formats what `PrepareRelease` wrote: add a `Command` step after it, such as `bun fmt CHANGELOG.md package.json` (list the other manifests it changes). Without it the release pull request can fail the caller's format check.
