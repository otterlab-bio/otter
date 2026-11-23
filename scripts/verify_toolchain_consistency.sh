#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RULES_DIR="${ROOT_DIR}/inst/rules"
INSTALL_SCRIPT="${ROOT_DIR}/scripts/install.sh"
BUILD_SCRIPT="${ROOT_DIR}/scripts/build-all-submodules.sh"

KNOWN_EXTERNAL_CMDS=(
  enva
  fastqcx
  xenofilx
  pairbam
  seq2mat
  methx
  qctb
  matsrun
)

if [ ! -d "${RULES_DIR}" ]; then
  echo "Rules directory not found: ${RULES_DIR}"
  exit 1
fi

required_cmds=()
for cmd in "${KNOWN_EXTERNAL_CMDS[@]}"; do
  if grep -Rqs -E "(^|[^A-Za-z0-9_-])${cmd}([^A-Za-z0-9_-]|$)" "${RULES_DIR}"; then
    required_cmds+=("${cmd}")
  fi
done

install_bins=()
while IFS= read -r bin; do install_bins+=("$bin"); done < <(
  awk '/^TOOLS=\(/,/^\)/ { print }' "${INSTALL_SCRIPT}" \
    | grep -Eo '"[^"]+"' \
    | tr -d '"'
)

build_bins=()
while IFS= read -r bin; do build_bins+=("$bin"); done < <(
  grep -E "required_bins=\(" "${BUILD_SCRIPT}" \
    | sed -E 's/.*\((.*)\).*/\1/' \
    | tr ' ' '\n' \
    | sed '/^$/d'
)

missing_install=()
missing_build=()

contains() {
  local needle="$1"
  shift
  local item
  for item in "$@"; do
    if [ "${item}" = "${needle}" ]; then
      return 0
    fi
  done
  return 1
}

for cmd in "${required_cmds[@]}"; do
  if ! contains "${cmd}" "${install_bins[@]}"; then
    missing_install+=("${cmd}")
  fi
  if ! contains "${cmd}" "${build_bins[@]}"; then
    missing_build+=("${cmd}")
  fi
done

echo "Required by rules: ${required_cmds[*]:-none}"
echo "Install script bins: ${install_bins[*]:-none}"
echo "Build script bins: ${build_bins[*]:-none}"

if [ "${#missing_install[@]}" -gt 0 ]; then
  echo "Missing in scripts/install.sh: ${missing_install[*]}"
fi
if [ "${#missing_build[@]}" -gt 0 ]; then
  echo "Missing in scripts/build-all-submodules.sh: ${missing_build[*]}"
fi

if [ "${#missing_install[@]}" -gt 0 ] || [ "${#missing_build[@]}" -gt 0 ]; then
  exit 1
fi

# The release tag drives -ldflags, but these fallbacks are what a plain
# `go build` or `make` reports, so they must agree with each other.
root_version="$(sed -n 's/^[[:space:]]*buildVersion = "\([^"]*\)".*/\1/p' "${ROOT_DIR}/cmd/root.go")"
installer_version="$(sed -n 's/^[[:space:]]*version = "\([^"]*\)".*/\1/p' "${ROOT_DIR}/installer/main.go")"
makefile_version="$(sed -n 's/^VERSION ?= //p' "${ROOT_DIR}/installer/Makefile")"
if [ -z "${root_version}" ] || [ "${root_version}" != "${installer_version}" ] || [ "${root_version}" != "${makefile_version}" ]; then
  echo "Version drift: cmd/root.go=${root_version:-missing} installer/main.go=${installer_version:-missing} installer/Makefile=${makefile_version:-missing}" >&2
  exit 1
fi
echo "Version fallbacks agree: ${root_version}"

echo "Toolchain consistency check passed."
