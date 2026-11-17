---
default: minor
---

# The tag job owns the release's title and notes

`release-pr.yml` takes `name` (the product, required) and `docs`, and its tag job writes the draft release's title and notes with them. `release.yml` no longer takes `name` and `docs`; it only adds the assets to the draft. A caller moves `name` and `docs` from its Release workflow to its Release pull request workflow.
