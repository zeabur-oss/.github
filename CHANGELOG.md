# Changelog

All notable changes to the shared workflows, actions and `@zeabur-oss/release` of zeabur-oss/.github. Versions follow [Semantic Versioning](https://semver.org/); each section comes from the change files in `.changeset/` (CONTRIBUTING.md, "Releases").

## 0.2.0 (2026-11-17)

### Breaking Changes

#### Maintenance branches are gone

The Release pull request workflow now runs on `main` only, and the Release workflow no longer accepts a tag that is on a `maint/vX.Y` branch instead of `main`. A repository that has a `maint/*` branch must remove `maint/*` from the `branches` of its release-pr.yml before it updates.

#### A release is release-build.yml and release.yml

`release.yml` is split in two. `release-build.yml` checks the version and builds, smokes and signs every platform; it takes `version`, `platforms`, `runner`, `signing-environment`, `telemetry-key` and the `APPLE_*` secrets, and outputs `version` and `platforms`. `release.yml` publishes, attests, finishes, adds the assets to the draft and verifies it; it takes the two outputs, `finish`, `finish-platforms`, `runner` and `telemetry-key`, and no longer runs on a rehearsal by itself.

A caller's Release workflow gets two jobs: `build` calls `release-build.yml`, and `release`, with `if: github.event_name == 'push'` and `needs: build`, calls `release.yml` with the outputs of `build`. The `signing-environment` and the `APPLE_*` secrets move to the `build` job, whose permissions are `contents: read`.

### Features

#### The tag job leaves the draft release

The tag job of the Release pull request workflow now creates the draft release, with the `CHANGELOG.md` section as its notes, right after it tags the commit. The Release workflow adds the assets to that draft and sets its title and notes, instead of creating it. A caller changes nothing, but a release made from a tag pushed by hand needs `gh release create --draft` first.

#### The tag job owns the release's title and notes

`release-pr.yml` takes `name` (the product, required) and `docs`, and its tag job writes the draft release's title and notes with them. `release.yml` no longer takes `name` and `docs`; it only adds the assets to the draft. A caller moves `name` and `docs` from its Release workflow to its Release pull request workflow.

#### zeabur-oss-release pushes, pins and verifies as commands

`zeabur-oss-release` gains `absent`, `push-image`, `push-chart`, `pin-digest`, `verify-image` and `verify-chart`, so a repository's own Release workflow runs the steps as shell steps and needs no `scripts/release.ts` stage. A workflow that sets neither `RELEASE_CERT_IDENTITY_REGEXP` nor `RELEASE_SIGNER_WORKFLOW` is verified against `.github/workflows/release.yml` of its own repository at the tag, the signer of a repository's own release.

### Fixes

#### Every command step of the shared workflows has a name

The job logs showed the raw command for each `run:` step. Each step now has a short name.

## 0.1.0 (2026-11-17)

### Features

#### ci-setup caches Cargo's build output with kache

With `rust-cache: true`, `actions/ci-setup` installs kache instead of using Swatinem/rust-cache. A caller runs Cargo as `kache cargo -- <command>` in its scripts; a bare `cargo` still builds, but nothing is cached. Only runs of `main` save the cache. The Nix store cache is hestia instead of magic-nix-cache.

#### The shared workflows, actions and release package get versions

Callers still pin by commit; a version names the commit and the release notes list what a caller changes when its pin moves past it. The base shell now has jactionlint instead of actionlint, the shared skills and the Vale style are gone, and `zeabur-oss-release` is called as `bun x zeabur-oss-release`.

### Fixes

#### knope formats the release it writes

A caller's `knope.toml` formats what `PrepareRelease` wrote: add a `Command` step after it, such as `bun fmt CHANGELOG.md package.json` (list the other manifests it changes). Without it the release pull request can fail the caller's format check.
