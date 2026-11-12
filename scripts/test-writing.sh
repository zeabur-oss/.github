#!/usr/bin/env bash
# Runs the zeabur-writing Vale style on its fixtures. Each file under tests/zeabur-writing/hit/
# names the one rule it must trigger on its first line (`expect: ZeaburZh.Pad`); no other rule
# may fire there. Files under clean/ must raise nothing. Every rule needs a hit fixture.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
styles=skills/zeabur-writing/vale
fixtures=tests/zeabur-writing
status=0

fail() {
  echo "FAIL $1" >&2
  status=1
}

# The rules that fire on one file. Vale exits 1 when an error-level rule fires, 2 when it breaks.
checks() {
  local out rc=0
  out=$(vale --config .vale.ini --output=JSON "$1") || rc=$?
  ((rc < 2)) || { echo "vale broke on $1: $out" >&2; exit 2; }
  jq -r '[.[][] | .Check] | unique | .[]' <<<"$out"
}

covered=()
while IFS= read -r file; do
  want=$(head -n 1 "$file" | sed -n 's/.*expect: \([A-Za-z.]*\).*/\1/p')
  [[ -n "$want" ]] || { fail "$file: no expect: line"; continue; }
  covered+=("$want")
  got=$(checks "$file")
  grep -qx -- "$want" <<<"$got" || fail "$file: $want did not fire"
  others=$(grep -vx -- "$want" <<<"$got" || true)
  [[ -z "$others" ]] || fail "$file: other rules fired: $(tr '\n' ' ' <<<"$others")"
done < <(find "$fixtures/hit" -type f | sort)

while IFS= read -r file; do
  got=$(checks "$file")
  [[ -z "$got" ]] || fail "$file: expected no alerts, got: $(tr '\n' ' ' <<<"$got")"
done < <(find "$fixtures/clean" -type f | sort)

while IFS= read -r rule; do
  printf '%s\n' "${covered[@]}" | grep -qx -- "$rule" || fail "$rule has no hit fixture"
done < <(find "$styles" -path "$styles/config" -prune -o -name '*.yml' -print |
  sed -E "s|^$styles/([^/]+)/([^/]+)\.yml$|\1.\2|" | sort)

if ((status == 0)); then echo "zeabur-writing fixtures pass"; fi
exit "$status"
