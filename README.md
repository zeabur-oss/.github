# zeabur-oss/.github

The CI, DCO, changeset, release pull request, release and skills workflows every zeabur-oss repository calls, and the shared Renovate
preset. A repository keeps what is its own in two scripts; the workflows here only check out,
install Nix, cache and upload.

## The contract

### `scripts/ci.sh`

- `scripts/ci.sh --list` prints the job names, one per line, without the Nix dev shell.
- `scripts/ci.sh <job>` runs one job. It enters the repository's Nix dev shell itself, so a job
  runs the same on GitHub and on any Linux machine with Nix (and Docker, for cluster jobs).
- A job may leave files in `$CI_ARTIFACTS`; they are uploaded as the artifact `ci-<job>`.
- Inputs of the shared `ci.yml`: `runner`, `fetch-depth` and `rust-cache`.

### `scripts/release.ts`

One entrypoint per stage, `bun scripts/release.ts <stage>`, run in the dev shell of the commit being released (so the dev shell has Bun):

| Stage     | Arguments                                  | Does                                                                                                                                                        |
| --------- | ------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `check`   | `<version>`                                | The version is the manifests'. The workflow then checks the `CHANGELOG.md` section.                                                                         |
| `build`   | `<version> <os> <arch> <dist>`             | Builds one platform into `<dist>` and smokes what it built.                                                                                                 |
| `sign`    | `<version> <os> <arch> <dist>`             | Only with `signing-environment`, on macOS: signs `<dist>`'s build with the Developer ID and notarizes it; leaves it as it is without the `APPLE_*` secrets. |
| `publish` | `<version> <artifacts> <published>`        | Pushes by digest and signs; writes `<published>/subjects.json`, `[{"name": ..., "digest": "sha256:..."}]` (`[]` when nothing is pushed).                    |
| `finish`  | `<version> <os> <arch> <artifacts> <dist>` | Only with `finish: true`: per platform, what needs the published digests, such as an installer.                                                             |
| `assets`  | `<version> <artifacts> <assets>`           | Copies the release's files into `<assets>`. The workflow adds the signed `SHA256SUMS` and the notes.                                                        |
| `verify`  | `<version> <os> <arch> <downloaded>`       | Checks the draft's images and charts. The workflow has checked the assets and `SHA256SUMS` first.                                                           |

Inputs of the shared `release.yml`: `name`, `docs` (the documentation the notes link to), `version`, `platforms` (JSON array of
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
signs `SHA256SUMS`, verifies a release and copies the release notes from `CHANGELOG.md`. A
repository depends on it as `@zeabur-oss/release` in its `package.json`, pinned in its
`bun.lock`, and imports it in its `scripts/release.ts` stages; the workflow runs `notes`,
`checksums` and `verify-assets` itself, from the same package.

### Unsafe report

`zeabur-oss-unsafe-report <package>...`, the second command of `@zeabur-oss/release`, lists the shipped
dependencies of those packages that have `unsafe` code or link native code. It writes
`target/unsafe-report.md` and fails on any that the repository's
`supply-chain/unsafe-allowlist.toml` does not approve, and on any entry there whose crate and kind
the shipped graph doesn't need: a crate that comes back needs a new approval. A Rust repository runs
`node_modules/.bin/zeabur-oss-unsafe-report` from its `scripts/ci.sh`; Cargo comes from the caller's
dev shell.

### Change files and release pull requests

A repository has a `knope.toml` with a `prepare-release` workflow: its `PrepareRelease` step turns
the change files in `.changeset/` into the next `## X.Y.Z (YYYY-MM-DD)` section of `CHANGELOG.md`
and sets the version in the file `versioned_files` names; a `Command` step with
`variables = { "$version" = "Version" }` may set it in the other manifests. `changeset.yml` checks
every pull request: a `feat`, `fix` or `perf` title, or any with `!`, needs a change file or the
label `changelog: none`, and each change file must name a package of `knope.toml` with `major`,
`minor`, `patch` or a type of the package's `extra_changelog_sections` (such as `security` for a
`### Security` section, which knope releases as a patch) and start with a `# ` summary. `release-pr.yml`, run by hand, runs knope,
`bun scripts/release.ts check` (unless `check: false`), and opens `chore(release): vX.Y.Z` from
`release/next` as zeabur-oss-bot; its inputs are `version`, `prerelease` (a label such as `rc`),
`check` and `runner`. On `main`, the merged release pull request is tagged by the bot, whose tag
starts the repository's release. Both run `zeabur-oss-release` from the caller's `bun.lock`.

The repository needs the label `changelog: none`, and a tag ruleset that lets zeabur-oss-bot create
`v*` tags while nobody may move or delete them.

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

If the `verify` job fails on a draft, fix the check and run it again from a checkout, in the dev
shell, after `bun install`:

```sh
export RELEASE_CERT_IDENTITY_REGEXP='^https://github\.com/zeabur-oss/\.github/\.github/workflows/release\.yml@' \
  RELEASE_SIGNER_WORKFLOW=zeabur-oss/.github/.github/workflows/release.yml \
  RELEASE_SOURCE_REPOSITORY=zeabur-oss/<repo> RELEASE_SOURCE_REF=refs/tags/<version>
gh release download <version> --repo zeabur-oss/<repo> --dir downloaded
node_modules/.bin/zeabur-oss-release verify-assets downloaded
bun scripts/release.ts verify <version> linux arm64 downloaded
```

## Runners

Jobs run on the newest Ubuntu, `ubuntu-26.04-arm`. A job that only runs a short script without Nix
(a check of the pull request, a summary) runs on `ubuntu-slim`. `ubuntu-26.04` and macOS are only
for work that needs that platform, such as an amd64 build or e2e run.

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
    tags: ["v[0-9]*"] # the shared workflow checks the rest (SemVer)
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
  checks:
    uses: ./.github/workflows/ci.yml
  release:
    needs: checks
    uses: zeabur-oss/.github/.github/workflows/release.yml@<sha> # main
    permissions:
      contents: write
      packages: write
      id-token: write
      attestations: write
    with:
      name: Zeabur Example
      version: ${{ inputs.version }}
```

`.github/workflows/changeset.yml`:

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
        description: "Version instead of the one the change files give (v0.2.0); empty for theirs"
        default: ""
      prerelease:
        description: "Pre-release label (rc, beta) for vX.Y.Z-<label>.N; empty for a normal release"
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
      version: ${{ inputs.version }}
      prerelease: ${{ inputs.prerelease }}
    secrets:
      private-key: ${{ secrets.ZEABUR_OSS_BOT_PRIVATE_KEY }}
```

`.github/workflows/skills.yml`, Monday morning in Asia/Taipei like Renovate's lock file maintenance:

```yaml
name: Skills
on:
  schedule:
    - cron: "0 17 * * 0"
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

1. Every pull request that users notice adds a change file (`knope document-change`).
2. Run the repository's Release pull request workflow, with `prerelease: rc` for a release
   candidate. Check the pull request it opens; edit `CHANGELOG.md` on its branch if needed.
3. Optionally rehearse: run the repository's Release workflow by hand with the version.
4. Merge the release pull request. zeabur-oss-bot tags the merge commit, and the tag's workflow
   leaves a draft release.
5. Check the draft and publish it. Releases are immutable once published.

A tag is a [SemVer 2.0.0](https://semver.org/spec/v2.0.0.html) version with a `v` and no build
metadata. `vX.Y.Z`, `v0.*` included, is a normal release. A tag with a pre-release part
(`v0.1.2-rc.0`) is a GitHub pre-release, never marked latest. knope gives a pre-release its own
`CHANGELOG.md` section with every change since the last normal release and keeps the change files,
so the normal release lists them all again. Before 1.0, knope makes a `major` change a minor
release and a `minor` change a patch.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md): signed-off commits ([DCO](DCO)) and Conventional Commits.
