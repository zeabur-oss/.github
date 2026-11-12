# AGENTS.md

The shared CI, release and Update workflows of every zeabur-oss repository ([README.md](README.md)), and the agent skills they share. The Update workflow copies each skill into a repository's `.claude/skills/` as it is.

## Layout

```
.github/workflows/         the shared workflows, and this repository's own Lint
actions/                   the CI setup and bot pull request actions
release/, checks/          @zeabur-oss/release
skills/<name>/SKILL.md     a shared skill: frontmatter, then instructions
skills/<name>/*.md         references SKILL.md links to, loaded when needed
skills/<name>/scripts/     scripts a skill tells the reader to run
skills/zeabur-writing/vale the Vale style, its TypeScript view and the sample vale.ini
tests/zeabur-writing/      Vale fixtures: hit/ must trigger the rule named on line 1, clean/ nothing
scripts/ci/<job>.sh        the skills, lint and prose jobs of Lint
```

## Writing a skill

- The directory name is kebab-case and equals `name` in the frontmatter. `description` says what the skill does and when to use it, with the words a request would contain.
- A shared skill holds what is the same in every repository. Commands, paths, crate names and term lists stay in that repository's own skill, which points here.
- Merge facts from the repositories; do not invent a rule none of them has.
- Every `.md` under `skills/` starts (after any frontmatter) with `<!-- Edit this in zeabur-oss/.github (<its path>); ... -->`, so whoever opens a copy in another repository edits it here; `scripts/ci/skills.sh` checks it.
- Write by `zeabur-writing`: plain words, no filler, no caveat that does not change what the reader does.
- `no-ai-flavour` and `story-writing` are kept as their author wrote them; change them only when asked.

## Changing the Vale style

- A new rule gets a fixture in `tests/zeabur-writing/hit/` that triggers it and nothing else. A false positive you fix goes into a file under `clean/`.
- A file's locale comes from its path (`zh-TW`, `zh-CN`, `i18n/en.ts`), as in `vale/vale.ini`. `.vale.ini` at the root is the same file with this repository's `StylesPath`; keep them equal.

## Checks

Run every job before you commit, in the dev shell (`nix develop`):

```sh
scripts/ci/skills.sh   # frontmatter of each SKILL.md, the edit-here note
scripts/ci/lint.sh     # actionlint, shellcheck, nixfmt
scripts/ci/prose.sh    # Vale fixtures, then Vale on this repository's own text
```

Commits follow [CONTRIBUTING.md](CONTRIBUTING.md).
