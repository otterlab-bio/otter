#!/usr/bin/env bash
# Build the supported release set on a native runner. Rust/CMake use the host SDK/CPU.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
source "$ROOT/scripts/release-platform.sh"
platform="${1:?expected release platform}"
version="${2:?expected release version}"
mkdir -p "${3:-dist}"
dist="$(cd "${3:-dist}" && pwd)"
host="$(release_host_platform)" || { echo 'Unsupported build host' >&2; exit 1; }
if [[ "$host" != "$platform" ]]; then
  echo "Expected a native $platform runner, got $host" >&2
  exit 1
fi
cd "$ROOT"

os="${platform%-*}"
arch="${platform#*-}"
commit="$(git rev-parse --short=7 HEAD)"
build_date="$(git show -s --format=%cI HEAD)"
suffix="$platform"
case "$platform" in
  linux-amd64) rust_target=x86_64-unknown-linux-musl ;;
  linux-arm64) rust_target=aarch64-unknown-linux-musl ;;
  darwin-amd64) rust_target=x86_64-apple-darwin ;;
  darwin-arm64) rust_target=aarch64-apple-darwin ;;
esac
if [[ "$os" == linux ]]; then
  suffix="$platform-static"
  # Native C libraries (including HDF5/zlib) must use musl headers, not glibc's.
  export "CC_${rust_target//-/_}=musl-gcc"
  # HDF5 runs CMake feature probes. The CPU is native even though Rust's
  # host triple uses glibc; don't let the cmake crate mark this a cross build.
  musl_toolchain="$(mktemp)"
  trap 'rm -f "$musl_toolchain"' EXIT
  printf '%s\n' 'set(CMAKE_C_COMPILER musl-gcc)' > "$musl_toolchain"
else
  # Set the deployment target for Rust and its native C dependencies.
  export MACOSX_DEPLOYMENT_TARGET="${MACOSX_DEPLOYMENT_TARGET:-13.0}"
fi

build_go() {
  local dir="$1" tool="$2" pkg="$3" ldflags="$4"
  (cd "$dir" && CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -mod=readonly \
    -trimpath -buildvcs=false -ldflags="-s -w $ldflags" -o "$dist/$tool-$suffix" "$pkg")
}
build_go . otter . "-X github.com/otterlab-bio/otter/cmd.buildVersion=$version -X github.com/otterlab-bio/otter/cmd.buildCommit=$commit -X github.com/otterlab-bio/otter/cmd.buildDate=$build_date"
build_go installer otter-install . "-X main.version=$version -X main.commit=$commit -X main.date=$build_date"
build_go craftmake craftmake ./cmd/craftmake "-X main.version=$version -X main.commit=$commit -X main.date=$build_date"
build_go xenofilx xenofilx ./cmd/xenofilx "-X github.com/otterlab-bio/xenofilx/pkg/cli.Version=$version"
build_go pairbam pairbam ./cmd/pairbam "-X main.Version=$version"
build_go seq2mat seq2mat ./cmd/seq2mat "-X main.Version=$version"
build_go matsrun matsrun ./cmd/matsrun "-X github.com/otterlab-bio/matsrun/pkg/cli.Version=$version"
# bamdriver is a Go library, not a release executable.
(cd bamdriver && CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -mod=readonly ./...)

for tool in enva fastqcx methx qctb; do
  target_dir="${CARGO_TARGET_DIR:-$ROOT/$tool/target}"
  [[ "$target_dir" == /* ]] || target_dir="$ROOT/$target_dir"
  args=(build --locked --release --target "$rust_target" --target-dir "$target_dir" --bin "$tool")
  [[ "$tool" != methx ]] || args+=(--features download)
  (
    cd "$tool"
    if [[ "$os" == linux && "$tool" == methx ]]; then
      export "CMAKE_TOOLCHAIN_FILE_${rust_target//-/_}=$musl_toolchain"
    fi
    cargo "${args[@]}"
  )
  cp "$target_dir/$rust_target/release/$tool" "$dist/$tool-$suffix"
done
