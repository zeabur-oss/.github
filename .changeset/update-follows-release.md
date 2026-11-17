---
default: minor
---

# Update moves the pins to the newest release

`update.yml` moved every pin of zeabur-oss/.github to the head of `main`. It now moves them to the newest release tag, pre-releases skipped: it writes the tag into the flake input's URL in `flake.nix` (`github:zeabur-oss/.github/v0.2.1`), updates `flake.lock`, and gives each `uses:` the tag's commit with the tag as its comment. The pull request title names the tag.
