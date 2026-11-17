---
default: minor
---

# ci-setup caches Cargo's build output with kache

With `rust-cache: true`, `actions/ci-setup` installs kache instead of using Swatinem/rust-cache. A caller runs Cargo as `kache cargo -- <command>` in its scripts; a bare `cargo` still builds, but nothing is cached. Only runs of `main` save the cache. The Nix store cache is hestia instead of magic-nix-cache.
