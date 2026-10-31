#!/usr/bin/env bash
#
# Verify every Go module in the tree against a *cold* module cache.
#
# Why this exists: a dependency tag can be moved after publication, which
# changes the module zip while sum.golang.org keeps the original hash. Every
# subsequent download then fails with "checksum mismatch". That failure is
# invisible on a machine whose module cache was populated before the tag moved,
# so a warm actions/setup-go cache hid exactly this for six weeks here and it
# only surfaced when the runner image changed and the cache stopped matching.
#
# This gate therefore ignores the ambient cache. Each module is copied into a
# scratch directory (so the work tree is never touched) and its full dependency
# graph is downloaded into a throwaway GOMODCACHE and verified against go.sum.
#
# Note: `go mod download` can append entries to go.sum, so verification always
# happens on the copy, never on the checked-out file.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORK_DIR="$(mktemp -d "${RUNNER_TEMP:-/tmp}/modverify.XXXXXX")"
TMP_CACHE="$WORK_DIR/modcache"

# Go marks everything it writes to the module cache read-only, so a plain
# `rm -rf` fails on the extracted module trees. Restore write bits first.
cleanup() {
  chmod -R u+w "$WORK_DIR" 2>/dev/null || true
  rm -rf "$WORK_DIR" 2>/dev/null || true
}
trap cleanup EXIT

status=0
checked=0

while IFS= read -r gomod; do
  dir="$(dirname "$gomod")"
  name="${dir#"$ROOT_DIR"/}"
  [ "$dir" = "$ROOT_DIR" ] && name="(umbrella)"

  printf '== %s\n' "$name"

  # A relative replace points at a sibling checkout, so the module cannot be
  # verified in isolation. Report it rather than silently claiming a pass.
  if grep -Eq '^[[:space:]]*replace[[:space:]].*=>[[:space:]]*\.{1,2}/' "$dir/go.mod"; then
    printf '   skip: go.mod uses a relative replace, cannot verify in isolation\n'
    continue
  fi

  scratch="$WORK_DIR/src/$checked"
  mkdir -p "$scratch"
  cp "$dir/go.mod" "$scratch/go.mod"
  [ -f "$dir/go.sum" ] && cp "$dir/go.sum" "$scratch/go.sum"

  # Transient network failures (sumdb tiles, proxy redirects) must not be
  # reported as a checksum problem, so retry before deciding it failed.
  downloaded=0
  for attempt in 1 2 3; do
    if (cd "$scratch" && GOFLAGS=-mod=readonly GOMODCACHE="$TMP_CACHE" go mod download all); then
      downloaded=1
      break
    fi
    if [ "$attempt" -lt 3 ]; then
      printf '   retrying download (%d/3)\n' "$((attempt + 1))"
      sleep 5
    fi
  done
  if [ "$downloaded" -ne 1 ]; then
    printf '   FAIL: %s could not download its modules from a cold cache\n' "$name" >&2
    status=1
    continue
  fi

  if ! (cd "$scratch" && GOFLAGS=-mod=readonly GOMODCACHE="$TMP_CACHE" go mod verify); then
    printf '   FAIL: %s has modules that do not match go.sum\n' "$name" >&2
    status=1
    continue
  fi

  checked=$((checked + 1))
  printf '   ok\n'
done < <(find "$ROOT_DIR" -name go.mod -not -path '*/.git/*' | sort)

if [ "$checked" -eq 0 ]; then
  echo 'error: no go.mod files found' >&2
  exit 1
fi

if [ "$status" -ne 0 ]; then
  cat >&2 <<'MSG'

A module failed verification from a cold cache. The usual cause is a
dependency whose tag was moved after publication: the bytes served for that
version no longer match the hash in go.sum / sum.golang.org. Do NOT silence
this by deleting go.sum entries or setting GONOSUMDB / GOFLAGS=-mod=mod --
publish a new patch version of the dependency instead and update consumers.
MSG
  exit 1
fi

echo "verified $checked module(s) against a cold cache"
