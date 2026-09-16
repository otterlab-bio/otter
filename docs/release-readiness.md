# Release readiness

This page is the release decision checklist for the current documentation and evidence boundary. It intentionally separates “the code builds,” “the bounded evidence is accepted,” and “the product is ready for a formal public release.”

## Current position

The project is in a **dual-track, conditional release-readiness** state:

- the root CLI, canonical v1 configuration path, workflow catalog, artifact contracts, and component documentation are available;
- bounded Gate 6 evidence is accepted for the documented Craftmake–Snakemake comparison, corrected classification controls, and Methx/Methrix parity;
- legacy projects still require explicit Snakemake compatibility routing;
- production-scale qualification, fresh representative matrices, complete WGBS qualification, and universal Snakemake retirement are not current claims.

## Current Paracloud status (2026-09-06)

Paracloud SSH access was restored in pseudo-terminal mode. The public-data inventory and Craftmake fixture-generation stage are complete:

- 8 paired FASTQ fixture sets were generated with 20,000 synchronized pairs per sample;
- 9 canonical paired BAM fixture sets follow the aligned-BAM-per-SRA policy: every mainline SRA contributes its alignment BAM (single reference for RRBS/RNA-seq; both hg38 and mm10 for PDX projects), each with exactly 20,000 complete paired templates;
- filtered (Xenofilx) outputs and the retired `SRR23802966` line are excluded from the BAM fixtures;
- all generated FASTQ, BAM, BAI, and manifest files are below 20 MiB;
- per-sample and aggregate SHA-256 manifests were created;
- source acquisition and accepted evidence directories were not modified.

The current Paracloud fixture inventory distinguishes source datasets from derived evidence artifacts. There are 8 unique decoded SRA accessions (9 acquisition directories because `SRR31480456` has a duplicate acquisition directory), and the current FASTQ fixture run covers all 8 unique accessions. `SRR6373947` was not found under `acquisitions/` or `data/`, including no decoded paired FASTQ.

The BAM inventory contains approximately 1,495 files: 172 under `benchmark-projects/`, 39 under `projects/`, and 1,284 under `evidence/`. These include repeated Craftmake/Snakemake runs, modern/legacy comparison outputs, temporary alignment products, and derived filtered BAMs; they are not 1,495 independent SRA datasets. The canonical inventory selects the aligned BAM of each mainline SRA (9 fixtures: 2 RRBS, 3 RNA-seq, 2 BS-PDX, 2 RNA-PDX) rather than blindly sampling every historical derivative, and all nine have been processed by Craftmake.

## Current installer and release status (2026-09-07)

- The release installer is now a statically compiled Go binary in `installer/` (standard library only, `CGO_ENABLED=0`), replacing the shell installer as the primary path.
- The installer downloads pre-built static binaries from GitHub Releases, creates conda environments, verifies pinned tool versions, and can run the Craftmake ReferenceBuild workflow.
- The installer binary is published to [`otterlab-bio/otter`](https://github.com/otterlab-bio/otter) as a release asset; the legacy `scripts/install.sh` remains for compatibility.
- Craftmake gained a `--gate` mode (immutable backend/run identity/Slurm resources only when `--gate` is passed) and a configurable ReferenceBuild backend/partition, enabling local reference builds for clean-server acceptance.

## Current Hangzhou cluster acceptance status (2026-09-07)

- Verified clean installation of the complete 10-tool static toolchain via `otter-install`.
- Verified `otter-core` conda environment creation with pinned Bismark 3.1.0, Bowtie2 2.5.5, samtools 1.24, STAR 2.7.11b, and rmats.py v4.4.0.
- Executed real migration of legacy `hg19` reference genome from `$beaverhome/inst` to modern immutable registry (`$OTTER_REFERENCE_ROOT/genomes/hg19/hg19-legacy-ensGene`), atomically published and sealed read-only.
- Executed BeaverBS step1 workflow on Slurm compute nodes (`comput3` on partition `cpu112c`) via `login1`: all 3 tasks succeeded, `allocations.csv` and step logs captured, and `craftmake resume` verified with 3/3 cache hits.

## Readiness checklist

### Documentation and user entry points

- [x] Root README and Chinese README explain value, first use, architecture, and limitations.
- [x] `docs/README.md` indexes current manuals, contracts, build instructions, and evidence.
- [x] Each component README explains inputs, outputs, installation, an example, limitations, and tests.
- [x] Current documentation uses current product names; historical evidence keeps historical names.
- [ ] Rendered SVG preview has been checked on GitHub and a narrow viewport; local XML and safety validation is complete.

### Paracloud downsample status (2026-09-06)

- [x] Inventory all current acquisition directories and deduplicate the 8 unique decoded SRA accessions.
- [x] Generate paired FASTQ fixtures for all 8 unique decoded accessions with synchronized read pairs and per-file size/checksum validation.
- [x] Build the canonical source-BAM inventory and generate matching BAM fixtures for every mainline SRA alignment BAM (9 fixtures: 2 RRBS, 3 RNA-seq, 2 BS-PDX, 2 RNA-PDX; PDX projects contribute both hg38 and mm10).
- [ ] Locate or reacquire WGBS `SRR6373947`; no decoded paired FASTQ is currently present on Paracloud.

### Runtime and compatibility

- [x] Legacy `otter.yaml` examples explicitly select `--executor snakemake`.
- [x] Canonical v1 examples resolve an immutable `run.yaml` before Craftmake execution.
- [x] Craftmake does not silently fall back to Snakemake.
- [x] Site/backend selection and reference validation have fail-closed rules.
- [x] A formal release build has been produced from the final versioned source and checked on the supported environments (verified on Hangzhou cluster CentOS 7 and compute nodes).

### Evidence and scientific claims

- [x] Accepted bounded executor-comparison evidence is indexed.
- [x] Corrected Gate A–D classification evidence is indexed.
- [x] Methx/Methrix parity and the R conversion path are documented.
- [x] Deferred work is visible and excluded from current claims.
- [ ] Any new release-specific acceptance requirement has an owner, evidence path, and decision record.

## Deferred, not silently missing

The following are deliberately deferred extensions, not undocumented completed work:

- fresh seven-input legacy-equivalent matrix;
- representative `20 samples × 3 repeats` matrix;
- production-scale throughput and scheduler pressure;
- WGBS `SRR6373947` requalification;
- additional Snakemake interruption/recovery studies.

A release may proceed only if the release owner explicitly accepts these limitations. Otherwise, each item needs its own plan and evidence entry; none should be inferred from local tests or README completeness.

## Recommended final release sequence

1. Correct any remaining cross-language command or link inconsistency.
2. Run root tests, component CI, and the final release build.
3. Verify README links, SVG metadata, and rendered images.
4. Freeze the evidence register and write a release decision record.
5. Publish with explicit `dual-track` wording and the deferred-work list.
6. Treat any future Snakemake retirement as a separate migration decision.

[Back to the documentation hub](README.md)
