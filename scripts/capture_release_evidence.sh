#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/capture_release_evidence.sh [options]

Capture minimal release evidence for:
  1. RRBS local dry-run
  2. RNASEQ slurm dry-run

Options:
  --out-dir DIR          Output directory for reports and logs (default: docs/review)
  --date YYYY-MM-DD      Override UTC report date (default: today UTC)
  --binary PATH          Use an existing otter binary instead of building one
  --slurm-partition NAME SLURM partition for the RNASEQ dry-run (default: auto-detect via sinfo)
  --keep-workdir         Do not delete the temporary working directory
  --skip-rrbs            Skip RRBS local dry-run capture
  --skip-rnaseq          Skip RNASEQ slurm dry-run capture
  --help                 Show this help message
USAGE
}

OUT_DIR="docs/review"
REPORT_DATE="$(date -u +%Y-%m-%d)"
BINARY=""
SLURM_PARTITION="${OTTER_RELEASE_SLURM_PARTITION:-}"
KEEP_WORKDIR=0
SKIP_RRBS=0
SKIP_RNASEQ=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --out-dir)
      OUT_DIR="$2"
      shift 2
      ;;
    --date)
      REPORT_DATE="$2"
      shift 2
      ;;
    --binary)
      BINARY="$2"
      shift 2
      ;;
    --slurm-partition)
      SLURM_PARTITION="$2"
      shift 2
      ;;
    --keep-workdir)
      KEEP_WORKDIR=1
      shift
      ;;
    --skip-rrbs)
      SKIP_RRBS=1
      shift
      ;;
    --skip-rnaseq)
      SKIP_RNASEQ=1
      shift
      ;;
    --help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ "$SKIP_RRBS" -eq 1 && "$SKIP_RNASEQ" -eq 1 ]]; then
  echo "At least one scenario must remain enabled." >&2
  exit 1
fi

REPO_ROOT="$(pwd)"
OUT_DIR="$(mkdir -p "$OUT_DIR" && cd "$OUT_DIR" && pwd)"
EVIDENCE_DIR="$OUT_DIR/release_evidence_${REPORT_DATE}"
mkdir -p "$EVIDENCE_DIR"

WORK_ROOT="$(mktemp -d /tmp/otter-release-evidence-XXXXXX)"
cleanup() {
  if [[ "$KEEP_WORKDIR" -eq 1 ]]; then
    echo "Keeping work directory: $WORK_ROOT"
    return
  fi
  rm -rf "$WORK_ROOT"
}
trap cleanup EXIT

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Required command not found: $cmd" >&2
    return 1
  fi
}

resolve_slurm_partition() {
  if [[ -n "$SLURM_PARTITION" ]]; then
    printf '%s\n' "$SLURM_PARTITION"
    return 0
  fi

  local detected
  detected="$(sinfo -h -o '%P' 2>/dev/null | sed 's/*//g' | head -n 1 | tr -d '[:space:]')"
  if [[ -z "$detected" ]]; then
    return 1
  fi
  printf '%s\n' "$detected"
}

if [[ -z "$BINARY" ]]; then
  BINARY="$WORK_ROOT/otter"
  echo "Building otter for evidence capture..."
  go build -trimpath -o "$BINARY" .
fi

if [[ ! -x "$BINARY" ]]; then
  echo "otter binary is not executable: $BINARY" >&2
  exit 1
fi

RRBS_STATUS="SKIPPED"
RRBS_LOG=""
RRBS_CMD=""
RNASEQ_STATUS="SKIPPED"
RNASEQ_LOG=""
RNASEQ_CMD=""

run_case() {
  local key="$1"
  local cmd="$2"
  local log_file="$3"

  printf 'Command: %s\n\n' "$cmd" > "$log_file"
  set +e
  bash -lc "$cmd" >> "$log_file" 2>&1
  local exit_code=$?
  set -e

  if [[ $exit_code -eq 0 ]]; then
    printf 'PASS\n' >> "$log_file"
    return 0
  fi

  printf 'FAIL (exit=%d)\n' "$exit_code" >> "$log_file"
  return $exit_code
}

if [[ "$SKIP_RRBS" -eq 0 ]]; then
  require_cmd snakemake
  RRBS_LOG="$EVIDENCE_DIR/rrbs_local_dry_run.log"
  RRBS_PROJECT="$WORK_ROOT/rrbs_project"
  RRBS_CMD=$(cat <<CMD
cd "$REPO_ROOT" && \
"$BINARY" init "$RRBS_PROJECT" && \
"$BINARY" create --fastq "$REPO_ROOT/testdata/fastq/test_fastq" --pdata "$REPO_ROOT/testdata/pdata/test_pdata.csv" --mode RRBS --output "$RRBS_PROJECT/userspace" --jobid rrbs_release_smoke && \
"$BINARY" run --config "$RRBS_PROJECT/userspace/rrbs_release_smoke/config/otter.yaml" --engine local --dry-run --step1-cores 4 --step1-memory 1G --step2-cores 4 --step2-memory 1G --step3-cores 4 --step3-memory 1G
CMD
)
  if run_case rrbs_local_dry_run "$RRBS_CMD" "$RRBS_LOG"; then
    RRBS_STATUS="PASS"
  else
    RRBS_STATUS="FAIL"
  fi
fi

if [[ "$SKIP_RNASEQ" -eq 0 ]]; then
  require_cmd snakemake
  require_cmd sinfo
  require_cmd sbatch
  require_cmd squeue
  SLURM_PARTITION="$(resolve_slurm_partition)"
  if [[ -z "$SLURM_PARTITION" ]]; then
    echo "Could not determine SLURM partition; pass --slurm-partition." >&2
    exit 1
  fi

  RNASEQ_LOG="$EVIDENCE_DIR/rnaseq_slurm_dry_run.log"
  RNASEQ_PROJECT="$WORK_ROOT/rnaseq_project"
  RNASEQ_CMD=$(cat <<CMD
cd "$REPO_ROOT" && \
"$BINARY" init "$RNASEQ_PROJECT" && \
"$BINARY" create --fastq "$REPO_ROOT/testdata/fastq/test_fastq" --pdata "$REPO_ROOT/testdata/pdata/test_pdata.csv" --mode RNASEQ --output "$RNASEQ_PROJECT/userspace" --jobid rnaseq_release_smoke && \
"$BINARY" run --config "$RNASEQ_PROJECT/userspace/rnaseq_release_smoke/config/otter.yaml" --engine slurm --slurm-partition "$SLURM_PARTITION" --dry-run --step1-cores 4 --step1-memory 1G --step2-cores 4 --step2-memory 1G --step3-cores 4 --step3-memory 1G
CMD
)
  if run_case rnaseq_slurm_dry_run "$RNASEQ_CMD" "$RNASEQ_LOG"; then
    RNASEQ_STATUS="PASS"
  else
    RNASEQ_STATUS="FAIL"
  fi
fi

GENERATED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
DATE_REPORT="$OUT_DIR/release_evidence_${REPORT_DATE}.md"
LATEST_REPORT="$OUT_DIR/release_evidence_latest.md"
LATEST_ENV="$OUT_DIR/release_evidence_latest.env"

cat > "$DATE_REPORT" <<EOF2
# Release Evidence (${REPORT_DATE})

- Generated at (UTC): \
  \
  \
- Commit: \
  \
  \
- Binary: \
  \
  \

| Scenario | Status | Log |
|---|---|---|
| RRBS local dry-run | ${RRBS_STATUS} | \
| RNASEQ slurm dry-run | ${RNASEQ_STATUS} | \

## RRBS command

\

## RNASEQ command

\
EOF2

python3 - <<'PY' "$DATE_REPORT" "$GENERATED_AT" "$COMMIT" "$BINARY" "$RRBS_STATUS" "$RRBS_LOG" "$RNASEQ_STATUS" "$RNASEQ_LOG" "$RRBS_CMD" "$RNASEQ_CMD"
from pathlib import Path
import sys
report = Path(sys.argv[1])
generated_at, commit, binary = sys.argv[2], sys.argv[3], sys.argv[4]
rrbs_status, rrbs_log = sys.argv[5], sys.argv[6]
rnaseq_status, rnaseq_log = sys.argv[7], sys.argv[8]
rrbs_cmd, rnaseq_cmd = sys.argv[9], sys.argv[10]
text = report.read_text()
text = text.replace('- Generated at (UTC): \\\n  \\\n  \\\n', f'- Generated at (UTC): `{generated_at}`\n')
text = text.replace('- Commit: \\\n  \\\n  \\\n', f'- Commit: `{commit}`\n')
text = text.replace('- Binary: \\\n  \\\n  \\\n', f'- Binary: `{binary}`\n')
text = text.replace('| RRBS local dry-run | ' + rrbs_status + ' | \\\n', f'| RRBS local dry-run | {rrbs_status} | `{Path(rrbs_log).name if rrbs_log else "-"}` |\n')
text = text.replace('| RNASEQ slurm dry-run | ' + rnaseq_status + ' | \\\n', f'| RNASEQ slurm dry-run | {rnaseq_status} | `{Path(rnaseq_log).name if rnaseq_log else "-"}` |\n')
text = text.replace('## RRBS command\n\n\\\n', f'## RRBS command\n\n```bash\n{rrbs_cmd or "(skipped)"}\n```\n')
text = text.replace('## RNASEQ command\n\n\\\n', f'## RNASEQ command\n\n```bash\n{rnaseq_cmd or "(skipped)"}\n```\n')
report.write_text(text)
PY

cp "$DATE_REPORT" "$LATEST_REPORT"
cat > "$LATEST_ENV" <<EOF2
EVIDENCE_DATE=${REPORT_DATE}
GENERATED_AT_UTC=${GENERATED_AT}
COMMIT=${COMMIT}
RRBS_LOCAL_STATUS=${RRBS_STATUS}
RRBS_LOCAL_LOG=$(basename "${RRBS_LOG:-}")
RNASEQ_SLURM_STATUS=${RNASEQ_STATUS}
RNASEQ_SLURM_LOG=$(basename "${RNASEQ_LOG:-}")
EOF2

if [[ "$RRBS_STATUS" != "PASS" || "$RNASEQ_STATUS" != "PASS" ]]; then
  echo "Release evidence capture incomplete. See $DATE_REPORT and $EVIDENCE_DIR/*.log" >&2
  exit 1
fi

echo "Release evidence written to: $DATE_REPORT"
echo "Latest evidence manifest: $LATEST_ENV"
