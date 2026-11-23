#!/usr/bin/env bash
# Shared by the release builder and artifact verifiers. Assets use Go's OS/arch names.
release_platforms=(linux-amd64 linux-arm64 darwin-amd64 darwin-arm64)
release_tools=(otter otter-install craftmake enva xenofilx pairbam seq2mat matsrun qctb fastqcx methx)

release_host_platform() {
  local os arch
  case "$(uname -s)" in Linux) os=linux ;; Darwin) os=darwin ;; *) return 1 ;; esac
  case "$(uname -m)" in x86_64|amd64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; *) return 1 ;; esac
  printf '%s-%s\n' "$os" "$arch"
}

release_assets() {
  local platform="$1" tool suffix
  case "$platform" in
    linux-amd64|linux-arm64) suffix=-static ;;
    darwin-amd64|darwin-arm64) suffix='' ;;
    *) echo "Unsupported release platform: $platform" >&2; return 1 ;;
  esac
  for tool in "${release_tools[@]}"; do
    printf '%s-%s%s\n' "$tool" "$platform" "$suffix"
  done
}
