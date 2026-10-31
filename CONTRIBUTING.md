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
3. After merging, move the pin in each caller (Renovate does it for the `github-actions` manager)
   and let each repository's CI pass before relying on the change.

`Lint` runs actionlint (with shellcheck) on every workflow here, and the tests of
[`zeabur-oss-release`](release/README.md); run them locally with `nix develop`, then `actionlint`,
`bun install`, `bun fmt --check .`, `bun check release/ checks/`, `bun test release/` (with Docker) and `bun test --timeout 60000 checks/`. Without the dev shell, actionlint alone is
`nix shell nixpkgs#actionlint nixpkgs#shellcheck --command actionlint`.
