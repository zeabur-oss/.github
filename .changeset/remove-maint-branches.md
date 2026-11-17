---
default: major
---

# Maintenance branches are gone

The Release pull request workflow now runs on `main` only, and the Release workflow no longer accepts a tag that is on a `maint/vX.Y` branch instead of `main`. A repository that has a `maint/*` branch must remove `maint/*` from the `branches` of its release-pr.yml before it updates.
