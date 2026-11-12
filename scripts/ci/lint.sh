#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."

actionlint
shellcheck scripts/*.sh scripts/ci/*.sh skills/*/scripts/*.sh
nixfmt --check flake.nix nix/*.nix
