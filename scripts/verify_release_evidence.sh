#!/usr/bin/env bash

set -euo pipefail

MANIFEST_PATH="${1:-docs/review/release_evidence_latest.env}"

if [[ ! -f "$MANIFEST_PATH" ]]; then
  echo "Release evidence manifest not found: $MANIFEST_PATH" >&2
  echo "Run: bash scripts/capture_release_evidence.sh" >&2
  exit 1
fi

# shellcheck disable=SC1090
source "$MANIFEST_PATH"

: "${EVIDENCE_DATE:?missing EVIDENCE_DATE}"
: "${RRBS_LOCAL_STATUS:?missing RRBS_LOCAL_STATUS}"
: "${RNASEQ_SLURM_STATUS:?missing RNASEQ_SLURM_STATUS}"

if [[ "$RRBS_LOCAL_STATUS" != "PASS" ]]; then
  echo "RRBS local dry-run evidence is not PASS (got: $RRBS_LOCAL_STATUS)" >&2
  exit 1
fi

if [[ "$RNASEQ_SLURM_STATUS" != "PASS" ]]; then
  echo "RNASEQ slurm dry-run evidence is not PASS (got: $RNASEQ_SLURM_STATUS)" >&2
  exit 1
fi

REPORT_PATH="$(dirname "$MANIFEST_PATH")/release_evidence_latest.md"
if [[ ! -f "$REPORT_PATH" ]]; then
  echo "Release evidence report not found: $REPORT_PATH" >&2
  exit 1
fi

echo "Release evidence verified: $EVIDENCE_DATE"
