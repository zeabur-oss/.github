#!/usr/bin/env bash
# Each skills/<name>/SKILL.md has a name equal to <name> and a description, and every .md under
# skills/ says it is edited here.
set -euo pipefail
cd "$(dirname "$0")/../.."

status=0
for file in skills/*/SKILL.md; do
  dir=$(basename "$(dirname "$file")")
  name=$(yq --front-matter=extract '.name // ""' "$file")
  description=$(yq --front-matter=extract '.description // ""' "$file")
  [[ "$name" == "$dir" ]] || { echo "$file: name is '$name', not '$dir'" >&2; status=1; }
  [[ -n "$description" ]] || { echo "$file: no description" >&2; status=1; }
done
for dir in skills/*/; do
  [[ -f "$dir/SKILL.md" ]] || { echo "$dir: no SKILL.md" >&2; status=1; }
done
while IFS= read -r -d '' file; do
  grep -qF "<!-- Edit this in zeabur-oss/.github ($file);" "$file" ||
    { echo "$file: no note that it is edited in zeabur-oss/.github" >&2; status=1; }
done < <(find skills -name '*.md' -print0)
exit "$status"
