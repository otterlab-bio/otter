#!/usr/bin/env bash
#
# Render the job summary briefing shown on the GitHub Actions summary page.
#
# This file is duplicated verbatim into every repository that has workflows, because each
# repository is checked out independently and cannot rely on a sibling's copy. Keep the copies
# byte-identical: `scripts/ci/check-job-summary-sync.sh` in the umbrella repository fails when
# they drift.
#
# Why a single trailing step: the GitHub docs state that `GITHUB_STEP_SUMMARY` is unique and
# isolated per step, capped at 1 MiB per step, and that only 20 step summaries are displayed per
# job. A workflow must therefore emit its whole briefing from one step (`if: always()`), not by
# appending fragments from every step.
#
# This helper only reports. It never decides whether the job passes: assertions stay in the steps
# that can fail, so a summary bug can never mask a real failure or fabricate a pass.
#
# usage (inside a step with `if: always()`):
#
#   bash scripts/ci/job-summary.sh \
#     --verdict "${{ job.status }}" \
#     --title "Xenofilx Gate C" \
#     --scope "${{ matrix.cell }}" \
#     --checked "Compares the reference-aware NM and classification score with the independent
#                bamdriver oracle, record for record, on the original and converted references." \
#     --check "Primary arm oracle equality|200000/200000|200000/200000|PASS" \
#     --metric "Fixture BAM|3,125,450,579 bytes" \
#     --artifact "xenofilx-gate-c-${{ matrix.cell }}" \
#     --limit "A 200,000-record contract audit, not a production-scale performance claim."
#
# `--check` takes `assertion|expected|observed|result`; `--metric` takes `name|value`.
# Both are repeatable. Missing trailing fields render as `-`.
set -euo pipefail

script_directory="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
workflow_name="${GITHUB_WORKFLOW:-}"
job_name="${GITHUB_JOB:-}"
title=""
scope=""
verdict=""
checked=""
summary_file="${GITHUB_STEP_SUMMARY:-}"
print_to_stdout=false

declare -a checks=()
declare -a metrics=()
declare -a artifacts=()
declare -a limits=()
declare -a notes=()

usage() {
  sed -n '2,30p' "${script_directory}/job-summary.sh" | sed 's/^#\{1,\} \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --title) title="${2:?}"; shift 2 ;;
    --scope) scope="${2:?}"; shift 2 ;;
    --verdict) verdict="${2:?}"; shift 2 ;;
    --checked) checked="${2:?}"; shift 2 ;;
    --check) checks+=("${2:?}"); shift 2 ;;
    --metric) metrics+=("${2:?}"); shift 2 ;;
    --artifact) artifacts+=("${2:?}"); shift 2 ;;
    --limit) limits+=("${2:?}"); shift 2 ;;
    --note) notes+=("${2:?}"); shift 2 ;;
    --summary-file) summary_file="${2:?}"; shift 2 ;;
    --stdout) print_to_stdout=true; shift ;;
    --help|-h) usage; exit 0 ;;
    *) echo "job-summary.sh: unknown argument: $1" >&2; exit 2 ;;
  esac
done

# Escape the characters that would break a Markdown table cell and flatten newlines.
sanitize_cell() {
  printf '%s' "${1:--}" | tr '\n' ' ' | sed 's/|/\\|/g'
}

# Fill a `a|b|c` row up to `count` fields with `-`, so a partially specified row still renders.
split_row() {
  local value="$1" count="$2" field
  local -a fields=()
  IFS='|' read -r -a fields <<<"${value}"
  for ((field = 0; field < count; field++)); do
    printf '%s\n' "${fields[field]:--}"
  done
}

render() {
  local verdict_body
  case "${verdict}" in
    success) verdict_body="PASS" ;;
    failure) verdict_body="FAIL" ;;
    cancelled) verdict_body="CANCELLED" ;;
    skipped) verdict_body="SKIPPED" ;;
    "" | *) verdict_body="${verdict:-unknown}" ;;
  esac

  printf '# %s\n\n' "${title:-${workflow_name:-Job summary}}"
  printf '### Verdict: %s' "${verdict_body}"
  [[ -n "${scope}" ]] && printf ' · scope `%s`' "$(sanitize_cell "${scope}")"
  printf '\n\n'

  printf '| | |\n|---|---|\n'
  printf '| Repository | `%s` |\n' "${GITHUB_REPOSITORY:-local}"
  printf '| Workflow / job | `%s` / `%s` |\n' \
    "$(sanitize_cell "${workflow_name:-unknown}")" "$(sanitize_cell "${job_name:-unknown}")"
  printf '| Commit | `%s` |\n' "${GITHUB_SHA:-unknown}"
  printf '| Ref / event | `%s` · `%s` |\n' \
    "${GITHUB_REF_NAME:-unknown}" "${GITHUB_EVENT_NAME:-unknown}"
  if [[ -n "${GITHUB_RUN_ID:-}" ]]; then
    printf '| Run | [%s](%s/%s/actions/runs/%s) |\n' \
      "${GITHUB_RUN_ID}" "${GITHUB_SERVER_URL:-https://github.com}" \
      "${GITHUB_REPOSITORY:-}" "${GITHUB_RUN_ID}"
  fi
  printf '| Attempt | %s |\n' "${GITHUB_RUN_ATTEMPT:-1}"
  printf '\n'

  if [[ -n "${checked}" ]]; then
    printf '## What this job checked\n\n%s\n\n' "${checked}"
  fi

  if [[ ${#checks[@]} -gt 0 ]]; then
    printf '## Contract\n\n| Assertion | Expected | Observed | Result |\n|---|---|---|---|\n'
    local entry assertion expected observed result
    for entry in "${checks[@]}"; do
      mapfile -t _row < <(split_row "${entry}" 4)
      assertion="${_row[0]}"; expected="${_row[1]}"; observed="${_row[2]}"; result="${_row[3]}"
      printf '| %s | %s | %s | %s |\n' \
        "$(sanitize_cell "${assertion}")" "$(sanitize_cell "${expected}")" \
        "$(sanitize_cell "${observed}")" "$(sanitize_cell "${result}")"
    done
    printf '\n'
  fi

  if [[ ${#metrics[@]} -gt 0 ]]; then
    printf '## Observed values\n\n| Metric | Value |\n|---|---|\n'
    local metric metric_name metric_value
    for metric in "${metrics[@]}"; do
      mapfile -t _row < <(split_row "${metric}" 2)
      metric_name="${_row[0]}"; metric_value="${_row[1]}"
      printf '| %s | %s |\n' "$(sanitize_cell "${metric_name}")" "$(sanitize_cell "${metric_value}")"
    done
    printf '\n'
  fi

  if [[ ${#artifacts[@]} -gt 0 ]]; then
    printf '## Evidence\n\n'
    local artifact
    for artifact in "${artifacts[@]}"; do
      printf -- '- artifact `%s`\n' "$(sanitize_cell "${artifact}")"
    done
    printf '\n'
  fi

  if [[ ${#limits[@]} -gt 0 ]]; then
    printf '## Scope and limits\n\n'
    local limit
    for limit in "${limits[@]}"; do
      printf -- '- %s\n' "${limit}"
    done
    printf '\n'
  fi

  if [[ ${#notes[@]} -gt 0 ]]; then
    printf '## Notes\n\n'
    local note
    for note in "${notes[@]}"; do
      printf -- '- %s\n' "${note}"
    done
    printf '\n'
  fi

  printf -- '---\n\n_Briefing generated by `scripts/ci/job-summary.sh`; step logs and assertions remain authoritative._\n'
}

rendered="$(render)"

# The docs cap one step summary at 1 MiB and fail the upload beyond it. Truncate deterministically
# so an oversized briefing degrades visibly instead of disappearing.
#
# Two details matter. The notice is appended after the cut, so the cut must reserve room for it or
# the result would exceed the very cap being enforced. And the cut must not be done by piping into
# `head -c`: head exits early, the writer takes SIGPIPE, and under `pipefail` the whole helper would
# die with no output at all. Writing to a scratch file and cutting at a line boundary avoids both.
truncation_notice="

> Truncated: this briefing exceeded the 1 MiB per-step summary limit."
maximum_bytes=$((1024 * 1024))
notice_bytes="$(printf '%s' "${truncation_notice}" | wc -c | tr -d ' ')"
rendered_bytes="$(printf '%s' "${rendered}" | wc -c | tr -d ' ')"
if (( rendered_bytes > maximum_bytes )); then
  scratch_file="$(mktemp)"
  trap 'rm -f "${scratch_file}"' EXIT
  printf '%s' "${rendered}" >"${scratch_file}"
  rendered="$(awk -v limit="$((maximum_bytes - notice_bytes))" '
    BEGIN { total = 0 }
    { if (total + length($0) + 1 > limit) { exit } ; print ; total += length($0) + 1 }
  ' "${scratch_file}")${truncation_notice}"
  rm -f "${scratch_file}"
  trap - EXIT
fi

if [[ "${print_to_stdout}" == true || -z "${summary_file}" ]]; then
  printf '%s\n' "${rendered}"
else
  printf '%s\n' "${rendered}" >>"${summary_file}"
fi
