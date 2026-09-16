#!/usr/bin/env bash

set -euo pipefail

DIST_DIR="${1:-dist}"

required=(
  otter-linux-amd64-static
  otter-install-linux-amd64-static
  craftmake-linux-amd64-static
  enva-linux-amd64-static
  xenofilx-linux-amd64-static
  pairbam-linux-amd64-static
  seq2mat-linux-amd64-static
  matsrun-linux-amd64-static
  qctb-linux-amd64-static
  fastqcx-linux-amd64-static
  methx-linux-amd64-static
)

if [[ "${DIST_DIR}" == "--print-required" ]]; then
  printf '%s\n' "${required[@]}"
  exit 0
fi

missing=()
not_exec=()

for artifact in "${required[@]}"; do
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
done

if [ ${#missing[@]} -gt 0 ] || [ ${#not_exec[@]} -gt 0 ]; then
  [ ${#missing[@]} -gt 0 ] && echo "Missing artifacts: ${missing[*]}"
  [ ${#not_exec[@]} -gt 0 ] && echo "Non-executable artifacts: ${not_exec[*]}"
  exit 1
fi

echo "All required release artifacts are present and executable."
