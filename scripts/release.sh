#!/bin/bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/release.sh <version> [options]

Accepted version formats:
  1.2.3            -> immutable tag v1.2.3
  2026.03.23.1     -> immutable tag v2026.03.23.1

For date-based releases, the script updates the movable daily alias
`daily-YYYYMMDD` by default so multiple builds in one day can share a
single discovery tag while each archived release remains immutable.

Options:
  --no-daily-alias   Do not update the daily alias for date-based releases
  --help             Show this help message

Examples:
  bash scripts/release.sh 0.1.0
  bash scripts/release.sh 2026.03.23.1
  bash scripts/release.sh 2026.03.23.2 --no-daily-alias
USAGE
}

VERSION=""
UPDATE_DAILY_ALIAS=true

while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-daily-alias)
      UPDATE_DAILY_ALIAS=false
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    -* )
      echo "Error: Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
    *)
      if [[ -n "$VERSION" ]]; then
        echo "Error: multiple version arguments provided" >&2
        usage >&2
        exit 1
      fi
      VERSION="$1"
      shift
      ;;
  esac
done

if [[ -z "$VERSION" ]]; then
  usage >&2
  exit 1
fi

TAG=""
DAILY_ALIAS=""
VERSION_KIND=""

if [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  VERSION_KIND="semver"
  TAG="v${VERSION}"
elif [[ "$VERSION" =~ ^([0-9]{4})\.([0-9]{2})\.([0-9]{2})\.([0-9]+)$ ]]; then
  VERSION_KIND="dated"
  TAG="v${VERSION}"
  DAILY_ALIAS="daily-${BASH_REMATCH[1]}${BASH_REMATCH[2]}${BASH_REMATCH[3]}"
else
  echo "Error: Version must be X.Y.Z or YYYY.MM.DD.N" >&2
  exit 1
fi

if git rev-parse -q --verify "refs/tags/${TAG}" >/dev/null; then
  echo "Error: Tag ${TAG} already exists locally" >&2
  exit 1
fi

if git ls-remote --exit-code --tags origin "refs/tags/${TAG}" >/dev/null 2>&1; then
  echo "Error: Tag ${TAG} already exists on origin" >&2
  exit 1
fi

echo "Running release checks..."
bash scripts/verify_release_evidence.sh
go test ./...
go vet ./...
./scripts/build.sh "$VERSION"

echo "Creating tag ${TAG}..."
git tag -a "$TAG" -m "Release ${TAG}"

if [[ "$VERSION_KIND" = "dated" && "$UPDATE_DAILY_ALIAS" = true ]]; then
  echo "Updating daily alias ${DAILY_ALIAS} -> ${TAG}..."
  git tag -fa "$DAILY_ALIAS" -m "Daily build alias for ${TAG}"
fi

echo "Pushing ${TAG} to remote..."
git push origin "refs/tags/${TAG}"

if [[ "$VERSION_KIND" = "dated" && "$UPDATE_DAILY_ALIAS" = true ]]; then
  echo "Pushing daily alias ${DAILY_ALIAS} to remote..."
  git push --force origin "refs/tags/${DAILY_ALIAS}"
fi

echo "Release ${TAG} created and pushed!"
if [[ "$VERSION_KIND" = "dated" && "$UPDATE_DAILY_ALIAS" = true ]]; then
  echo "Daily alias ${DAILY_ALIAS} now points to ${TAG}."
fi
echo "GitHub Actions will build and publish the release."
