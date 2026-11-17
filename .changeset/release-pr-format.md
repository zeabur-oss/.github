---
default: patch
---

# knope formats the release it writes

A caller's `knope.toml` formats what `PrepareRelease` wrote: add a `Command` step after it, such as `bun fmt CHANGELOG.md package.json` (list the other manifests it changes). Without it the release pull request can fail the caller's format check.
