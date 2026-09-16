#!/usr/bin/env bash

set -euo pipefail

DIST_DIR="${1:-dist}"
REPORT_DIR="${2:-${DIST_DIR}/release-verification}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ARTIFACT_LIST_SCRIPT="${ROOT_DIR}/scripts/verify_release_artifacts.sh"

if [[ ! -d "${DIST_DIR}" ]]; then
  echo "Distribution directory does not exist: ${DIST_DIR}" >&2
  exit 1
fi

mkdir -p "${REPORT_DIR}/file" "${REPORT_DIR}/ldd"

report_table="${REPORT_DIR}/static-linkage.tsv"
summary_file="${REPORT_DIR}/static-linkage-summary.md"
printf 'artifact\tsha256\tsize_bytes\tfile_status\tldd_exit_code\tlinkage_status\n' > "${report_table}"
printf '# Static Linkage Verification\n\n' > "${summary_file}"
printf '| Artifact | Status | ldd exit | Evidence |\n|---|---|---:|---|\n' >> "${summary_file}"

failure_count=0
while IFS= read -r artifact_name; do
  artifact_path="${DIST_DIR}/${artifact_name}"
  file_log="${REPORT_DIR}/file/${artifact_name}.log"
  ldd_log="${REPORT_DIR}/ldd/${artifact_name}.log"

  if [[ ! -x "${artifact_path}" ]]; then
    printf 'missing\n' > "${file_log}"
    printf 'missing\n' > "${ldd_log}"
    printf '%s\t%s\t%s\t%s\t%s\t%s\n' "${artifact_name}" "-" "-" "missing" "-" "FAIL" >> "${report_table}"
    printf '| `%s` | FAIL | - | missing executable |\n' "${artifact_name}" >> "${summary_file}"
    failure_count=$((failure_count + 1))
    continue
  fi

  file_output="$(file "${artifact_path}" 2>&1)"
  printf '%s\n' "${file_output}" > "${file_log}"

  set +e
  ldd_output="$(ldd "${artifact_path}" 2>&1)"
  ldd_exit_code=$?
  set -e
  printf '%s\n' "${ldd_output}" > "${ldd_log}"

  artifact_sha256="$(sha256sum "${artifact_path}" | awk '{print $1}')"
  artifact_size_bytes="$(stat --format='%s' "${artifact_path}")"

  linkage_status="FAIL"
  if printf '%s\n' "${ldd_output}" | grep -Eqi 'not a dynamic executable|statically linked'; then
    if ! printf '%s\n' "${ldd_output}" | grep -Eqi '=>[[:space:]]*/|linux-vdso|ld-linux|libc\.so|not found'; then
      linkage_status="PASS"
    fi
  fi

  printf '%s\t%s\t%s\t%s\t%s\t%s\n' \
    "${artifact_name}" \
    "${artifact_sha256}" \
    "${artifact_size_bytes}" \
    "$(printf '%s' "${file_output}" | tr '\t\n' ' ')" \
    "${ldd_exit_code}" \
    "${linkage_status}" >> "${report_table}"
  printf '| `%s` | %s | %s | `file/%s.log`, `ldd/%s.log` |\n' \
    "${artifact_name}" \
    "${linkage_status}" \
    "${ldd_exit_code}" \
    "${artifact_name}" \
    "${artifact_name}" >> "${summary_file}"

  if [[ "${linkage_status}" != "PASS" ]]; then
    failure_count=$((failure_count + 1))
  fi
done < <("${ARTIFACT_LIST_SCRIPT}" --print-required)

printf '\nVerified artifacts: %s\nFailures: %s\n' \
  "$("${ARTIFACT_LIST_SCRIPT}" --print-required | wc -l | tr -d ' ')" \
  "${failure_count}" >> "${summary_file}"

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  cat "${summary_file}" >> "${GITHUB_STEP_SUMMARY}"
fi

if [[ "${failure_count}" -gt 0 ]]; then
  echo "Static linkage verification failed for ${failure_count} release artifact(s)." >&2
  exit 1
fi

echo "Static linkage verification passed for all release artifacts."
