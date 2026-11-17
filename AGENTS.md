# AGENTS.md

The shared CI, release and Update workflows of every zeabur-oss repository ([README.md](README.md)).

## Layout

```
.github/workflows/         the shared workflows, and this repository's own Lint
actions/                   the CI setup and bot pull request actions
release/, checks/          @zeabur-oss/release
scripts/ci/<job>.sh        the lint job of Lint
```

## Checks

Run every job before you commit, in the dev shell (`nix develop`):

```sh
scripts/ci/lint.sh     # actionlint, shellcheck, nixfmt
```

Commits follow [CONTRIBUTING.md](CONTRIBUTING.md).
