# otter repo map

## Root repo

`otter` is the orchestration layer. It provides the main Go CLI, embeds workflow assets under `inst/`, and coordinates end-user workflows for RRBS, WGBS, RNA-seq, and PDX analysis.

Read these areas first when the task targets the root repo:

- `main.go`, `cmd/`, `internal/`, `pkg/` for Go CLI behavior
- `inst/` for embedded Snakemake rules, env YAMLs, and helper R scripts
- `scripts/` for install, setup, build, and release flows
- `testdata/` for fixtures

## Direct submodules

| Repo | Language | Role in the stack | Use when |
|------|----------|-------------------|----------|
| `craftmake` | Go | Native workflow compiler and controller for immutable runs (Local/SLURM, SQLite state, recovery, reports) | The task is about workflow compilation, YAML workflow authoring, execution backends, run state, resume/cancel, or artifact publication |
| `enva` | Rust | Rattler-first environment manager for `otter` runtime environments | The task is about env creation, activation, adoption, or compatibility with conda/mamba/micromamba |
| `bamdriver` | Go | Shared BAM and BGZF driver library extracted from `xenofilx` and `pairbam` | The task changes BAM I/O, sorting, indexing, or shared low-level sequence utilities |
| `pairbam` | Go | Filters paired-end BAM files and keeps only properly paired reads | The task is about paired BAM filtering outputs or BAM pairing logic |
| `xenofilx` | Go | Pure-Go XenofilteR implementation for graft vs host BAM filtering | The task is about PDX host/graft classification or XenofilteR-style BAM processing |
| `matsrun` | Go | rMATS orchestration CLI for pairwise RNA splicing comparisons | The task is about RNA splicing job generation, pdata parsing, or rmats.py execution flow |
| `seq2mat` | Go | HTSeq count directory to expression matrix converter | The task is about HTSeq parsing, gene mapping, or matrix output generation |
| `methx` | Rust | Bismark-to-HDF5 methylation processor compatible with the methrix R package | The task is about methylation processing, CpG extraction, or QC report generation |
| `qctb` | Rust | QC summary reporting CLI for sequencing workflows | The task is about QC summary generation or replacing legacy R QC scripts |
| `fastqcx` | Rust | FASTQ QC report generator with HTML and MultiQC summary output | The task is about raw FASTQ QC metrics, HTML reports, or summary file generation |

## Cross-repo guidance

- Start in the repo that owns the behavior being changed. Only cross into another repo when the interface boundary demands it.
- If a change in `bamdriver` affects `xenofilx` or `pairbam`, verify both consumers still compile or explain why you did not verify them.
- If `otter` changes integration with `enva`, `qctb`, `matsrun`, `seq2mat`, or other submodules, mention which boundary changed: CLI invocation, file layout, embedded assets, or expected outputs.
- `fastqcx` and some other submodules are standalone tools even when vendored here; avoid root-specific assumptions leaking into their own CLI semantics.
