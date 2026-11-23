#!/usr/bin/env bash
# Exercise the shell entry point without GitHub or any installation side effects.
set -euo pipefail
root="$(cd "$(dirname "$0")/../.." && pwd)"
source "$root/scripts/release-platform.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir "$tmp/mock-bin"
cat > "$tmp/mock-bin/uname" <<'MOCK'
#!/bin/sh
case "$1" in
  -s) echo "$MOCK_OS" ;;
  -m) echo "$MOCK_ARCH" ;;
  *) exec /usr/bin/uname "$@" ;;
esac
MOCK
cat > "$tmp/mock-bin/curl" <<'MOCK'
#!/bin/sh
for arg do
  case "$arg" in
    'https://api.github.com/repos/example/otter/releases?per_page=20')
      printf '%s\n' '[{' '  "tag_name": "v9.0.0"' '}, {' '  "tag_name": "v8.0.0"' '}]'
      exit 0 ;;
    'https://api.github.com/repos/example/otter/releases/tags/v9.0.0')
      printf '%s\n' '{"assets": []}'
      exit 0 ;;
    'https://api.github.com/repos/example/otter/releases/tags/v8.0.0')
      cat "$MOCK_ASSETS"
      exit 0 ;;
  esac
done
printf 'unexpected curl invocation\n' >&2
printf 'unexpected\n' >> "$MOCK_UNEXPECTED"
exit 1
MOCK
chmod +x "$tmp/mock-bin/"*
export PATH="$tmp/mock-bin:$PATH"
export GITHUB_TOKEN='' GH_TOKEN='' GITHUB_PAT='' GITHUB_PROXY_PREFIX='' OTTER_GITHUB_PROXY=''
export MOCK_UNEXPECTED="$tmp/unexpected" MOCK_ASSETS="$tmp/assets.json"

install_plan() {
  /bin/bash "$root/scripts/install.sh" --non-interactive --skip-envs --dry-run --version "$1" \
    --releases-repo example/otter --fallback-releases-repo example/otter --install-dir "$tmp/not-created"
}

for entry in 'Linux:x86_64:linux-amd64' 'Linux:aarch64:linux-arm64' 'Darwin:x86_64:darwin-amd64' 'Darwin:arm64:darwin-arm64'; do
  IFS=: read -r os arch platform <<< "$entry"
  export MOCK_OS="$os" MOCK_ARCH="$arch"
  plan="$(install_plan v1.2.3)"
  while IFS= read -r asset; do
    [[ "$asset" == otter-install-* ]] || grep -Fq "/v1.2.3/$asset\"" <<< "$plan"
  done < <(release_assets "$platform")
done

export MOCK_OS=Darwin MOCK_ARCH=arm64
python3 -c 'import json,sys; print(json.dumps({"assets":[{"name": a.strip()} for a in sys.stdin]}, indent=2))' \
  < <(release_assets darwin-arm64) > "$MOCK_ASSETS"
plan="$(install_plan latest)"
grep -Fq '/v8.0.0/otter-darwin-arm64"' <<< "$plan"
if MOCK_OS=Windows install_plan v1.2.3 > "$tmp/unsupported.log" 2>&1; then
  echo 'Installer accepted an unsupported platform' >&2
  exit 1
fi
grep -Fq 'Unsupported operating system' "$tmp/unsupported.log"
[[ ! -e "$tmp/not-created" && ! -e "$MOCK_UNEXPECTED" ]]
echo 'Shell installer plans passed for all four platforms; latest selected the complete release.'
