---
default: major
---

# A release is release-build.yml and release.yml

`release.yml` is split in two. `release-build.yml` checks the version and builds, smokes and signs every platform; it takes `version`, `platforms`, `runner`, `signing-environment`, `telemetry-key` and the `APPLE_*` secrets, and outputs `version` and `platforms`. `release.yml` publishes, attests, finishes, adds the assets to the draft and verifies it; it takes the two outputs, `finish`, `finish-platforms`, `runner` and `telemetry-key`, and no longer runs on a rehearsal by itself.

A caller's Release workflow gets two jobs: `build` calls `release-build.yml`, and `release`, with `if: github.event_name == 'push'` and `needs: build`, calls `release.yml` with the outputs of `build`. The `signing-environment` and the `APPLE_*` secrets move to the `build` job, whose permissions are `contents: read`.
