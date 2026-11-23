#!/usr/bin/env bash

set -euo pipefail

DIST_DIR="${1:-dist}"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$ROOT_DIR/scripts/release-platform.sh"
platform="${2:-linux-amd64}" # Preserve the legacy evidence scripts' default.
if [[ "$platform" == all ]]; then
  names="$(for target in "${release_platforms[@]}"; do release_assets "$target"; done)"
else
  names="$(release_assets "$platform")"
fi
if [[ "$DIST_DIR" == --print-required ]]; then
  printf '%s\n' "$names"
  exit 0
fi

missing=()
not_exec=()

while IFS= read -r artifact; do
  path="${DIST_DIR}/${artifact}"
  if [ ! -f "$path" ]; then
    echo "✗ missing: $artifact"
    missing+=("$artifact")
    continue
  fi
  if [ ! -x "$path" ]; then
    echo "✗ not executable: $artifact"
    not_exec+=("$artifact")
    continue
  fi
  echo "✓ $artifact"
done <<< "$names"

if [ ${#missing[@]} -gt 0 ] || [ ${#not_exec[@]} -gt 0 ]; then
  [ ${#missing[@]} -gt 0 ] && echo "Missing artifacts: ${missing[*]}"
  [ ${#not_exec[@]} -gt 0 ] && echo "Non-executable artifacts: ${not_exec[*]}"
  exit 1
fi

echo "All required release artifacts are present and executable."
