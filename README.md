# zeabur-oss/.github

The shared CI setup, the release, changeset and Update workflows, the Renovate preset and
`@zeabur-oss/release` that every zeabur-oss repository uses. Pin this repository by commit of
`main`; the Update workflow moves the pins.

## Set up a repository

1. Install zeabur-oss-bot with Workflows write, and make `ZEABUR_OSS_BOT_PRIVATE_KEY` and
   `ZEABUR_OSS_BOT_CLIENT_ID` visible to the repository.
2. Add the label `changelog: none` and a tag ruleset that lets zeabur-oss-bot create `v*` tags
   while nobody may move or delete them.
3. Add the workflows below, `knope.toml`, `scripts/release.ts` and `scripts/ci/<job>.sh`.
4. Add `@zeabur-oss/release` to `package.json` as
   `github:zeabur-oss/.github#<sha>`, and `renovate.json` with
   `{"extends": ["github>zeabur-oss/.github"]}` plus the repository's `enabledManagers`.
5. Require the checks `ci / ok` and `DCO` (the DCO app) on `main`.

## CI

Each job checks out, runs `actions/ci-setup` and then `nix develop -c scripts/ci/<job>.sh`, so it
runs the same way locally. Each script is bash that starts with `set -euo pipefail` and
`cd "$(dirname "$0")/../.."`.

- `actions/ci-setup` installs Nix. `rust-cache: true` also installs kache; run Cargo as
  `kache cargo -- <command>`.
- Short checks share one job, `lint.sh`. It runs jactionlint on the workflows.
- Slow jobs run only in the merge queue, on a schedule, by hand or on a tag (the `if:` below).
- `ok` needs every job and fails when one failed or was cancelled.
- Jobs run on `instabox-small`. A job without Nix runs on `ubuntu-slim`; `ubuntu-26.04` and macOS
  are for work that needs that platform.

```yaml
name: CI
on:
  push:
    branches: [main]
  pull_request:
  merge_group:
  schedule:
    - cron: "0 16 * * *"
  workflow_dispatch:
permissions:
  contents: read
concurrency:
  group: ci-${{ github.workflow }}-${{ github.event_name }}-${{ github.ref }}
  cancel-in-progress: true
jobs:
  lint:
    runs-on: instabox-small
    timeout-minutes: 15
    steps:
      - uses: actions/checkout@<sha> # v7.0.1
        with:
          persist-credentials: false
      - uses: zeabur-oss/.github/actions/ci-setup@<sha> # main
        with:
          rust-cache: true
      - run: nix develop -c scripts/ci/lint.sh
  kubernetes:
    if: github.event_name != 'pull_request' && !(github.event_name == 'push' && github.ref_type == 'branch')
    runs-on: instabox-small
    timeout-minutes: 40
    steps:
      - uses: actions/checkout@<sha> # v7.0.1
        with:
          persist-credentials: false
      - uses: zeabur-oss/.github/actions/ci-setup@<sha> # main
        with:
          rust-cache: true
      - run: nix develop -c scripts/ci/kubernetes.sh
  ok:
    name: ci / ok
    if: always()
    needs: [lint, kubernetes]
    runs-on: ubuntu-slim
    timeout-minutes: 5
    steps:
      - if: contains(needs.*.result, 'failure') || contains(needs.*.result, 'cancelled')
        run: exit 1
```

## Change files

A pull request that users notice adds a change file: `knope document-change` writes it to
`.changeset/`. The Changeset check fails a `feat`, `fix` or `perf` title, or any title with `!`,
that has neither a change file nor the label `changelog: none`.

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

`knope.toml` needs a `prepare-release` workflow whose `PrepareRelease` step writes `CHANGELOG.md`
and the version to the manifest in `versioned_files`.

## Releasing

1. Run the Release pull request workflow, with `prerelease: rc` for a release candidate. Check the
   pull request it opens, and edit `CHANGELOG.md` on its branch if needed.
2. To rehearse, run the Release workflow by hand with the version. A rehearsal publishes nothing.
3. Merge the release pull request. zeabur-oss-bot tags it and leaves a draft release, and the tag's
   Release workflow adds the assets.
4. Check the draft and publish it. A published release can't change.

A tag is `vX.Y.Z` ([SemVer](https://semver.org/spec/v2.0.0.html)). `vX.Y.Z-rc.N` is a pre-release
and is never marked latest. Before 1.0, a `major` change file makes a minor release and `minor`
makes a patch.

```yaml
name: Release pull request
on:
  workflow_dispatch:
    inputs:
      version:
        description: "Version instead of the one the change files give (v0.2.0); empty for theirs"
        default: ""
      prerelease:
        description: "Pre-release label (rc, beta); empty for a normal release"
        default: ""
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
      name: Zeabur Example
      docs: https://oss.zeabur.com/docs/example/
      version: ${{ inputs.version }}
      prerelease: ${{ inputs.prerelease }}
    secrets:
      private-key: ${{ secrets.ZEABUR_OSS_BOT_PRIVATE_KEY }}
```

```yaml
name: Release
on:
  push:
    tags: ["v[0-9]*"]
  workflow_dispatch:
    inputs:
      version:
        description: "Version to rehearse (v0.1.1); a rehearsal never publishes"
        required: true
permissions:
  contents: read
concurrency:
  group: release-${{ github.ref }}
  cancel-in-progress: false
jobs:
  build:
    uses: zeabur-oss/.github/.github/workflows/release-build.yml@<sha> # main
    permissions:
      contents: read
    with:
      version: ${{ inputs.version }}
  release:
    if: github.event_name == 'push'
    needs: build
    uses: zeabur-oss/.github/.github/workflows/release.yml@<sha> # main
    permissions:
      contents: write
      packages: write
      id-token: write
      attestations: write
    with:
      version: ${{ needs.build.outputs.version }}
      platforms: ${{ needs.build.outputs.platforms }}
```

`release-build.yml` also takes `platforms` (a JSON array of `{os, arch, runner}`), `runner`,
`telemetry-key` and `signing-environment`. `release.yml` also takes `finish`, `finish-platforms`,
`runner` and `telemetry-key`.

To sign macOS builds, pass `signing-environment`: an environment limited to `main` and `v*` tags
that holds `APPLE_CERTIFICATE_P12` (base64), `APPLE_CERTIFICATE_PASSWORD`, `APPLE_API_KEY_P8`,
`APPLE_API_KEY_ID` and `APPLE_API_ISSUER_ID`. Without it, macOS builds are unsigned.

### `scripts/release.ts`

The workflows run `bun scripts/release.ts <stage>` in the repository's dev shell. The stages use
the library of `@zeabur-oss/release` ([release/README.md](release/README.md)).

| Stage     | Arguments                                  | Does                                                                         |
| --------- | ------------------------------------------ | ---------------------------------------------------------------------------- |
| `check`   | `<version>`                                | Fails unless the manifests have this version.                                |
| `build`   | `<version> <os> <arch> <dist>`             | Builds one platform into `<dist>` and smokes it. Logs go to `$CI_ARTIFACTS`. |
| `sign`    | `<version> <os> <arch> <dist>`             | On macOS with `signing-environment`: signs and notarizes the build.          |
| `publish` | `<version> <artifacts> <published>`        | Pushes by digest, signs and writes `<published>/subjects.json`.              |
| `finish`  | `<version> <os> <arch> <artifacts> <dist>` | With `finish: true`: what needs the published digests, such as an installer. |
| `assets`  | `<version> <artifacts> <assets>`           | Copies the release's files into `<assets>`.                                  |
| `verify`  | `<version> <os> <arch> <downloaded>`       | Checks the draft's images and charts.                                        |

`<artifacts>` holds `candidate-<os>-<arch>`, `published` and `finish-<os>-<arch>`.

A Rust repository also runs `bun x zeabur-oss-unsafe-report` in `lint.sh`. It fails on a shipped
crate with `unsafe` or native code that `supply-chain/unsafe-allowlist.toml` does not approve.

### Verify a release

The signer is this repository's `release.yml`, not the caller's:

```sh
cosign verify <image>@<digest> \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github\.com/zeabur-oss/\.github/\.github/workflows/release\.yml@' \
  --certificate-github-workflow-repository zeabur-oss/<repo> \
  --certificate-github-workflow-ref refs/tags/<version>
gh attestation verify <file> --repo zeabur-oss/<repo> \
  --signer-workflow zeabur-oss/.github/.github/workflows/release.yml
```

## Update

Every Monday at 02:30 Asia/Taipei, Update opens `update/zeabur-oss` as zeabur-oss-bot. It moves
every pin of this repository to the head of `main` (the flake input, each `uses:`, and
`@zeabur-oss/release`) and lists the commits in between. A `!` in a subject marks a change you must
act on. Run it by hand when a change is urgent. Renovate leaves these pins alone.

```yaml
name: Update
on:
  schedule:
    - cron: "30 18 * * 0"
  workflow_dispatch:
permissions:
  contents: read
jobs:
  update:
    uses: zeabur-oss/.github/.github/workflows/update.yml@<sha> # main
    secrets:
      private-key: ${{ secrets.ZEABUR_OSS_BOT_PRIVATE_KEY }}
```

## Renovate

`default.json` checks third-party updates daily from 03:00 to 06:59 Asia/Taipei, and waits seven
days after a release. `@zeabur-oss/*` and `zeabur-oss/*` move at once. A repository that pins the
`rancher/k3s` image or `INSTALL_K3S_VERSION` also extends `github>zeabur-oss/.github:k3s` and
enables `custom.regex`.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
