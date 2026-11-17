---
default: patch
---

# The release pull request is formatted

After knope writes `CHANGELOG.md` and the versions, the Release pull request workflow runs `bun fmt --changed=HEAD` in the caller's dev shell, so the release pull request passes the caller's format check.
