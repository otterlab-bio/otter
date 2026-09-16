#!/usr/bin/env bash

set -euo pipefail

BIN_DIR="${1:-$HOME/.cargo/bin}"

# Each entry: "binary_name|version_prefix"
# binary_name: file names to look for (binary, binary-linux-amd64, binary-linux-amd64-static)
# version_prefix: the name the binary reports in --version output
required_bins=(
  "enva|enva"
  "fastqcx|fastqcx"
  "xenofilx|xenofilx"
  "pairbam|pairbam"
  "seq2mat|seq2mat"
  "methx|methx"
  "qctb|qctb"
  "matsrun|matsrun"
)
version_re='([0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?|[0-9]{4}\.[0-9]{2}\.[0-9]{2}\.[0-9]+|daily-[0-9]{8})'

missing_bins=()
invalid_versions=()

for entry in "${required_bins[@]}"; do
  binary="${entry%%|*}"
  version_prefix="${entry##*|}"
  path="${BIN_DIR}/${binary}"
  if [ ! -x "${path}" ] && [ -x "${BIN_DIR}/${binary}-linux-amd64" ]; then
    path="${BIN_DIR}/${binary}-linux-amd64"
  fi
  if [ ! -x "${path}" ] && [ -x "${BIN_DIR}/${binary}-linux-amd64-static" ]; then
    path="${BIN_DIR}/${binary}-linux-amd64-static"
  fi
  if [ ! -x "${path}" ]; then
    echo "✗ ${binary}: missing executable at ${path}"
    missing_bins+=("${binary}")
    continue
  fi

  output="$("${path}" --version 2>&1 | head -1 || true)"
  expected_re="^${version_prefix} ${version_re}$"
  if printf '%s\n' "${output}" | grep -Eq "${expected_re}"; then
    echo "✓ ${binary}: ${output}"
  else
    echo "✗ ${binary}: invalid --version output: ${output}"
    invalid_versions+=("${binary}")
  fi
done

if [ "${#missing_bins[@]}" -gt 0 ] || [ "${#invalid_versions[@]}" -gt 0 ]; then
  [ "${#missing_bins[@]}" -gt 0 ] && echo "Missing binaries: ${missing_bins[*]}"
  [ "${#invalid_versions[@]}" -gt 0 ] && echo "Invalid version outputs: ${invalid_versions[*]}"
  exit 1
fi

echo "All tool versions are valid."
