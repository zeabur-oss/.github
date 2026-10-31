# zeabur-oss-release

The steps of a release that every zeabur-oss repository does the same way: push images and charts
by digest, sign, write and sign `SHA256SUMS`, copy a `CHANGELOG.md` section into the release notes,
and verify a release as a user would. It is one TypeScript file run by Bun, and each step is a
call to crane, helm, cosign or gh. A repository's `scripts/release.ts` keeps what is its own (what
to build, how to smoke it, which assets exist) and imports this for the rest. The shared
`release.yml` runs `notes`, `checksums` and `verify-assets` itself, as the `zeabur-oss-release`
command of the same package.

## Getting it

A git dependency in the caller's `package.json`, pinned by its `bun.lock`. crane, cosign, gh and
helm come from the caller's dev shell.

```json
{
  "private": true,
  "dependencies": { "@zeabur-oss/release": "github:zeabur-oss/.github#<commit>" }
}
```

```ts
import { absent, pushImage, verifyImage } from "@zeabur-oss/release";

const index = await pushImage("published", "builder", "ghcr.io/zeabur-oss/builder:0.1.1", {
  "linux/amd64": "candidate-linux-amd64/builder.tar",
  "linux/arm64": "candidate-linux-arm64/builder.tar",
});
```

Each function throws `Failure` when the release must stop; push* return what they pushed and
verify* what they verified.

## Commands

The same steps as a command, `node_modules/.bin/zeabur-oss-release` after `bun install`:

```sh
# publish: refuse every tag first, then each image by digest and its index tagged X.Y.Z, then
# the chart; each digest is signed and recorded in published/digests.txt and subjects.json
zeabur-oss-release absent ghcr.io/zeabur-oss/builder:0.1.1 ghcr.io/zeabur-oss/charts/zeabur-builder:0.1.1
zeabur-oss-release push-image published builder ghcr.io/zeabur-oss/builder:0.1.1 \
  linux/amd64=candidate-linux-amd64/builder.tar linux/arm64=candidate-linux-arm64/builder.tar
zeabur-oss-release push-chart published oci://ghcr.io/zeabur-oss/charts zeabur-builder-0.1.1.tgz

# the release workflow: the signed checksums of every asset, and the notes
zeabur-oss-release checksums release/assets
zeabur-oss-release notes CHANGELOG.md v0.1.1 https://oss.zeabur.com/docs/builder/ >release/notes.md

# verify: what a user checks, with no registry login
zeabur-oss-release verify-assets downloaded
zeabur-oss-release verify-image downloaded/digests.txt builder linux/amd64 linux/arm64
zeabur-oss-release verify-chart downloaded/digests.txt downloaded/zeabur-builder-0.1.1.tgz
```

`verify-*` read the signer from `RELEASE_CERT_IDENTITY_REGEXP`, `RELEASE_SIGNER_WORKFLOW`,
`RELEASE_SOURCE_REPOSITORY` and `RELEASE_SOURCE_REF`, which the release workflow sets.

## What it holds to

- **A digest names everything; the version tag comes last.** Platform images are pushed by
  digest, with no temporary tag. The index of exactly those digests is pushed last, under
  `X.Y.Z` (the version without `v`, as every image and chart is tagged). If anything fails
  before it, the tag does not exist and the run can be repeated.
- **Refuse before the first write; never replace.** An existing tag or an image for the wrong
  platform stops the command before it pushes anything. Only a registry's answer that the tag
  or the repository does not exist counts as absent.
- **The registry holds what was tested.** `push-chart` checks that the chart the registry
  serves is the packaged file; `verify-image` checks that the index lists exactly the recorded
  platform images.
- **One signer identity.** Signatures name this repository's `release.yml`, run for the
  released repository at its tag.
- **The notes are the changelog section, as written.** `CHANGELOG.md` uses the headings knope
  writes (`## 0.1.1 (2026-10-28)`) and keeps each paragraph and list item on one line, since a
  release body shows every line break. A pre-release has its own section. The body ends with one
  link to the product's documentation, which covers installing, upgrading and verifying.
- **Untagged versions are part of a release.** The platform images an index lists, and the
  signatures cosign stores beside them, are untagged in GHCR. Never delete untagged versions
  of a release package.

## digests.txt

One line per image or chart: a kind, two spaces, `repository@sha256:...`. The kind is an optional
label (the image's name, when a release has several), then the platform, `index` or `chart`:

```text
builder linux/amd64  ghcr.io/zeabur-oss/builder@sha256:...
builder index  ghcr.io/zeabur-oss/builder@sha256:...
chart  ghcr.io/zeabur-oss/charts/zeabur-builder@sha256:...
```

## Tests

`bun test release/` in the dev shell, with Docker. It pushes images that `docker image save`
wrote, and a chart, to a registry in memory (`crane registry serve`), and reads them back. cosign
and gh are replaced by stand-ins that log their arguments.
