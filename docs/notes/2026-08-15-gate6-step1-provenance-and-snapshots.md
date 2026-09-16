# Note 2026-08-15: Gate 6 Seven-Input Provenance, Snapshots, and Step1 Execution

> Status note for this round of updates. Companion records: `docs/gate6-paracloud-operations.md`, `docs/gate6-canary-matrix.md`, `docs/benchmark-plan.md`, `docs/active_context.md`.

## Commits in this round

| Repository | Commit | Scope |
|---|---|---|
| craftmake | `3fb94d0` | legacy FastQC/SeqKit step1 QC tasks in BeaverBS/BeaverRNA/BeaverPDX/BeaverRNASEQPDX; mutable run ID collision fix; standalone decode adapters; incident/evidence reporting; SRA decode workflows; compiler/CLI tests for the enlarged DAG |
| otter (parent) | `3088bb8` | `otter.sra-acquisition/v1` publication CLI (`acquisition publish`) and strict decode-manifest loader; canary-inputs schema; Gate 6 comparison docs; immutable run schema parity phase-resource contract; `.gitignore` for the 7.2 GB local Xenofilx replay fixture; submodule pointers (craftmake, bamdriver, pairbam, xenofilx) |

## What changed (execution evidence)

- **Provenance**: all seven decoded inputs (`SRR31480456`, `SRR1039508`, `SRR018258`, `SRR037954`, `SRR10025242`, `SRR23802966`, `SRR30880970`) now have create-only `otter.sra-acquisition/v1` records published at each acquisition root under `provenance/otter-sra-acquisition.json` (mode `0444`), with archive/FASTQ identity, sizes, SHA-256, paired record counts, and scenario reference bindings re-verified.
- **Runtime**: an isolated comparison runtime was deployed at `otter-gate6/toolchain-comparison-20260815T070000Z/runtime` (static Otter, static current-session Craftmake, current workflow catalog).
- **Snapshots**: seven authoritative `otter.run/v1` snapshots were resolved through `otter config resolve --site paracloud-gate6`; per-project `data/<sample>_R1/_R2.fastq.gz` symlinks point to the acquisition-declared `decoded/R1/R2.fastq.gz` files, and frozen sample digests equal the acquisition manifests. See the operations record for the authoritative run IDs.
- **Step1 (QC + Trim Galore)**: all seven projects were submitted through `otter run --foreground --executor craftmake --phase step1` (controllers `41457428`, `41457463`–`41457468`). Six of seven completed with all outputs (modern fastqcx QC, legacy FastQC + SeqKit oracle QC, shared Trim Galore paired outputs, trimmed QC, `seqkit_stat.txt`).
- **Legacy QC environment fix**: legacy FastQC/SeqKit tasks invoke `enva run otter-core -- fastqc|seqkit`; `otter-core` provides FastQC 0.12.1 and SeqKit 2.13.0 on Paracloud.
- **Recovery incident (documented)**: `human-rnaseq-SRR018258` raw FastQC attempt `41457483` hit the 2-hour step1 wall limit (`TIMEOUT`) under extreme node contention. Foreground resume controller `41457709` re-attempted the task, which succeeded on a fresh node in 71.6 s (job `41457710`); the phase-run status query still reports the original failed attempt while the resume run `8c8a455e…` succeeded.
- **In progress**: `bs-pdx-SRR23802966` (57 GB input) step1 controller `41457463` was still running; an autonomous recovery watcher (`runtime/recover-bs-pdx.sh`) waits up to 15 h for the controller to finalize and submits a resume controller on non-zero exit.

## Next steps

After bs-pdx step1 completes (or recovers), run the workflow-specific downstream phases (step2 alignment/count/splicing; step3 methylation for BS workflows) per scenario, then the modern/legacy toolchain comparison with `otter artifact compare`.
