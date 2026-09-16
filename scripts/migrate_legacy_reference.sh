#!/usr/bin/env bash
# =============================================================================
#  migrate_legacy_reference.sh
#  Migrates legacy reference genome assets from $beaverhome/inst into the
#  canonical immutable Otter Reference Registry ($OTTER_REFERENCE_ROOT).
#
#  Usage:
#    bash scripts/migrate_legacy_reference.sh [OPTIONS]
#
#  Options:
#    --legacy-inst PATH    Path to legacy inst directory (default: $beaverhome/inst)
#    --species NAME        Species to migrate: hg19, mm10, or all (default: all)
#    --registry-root PATH  Target Reference Registry root (default: $OTTER_REFERENCE_ROOT or ~/.otter/references)
#    --mode MODE           Migration mode: rebuild (recommended) or ingest (default: rebuild)
#    --threads NUM         Build threads for indexing in rebuild mode (default: 4)
#    --otter-core-bin PATH Path to otter-core environment bin/ (default: auto-detect via enva or conda)
#    --otter-bin PATH      Path to otter executable (default: auto-detect on PATH or ~/.cargo/bin/otter)
#    --dry-run             Print planned actions without modifying the registry
#    --help                Show this help message
#
#  Example:
#    bash scripts/migrate_legacy_reference.sh \
#      --legacy-inst /data_center_02/project/SR/zhengyanhua/beaverflow1016/inst \
#      --species hg19 \
#      --registry-root /data_center_02/project/SR/zhengyanhua/otter0907/references \
#      --mode rebuild \
#      --threads 8
# =============================================================================

set -euo pipefail

LEGACY_INST="${beaverhome:-}/inst"
SPECIES_CHOICE="all"
REGISTRY_ROOT="${OTTER_REFERENCE_ROOT:-$HOME/.otter/references}"
MIGRATION_MODE="rebuild"
THREADS=4
OTTER_CORE_BIN=""
OTTER_BIN=""
DRY_RUN=false

log_info()    { printf '  \033[1m[INFO]\033[0m  %s\n' "$*"; }
log_success() { printf '  \033[0;32m✓\033[0m %s\n' "$*"; }
log_warn()    { printf '  \033[1;33m⚠\033[0m  %s\n' "$*"; }
log_error()   { printf '  \033[0;31m✗\033[0m  %s\n' "$*" >&2; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --legacy-inst)    LEGACY_INST="$2"; shift 2 ;;
    --species)        SPECIES_CHOICE="$2"; shift 2 ;;
    --registry-root)  REGISTRY_ROOT="$2"; shift 2 ;;
    --mode)           MIGRATION_MODE="$2"; shift 2 ;;
    --threads)        THREADS="$2"; shift 2 ;;
    --otter-core-bin) OTTER_CORE_BIN="$2"; shift 2 ;;
    --otter-bin)      OTTER_BIN="$2"; shift 2 ;;
    --dry-run)        DRY_RUN=true; shift ;;
    --help|-h)
      sed -n '2,24p' "$0" | sed 's/^# *//'
      exit 0
      ;;
    *)
      log_error "Unknown option: $1"
      exit 2
      ;;
  esac
done

if [[ -z "$LEGACY_INST" || ! -d "$LEGACY_INST" ]]; then
  log_error "Legacy inst directory not found: '$LEGACY_INST'. Specify --legacy-inst PATH."
  exit 1
fi

if [[ -z "$OTTER_BIN" ]]; then
  if command -v otter >/dev/null 2>&1; then
    OTTER_BIN="$(command -v otter)"
  elif [[ -x "$HOME/.cargo/bin/otter" ]]; then
    OTTER_BIN="$HOME/.cargo/bin/otter"
  else
    log_error "otter executable not found. Specify --otter-bin PATH."
    exit 1
  fi
fi

if [[ -z "$OTTER_CORE_BIN" ]]; then
  for candidate in \
    "$HOME/miniconda3/envs/otter-core/bin" \
    "$HOME/.conda/envs/otter-core/bin" \
    "/opt/conda/envs/otter-core/bin"; do
    if [[ -d "$candidate" && -x "$candidate/samtools" ]]; then
      OTTER_CORE_BIN="$candidate"
      break
    fi
  done
  if [[ -z "$OTTER_CORE_BIN" ]] && command -v enva >/dev/null 2>&1; then
    prefix="$(enva list 2>/dev/null | awk '$1=="otter-core"{print $NF}' || true)"
    if [[ -n "$prefix" && -d "$prefix/bin" ]]; then
      OTTER_CORE_BIN="$prefix/bin"
    fi
  fi
fi

if [[ -z "$OTTER_CORE_BIN" || ! -d "$OTTER_CORE_BIN" ]]; then
  log_error "otter-core environment bin/ not found. Specify --otter-core-bin PATH."
  exit 1
fi

SAMTOOLS="$OTTER_CORE_BIN/samtools"
BOWTIE2_BUILD="$OTTER_CORE_BIN/bowtie2-build"
BISMARK_PREP="$OTTER_CORE_BIN/bismark_genome_preparation"
STAR_BIN="$OTTER_CORE_BIN/STAR"

for required_tool in "$SAMTOOLS" "$BOWTIE2_BUILD" "$BISMARK_PREP"; do
  if [[ ! -x "$required_tool" ]]; then
    log_error "Required tool not executable: $required_tool"
    exit 1
  fi
done

printf '\033[1m=================================================================\033[0m\n'
printf '\033[1m  Otter Legacy Reference Genome Migration Tool\033[0m\n'
printf '\033[1m=================================================================\033[0m\n'
log_info "Legacy inst root: $LEGACY_INST"
log_info "Target registry : $REGISTRY_ROOT"
log_info "Migration mode  : $MIGRATION_MODE"
log_info "Species scope   : $SPECIES_CHOICE"
log_info "Build threads   : $THREADS"
log_info "otter CLI       : $OTTER_BIN"
log_info "otter-core bin  : $OTTER_CORE_BIN"
if [[ "$DRY_RUN" = true ]]; then
  log_warn "DRY-RUN mode enabled - no files will be written"
fi
printf '\n'

# ── Migration routine for one species release ────────────────────────────────
migrate_species() {
  local ref_id="$1"
  local release="$2"
  local organism="$3"
  local assembly="$4"
  local fasta_path="$5"
  local gtf_path="$6"
  local has_star="$7"

  printf '\033[1m-----------------------------------------------------------------\033[0m\n'
  printf '\033[1m  Migrating %s (%s, release: %s)\033[0m\n' "$ref_id" "$organism" "$release"
  printf '\033[1m-----------------------------------------------------------------\033[0m\n'

  local target_release_dir="$REGISTRY_ROOT/genomes/$ref_id/$release"
  if [[ -d "$target_release_dir" ]]; then
    log_warn "Release already exists at $target_release_dir; skipping migration."
    return 0
  fi

  if [[ ! -f "$fasta_path" ]]; then
    log_error "FASTA file missing: $fasta_path"
    return 1
  fi
  if [[ ! -f "$gtf_path" ]]; then
    log_error "GTF file missing: $gtf_path"
    return 1
  fi

  log_info "Source FASTA: $fasta_path ($(du -h "$fasta_path" | cut -f1))"
  log_info "Source GTF  : $gtf_path ($(du -h "$gtf_path" | cut -f1))"

  local indexes_flag="bismark,bowtie2"
  if [[ "$has_star" == "true" && -x "$STAR_BIN" ]]; then
    indexes_flag="bismark,bowtie2,star"
  fi
  log_info "Target indexes: $indexes_flag"

  if [[ "$DRY_RUN" = true ]]; then
    if [[ "$MIGRATION_MODE" == "ingest" ]]; then
      log_info "[DRY-RUN] Would ingest existing legacy indexes into: $target_release_dir"
      log_info "[DRY-RUN]   - Link FASTA: $fasta_path -> $target_release_dir/fasta/${ref_id}.fa"
      log_info "[DRY-RUN]   - Generate FAI with: $SAMTOOLS faidx"
      log_info "[DRY-RUN]   - Link GTF: $gtf_path -> $target_release_dir/annotations/${ref_id}.gtf"
      log_info "[DRY-RUN]   - Link Bismark index from: $(dirname "$fasta_path")/Bisulfite_Genome"
      log_info "[DRY-RUN]   - Link STAR index from: $(dirname "$gtf_path")"
      log_info "[DRY-RUN]   - Generate reference.yaml, manifest.json, checksums.sha256, and seal read-only"
      return 0
    fi
    log_info "[DRY-RUN] Would run: otter reference build \\"
    log_info "  --registry-root '$REGISTRY_ROOT' \\"
    log_info "  --id '$ref_id' --release '$release' \\"
    log_info "  --organism '$organism' --assembly '$assembly' \\"
    log_info "  --alias '$ref_id' \\"
    log_info "  --fasta '$fasta_path' --gtf '$gtf_path' \\"
    log_info "  --indexes '$indexes_flag' --index-build-threads $THREADS \\"
    log_info "  --samtools '$SAMTOOLS' --bowtie2-build '$BOWTIE2_BUILD' \\"
    log_info "  --bismark-genome-preparation '$BISMARK_PREP'"
    return 0
  fi

  if [[ "$MIGRATION_MODE" == "ingest" ]]; then
    log_info "Ingest mode: reusing existing legacy indexes (hardlinking + manifest generation)..."
    mkdir -p "$REGISTRY_ROOT/genomes/$ref_id"
    local staging_dir
    staging_dir="$(mktemp -d "$REGISTRY_ROOT/genomes/$ref_id/.staging-XXXXXX")"
    trap 'rm -rf "$staging_dir"' EXIT

    mkdir -p "$staging_dir/fasta" "$staging_dir/annotations" "$staging_dir/indexes"

    log_info "Linking FASTA and indexing with samtools faidx..."
    cp -l "$fasta_path" "$staging_dir/fasta/${ref_id}.fa" 2>/dev/null || cp "$fasta_path" "$staging_dir/fasta/${ref_id}.fa"
    "$SAMTOOLS" faidx "$staging_dir/fasta/${ref_id}.fa"

    log_info "Linking GTF..."
    cp -l "$gtf_path" "$staging_dir/annotations/${ref_id}.gtf" 2>/dev/null || cp "$gtf_path" "$staging_dir/annotations/${ref_id}.gtf"

    local legacy_bismark_dir="$(dirname "$fasta_path")/Bisulfite_Genome"
    if [[ -d "$legacy_bismark_dir" ]]; then
      log_info "Linking existing Bismark Bisulfite_Genome index..."
      mkdir -p "$staging_dir/indexes/bismark/genome"
      cp -rl "$legacy_bismark_dir" "$staging_dir/indexes/bismark/genome/" 2>/dev/null || cp -r "$legacy_bismark_dir" "$staging_dir/indexes/bismark/genome/"
    fi

    local legacy_star_dir="$(dirname "$gtf_path")"
    if [[ -f "$legacy_star_dir/Genome" ]]; then
      log_info "Linking existing STAR index..."
      mkdir -p "$staging_dir/indexes/star"
      for star_file in Genome SA SAindex chrLength.txt chrName.txt chrNameLength.txt chrStart.txt exonGeTrInfo.tab exonInfo.tab geneInfo.tab genomeParameters.txt sjdbInfo.txt sjdbList.fromGTF.out.tab sjdbList.out.tab transcriptInfo.tab; do
        if [[ -f "$legacy_star_dir/$star_file" ]]; then
          cp -l "$legacy_star_dir/$star_file" "$staging_dir/indexes/star/" 2>/dev/null || cp "$legacy_star_dir/$star_file" "$staging_dir/indexes/star/"
        fi
      done
    fi

    log_info "Generating reference.yaml..."
    local fasta_sha256
    fasta_sha256="sha256:$(sha256sum "$staging_dir/fasta/${ref_id}.fa" | cut -d' ' -f1)"
    local fasta_size
    fasta_size="$(stat -c '%s' "$staging_dir/fasta/${ref_id}.fa")"
    local gtf_sha256
    gtf_sha256="sha256:$(sha256sum "$staging_dir/annotations/${ref_id}.gtf" | cut -d' ' -f1)"

    cat > "$staging_dir/reference.yaml" << EOF_REF
schema_version: otter.reference/v1
reference:
    id: $ref_id
    release: $release
    organism: $organism
    assembly: $assembly
    aliases:
        - $ref_id
assets:
    fasta:
        path: fasta/${ref_id}.fa
        sha256: $fasta_sha256
        size_bytes: $fasta_size
        fai: fasta/${ref_id}.fa.fai
    annotations:
        - id: primary-gtf
          type: gtf
          path: annotations/${ref_id}.gtf
          sha256: $gtf_sha256
    indexes:
        - type: bismark
          path: indexes/bismark
          reference_fasta_sha256: $fasta_sha256
          tool: bismark_genome_preparation
          tool_version: legacy
          parameters:
            arguments:
                - --bowtie2
        - type: star
          path: indexes/star
          reference_fasta_sha256: $fasta_sha256
          tool: STAR
          tool_version: legacy
compatibility:
    scenarios:
        - bs-pdx
        - rrbs
        - wgbs
        - rnaseq
        - rna-pdx
    workflows:
        - BeaverBS
        - BeaverPDX
        - BeaverRNA
        - BeaverRNASEQPDX
EOF_REF

    log_info "Computing SHA-256 digests for manifest.json and checksums.sha256..."
    python3 -c "
import os, hashlib, json

root = '$staging_dir'
entries = []
for dirpath, _, filenames in os.walk(root):
    for f in filenames:
        if f in ['manifest.json', 'checksums.sha256']:
            continue
        full = os.path.join(dirpath, f)
        rel = os.path.relpath(full, root)
        h = hashlib.sha256()
        with open(full, 'rb') as fp:
            while chunk := fp.read(1048576):
                h.update(chunk)
        entries.append({'path': rel, 'sha256': 'sha256:' + h.hexdigest()})

entries.sort(key=lambda x: x['path'])
manifest_path = os.path.join(root, 'manifest.json')
with open(manifest_path, 'w') as fp:
    json.dump(entries, fp, indent=2)
    fp.write('\n')

h_man = hashlib.sha256()
with open(manifest_path, 'rb') as fp:
    while chunk := fp.read(1048576):
        h_man.update(chunk)

entries.append({'path': 'manifest.json', 'sha256': 'sha256:' + h_man.hexdigest()})
entries.sort(key=lambda x: x['path'])

with open(os.path.join(root, 'checksums.sha256'), 'w') as fp:
    for e in entries:
        fp.write(f\"{e['sha256'].replace('sha256:', '')}  {e['path']}\n\")
"

    log_info "Atomically publishing to $target_release_dir..."
    trap - EXIT
    chmod -R u+rwX "$target_release_dir" 2>/dev/null || true
    rm -rf "$target_release_dir"
    mv "$staging_dir" "$target_release_dir"

    log_info "Sealing reference release read-only..."
    find "$target_release_dir" -type f -exec chmod 0444 {} +
    find "$target_release_dir" -depth -type d -exec chmod 0555 {} +

    local manifest_digest
    manifest_digest="$(sha256sum "$target_release_dir/manifest.json" | cut -d' ' -f1)"
    log_success "Successfully ingested and published $ref_id to $target_release_dir"
    printf '\n'
    log_info "Add this snippet to your project's references.lock.yaml:"
    printf '    %s:\n' "$ref_id"
    printf '      release: %s\n' "$release"
    printf '      manifest_sha256: sha256:%s\n' "$manifest_digest"
    printf '\n'
    return 0
  fi

  local build_cmd=(
    "$OTTER_BIN" reference build
    --registry-root "$REGISTRY_ROOT"
    --id "$ref_id"
    --release "$release"
    --organism "$organism"
    --assembly "$assembly"
    --alias "$ref_id"
    --fasta "$fasta_path"
    --gtf "$gtf_path"
    --indexes "$indexes_flag"
    --index-build-threads "$THREADS"
    --samtools "$SAMTOOLS"
    --bowtie2-build "$BOWTIE2_BUILD"
    --bismark-genome-preparation "$BISMARK_PREP"
  )
  if [[ "$has_star" == "true" && -x "$STAR_BIN" ]]; then
    build_cmd+=(--star "$STAR_BIN")
  fi

  log_info "Starting reference build (this may take several minutes to hours depending on genome size)..."
  if "${build_cmd[@]}"; then
    log_success "Successfully published reference release to $target_release_dir"
    local manifest_digest
    manifest_digest="$(sha256sum "$target_release_dir/manifest.json" | cut -d' ' -f1)"
    printf '\n'
    log_info "Add this snippet to your project's references.lock.yaml:"
    printf '    %s:\n' "$ref_id"
    printf '      release: %s\n' "$release"
    printf '      manifest_sha256: sha256:%s\n' "$manifest_digest"
    printf '\n'
  else
    log_error "Reference build failed for $ref_id"
    return 1
  fi
}

# ── Discover and migrate human (hg19) ────────────────────────────────────────
migrate_hg19() {
  local hg19_fasta=""
  local hg19_gtf=""

  for candidate in \
    "$LEGACY_INST/pdx/homo_sapiens/hg19.fasta" \
    "$LEGACY_INST/rnaseq/homo_sapiens/hg19.fasta"; do
    if [[ -f "$candidate" ]]; then
      hg19_fasta="$candidate"
      break
    fi
  done

  for candidate in \
    "$LEGACY_INST/rnaseq/homo_sapiens/hg19.ensGene.gtf" \
    "$LEGACY_INST/rnaseq/homo_sapiens/hg19.ensGene_sorted.gtf"; do
    if [[ -f "$candidate" ]]; then
      hg19_gtf="$candidate"
      break
    fi
  done

  if [[ -n "$hg19_fasta" && -n "$hg19_gtf" ]]; then
    migrate_species "hg19" "hg19-legacy-ensGene" "Homo sapiens" "hg19" "$hg19_fasta" "$hg19_gtf" "true"
  else
    log_warn "hg19 assets incomplete in $LEGACY_INST (FASTA: ${hg19_fasta:-missing}, GTF: ${hg19_gtf:-missing})"
  fi
}

# ── Discover and migrate mouse (mm10 / GRCm38) ────────────────────────────────
migrate_mm10() {
  local mm10_fasta=""
  local mm10_gtf=""

  for candidate in \
    "$LEGACY_INST/pdx/mouse/GRCm38.fasta" \
    "$LEGACY_INST/rnaseq/mouse/GRCm38.fasta" \
    "$LEGACY_INST/rnaseq/mouse/mm10.fasta"; do
    if [[ -f "$candidate" ]]; then
      mm10_fasta="$candidate"
      break
    fi
  done

  for candidate in \
    "$LEGACY_INST/rnaseq/mouse/GRCm38.ensGene.gtf" \
    "$LEGACY_INST/rnaseq/mouse/mm10.ensGene.gtf" \
    "$LEGACY_INST/pdx/mouse/GRCm38.ensGene.gtf"; do
    if [[ -f "$candidate" ]]; then
      mm10_gtf="$candidate"
      break
    fi
  done

  if [[ -n "$mm10_fasta" ]]; then
    # If mouse GTF is not present in legacy pdx directory, generate minimal annotation stub or use available
    if [[ -z "$mm10_gtf" ]]; then
      log_warn "mm10 GTF not found in legacy pdx/mouse; checking for annotations"
    fi
    if [[ -n "$mm10_gtf" && -f "$mm10_gtf" ]]; then
      migrate_species "mm10" "GRCm38-legacy" "Mus musculus" "GRCm38" "$mm10_fasta" "$mm10_gtf" "true"
    fi
  else
    log_warn "mm10 assets incomplete in $LEGACY_INST (FASTA: ${mm10_fasta:-missing})"
  fi
}

case "$SPECIES_CHOICE" in
  hg19) migrate_hg19 ;;
  mm10) migrate_mm10 ;;
  all)
    migrate_hg19
    migrate_mm10
    ;;
  *)
    log_error "Unknown species choice: $SPECIES_CHOICE (expected: hg19, mm10, or all)"
    exit 2
    ;;
esac

log_success "Migration procedure completed."
