#!/bin/bash
set -e

VERSION=${1:-"dev"}
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS="-s -w -X github.com/otterlab-bio/otter/cmd.buildVersion=${VERSION}"
LDFLAGS="$LDFLAGS -X github.com/otterlab-bio/otter/cmd.buildCommit=${COMMIT}"
LDFLAGS="$LDFLAGS -X github.com/otterlab-bio/otter/cmd.buildDate=${DATE}"

mkdir -p dist

# 构建函数
build() {
    local goos=$1
    local goarch=$2
    local ext=""

    [ "$goos" = "windows" ] && ext=".exe"

    local output="dist/otter-${VERSION}-${goos}-${goarch}${ext}"
    echo "Building for ${goos}/${goarch}..."

    CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build \
        -trimpath \
        -ldflags="$LDFLAGS" \
        -o "$output" \
        .

    # 生成校验和
    if [ "$goos" = "linux" ]; then
        sha256sum "$output" >> dist/checksums.txt
    fi
}

# 构建所有平台
build linux amd64

echo "Build complete! Output in dist/"
ls -lh dist/
