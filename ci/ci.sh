#!/usr/bin/env bash
# One CI job, as zeabur-oss/.github's shared CI workflow runs it and as anyone runs it locally. The
# jobs are the jobs of scripts/ci/jobs.toml; each one is scripts/ci/<job>.sh, run in its Nix dev shell.
# This file is the same in every zeabur-oss repository.
#
#   scripts/ci.sh <job>             # run one job; it enters its dev shell itself
#   scripts/ci.sh --list            # every job name
#
# A job may leave files in $CI_ARTIFACTS; the workflow uploads them as the artifact `ci-<job>`.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
matrix="$root/scripts/ci/matrix.ts"

# scripts/ci/matrix.ts reads scripts/ci/jobs.toml; the dev shell has Bun.
table() {
  if command -v bun >/dev/null; then
    bun "$matrix" "$@"
  else
    nix develop "$root" --command bun "$matrix" "$@"
  fi
}

case "${1:-}" in
  --list)
    table list
    exit 0
    ;;
esac

job="${1:?usage: scripts/ci.sh <job> | --list}"
table get "$job" tier >/dev/null || { echo "error: no job named $job (scripts/ci.sh --list)" >&2; exit 1; }

# A group runs every member, also after one fails, and fails if any did.
members=$(table members "$job")
if [[ -n "$members" ]]; then
  failed=()
  for member in $members; do
    echo "::group::$member"
    "$0" "$member" || failed+=("$member")
    echo "::endgroup::"
  done
  [[ ${#failed[@]} -eq 0 ]] || { echo "::error::failed: ${failed[*]}"; exit 1; }
  exit 0
fi

# The marker tells a run inside the job's dev shell apart from a call outside it.
shell=$(table get "$job" shell)
shell="${shell:-default}"
if [[ "${CI_SHELL:-}" != "$shell" ]]; then
  export CI_SHELL="$shell"
  exec nix develop "$root#$shell" --command "$0" "$@"
fi
exec "$root/scripts/ci/$job.sh"
