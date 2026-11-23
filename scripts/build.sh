#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
source "$ROOT/scripts/release-platform.sh"
cd "$ROOT"
VERSION="${1:-dev}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-s -w -X github.com/otterlab-bio/otter/cmd.buildVersion=$VERSION"
LDFLAGS="$LDFLAGS -X github.com/otterlab-bio/otter/cmd.buildCommit=$COMMIT -X github.com/otterlab-bio/otter/cmd.buildDate=$DATE"
mkdir -p dist
outputs=()
for platform in "${release_platforms[@]}"; do
    output="otter-$VERSION-$platform"
    echo "Building $platform..."
    CGO_ENABLED=0 GOOS="${platform%-*}" GOARCH="${platform#*-}" go build \
        -mod=readonly -trimpath -buildvcs=false -ldflags="$LDFLAGS" -o "dist/$output" .
    outputs+=("$output")
done
(cd dist && if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${outputs[@]}"
else
    shasum -a 256 "${outputs[@]}"
fi > checksums.txt)
echo 'Build complete! Output in dist/'
