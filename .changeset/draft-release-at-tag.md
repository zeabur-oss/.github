---
default: minor
---

# The tag job leaves the draft release

The tag job of the Release pull request workflow now creates the draft release, with the `CHANGELOG.md` section as its notes, right after it tags the commit. The Release workflow adds the assets to that draft and sets its title and notes, instead of creating it. A caller changes nothing, but a release made from a tag pushed by hand needs `gh release create --draft` first.
