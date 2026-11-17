---
default: minor
---

# zeabur-oss-release pushes, pins and verifies as commands

`zeabur-oss-release` gains `absent`, `push-image`, `push-chart`, `pin-digest`, `verify-image` and `verify-chart`, so a repository's own Release workflow runs the steps as shell steps and needs no `scripts/release.ts` stage. A workflow that sets neither `RELEASE_CERT_IDENTITY_REGEXP` nor `RELEASE_SIGNER_WORKFLOW` is verified against `.github/workflows/release.yml` of its own repository at the tag, the signer of a repository's own release.
