# zeabur-oss/.github

The CI, DCO, release and skills workflows every zeabur-oss repository calls, and the shared Renovate
preset. A repository keeps what is its own in two scripts; the workflows here only check out,
install Nix, cache and upload.

## The contract

### `scripts/ci.sh`

- `scripts/ci.sh --list` prints the job names, one per line, without the Nix dev shell.
- `scripts/ci.sh <job>` runs one job. It enters the repository's Nix dev shell itself, so a job
  runs the same on GitHub and on any Linux machine with Nix (and Docker, for cluster jobs).
- A job may leave files in `$CI_ARTIFACTS`; they are uploaded as the artifact `ci-<job>`.
- Inputs of the shared `ci.yml`: `runner`, `fetch-depth` and `rust-cache`.

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
`{os, arch, runner}`, also the platforms a draft is verified on), `finish` with `finish-platforms`
(default `platforms`), `runner`, `signing-environment` and `telemetry-key`. `build` may leave logs in `$CI_ARTIFACTS`; they
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

### Bot pull requests

`actions/bot-pr` opens or updates a pull request with the working tree's changes as
zeabur-oss-bot, whose pull requests start CI. Pass `client-id: ${{ vars.ZEABUR_OSS_BOT_CLIENT_ID }}`,
`private-key`, `branch`, `title` and optionally `body` and `add-paths`; the job needs no write
permission of its own.

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

1. Bump the version in every manifest, add its `CHANGELOG.md` section, merge to `main`.
2. Optionally rehearse: run the repository's Release workflow by hand with the version.
3. Push the tag `vX.Y.Z` from `main`. The workflow leaves a draft release.
4. Check the draft and publish it. Releases are immutable once published.

A tag is a [SemVer 2.0.0](https://semver.org/spec/v2.0.0.html) version with a `v` and no build
metadata. `vX.Y.Z`, `v0.*` included, is a normal release. A tag with a pre-release part
(`v0.1.2-beta.1`, `v1.0.0-rc.1`) is a GitHub pre-release, never marked latest. A pre-release
shares the `CHANGELOG.md` section of its normal version (`## [0.1.2]`), is tagged on `main` like
any other release, and its manifests carry the full version (`0.1.2-beta.1`).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md): signed-off commits ([DCO](DCO)) and Conventional Commits.
