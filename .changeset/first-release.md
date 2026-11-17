---
default: minor
---

# The shared workflows, actions and release package get versions

Callers still pin by commit; a version names the commit and the release notes list what a caller changes when its pin moves past it. The base shell now has jactionlint instead of actionlint, the shared skills and the Vale style are gone, and `zeabur-oss-release` is called as `bun x zeabur-oss-release`.
