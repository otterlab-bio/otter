#!/usr/bin/env bash
#
# otter_e2e.sh — offline end-to-end rehearsal of the Otter authoring and
# execution contracts.
#
# What this covers
#   install  : otter-install plans an installation without touching the network
#   v1 track : otter init -> otter create -> otter config resolve ->
#              otter run --dry-run, for four scenarios, against a simulated
#              reference registry
#   build    : the same four scenarios authored a second way, through
#              "otter build", compared artifact-by-artifact against the manual
#              chain above. The shortcut's contract is equivalence, not merely
#              success, so this leg fails if project.yaml, samples.tsv, or
#              references.lock.yaml differ between the two paths.
#   legacy   : otter init --legacy -> otter create --legacy -> assets verify ->
#              config validate -> config migrate -> canonical validate ->
#              resolve -> run --dry-run
#   pairing  : the executor contract, asserted directly. The immutable
#              run.yaml is the single execution boundary for BOTH executors:
#              the snapshot selects its executor and --executor must agree.
#              A legacy otter.yaml is an authoring format only, so it is
#              refused by both executors until it is migrated and resolved.
#   guards   : cross-track authoring refusals
#
# What this deliberately does not do
#   It never downloads a reference genome and never runs a genome index builder.
#   The reference registry is simulated by stub-registry, which writes the exact
#   on-disk layout "otter reference build" produces and then verifies it with the
#   production reference verifier. It also never executes a workflow: both the
#   Craftmake and the build legs stop at "plan".
#
# Every stage writes its command, stdout, stderr, and exit code under
# --artifacts-dir so a failing run can be diagnosed without rerunning it.

set -euo pipefail

script_directory="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repository_root="$(cd "${script_directory}/../.." && pwd)"

otter_binary=""
craftmake_binary=""
stub_registry_binary=""
installer_binary=""
snakemake_stub=""
work_directory=""
artifacts_directory=""
fixture_root="${repository_root}/testdata/gate6/craftmake-downsample-20260906/fastq"
craftmake_catalog="${repository_root}/craftmake/workflows"
scenarios="rrbs,rnaseq,bs-pdx,rna-pdx"
run_legacy_leg=true
run_build_leg_flag=true
keep_work_directory=false

usage() {
  cat <<'USAGE'
usage: otter_e2e.sh --otter PATH --craftmake PATH [OPTIONS]

required:
  --otter PATH             otter binary built from this tree
  --craftmake PATH         craftmake binary built from the craftmake submodule

optional:
  --stub-registry PATH     stub-registry binary (default: built from this tree into the work dir)
  --installer PATH         otter-install binary; when given, the install plan leg runs
  --work-dir PATH          scratch directory (default: mktemp -d)
  --artifacts-dir PATH     evidence directory (default: <work-dir>/artifacts)
  --fixture-root PATH      downsampled FASTQ root (default: testdata/gate6/...)
  --craftmake-catalog PATH craftmake workflow catalog (default: craftmake/workflows)
  --scenarios LIST         comma-separated subset of rrbs,rnaseq,bs-pdx,rna-pdx
  --skip-legacy            skip the legacy compatibility leg
  --skip-build             skip the otter build comparison leg
  --keep                   keep the work directory (implies printing its path)
  --help                   show this message
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --otter) otter_binary="${2:?}"; shift 2 ;;
    --craftmake) craftmake_binary="${2:?}"; shift 2 ;;
    --stub-registry) stub_registry_binary="${2:?}"; shift 2 ;;
    --installer) installer_binary="${2:?}"; shift 2 ;;
    --work-dir) work_directory="${2:?}"; shift 2 ;;
    --artifacts-dir) artifacts_directory="${2:?}"; shift 2 ;;
    --fixture-root) fixture_root="${2:?}"; shift 2 ;;
    --craftmake-catalog) craftmake_catalog="${2:?}"; shift 2 ;;
    --scenarios) scenarios="${2:?}"; shift 2 ;;
    --skip-legacy) run_legacy_leg=false; shift ;;
    --skip-build) run_build_leg_flag=false; shift ;;
    --keep) keep_work_directory=true; shift ;;
    --help|-h) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

# Stage commands are executed from a scenario working directory rather than from the
# invocation directory, so a binary given as a relative path would not resolve. Resolve
# those here. A bare name is left untouched so a PATH lookup still works.
for variable_name in otter_binary craftmake_binary stub_registry_binary installer_binary; do
  variable_value="${!variable_name}"
  if [[ -n "${variable_value}" && "${variable_value}" == */* ]]; then
    printf -v "${variable_name}" '%s' \
      "$(cd "$(dirname "${variable_value}")" && pwd)/$(basename "${variable_value}")"
  fi
done

if [[ -z "${otter_binary}" || -z "${craftmake_binary}" ]]; then
  echo "error: --otter and --craftmake are required" >&2
  usage >&2
  exit 2
fi
for required_binary in "${otter_binary}" "${craftmake_binary}"; do
  if [[ ! -x "${required_binary}" ]]; then
    echo "error: not an executable: ${required_binary}" >&2
    exit 2
  fi
done
if [[ ! -d "${fixture_root}" ]]; then
  echo "error: FASTQ fixture root not found: ${fixture_root}" >&2
  exit 2
fi
if [[ ! -d "${craftmake_catalog}" ]]; then
  echo "error: craftmake catalog not found: ${craftmake_catalog}" >&2
  exit 2
fi

if [[ -z "${work_directory}" ]]; then
  work_directory="$(mktemp -d "${TMPDIR:-/tmp}/otter-e2e-XXXXXX")"
fi
mkdir -p "${work_directory}"
if [[ -z "${artifacts_directory}" ]]; then
  artifacts_directory="${work_directory}/artifacts"
fi
mkdir -p "${artifacts_directory}"
work_directory="$(cd "${work_directory}" && pwd)"
artifacts_directory="$(cd "${artifacts_directory}" && pwd)"
fixture_root="$(cd "${fixture_root}" && pwd)"
craftmake_catalog="$(cd "${craftmake_catalog}" && pwd)"
export OTTER_REFERENCE_ROOT="${work_directory}/registry"

if [[ "${keep_work_directory}" == true ]]; then
  cleanup_work_directory=false
else
  cleanup_work_directory=true
fi
cleanup() {
  if [[ "${cleanup_work_directory}" == true ]]; then
    rm -rf "${work_directory}"
  fi
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# reporting helpers
# ---------------------------------------------------------------------------

passed_stages=0
failed_stages=0
declare -a stage_results=()

log_section() { printf '\n=== %s\n' "$*"; }
log_info() { printf '  %s\n' "$*"; }
log_pass() { printf '  PASS  %s\n' "$*"; }
log_fail() { printf '  FAIL  %s\n' "$*" >&2; }

# capture_stage runs one command and records its evidence, without touching the
# pass/fail counters. Callers decide the verdict so a negative assertion is not
# double counted.
#
# usage: capture_stage <stage-name> <command...>
capture_stage() {
  local stage_name="$1"
  shift
  local stage_directory="${artifacts_directory}/${CURRENT_SCENARIO:-global}/${stage_name}"
  mkdir -p "${stage_directory}"
  printf '%s\n' "$*" > "${stage_directory}/command.txt"
  local exit_code=0
  ( cd "${stage_working_directory:-${work_directory}}" && "$@" ) \
    > "${stage_directory}/stdout.txt" 2> "${stage_directory}/stderr.txt" || exit_code=$?
  printf '%d\n' "${exit_code}" > "${stage_directory}/exit_code.txt"
  return "${exit_code}"
}

stage_artifact_directory() {
  printf '%s/%s/%s' "${artifacts_directory}" "${CURRENT_SCENARIO:-global}" "$1"
}

# run_stage records one expected-success command as evidence.
#
# usage: run_stage <stage-name> <command...>
run_stage() {
  local stage_name="$1"
  shift
  if capture_stage "${stage_name}" "$@"; then
    passed_stages=$((passed_stages + 1))
    stage_results+=("${CURRENT_SCENARIO:-global}/${stage_name}|pass")
    log_pass "${CURRENT_SCENARIO:+${CURRENT_SCENARIO}: }${stage_name}"
    return 0
  fi
  local stage_directory
  stage_directory="$(stage_artifact_directory "${stage_name}")"
  failed_stages=$((failed_stages + 1))
  stage_results+=("${CURRENT_SCENARIO:-global}/${stage_name}|fail")
  log_fail "${CURRENT_SCENARIO:+${CURRENT_SCENARIO}: }${stage_name} (exit $(cat "${stage_directory}/exit_code.txt"))"
  sed -n '1,20p' "${stage_directory}/stderr.txt" >&2 || true
  return 1
}

# expect_failure asserts that a command fails AND that its stderr matches a
# pattern. A silent wrong failure is not an acceptable pass.
#
# usage: expect_failure <stage-name> <pattern> <command...>
expect_failure() {
  local stage_name="$1"
  local pattern="$2"
  shift 2
  if capture_stage "${stage_name}" "$@"; then
    failed_stages=$((failed_stages + 1))
    stage_results+=("${CURRENT_SCENARIO:-global}/${stage_name}|unexpected-pass")
    log_fail "${stage_name} unexpectedly succeeded"
    return 0
  fi
  local stage_directory
  stage_directory="$(stage_artifact_directory "${stage_name}")"
  if ! grep -qF -- "${pattern}" "${stage_directory}/stderr.txt"; then
    failed_stages=$((failed_stages + 1))
    stage_results+=("${CURRENT_SCENARIO:-global}/${stage_name}|wrong-error")
    log_fail "${stage_name} failed without the expected message (${pattern})"
    sed -n '1,10p' "${stage_directory}/stderr.txt" >&2 || true
    return 0
  fi
  passed_stages=$((passed_stages + 1))
  stage_results+=("${CURRENT_SCENARIO:-global}/${stage_name}|expected-failure")
  log_pass "${stage_name} (refused with the expected message)"
  return 0
}

# extract_field reads a scalar from a YAML file, used to assert snapshot content.
extract_field() {
  local file="$1"
  local key="$2"
  awk -v key="${key}" '
    $0 ~ "^[[:space:]]*"key":" {
      sub("^[[:space:]]*"key":[[:space:]]*", "")
      gsub(/^"|"$/, "")
      print
      exit
    }
  ' "${file}"
}

# ---------------------------------------------------------------------------
# reference registry simulation
# ---------------------------------------------------------------------------

ensure_stub_registry() {
  if [[ -n "${stub_registry_binary}" ]]; then
    return 0
  fi
  stub_registry_binary="${work_directory}/stub-registry"
  log_info "building stub-registry"
  ( cd "${repository_root}" && go build -o "${stub_registry_binary}" ./internal/e2esupport/cmd/stub-registry )
}

# install_snakemake_stub writes a stand-in "snakemake" executable.
#
# The Snakemake executor validates the runtime with "snakemake --version" before
# planning, so the compatibility leg needs something on PATH. This stub answers
# that probe and nothing else; it is not Snakemake and it executes no rule.
install_snakemake_stub() {
  snakemake_stub="${work_directory}/stub-runtime/snakemake"
  mkdir -p "$(dirname "${snakemake_stub}")"
  cat > "${snakemake_stub}" <<'STUB'
#!/bin/sh
case "$1" in
  --version) echo "snakemake 7.32.4 (otter e2e stub; not a real runtime)"; exit 0 ;;
  *) exit 0 ;;
esac
STUB
  chmod +x "${snakemake_stub}"
}

# build_reference_registry writes the releases every scenario resolves against.
#
# A release label is mandatory and semantic, not decorative: it pins the assembly
# and the annotation provenance, which is what makes a locked reference
# reproducible. The identifiers below mirror the production registry:
#
#   hg19@GRCh37.p13-gencode-v19
#   hg38@GRCh38-gencode-v44
#   mm10@GRCm38-gencode-M25
#
# stub-registry writes the same on-disk layout and runs the production verifier
# before reporting success.
build_reference_registry() {
  log_section "Simulated reference registry"
  register_release "hg19" "GRCh37.p13-gencode-v19" "Homo sapiens" "GRCh37.p13" "hg19,human,grch37"
  register_release "hg38" "GRCh38-gencode-v44" "Homo sapiens" "GRCh38" "hg38,human,grch38"
  register_release "mm10" "GRCm38-gencode-M25" "Mus musculus" "GRCm38" "mm10,mouse,mm38"
  log_info "registry root: ${OTTER_REFERENCE_ROOT}"
  log_info "note: these are stub releases, not genomes; no genome was downloaded"
}

register_release() {
  local reference_id="$1"
  local release="$2"
  local organism="$3"
  local assembly="$4"
  local alias="$5"
  run_stage "registry-${reference_id}" \
    "${stub_registry_binary}" \
    --registry-root "${OTTER_REFERENCE_ROOT}" \
    --id "${reference_id}" \
    --release "${release}" \
    --organism "${organism}" \
    --assembly "${assembly}" \
    --alias "${alias}"
}

# scenario_reference_selection echoes "<id>@<release>" for a scenario role.
scenario_reference_selection() {
  case "$1" in
    hg19) echo "hg19@GRCh37.p13-gencode-v19" ;;
    hg38) echo "hg38@GRCh38-gencode-v44" ;;
    mm10) echo "mm10@GRCm38-gencode-M25" ;;
    *) echo "unknown reference: $1" >&2; return 1 ;;
  esac
}

# ---------------------------------------------------------------------------
# scenario execution
# ---------------------------------------------------------------------------

# stage_scenario_fastq copies one accession's pair under the <sample>_R{1,2}
# naming that otter create discovers, and writes a matching pdata file.
#
# The copy is deliberate: the resolver records input digests, so a scenario must
# own its input tree to stay independent of its neighbours.
stage_scenario_fastq() {
  local accession="$1"
  local scenario_directory="$2"
  local source_directory="${fixture_root}/${accession}"
  if [[ ! -f "${source_directory}/R1.fastq.gz" || ! -f "${source_directory}/R2.fastq.gz" ]]; then
    echo "error: fixture ${accession} is missing R1/R2 under ${source_directory}" >&2
    return 1
  fi
  mkdir -p "${scenario_directory}/fastq"
  cp "${source_directory}/R1.fastq.gz" "${scenario_directory}/fastq/${accession}_R1.fastq.gz"
  cp "${source_directory}/R2.fastq.gz" "${scenario_directory}/fastq/${accession}_R2.fastq.gz"
  {
    printf 'sampleid,inline_barcode_sequence,condition\n'
    printf '%s,,case\n' "${accession}"
  } > "${scenario_directory}/pdata.csv"
}

run_v1_scenario() {
  local scenario_name="$1"
  local mode="$2"
  local primary_reference="$3"
  local graft_reference="$4"
  local host_reference="$5"
  local accession="$6"

  CURRENT_SCENARIO="${scenario_name}"
  local scenario_directory="${work_directory}/${scenario_name}"
  mkdir -p "${scenario_directory}"
  stage_scenario_fastq "${accession}" "${scenario_directory}"

  log_section "canonical scenario: ${scenario_name} (${mode})"

  stage_working_directory="${work_directory}"
  run_stage "init" "${otter_binary}" init "${scenario_name}"
  stage_working_directory="${scenario_directory}"

  local -a create_arguments=(
    create
    --output "${scenario_directory}"
    --fastq "${scenario_directory}/fastq"
    --pdata "${scenario_directory}/pdata.csv"
    --mode "${mode}"
    --jobid "${scenario_name}"
    --reference-root "${OTTER_REFERENCE_ROOT}"
  )
  if [[ -n "${graft_reference}" ]]; then
    create_arguments+=(--species2 mouse)
    create_arguments+=(--reference-graft "$(scenario_reference_selection "${graft_reference}")")
    create_arguments+=(--reference-host "$(scenario_reference_selection "${host_reference}")")
  else
    create_arguments+=(--reference-primary "$(scenario_reference_selection "${primary_reference}")")
  fi
  run_stage "create" "${otter_binary}" "${create_arguments[@]}"

  run_stage "config-validate" "${otter_binary}" config validate \
    --config "${scenario_directory}/project.yaml" --schema v1

  if ! run_stage "config-resolve" "${otter_binary}" config resolve \
    --project "${scenario_directory}/project.yaml" \
    --reference-root "${OTTER_REFERENCE_ROOT}" \
    --backend local; then
    return 1
  fi
  local run_yaml
  run_yaml="$(tail -n 1 "${artifacts_directory}/${scenario_name}/config-resolve/stdout.txt")"

  # The snapshot is the contract, so assert its shape rather than assuming it.
  local snapshot_schema snapshot_immutable snapshot_executor
  snapshot_schema="$(extract_field "${run_yaml}" "schema_version")"
  snapshot_immutable="$(extract_field "${run_yaml}" "immutable")"
  snapshot_executor="$(awk '/^execution:/{found=1} found && /value:/{print $2; exit}' "${run_yaml}")"
  if [[ "${snapshot_schema}" != "otter.run/v1" || "${snapshot_immutable}" != "true" || "${snapshot_executor}" != "craftmake" ]]; then
    failed_stages=$((failed_stages + 1))
    stage_results+=("${scenario_name}/snapshot-contract|fail")
    log_fail "${scenario_name}: snapshot contract mismatch (schema=${snapshot_schema} immutable=${snapshot_immutable} executor=${snapshot_executor})"
    return 1
  fi
  local file_mode
  file_mode="$(stat -c '%a' "${run_yaml}")"
  if [[ "${file_mode}" != "444" ]]; then
    failed_stages=$((failed_stages + 1))
    stage_results+=("${scenario_name}/snapshot-immutable-mode|fail")
    log_fail "${scenario_name}: run.yaml mode is ${file_mode}, expected 444"
    return 1
  fi
  passed_stages=$((passed_stages + 1))
  stage_results+=("${scenario_name}/snapshot-contract|pass")
  log_pass "${scenario_name}: run.yaml is immutable otter.run/v1 (mode ${file_mode})"

  run_stage "run-dry-run" "${otter_binary}" run \
    --config "${run_yaml}" \
    --executor craftmake \
    --phase step1 \
    --dry-run \
    --foreground \
    --craftmake-binary "${craftmake_binary}" \
    --catalog "${craftmake_catalog}"

  # The dry-run must actually reach Craftmake's planner, not just exit zero.
  if grep -q '"command": *"plan"' "${artifacts_directory}/${scenario_name}/run-dry-run/stdout.txt" \
    && grep -q '"ok": *true' "${artifacts_directory}/${scenario_name}/run-dry-run/stdout.txt"; then
    passed_stages=$((passed_stages + 1))
    stage_results+=("${scenario_name}/craftmake-plan-envelope|pass")
    log_pass "${scenario_name}: craftmake plan envelope received"
  else
    failed_stages=$((failed_stages + 1))
    stage_results+=("${scenario_name}/craftmake-plan-envelope|fail")
    log_fail "${scenario_name}: no successful craftmake plan envelope on stdout"
    return 1
  fi
  return 0
}

# ---------------------------------------------------------------------------
# legacy compatibility leg
# ---------------------------------------------------------------------------

run_legacy_leg() {
  CURRENT_SCENARIO="legacy"
  local scenario_directory="${work_directory}/legacy"
  mkdir -p "${scenario_directory}"
  stage_scenario_fastq "SRR31480456" "${scenario_directory}"

  log_section "legacy compatibility track"

  stage_working_directory="${work_directory}"
  run_stage "init" "${otter_binary}" init "${scenario_directory}/project" --legacy
  stage_working_directory="${scenario_directory}"

  run_stage "create" "${otter_binary}" create \
    --legacy \
    --fastq "${scenario_directory}/fastq" \
    --pdata "${scenario_directory}/pdata.csv" \
    --mode RRBS \
    --output "${scenario_directory}/project/userspace" \
    --jobid legacy-demo

  local legacy_config="${scenario_directory}/project/userspace/legacy-demo/config/otter.yaml"
  run_stage "assets-verify" "${otter_binary}" assets verify \
    --project "${scenario_directory}/project" --strict
  run_stage "config-validate-legacy" "${otter_binary}" config validate \
    --config "${legacy_config}" --schema legacy

  # The bridge the handoff found broken. Migration produces project INTENT
  # (project.yaml, samples.tsv, references.lock.yaml) but not the pinned asset
  # skeleton, so it must be composed with a canonical project root to be
  # resolvable. This mirrors the documented flow.
  local migrated_root="${scenario_directory}/migrated-project"
  stage_working_directory="${work_directory}"
  run_stage "init-migrated-root" "${otter_binary}" init "${migrated_root}"
  stage_working_directory="${scenario_directory}"
  run_stage "config-migrate" "${otter_binary}" config migrate \
    --input "${legacy_config}" \
    --output "${migrated_root}/project.yaml" \
    --reference-primary "hg19@GRCh37.p13-gencode-v19" \
    --reference-root "${OTTER_REFERENCE_ROOT}"
  run_stage "config-validate-migrated" "${otter_binary}" config validate \
    --config "${migrated_root}/project.yaml" --schema v1
  if run_stage "config-resolve-migrated" "${otter_binary}" config resolve \
    --project "${migrated_root}/project.yaml" \
    --reference-root "${OTTER_REFERENCE_ROOT}" \
    --backend local; then
    local migrated_run_yaml
    migrated_run_yaml="$(tail -n 1 "${artifacts_directory}/legacy/config-resolve-migrated/stdout.txt")"

    # The migrated snapshot must select snakemake, not craftmake: the legacy
    # adapter maps the compatibility track onto the Snakemake executor. This is
    # the "legacy config pairs with snakemake" half of the contract, asserted
    # from the snapshot itself.
    local migrated_executor
    migrated_executor="$(awk '/^execution:/{found=1} found && /^ *value:/{print $2; exit}' "${migrated_run_yaml}")"
    if [[ "${migrated_executor}" == "snakemake" ]]; then
      passed_stages=$((passed_stages + 1))
      stage_results+=("legacy/migrated-snapshot-executor|pass")
      log_pass "legacy: migrated snapshot selects snakemake"
    else
      failed_stages=$((failed_stages + 1))
      stage_results+=("legacy/migrated-snapshot-executor|fail")
      log_fail "legacy: migrated snapshot selects ${migrated_executor}, expected snakemake"
    fi

    # Craftmake must refuse a snakemake-selected snapshot, and the snakemake
    # executor must accept it. The snakemake dry-run uses a stub runtime on PATH
    # (see scope-and-limits): it asserts Otter-side dispatch, configuration
    # projection and planning, not Snakemake execution.
    expect_failure "migrated-snapshot-with-craftmake" "resolves executor" \
      "${otter_binary}" run --config "${migrated_run_yaml}" --executor craftmake \
      --phase step1 --dry-run --foreground --craftmake-binary "${craftmake_binary}" \
      --catalog "${craftmake_catalog}"

    local previous_path="${PATH}"
    PATH="$(dirname "${snakemake_stub}"):${PATH}" run_stage "run-snakemake-dry-run-stub" \
      "${otter_binary}" run --config "${migrated_run_yaml}" --executor snakemake --dry-run --foreground
    PATH="${previous_path}"
    if grep -q "All workflow steps completed successfully" \
      "${artifacts_directory}/legacy/run-snakemake-dry-run-stub/stdout.txt" 2>/dev/null; then
      passed_stages=$((passed_stages + 1))
      stage_results+=("legacy/snakemake-planned-all-steps|pass")
      log_pass "legacy: snakemake executor planned every step from the snapshot"
    else
      failed_stages=$((failed_stages + 1))
      stage_results+=("legacy/snakemake-planned-all-steps|fail")
      log_fail "legacy: snakemake dry-run did not plan every step"
    fi
  fi

  # Cross-track guards. Each guard must fail for the right reason, so the
  # expected message is asserted rather than merely a non-zero exit.
  #
  # 1. Canonical create refuses a legacy project directory.
  expect_failure "guard-canonical-create-on-legacy" "legacy compatibility project" \
    "${otter_binary}" create --output "${scenario_directory}/project" \
    --fastq "${scenario_directory}/fastq" --mode RRBS --jobid mixed \
    --reference-primary "hg19@GRCh37.p13-gencode-v19"
  # 2. Legacy init refuses a canonical project directory. Use the canonical
  #    scenario created earlier rather than the legacy one.
  expect_failure "guard-legacy-init-on-canonical" "canonical v1 project" \
    "${otter_binary}" init "${work_directory}/rrbs" --legacy

  run_executor_pairing_matrix "${legacy_config}"
}

# run_executor_pairing_matrix demonstrates the executor contract directly.
#
# The immutable run.yaml is the single execution boundary for both executors: the
# snapshot selects its executor, and `--executor` must agree with that selection.
# A legacy otter.yaml is an authoring format only, so it is refused by both
# executors until it has been migrated and resolved.
run_executor_pairing_matrix() {
  local legacy_config="$1"
  log_section "executor pairing contract"

  local rrbs_snapshot
  rrbs_snapshot="$(tail -n 1 "${artifacts_directory}/rrbs/config-resolve/stdout.txt" 2>/dev/null || true)"
  if [[ -z "${rrbs_snapshot}" || ! -f "${rrbs_snapshot}" ]]; then
    log_fail "rrbs snapshot unavailable; cannot assert the executor pairing contract"
    failed_stages=$((failed_stages + 1))
    stage_results+=("pairing/setup|fail")
    return 0
  fi

  # 1. A craftmake snapshot is accepted by craftmake (covered in the scenario leg)
  #    and refused by the snakemake executor, because the snapshot selects
  #    craftmake.
  expect_failure "pairing-craftmake-snapshot-with-snakemake" "resolves executor" \
    "${otter_binary}" run --config "${rrbs_snapshot}" --executor snakemake --dry-run --foreground
  # 2. A legacy otter.yaml is refused by the craftmake executor with migration
  #    guidance.
  expect_failure "pairing-legacy-config-with-craftmake" "legacy compatibility configuration" \
    "${otter_binary}" run --config "${legacy_config}" --executor craftmake --dry-run --foreground
  # 3. The same legacy otter.yaml is also refused by the snakemake executor: the
  #    compatibility executor consumes the snapshot, not the legacy file.
  expect_failure "pairing-legacy-config-with-snakemake" "legacy compatibility configuration" \
    "${otter_binary}" run --config "${legacy_config}" --executor snakemake --dry-run --foreground
}

# ---------------------------------------------------------------------------
# otter build leg
# ---------------------------------------------------------------------------

# run_build_leg exercises "otter build" against the manual init/create/resolve
# chain, for every scenario the v1 leg already covers.
#
# The comparison is the whole point. "otter build" is a shortcut over three
# existing commands, so its contract is not "it worked" but "it produced the
# same project the three commands produce". Each scenario therefore authorises
# the same inputs twice, once per path, and compares the authoring artifacts
# byte for byte.
run_build_leg() {
  local comparison_failures=0

  for scenario_name in ${scenarios//,/ }; do
    case "${scenario_name}" in
      rrbs)   build_scenario "rrbs"    "RRBS"   "hg19" ""     ""     "SRR31480456" ;;
      rnaseq) build_scenario "rnaseq"  "RNASEQ" "hg38" ""     ""     "SRR018258"   ;;
      bs-pdx) build_scenario "bs-pdx"  "RRBS"   ""     "hg38" "mm10" "SRR36187610" ;;
      rna-pdx) build_scenario "rna-pdx" "RNASEQ" ""    "hg38" "mm10" "SRR30880970" ;;
      *) echo "error: unknown scenario ${scenario_name}" >&2; exit 2 ;;
    esac || comparison_failures=$((comparison_failures + 1))
  done

  if [[ "${comparison_failures}" -eq 0 ]]; then
    passed_stages=$((passed_stages + 1))
    stage_results+=("build/all-scenarios-match-manual|pass")
    log_pass "build: every scenario matches the manual chain"
  else
    failed_stages=$((failed_stages + 1))
    stage_results+=("build/all-scenarios-match-manual|fail")
    log_fail "build: ${comparison_failures} scenario(s) diverged from the manual chain"
  fi
}

# build_scenario runs both authoring paths for one scenario and compares them.
build_scenario() {
  local scenario_name="$1"
  local mode="$2"
  local primary_reference="$3"
  local graft_reference="$4"
  local host_reference="$5"
  local accession="$6"

  CURRENT_SCENARIO="build-${scenario_name}"
  local scenario_directory="${work_directory}/build-${scenario_name}"
  local manual_root="${scenario_directory}/manual"
  local build_root="${scenario_directory}/build"

  # Both paths read the SAME input tree. This is required for the comparison to
  # mean anything: samples.tsv records each input path relative to its own
  # project root, so two sibling project roots over one input tree record the
  # same relative path, while two separate input trees would record different
  # ones and the manifest would differ for a reason that has nothing to do with
  # the authoring code. The digests are content-addressed, so sharing the bytes
  # does not make the two runs indistinguishable either.
  stage_scenario_fastq "${accession}" "${scenario_directory}"

  log_section "build comparison: ${scenario_name} (${mode})"

  # The shared reference arguments, so the two paths cannot differ by flags.
  local -a reference_arguments=()
  if [[ -n "${graft_reference}" ]]; then
    reference_arguments+=(--reference-graft "$(scenario_reference_selection "${graft_reference}")")
    reference_arguments+=(--reference-host "$(scenario_reference_selection "${host_reference}")")
  else
    reference_arguments+=(--reference-primary "$(scenario_reference_selection "${primary_reference}")")
  fi

  # --- path 1: the manual chain -------------------------------------------
  stage_working_directory="${work_directory}"
  run_stage "manual-init" "${otter_binary}" init "${manual_root}"
  stage_working_directory="${scenario_directory}"
  run_stage "manual-create" "${otter_binary}" create \
    --output "${manual_root}" \
    --fastq "${scenario_directory}/fastq" \
    --pdata "${scenario_directory}/pdata.csv" \
    --mode "${mode}" \
    --jobid "${scenario_name}" \
    --reference-root "${OTTER_REFERENCE_ROOT}" \
    "${reference_arguments[@]}"
  run_stage "manual-validate" "${otter_binary}" config validate \
    --config "${manual_root}/project.yaml" --schema v1
  run_stage "manual-resolve" "${otter_binary}" config resolve \
    --project "${manual_root}/project.yaml" \
    --reference-root "${OTTER_REFERENCE_ROOT}" \
    --backend local

  # --- path 2: otter build -------------------------------------------------
  run_stage "build" "${otter_binary}" build \
    --project-root "${build_root}" \
    --fastq "${scenario_directory}/fastq" \
    --pdata "${scenario_directory}/pdata.csv" \
    --mode "${mode}" \
    --jobid "${scenario_name}" \
    --reference-root "${OTTER_REFERENCE_ROOT}" \
    --backend local \
    "${reference_arguments[@]}"

  # --- comparison ----------------------------------------------------------
  if compare_authoring_artifacts "${scenario_name}" "${manual_root}" "${build_root}"; then
    passed_stages=$((passed_stages + 1))
    stage_results+=("build-${scenario_name}/artifacts-match|pass")
    log_pass "build-${scenario_name}: authoring artifacts match the manual chain"
  else
    failed_stages=$((failed_stages + 1))
    stage_results+=("build-${scenario_name}/artifacts-match|fail")
    log_fail "build-${scenario_name}: authoring artifacts differ from the manual chain"
    return 1
  fi

  # The executor default is the other half of the contract: the shortcut must
  # select craftmake without being told, because that is the execution layer
  # under development.
  local build_snapshot
  build_snapshot="$(tail -n 1 "${artifacts_directory}/build-${scenario_name}/build/stdout.txt")"
  if [[ ! -f "${build_snapshot}" ]]; then
    failed_stages=$((failed_stages + 1))
    stage_results+=("build-${scenario_name}/snapshot-written|fail")
    log_fail "build-${scenario_name}: no readable snapshot at ${build_snapshot}"
    return 1
  fi
  local build_executor build_schema build_immutable
  build_executor="$(awk '/^execution:/{found=1} found && /value:/{print $2; exit}' "${build_snapshot}")"
  build_schema="$(extract_field "${build_snapshot}" "schema_version")"
  build_immutable="$(extract_field "${build_snapshot}" "immutable")"
  if [[ "${build_executor}" != "craftmake" || "${build_schema}" != "otter.run/v1" || "${build_immutable}" != "true" ]]; then
    failed_stages=$((failed_stages + 1))
    stage_results+=("build-${scenario_name}/snapshot-contract|fail")
    log_fail "build-${scenario_name}: snapshot contract mismatch (schema=${build_schema} immutable=${build_immutable} executor=${build_executor})"
    return 1
  fi
  passed_stages=$((passed_stages + 1))
  stage_results+=("build-${scenario_name}/snapshot-contract|pass")
  log_pass "build-${scenario_name}: snapshot is immutable otter.run/v1 selecting craftmake"

  # The snapshot must be usable by the executor boundary, not merely present.
  run_stage "build-run-dry-run" "${otter_binary}" run \
    --config "${build_snapshot}" \
    --executor craftmake \
    --phase step1 \
    --dry-run \
    --foreground \
    --craftmake-binary "${craftmake_binary}" \
    --catalog "${craftmake_catalog}"

  if grep -q '"command": *"plan"' "${artifacts_directory}/build-${scenario_name}/build-run-dry-run/stdout.txt" \
    && grep -q '"ok": *true' "${artifacts_directory}/build-${scenario_name}/build-run-dry-run/stdout.txt"; then
    passed_stages=$((passed_stages + 1))
    stage_results+=("build-${scenario_name}/craftmake-plan-envelope|pass")
    log_pass "build-${scenario_name}: craftmake plan envelope received"
  else
    failed_stages=$((failed_stages + 1))
    stage_results+=("build-${scenario_name}/craftmake-plan-envelope|fail")
    log_fail "build-${scenario_name}: no successful craftmake plan envelope on stdout"
    return 1
  fi

  return 0
}

# compare_authoring_artifacts asserts the two authoring paths agree.
#
# Only the authoring artifacts are compared. The snapshots are not, because a
# snapshot embeds its own run directory and creation timestamp; the comparison
# that matters is the project intent, the samples, and the locked reference
# digest, which must be identical or the two paths disagree about what a
# project is.
compare_authoring_artifacts() {
  local scenario_name="$1"
  local manual_root="$2"
  local build_root="$3"
  local identical=0

  for artifact in project.yaml samples.tsv references.lock.yaml; do
    if [[ ! -f "${manual_root}/${artifact}" ]]; then
      log_fail "build-${scenario_name}: manual chain did not write ${artifact}"
      return 1
    fi
    if [[ ! -f "${build_root}/${artifact}" ]]; then
      log_fail "build-${scenario_name}: otter build did not write ${artifact}"
      return 1
    fi
    if ! diff -q "${manual_root}/${artifact}" "${build_root}/${artifact}" > /dev/null; then
      log_fail "build-${scenario_name}: ${artifact} differs from the manual chain"
      diff "${manual_root}/${artifact}" "${build_root}/${artifact}" | sed -n '1,10p' >&2 || true
      identical=1
    fi
  done

  [[ "${identical}" -eq 0 ]]
}

run_install_leg() {
  CURRENT_SCENARIO="install"
  log_section "otter-install plan (no network)"
  if [[ -z "${installer_binary}" ]]; then
    log_info "skipped: --installer was not supplied"
    stage_results+=("install/otter-install-dry-run|skipped")
    return 0
  fi
  stage_working_directory="${work_directory}"
  # --version avoids the GitHub API tag lookup; --dry-run keeps every download
  # and environment mutation off, so this leg is network-free.
  run_stage "otter-install-dry-run" "${installer_binary}" \
    --dry-run --non-interactive --skip-envs \
    --install-dir "${work_directory}/install" \
    --version v0.0.0-e2e
  if grep -q "DRY-RUN" "${artifacts_directory}/install/otter-install-dry-run/stdout.txt"; then
    passed_stages=$((passed_stages + 1))
    stage_results+=("install/no-network|pass")
    log_pass "installer reported a dry-run plan"
  else
    failed_stages=$((failed_stages + 1))
    stage_results+=("install/no-network|fail")
    log_fail "installer output did not confirm dry-run mode"
  fi
}

# ---------------------------------------------------------------------------
# site profile leg
# ---------------------------------------------------------------------------

# run_site_profile_leg proves that a generated site profile is real
# configuration, not a file nobody reads.
#
# The decisive assertion is the pair below: resolving the project WITHOUT a
# reference root fails, and resolving it WITH the generated profile succeeds
# because the profile supplies the reference root. A profile that were ignored
# could not change that outcome.
run_site_profile_leg() {
  CURRENT_SCENARIO="site"
  log_section "site profile generation"

  # A work-local HOME keeps the default locator inside the work directory, so
  # generate/list/validate all exercise the real default path.
  local site_home="${work_directory}/site-home"
  local site_id="e2e-local"
  mkdir -p "${site_home}/.config/otter/sites"
  stage_working_directory="${work_directory}"

  run_stage "generate" env "HOME=${site_home}" \
    "${otter_binary}" site generate \
    --id "${site_id}" \
    --reference-root "${OTTER_REFERENCE_ROOT}"

  local profile_path="${site_home}/.config/otter/sites/${site_id}.yaml"
  if [[ -f "${profile_path}" ]]; then
    passed_stages=$((passed_stages + 1))
    stage_results+=("site/profile-at-default-locator|pass")
    log_pass "site: profile written to the default locator path"
  else
    failed_stages=$((failed_stages + 1))
    stage_results+=("site/profile-at-default-locator|fail")
    log_fail "site: no profile at ${profile_path}"
  fi

  # The generated profile must be loadable by the loader that will read it.
  run_stage "list" env "HOME=${site_home}" "${otter_binary}" site list
  if grep -q "${site_id}" "${artifacts_directory}/site/list/stdout.txt" 2>/dev/null; then
    passed_stages=$((passed_stages + 1))
    stage_results+=("site/profile-is-listed|pass")
    log_pass "site: generated profile is discoverable"
  else
    failed_stages=$((failed_stages + 1))
    stage_results+=("site/profile-is-listed|fail")
    log_fail "site: generated profile was not listed"
  fi

  # Validating the named profile must report THAT profile, not the machine's own
  # detection.
  run_stage "validate" env "HOME=${site_home}" "${otter_binary}" site validate "${site_id}"
  if grep -q "Site:    ${site_id}" "${artifacts_directory}/site/validate/stdout.txt" 2>/dev/null \
    && grep -q "Source:  profile" "${artifacts_directory}/site/validate/stdout.txt" 2>/dev/null; then
    passed_stages=$((passed_stages + 1))
    stage_results+=("site/validate-reads-the-named-profile|pass")
    log_pass "site: validate reports the named profile"
  else
    failed_stages=$((failed_stages + 1))
    stage_results+=("site/validate-reads-the-named-profile|fail")
    log_fail "site: validate did not report the named profile"
    sed -n '1,10p' "${artifacts_directory}/site/validate/stdout.txt" >&2 || true
  fi

  # Guards: an existing profile is not overwritten, and policy fields are not
  # invented.
  expect_failure "guard-overwrite-refused" "already exists" \
    env "HOME=${site_home}" "${otter_binary}" site generate \
    --id "${site_id}" --reference-root "${OTTER_REFERENCE_ROOT}"
  expect_failure "guard-relative-reference-root" "must be an absolute path" \
    env "HOME=${site_home}" "${otter_binary}" site generate \
    --id other --reference-root relative/refs
  expect_failure "guard-slurm-needs-partition" "requires --partition" \
    env "HOME=${site_home}" "${otter_binary}" site generate \
    --id prod --backend slurm --reference-root "${OTTER_REFERENCE_ROOT}"

  # Consumption proof. The rrbs project resolved earlier needs a reference root;
  # the profile is the only thing supplying it in the second command.
  local rrbs_project="${work_directory}/rrbs/project.yaml"
  if [[ ! -f "${rrbs_project}" ]]; then
    log_info "site: rrbs project unavailable; skipping the profile-consumption proof"
    stage_results+=("site/resolve-without-reference-root|skipped")
    stage_results+=("site/resolve-with-profile|skipped")
    return 0
  fi

  expect_failure "resolve-without-reference-root" "reference root" \
    env "HOME=${site_home}" "${otter_binary}" config resolve \
    --project "${rrbs_project}" --backend local

  if run_stage "resolve-with-profile" env "HOME=${site_home}" "${otter_binary}" config resolve \
    --project "${rrbs_project}" --site "${site_id}" --backend local; then
    local profile_run_yaml
    profile_run_yaml="$(tail -n 1 "${artifacts_directory}/site/resolve-with-profile/stdout.txt")"
    if grep -q "${OTTER_REFERENCE_ROOT}" "${profile_run_yaml}" 2>/dev/null; then
      passed_stages=$((passed_stages + 1))
      stage_results+=("site/profile-supplied-the-reference-root|pass")
      log_pass "site: snapshot embeds the reference root from the profile"
    else
      failed_stages=$((failed_stages + 1))
      stage_results+=("site/profile-supplied-the-reference-root|fail")
      log_fail "site: snapshot does not reference the profile's reference root"
    fi
  fi
}

# ---------------------------------------------------------------------------
# main
# ---------------------------------------------------------------------------

CURRENT_SCENARIO=""
stage_working_directory="${work_directory}"

log_section "OTTER end-to-end rehearsal"
log_info "otter:    ${otter_binary}"
log_info "craftmake: ${craftmake_binary}"
log_info "work dir: ${work_directory}"
log_info "scenarios: ${scenarios}"

# ---------------------------------------------------------------------------
# relative input leg
# ---------------------------------------------------------------------------

# run_relative_input_leg pins the authoring convention the manual teaches.
#
# "otter create" reads --fastq relative to the working directory, while every
# later reader anchors a relative path in samples.tsv to the project root. When
# the FASTQ directory sits OUTSIDE the project root those two readings disagree,
# and create can report success for a project that "otter config resolve" then
# rejects with "no such file or directory".
#
# The scenario legs above always pass absolute paths, so they cannot detect this.
# This leg deliberately reproduces the manual: a working directory holding
# fastq/ and pdata.csv, and a sibling project directory created by "otter init".
run_relative_input_leg() {
  CURRENT_SCENARIO="relative-inputs"
  log_section "relative input paths"

  local leg_directory="${work_directory}/relative-inputs"
  mkdir -p "${leg_directory}/data-source"
  stage_scenario_fastq "SRR018258" "${leg_directory}/data-source"

  # Work from a directory that is NOT the project root, which is what makes the
  # two interpretations of "relative" diverge.
  local working_directory="${leg_directory}/working"
  mkdir -p "${working_directory}"
  cp -r "${leg_directory}/data-source/fastq" "${working_directory}/fastq"
  cp "${leg_directory}/data-source/pdata.csv" "${working_directory}/pdata.csv"

  stage_working_directory="${working_directory}"
  run_stage "init" "${otter_binary}" init project

  run_stage "create-relative" "${otter_binary}" create \
    --output project \
    --fastq ./fastq \
    --pdata ./pdata.csv \
    --mode RRBS \
    --jobid relative-demo \
    --reference-root "${OTTER_REFERENCE_ROOT}" \
    --reference-primary "$(scenario_reference_selection hg19)"

  # create must have anchored the recorded inputs so that the path resolves when
  # read the way the resolver reads it: relative to the project root.
  local recorded_unresolved=0
  local recorded_path
  while IFS= read -r recorded_path; do
    [[ -z "${recorded_path}" ]] && continue
    case "${recorded_path}" in
      /*) ;;
      *) recorded_path="${working_directory}/project/${recorded_path}" ;;
    esac
    if [[ ! -f "${recorded_path}" ]]; then
      recorded_unresolved=$((recorded_unresolved + 1))
    fi
  done < <(awk -F'\t' 'NR>1 {print $2; print $3}' "${working_directory}/project/samples.tsv" 2>/dev/null)

  if [[ "${recorded_unresolved}" -eq 0 ]]; then
    passed_stages=$((passed_stages + 1))
    stage_results+=("relative-inputs/recorded-inputs-resolve|pass")
    log_pass "relative-inputs: recorded inputs resolve under the project root"
  else
    failed_stages=$((failed_stages + 1))
    stage_results+=("relative-inputs/recorded-inputs-resolve|fail")
    log_fail "relative-inputs: ${recorded_unresolved} recorded input(s) do not resolve under the project root"
  fi

  # And the decisive consequence: resolve consumes what create wrote.
  run_stage "config-resolve" "${otter_binary}" config resolve \
    --project project/project.yaml \
    --reference-root "${OTTER_REFERENCE_ROOT}" \
    --backend local

  stage_working_directory="${work_directory}"
}

ensure_stub_registry
install_snakemake_stub
build_reference_registry
run_install_leg

for scenario_name in ${scenarios//,/ }; do
  case "${scenario_name}" in
    rrbs) run_v1_scenario "rrbs" "RRBS" "hg19" "" "" "SRR31480456" || true ;;
    rnaseq) run_v1_scenario "rnaseq" "RNASEQ" "hg38" "" "" "SRR018258" || true ;;
    bs-pdx) run_v1_scenario "bs-pdx" "RRBS" "" "hg38" "mm10" "SRR36187610" || true ;;
    rna-pdx) run_v1_scenario "rna-pdx" "RNASEQ" "" "hg38" "mm10" "SRR30880970" || true ;;
    *) echo "error: unknown scenario ${scenario_name}" >&2; exit 2 ;;
  esac
done

# The site leg runs after the scenarios because its decisive assertion resolves
# a scenario project using the generated profile.
run_site_profile_leg

if [[ "${run_legacy_leg}" == true ]]; then
  run_legacy_leg || true
fi

# The build leg runs after the scenarios so it reuses the registry they built,
# and before the relative-input leg because it is an authoring contract rather
# than a convention check.
if [[ "${run_build_leg_flag}" == true ]]; then
  run_build_leg || true
fi

# The relative-input leg runs last because it is an authoring convention check,
# not a scenario.
run_relative_input_leg

CURRENT_SCENARIO=""
log_section "summary"
log_info "stages passed: ${passed_stages}"
log_info "stages failed: ${failed_stages}"
log_info "evidence:      ${artifacts_directory}"

# Emit a machine-readable stage table for the workflow briefing.
{
  printf 'stage|result\n'
  for stage_result in "${stage_results[@]}"; do
    printf '%s\n' "${stage_result}"
  done
} > "${artifacts_directory}/stage-results.tsv"

# Explicit limits, written into the evidence so a reader cannot mistake this for
# a scientific or performance qualification.
cat > "${artifacts_directory}/scope-and-limits.txt" <<'LIMITS'
This rehearsal proves contract compatibility, not scientific correctness.

- Reference genomes: the registry is simulated by stub-registry. It reproduces
  the directory layout, reference.yaml, manifest.json, and digest relationships
  that "otter reference build" produces, and it is verified by the production
  reference verifier. No genome was downloaded and no index builder ran.
  Release identifiers mirror the production registry, where a release is
  <assembly>-<annotation-source>-<annotation-version> (for example
  hg19@GRCh37.p13-gencode-v19). A release label is mandatory and semantic: it
  pins the assembly and annotation provenance.
- Index contents: stub index files are placeholders. They satisfy the reference
  contract; they cannot and do not align reads.
- Craftmake: the dry-run leg stops at "craftmake plan". No workflow task
  executed, so no scientific output exists.
- Snakemake: the compatibility leg runs with a STUB "snakemake" executable on
  PATH that answers only "snakemake --version". It therefore proves Otter-side
  dispatch, snapshot projection, resource resolution, and workflow planning. It
  executes no rule and produces no scientific output. In production this leg
  needs a real enva/conda runtime.
- Known limitation exposed by that leg: a project migrated into an "otter init"
  root does not carry the ".snakemake" entry points where the compatibility
  executor looks for them, so it falls back to the workflow name and logs
  "Snakemake file not found in current directory or otter-project/". Real
  compatibility execution still needs the legacy asset layout.
- Site profiles: "otter site generate" runs, and the profile it writes is
  discovered through the default locator directory (a work-local HOME keeps
  ~/.config/otter/sites inside the work directory), then listed, validated by
  name, and consumed by "otter config resolve" to supply the reference root.
  Not covered here: real SLURM cluster probing (scontrol, sinfo, sacctmgr,
  compute-node path checks) and machine-level /etc/otter/sites. This rehearsal
  forces --backend local, so the generated profile is always a local one.
- Inputs: downsampled FASTQ fixtures (20,000 synchronized pairs per accession).
  Absolute digests are recorded in each run.yaml.
- The relative-input leg covers the authoring convention only: it asserts that
  "otter create" records input paths that resolve the way later commands read
  them. It stops at "config resolve" and runs no workflow.
- The install leg runs otter-install --dry-run only; it never downloads release
  binaries or creates environments.
LIMITS

if [[ "${failed_stages}" -gt 0 ]]; then
  log_fail "end-to-end rehearsal failed"
  exit 1
fi
log_pass "end-to-end rehearsal passed"
