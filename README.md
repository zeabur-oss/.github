# zeabur-oss/.github

The CI, DCO, changeset, release and skills workflows every zeabur-oss repository calls, and the shared Renovate
preset. A repository keeps what is its own in two scripts; the workflows here only check out,
install Nix, cache and upload.

## The contract

### `scripts/ci.sh`

- `scripts/ci.sh --list` prints the job names, one per line, without the Nix dev shell.
- `scripts/ci.sh <job>` runs one job. It enters the repository's Nix dev shell itself, so a job
  runs the same on GitHub and on any Linux machine with Nix (and Docker, for cluster jobs).
- A job may leave files in `$CI_ARTIFACTS`; they are uploaded as the artifact `ci-<job>`.
- Inputs of the shared `ci.yml`: `runner`, `fetch-depth`, `rust-cache`, and `cache-paths` with
  `cache-key-files` for anything else worth keeping between runs (Go modules, a tool built from
  source), one cache per job keyed on those files.

### `scripts/release.sh`

One entrypoint per stage, called in the dev shell of the commit being released:

| Stage | Arguments | Does |
|---|---|---|
| `check` | `<version>` | The version is the manifests' and has a `CHANGELOG.md` section. |
| `build` | `<version> <os> <arch> <dist>` | Builds one platform into `<dist>` and smokes what it built. |
| `sign` | `<version> <os> <arch> <dist>` | Only with `signing-environment`, on macOS: signs `<dist>`'s build with the Developer ID and notarizes it; leaves it as it is without the `APPLE_*` secrets. |
| `publish` | `<version> <artifacts> <published>` | Pushes by digest and signs; writes `<published>/subjects.json`, `[{"name": ..., "digest": "sha256:..."}]` (`[]` when nothing is pushed). |
| `finish` | `<version> <os> <arch> <artifacts> <dist>` | Only with `finish: true`: per platform, what needs the published digests, such as an installer. |
| `assets` | `<version> <artifacts> <release>` | Writes `<release>/assets/*` with `SHA256SUMS`, and `<release>/notes.md`. |
| `verify` | `<version> <os> <arch> <downloaded>` | Checks the draft release's assets and their signatures. |

Inputs of the shared `release.yml`: `name`, `version`, `verify-only`, `platforms` (JSON array of
`{os, arch, runner}`), `finish` with `finish-platforms`, `verify-platforms` (both default to
`platforms`), `runner` and `signing-environment`. `build` may leave logs in `$CI_ARTIFACTS`; they
are uploaded when it fails.

`signing-environment` names an environment whose deployment branches are `main` and `v*` tags,
holding `APPLE_CERTIFICATE_P12` (the Developer ID Application certificate and key, base64),
`APPLE_CERTIFICATE_PASSWORD`, and an App Store Connect API key for notarization: `APPLE_API_KEY_P8`
(the file's text), `APPLE_API_KEY_ID` and `APPLE_API_ISSUER_ID`. The caller passes the five by
name. macOS builds of `main` and tags run in it; others build unsigned.

`<artifacts>` holds one directory per earlier artifact: `candidate-<os>-<arch>`, `published` and
`finish-<os>-<arch>`.

### Shared steps

[`zeabur-oss-release`](release/README.md) pushes images and charts by digest, signs, writes and
signs `SHA256SUMS`, verifies a release and makes the release notes from `CHANGELOG.md`. A
repository gets it from this flake, pinned in its own `flake.lock`, and calls it from its
`scripts/release.sh` stages.

### Unsafe report

`zeabur-oss-unsafe-report <package>...`, from the same flake and pin, lists the shipped
dependencies of those packages that have `unsafe` code or link native code. It writes
`target/unsafe-report.md` and fails on any that the repository's
`supply-chain/unsafe-allowlist.toml` does not approve. A Rust repository runs it from its
`scripts/ci.sh`; Cargo comes from the caller's dev shell.

### Skills

`skills.yml` copies each skill in `.claude/skills` that `zeabur-oss/skills` also has, by directory
name, again from its `main`, and opens `update/skills` as zeabur-oss-bot. To use a shared skill,
copy its directory from `zeabur-oss/skills/skills/` into `.claude/skills/`. The repository needs
the bot installed and `ZEABUR_OSS_BOT_PRIVATE_KEY` with `ZEABUR_OSS_BOT_CLIENT_ID` visible to it.

### Changes and release pull requests

Each user-facing change brings its changelog entry: a change file in `.changeset/`, written by the
pull request's author and checked by its reviewer. `knope document-change` asks for the level and
summary and writes one:

```md
---
default: minor
---

# Popups keep focus when their trigger is swapped

Optional details, as many paragraphs as needed.
```

The level is the change's effect on users (`major`, `minor`, `patch`; a `major` before 1.0 bumps
the minor), not the commit type: a breaking change to CI is `ci!:` with no change file, or one at
`patch`. `changeset.yml` fails a pull request whose title is `feat`, `fix` or `perf`, or has `!`,
when it adds no change file and has no label `changelog: none`, and fails any change file it adds
that knope would not read.

`release-pr.yml`, run by hand, runs `knope prepare-release` in the caller's dev shell: the change
files become the next `## X.Y.Z (date)` section of `CHANGELOG.md`, the version in every versioned
file follows, and the change files are deleted. zeabur-oss-bot opens `release/next` with that as
`chore(release): vX.Y.Z`; `version` sets the version instead, and `prerelease` makes it a
pre-release. Merged, the same workflow on `main`
tags the commit `vX.Y.Z` as the bot, which starts `release.yml`. Commit titles never reach the
changelog: knope reads only the change files.

The repository needs `knope` in its dev shell and a `knope.toml`:

```toml
[package]
versioned_files = ["package.json"] # every manifest with the version: Cargo.toml, Cargo.lock, ...
changelog = "CHANGELOG.md"

[changes]
ignore_conventional_commits = true

[[workflows]]
name = "prepare-release"

[[workflows.steps]]
type = "PrepareRelease"

[[workflows]]
name = "document-change"

[[workflows.steps]]
type = "CreateChangeFile"
```

It also needs the label `changelog: none`, the bot installed with `ZEABUR_OSS_BOT_PRIVATE_KEY`
and `ZEABUR_OSS_BOT_CLIENT_ID` visible to it, and the bot allowed to create `v*` tags by any tag
ruleset.

### Signatures

Keyless signatures and attestations name the workflow that ran, which is this repository's
`release.yml`, not the caller's. Verify a release of `zeabur-oss/<repo>` with:

```sh
cosign verify <image>@<digest> \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github\.com/zeabur-oss/\.github/\.github/workflows/release\.yml@' \
  --certificate-github-workflow-repository zeabur-oss/<repo> \
  --certificate-github-workflow-ref refs/tags/<version>
gh attestation verify <file> --repo zeabur-oss/<repo> \
  --signer-workflow zeabur-oss/.github/.github/workflows/release.yml
```

The `verify` stage gets these values in `RELEASE_CERT_IDENTITY_REGEXP`, `RELEASE_SIGNER_WORKFLOW`,
`RELEASE_SOURCE_REPOSITORY` and `RELEASE_SOURCE_REF`.

## Calling the workflows

Pin this repository by commit; Renovate updates the pins.

`.github/workflows/ci.yml`:

```yaml
name: CI
on:
  push:
    branches: [main]
  pull_request:
  workflow_call: # the release runs all of CI on the commit it releases
permissions:
  contents: read
concurrency:
  group: ci-${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true
jobs:
  ci:
    uses: zeabur-oss/.github/.github/workflows/ci.yml@<sha> # main
    with:
      rust-cache: true
  dco:
    uses: zeabur-oss/.github/.github/workflows/dco.yml@<sha> # main
```

`.github/workflows/changeset.yml`, apart from CI so that a label or a new title checks again
without rerunning it:

```yaml
name: Changeset
on:
  pull_request:
    types: [opened, edited, synchronize, reopened, labeled, unlabeled]
permissions:
  contents: read
jobs:
  changeset:
    uses: zeabur-oss/.github/.github/workflows/changeset.yml@<sha> # main
```

`.github/workflows/release-pr.yml`:

```yaml
name: Release pull request
on:
  workflow_dispatch:
    inputs:
      version:
        description: 'Version instead of the one the change files give (0.2.0); empty for theirs'
        default: ''
      prerelease:
        description: 'Pre-release label (rc, beta) for vX.Y.Z-<label>.N; empty for a normal release'
        default: ''
  push:
    branches: [main]
permissions:
  contents: read
concurrency:
  group: release-pr-${{ github.ref }}
  cancel-in-progress: false
jobs:
  release-pr:
    uses: zeabur-oss/.github/.github/workflows/release-pr.yml@<sha> # main
    with:
      version: ${{ inputs.version }}
      prerelease: ${{ inputs.prerelease }}
    secrets:
      private-key: ${{ secrets.ZEABUR_OSS_BOT_PRIVATE_KEY }}
```

`.github/workflows/release.yml`:

```yaml
name: Release
on:
  push:
    tags: ['v[0-9]*'] # the shared workflow checks the rest (SemVer)
  workflow_dispatch:
    inputs:
      version:
        description: 'Version to rehearse or verify (v0.1.1); a rehearsal never publishes'
        required: true
      verify_only:
        description: 'Verify the existing draft release of this version; build and publish nothing'
        type: boolean
        default: false
permissions:
  contents: read
concurrency:
  group: release-${{ github.ref }}
  cancel-in-progress: false
jobs:
  checks:
    if: ${{ !inputs.verify_only }}
    uses: ./.github/workflows/ci.yml
  release:
    needs: checks
    if: ${{ !cancelled() && (needs.checks.result == 'success' || inputs.verify_only) }}
    uses: zeabur-oss/.github/.github/workflows/release.yml@<sha> # main
    permissions:
      contents: write
      packages: write
      id-token: write
      attestations: write
    with:
      name: Zeabur Example
      version: ${{ inputs.version }}
      verify-only: ${{ inputs.verify_only == true }}
```

`.github/workflows/skills.yml`, Monday morning in Asia/Taipei like Renovate:

```yaml
name: Skills
on:
  schedule:
    - cron: '0 17 * * 0'
  workflow_dispatch:
permissions:
  contents: read
jobs:
  skills:
    uses: zeabur-oss/.github/.github/workflows/skills.yml@<sha> # main
    secrets:
      private-key: ${{ secrets.ZEABUR_OSS_BOT_PRIVATE_KEY }}
```

`renovate.json`: `{"extends": ["github>zeabur-oss/.github"], ...}` plus the repository's
`enabledManagers` and package rules.

## Releasing

1. Run the repository's Release pull request workflow, with a version only to override the
   change files'.
2. Read `chore(release): vX.Y.Z`, edit its `CHANGELOG.md` section on the branch if needed, and
   merge it. Optionally rehearse first: run the Release workflow by hand with the version.
3. zeabur-oss-bot tags the merge commit; the Release workflow leaves a draft release.
4. Check the draft and publish it. Releases are immutable once published.

For a pre-release, run it with a `prerelease` label (`rc`, `beta`): knope writes `vX.Y.Z-rc.N`, its
own section with every change since the last normal release, and keeps the change files, so the
normal release later collects them all into `## X.Y.Z`.

A tag is a [SemVer 2.0.0](https://semver.org/spec/v2.0.0.html) version with a `v` and no build
metadata. `vX.Y.Z`, `v0.*` included, is a normal release. A tag with a pre-release part
(`v0.1.2-beta.1`, `v1.0.0-rc.1`) is a GitHub pre-release, never marked latest. A pre-release has
its own `CHANGELOG.md` section (`## 0.1.2-rc.1`), or else reads that of its normal version
(`## 0.1.2`); it is tagged on `main` like any other release, and its manifests carry the full
version (`0.1.2-rc.1`).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md): signed-off commits ([DCO](DCO)) and Conventional Commits.
