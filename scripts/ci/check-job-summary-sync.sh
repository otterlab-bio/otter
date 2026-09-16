#!/usr/bin/env bash
#
# Fail when a repository's copy of a CI helper drifts from the umbrella copy.
#
# Every repository carries its own `scripts/ci/*.sh` because each one is checked out independently
# and cannot read a sibling repository at workflow runtime. That duplication is only safe with a
# guard, which is this script: it compares every submodule copy with the umbrella copy and reports
# any difference.
#
# usage: check-job-summary-sync.sh [repo-root]
set -euo pipefail

repository_root="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"

# Helpers that must stay byte-identical across every repository that ships workflows.
helper_names=(
  "job-summary.sh"
  "summary-field.sh"
)

drift_count=0
checked_count=0

for helper_name in "${helper_names[@]}"; do
  canonical="${repository_root}/scripts/ci/${helper_name}"
  if [[ ! -f "${canonical}" ]]; then
    echo "canonical helper not found: ${canonical}" >&2
    exit 1
  fi
  canonical_digest="$(sha256sum "${canonical}" | awk '{print $1}')"

  for candidate in "${repository_root}"/*/scripts/ci/"${helper_name}"; do
    [[ -f "${candidate}" ]] || continue
    repository_name="$(basename "$(dirname "$(dirname "$(dirname "${candidate}")")")")"
    checked_count=$((checked_count + 1))
    candidate_digest="$(sha256sum "${candidate}" | awk '{print $1}')"
    if [[ "${candidate_digest}" != "${canonical_digest}" ]]; then
      echo "DRIFT: ${repository_name}/scripts/ci/${helper_name} differs from the umbrella copy" >&2
      diff -u "${canonical}" "${candidate}" >&2 || true
      drift_count=$((drift_count + 1))
    fi
  done
done

echo "CI helpers: ${checked_count} repository copy/copies checked across ${#helper_names[@]} helper(s), ${drift_count} drifted"
if [[ "${drift_count}" -gt 0 ]]; then
  echo "Copy the umbrella scripts/ci/<helper>.sh over each drifted copy." >&2
  exit 1
fi
