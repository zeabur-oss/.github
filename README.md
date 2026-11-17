# zeabur-oss/.github

The shared CI setup, the release and changeset workflows, the Renovate preset and
`@zeabur-oss/release` that every zeabur-oss repository uses. Pin this repository by commit of
`main`; Renovate moves the pins.

## Set up a repository

1. Install zeabur-oss-bot, and make `ZEABUR_OSS_BOT_PRIVATE_KEY` and
   `ZEABUR_OSS_BOT_CLIENT_ID` visible to the repository.
2. Add the label `changelog: none` and a tag ruleset that lets zeabur-oss-bot create `v*` tags
   while nobody may move or delete them.
3. Add the workflows below, `knope.toml` and `scripts/ci/<job>.sh`.
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
- Every job, `ok` too, runs on Instabox (`instabox-small`, or `-medium` and `-large` for heavy
  jobs), so no job waits for a GitHub-hosted runner. `ubuntu-26.04` and macOS are only for work
  that needs that platform.

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
    runs-on: instabox-small
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
3. Merge the release pull request. zeabur-oss-bot tags it and creates a draft release, and the
   tag's Release workflow adds the assets. A tag pushed by hand needs
   `gh release create <tag> --draft` first.
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
      # The check job of release.yml checks the version; there is no scripts/release.ts.
      check: false
    secrets:
      private-key: ${{ secrets.ZEABUR_OSS_BOT_PRIVATE_KEY }}
```

### The Release workflow

Each repository writes its own `.github/workflows/release.yml`, with no `scripts/release.ts`.
[builder's](https://github.com/zeabur-oss/builder/blob/main/.github/workflows/release.yml) is the
model:

- `check`: the manifests and `CHANGELOG.md` have the version.
- `build`, one job for each architecture: build, smoke, and upload the archives.
- `publish`: `zeabur-oss-release push-image` and `push-chart` push the archives that were smoked,
  by digest, and sign them.
- `attest`, `assets` (`checksums`, then `gh release upload` to the draft) and `verify`
  (`verify-assets`, `verify-image`, `verify-chart`).

Each step is a shell line in the dev shell (`defaults.run.shell: nix develop --command bash -eo
pipefail {0}`) that calls crane, helm, cosign, gh or `bun x zeabur-oss-release`
([release/README.md](release/README.md)). The signer is the repository's own `release.yml`:

```sh
cosign verify <image>@<digest> \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github\.com/zeabur-oss/<repo>/\.github/workflows/release\.yml@' \
  --certificate-github-workflow-ref refs/tags/<version>
gh attestation verify <file> --repo zeabur-oss/<repo> \
  --signer-workflow zeabur-oss/<repo>/.github/workflows/release.yml
```

If the `verify` job fails because of the check itself, fix the check and run the failed jobs again.

## Renovate

`default.json` checks third-party updates daily from 03:00 to 06:59 Asia/Taipei, and waits seven
days after a release. `@zeabur-oss/*` and `zeabur-oss/*` move at once.

The pins of this repository move to the head of `main` together, in one pull request
`renovate/zeabur-oss.github`: each `uses:`, `@zeabur-oss/release` with `bun.lock`, and the flake
input with `flake.lock`. The flake input must name a commit,
`url = "github:zeabur-oss/.github/<sha>";`, and `enabledManagers` must list `nix`; Renovate does not
move an input without one. A `!` in a linked commit's subject marks a change you must act on.

A repository that pins the
`rancher/k3s` image or `INSTALL_K3S_VERSION` also extends `github>zeabur-oss/.github:k3s` and
enables `custom.regex`.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
