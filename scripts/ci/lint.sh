#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."

actionlint
shellcheck scripts/ci/*.sh
nixfmt --check flake.nix
