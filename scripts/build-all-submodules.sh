#!/usr/bin/env bash
# Build all submodules and install to $HOME/.cargo/bin

set -euo pipefail

CARGO_BIN="${HOME}/.cargo/bin"
STRICT_MODE="${STRICT_MODE:-1}" # 1: fail if required binaries missing, 0: warn only

mkdir -p "${CARGO_BIN}"

GO_ENV_NAME=""
RUST_ENV_NAME=""
GO_SOURCE=""
RUST_SOURCE=""

pick_best_go() {
  local best_src=""
  local best_ver="0.0.0"
  local ver=""

  if command -v go >/dev/null 2>&1; then
    ver="$(go version | awk '{print $3}' | sed 's/^go//')"
    best_src="system"
    best_ver="${ver}"
  fi

  if command -v conda >/dev/null 2>&1 && conda env list 2>/dev/null | grep -q "^go-env "; then
    ver="$(conda run -n go-env go version 2>/dev/null | awk '{print $3}' | sed 's/^go//')"
    if [ -n "${ver}" ] && [ "$(printf '%s\n%s\n' "${best_ver}" "${ver}" | sort -V | tail -n1)" = "${ver}" ]; then
      best_src="go-env"
      best_ver="${ver}"
    fi
  fi

  if command -v conda >/dev/null 2>&1 && conda env list 2>/dev/null | grep -q "^go-build "; then
    ver="$(conda run -n go-build go version 2>/dev/null | awk '{print $3}' | sed 's/^go//')"
    if [ -n "${ver}" ] && [ "$(printf '%s\n%s\n' "${best_ver}" "${ver}" | sort -V | tail -n1)" = "${ver}" ]; then
      best_src="go-build"
      best_ver="${ver}"
    fi
  fi

  if [ -z "${best_src}" ]; then
    return 1
  fi

  GO_SOURCE="${best_src} (${best_ver})"
  case "${best_src}" in
    system) GO_CMD=(go) ;;
    go-env) GO_ENV_NAME="go-env"; GO_CMD=(conda run -n "${GO_ENV_NAME}" go) ;;
    go-build) GO_ENV_NAME="go-build"; GO_CMD=(conda run -n "${GO_ENV_NAME}" go) ;;
  esac
}

if ! pick_best_go; then
  echo "Error: Go not found (PATH/go-env/go-build)."
  exit 1
fi

if command -v conda >/dev/null 2>&1 && conda env list 2>/dev/null | grep -q "^rust_build "; then
  RUST_ENV_NAME="rust_build"
  RUST_CMD=(conda run -n "${RUST_ENV_NAME}" cargo)
  RUST_SOURCE="rust_build"
elif command -v conda >/dev/null 2>&1 && conda env list 2>/dev/null | grep -q "^rust-build "; then
  RUST_ENV_NAME="rust-build"
  RUST_CMD=(conda run -n "${RUST_ENV_NAME}" cargo)
  RUST_SOURCE="rust-build"
elif command -v cargo >/dev/null 2>&1; then
  RUST_CMD=(cargo)
  RUST_SOURCE="system"
else
  echo "Error: Cargo not found (PATH/rust_build/rust-build)."
  exit 1
fi

echo "======================================"
echo "Building all otter submodules"
echo "======================================"
echo "Go: ${GO_SOURCE}"
echo "Rust: ${RUST_SOURCE}"
echo "Strict mode: ${STRICT_MODE}"
echo ""

build_go() {
  local name="$1"
  local dir="$2"
  local binary="$3"
  local main_pkg="$4"
  local version_symbol="$5"
  local version_value
  local ldflags

  echo "Building ${name}..."
  if [ "${dir}" = "seq2mat" ]; then
    bash scripts/ensure_seq2mat_entrypoint.sh "${dir}"
  fi

  version_value="$(git -C "${dir}" describe --tags --abbrev=7 --dirty 2>/dev/null || true)"
  if [[ "${version_value}" =~ ^v?([0-9]+\.[0-9]+\.[0-9]+)(.*)$ ]]; then
    version_value="${BASH_REMATCH[1]}${BASH_REMATCH[2]}"
  else
    version_value="0.1.0-$(git -C "${dir}" rev-parse --short HEAD 2>/dev/null || echo dev)"
  fi
  ldflags="-X ${version_symbol}=${version_value}"

  (
    cd "${dir}"
    "${GO_CMD[@]}" build -ldflags "${ldflags}" -o "${CARGO_BIN}/${binary}" "${main_pkg}"
  )
  echo "  ✓ Installed to ${CARGO_BIN}/${binary} (version: ${version_value})"
  echo ""
}

build_rust() {
  local name="$1"
  local dir="$2"
  local binary="$3"
  local source_binary="${4:-$3}"
  local cargo_args=()

  if [ "${dir}" = "methx" ]; then
    cargo_args+=(--features download)
  fi

  echo "Building ${name}..."
  (
    cd "${dir}"
    "${RUST_CMD[@]}" build --release "${cargo_args[@]}"
    cp "target/release/${source_binary}" "${CARGO_BIN}/${binary}"
  )
  echo "  ✓ Installed to ${CARGO_BIN}/${binary}"
  echo ""
}

# Go projects
build_go "xenofilx" "xenofilx" "xenofilx" "./cmd/xenofilx" "github.com/otterlab-bio/xenofilx/pkg/cli.Version"
build_go "pairbam" "pairbam" "pairbam" "./cmd/pairbam" "main.Version"
build_go "seq2mat" "seq2mat" "seq2mat" "./cmd/seq2mat" "main.Version"
build_go "matsrun" "matsrun" "matsrun" "./cmd/matsrun" "github.com/otterlab-bio/matsrun/pkg/cli.Version"

# Rust projects
build_rust "enva" "enva" "enva"
build_rust "fastqcx" "fastqcx" "fastqcx"
build_rust "methx" "methx" "methx"
build_rust "qctb" "qctb" "qctb"

echo "======================================"
echo "Verifying installations"
echo "======================================"

required_bins=(enva fastqcx xenofilx pairbam seq2mat methx qctb matsrun)
version_re='([0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?|[0-9]{4}\.[0-9]{2}\.[0-9]{2}\.[0-9]+|daily-[0-9]{8})'
missing_bins=()
invalid_versions=()

for binary in "${required_bins[@]}"; do
  if [ ! -x "${CARGO_BIN}/${binary}" ]; then
    echo "✗ ${binary} - NOT FOUND or NOT EXECUTABLE"
    missing_bins+=("${binary}")
    continue
  fi

  version_output="$("${CARGO_BIN}/${binary}" --version 2>&1 | head -1 || true)"
  expected_re="^${binary} ${version_re}$"
  if printf '%s\n' "${version_output}" | grep -Eq "${expected_re}"; then
    echo "✓ ${binary} - ${version_output}"
  else
    echo "✗ ${binary} - invalid version output: ${version_output}"
    invalid_versions+=("${binary}")
  fi
done

if [ "${#missing_bins[@]}" -gt 0 ] || [ "${#invalid_versions[@]}" -gt 0 ]; then
  echo ""
  if [ "${#missing_bins[@]}" -gt 0 ]; then
    echo "Missing required binaries: ${missing_bins[*]}"
  fi
  if [ "${#invalid_versions[@]}" -gt 0 ]; then
    echo "Invalid --version outputs: ${invalid_versions[*]}"
  fi
  if [ "${STRICT_MODE}" = "1" ]; then
    echo "Strict mode enabled, exiting with failure."
    exit 1
  fi
fi

echo ""
echo "Build complete!"
echo "Binaries installed to: ${CARGO_BIN}"
