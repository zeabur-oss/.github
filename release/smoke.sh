#!/usr/bin/env bash
# Pushes two images that `docker image save` wrote with zeabur-oss-release, then reads them back
# from the registry: the index lists exactly the recorded digests, 0.0.1 is the only tag, and a
# second push of 0.0.1 is refused. Needs Docker, a registry without the Referrers API on
# localhost:5000 (as GHCR answers) and ./result from `nix build .#zeabur-oss-release`.
set -euo pipefail

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
for arch in amd64 arm64; do
  printf 'FROM scratch\nCOPY flake.nix /\n' |
    docker build --platform "linux/$arch" --tag "smoke:$arch" --file - .
  docker image save --output "$tmp/$arch.tar" "smoke:$arch"
done
./result/bin/zeabur-oss-release push-image --insecure --no-sign \
  --repository localhost:5000/zeabur-oss/smoke --tag 0.0.1 \
  --digests "$tmp/digests.txt" --subjects "$tmp/subjects.json" \
  "linux/amd64=$tmp/amd64.tar" "linux/arm64=$tmp/arm64.tar"
cat "$tmp/digests.txt" "$tmp/subjects.json"
index=$(awk '$1 == "index" { print $2 }' "$tmp/digests.txt")
# The index lists the two images docker saved, by the digests recorded.
curl -fsS -H 'Accept: application/vnd.docker.distribution.manifest.list.v2+json' \
  "http://localhost:5000/v2/zeabur-oss/smoke/manifests/${index#*@}" |
  jq -r '.manifests[] | "\(.platform.os)/\(.platform.architecture) \(.digest)"' | sort >"$tmp/listed"
awk '$1 != "index" { sub(/.*@/, "", $2); print $1, $2 }' "$tmp/digests.txt" | sort |
  diff - "$tmp/listed"
# The tag is the index; no other tag was made.
[[ "$(curl -fsS http://localhost:5000/v2/zeabur-oss/smoke/tags/list | jq -c .tags)" == '["0.0.1"]' ]]
# A published version is never replaced.
if ./result/bin/zeabur-oss-release push-image --insecure --no-sign \
  --repository localhost:5000/zeabur-oss/smoke --tag 0.0.1 "linux/amd64=$tmp/amd64.tar"; then
  echo "::error::pushed over 0.0.1"; exit 1
fi
