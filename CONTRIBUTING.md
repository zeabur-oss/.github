# Contributing

## Developer Certificate of Origin

This project uses the [Developer Certificate of Origin 1.1](DCO). Sign off every commit to
certify that you wrote it or otherwise have the right to submit it under the project licence:

```sh
git commit -s
```

CI rejects pull requests whose commits lack a `Signed-off-by` line matching the author. Commit
messages follow [Conventional Commits](https://www.conventionalcommits.org/) (`ci:`, `docs:`,
`chore:`, `fix:`, `feat:`). Pull requests are squash-merged, so the pull request title is the commit
message on `main` and follows the same form.

## Changing a shared workflow

Every zeabur-oss repository calls these workflows pinned by commit, so a change reaches a repository
only when its pin moves:

1. Keep inputs backward compatible: a new input has a default that keeps today's behaviour.
2. Update the contract in [README.md](README.md) together with the workflow.
3. After merging, each caller's Update workflow moves its pins on Monday, or when run by hand;
   review its pull request and let each repository's CI pass before relying on the change.

`Lint` runs jactionlint (with shellcheck) on every workflow here, and the tests of
[`zeabur-oss-release`](release/README.md); run them locally with `nix develop`, then `jactionlint`,
`bun install`, `bun fmt --check .`, `bun check release/ checks/` and `bun test release/` (with Docker). Without the dev shell, jactionlint alone is
`nix shell nixpkgs#jactionlint nixpkgs#shellcheck --command jactionlint`.

## Releases

This repository has versions too, though nothing is built: a version is a tag on a commit of `main`.
Callers still pin by commit, and the tag names it in the Update pull request and the release notes.

A pull request that changes what a caller sees adds a change file. `nix develop -c knope document-change`
writes it to `.changeset/`. Choose the level by the effect on callers: `major` when they have to
change something (before 1.0 that makes the next minor version), `minor` for something new, `patch`
for a fix. The Changeset check enforces this; a change that only touches this repository's own CI
takes the label `changelog: none`. Say in the file what a caller changes when its pin moves.

To release, run the Release pull request workflow, merge the pull request it opens, and
zeabur-oss-bot tags the merge commit and leaves a draft release with the `CHANGELOG.md` section.
Publish the draft when it reads right.
