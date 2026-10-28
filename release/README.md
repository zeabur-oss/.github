# zeabur-oss-release

The steps of a release that every zeabur-oss repository does the same way: push images and charts
by digest, sign, write and sign `SHA256SUMS`, verify a release as a user would, and turn a
`CHANGELOG.md` section into release notes. A repository's `scripts/release.sh` keeps what is its
own (what to build, how to smoke it, which assets exist) and calls this command for the rest.

`zeabur-oss-release --help` lists the commands and their arguments.

## Getting it

From this repository's flake, pinned by the caller's `flake.lock` like any other input, so a
release, a rehearsal and a run on a laptop use the same build:

```nix
inputs.oss-release.url = "github:zeabur-oss/.github";
# in the dev shell:
packages = [ inputs.oss-release.packages.${system}.zeabur-oss-release ];
```

The package carries `cosign`, `gh` and `helm` on its `PATH`: it signs and verifies through the
same commands a user runs, and pushes charts with `helm push`.

## A release with it

```sh
# publish: each image by digest, then its index tagged X.Y.Z; every digest signed
zeabur-oss-release push-image --repository ghcr.io/zeabur-oss/zeabur --tag 0.1.1 \
  --digests published/digests.txt --subjects published/subjects.json \
  linux/amd64=artifacts/candidate-linux-amd64/image.tar linux/arm64=artifacts/candidate-linux-arm64/image.tar
zeabur-oss-release push-chart --repository ghcr.io/zeabur-oss/charts \
  --digests published/digests.txt --subjects published/subjects.json zeabur-0.1.1.tgz

# assets: the checksums of every asset, signed; the notes
zeabur-oss-release checksums release/assets
zeabur-oss-release notes CHANGELOG.md v0.1.1 >release/notes.md

# verify: what a user checks, with no registry login
zeabur-oss-release verify-assets --repository zeabur-oss/zeabur --version v0.1.1 downloaded \
  digests.txt zeabur-0.1.1.tgz ...
zeabur-oss-release verify-image --repository zeabur-oss/zeabur --version v0.1.1 \
  --digests downloaded/digests.txt linux/amd64 linux/arm64
zeabur-oss-release verify-chart --repository zeabur-oss/zeabur --version v0.1.1 \
  --digests downloaded/digests.txt downloaded/zeabur-0.1.1.tgz
```

## What it holds to

- **A digest names everything; the version tag comes last.** Platform images are pushed by
  digest, with no temporary tag. The index of exactly those digests is pushed last, under
  `X.Y.Z` (the version without `v`, as every image and chart is tagged). If anything fails
  before it, the tag does not exist and the run can be repeated.
- **Refuse before the first write; never replace.** An existing tag, an image for the wrong
  platform or two images for one platform stop the command before it pushes anything.
- **The registry holds what was tested.** `push-chart` checks that the chart the registry
  serves is the packaged file; `verify-image` checks that the index lists exactly the recorded
  platform images.
- **One signer identity.** Signatures name this repository's `release.yml`, run for the
  released repository at its tag. `verify-*` check all three, with the defaults in one place.
- **Release notes read as written.** A release body shows every line break, so `notes` joins the
  wrapped lines of paragraphs and list items and leaves code blocks, tables, quotes, headings and
  hard breaks as they are.
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

`go test ./...` in the dev shell runs everything against an in-process registry, with cosign, gh
and helm replaced by a recorder. The `release-cli` job of `Lint` also pushes images that
`docker image save` wrote to a `registry:2` service and reads them back.
