# zeabur-oss/.github

The CI, DCO and release workflows every zeabur-oss repository calls, and the shared Renovate
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
| `publish` | `<version> <artifacts> <published>` | Pushes by digest and signs; writes `<published>/subjects.json`, `[{"name": ..., "digest": "sha256:..."}]` (`[]` when nothing is pushed). |
| `finish` | `<version> <os> <arch> <artifacts> <dist>` | Only with `finish: true`: per platform, what needs the published digests, such as an installer. |
| `assets` | `<version> <artifacts> <release>` | Writes `<release>/assets/*` with `SHA256SUMS`, and `<release>/notes.md`. |
| `verify` | `<version> <os> <arch> <downloaded>` | Checks the draft release's assets and their signatures. |

Inputs of the shared `release.yml`: `name`, `version`, `verify-only`, `platforms` (JSON array of
`{os, arch, runner}`), `finish` with `finish-platforms`, `verify-platforms` (both default to
`platforms`) and `runner`. `build` may leave logs in `$CI_ARTIFACTS`; they are uploaded when it
fails.

`<artifacts>` holds one directory per earlier artifact: `candidate-<os>-<arch>`, `published` and
`finish-<os>-<arch>`.

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
    tags: ['v[0-9]+.[0-9]+.[0-9]+']
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

`renovate.json`: `{"extends": ["github>zeabur-oss/.github"], ...}` plus the repository's
`enabledManagers` and package rules.

## Releasing

1. Bump the version in every manifest, add its `CHANGELOG.md` section, merge to `main`.
2. Optionally rehearse: run the repository's Release workflow by hand with the version.
3. Push the tag `vX.Y.Z` from `main`. The workflow leaves a draft release.
4. Check the draft and publish it. Releases are immutable once published.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md): signed-off commits ([DCO](DCO)) and Conventional Commits.
