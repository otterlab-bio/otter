#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/../.." && pwd)"
source "$root/scripts/release-platform.sh"
dist="${1:?expected release directory}"
platform="${2:?expected platform}"
bash "$root/scripts/verify_release_artifacts.sh" "$dist" "$platform"
assets="$(release_assets "$platform")"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
while IFS= read -r asset; do
  binary="$dist/$asset"
  description="$(file -b "$binary")"
  case "$platform:$description" in
    linux-amd64:ELF*'x86-64'*|linux-arm64:ELF*'aarch64'*|darwin-amd64:'Mach-O 64-bit executable x86_64'*|darwin-arm64:'Mach-O 64-bit executable arm64'*) ;;
    *) printf '%s: unexpected binary format or architecture: %s\n' "$asset" "$description" >&2; exit 1 ;;
  esac
  if [[ "$platform" == darwin-* ]]; then
    dependencies="$(otool -L "$binary" | sed -n '2,$p')"
    # A release must not rely on libraries installed only on the build machine.
    if grep -Ev '^[[:space:]]+(/usr/lib/|/System/Library/)' <<< "$dependencies"; then
      echo "$asset has non-system dynamic dependencies" >&2
      exit 1
    fi
  fi
  printf '%s: ' "$asset"
  if [[ "$asset" == otter-install-* ]]; then
    output="$("$binary" --dry-run --non-interactive --skip-envs --version v0.0.0 --install-dir "$tmp/bin")"
    while IFS= read -r expected; do
      [[ "$expected" == otter-install-* ]] || grep -Fq "/v0.0.0/$expected ->" <<< "$output"
    done <<< "$assets"
    [[ ! -e "$tmp/bin" ]]
    printf 'offline install plan passed\n'
  else
    "$binary" --version
  fi
done <<< "$assets"

output="$(/bin/bash "$root/scripts/install.sh" --dry-run --non-interactive --skip-envs --version v0.0.0 --install-dir "$tmp/bin")"
while IFS= read -r asset; do
  [[ "$asset" == otter-install-* ]] || grep -Fq "/v0.0.0/$asset\"" <<< "$output"
done <<< "$assets"
[[ ! -e "$tmp/bin" ]]
