#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${GITHUB_PAT:-}" ]]; then
  echo "[error] GITHUB_PAT is not set" >&2
  echo "Usage: GITHUB_PAT=... $0" >&2
  exit 2
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

with_pat_origin() {
  local repo_dir="$1"
  shift

  local old_url
  old_url="$(git -C "$repo_dir" remote get-url origin)"

  local new_url="$old_url"
  if [[ "$old_url" == https://github.com/* ]]; then
    new_url="https://x-access-token:${GITHUB_PAT}@github.com/${old_url#https://github.com/}"
  elif [[ "$old_url" == http://github.com/* ]]; then
    new_url="http://x-access-token:${GITHUB_PAT}@github.com/${old_url#http://github.com/}"
  fi

  if [[ "$new_url" != "$old_url" ]]; then
    git -C "$repo_dir" remote set-url origin "$new_url"
  fi

  trap 'git -C "'"$repo_dir"'" remote set-url origin "'"$old_url"'" >/dev/null 2>&1 || true' RETURN
  "$@"
  trap - RETURN

  if [[ "$new_url" != "$old_url" ]]; then
    git -C "$repo_dir" remote set-url origin "$old_url"
  fi
}

require_clean() {
  local repo_dir="$1"
  if [[ -n "$(git -C "$repo_dir" status --porcelain)" ]]; then
    echo "[error] dirty working tree: $repo_dir" >&2
    git -C "$repo_dir" status -sb >&2
    exit 3
  fi
}

push_repo() {
  local repo_dir="$1"
  local branch="${2:-main}"

  require_clean "$repo_dir"

  echo "[push] $(basename "$repo_dir") ($branch)"
  with_pat_origin "$repo_dir" git -C "$repo_dir" push --follow-tags origin "$branch"
}

push_repo "$ROOT_DIR/bamdriver" main
push_repo "$ROOT_DIR" main

echo "[done] pushed bamdriver + otter"

